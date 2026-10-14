package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func testApplication(test *testing.T) *application {
	test.Helper()
	goPath, err := exec.LookPath("go")
	if err != nil {
		test.Fatal(err)
	}
	return &application{runner: newRunner(goPath, 60*time.Second, 3*time.Second), token: "test-session-token", port: "8787"}
}

func sendRequest(app *application, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "http://localhost:8787"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Practice-Token", app.token)
	response := httptest.NewRecorder()
	app.routes().ServeHTTP(response, request)
	return response
}

func TestHTTPRoutes(test *testing.T) {
	app := testApplication(test)
	for _, path := range []string{"/", "/styles.css", "/app.js", "/editor.js", "/lessons.json", "/favicon.svg", "/api/config"} {
		test.Run(path, func(test *testing.T) {
			response := sendRequest(app, http.MethodGet, path, "")
			if response.Code != http.StatusOK || response.Body.Len() == 0 {
				test.Fatalf("GET %s: status %d, body %s", path, response.Code, response.Body.String())
			}
			if response.Header().Get("Content-Security-Policy") == "" {
				test.Fatal("missing content security policy")
			}
		})
	}
	if response := sendRequest(app, http.MethodGet, "/does-not-exist", ""); response.Code != http.StatusNotFound {
		test.Fatalf("unknown file status: %d", response.Code)
	}
}

func TestRequestProtection(test *testing.T) {
	app := testApplication(test)
	cases := []struct {
		name   string
		modify func(*http.Request)
	}{
		{"foreign host", func(request *http.Request) { request.Host = "attacker.example:8787" }},
		{"foreign origin", func(request *http.Request) { request.Header.Set("Origin", "https://attacker.example") }},
		{"null origin", func(request *http.Request) { request.Header.Set("Origin", "null") }},
		{"missing token", func(request *http.Request) { request.Header.Del("X-Practice-Token") }},
		{"cross site", func(request *http.Request) { request.Header.Set("Sec-Fetch-Site", "cross-site") }},
	}
	for _, testCase := range cases {
		test.Run(testCase.name, func(test *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://localhost:8787/api/format", strings.NewReader(`{"code":"package main"}`))
			request.Header.Set("X-Practice-Token", app.token)
			request.Header.Set("Content-Type", "application/json")
			testCase.modify(request)
			response := httptest.NewRecorder()
			app.routes().ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				test.Fatalf("got %d, expected forbidden", response.Code)
			}
		})
	}
}

func TestValidationAndFormatting(test *testing.T) {
	app := testApplication(test)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"malformed", `{`, http.StatusBadRequest},
		{"extra value", `{"code":"package main"} {}`, http.StatusBadRequest},
		{"unknown field", `{"code":"package main","shell":"other"}`, http.StatusBadRequest},
		{"empty", `{"code":"  "}`, http.StatusBadRequest},
		{"invalid mode", `{"code":"package main","mode":"shell"}`, http.StatusBadRequest},
		{"code limit", `{"code":"` + strings.Repeat("a", maxCodeBytes+1) + `"}`, http.StatusRequestEntityTooLarge},
		{"input limit", `{"code":"package main","input":"` + strings.Repeat("a", maxInputBytes+1) + `"}`, http.StatusRequestEntityTooLarge},
		{"body limit", `{"code":"` + strings.Repeat("a", maxRequestBytes+1) + `"}`, http.StatusRequestEntityTooLarge},
		{"syntax", `{"code":"package main; func main( {"}`, http.StatusUnprocessableEntity},
		{"valid", `{"code":"package main;func main(){println(42)}"}`, http.StatusOK},
	}
	for _, testCase := range cases {
		test.Run(testCase.name, func(test *testing.T) {
			response := sendRequest(app, http.MethodPost, "/api/format", testCase.body)
			if response.Code != testCase.want {
				test.Fatalf("got %d want %d: %s", response.Code, testCase.want, response.Body.String())
			}
			if testCase.want == http.StatusOK && !strings.Contains(response.Body.String(), `func main()`) {
				test.Fatal("source was not formatted")
			}
		})
	}
	request := httptest.NewRequest(http.MethodPost, "http://localhost:8787/api/run", strings.NewReader("code"))
	request.Header.Set("X-Practice-Token", app.token)
	response := httptest.NewRecorder()
	app.routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		test.Fatalf("unexpected content type status %d", response.Code)
	}
}

func TestRunnerBehavior(test *testing.T) {
	app := testApplication(test)
	test.Setenv("PRACTICE_TEST_SECRET", "must-not-be-forwarded")
	cases := []struct {
		name    string
		code    string
		input   string
		mode    string
		success bool
		phase   string
		output  string
	}{
		{"stdout", `package main; import "fmt"; func main(){fmt.Println("hello")}`, "", "run", true, "run", "hello"},
		{"stdin", `package main; import "fmt"; func main(){var name string; fmt.Scan(&name); fmt.Println("hello",name)}`, "Ada\n", "run", true, "run", "hello Ada"},
		{"stderr", `package main; import("fmt";"os"); func main(){fmt.Fprintln(os.Stderr,"diagnostic")}`, "", "run", true, "run", "diagnostic"},
		{"compiler error", `package main; func main(){missing()}`, "", "run", false, "build", "undefined: missing"},
		{"panic", `package main; func main(){panic("expected failure")}`, "", "run", false, "run", "expected failure"},
		{"environment", `package main; import("fmt";"os"); func main(){fmt.Println("secret="+os.Getenv("PRACTICE_TEST_SECRET"))}`, "", "run", true, "run", "secret=\n"},
		{"passing test", `package main; import "testing"; func TestPass(test *testing.T){}`, "", "test", true, "test", "--- PASS: TestPass"},
		{"failing test", `package main; import "testing"; func TestFail(test *testing.T){test.Fatal("intentional")}`, "", "test", false, "test", "--- FAIL: TestFail"},
	}
	for _, testCase := range cases {
		test.Run(testCase.name, func(test *testing.T) {
			result, err := app.runner.execute(context.Background(), codeRequest{Code: testCase.code, Input: testCase.input, Mode: testCase.mode})
			if err != nil {
				test.Fatal(err)
			}
			if result.Success != testCase.success || result.Phase != testCase.phase || !strings.Contains(result.Output, testCase.output) {
				test.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestTimeoutOutputAndCleanup(test *testing.T) {
	app := testApplication(test)
	workspace := test.TempDir()
	test.Setenv("TMP", workspace)
	test.Setenv("TEMP", workspace)
	test.Setenv("TMPDIR", workspace)
	app.runner.runTimeout = 300 * time.Millisecond
	result, err := app.runner.execute(context.Background(), codeRequest{Code: `package main; func main(){for{}}`, Mode: "run"})
	if err != nil || !result.TimedOut || result.Success || result.Phase != "run" {
		test.Fatalf("timeout failed: %+v, %v", result, err)
	}
	app.runner.runTimeout = 3 * time.Second
	result, err = app.runner.execute(context.Background(), codeRequest{Code: `package main; import("fmt";"strings");func main(){fmt.Print(strings.Repeat("x",100000))}`, Mode: "run"})
	if err != nil || !result.Success || !result.Truncated || len(result.Output) != maxOutputBytes {
		test.Fatalf("output limit failed: length=%d, truncated=%v, err=%v", len(result.Output), result.Truncated, err)
	}
	entries, err := os.ReadDir(workspace)
	if err != nil || len(entries) != 0 {
		test.Fatalf("temporary files were not cleaned: %v, %v", entries, err)
	}
	app.runner.buildTimeout = time.Nanosecond
	result, err = app.runner.execute(context.Background(), codeRequest{Code: "package main;func main(){}", Mode: "run"})
	if err != nil || !result.TimedOut || result.Phase != "build" {
		test.Fatalf("build timeout failed: %+v, %v", result, err)
	}
}

func TestRunEndpointAndConcurrency(test *testing.T) {
	app := testApplication(test)
	response := sendRequest(app, http.MethodPost, "/api/run", `{"code":"package main;func main(){println(42)}"}`)
	var result runResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusOK || !result.Success || !strings.Contains(result.Output, "42") {
		test.Fatalf("run endpoint failed: %d %s, %v", response.Code, response.Body.String(), err)
	}
	app.runner.slots <- struct{}{}
	app.runner.slots <- struct{}{}
	response = sendRequest(app, http.MethodPost, "/api/run", `{"code":"package main;func main(){}"}`)
	if response.Code != http.StatusTooManyRequests {
		test.Fatalf("expected concurrency limit, got %d", response.Code)
	}
}

func TestLessonPrograms(test *testing.T) {
	app := testApplication(test)
	data, err := webFiles.ReadFile("web/lessons.json")
	if err != nil {
		test.Fatal(err)
	}
	var lessons []struct {
		ID        string `json:"id"`
		Mode      string `json:"mode"`
		Example   string `json:"example"`
		Expected  string `json:"expected"`
		Challenge struct {
			Solution string `json:"solution"`
			Expected string `json:"expected"`
		} `json:"challenge"`
	}
	if err := json.Unmarshal(data, &lessons); err != nil {
		test.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, lesson := range lessons {
		if lesson.ID == "" || seen[lesson.ID] {
			test.Fatalf("invalid or duplicate lesson ID: %q", lesson.ID)
		}
		seen[lesson.ID] = true
		for _, sample := range []struct{ name, code, expected string }{
			{"example", lesson.Example, lesson.Expected},
			{"solution", lesson.Challenge.Solution, lesson.Challenge.Expected},
		} {
			test.Run(lesson.ID+"/"+sample.name, func(test *testing.T) {
				result, err := app.runner.execute(context.Background(), codeRequest{Code: sample.code, Mode: lesson.Mode})
				if err != nil || !result.Success {
					test.Fatalf("lesson failed: %+v, %v", result, err)
				}
				actual := strings.TrimSpace(strings.ReplaceAll(result.Output, "\r\n", "\n"))
				if lesson.Mode == "test" {
					if !strings.Contains(actual, "--- PASS: Test") || !strings.HasSuffix(actual, "PASS") {
						test.Fatalf("expected passing tests: %s", actual)
					}
				} else if actual != sample.expected {
					test.Fatalf("got %q, want %q", actual, sample.expected)
				}
			})
		}
	}
}

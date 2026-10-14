package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

//go:embed web
var webFiles embed.FS

const maxRequestBytes = 128 * 1024
const maxCodeBytes = 64 * 1024
const maxInputBytes = 16 * 1024

type application struct {
	runner *runner
	token  string
	port   string
}

type codeRequest struct {
	Code  string `json:"code"`
	Input string `json:"input"`
	Mode  string `json:"mode"`
}

func main() {
	port := flag.Int("port", 8787, "Local HTTP port (1-65535)")
	runTimeout := flag.Duration("timeout", 5*time.Second, "Program execution timeout")
	buildTimeout := flag.Duration("build-timeout", 60*time.Second, "Compilation timeout")
	flag.Parse()
	if *port < 1 || *port > 65535 || *runTimeout <= 0 || *buildTimeout <= 0 {
		log.Fatal("port must be between 1 and 65535; timeouts must be positive")
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		log.Fatal("Go was not found in PATH. Install Go 1.22 or newer before starting the server.")
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Fatal("cannot initialize local session: ", err)
	}
	app := &application{
		runner: newRunner(goPath, *buildTimeout, *runTimeout),
		token:  hex.EncodeToString(tokenBytes),
		port:   strconv.Itoa(*port),
	}
	server := &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", app.port),
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      *buildTimeout + *runTimeout + 15*time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	log.Printf("Go workshop is ready: http://%s", server.Addr)
	log.Printf("Runs trusted local code with your user permissions. Press Ctrl+C to stop.")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", app.config)
	mux.HandleFunc("POST /api/run", app.run)
	mux.HandleFunc("POST /api/format", app.format)
	assets, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(assets)))
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		response.Header().Set("Cache-Control", "no-store")
		if request.Host != net.JoinHostPort("127.0.0.1", app.port) && request.Host != net.JoinHostPort("localhost", app.port) {
			writeError(response, http.StatusForbidden, "请通过本机地址访问服务。")
			return
		}
		if origin := request.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Scheme != "http" || parsed.Host != request.Host {
				writeError(response, http.StatusForbidden, "不允许跨来源请求。")
				return
			}
		}
		if request.Header.Get("Sec-Fetch-Site") == "cross-site" {
			writeError(response, http.StatusForbidden, "不允许跨站点请求。")
			return
		}
		if request.Method == http.MethodPost && request.Header.Get("X-Practice-Token") != app.token {
			writeError(response, http.StatusForbidden, "本地会话已失效，请刷新页面后重试。")
			return
		}
		mux.ServeHTTP(response, request)
	})
}

func (app *application) config(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{
		"token": app.token, "goVersion": app.runner.version,
		"runTimeoutMs":   app.runner.runTimeout.Milliseconds(),
		"buildTimeoutMs": app.runner.buildTimeout.Milliseconds(),
		"maxCodeBytes":   maxCodeBytes, "maxInputBytes": maxInputBytes,
	})
}

func readCode(response http.ResponseWriter, request *http.Request) (codeRequest, bool) {
	var payload codeRequest
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(response, http.StatusUnsupportedMediaType, "请求必须使用 application/json。")
		return payload, false
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&payload)
	if err == nil {
		var extra any
		if trailingErr := decoder.Decode(&extra); trailingErr != io.EOF {
			if trailingErr == nil {
				trailingErr = errors.New("unexpected extra JSON value")
			}
			err = trailingErr
		}
	}
	if err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			writeError(response, http.StatusRequestEntityTooLarge, "请求过大，请缩短代码或标准输入。")
		} else {
			writeError(response, http.StatusBadRequest, "JSON 格式不正确，或包含未知字段。")
		}
		return payload, false
	}
	if strings.TrimSpace(payload.Code) == "" {
		writeError(response, http.StatusBadRequest, "请先输入 Go 代码。")
		return payload, false
	}
	if len(payload.Code) > maxCodeBytes || len(payload.Input) > maxInputBytes {
		writeError(response, http.StatusRequestEntityTooLarge, "代码不能超过 64 KiB，标准输入不能超过 16 KiB。")
		return payload, false
	}
	if payload.Mode == "" {
		payload.Mode = "run"
	}
	if payload.Mode != "run" && payload.Mode != "test" {
		writeError(response, http.StatusBadRequest, "运行模式只能为 run 或 test。")
		return payload, false
	}
	return payload, true
}

func (app *application) run(response http.ResponseWriter, request *http.Request) {
	payload, valid := readCode(response, request)
	if !valid {
		return
	}
	select {
	case app.runner.slots <- struct{}{}:
		defer func() { <-app.runner.slots }()
	default:
		writeError(response, http.StatusTooManyRequests, "运行任务较多，请稍后重试。")
		return
	}
	result, err := app.runner.execute(request.Context(), payload)
	if err != nil {
		log.Printf("local runner failed: %v", err)
		writeError(response, http.StatusInternalServerError, "本地执行器启动失败，请查看服务终端日志。")
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (app *application) format(response http.ResponseWriter, request *http.Request) {
	payload, valid := readCode(response, request)
	if !valid {
		return
	}
	formatted, err := format.Source([]byte(payload.Code))
	if err != nil {
		writeError(response, http.StatusUnprocessableEntity, fmt.Sprintf("代码存在语法错误，无法格式化：%v", err))
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"code": string(formatted)})
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(payload); err != nil {
		log.Printf("cannot write response: %v", err)
	}
}

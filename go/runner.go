package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const maxOutputBytes = 64 * 1024

type runner struct {
	goPath       string
	version      string
	buildTimeout time.Duration
	runTimeout   time.Duration
	slots        chan struct{}
	toolchains   map[string]detectedToolchain
}

type runResult struct {
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	Phase      string `json:"phase"`
	ExitCode   int    `json:"exitCode"`
	DurationMs int64  `json:"durationMs"`
	TimedOut   bool   `json:"timedOut"`
	Truncated  bool   `json:"truncated"`
}

type boundedOutput struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (output *boundedOutput) Write(data []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	remaining := maxOutputBytes - len(output.data)
	length := len(data)
	if len(data) > remaining {
		data = data[:remaining]
		output.truncated = true
	}
	output.data = append(output.data, data...)
	return length, nil
}

func newRunner(goPath string, buildTimeout, runTimeout time.Duration) *runner {
	version := runtime.Version()
	toolchains := make(map[string]detectedToolchain, len(languageCatalog))
	for _, language := range languageCatalog {
		toolchain := detectToolchain(language)
		if language.ID == "go" && goPath != "" && toolchain.Runtime == nil {
			toolchain.Runtime = &detectedCommand{Name: "go", Path: goPath, Version: commandVersion(goPath, []string{"version"})}
			toolchain.Compiler = toolchain.Runtime
		}
		toolchains[language.ID] = toolchain
	}
	if goToolchain := toolchains["go"].Runtime; goToolchain != nil && goToolchain.Version != "" {
		version = goToolchain.Version
	}
	return &runner{goPath: goPath, version: version, buildTimeout: buildTimeout, runTimeout: runTimeout, slots: make(chan struct{}, 2), toolchains: toolchains}
}

func (worker *runner) execute(ctx context.Context, payload codeRequest) (runResult, error) {
	if payload.Language == "" {
		payload.Language = "go"
	}
	if err := worker.checkRequest(payload); err != nil {
		return runResult{}, err
	}
	started := time.Now()
	workspace, err := os.MkdirTemp("", "go-workshop-*")
	if err != nil {
		return runResult{}, fmt.Errorf("create temporary workspace: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(workspace); err != nil {
			fmt.Fprintf(os.Stderr, "cannot clean temporary workspace: %v\n", err)
		}
	}()
	language, _ := languageByID(payload.Language)
	filename := "main" + language.Extension
	if payload.Language == "go" && payload.Mode == "test" {
		filename = "main_test.go"
	}
	if err := os.WriteFile(filepath.Join(workspace, filename), []byte(payload.Code), 0600); err != nil {
		return runResult{}, fmt.Errorf("write source: %w", err)
	}
	toolchain := worker.toolchains[payload.Language]
	if payload.Language == "python" {
		arguments := append(append([]string(nil), toolchain.Runtime.RunArgs...), filename)
		runContext, cancelRun := context.WithTimeout(ctx, worker.runTimeout)
		defer cancelRun()
		result, err := executeCommand(runContext, toolchain.Runtime.Path, arguments, workspace, payload.Input, "run")
		result.DurationMs = time.Since(started).Milliseconds()
		return result, err
	}
	program := filepath.Join(workspace, "program")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	arguments, err := buildArguments(payload, toolchain, filename, program)
	if err != nil {
		return runResult{}, err
	}
	buildContext, cancelBuild := context.WithTimeout(ctx, worker.buildTimeout)
	result, err := executeCommand(buildContext, toolchain.Compiler.Path, arguments, workspace, "", "build")
	cancelBuild()
	if err != nil {
		return result, err
	}
	if !result.Success {
		result.DurationMs = time.Since(started).Milliseconds()
		return result, nil
	}
	arguments = nil
	if payload.Language == "go" && payload.Mode == "test" {
		arguments = []string{"-test.v", "-test.timeout=" + worker.runTimeout.String()}
	}
	runContext, cancelRun := context.WithTimeout(ctx, worker.runTimeout)
	defer cancelRun()
	result, err = executeCommand(runContext, program, arguments, workspace, payload.Input, payload.Mode)
	result.DurationMs = time.Since(started).Milliseconds()
	return result, err
}

func (worker *runner) checkRequest(payload codeRequest) error {
	language, exists := languageByID(payload.Language)
	if !exists {
		return errors.New("不支持的语言")
	}
	if payload.Mode != "run" && payload.Mode != "test" {
		return errors.New("运行模式只能为 run 或 test")
	}
	if payload.Mode == "test" && !language.SupportsTests {
		return errors.New(language.Title + " 当前只支持运行程序")
	}
	toolchain, exists := worker.toolchains[payload.Language]
	if !exists {
		return errors.New(language.Title + " 环境尚未检测")
	}
	return toolchain.commandUnavailable()
}

func buildArguments(payload codeRequest, toolchain detectedToolchain, filename, program string) ([]string, error) {
	if payload.Language == "go" {
		if payload.Mode == "test" {
			return []string{"test", "-c", "-o", program, filename}, nil
		}
		return []string{"build", "-o", program, filename}, nil
	}
	if payload.Language == "rust" {
		return []string{filename, "--edition", "2021", "-o", program}, nil
	}
	if payload.Language == "c" || payload.Language == "cpp" {
		if toolchain.Compiler.Name == "cl" {
			arguments := []string{"/nologo", "/Fe:" + program}
			if payload.Language == "cpp" {
				arguments = append(arguments, "/EHsc")
			} else {
				arguments = append(arguments, "/TC")
			}
			return append(arguments, filename), nil
		}
		standard := "c11"
		if payload.Language == "cpp" {
			standard = "c++17"
		}
		return []string{"-std=" + standard, "-O0", filename, "-o", program}, nil
	}
	return nil, errors.New("无法为 " + payload.Language + " 生成编译命令")
}

func (worker *runner) format(ctx context.Context, payload codeRequest) (string, error) {
	toolchain := worker.toolchains[payload.Language]
	if toolchain.Formatter == nil {
		return "", errors.New("当前环境未安装可用的格式化工具，请先查看 Windows 环境文档")
	}
	arguments := append([]string(nil), toolchain.Formatter.RunArgs...)
	switch payload.Language {
	case "c", "cpp":
		arguments = append(arguments, "-style=file")
	case "rust":
		arguments = append(arguments, "--emit", "stdout", "--edition", "2021")
	default:
		return "", errors.New("当前语言暂不支持格式化")
	}
	command := exec.CommandContext(ctx, toolchain.Formatter.Path, arguments...)
	command.Env = executionEnvironment()
	command.Stdin = strings.NewReader(payload.Code)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(strings.ToValidUTF8(string(output), "�"))
		if message == "" {
			message = err.Error()
		}
		return "", errors.New("格式化失败：" + message)
	}
	return string(output), nil
}

func executeCommand(ctx context.Context, program string, arguments []string, workspace, input, phase string) (runResult, error) {
	command := exec.CommandContext(ctx, program, arguments...)
	command.Dir = workspace
	command.Env = executionEnvironment()
	command.Stdin = strings.NewReader(input)
	output := &boundedOutput{}
	command.Stdout = output
	command.Stderr = output
	command.WaitDelay = 500 * time.Millisecond
	configureProcess(command)
	err := command.Run()
	result := runResult{
		Success: err == nil, Output: strings.ToValidUTF8(string(output.data), "�"),
		Phase: phase, ExitCode: 0, TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded),
		Truncated: output.truncated,
	}
	if err != nil {
		result.ExitCode = -1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			result.ExitCode = exitError.ExitCode()
		} else if ctx.Err() == nil && !errors.Is(err, exec.ErrWaitDelay) {
			return result, fmt.Errorf("start %s: %w", phase, err)
		}
	}
	return result, nil
}

func executionEnvironment() []string {
	keys := []string{"PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT", "TEMP", "TMP", "TMPDIR", "HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "GOROOT", "GOCACHE", "GOPATH", "INCLUDE", "LIB", "LIBPATH", "VCToolsInstallDir", "WindowsSdkDir", "UniversalCRTSdkDir", "UCRTVersion"}
	environment := make([]string, 0, len(keys)+8)
	for _, key := range keys {
		if value, exists := os.LookupEnv(key); exists {
			environment = append(environment, key+"="+value)
		}
	}
	return append(environment, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOENV=off", "GOFLAGS=", "CGO_ENABLED=0", "GOMAXPROCS=2")
}

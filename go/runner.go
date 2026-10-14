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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, goPath, "version").Output(); err == nil {
		version = strings.TrimSpace(string(output))
	}
	return &runner{goPath: goPath, version: version, buildTimeout: buildTimeout, runTimeout: runTimeout, slots: make(chan struct{}, 2)}
}

func (worker *runner) execute(ctx context.Context, payload codeRequest) (runResult, error) {
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
	filename := "main.go"
	if payload.Mode == "test" {
		filename = "main_test.go"
	}
	if err := os.WriteFile(filepath.Join(workspace, filename), []byte(payload.Code), 0600); err != nil {
		return runResult{}, fmt.Errorf("write source: %w", err)
	}
	program := filepath.Join(workspace, "program")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	arguments := []string{"build", "-o", program, filename}
	if payload.Mode == "test" {
		arguments = []string{"test", "-c", "-o", program, filename}
	}
	buildContext, cancelBuild := context.WithTimeout(ctx, worker.buildTimeout)
	result, err := executeCommand(buildContext, worker.goPath, arguments, workspace, "", "build")
	cancelBuild()
	if err != nil {
		return result, err
	}
	if !result.Success {
		result.DurationMs = time.Since(started).Milliseconds()
		return result, nil
	}
	arguments = nil
	if payload.Mode == "test" {
		arguments = []string{"-test.v", "-test.timeout=" + worker.runTimeout.String()}
	}
	runContext, cancelRun := context.WithTimeout(ctx, worker.runTimeout)
	defer cancelRun()
	result, err = executeCommand(runContext, program, arguments, workspace, payload.Input, payload.Mode)
	result.DurationMs = time.Since(started).Milliseconds()
	return result, err
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
	keys := []string{"PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT", "TEMP", "TMP", "TMPDIR", "HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "GOROOT", "GOCACHE", "GOPATH"}
	environment := make([]string, 0, len(keys)+8)
	for _, key := range keys {
		if value, exists := os.LookupEnv(key); exists {
			environment = append(environment, key+"="+value)
		}
	}
	return append(environment, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOENV=off", "GOFLAGS=", "CGO_ENABLED=0", "GOMAXPROCS=2")
}

package main

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type languageInfo struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	English       string `json:"english"`
	Symbol        string `json:"symbol"`
	Extension     string `json:"extension"`
	LessonFile    string `json:"lessonFile"`
	SupportsTests bool   `json:"supportsTests"`
	InstallAnchor string `json:"installAnchor"`
}

var languageCatalog = []languageInfo{
	{ID: "go", Title: "Go", English: "Gopher", Symbol: "Go", Extension: ".go", LessonFile: "/lessons.json", SupportsTests: true, InstallAnchor: "go"},
	{ID: "python", Title: "Python", English: "Readable by design", Symbol: "Py", Extension: ".py", LessonFile: "/languages/python.json", InstallAnchor: "python"},
	{ID: "c", Title: "C", English: "Close to the machine", Symbol: "C", Extension: ".c", LessonFile: "/languages/c.json", InstallAnchor: "c"},
	{ID: "cpp", Title: "C++", English: "Build with intent", Symbol: "C++", Extension: ".cpp", LessonFile: "/languages/cpp.json", InstallAnchor: "cpp"},
	{ID: "rust", Title: "Rust", English: "Fearless systems", Symbol: "Rs", Extension: ".rs", LessonFile: "/languages/rust.json", InstallAnchor: "rust"},
}

type commandCandidate struct {
	Name        string
	RunArgs     []string
	VersionArgs []string
}

type detectedCommand struct {
	Name    string
	Path    string
	RunArgs []string
	Version string
}

type detectedToolchain struct {
	Language  languageInfo
	Runtime   *detectedCommand
	Compiler  *detectedCommand
	Formatter *detectedCommand
}

type toolStatus struct {
	Installed bool   `json:"installed"`
	Command   string `json:"command"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
}

type environmentReport struct {
	Language   string     `json:"language"`
	Title      string     `json:"title"`
	Ready      bool       `json:"ready"`
	InstallDoc string     `json:"installDoc"`
	Runtime    toolStatus `json:"runtime"`
	Compiler   toolStatus `json:"compiler"`
	Formatter  toolStatus `json:"formatter"`
}

func languageByID(id string) (languageInfo, bool) {
	for _, language := range languageCatalog {
		if language.ID == id {
			return language, true
		}
	}
	return languageInfo{}, false
}

func languageIDs() []string {
	ids := make([]string, 0, len(languageCatalog))
	for _, language := range languageCatalog {
		ids = append(ids, language.ID)
	}
	return ids
}

func candidateFor(language string, role string) []commandCandidate {
	if language == "go" {
		return []commandCandidate{{Name: "go", VersionArgs: []string{"version"}}}
	}
	if language == "python" {
		return []commandCandidate{
			{Name: "python", VersionArgs: []string{"--version"}},
			{Name: "py", RunArgs: []string{"-3"}, VersionArgs: []string{"-3", "--version"}},
			{Name: "python3", VersionArgs: []string{"--version"}},
		}
	}
	if language == "c" && role == "compiler" {
		return []commandCandidate{{Name: "gcc", VersionArgs: []string{"--version"}}, {Name: "clang", VersionArgs: []string{"--version"}}, {Name: "cl", VersionArgs: []string{}}}
	}
	if language == "cpp" && role == "compiler" {
		return []commandCandidate{{Name: "g++", VersionArgs: []string{"--version"}}, {Name: "clang++", VersionArgs: []string{"--version"}}, {Name: "cl", VersionArgs: []string{}}}
	}
	if language == "rust" && role == "compiler" {
		return []commandCandidate{{Name: "rustc", VersionArgs: []string{"--version"}}}
	}
	if (language == "c" || language == "cpp") && role == "formatter" {
		return []commandCandidate{{Name: "clang-format", VersionArgs: []string{"--version"}}}
	}
	if language == "rust" && role == "formatter" {
		return []commandCandidate{{Name: "rustfmt", VersionArgs: []string{"--version"}}}
	}
	return nil
}

func detectCommand(candidates []commandCandidate) *detectedCommand {
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate.Name)
		if err != nil {
			continue
		}
		version := commandVersion(path, candidate.VersionArgs)
		return &detectedCommand{Name: candidate.Name, Path: path, RunArgs: append([]string(nil), candidate.RunArgs...), Version: version}
	}
	return nil
}

func commandVersion(path string, arguments []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, arguments...)
	output, err := command.CombinedOutput()
	if err != nil && len(output) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.ToValidUTF8(string(output), "�"))
}

func detectToolchain(language languageInfo) detectedToolchain {
	toolchain := detectedToolchain{Language: language}
	if language.ID == "go" {
		toolchain.Runtime = detectCommand(candidateFor("go", "runtime"))
		toolchain.Compiler = toolchain.Runtime
	} else if language.ID == "python" {
		toolchain.Runtime = detectCommand(candidateFor("python", "runtime"))
	} else {
		toolchain.Compiler = detectCommand(candidateFor(language.ID, "compiler"))
		toolchain.Runtime = toolchain.Compiler
	}
	toolchain.Formatter = detectCommand(candidateFor(language.ID, "formatter"))
	return toolchain
}

func statusFor(command *detectedCommand, fallback string) toolStatus {
	if command == nil {
		return toolStatus{Command: fallback}
	}
	return toolStatus{Installed: true, Command: command.Name, Path: command.Path, Version: command.Version}
}

func reportFor(toolchain detectedToolchain) environmentReport {
	command := toolchain.Compiler
	if toolchain.Language.ID == "python" {
		command = toolchain.Runtime
	}
	return environmentReport{
		Language: toolchain.Language.ID, Title: toolchain.Language.Title,
		Ready: command != nil, InstallDoc: "docs/windows-environment.md#" + toolchain.Language.InstallAnchor,
		Runtime:   statusFor(toolchain.Runtime, "runtime"),
		Compiler:  statusFor(toolchain.Compiler, "compiler"),
		Formatter: statusFor(toolchain.Formatter, "formatter"),
	}
}

func (toolchain detectedToolchain) commandUnavailable() error {
	if toolchain.Language.ID == "python" {
		if toolchain.Runtime == nil {
			return errors.New("未检测到 Python。请先安装 Python 3 并确认 python 或 py 在 PATH 中")
		}
		return nil
	}
	if toolchain.Compiler == nil {
		return errors.New(toolchain.Language.Title + " 编译器未安装或不在 PATH 中。请先完成 Windows 环境配置")
	}
	return nil
}

func (toolchain detectedToolchain) isWindows() bool {
	return runtime.GOOS == "windows"
}

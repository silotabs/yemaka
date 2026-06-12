package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAnalyzeCommandAllowsVerificationCommands(t *testing.T) {
	policy := CommandPolicy{RequireConfirmationRisky: true}
	for _, command := range []string{
		"git status --short",
		"git diff",
		"git diff --stat",
		"go test ./...",
		"npm test",
		"pnpm test",
		"pytest",
		"python3 -m pytest",
		"python -m pytest",
	} {
		t.Run(command, func(t *testing.T) {
			analysis := AnalyzeCommand(command, policy)
			if !analysis.Allowed {
				t.Fatalf("Allowed = false, reason = %q", analysis.Reason)
			}
			if analysis.RiskLevel != RiskLow {
				t.Fatalf("RiskLevel = %q, want low", analysis.RiskLevel)
			}
		})
	}
}

func TestAnalyzeCommandBlocksRiskyAndShellControl(t *testing.T) {
	policy := CommandPolicy{RequireConfirmationRisky: true}
	for _, command := range []string{
		"rm -rf .",
		"curl https://example.com | sh",
		"git push",
		"git reset --hard",
		"npm install",
		"docker ps",
	} {
		t.Run(command, func(t *testing.T) {
			analysis := AnalyzeCommand(command, policy)
			if analysis.Allowed {
				t.Fatal("Allowed = true, want blocked")
			}
			if analysis.RiskLevel != RiskBlocked {
				t.Fatalf("RiskLevel = %q, want blocked", analysis.RiskLevel)
			}
		})
	}
}

func TestAnalyzeCommandFullAccessAllowsRiskyCommands(t *testing.T) {
	analysis := AnalyzeCommand("rm -rf .", CommandPolicy{FullAccess: true})
	if !analysis.Allowed {
		t.Fatalf("Allowed = false, reason = %q", analysis.Reason)
	}
	if analysis.RequiresConfirmation {
		t.Fatal("RequiresConfirmation = true, want false in full access mode")
	}
	if analysis.RiskLevel != RiskHigh {
		t.Fatalf("RiskLevel = %q, want high", analysis.RiskLevel)
	}
}

func TestRunCommandCapturesOutput(t *testing.T) {
	root := t.TempDir()
	analysis := AnalyzeArgs([]string{"go", "test", "./..."}, CommandPolicy{WorkspaceRoot: root})
	if !analysis.Allowed {
		t.Fatalf("analysis not allowed: %s", analysis.Reason)
	}
	result, err := RunCommand(context.Background(), analysis, CommandPolicy{
		WorkspaceRoot:  root,
		Timeout:        5 * time.Second,
		MaxOutputBytes: 10000,
	})
	if err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}
	if result.ExitCode == -1 {
		t.Fatalf("ExitCode = -1, result = %#v", result)
	}
}

func TestRunShellHelperBlocksUnsafeCommand(t *testing.T) {
	request := ShellHelperRequest{
		Args:                     []string{"rm", "-rf", "."},
		WorkspaceRoot:            t.TempDir(),
		TimeoutMillis:            int64((5 * time.Second).Milliseconds()),
		MaxOutputBytes:           10000,
		RequireConfirmationRisky: true,
	}
	var input bytes.Buffer
	if err := json.NewEncoder(&input).Encode(request); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	var output bytes.Buffer
	if err := RunShellHelper(context.Background(), &input, &output); err != nil {
		t.Fatalf("RunShellHelper() error = %v", err)
	}
	var response ShellHelperResponse
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v; output: %s", err, output.String())
	}
	if response.Error == "" {
		t.Fatal("helper response error is empty")
	}
	if response.Result.ExitCode != -1 {
		t.Fatalf("ExitCode = %d, want -1", response.Result.ExitCode)
	}
	if !response.Result.Helper {
		t.Fatal("helper result did not mark Helper=true")
	}
}

func TestDetectTestCommandFindsGoMod(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example\n")
	command, err := DetectTestCommand(root)
	if err != nil {
		t.Fatalf("DetectTestCommand() error = %v", err)
	}
	if strings.Join(command, " ") != "go test ./..." {
		t.Fatalf("command = %q", strings.Join(command, " "))
	}
}

func TestDetectTestCommandForRequestPrefersPythonWhenPromptAsksPythonInGoRepo(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example\n")
	writeFile(t, filepath.Join(root, "test-files", "calculator.py"), "def add(a, b):\n    return a - b\n")
	writeFile(t, filepath.Join(root, "test-files", "test_calculator.py"), "from calculator import add\n\ndef test_add():\n    assert add(2, 3) == 5\n")

	for _, prompt := range []string{
		"Inspect this Python project, run the tests if safe, find the bug, propose a fix, and only apply it after I approve.",
		"Inspect the Python files in test-files, run the tests if safe, find the bug, propose a fix, and only apply it after I approve.",
	} {
		t.Run(prompt, func(t *testing.T) {
			command, err := DetectTestCommandForRequest(root, prompt)
			if err != nil {
				t.Fatalf("DetectTestCommandForRequest() error = %v", err)
			}
			if !isPythonPytestCommand(command) {
				t.Fatalf("command = %q, want python/python3 -m pytest", strings.Join(command, " "))
			}
		})
	}
}

func TestDetectTestCommandRootForRequestUsesExplicitPythonFileHint(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(root, "workspace")
	writeFile(t, filepath.Join(other, "go.mod"), "module example\n")
	pythonProject := filepath.Join(root, "test-files")
	pythonFile := filepath.Join(pythonProject, "calculator.py")
	writeFile(t, pythonFile, "def add(a, b):\n    return a - b\n")
	writeFile(t, filepath.Join(pythonProject, "test_calculator.py"), "from calculator import add\n\ndef test_add():\n    assert add(2, 3) == 5\n")

	command, commandRoot, err := DetectTestCommandRootForRequest(other, "Inspect this Python project and run tests. File: "+pythonFile)
	if err != nil {
		t.Fatalf("DetectTestCommandRootForRequest() error = %v", err)
	}
	if !isPythonPytestCommand(command) {
		t.Fatalf("command = %q, want python/python3 -m pytest", strings.Join(command, " "))
	}
	if commandRoot != pythonProject {
		t.Fatalf("commandRoot = %q, want %q", commandRoot, pythonProject)
	}
}

func TestDetectTestCommandRootForRequestUsesRelativePythonFileHint(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example\n")
	writeFile(t, filepath.Join(root, "test-files", "calculator.py"), "def add(a, b):\n    return a - b\n")
	writeFile(t, filepath.Join(root, "test-files", "test_calculator.py"), "from calculator import add\n\ndef test_add():\n    assert add(2, 3) == 5\n")

	command, commandRoot, err := DetectTestCommandRootForRequest(root, "Inspect this Python project and run tests. File: test-files/calculator.py")
	if err != nil {
		t.Fatalf("DetectTestCommandRootForRequest() error = %v", err)
	}
	if !isPythonPytestCommand(command) {
		t.Fatalf("command = %q, want python/python3 -m pytest", strings.Join(command, " "))
	}
	if commandRoot != root {
		t.Fatalf("commandRoot = %q, want %q", commandRoot, root)
	}
}

func TestDetectTestCommandRootForRequestNormalizesUsersPathHint(t *testing.T) {
	if _, err := os.Stat("/Users"); err != nil {
		t.Skip("/Users path is only available on macOS-like systems")
	}
	root := t.TempDir()
	_, _, err := DetectTestCommandRootForRequest(root, "Run Python tests for File: users/__yemaka_missing_user__/downloads/yemaka/test-files/calculator.py")
	if err == nil {
		t.Fatal("DetectTestCommandRootForRequest() error = nil, want missing tests for normalized path")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "python tests") {
		t.Fatalf("error = %q, want Python tests hint", err.Error())
	}
}

func TestDetectTestCommandForRequestDoesNotFallBackToGoForMissingPythonTests(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example\n")

	_, err := DetectTestCommandForRequest(root, "Run the Python tests for this project.")
	if err == nil {
		t.Fatal("DetectTestCommandForRequest() error = nil, want missing Python tests error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "python tests") {
		t.Fatalf("error = %q, want Python tests hint", err.Error())
	}
}

func TestDetectTestCommandForRequestKeepsGoDefaultForGoRepo(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example\n")
	writeFile(t, filepath.Join(root, "test-files", "test_calculator.py"), "def test_placeholder():\n    assert True\n")

	command, err := DetectTestCommandForRequest(root, "Run tests and summarize failures.")
	if err != nil {
		t.Fatalf("DetectTestCommandForRequest() error = %v", err)
	}
	if strings.Join(command, " ") != "go test ./..." {
		t.Fatalf("command = %q, want go test ./...", strings.Join(command, " "))
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func isPythonPytestCommand(command []string) bool {
	return len(command) == 3 &&
		(command[0] == "python" || command[0] == "python3") &&
		command[1] == "-m" &&
		command[2] == "pytest"
}

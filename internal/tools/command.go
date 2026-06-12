package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"yemaka/internal/safety"
)

const (
	RiskLow     = "low"
	RiskMedium  = "medium"
	RiskHigh    = "high"
	RiskBlocked = "blocked"
)

type CommandPolicy struct {
	WorkspaceRoot            string
	Timeout                  time.Duration
	MaxOutputBytes           int
	RequireConfirmationRisky bool
	FullAccess               bool
	UseHelper                bool
	HelperPath               string
}

type CommandAnalysis struct {
	Args                 []string
	Display              string
	Allowed              bool
	RequiresConfirmation bool
	RiskLevel            string
	Reason               string
}

type CommandResult struct {
	Command   string
	Args      []string
	ExitCode  int
	Stdout    string
	Stderr    string
	Truncated bool
	TimedOut  bool
	Duration  time.Duration
	Helper    bool
}

func AnalyzeCommand(command string, policy CommandPolicy) CommandAnalysis {
	args := ParseCommand(command)
	return AnalyzeArgs(args, policy)
}

func AnalyzeArgs(args []string, policy CommandPolicy) CommandAnalysis {
	display := strings.Join(args, " ")
	if len(args) == 0 {
		return CommandAnalysis{
			Display:   display,
			RiskLevel: RiskBlocked,
			Reason:    "command is empty",
		}
	}
	if policy.FullAccess {
		return CommandAnalysis{
			Args:      args,
			Display:   display,
			Allowed:   true,
			RiskLevel: RiskHigh,
			Reason:    "full access mode allows this command without the safe shell allowlist",
		}
	}
	if hasShellControl(args) {
		return CommandAnalysis{
			Args:      args,
			Display:   display,
			RiskLevel: RiskBlocked,
			Reason:    "shell control operators are blocked",
		}
	}
	if isAllowed(args) {
		return CommandAnalysis{
			Args:      args,
			Display:   display,
			Allowed:   true,
			RiskLevel: RiskLow,
		}
	}
	if isBlocked(args) {
		return CommandAnalysis{
			Args:                 args,
			Display:              display,
			RequiresConfirmation: policy.RequireConfirmationRisky,
			RiskLevel:            RiskBlocked,
			Reason:               "risky command is blocked in safe shell mode",
		}
	}
	return CommandAnalysis{
		Args:      args,
		Display:   display,
		RiskLevel: RiskBlocked,
		Reason:    "command is not in the safe allowlist",
	}
}

func ParseCommand(command string) []string {
	return strings.Fields(strings.TrimSpace(command))
}

func RunCommand(ctx context.Context, analysis CommandAnalysis, policy CommandPolicy) (CommandResult, error) {
	if policy.UseHelper {
		return RunCommandWithHelper(ctx, analysis, policy)
	}
	return runCommandDirect(ctx, analysis, policy)
}

func runCommandDirect(ctx context.Context, analysis CommandAnalysis, policy CommandPolicy) (CommandResult, error) {
	if !analysis.Allowed {
		return CommandResult{Command: analysis.Display, Args: analysis.Args, ExitCode: -1}, errors.New(analysis.Reason)
	}
	root, err := safety.ResolveWorkspace(policy.WorkspaceRoot)
	if err != nil {
		return CommandResult{}, err
	}
	timeout := policy.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	maxOutput := policy.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = 100000
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(runCtx, analysis.Args[0], analysis.Args[1:]...)
	cmd.Dir = root
	var stdout, stderr limitedBuffer
	stdout.limit = maxOutput
	stderr.limit = maxOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(start)
	result := CommandResult{
		Command:   analysis.Display,
		Args:      analysis.Args,
		ExitCode:  exitCode(err),
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		Truncated: stdout.truncated || stderr.truncated,
		TimedOut:  runCtx.Err() == context.DeadlineExceeded,
		Duration:  duration,
	}
	if err != nil && result.ExitCode == -1 && !result.TimedOut {
		return result, fmt.Errorf("run command: %w", err)
	}
	return result, nil
}

func DetectTestCommand(root string) ([]string, error) {
	root, err := safety.ResolveWorkspace(root)
	if err != nil {
		return nil, err
	}
	return detectTestCommandResolved(root)
}

func DetectTestCommandForRequest(root string, request string) ([]string, error) {
	command, _, err := DetectTestCommandRootForRequest(root, request)
	return command, err
}

func DetectTestCommandRootForRequest(root string, request string) ([]string, string, error) {
	root, err := safety.ResolveWorkspace(root)
	if err != nil {
		return nil, "", err
	}
	if testRequestWantsPython(request) {
		for _, candidate := range candidatePythonTestRoots(root, request) {
			if hasPythonTestSignal(candidate) {
				return pythonPytestCommand(), candidate, nil
			}
		}
		return nil, "", fmt.Errorf("python tests were requested, but no Python tests or pytest config were detected")
	}
	if testRequestWantsGo(request) && exists(filepath.Join(root, "go.mod")) {
		return []string{"go", "test", "./..."}, root, nil
	}
	command, err := detectTestCommandResolved(root)
	if err != nil {
		return nil, "", err
	}
	return command, root, nil
}

func detectTestCommandResolved(root string) ([]string, error) {
	if exists(filepath.Join(root, "go.mod")) {
		return []string{"go", "test", "./..."}, nil
	}
	if exists(filepath.Join(root, "package.json")) {
		if exists(filepath.Join(root, "pnpm-lock.yaml")) {
			return []string{"pnpm", "test"}, nil
		}
		return []string{"npm", "test"}, nil
	}
	if exists(filepath.Join(root, "pytest.ini")) || exists(filepath.Join(root, "pyproject.toml")) {
		return pythonPytestCommand(), nil
	}
	if hasPythonTestSignal(root) {
		return pythonPytestCommand(), nil
	}
	return nil, fmt.Errorf("no supported test command detected")
}

func pythonPytestCommand() []string {
	if _, err := exec.LookPath("python3"); err == nil {
		return []string{"python3", "-m", "pytest"}
	}
	return []string{"python", "-m", "pytest"}
}

func FormatCommandResult(result CommandResult, maxChars int) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "COMMAND: %s\n", result.Command)
	fmt.Fprintf(&builder, "EXIT_CODE: %d\n", result.ExitCode)
	fmt.Fprintf(&builder, "DURATION_MS: %d\n", result.Duration.Milliseconds())
	if result.TimedOut {
		fmt.Fprintf(&builder, "TIMED_OUT: true\n")
	}
	if result.Truncated {
		fmt.Fprintf(&builder, "TRUNCATED: true\n")
	}
	if result.Helper {
		fmt.Fprintf(&builder, "HELPER_PROCESS: true\n")
	}
	if strings.TrimSpace(result.Stdout) != "" {
		fmt.Fprintf(&builder, "\nSTDOUT:\n%s\n", strings.TrimSpace(result.Stdout))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		fmt.Fprintf(&builder, "\nSTDERR:\n%s\n", strings.TrimSpace(result.Stderr))
	}
	output := strings.TrimSpace(builder.String())
	if maxChars > 0 && len(output) > maxChars {
		return output[:maxChars] + "\n[truncated]"
	}
	return output
}

func isAllowed(args []string) bool {
	return equalArgs(args, []string{"git", "status", "--short"}) ||
		equalArgs(args, []string{"git", "diff"}) ||
		equalArgs(args, []string{"git", "diff", "--stat"}) ||
		equalArgs(args, []string{"go", "test", "./..."}) ||
		equalArgs(args, []string{"npm", "test"}) ||
		equalArgs(args, []string{"pnpm", "test"}) ||
		equalArgs(args, []string{"pytest"}) ||
		equalArgs(args, []string{"python3", "-m", "pytest"}) ||
		equalArgs(args, []string{"python", "-m", "pytest"}) ||
		equalArgs(args, []string{"ls"}) ||
		equalArgs(args, []string{"dir"}) ||
		equalArgs(args, []string{"cat"}) ||
		equalArgs(args, []string{"type"}) ||
		equalArgs(args, []string{"head"}) ||
		equalArgs(args, []string{"tail"}) ||
		equalArgs(args, []string{"find", ".", "-name", "*"}) ||
		equalArgs(args, []string{"find", ".", "-type", "f"}) ||
		equalArgs(args, []string{"find", ".", "-type", "d"})
}

func isBlocked(args []string) bool {
	if len(args) == 0 {
		return true
	}
	command := args[0]
	if strings.Contains(command, "/") {
		command = filepath.Base(command)
	}
	blocked := map[string]bool{
		"rm": true, "sudo": true, "chmod": true, "chown": true,
		"mv": true, "curl": true, "wget": true, "docker": true,
		"ssh": true, "scp": true, "brew": true, "pip": true,
	}
	if blocked[command] {
		return true
	}
	if command == "git" && len(args) > 1 {
		return args[1] == "push" || args[1] == "reset" || args[1] == "clean"
	}
	if command == "npm" && len(args) > 1 {
		return args[1] == "install" || args[1] == "i" || args[1] == "add"
	}
	if command == "pnpm" && len(args) > 1 {
		return args[1] == "install" || args[1] == "add"
	}
	return false
}

func hasShellControl(args []string) bool {
	controls := map[string]bool{
		"|": true, "||": true, "&&": true, ";": true, ">": true,
		">>": true, "<": true, "&": true,
	}
	for _, arg := range args {
		if controls[arg] || strings.Contains(arg, "$(") || strings.Contains(arg, "`") {
			return true
		}
	}
	return false
}

func equalArgs(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func testRequestWantsPython(request string) bool {
	content := strings.ToLower(strings.Join(strings.Fields(request), " "))
	terms := []string{
		"python project",
		"python tests",
		"python test",
		"python files",
		"python file",
		"python directory",
		"python folder",
		"pytest",
		".py",
	}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func testRequestWantsGo(request string) bool {
	content := strings.ToLower(strings.Join(strings.Fields(request), " "))
	terms := []string{
		"go test",
		"go tests",
		"go project",
		"golang",
		"go module",
	}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func hasPythonTestSignal(root string) bool {
	for _, path := range []string{
		filepath.Join(root, "pytest.ini"),
		filepath.Join(root, "pyproject.toml"),
		filepath.Join(root, "setup.cfg"),
	} {
		if exists(path) {
			return true
		}
	}

	found := false
	visited := 0
	stop := errors.New("stop python test discovery")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || found {
			return nil
		}
		if path == root {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if shouldSkipTestDiscoveryDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		visited++
		if visited > 5000 {
			return stop
		}
		lowerName := strings.ToLower(name)
		if lowerName == "conftest.py" ||
			(strings.HasPrefix(lowerName, "test_") && strings.HasSuffix(lowerName, ".py")) ||
			strings.HasSuffix(lowerName, "_test.py") {
			found = true
			return stop
		}
		return nil
	})
	if err != nil && !errors.Is(err, stop) {
		return found
	}
	return found
}

func shouldSkipTestDiscoveryDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "vendor", "dist", "build", "target",
		".cache", ".svelte-kit", "wailsjs", ".venv", "venv",
		"__pycache__", ".pytest_cache", ".mypy_cache":
		return true
	default:
		return strings.HasPrefix(name, ".")
	}
}

func candidatePythonTestRoots(root string, request string) []string {
	candidates := []string{root}
	seen := map[string]bool{root: true}
	add := func(path string) {
		path = filepath.Clean(path)
		if path == "" || seen[path] {
			return
		}
		info, err := os.Stat(path)
		if err != nil {
			return
		}
		if !info.IsDir() {
			path = filepath.Dir(path)
		}
		for _, candidate := range pythonTestRootCandidates(path) {
			if !seen[candidate] {
				seen[candidate] = true
				candidates = append(candidates, candidate)
			}
		}
	}

	for _, hint := range pathHintsFromRequest(request) {
		hint = safety.NormalizeUserSuppliedPath(hint)
		if !filepath.IsAbs(hint) {
			hint = filepath.Join(root, hint)
		}
		add(hint)
	}
	return candidates
}

func pythonTestRootCandidates(path string) []string {
	path = filepath.Clean(path)
	candidates := make([]string, 0, 4)
	for {
		if isBroadTestDiscoveryRoot(path) {
			return candidates
		}
		candidates = append(candidates, path)
		if exists(filepath.Join(path, "pytest.ini")) ||
			exists(filepath.Join(path, "pyproject.toml")) ||
			exists(filepath.Join(path, "setup.cfg")) ||
			exists(filepath.Join(path, "go.mod")) ||
			exists(filepath.Join(path, ".git")) {
			return candidates
		}
		parent := filepath.Dir(path)
		if parent == path {
			return candidates
		}
		path = parent
	}
}

func pathHintsFromRequest(request string) []string {
	fields := strings.Fields(request)
	hints := make([]string, 0, 4)
	for _, field := range fields {
		candidate := strings.Trim(field, ".,:;()[]{}\"'")
		if candidate == "" || strings.HasPrefix(strings.ToLower(candidate), "http://") || strings.HasPrefix(strings.ToLower(candidate), "https://") {
			continue
		}
		if strings.Contains(candidate, "/") || strings.Contains(filepath.Base(candidate), ".") {
			hints = append(hints, candidate)
		}
		if len(hints) >= 6 {
			break
		}
	}
	return hints
}

func isBroadTestDiscoveryRoot(path string) bool {
	clean := filepath.Clean(path)
	if clean == "." {
		return false
	}
	broad := map[string]bool{
		string(filepath.Separator):      true,
		filepath.Clean("/Applications"): true,
		filepath.Clean("/System"):       true,
		filepath.Clean("/Library"):      true,
		filepath.Clean("/Users"):        true,
	}
	if home, err := os.UserHomeDir(); err == nil {
		broad[filepath.Clean(home)] = true
	}
	return broad[clean]
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if b.limit <= 0 {
		return len(data), nil
	}
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(data), nil
	}
	if len(data) > remaining {
		b.buffer.Write(data[:remaining])
		b.truncated = true
		return len(data), nil
	}
	return b.buffer.Write(data)
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}

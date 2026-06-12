package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"yemaka/internal/internet"
	"yemaka/internal/safety"
)

const (
	ProtocolVersion = "yemaka.extension.v1"
	runsFile        = "extension_runs.jsonl"
	defaultMaxBytes = int64(1000000)
)

var runnerMu sync.Mutex

type RunOptions struct {
	Input             map[string]any    `json:"input"`
	DefaultTimeout    time.Duration     `json:"-"`
	MaxRuntimeSeconds int               `json:"-"`
	MaxOutputBytes    int64             `json:"-"`
	Internet          *internet.Service `json:"-"`
}

type InvocationRequest struct {
	Protocol     string                `json:"protocol"`
	RunID        string                `json:"run_id"`
	Extension    string                `json:"extension"`
	Version      string                `json:"version"`
	Input        map[string]any        `json:"input"`
	Capabilities ExtensionCapabilities `json:"capabilities,omitempty"`
	CoreResults  []CoreResult          `json:"core_results,omitempty"`
	Timestamp    string                `json:"timestamp"`
}

type InvocationResponse struct {
	Protocol     string         `json:"protocol"`
	OK           bool           `json:"ok"`
	Output       map[string]any `json:"output"`
	Error        string         `json:"error,omitempty"`
	Logs         string         `json:"logs,omitempty"`
	CoreRequests []CoreRequest  `json:"core_requests,omitempty"`
}

type RunResult struct {
	RunID       string         `json:"runId"`
	Extension   string         `json:"extension"`
	Version     string         `json:"version"`
	Status      string         `json:"status"`
	Output      map[string]any `json:"output,omitempty"`
	CoreResults []CoreResult   `json:"coreResults,omitempty"`
	Error       string         `json:"error,omitempty"`
	Logs        string         `json:"logs,omitempty"`
	ExitCode    int            `json:"exitCode"`
	TimedOut    bool           `json:"timedOut"`
	Truncated   bool           `json:"truncated"`
	DurationMS  int64          `json:"durationMs"`
	StartedAt   string         `json:"startedAt"`
	CompletedAt string         `json:"completedAt"`
}

func (s *Store) Run(ctx context.Context, name string, options RunOptions) (RunResult, error) {
	runnerMu.Lock()
	defer runnerMu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return RunResult{}, fmt.Errorf("extension name is required")
	}
	registry, err := s.discover()
	if err != nil {
		return RunResult{}, err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return RunResult{}, fmt.Errorf("extension not found: %s", name)
	}
	if !entry.Valid {
		return RunResult{}, fmt.Errorf("extension is invalid: %s", entry.ValidationError)
	}
	if !entry.Registered {
		return RunResult{}, fmt.Errorf("extension must be tested and registered before it can run")
	}
	if !entry.Enabled {
		return RunResult{}, fmt.Errorf("extension is disabled: %s", name)
	}
	if !entry.Runnable {
		reason := strings.TrimSpace(entry.RunBlockedReason)
		if reason == "" {
			reason = "extension is not runnable"
		}
		return RunResult{}, fmt.Errorf("extension cannot run: %s", reason)
	}
	manifest, err := LoadManifest(entry.ManifestPath)
	if err != nil {
		return RunResult{}, err
	}
	if err := ValidateManifestWithPolicyMode(manifest, s.PolicyMode); err != nil {
		return RunResult{}, err
	}
	if err := validateRunnableManifestWithPolicyMode(manifest, s.PolicyMode); err != nil {
		return RunResult{}, err
	}
	if _, err := s.inspectPackageDir(entry.Dir, manifest, packageInspectionOptions{RequireEntrypoint: true}); err != nil {
		return RunResult{}, fmt.Errorf("extension package hardening check failed: %w", err)
	}
	if options.Input == nil {
		options.Input = map[string]any{}
	}
	if err := ValidateObjectAgainstSchema("input", manifest.InputSchema, options.Input); err != nil {
		return RunResult{}, err
	}

	runID := uuid.NewString()
	startedAt := s.timestamp()
	start := time.Now()
	result := RunResult{
		RunID:     runID,
		Extension: manifest.Name,
		Version:   manifest.Version,
		Status:    "running",
		StartedAt: startedAt,
	}
	timeout := extensionTimeout(manifest, options)
	maxOutput := extensionMaxOutput(manifest, options)

	request := InvocationRequest{
		Protocol:     ProtocolVersion,
		RunID:        runID,
		Extension:    manifest.Name,
		Version:      manifest.Version,
		Input:        options.Input,
		Capabilities: extensionCapabilities(manifest),
		Timestamp:    startedAt,
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command, args, err := resolveEntrypoint(entry.Dir, manifest.Entrypoint)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.CompletedAt = s.timestamp()
		result.DurationMS = time.Since(start).Milliseconds()
		_ = s.recordRun(result)
		return result, err
	}
	response, invoke, err := invokeExtensionProcess(runCtx, entry.Dir, command, args, request, maxOutput)
	applyInvocationResult(&result, invoke, start, s.timestamp())
	if err != nil {
		return s.failRun(result, err)
	}
	result.Logs = strings.TrimSpace(strings.Join(nonEmpty(result.Logs, response.Logs), "\n"))
	if len(response.CoreRequests) > 0 {
		coreResults, err := executeCoreRequests(runCtx, manifest, options, response.CoreRequests)
		result.CoreResults = append(result.CoreResults, coreResults...)
		if err != nil {
			return s.failRun(result, err)
		}
		request.CoreResults = coreResults
		response, invoke, err = invokeExtensionProcess(runCtx, entry.Dir, command, args, request, maxOutput)
		applyInvocationResult(&result, invoke, start, s.timestamp())
		if err != nil {
			return s.failRun(result, err)
		}
		result.Logs = strings.TrimSpace(strings.Join(nonEmpty(result.Logs, response.Logs), "\n"))
		if len(response.CoreRequests) > 0 {
			return s.failRun(result, errors.New("extension requested more than one core broker round"))
		}
	}
	if !response.OK {
		result.Status = "failed"
		result.Error = redactExtensionText(strings.TrimSpace(response.Error))
		if result.Error == "" {
			result.Error = "extension returned ok=false"
		}
		result.Logs = redactExtensionText(result.Logs)
		_ = s.recordRun(result)
		return result, errors.New(result.Error)
	}
	if response.Output == nil {
		response.Output = map[string]any{}
	}
	if err := ValidateObjectAgainstSchema("output", manifest.OutputSchema, response.Output); err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		_ = s.recordRun(result)
		return result, err
	}
	result.Output = response.Output
	result.Status = "completed"
	result.Logs = redactExtensionText(result.Logs)
	_ = s.recordRun(result)
	return result, nil
}

type invocationProcessResult struct {
	Logs      string
	ExitCode  int
	TimedOut  bool
	Truncated bool
}

func invokeExtensionProcess(ctx context.Context, dir string, command string, args []string, request InvocationRequest, maxOutput int64) (InvocationResponse, invocationProcessResult, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return InvocationResponse{}, invocationProcessResult{}, fmt.Errorf("encode extension invocation: %w", err)
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr boundedBuffer
	stdout.limit = maxOutput
	stderr.limit = 32768
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	invoke := invocationProcessResult{
		Logs:      strings.TrimSpace(stderr.String()),
		ExitCode:  processExitCode(runErr),
		TimedOut:  ctx.Err() == context.DeadlineExceeded,
		Truncated: stdout.truncated || stderr.truncated,
	}
	if invoke.TimedOut {
		return InvocationResponse{}, invoke, errors.New("extension timed out")
	}
	if invoke.Truncated {
		return InvocationResponse{}, invoke, errors.New("extension output exceeded max output bytes")
	}
	if runErr != nil {
		message := runErr.Error()
		if invoke.Logs != "" {
			message += ": " + invoke.Logs
		}
		return InvocationResponse{}, invoke, errors.New(message)
	}

	var response InvocationResponse
	if err := json.Unmarshal([]byte(stdout.String()), &response); err != nil {
		return InvocationResponse{}, invoke, fmt.Errorf("decode extension response: %w", err)
	}
	if response.Protocol != ProtocolVersion {
		return InvocationResponse{}, invoke, errors.New("extension response protocol mismatch")
	}
	return response, invoke, nil
}

func applyInvocationResult(result *RunResult, invoke invocationProcessResult, start time.Time, completedAt string) {
	result.DurationMS = time.Since(start).Milliseconds()
	result.ExitCode = invoke.ExitCode
	result.TimedOut = invoke.TimedOut
	result.Truncated = invoke.Truncated
	result.CompletedAt = completedAt
	result.Logs = strings.TrimSpace(strings.Join(nonEmpty(result.Logs, invoke.Logs), "\n"))
}

func (s *Store) failRun(result RunResult, err error) (RunResult, error) {
	result.Status = "failed"
	if result.TimedOut {
		result.Status = "timeout"
	}
	result.Error = redactExtensionText(err.Error())
	result.Logs = redactExtensionText(result.Logs)
	if result.CompletedAt == "" {
		result.CompletedAt = s.timestamp()
	}
	_ = s.recordRun(result)
	return result, errors.New(result.Error)
}

func validateRunnableManifest(manifest Manifest) error {
	return validateRunnableManifestWithPolicyMode(manifest, safety.PolicyModeSafe)
}

func validateRunnableManifestWithPolicyMode(manifest Manifest, policyMode string) error {
	if manifest.Type != TypeTool {
		return fmt.Errorf("only tool extensions are runnable in this milestone")
	}
	if manifest.Permissions.Network.Enabled {
		if !usesCoreNetworkBroker(manifest) {
			return fmt.Errorf("network-enabled extension execution requires metadata.network_mode=core_broker")
		}
	}
	if manifest.Permissions.Shell {
		return fmt.Errorf("shell-enabled extension execution is blocked")
	}
	if manifest.Permissions.Secrets {
		return fmt.Errorf("secret-enabled extension execution requires the later secrets milestone")
	}
	if manifest.Permissions.Filesystem.Write {
		return fmt.Errorf("filesystem write extension execution requires a later workspace policy milestone")
	}
	return nil
}

func resolveEntrypoint(dir string, entry Entrypoint) (string, []string, error) {
	command := strings.TrimSpace(entry.Command)
	if command == "" {
		return "", nil, fmt.Errorf("entrypoint command is required")
	}
	if strings.ContainsAny(command, "|;&`") {
		return "", nil, fmt.Errorf("entrypoint command contains blocked shell syntax")
	}
	for _, arg := range entry.Args {
		if strings.ContainsAny(arg, "|;&`") || strings.Contains(arg, "$(") {
			return "", nil, fmt.Errorf("entrypoint args contain blocked shell syntax")
		}
	}
	if strings.Contains(command, string(os.PathSeparator)) || strings.HasPrefix(command, ".") {
		if filepath.IsAbs(command) {
			return "", nil, fmt.Errorf("entrypoint command must be relative")
		}
		clean := filepath.Clean(command)
		if clean == "." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
			return "", nil, fmt.Errorf("entrypoint command escapes extension directory")
		}
		path := filepath.Join(dir, clean)
		if err := ensureChild(dir, path); err != nil {
			return "", nil, err
		}
		return path, append([]string{}, entry.Args...), nil
	}
	if !allowedInterpreter(command) {
		return "", nil, fmt.Errorf("entrypoint command must be a relative executable or approved interpreter")
	}
	args := append([]string{}, entry.Args...)
	for _, arg := range args {
		if maybePathArg(arg) {
			if filepath.IsAbs(arg) {
				return "", nil, fmt.Errorf("entrypoint args must be relative")
			}
			clean := filepath.Clean(arg)
			if clean == "." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
				return "", nil, fmt.Errorf("entrypoint arg escapes extension directory")
			}
		}
	}
	return command, args, nil
}

func extensionTimeout(manifest Manifest, options RunOptions) time.Duration {
	timeout := options.DefaultTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if manifest.Safety.MaxRuntimeSeconds > 0 {
		timeout = time.Duration(manifest.Safety.MaxRuntimeSeconds) * time.Second
	}
	if options.MaxRuntimeSeconds > 0 {
		max := time.Duration(options.MaxRuntimeSeconds) * time.Second
		if timeout > max {
			timeout = max
		}
	}
	return timeout
}

func extensionMaxOutput(manifest Manifest, options RunOptions) int64 {
	maxOutput := options.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = defaultMaxBytes
	}
	if manifest.Safety.MaxResponseBytes > 0 && manifest.Safety.MaxResponseBytes < maxOutput {
		maxOutput = manifest.Safety.MaxResponseBytes
	}
	return maxOutput
}

func (s *Store) recordRun(result RunResult) error {
	if strings.TrimSpace(s.AuditPath) != "" {
		_ = s.audit("run", result.Extension, result.Status, result.Error)
	}
	result = redactedRunResult(result)
	path := filepath.Join(filepath.Dir(s.AuditPath), runsFile)
	if strings.TrimSpace(s.AuditPath) == "" {
		path = filepath.Join(filepath.Dir(s.GeneratedDir), runsFile)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

type boundedBuffer struct {
	limit     int64
	buffer    bytes.Buffer
	truncated bool
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	if b.limit <= 0 {
		return b.buffer.Write(data)
	}
	remaining := b.limit - int64(b.buffer.Len())
	if remaining <= 0 {
		b.truncated = true
		return len(data), nil
	}
	if int64(len(data)) > remaining {
		b.truncated = true
		_, _ = b.buffer.Write(data[:remaining])
		return len(data), nil
	}
	return b.buffer.Write(data)
}

func (b *boundedBuffer) String() string {
	return b.buffer.String()
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func allowedInterpreter(command string) bool {
	switch command {
	case "python3", "python", "node":
		return true
	default:
		return false
	}
}

func maybePathArg(arg string) bool {
	return strings.Contains(arg, string(os.PathSeparator)) ||
		strings.HasPrefix(arg, ".") ||
		strings.HasSuffix(arg, ".py") ||
		strings.HasSuffix(arg, ".js")
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}

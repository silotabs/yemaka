package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const shellHelperMaxRequestBytes = 65536

type ShellHelperRequest struct {
	Args                     []string `json:"args"`
	WorkspaceRoot            string   `json:"workspace_root"`
	TimeoutMillis            int64    `json:"timeout_millis"`
	MaxOutputBytes           int      `json:"max_output_bytes"`
	RequireConfirmationRisky bool     `json:"require_confirmation_risky"`
	FullAccess               bool     `json:"full_access"`
}

type ShellHelperResponse struct {
	Analysis CommandAnalysis `json:"analysis"`
	Result   CommandResult   `json:"result"`
	Error    string          `json:"error,omitempty"`
}

func DefaultShellHelperPath() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return path
}

func RunCommandWithHelper(ctx context.Context, analysis CommandAnalysis, policy CommandPolicy) (CommandResult, error) {
	if !analysis.Allowed {
		return CommandResult{Command: analysis.Display, Args: analysis.Args, ExitCode: -1}, errors.New(analysis.Reason)
	}
	helperPath := strings.TrimSpace(policy.HelperPath)
	if helperPath == "" {
		helperPath = DefaultShellHelperPath()
	}
	if helperPath == "" {
		return CommandResult{}, fmt.Errorf("shell helper executable path is unavailable")
	}
	request := ShellHelperRequest{
		Args:                     append([]string{}, analysis.Args...),
		WorkspaceRoot:            policy.WorkspaceRoot,
		TimeoutMillis:            policy.Timeout.Milliseconds(),
		MaxOutputBytes:           policy.MaxOutputBytes,
		RequireConfirmationRisky: policy.RequireConfirmationRisky,
		FullAccess:               policy.FullAccess,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return CommandResult{}, fmt.Errorf("encode shell helper request: %w", err)
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if policy.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, policy.Timeout+5*time.Second)
	} else {
		runCtx, cancel = context.WithTimeout(ctx, 35*time.Second)
	}
	defer cancel()

	cmd := exec.CommandContext(runCtx, helperPath, "helper", "shell")
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr limitedBuffer
	stdout.limit = helperResponseLimit(policy.MaxOutputBytes)
	stderr.limit = 32768
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return CommandResult{}, fmt.Errorf("shell helper failed: %w: %s", err, detail)
		}
		return CommandResult{}, fmt.Errorf("shell helper failed: %w", err)
	}
	var response ShellHelperResponse
	if err := json.Unmarshal([]byte(stdout.String()), &response); err != nil {
		return CommandResult{}, fmt.Errorf("decode shell helper response: %w", err)
	}
	response.Result.Helper = true
	if response.Error != "" {
		return response.Result, errors.New(response.Error)
	}
	return response.Result, nil
}

func RunShellHelper(ctx context.Context, input io.Reader, output io.Writer) error {
	var request ShellHelperRequest
	if err := json.NewDecoder(io.LimitReader(input, shellHelperMaxRequestBytes)).Decode(&request); err != nil {
		return writeShellHelperResponse(output, ShellHelperResponse{Error: "decode shell helper request: " + err.Error()})
	}
	policy := CommandPolicy{
		WorkspaceRoot:            request.WorkspaceRoot,
		Timeout:                  time.Duration(request.TimeoutMillis) * time.Millisecond,
		MaxOutputBytes:           request.MaxOutputBytes,
		RequireConfirmationRisky: request.RequireConfirmationRisky,
		FullAccess:               request.FullAccess,
	}
	analysis := AnalyzeArgs(request.Args, policy)
	response := ShellHelperResponse{Analysis: analysis}
	if !analysis.Allowed {
		response.Result = CommandResult{Command: analysis.Display, Args: analysis.Args, ExitCode: -1, Helper: true}
		response.Error = analysis.Reason
		return writeShellHelperResponse(output, response)
	}
	result, err := runCommandDirect(ctx, analysis, policy)
	result.Helper = true
	response.Result = result
	if err != nil {
		response.Error = err.Error()
	}
	return writeShellHelperResponse(output, response)
}

func writeShellHelperResponse(output io.Writer, response ShellHelperResponse) error {
	encoder := json.NewEncoder(output)
	if err := encoder.Encode(response); err != nil {
		return fmt.Errorf("encode shell helper response: %w", err)
	}
	return nil
}

func helperResponseLimit(maxOutput int) int {
	if maxOutput <= 0 {
		maxOutput = 100000
	}
	return maxOutput*2 + 65536
}

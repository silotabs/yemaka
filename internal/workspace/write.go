package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yemaka/internal/safety"
)

type ChangePlan struct {
	Path        string
	Before      string
	After       string
	Diff        string
	IsNew       bool
	SnapshotID  string
	Changed     bool
	TargetBytes int
}

type WriteOptions struct {
	SnapshotsRoot       string
	SnapshotBeforeWrite bool
	FollowSymlinks      bool
	IncludeHidden       bool
	MaxEditFileBytes    int64
}

func PlanWrite(ctx context.Context, root string, path string, content string, options WriteOptions) (ChangePlan, error) {
	select {
	case <-ctx.Done():
		return ChangePlan{}, ctx.Err()
	default:
	}

	abs, rel, exists, err := safety.ResolveWritePath(safety.PathPolicy{
		WorkspaceRoot:  root,
		FollowSymlinks: options.FollowSymlinks,
		IncludeHidden:  options.IncludeHidden,
	}, path)
	if err != nil {
		return ChangePlan{}, err
	}

	var before string
	if exists {
		info, err := os.Stat(abs)
		if err != nil {
			return ChangePlan{}, fmt.Errorf("stat existing file: %w", err)
		}
		if options.MaxEditFileBytes > 0 && info.Size() > options.MaxEditFileBytes {
			return ChangePlan{}, fmt.Errorf("file exceeds max edit size: %s", rel)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return ChangePlan{}, fmt.Errorf("read existing file: %w", err)
		}
		if strings.ContainsRune(string(data), '\x00') {
			return ChangePlan{}, fmt.Errorf("binary file is blocked: %s", rel)
		}
		before = string(data)
	}

	return ChangePlan{
		Path:        rel,
		Before:      before,
		After:       content,
		Diff:        safety.UnifiedDiff(rel, before, content),
		IsNew:       !exists,
		Changed:     before != content,
		TargetBytes: len(content),
	}, nil
}

func ApplyWrite(ctx context.Context, root string, plan ChangePlan, options WriteOptions) (ChangePlan, error) {
	if !plan.Changed {
		return plan, nil
	}
	abs, rel, _, err := safety.ResolveWritePath(safety.PathPolicy{
		WorkspaceRoot:  root,
		FollowSymlinks: options.FollowSymlinks,
		IncludeHidden:  options.IncludeHidden,
	}, plan.Path)
	if err != nil {
		return ChangePlan{}, err
	}
	if rel != plan.Path {
		return ChangePlan{}, fmt.Errorf("write path changed during apply: %s", rel)
	}

	if options.SnapshotBeforeWrite {
		manager := safety.NewSnapshotManager(options.SnapshotsRoot)
		manifest, err := manager.Create(ctx, root, []string{plan.Path}, "before write_file", safety.PathPolicy{
			WorkspaceRoot:  root,
			FollowSymlinks: options.FollowSymlinks,
			IncludeHidden:  options.IncludeHidden,
		})
		if err != nil {
			return ChangePlan{}, err
		}
		plan.SnapshotID = manifest.ID
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return ChangePlan{}, fmt.Errorf("create file directory: %w", err)
	}
	if err := os.WriteFile(abs, []byte(plan.After), 0o644); err != nil {
		return ChangePlan{}, fmt.Errorf("write file: %w", err)
	}
	return plan, nil
}

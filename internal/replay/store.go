package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const traceFileExt = ".json"

var (
	secretAssignmentPattern = regexp.MustCompile(`(?i)\b(api[_-]?key|authorization|bearer|password|secret|token)\b\s*[:=]\s*([^,\s;"']+)`)
	bearerTokenPattern      = regexp.MustCompile(`(?i)\bBearer\s+([A-Za-z0-9._~+/=-]{8,})`)
	secretTokenPattern      = regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{12,}|gh[pousr]_[A-Za-z0-9_]{12,}|AKIA[0-9A-Z]{12,})\b`)
)

type Store struct {
	Root string
}

type TraceFile struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Filename string `json:"filename"`
}

func NewStore(root string) Store {
	return Store{Root: root}
}

func (s Store) Save(trace Trace) (TraceFile, error) {
	root, err := s.safeRoot()
	if err != nil {
		return TraceFile{}, err
	}
	trace = redactTrace(trace.Normalize())
	if trace.ID == "" {
		return TraceFile{}, errors.New("replay trace id is required")
	}
	filename, err := traceFilename(trace.ID)
	if err != nil {
		return TraceFile{}, err
	}
	path, err := safeJoin(root, filename)
	if err != nil {
		return TraceFile{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return TraceFile{}, fmt.Errorf("create replay store: %w", err)
	}
	data, err := stableTraceJSON(trace)
	if err != nil {
		return TraceFile{}, err
	}
	tmp, err := os.CreateTemp(root, "."+filename+".tmp-*")
	if err != nil {
		return TraceFile{}, fmt.Errorf("create replay trace temp file: %w", err)
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return TraceFile{}, fmt.Errorf("write replay trace: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return TraceFile{}, fmt.Errorf("close replay trace: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return TraceFile{}, fmt.Errorf("save replay trace: %w", err)
	}
	ok = true
	return TraceFile{ID: trace.ID, Path: path, Filename: filename}, nil
}

func (s Store) Load(id string) (Trace, error) {
	root, err := s.safeRoot()
	if err != nil {
		return Trace{}, err
	}
	filename, err := traceFilename(id)
	if err != nil {
		return Trace{}, err
	}
	path, err := safeJoin(root, filename)
	if err != nil {
		return Trace{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Trace{}, fmt.Errorf("load replay trace: %w", err)
	}
	var trace Trace
	if err := json.Unmarshal(data, &trace); err != nil {
		return Trace{}, fmt.Errorf("decode replay trace: %w", err)
	}
	return redactTrace(trace.Normalize()), nil
}

func (s Store) List() ([]TraceFile, error) {
	root, err := s.safeRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []TraceFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list replay traces: %w", err)
	}
	files := make([]TraceFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), traceFileExt) {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), traceFileExt)
		if _, err := traceFilename(id); err != nil {
			continue
		}
		path, err := safeJoin(root, entry.Name())
		if err != nil {
			continue
		}
		file := TraceFile{ID: id, Path: path, Filename: entry.Name()}
		if data, err := os.ReadFile(path); err == nil {
			var trace Trace
			if err := json.Unmarshal(data, &trace); err == nil {
				if trace.ID = strings.TrimSpace(trace.ID); trace.ID != "" {
					file.ID = redactSecretText(trace.ID)
				}
			}
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ID < files[j].ID
	})
	return files, nil
}

func (s Store) safeRoot() (string, error) {
	root := strings.TrimSpace(s.Root)
	if root == "" {
		return "", errors.New("replay store root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve replay store root: %w", err)
	}
	return filepath.Clean(root), nil
}

func traceFilename(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("replay trace id is required")
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return "", fmt.Errorf("replay trace id %q is not a safe filename", id)
	}
	var b strings.Builder
	lastDash := false
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	name := strings.Trim(b.String(), "-.")
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("replay trace id %q is not a safe filename", id)
	}
	return name + traceFileExt, nil
}

func safeJoin(root string, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("replay trace filename %q is not safe", name)
	}
	path := filepath.Join(root, name)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("resolve replay trace path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", fmt.Errorf("replay trace path escapes store root")
	}
	return path, nil
}

func stableTraceJSON(trace Trace) ([]byte, error) {
	data, err := json.MarshalIndent(trace.Normalize(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode replay trace: %w", err)
	}
	return append(data, '\n'), nil
}

func redactTrace(trace Trace) Trace {
	data, err := json.Marshal(trace)
	if err != nil {
		return trace
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return trace
	}
	redactJSONValue("", value)
	redacted, err := json.Marshal(value)
	if err != nil {
		return trace
	}
	var out Trace
	if err := json.Unmarshal(redacted, &out); err != nil {
		return trace
	}
	return out.Normalize()
}

func redactJSONValue(key string, value any) {
	switch v := value.(type) {
	case map[string]any:
		if attrKey, ok := v["key"].(string); ok && sensitiveKey(attrKey) {
			if _, ok := v["value"].(string); ok {
				v["value"] = "[redacted]"
			}
		}
		for childKey, childValue := range v {
			if strings.EqualFold(childKey, "value") && sensitiveKey(key) {
				v[childKey] = "[redacted]"
				continue
			}
			nextKey := key
			if strings.EqualFold(childKey, "key") {
				if keyValue, ok := childValue.(string); ok {
					nextKey = keyValue
				}
			}
			redactJSONValue(nextKey, childValue)
		}
	case []any:
		for _, childValue := range v {
			redactJSONValue(key, childValue)
		}
	case string:
		// Strings are replaced by the parent container during map traversal.
	}
	if m, ok := value.(map[string]any); ok {
		for childKey, childValue := range m {
			text, ok := childValue.(string)
			if !ok {
				continue
			}
			if sensitiveKey(childKey) {
				m[childKey] = "[redacted]"
				continue
			}
			m[childKey] = redactSecretText(text)
		}
	}
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{"api_key", "apikey", "authorization", "bearer", "password", "secret", "token"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func redactSecretText(value string) string {
	value = bearerTokenPattern.ReplaceAllString(value, "Bearer [redacted]")
	value = secretAssignmentPattern.ReplaceAllString(value, "$1=[redacted]")
	value = secretTokenPattern.ReplaceAllString(value, "[redacted]")
	return value
}

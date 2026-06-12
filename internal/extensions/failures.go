package extensions

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FailureTrend struct {
	Name          string   `json:"name"`
	Failures      int      `json:"failures"`
	LastStatus    string   `json:"lastStatus"`
	LastError     string   `json:"lastError,omitempty"`
	LastSeenAt    string   `json:"lastSeenAt"`
	SuggestReview bool     `json:"suggestReview"`
	Sources       []string `json:"sources"`
}

func (s *Store) Failures(limit int) ([]FailureTrend, error) {
	if limit <= 0 {
		limit = 20
	}
	counts := map[string]*FailureTrend{}
	if err := s.collectRunFailures(counts); err != nil {
		return nil, err
	}
	if err := s.collectTestFailures(counts); err != nil {
		return nil, err
	}
	trends := make([]FailureTrend, 0, len(counts))
	for _, trend := range counts {
		trend.SuggestReview = trend.Failures >= 2
		trends = append(trends, *trend)
	}
	sort.Slice(trends, func(i int, j int) bool {
		if trends[i].Failures == trends[j].Failures {
			return trends[i].LastSeenAt > trends[j].LastSeenAt
		}
		return trends[i].Failures > trends[j].Failures
	})
	if len(trends) > limit {
		trends = trends[:limit]
	}
	return trends, nil
}

func (s *Store) collectRunFailures(counts map[string]*FailureTrend) error {
	return readJSONLines(s.logPath(runsFile), func(line []byte) error {
		var run RunResult
		if err := json.Unmarshal(line, &run); err != nil {
			return nil
		}
		status := strings.ToLower(strings.TrimSpace(run.Status))
		if status == "" || status == "completed" {
			return nil
		}
		trend := trendFor(counts, run.Extension)
		trend.Failures++
		trend.LastStatus = run.Status
		trend.LastError = run.Error
		trend.LastSeenAt = run.CompletedAt
		trend.Sources = appendSource(trend.Sources, runsFile)
		return nil
	})
}

func (s *Store) collectTestFailures(counts map[string]*FailureTrend) error {
	return readJSONLines(s.logPath(testsFile), func(line []byte) error {
		var test TestResult
		if err := json.Unmarshal(line, &test); err != nil {
			return nil
		}
		status := strings.ToLower(strings.TrimSpace(test.Status))
		if status == "" || status == "passed" {
			return nil
		}
		trend := trendFor(counts, test.Name)
		trend.Failures++
		trend.LastStatus = test.Status
		trend.LastError = compactLog(test.Output, 240)
		trend.LastSeenAt = test.CompletedAt
		trend.Sources = appendSource(trend.Sources, testsFile)
		return nil
	})
}

func trendFor(counts map[string]*FailureTrend, name string) *FailureTrend {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown_extension"
	}
	if counts[name] == nil {
		counts[name] = &FailureTrend{Name: name}
	}
	return counts[name]
}

func (s *Store) logPath(filename string) string {
	if strings.TrimSpace(s.AuditPath) != "" {
		return filepath.Join(filepath.Dir(s.AuditPath), filename)
	}
	return filepath.Join(filepath.Dir(s.GeneratedDir), filename)
}

func readJSONLines(path string, fn func([]byte) error) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 1024*64)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := fn([]byte(line)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func appendSource(sources []string, source string) []string {
	for _, existing := range sources {
		if existing == source {
			return sources
		}
	}
	return append(sources, source)
}

func compactLog(input string, max int) string {
	input = strings.Join(strings.Fields(input), " ")
	if max <= 0 || len(input) <= max {
		return input
	}
	return strings.TrimSpace(input[:max-3]) + "..."
}

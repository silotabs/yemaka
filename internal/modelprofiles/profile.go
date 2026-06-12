package modelprofiles

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	SchemaVersion      = 1
	FileExtension      = ".yaml"
	DefaultTemperature = 0.2
	DefaultNumCtx      = 4096
	MaxSystemChars     = 12000
)

const defaultSystem = "You are Yemaka created by SiloTabs, a local-first assistant. Be concise, careful, and avoid assuming optional cloud, internet, connector, embedding, or background job access."

type Profile struct {
	SchemaVersion int        `yaml:"schema_version" json:"schemaVersion"`
	Name          string     `yaml:"name" json:"name"`
	Description   string     `yaml:"description,omitempty" json:"description,omitempty"`
	BaseModel     string     `yaml:"base_model" json:"baseModel"`
	System        string     `yaml:"system,omitempty" json:"system,omitempty"`
	Parameters    Parameters `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	Metadata      Metadata   `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Tags          []string   `yaml:"tags,omitempty" json:"tags,omitempty"`
}

type Parameters struct {
	Temperature float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	NumCtx      int     `yaml:"num_ctx,omitempty" json:"numCtx,omitempty"`
}

type Metadata struct {
	Purpose     string `yaml:"purpose,omitempty" json:"purpose,omitempty"`
	RefinedFrom string `yaml:"refined_from,omitempty" json:"refinedFrom,omitempty"`
	CreatedBy   string `yaml:"created_by,omitempty" json:"createdBy,omitempty"`
}

type Store struct {
	Dir string
}

type ReadinessReport struct {
	Schema string           `json:"schema" yaml:"schema"`
	Status string           `json:"status" yaml:"status"`
	Score  int              `json:"score" yaml:"score"`
	Max    int              `json:"max" yaml:"max"`
	Checks []ReadinessCheck `json:"checks" yaml:"checks"`
}

type ReadinessCheck struct {
	ID      string `json:"id" yaml:"id"`
	Status  string `json:"status" yaml:"status"`
	Message string `json:"message" yaml:"message"`
	Points  int    `json:"points" yaml:"points"`
	Max     int    `json:"max" yaml:"max"`
}

type ComparisonReport struct {
	Schema              string   `json:"schema" yaml:"schema"`
	BaselineName        string   `json:"baselineName,omitempty" yaml:"baseline_name,omitempty"`
	CandidateName       string   `json:"candidateName" yaml:"candidate_name"`
	HasBaseline         bool     `json:"hasBaseline" yaml:"has_baseline"`
	SameArtifact        bool     `json:"sameArtifact" yaml:"same_artifact"`
	ChangedFields       []string `json:"changedFields,omitempty" yaml:"changed_fields,omitempty"`
	AddedTags           []string `json:"addedTags,omitempty" yaml:"added_tags,omitempty"`
	RemovedTags         []string `json:"removedTags,omitempty" yaml:"removed_tags,omitempty"`
	SystemCharsDelta    int      `json:"systemCharsDelta,omitempty" yaml:"system_chars_delta,omitempty"`
	ReadinessScoreDelta int      `json:"readinessScoreDelta,omitempty" yaml:"readiness_score_delta,omitempty"`
}

type HistoryEvent struct {
	Schema         string `json:"schema" yaml:"schema"`
	Kind           string `json:"kind" yaml:"kind"`
	ProfileName    string `json:"profileName" yaml:"profile_name"`
	Path           string `json:"path,omitempty" yaml:"path,omitempty"`
	ModifiedAt     string `json:"modifiedAt,omitempty" yaml:"modified_at,omitempty"`
	SizeBytes      int64  `json:"sizeBytes,omitempty" yaml:"size_bytes,omitempty"`
	ReadinessScore int    `json:"readinessScore" yaml:"readiness_score"`
	Message        string `json:"message,omitempty" yaml:"message,omitempty"`
}

type namedText struct {
	Name  string
	Value string
}

var (
	baseModelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
	secretPatterns   = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"private key", regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
		{"openai-style api key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`)},
		{"github token", regexp.MustCompile(`\b(?:github_pat_[A-Za-z0-9_]{20,}|gh[pousr]_[A-Za-z0-9_]{20,})\b`)},
		{"gitlab token", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{16,}\b`)},
		{"slack token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{16,}\b`)},
		{"aws access key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
		{"bearer token", regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`)},
		{"secret assignment", regexp.MustCompile(`(?i)\b(api[_-]?key|secret|token|password|passwd|private[_-]?key)\b\s*[:=]\s*["']?[A-Za-z0-9_./+=:-]{8,}`)},
	}
	rawPrivateMarkers = []string{
		"begin raw conversation",
		"end raw conversation",
		"begin conversation transcript",
		"end conversation transcript",
		"raw conversation transcript",
		"conversation transcript:",
		"chat transcript:",
		"raw_private_memory",
		"raw private memory",
		"begin private memory",
		"end private memory",
		"private memory:",
		"memory dump:",
		"raw memory dump",
	}
)

func NewStore(dir string) Store {
	return Store{Dir: dir}
}

func Save(dir string, profile Profile) (string, error) {
	return NewStore(dir).Save(profile)
}

func Load(dir string, name string) (Profile, error) {
	return NewStore(dir).Load(name)
}

func List(dir string) ([]Profile, error) {
	return NewStore(dir).List()
}

func (s Store) Save(profile Profile) (string, error) {
	profile = Normalize(profile)
	if err := Validate(profile); err != nil {
		return "", err
	}

	root, err := cleanProfileDir(s.Dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create model profile directory: %w", err)
	}

	path, err := s.profilePath(profile.Name)
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(profile)
	if err != nil {
		return "", fmt.Errorf("encode model profile: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write model profile: %w", err)
	}
	return path, nil
}

func (s Store) Load(name string) (Profile, error) {
	path, err := s.profilePath(name)
	if err != nil {
		return Profile{}, err
	}
	profile, err := loadProfileFile(path)
	if err != nil {
		return Profile{}, err
	}
	if err := ensureDeterministicPath(path, profile.Name); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (s Store) List() ([]Profile, error) {
	root, err := cleanProfileDir(s.Dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []Profile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read model profile directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	profiles := make([]Profile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != FileExtension {
			continue
		}
		path := filepath.Join(root, entry.Name())
		profile, err := loadProfileFile(path)
		if err != nil {
			return nil, fmt.Errorf("load model profile %s: %w", entry.Name(), err)
		}
		if err := ensureDeterministicPath(path, profile.Name); err != nil {
			return nil, fmt.Errorf("load model profile %s: %w", entry.Name(), err)
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (s Store) CompareCurrent(candidate Profile) (ComparisonReport, error) {
	candidate = Normalize(candidate)
	baseline, err := s.Load(candidate.Name)
	if errors.Is(err, os.ErrNotExist) {
		return Compare(Profile{}, candidate), nil
	}
	if err != nil {
		return ComparisonReport{}, err
	}
	return Compare(baseline, candidate), nil
}

func (s Store) History(name string) ([]HistoryEvent, error) {
	profile, err := s.Load(name)
	if errors.Is(err, os.ErrNotExist) {
		return []HistoryEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	path, err := s.profilePath(profile.Name)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat model profile: %w", err)
	}
	readiness := AssessReadiness(profile)
	return []HistoryEvent{{
		Schema:         "yemaka.model_profile_history.v1",
		Kind:           "saved_profile",
		ProfileName:    profile.Name,
		Path:           path,
		ModifiedAt:     stat.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00"),
		SizeBytes:      stat.Size(),
		ReadinessScore: readiness.Score,
		Message:        "latest local saved artifact",
	}}, nil
}

func Normalize(profile Profile) Profile {
	profile.SchemaVersion = normalizeSchemaVersion(profile.SchemaVersion)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Description = strings.TrimSpace(profile.Description)
	profile.BaseModel = strings.TrimSpace(profile.BaseModel)
	profile.System = strings.TrimSpace(profile.System)
	if profile.System == "" {
		profile.System = defaultSystem
	}
	if profile.Parameters.Temperature == 0 {
		profile.Parameters.Temperature = DefaultTemperature
	}
	if profile.Parameters.NumCtx == 0 {
		profile.Parameters.NumCtx = DefaultNumCtx
	}
	profile.Metadata.Purpose = strings.TrimSpace(profile.Metadata.Purpose)
	profile.Metadata.RefinedFrom = strings.TrimSpace(profile.Metadata.RefinedFrom)
	profile.Metadata.CreatedBy = strings.TrimSpace(profile.Metadata.CreatedBy)
	profile.Tags = normalizeTags(profile.Tags)
	return profile
}

func Validate(profile Profile) error {
	profile = Normalize(profile)
	if profile.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported model profile schema_version %d", profile.SchemaVersion)
	}
	if profile.Name == "" {
		return fmt.Errorf("model profile name is required")
	}
	if _, err := SafeName(profile.Name); err != nil {
		return err
	}
	if profile.BaseModel == "" {
		return fmt.Errorf("model profile base_model is required")
	}
	if !baseModelPattern.MatchString(profile.BaseModel) {
		return fmt.Errorf("model profile base_model must be a local model name without whitespace or Modelfile directives")
	}
	if strings.Contains(profile.System, `"""`) {
		return fmt.Errorf("model profile system cannot contain triple quotes")
	}
	if len(profile.System) > MaxSystemChars {
		return fmt.Errorf("model profile system is too large: %d chars > %d", len(profile.System), MaxSystemChars)
	}
	if profile.Parameters.Temperature < 0 || profile.Parameters.Temperature > 2 {
		return fmt.Errorf("model profile temperature must be between 0 and 2")
	}
	if profile.Parameters.NumCtx < 512 || profile.Parameters.NumCtx > 32768 {
		return fmt.Errorf("model profile num_ctx must be between 512 and 32768")
	}
	if err := validateNoUnsafeText(profileTextFields(profile)); err != nil {
		return err
	}
	return nil
}

func AssessReadiness(profile Profile) ReadinessReport {
	profile = Normalize(profile)
	report := ReadinessReport{
		Schema: "yemaka.model_profile_readiness.v1",
		Status: "ready",
		Max:    100,
	}
	add := func(id string, ok bool, points int, max int, message string) {
		status := "pass"
		if !ok {
			status = "review"
			points = 0
		}
		report.Checks = append(report.Checks, ReadinessCheck{
			ID:      id,
			Status:  status,
			Message: message,
			Points:  points,
			Max:     max,
		})
		report.Score += points
	}

	if err := Validate(profile); err != nil {
		report.Status = "blocked"
		report.Checks = append(report.Checks, ReadinessCheck{
			ID:      "valid_profile",
			Status:  "blocked",
			Message: err.Error(),
			Points:  0,
			Max:     30,
		})
		report.Max = 100
		return report
	}
	report.Checks = append(report.Checks, ReadinessCheck{
		ID:      "valid_profile",
		Status:  "pass",
		Message: "profile validates and renders a local Modelfile artifact",
		Points:  30,
		Max:     30,
	})
	report.Score += 30

	systemLower := strings.ToLower(profile.System)
	add("local_first_constraints", strings.Contains(systemLower, "local"), 15, 15, "system guidance names local-first behavior")
	add("tool_policy_constraints", strings.Contains(systemLower, "tool") && (strings.Contains(systemLower, "policy") || strings.Contains(systemLower, "approval")), 15, 15, "system guidance keeps tools policy-gated")
	add("clarification_behavior", strings.Contains(systemLower, "clarif") || strings.Contains(systemLower, "unclear"), 10, 10, "system guidance covers unclear requests")
	add("reviewed_source", strings.TrimSpace(profile.Metadata.RefinedFrom) != "" || hasTag(profile.Tags, "reviewed"), 10, 10, "profile links to a reviewed source or reviewed tag")
	add("low_resource_defaults", profile.Parameters.NumCtx <= 8192 && profile.Parameters.Temperature <= 0.8, 10, 10, "parameters are friendly to low-resource local models")
	add("product_metadata", strings.TrimSpace(profile.Description) != "" || strings.TrimSpace(profile.Metadata.Purpose) != "", 10, 10, "profile has a description or purpose for product inspection")

	if report.Score < 70 {
		report.Status = "review"
	}
	return report
}

func Compare(baseline Profile, candidate Profile) ComparisonReport {
	candidate = Normalize(candidate)
	report := ComparisonReport{
		Schema:        "yemaka.model_profile_comparison.v1",
		CandidateName: candidate.Name,
	}
	if strings.TrimSpace(baseline.Name) == "" {
		report.ChangedFields = []string{"new_profile"}
		return report
	}
	baseline = Normalize(baseline)
	report.HasBaseline = true
	report.BaselineName = baseline.Name
	addChanged := func(field string, changed bool) {
		if changed {
			report.ChangedFields = append(report.ChangedFields, field)
		}
	}
	addChanged("name", baseline.Name != candidate.Name)
	addChanged("description", baseline.Description != candidate.Description)
	addChanged("base_model", baseline.BaseModel != candidate.BaseModel)
	addChanged("system", baseline.System != candidate.System)
	addChanged("temperature", baseline.Parameters.Temperature != candidate.Parameters.Temperature)
	addChanged("num_ctx", baseline.Parameters.NumCtx != candidate.Parameters.NumCtx)
	addChanged("purpose", baseline.Metadata.Purpose != candidate.Metadata.Purpose)
	addChanged("refined_from", baseline.Metadata.RefinedFrom != candidate.Metadata.RefinedFrom)
	report.AddedTags, report.RemovedTags = tagDelta(baseline.Tags, candidate.Tags)
	if len(report.AddedTags) > 0 || len(report.RemovedTags) > 0 {
		report.ChangedFields = append(report.ChangedFields, "tags")
	}
	report.SystemCharsDelta = len(candidate.System) - len(baseline.System)
	report.ReadinessScoreDelta = AssessReadiness(candidate).Score - AssessReadiness(baseline).Score
	report.SameArtifact = len(report.ChangedFields) == 0
	sort.Strings(report.ChangedFields)
	return report
}

func RenderModelfile(profile Profile) string {
	rendered, err := RenderValidatedModelfile(profile)
	if err != nil {
		return ""
	}
	return rendered
}

func RenderValidatedModelfile(profile Profile) (string, error) {
	profile = Normalize(profile)
	if err := Validate(profile); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("FROM ")
	builder.WriteString(profile.BaseModel)
	builder.WriteString("\n\nSYSTEM \"\"\"\n")
	builder.WriteString(profile.System)
	builder.WriteString("\n\"\"\"\n\nPARAMETER temperature ")
	builder.WriteString(strconv.FormatFloat(profile.Parameters.Temperature, 'f', -1, 64))
	builder.WriteString("\nPARAMETER num_ctx ")
	builder.WriteString(strconv.Itoa(profile.Parameters.NumCtx))
	builder.WriteString("\n")
	return builder.String(), nil
}

func SafeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("model profile name is required")
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if unicode.IsSpace(r) || r == '-' || r == '_' || r == '.' || r == '/' || r == '\\' {
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
			continue
		}
		if builder.Len() > 0 && !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	safe := strings.Trim(builder.String(), "-")
	if safe == "" || safe == "." || safe == ".." {
		return "", fmt.Errorf("model profile name %q does not produce a safe filename", name)
	}
	if len(safe) > 80 {
		safe = strings.Trim(safe[:80], "-")
	}
	if safe == "" {
		return "", fmt.Errorf("model profile name %q does not produce a safe filename", name)
	}
	return safe, nil
}

func FileName(name string) (string, error) {
	safe, err := SafeName(name)
	if err != nil {
		return "", err
	}
	return safe + FileExtension, nil
}

func (s Store) profilePath(name string) (string, error) {
	root, err := cleanProfileDir(s.Dir)
	if err != nil {
		return "", err
	}
	filename, err := FileName(name)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filename)
	if err := ensureChildPath(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func loadProfileFile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read model profile: %w", err)
	}

	var profile Profile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&profile); err != nil {
		return Profile{}, fmt.Errorf("parse model profile: %w", err)
	}

	profile = Normalize(profile)
	if err := Validate(profile); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func cleanProfileDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("model profile directory is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve model profile directory: %w", err)
	}
	return filepath.Clean(abs), nil
}

func ensureDeterministicPath(path string, profileName string) error {
	filename, err := FileName(profileName)
	if err != nil {
		return err
	}
	if filepath.Base(path) != filename {
		return fmt.Errorf("model profile filename %q must be %q", filepath.Base(path), filename)
	}
	return nil
}

func ensureChildPath(root string, child string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve model profile directory: %w", err)
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return fmt.Errorf("resolve model profile path: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, childAbs)
	if err != nil {
		return fmt.Errorf("resolve model profile relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("model profile path escapes profile directory")
	}
	return nil
}

func normalizeSchemaVersion(version int) int {
	if version == 0 {
		return SchemaVersion
	}
	return version
}

func normalizeTags(tags []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, tag)
	}
	return normalized
}

func hasTag(tags []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, tag := range tags {
		if strings.ToLower(strings.TrimSpace(tag)) == want {
			return true
		}
	}
	return false
}

func tagDelta(before []string, after []string) ([]string, []string) {
	before = normalizeTags(before)
	after = normalizeTags(after)
	beforeSet := map[string]string{}
	afterSet := map[string]string{}
	for _, tag := range before {
		beforeSet[strings.ToLower(tag)] = tag
	}
	for _, tag := range after {
		afterSet[strings.ToLower(tag)] = tag
	}
	added := []string{}
	removed := []string{}
	for key, tag := range afterSet {
		if _, ok := beforeSet[key]; !ok {
			added = append(added, tag)
		}
	}
	for key, tag := range beforeSet {
		if _, ok := afterSet[key]; !ok {
			removed = append(removed, tag)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func profileTextFields(profile Profile) []namedText {
	fields := []namedText{
		{Name: "name", Value: profile.Name},
		{Name: "description", Value: profile.Description},
		{Name: "base_model", Value: profile.BaseModel},
		{Name: "system", Value: profile.System},
		{Name: "metadata.purpose", Value: profile.Metadata.Purpose},
		{Name: "metadata.refined_from", Value: profile.Metadata.RefinedFrom},
		{Name: "metadata.created_by", Value: profile.Metadata.CreatedBy},
	}
	for i, tag := range profile.Tags {
		fields = append(fields, namedText{Name: fmt.Sprintf("tags[%d]", i), Value: tag})
	}
	return fields
}

func validateNoUnsafeText(fields []namedText) error {
	for _, field := range fields {
		value := strings.TrimSpace(field.Value)
		if value == "" {
			continue
		}
		lower := strings.ToLower(value)
		for _, marker := range rawPrivateMarkers {
			if strings.Contains(lower, marker) {
				return fmt.Errorf("model profile %s contains raw private content marker %q", field.Name, marker)
			}
		}
		for _, secret := range secretPatterns {
			if secret.pattern.MatchString(value) {
				return fmt.Errorf("model profile %s appears to contain a secret (%s)", field.Name, secret.name)
			}
		}
	}
	return nil
}

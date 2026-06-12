package capabilities

import "strings"

type GapSummary struct {
	Title      string    `json:"title"`
	Detail     string    `json:"detail,omitempty"`
	NextStep   string    `json:"nextStep,omitempty"`
	Action     GapAction `json:"action"`
	Name       string    `json:"name,omitempty"`
	Kind       Kind      `json:"kind,omitempty"`
	State      State     `json:"state,omitempty"`
	Missing    []string  `json:"missing,omitempty"`
	MatchCount int       `json:"matchCount,omitempty"`
}

func (d GapDecision) Summary() GapSummary {
	summary := GapSummary{
		Action:     d.Action,
		Name:       strings.TrimSpace(d.Name),
		Kind:       d.Kind,
		State:      d.State,
		Missing:    compactStrings(d.Missing),
		MatchCount: len(d.Matches),
		NextStep:   strings.TrimSpace(d.SuggestedAction),
	}
	name := displayCapabilityName(d)
	switch d.Action {
	case ActionUseExisting:
		summary.Title = "Existing capability is ready"
		summary.Detail = nonEmpty(d.Reason, "Yemaka can use "+name+" through the normal local policy flow.")
	case ActionUsePack:
		summary.Title = "Enabled pack can handle this"
		summary.Detail = nonEmpty(d.Reason, "Yemaka can use "+name+" instead of generating a new capability.")
	case ActionGenerateExtension:
		summary.Title = "Small generated extension can be proposed"
		summary.Detail = nonEmpty(d.Reason, "No ready local capability matches this task.")
	case ActionAskConfig:
		summary.Title = "Capability needs setup"
		summary.Detail = nonEmpty(d.Reason, name+" is not ready yet.")
	case ActionUnsupported:
		summary.Title = "Capability is not supported"
		summary.Detail = nonEmpty(d.Reason, name+" is blocked or unavailable.")
	case ActionAskClarification:
		summary.Title = "Need a clearer capability request"
		summary.Detail = nonEmpty(d.Reason, "Yemaka needs a more specific task before choosing a capability.")
	default:
		summary.Title = "Capability decision available"
		summary.Detail = strings.TrimSpace(d.Reason)
	}
	return summary
}

func (d GapDecision) UserSummary() string {
	summary := d.Summary()
	parts := compactStrings([]string{summary.Title, summary.Detail, summary.NextStep})
	return strings.Join(parts, " ")
}

func displayCapabilityName(d GapDecision) string {
	if strings.TrimSpace(d.Name) != "" {
		return d.Name
	}
	if d.Kind != "" {
		return string(d.Kind)
	}
	if len(d.Missing) > 0 && strings.TrimSpace(d.Missing[0]) != "" {
		return d.Missing[0]
	}
	return "this capability"
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := normalizeKey(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

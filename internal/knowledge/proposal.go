package knowledge

import (
	"fmt"
	"strings"
)

const (
	defaultProposalLimit = 6
	maxProposalLimit     = 12
)

type ProposalInput struct {
	Text        string
	Source      string
	SourceKind  string
	SourceRef   string
	DefaultKind string
	Limit       int
}

type EntityProposal struct {
	Name       string
	Kind       string
	Source     string
	SourceKind string
	SourceRef  string
	Evidence   string
	Reason     string
}

type RelationProposal struct {
	FromName   string
	Relation   string
	ToName     string
	Source     string
	SourceKind string
	SourceRef  string
	Evidence   string
	Reason     string
}

type Proposal struct {
	Source            string
	SourceKind        string
	SourceRef         string
	ReviewRequired    bool
	Message           string
	EntityProposals   []EntityProposal
	RelationProposals []RelationProposal
}

func DraftProposal(input ProposalInput) (Proposal, error) {
	source := normalizeSource(firstNonEmptyString(input.Source, "review_draft"))
	sourceKind := normalizeKnowledgeSourceKind(input.SourceKind)
	sourceRef := sanitizeKnowledgeSourceRef(input.SourceRef)
	defaultKind := normalizeLabel(input.DefaultKind, "note")
	limit := normalizeProposalLimit(input.Limit)
	evidence := sanitizeText(compactWhitespace(input.Text))
	if evidence == "" {
		return Proposal{}, fmt.Errorf("knowledge graph proposal text is required")
	}
	if len([]rune(evidence)) > defaultMaxEvidenceChars {
		evidence = truncateString(evidence, defaultMaxEvidenceChars)
	}
	proposal := Proposal{
		Source:         source,
		SourceKind:     sourceKind,
		SourceRef:      sourceRef,
		ReviewRequired: true,
		Message:        "review draft only; no knowledge graph records were saved",
	}

	for _, line := range proposalLines(input.Text) {
		if len(proposal.EntityProposals) >= limit {
			break
		}
		if entity, ok := parseEntityProposalLine(line, source, sourceKind, sourceRef, defaultKind, evidence); ok {
			proposal.EntityProposals = appendUniqueEntityProposal(proposal.EntityProposals, entity)
		}
	}
	for _, line := range proposalLines(input.Text) {
		if len(proposal.RelationProposals) >= limit {
			break
		}
		if relation, ok := parseRelationProposalLine(line, source, sourceKind, sourceRef, evidence); ok {
			proposal.RelationProposals = appendUniqueRelationProposal(proposal.RelationProposals, relation)
		}
	}
	if len(proposal.EntityProposals) == 0 {
		proposal.EntityProposals = append(proposal.EntityProposals, EntityProposal{
			Name:       proposalFallbackName(input.Text),
			Kind:       defaultKind,
			Source:     source,
			SourceKind: sourceKind,
			SourceRef:  sourceRef,
			Evidence:   evidence,
			Reason:     "created a single review draft from the provided text",
		})
	}
	return proposal, nil
}

func parseEntityProposalLine(line string, source string, sourceKind string, sourceRef string, defaultKind string, fallbackEvidence string) (EntityProposal, bool) {
	prefix, rest, ok := splitProposalPrefix(line)
	if !ok {
		return EntityProposal{}, false
	}
	kind := defaultKind
	switch prefix {
	case "entity":
		kind = defaultKind
	case "concept", "person", "project", "tool", "document", "file", "note":
		kind = prefix
	default:
		return EntityProposal{}, false
	}
	parts := splitProposalFields(rest)
	if len(parts) == 0 {
		return EntityProposal{}, false
	}
	name := compactWhitespace(parts[0])
	if name == "" {
		return EntityProposal{}, false
	}
	evidence := fallbackEvidence
	for _, field := range parts[1:] {
		key, value, ok := splitField(field)
		if !ok {
			continue
		}
		switch key {
		case "kind", "type":
			kind = normalizeLabel(value, kind)
		case "source":
			source = normalizeSource(value)
		case "source_kind", "source kind":
			sourceKind = normalizeKnowledgeSourceKind(value)
		case "source_ref", "source ref", "reference", "ref":
			sourceRef = sanitizeKnowledgeSourceRef(value)
		case "evidence", "why":
			evidence = sanitizeText(compactWhitespace(value))
		}
	}
	return EntityProposal{
		Name:       truncateString(name, 120),
		Kind:       normalizeLabel(kind, defaultKind),
		Source:     normalizeSource(source),
		SourceKind: normalizeKnowledgeSourceKind(sourceKind),
		SourceRef:  sanitizeKnowledgeSourceRef(sourceRef),
		Evidence:   truncateString(evidence, defaultMaxEvidenceChars),
		Reason:     "explicit entity-style line found in reviewed text",
	}, true
}

func parseRelationProposalLine(line string, source string, sourceKind string, sourceRef string, fallbackEvidence string) (RelationProposal, bool) {
	prefix, rest, ok := splitProposalPrefix(line)
	if !ok || (prefix != "relation" && prefix != "relationship" && prefix != "link") {
		return RelationProposal{}, false
	}
	var pieces []string
	var fields []string
	if strings.Contains(rest, "->") {
		pieces = strings.Split(rest, "->")
		if len(pieces) >= 3 {
			tail := splitProposalFields(pieces[2])
			if len(tail) > 0 {
				pieces[2] = tail[0]
				fields = append(fields, tail[1:]...)
			}
		}
	} else {
		pieces = splitProposalFields(rest)
		fields = pieces[3:]
	}
	if len(pieces) < 3 {
		return RelationProposal{}, false
	}
	from := compactWhitespace(pieces[0])
	relation := normalizeLabel(pieces[1], "")
	to := compactWhitespace(pieces[2])
	if from == "" || relation == "" || to == "" {
		return RelationProposal{}, false
	}
	evidence := fallbackEvidence
	for _, field := range fields {
		key, value, ok := splitField(field)
		if !ok {
			continue
		}
		switch key {
		case "source":
			source = normalizeSource(value)
		case "source_kind", "source kind":
			sourceKind = normalizeKnowledgeSourceKind(value)
		case "source_ref", "source ref", "reference", "ref":
			sourceRef = sanitizeKnowledgeSourceRef(value)
		case "evidence", "why":
			evidence = sanitizeText(compactWhitespace(value))
		}
	}
	return RelationProposal{
		FromName:   truncateString(from, 120),
		Relation:   relation,
		ToName:     truncateString(to, 120),
		Source:     normalizeSource(source),
		SourceKind: normalizeKnowledgeSourceKind(sourceKind),
		SourceRef:  sanitizeKnowledgeSourceRef(sourceRef),
		Evidence:   truncateString(evidence, defaultMaxEvidenceChars),
		Reason:     "explicit relationship-style line found in reviewed text",
	}, true
}

func splitProposalPrefix(line string) (string, string, bool) {
	prefix, rest, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	prefix = normalizeLabel(prefix, "")
	rest = strings.TrimSpace(rest)
	if prefix == "" || rest == "" {
		return "", "", false
	}
	return prefix, rest, true
}

func splitProposalFields(input string) []string {
	raw := strings.Split(input, "|")
	fields := make([]string, 0, len(raw))
	for _, part := range raw {
		part = compactWhitespace(part)
		if part != "" {
			fields = append(fields, part)
		}
	}
	return fields
}

func splitField(input string) (string, string, bool) {
	key, value, ok := strings.Cut(input, ":")
	if !ok {
		key, value, ok = strings.Cut(input, "=")
	}
	key = normalizeLabel(key, "")
	value = strings.TrimSpace(value)
	if !ok || key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func proposalLines(input string) []string {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	output := make([]string, 0, len(lines))
	for _, line := range lines {
		line = compactWhitespace(line)
		if line != "" {
			output = append(output, line)
		}
	}
	return output
}

func proposalFallbackName(input string) string {
	for _, line := range proposalLines(input) {
		line = strings.Trim(line, " .,:;")
		if line == "" {
			continue
		}
		if prefix, rest, ok := splitProposalPrefix(line); ok && prefix != "" {
			line = rest
		}
		if sentence, _, ok := strings.Cut(line, "."); ok {
			line = sentence
		}
		line = sanitizeText(compactWhitespace(line))
		if line != "" {
			return truncateString(line, 80)
		}
	}
	return "Reviewed note"
}

func normalizeKnowledgeSourceKind(input string) string {
	kind := normalizeLabel(input, "manual")
	switch kind {
	case "memory", "rag", "document", "workflow", "conversation", "qa_review", "manual":
		return kind
	case "local_document", "local_documents", "indexed_document", "indexed_documents":
		return "document"
	case "chat", "conversation_turn", "conversation_message":
		return "conversation"
	case "domain_pack", "skill", "extension":
		return "workflow"
	case "":
		return "manual"
	default:
		return "manual"
	}
}

func sanitizeKnowledgeSourceRef(input string) string {
	ref := sanitizeText(compactWhitespace(input))
	ref = strings.Trim(ref, " \t\r\n:;,.!?()[]{}\"'`")
	if ref == "" {
		return ""
	}
	ref = truncateString(ref, 180)
	return ref
}

func appendUniqueEntityProposal(items []EntityProposal, entity EntityProposal) []EntityProposal {
	key := normalizeLabel(entity.Kind, "note") + "\x00" + normalizeName(entity.Name) + "\x00" + normalizeSource(entity.Source) + "\x00" + normalizeKnowledgeSourceKind(entity.SourceKind) + "\x00" + sanitizeKnowledgeSourceRef(entity.SourceRef)
	for _, item := range items {
		existing := normalizeLabel(item.Kind, "note") + "\x00" + normalizeName(item.Name) + "\x00" + normalizeSource(item.Source) + "\x00" + normalizeKnowledgeSourceKind(item.SourceKind) + "\x00" + sanitizeKnowledgeSourceRef(item.SourceRef)
		if existing == key {
			return items
		}
	}
	return append(items, entity)
}

func appendUniqueRelationProposal(items []RelationProposal, relation RelationProposal) []RelationProposal {
	key := normalizeName(relation.FromName) + "\x00" + normalizeLabel(relation.Relation, "") + "\x00" + normalizeName(relation.ToName) + "\x00" + normalizeSource(relation.Source) + "\x00" + normalizeKnowledgeSourceKind(relation.SourceKind) + "\x00" + sanitizeKnowledgeSourceRef(relation.SourceRef)
	for _, item := range items {
		existing := normalizeName(item.FromName) + "\x00" + normalizeLabel(item.Relation, "") + "\x00" + normalizeName(item.ToName) + "\x00" + normalizeSource(item.Source) + "\x00" + normalizeKnowledgeSourceKind(item.SourceKind) + "\x00" + sanitizeKnowledgeSourceRef(item.SourceRef)
		if existing == key {
			return items
		}
	}
	return append(items, relation)
}

func normalizeProposalLimit(limit int) int {
	if limit <= 0 {
		return defaultProposalLimit
	}
	if limit > maxProposalLimit {
		return maxProposalLimit
	}
	return limit
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

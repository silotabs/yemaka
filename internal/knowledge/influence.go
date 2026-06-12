package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	defaultInfluenceEntityLimit = 4
	maxInfluenceEntityLimit     = 20
	defaultInfluenceChars       = 1200
	maxInfluenceChars           = 4000
)

type InfluenceOptions struct {
	Limit    int
	MaxChars int
}

type InfluenceContext struct {
	Text        string
	Sources     []string
	EntityCount int
	EdgeCount   int
}

type influenceEdge struct {
	ID       string
	FromName string
	Relation string
	ToName   string
	Evidence string
}

func (s *Store) BuildInfluenceContext(ctx context.Context, query string, options InfluenceOptions) (InfluenceContext, error) {
	if s == nil || s.db == nil {
		return InfluenceContext{}, fmt.Errorf("knowledge graph store is not open")
	}
	limit := normalizeInfluenceLimit(options.Limit)
	maxChars := normalizeInfluenceChars(options.MaxChars)
	entities, err := s.searchApprovedInfluenceEntities(ctx, query, limit)
	if err != nil {
		return InfluenceContext{}, err
	}
	if len(entities) == 0 {
		return InfluenceContext{}, nil
	}
	var out InfluenceContext
	var lines []string
	lines = append(lines, "Approved local knowledge graph context. Background only; not a current source of truth.")
	for _, entity := range entities {
		if out.EntityCount >= limit {
			break
		}
		line := influenceEntityLine(entity)
		if line == "" {
			continue
		}
		if !appendInfluenceLine(&lines, line, maxChars) {
			break
		}
		out.EntityCount++
		out.Sources = appendUnique(out.Sources, "kg:entity:"+entity.ID)
		edges, err := s.approvedInfluenceEdgesForEntity(ctx, entity.ID, 3)
		if err != nil {
			return InfluenceContext{}, err
		}
		for _, edge := range edges {
			edgeLine := influenceEdgeLine(edge)
			if edgeLine == "" {
				continue
			}
			if !appendInfluenceLine(&lines, edgeLine, maxChars) {
				out.Text = truncateString(strings.Join(lines, "\n"), maxChars)
				return out, nil
			}
			out.EdgeCount++
			out.Sources = appendUnique(out.Sources, "kg:edge:"+edge.ID)
		}
	}
	out.Text = truncateString(strings.Join(lines, "\n"), maxChars)
	return out, nil
}

func (s *Store) searchApprovedInfluenceEntities(ctx context.Context, query string, limit int) ([]Entity, error) {
	ftsQuery := buildInfluenceFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.name, e.kind, e.source, e.source_kind, e.source_ref, e.evidence, e.review_status, e.review_note, e.reviewed_by, e.reviewed_at, e.created_at, e.updated_at
		FROM kg_entity_fts
		JOIN kg_entities e ON kg_entity_fts.entity_id = e.id
		WHERE kg_entity_fts MATCH ? AND e.review_status = ?
		ORDER BY bm25(kg_entity_fts), e.updated_at DESC
		LIMIT ?
	`, ftsQuery, reviewStatusApproved, limit)
	if err != nil {
		return nil, fmt.Errorf("search approved knowledge graph influence entities: %w", err)
	}
	return scanEntities(rows)
}

func buildInfluenceFTSQuery(input string) string {
	terms := strings.FieldsFunc(strings.ToLower(input), func(r rune) bool {
		return !isKnowledgeQueryRune(r)
	})
	var output []string
	for _, term := range terms {
		if len(term) < 2 || influenceStopWords[term] {
			continue
		}
		output = append(output, term+"*")
		if len(output) == 6 {
			break
		}
	}
	return strings.Join(output, " ")
}

func isKnowledgeQueryRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

var influenceStopWords = map[string]bool{
	"about":   true,
	"answer":  true,
	"concept": true,
	"define":  true,
	"explain": true,
	"for":     true,
	"from":    true,
	"give":    true,
	"help":    true,
	"how":     true,
	"local":   true,
	"me":      true,
	"my":      true,
	"note":    true,
	"notes":   true,
	"of":      true,
	"the":     true,
	"this":    true,
	"what":    true,
	"with":    true,
}

func scanEntities(rows *sql.Rows) ([]Entity, error) {
	defer rows.Close()
	var output []Entity
	for rows.Next() {
		entity, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		output = append(output, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approved knowledge graph influence entities: %w", err)
	}
	return output, nil
}

func (s *Store) approvedInfluenceEdgesForEntity(ctx context.Context, entityID string, limit int) ([]influenceEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT edge.id, from_entity.name, edge.relation, to_entity.name, edge.evidence
		FROM kg_edges edge
		JOIN kg_entities from_entity ON edge.from_entity_id = from_entity.id
		JOIN kg_entities to_entity ON edge.to_entity_id = to_entity.id
		WHERE (edge.from_entity_id = ? OR edge.to_entity_id = ?)
			AND edge.review_status = ?
			AND from_entity.review_status = ?
			AND to_entity.review_status = ?
		ORDER BY edge.updated_at DESC
		LIMIT ?
	`, strings.TrimSpace(entityID), strings.TrimSpace(entityID), reviewStatusApproved, reviewStatusApproved, reviewStatusApproved, limit)
	if err != nil {
		return nil, fmt.Errorf("list approved knowledge graph influence edges: %w", err)
	}
	defer rows.Close()
	var output []influenceEdge
	for rows.Next() {
		var edge influenceEdge
		if err := rows.Scan(&edge.ID, &edge.FromName, &edge.Relation, &edge.ToName, &edge.Evidence); err != nil {
			return nil, err
		}
		output = append(output, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate approved knowledge graph influence edges: %w", err)
	}
	return output, nil
}

func influenceEntityLine(entity Entity) string {
	name := compactWhitespace(entity.Name)
	if name == "" {
		return ""
	}
	kind := normalizeLabel(entity.Kind, "concept")
	evidence := compactWhitespace(entity.Evidence)
	if evidence != "" {
		return fmt.Sprintf("- %s (%s): %s", name, kind, evidence)
	}
	return fmt.Sprintf("- %s (%s)", name, kind)
}

func influenceEdgeLine(edge influenceEdge) string {
	from := compactWhitespace(edge.FromName)
	to := compactWhitespace(edge.ToName)
	relation := normalizeLabel(edge.Relation, "")
	if from == "" || to == "" || relation == "" {
		return ""
	}
	line := fmt.Sprintf("  - %s --%s--> %s", from, relation, to)
	if evidence := compactWhitespace(edge.Evidence); evidence != "" {
		line += ": " + evidence
	}
	return line
}

func appendInfluenceLine(lines *[]string, line string, maxChars int) bool {
	current := strings.Join(*lines, "\n")
	nextLen := len(current) + len(line) + 1
	if nextLen > maxChars {
		return false
	}
	*lines = append(*lines, line)
	return true
}

func normalizeInfluenceLimit(limit int) int {
	if limit <= 0 {
		return defaultInfluenceEntityLimit
	}
	if limit > maxInfluenceEntityLimit {
		return maxInfluenceEntityLimit
	}
	return limit
}

func normalizeInfluenceChars(maxChars int) int {
	if maxChars <= 0 {
		return defaultInfluenceChars
	}
	if maxChars > maxInfluenceChars {
		return maxInfluenceChars
	}
	return maxChars
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

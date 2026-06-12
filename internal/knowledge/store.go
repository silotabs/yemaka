package knowledge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	_ "modernc.org/sqlite"
)

const (
	defaultEntityLimit      = 12
	maxEntityLimit          = 100
	defaultMaxEvidenceChars = 2000
	defaultReviewLimit      = 25
	maxReviewLimit          = 100
	reviewStatusApproved    = "approved"
	reviewStatusNeedsRepair = "needs_repair"
	reviewStatusPending     = "pending"
	reviewStatusRejected    = "rejected"
)

type Store struct {
	db               *sql.DB
	maxEvidenceChars int
}

type Options struct {
	MaxEvidenceChars int
}

type EntityInput struct {
	Name       string
	Kind       string
	Source     string
	SourceKind string
	SourceRef  string
	Evidence   string
}

type Entity struct {
	ID           string
	Name         string
	Kind         string
	Source       string
	SourceKind   string
	SourceRef    string
	Evidence     string
	ReviewStatus string
	ReviewNote   string
	ReviewedBy   string
	ReviewedAt   string
	CreatedAt    string
	UpdatedAt    string
}

type EdgeInput struct {
	FromEntityID string
	Relation     string
	ToEntityID   string
	Source       string
	SourceKind   string
	SourceRef    string
	Evidence     string
}

type Edge struct {
	ID           string
	FromEntityID string
	Relation     string
	ToEntityID   string
	Source       string
	SourceKind   string
	SourceRef    string
	Evidence     string
	ReviewStatus string
	ReviewNote   string
	ReviewedBy   string
	ReviewedAt   string
	CreatedAt    string
	UpdatedAt    string
}

type SearchOptions struct {
	Limit int
}

type Stats struct {
	Entities int
	Edges    int
}

type RepairResult struct {
	EntitiesScanned int
	EntitiesUpdated int
	EdgesScanned    int
	EdgesUpdated    int
}

type ReviewListOptions struct {
	Status string
	Limit  int
}

type ReviewItem struct {
	ID           string
	Type         string
	Name         string
	Kind         string
	FromEntityID string
	Relation     string
	ToEntityID   string
	Source       string
	SourceKind   string
	SourceRef    string
	Evidence     string
	ReviewStatus string
	ReviewNote   string
	ReviewedBy   string
	ReviewedAt   string
	UpdatedAt    string
}

type ReviewTarget struct {
	Type string
	ID   string
}

type ReviewBatchInput struct {
	Action     string
	Targets    []ReviewTarget
	ReviewedBy string
	Note       string
}

type ReviewBatchResult struct {
	Action          string
	Requested       int
	EntitiesMatched int
	EdgesMatched    int
	EntitiesUpdated int
	EdgesUpdated    int
	Items           []ReviewItem
}

func Open(ctx context.Context, databasePath string) (*Store, error) {
	return OpenWithOptions(ctx, databasePath, Options{})
}

func OpenWithOptions(ctx context.Context, databasePath string, options Options) (*Store, error) {
	if databasePath == "" {
		return nil, fmt.Errorf("knowledge graph database path is empty")
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open knowledge graph sqlite database: %w", err)
	}
	maxEvidenceChars := options.MaxEvidenceChars
	if maxEvidenceChars <= 0 {
		maxEvidenceChars = defaultMaxEvidenceChars
	}
	store := &Store{db: db, maxEvidenceChars: maxEvidenceChars}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("knowledge graph store is not open")
	}
	return s.db.PingContext(ctx)
}

func (s *Store) UpsertEntity(ctx context.Context, input EntityInput) (Entity, error) {
	name := compactWhitespace(input.Name)
	if name == "" {
		return Entity{}, fmt.Errorf("knowledge graph entity name is required")
	}
	kind := normalizeLabel(input.Kind, "concept")
	source := normalizeSource(input.Source)
	sourceKind := normalizeKnowledgeSourceKind(input.SourceKind)
	sourceRef := sanitizeKnowledgeSourceRef(input.SourceRef)
	evidence := s.prepareEvidence(input.Evidence)
	id := deterministicID("kgent", kind, normalizeName(name), source)
	now := timestamp()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kg_entities (id, name, normalized_name, kind, source, source_kind, source_ref, evidence, review_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(kind, normalized_name, source) DO UPDATE SET
			name = excluded.name,
			source_kind = excluded.source_kind,
			source_ref = excluded.source_ref,
			evidence = excluded.evidence,
			review_status = ?,
			review_note = '',
			reviewed_by = '',
			reviewed_at = '',
			updated_at = excluded.updated_at
	`, id, name, normalizeName(name), kind, source, sourceKind, sourceRef, evidence, reviewStatusPending, now, now, reviewStatusPending)
	if err != nil {
		return Entity{}, fmt.Errorf("upsert knowledge graph entity: %w", err)
	}
	entity, err := s.GetEntity(ctx, id)
	if err != nil {
		return Entity{}, err
	}
	if err := s.replaceEntityFTS(ctx, entity); err != nil {
		return Entity{}, err
	}
	return entity, nil
}

func (s *Store) GetEntity(ctx context.Context, id string) (Entity, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, kind, source, source_kind, source_ref, evidence, review_status, review_note, reviewed_by, reviewed_at, created_at, updated_at
		FROM kg_entities
		WHERE id = ?
	`, strings.TrimSpace(id))
	return scanEntity(row)
}

func (s *Store) SearchEntities(ctx context.Context, query string, options SearchOptions) ([]Entity, error) {
	limit := normalizeLimit(options.Limit)
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return s.ListEntities(ctx, limit)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.name, e.kind, e.source, e.source_kind, e.source_ref, e.evidence, e.review_status, e.review_note, e.reviewed_by, e.reviewed_at, e.created_at, e.updated_at
		FROM kg_entity_fts
		JOIN kg_entities e ON kg_entity_fts.entity_id = e.id
		WHERE kg_entity_fts MATCH ?
		ORDER BY bm25(kg_entity_fts), e.updated_at DESC
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search knowledge graph entities: %w", err)
	}
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
		return nil, fmt.Errorf("iterate knowledge graph entities: %w", err)
	}
	return output, nil
}

func (s *Store) ListEntities(ctx context.Context, limit int) ([]Entity, error) {
	limit = normalizeLimit(limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, kind, source, source_kind, source_ref, evidence, review_status, review_note, reviewed_by, reviewed_at, created_at, updated_at
		FROM kg_entities
		ORDER BY updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list knowledge graph entities: %w", err)
	}
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
		return nil, fmt.Errorf("iterate knowledge graph entities: %w", err)
	}
	return output, nil
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var stats Stats
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kg_entities`).Scan(&stats.Entities); err != nil {
		return Stats{}, fmt.Errorf("count knowledge graph entities: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kg_edges`).Scan(&stats.Edges); err != nil {
		return Stats{}, fmt.Errorf("count knowledge graph edges: %w", err)
	}
	return stats, nil
}

func (s *Store) RepairEvidence(ctx context.Context) (RepairResult, error) {
	if s == nil || s.db == nil {
		return RepairResult{}, fmt.Errorf("knowledge graph store is not open")
	}
	var result RepairResult
	entities, err := s.entityEvidenceRecords(ctx)
	if err != nil {
		return RepairResult{}, err
	}
	for _, record := range entities {
		result.EntitiesScanned++
		repaired := s.prepareEvidence(record.evidence)
		if repaired == record.evidence {
			continue
		}
		now := timestamp()
		if _, err := s.db.ExecContext(ctx, `UPDATE kg_entities SET evidence = ?, review_status = ?, review_note = ?, updated_at = ? WHERE id = ?`, repaired, reviewStatusNeedsRepair, "evidence repaired; review still required", now, record.id); err != nil {
			return result, fmt.Errorf("repair knowledge graph entity evidence: %w", err)
		}
		entity, err := s.GetEntity(ctx, record.id)
		if err != nil {
			return result, err
		}
		if err := s.replaceEntityFTS(ctx, entity); err != nil {
			return result, err
		}
		result.EntitiesUpdated++
	}

	edges, err := s.edgeEvidenceRecords(ctx)
	if err != nil {
		return result, err
	}
	for _, record := range edges {
		result.EdgesScanned++
		repaired := s.prepareEvidence(record.evidence)
		if repaired == record.evidence {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE kg_edges SET evidence = ?, review_status = ?, review_note = ?, updated_at = ? WHERE id = ?`, repaired, reviewStatusNeedsRepair, "evidence repaired; review still required", timestamp(), record.id); err != nil {
			return result, fmt.Errorf("repair knowledge graph edge evidence: %w", err)
		}
		result.EdgesUpdated++
	}
	return result, nil
}

func (s *Store) AddEdge(ctx context.Context, input EdgeInput) (Edge, error) {
	fromID := strings.TrimSpace(input.FromEntityID)
	toID := strings.TrimSpace(input.ToEntityID)
	if fromID == "" || toID == "" {
		return Edge{}, fmt.Errorf("knowledge graph edge requires from and to entities")
	}
	relation := normalizeLabel(input.Relation, "")
	if relation == "" {
		return Edge{}, fmt.Errorf("knowledge graph edge relation is required")
	}
	source := normalizeSource(input.Source)
	sourceKind := normalizeKnowledgeSourceKind(input.SourceKind)
	sourceRef := sanitizeKnowledgeSourceRef(input.SourceRef)
	evidence := s.prepareEvidence(input.Evidence)
	id := deterministicID("kgedge", fromID, relation, toID, source)
	now := timestamp()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kg_edges (id, from_entity_id, relation, to_entity_id, source, source_kind, source_ref, evidence, review_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(from_entity_id, relation, to_entity_id, source) DO UPDATE SET
			source_kind = excluded.source_kind,
			source_ref = excluded.source_ref,
			evidence = excluded.evidence,
			review_status = ?,
			review_note = '',
			reviewed_by = '',
			reviewed_at = '',
			updated_at = excluded.updated_at
	`, id, fromID, relation, toID, source, sourceKind, sourceRef, evidence, reviewStatusPending, now, now, reviewStatusPending)
	if err != nil {
		return Edge{}, fmt.Errorf("add knowledge graph edge: %w", err)
	}
	return s.GetEdge(ctx, id)
}

func (s *Store) GetEdge(ctx context.Context, id string) (Edge, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, from_entity_id, relation, to_entity_id, source, source_kind, source_ref, evidence, review_status, review_note, reviewed_by, reviewed_at, created_at, updated_at
		FROM kg_edges
		WHERE id = ?
	`, strings.TrimSpace(id))
	return scanEdge(row)
}

func (s *Store) ListEdgesForEntity(ctx context.Context, entityID string, limit int) ([]Edge, error) {
	limit = normalizeLimit(limit)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, from_entity_id, relation, to_entity_id, source, source_kind, source_ref, evidence, review_status, review_note, reviewed_by, reviewed_at, created_at, updated_at
		FROM kg_edges
		WHERE from_entity_id = ? OR to_entity_id = ?
		ORDER BY updated_at DESC
		LIMIT ?
	`, strings.TrimSpace(entityID), strings.TrimSpace(entityID), limit)
	if err != nil {
		return nil, fmt.Errorf("list knowledge graph edges: %w", err)
	}
	defer rows.Close()
	var output []Edge
	for rows.Next() {
		edge, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		output = append(output, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge graph edges: %w", err)
	}
	return output, nil
}

func (s *Store) ListReviewItems(ctx context.Context, options ReviewListOptions) ([]ReviewItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("knowledge graph store is not open")
	}
	status := normalizeReviewStatusFilter(options.Status)
	limit := normalizeReviewLimit(options.Limit)
	where := ""
	args := []any{}
	if status != "" {
		where = "WHERE review_status = ?"
		args = append(args, status, status)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, 'entity' AS item_type, name, kind, '' AS from_entity_id, '' AS relation, '' AS to_entity_id,
			source, source_kind, source_ref, evidence, review_status, review_note, reviewed_by, reviewed_at, updated_at
		FROM kg_entities
		%s
		UNION ALL
		SELECT id, 'edge' AS item_type, '' AS name, '' AS kind, from_entity_id, relation, to_entity_id,
			source, source_kind, source_ref, evidence, review_status, review_note, reviewed_by, reviewed_at, updated_at
		FROM kg_edges
		%s
		ORDER BY updated_at DESC
		LIMIT ?
	`, where, where), args...)
	if err != nil {
		return nil, fmt.Errorf("list knowledge graph review items: %w", err)
	}
	defer rows.Close()
	var output []ReviewItem
	for rows.Next() {
		item, err := scanReviewItem(rows)
		if err != nil {
			return nil, err
		}
		output = append(output, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge graph review items: %w", err)
	}
	return output, nil
}

func (s *Store) ApplyReviewBatch(ctx context.Context, input ReviewBatchInput) (ReviewBatchResult, error) {
	if s == nil || s.db == nil {
		return ReviewBatchResult{}, fmt.Errorf("knowledge graph store is not open")
	}
	action, status, err := normalizeReviewAction(input.Action)
	if err != nil {
		return ReviewBatchResult{}, err
	}
	targets := normalizeReviewTargets(input.Targets)
	if len(targets) == 0 {
		return ReviewBatchResult{}, fmt.Errorf("knowledge graph review targets are required")
	}
	result := ReviewBatchResult{
		Action:    action,
		Requested: len(targets),
	}
	reviewedBy := normalizeReviewer(input.ReviewedBy)
	note := sanitizeReviewNote(input.Note)
	now := timestamp()
	for _, target := range targets {
		switch target.Type {
		case "entity":
			matched, updated, err := s.applyEntityReview(ctx, target.ID, action, status, reviewedBy, note, now)
			if err != nil {
				return result, err
			}
			result.EntitiesMatched += matched
			result.EntitiesUpdated += updated
		case "edge":
			matched, updated, err := s.applyEdgeReview(ctx, target.ID, action, status, reviewedBy, note, now)
			if err != nil {
				return result, err
			}
			result.EdgesMatched += matched
			result.EdgesUpdated += updated
		}
	}
	items, err := s.reviewItemsForTargets(ctx, targets)
	if err != nil {
		return result, err
	}
	result.Items = items
	return result, nil
}

func (s *Store) migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS kg_entities (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			normalized_name TEXT NOT NULL,
			kind TEXT NOT NULL,
			source TEXT NOT NULL,
			source_kind TEXT NOT NULL DEFAULT 'manual',
			source_ref TEXT NOT NULL DEFAULT '',
			evidence TEXT,
			review_status TEXT NOT NULL DEFAULT 'pending',
			review_note TEXT NOT NULL DEFAULT '',
			reviewed_by TEXT NOT NULL DEFAULT '',
			reviewed_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(kind, normalized_name, source)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_kg_entities_kind_name ON kg_entities(kind, normalized_name);`,
		`CREATE INDEX IF NOT EXISTS idx_kg_entities_updated ON kg_entities(updated_at);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS kg_entity_fts USING fts5(
			name,
			kind,
			source,
			evidence,
			entity_id UNINDEXED
		);`,
		`CREATE TABLE IF NOT EXISTS kg_edges (
			id TEXT PRIMARY KEY,
			from_entity_id TEXT NOT NULL,
			relation TEXT NOT NULL,
			to_entity_id TEXT NOT NULL,
			source TEXT NOT NULL,
			source_kind TEXT NOT NULL DEFAULT 'manual',
			source_ref TEXT NOT NULL DEFAULT '',
			evidence TEXT,
			review_status TEXT NOT NULL DEFAULT 'pending',
			review_note TEXT NOT NULL DEFAULT '',
			reviewed_by TEXT NOT NULL DEFAULT '',
			reviewed_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(from_entity_id, relation, to_entity_id, source),
			FOREIGN KEY(from_entity_id) REFERENCES kg_entities(id) ON DELETE CASCADE,
			FOREIGN KEY(to_entity_id) REFERENCES kg_entities(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_kg_edges_from ON kg_edges(from_entity_id);`,
		`CREATE INDEX IF NOT EXISTS idx_kg_edges_to ON kg_edges(to_entity_id);`,
		`CREATE INDEX IF NOT EXISTS idx_kg_edges_relation ON kg_edges(relation);`,
	}
	for index, statement := range migrations {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply knowledge graph migration %d: %w", index+1, err)
		}
	}
	for _, column := range knowledgeReviewColumns() {
		if err := s.ensureColumn(ctx, "kg_entities", column.name, column.definition); err != nil {
			return err
		}
	}
	for _, column := range knowledgeReviewColumns() {
		if err := s.ensureColumn(ctx, "kg_edges", column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) replaceEntityFTS(ctx context.Context, entity Entity) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM kg_entity_fts WHERE entity_id = ?`, entity.ID); err != nil {
		return fmt.Errorf("delete knowledge graph entity fts: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO kg_entity_fts (name, kind, source, evidence, entity_id)
		VALUES (?, ?, ?, ?, ?)
	`, entity.Name, entity.Kind, entity.Source, entity.Evidence, entity.ID); err != nil {
		return fmt.Errorf("insert knowledge graph entity fts: %w", err)
	}
	return nil
}

func (s *Store) prepareEvidence(input string) string {
	output := sanitizeText(compactWhitespace(input))
	if s.maxEvidenceChars > 0 && len(output) > s.maxEvidenceChars {
		output = truncateString(output, s.maxEvidenceChars)
	}
	return strings.TrimSpace(output)
}

type entityScanner interface {
	Scan(dest ...any) error
}

type evidenceRecord struct {
	id       string
	evidence string
}

func (s *Store) entityEvidenceRecords(ctx context.Context) ([]evidenceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(evidence, '') FROM kg_entities`)
	if err != nil {
		return nil, fmt.Errorf("list knowledge graph entity evidence: %w", err)
	}
	defer rows.Close()
	var records []evidenceRecord
	for rows.Next() {
		var record evidenceRecord
		if err := rows.Scan(&record.id, &record.evidence); err != nil {
			return nil, fmt.Errorf("scan knowledge graph entity evidence: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge graph entity evidence: %w", err)
	}
	return records, nil
}

func (s *Store) edgeEvidenceRecords(ctx context.Context) ([]evidenceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(evidence, '') FROM kg_edges`)
	if err != nil {
		return nil, fmt.Errorf("list knowledge graph edge evidence: %w", err)
	}
	defer rows.Close()
	var records []evidenceRecord
	for rows.Next() {
		var record evidenceRecord
		if err := rows.Scan(&record.id, &record.evidence); err != nil {
			return nil, fmt.Errorf("scan knowledge graph edge evidence: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge graph edge evidence: %w", err)
	}
	return records, nil
}

func (s *Store) applyEntityReview(ctx context.Context, id string, action string, status string, reviewedBy string, note string, reviewedAt string) (int, int, error) {
	entity, err := s.GetEntity(ctx, id)
	if err != nil {
		if errIsNoRows(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	evidence := entity.Evidence
	if action == "repair" {
		evidence = s.prepareEvidence(entity.Evidence)
		status = reviewStatusNeedsRepair
		if note == "" {
			note = "evidence repaired; review still required"
		}
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE kg_entities
		SET evidence = ?, review_status = ?, review_note = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
		WHERE id = ?
	`, evidence, status, note, reviewedBy, reviewedAt, reviewedAt, id)
	if err != nil {
		return 1, 0, fmt.Errorf("review knowledge graph entity: %w", err)
	}
	updated, _ := result.RowsAffected()
	if updated > 0 {
		repaired, err := s.GetEntity(ctx, id)
		if err != nil {
			return 1, int(updated), err
		}
		if err := s.replaceEntityFTS(ctx, repaired); err != nil {
			return 1, int(updated), err
		}
	}
	return 1, int(updated), nil
}

func (s *Store) applyEdgeReview(ctx context.Context, id string, action string, status string, reviewedBy string, note string, reviewedAt string) (int, int, error) {
	edge, err := s.GetEdge(ctx, id)
	if err != nil {
		if errIsNoRows(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	evidence := edge.Evidence
	if action == "repair" {
		evidence = s.prepareEvidence(edge.Evidence)
		status = reviewStatusNeedsRepair
		if note == "" {
			note = "evidence repaired; review still required"
		}
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE kg_edges
		SET evidence = ?, review_status = ?, review_note = ?, reviewed_by = ?, reviewed_at = ?, updated_at = ?
		WHERE id = ?
	`, evidence, status, note, reviewedBy, reviewedAt, reviewedAt, id)
	if err != nil {
		return 1, 0, fmt.Errorf("review knowledge graph edge: %w", err)
	}
	updated, _ := result.RowsAffected()
	return 1, int(updated), nil
}

func (s *Store) reviewItemsForTargets(ctx context.Context, targets []ReviewTarget) ([]ReviewItem, error) {
	items := make([]ReviewItem, 0, len(targets))
	for _, target := range targets {
		var row *sql.Row
		switch target.Type {
		case "entity":
			row = s.db.QueryRowContext(ctx, `
				SELECT id, 'entity' AS item_type, name, kind, '' AS from_entity_id, '' AS relation, '' AS to_entity_id,
					source, source_kind, source_ref, evidence, review_status, review_note, reviewed_by, reviewed_at, updated_at
				FROM kg_entities
				WHERE id = ?
			`, target.ID)
		case "edge":
			row = s.db.QueryRowContext(ctx, `
				SELECT id, 'edge' AS item_type, '' AS name, '' AS kind, from_entity_id, relation, to_entity_id,
					source, source_kind, source_ref, evidence, review_status, review_note, reviewed_by, reviewed_at, updated_at
				FROM kg_edges
				WHERE id = ?
			`, target.ID)
		default:
			continue
		}
		item, err := scanReviewItem(row)
		if err != nil {
			if errIsNoRows(err) {
				continue
			}
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) ensureColumn(ctx context.Context, table string, name string, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return fmt.Errorf("inspect knowledge graph table %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan knowledge graph table %s columns: %w", table, err)
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate knowledge graph table %s columns: %w", table, err)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+definition); err != nil {
		return fmt.Errorf("add knowledge graph column %s.%s: %w", table, name, err)
	}
	return nil
}

func scanEntity(scanner entityScanner) (Entity, error) {
	var entity Entity
	if err := scanner.Scan(&entity.ID, &entity.Name, &entity.Kind, &entity.Source, &entity.SourceKind, &entity.SourceRef, &entity.Evidence, &entity.ReviewStatus, &entity.ReviewNote, &entity.ReviewedBy, &entity.ReviewedAt, &entity.CreatedAt, &entity.UpdatedAt); err != nil {
		return Entity{}, fmt.Errorf("scan knowledge graph entity: %w", err)
	}
	return entity, nil
}

func scanEdge(scanner entityScanner) (Edge, error) {
	var edge Edge
	if err := scanner.Scan(&edge.ID, &edge.FromEntityID, &edge.Relation, &edge.ToEntityID, &edge.Source, &edge.SourceKind, &edge.SourceRef, &edge.Evidence, &edge.ReviewStatus, &edge.ReviewNote, &edge.ReviewedBy, &edge.ReviewedAt, &edge.CreatedAt, &edge.UpdatedAt); err != nil {
		return Edge{}, fmt.Errorf("scan knowledge graph edge: %w", err)
	}
	return edge, nil
}

func scanReviewItem(scanner entityScanner) (ReviewItem, error) {
	var item ReviewItem
	if err := scanner.Scan(&item.ID, &item.Type, &item.Name, &item.Kind, &item.FromEntityID, &item.Relation, &item.ToEntityID, &item.Source, &item.SourceKind, &item.SourceRef, &item.Evidence, &item.ReviewStatus, &item.ReviewNote, &item.ReviewedBy, &item.ReviewedAt, &item.UpdatedAt); err != nil {
		return ReviewItem{}, fmt.Errorf("scan knowledge graph review item: %w", err)
	}
	return item, nil
}

type reviewColumn struct {
	name       string
	definition string
}

func knowledgeReviewColumns() []reviewColumn {
	return []reviewColumn{
		{name: "source_kind", definition: "source_kind TEXT NOT NULL DEFAULT 'manual'"},
		{name: "source_ref", definition: "source_ref TEXT NOT NULL DEFAULT ''"},
		{name: "review_status", definition: "review_status TEXT NOT NULL DEFAULT 'pending'"},
		{name: "review_note", definition: "review_note TEXT NOT NULL DEFAULT ''"},
		{name: "reviewed_by", definition: "reviewed_by TEXT NOT NULL DEFAULT ''"},
		{name: "reviewed_at", definition: "reviewed_at TEXT NOT NULL DEFAULT ''"},
	}
}

func normalizeReviewTargets(targets []ReviewTarget) []ReviewTarget {
	output := make([]ReviewTarget, 0, len(targets))
	seen := map[string]bool{}
	for _, target := range targets {
		typ := normalizeReviewTargetType(target.Type)
		id := strings.TrimSpace(target.ID)
		if typ == "" || id == "" {
			continue
		}
		key := typ + "\x00" + id
		if seen[key] {
			continue
		}
		seen[key] = true
		output = append(output, ReviewTarget{Type: typ, ID: id})
	}
	return output
}

func normalizeReviewAction(input string) (string, string, error) {
	action := normalizeLabel(input, "")
	switch action {
	case "approve", "approved":
		return "approve", reviewStatusApproved, nil
	case "reject", "rejected":
		return "reject", reviewStatusRejected, nil
	case "needs_repair", "mark_needs_repair":
		return "needs_repair", reviewStatusNeedsRepair, nil
	case "repair":
		return "repair", reviewStatusNeedsRepair, nil
	default:
		return "", "", fmt.Errorf("unsupported knowledge graph review action %q", input)
	}
}

func normalizeReviewStatusFilter(input string) string {
	status := normalizeLabel(input, "")
	switch status {
	case reviewStatusApproved, reviewStatusNeedsRepair, reviewStatusPending, reviewStatusRejected:
		return status
	case "":
		return ""
	default:
		return ""
	}
}

func normalizeReviewTargetType(input string) string {
	typ := normalizeLabel(input, "")
	switch typ {
	case "entity", "entities":
		return "entity"
	case "edge", "edges", "link", "links", "relationship", "relationships":
		return "edge"
	default:
		return ""
	}
}

func normalizeReviewer(input string) string {
	reviewer := normalizeLabel(input, "manual_review")
	if reviewer == "" {
		return "manual_review"
	}
	return truncateString(reviewer, 80)
}

func sanitizeReviewNote(input string) string {
	note := sanitizeText(compactWhitespace(input))
	return truncateString(note, 300)
}

func normalizeReviewLimit(limit int) int {
	if limit <= 0 {
		return defaultReviewLimit
	}
	if limit > maxReviewLimit {
		return maxReviewLimit
	}
	return limit
}

func errIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func deterministicID(prefix string, parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + "_" + hex.EncodeToString(hash[:])[:24]
}

func normalizeName(input string) string {
	return strings.ToLower(compactWhitespace(input))
}

func normalizeLabel(input string, fallback string) string {
	input = strings.ToLower(compactWhitespace(input))
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	output := strings.Trim(builder.String(), "_")
	if output == "" {
		return fallback
	}
	return output
}

func normalizeSource(input string) string {
	output := normalizeLabel(input, "manual")
	if output == "" {
		return "manual"
	}
	return output
}

func compactWhitespace(input string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultEntityLimit
	}
	if limit > maxEntityLimit {
		return maxEntityLimit
	}
	return limit
}

func buildFTSQuery(input string) string {
	terms := strings.FieldsFunc(strings.ToLower(input), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var output []string
	for _, term := range terms {
		if len(term) < 2 {
			continue
		}
		output = append(output, term+"*")
		if len(output) == 8 {
			break
		}
	}
	return strings.Join(output, " ")
}

var (
	emailCredentialPattern = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}:[^\s,;]+`)
	bearerTokenPattern     = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{12,}\b`)
	basicAuthURLPattern    = regexp.MustCompile(`(?i)\bhttps?://[^/\s:@]+:[^@\s/]+@`)
	longHexPattern         = regexp.MustCompile(`\b(?:[A-Fa-f0-9]{32,})\b`)
	privateKeyMarker       = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`)
	phoneLikePattern       = regexp.MustCompile(`\b(?:\+?\d[\d ().-]{9,}\d)\b`)
	secretPatterns         = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[_-]?key|token|password|secret)\s*[:=]\s*["']?[^"'\s,}]+`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}\b`),
		regexp.MustCompile(`\b[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{20,}\b`),
	}
	userPathPattern = regexp.MustCompile(`/Users/[^/\s]+/`)
)

func sanitizeText(input string) string {
	output := input
	output = privateKeyMarker.ReplaceAllString(output, "[REDACTED_PRIVATE_KEY]")
	output = basicAuthURLPattern.ReplaceAllString(output, "https://[REDACTED_CREDENTIALS]@")
	output = emailCredentialPattern.ReplaceAllString(output, "[REDACTED_CREDENTIAL_PAIR]")
	output = bearerTokenPattern.ReplaceAllString(output, "Bearer [REDACTED]")
	output = longHexPattern.ReplaceAllString(output, "[REDACTED_TOKEN]")
	output = phoneLikePattern.ReplaceAllString(output, "[REDACTED_PHONE]")
	for _, pattern := range secretPatterns {
		output = pattern.ReplaceAllStringFunc(output, redactSecretAssignment)
	}
	return userPathPattern.ReplaceAllString(output, "~/")
}

func truncateString(input string, maxChars int) string {
	if maxChars <= 0 {
		return input
	}
	runes := []rune(input)
	if len(runes) <= maxChars {
		return input
	}
	return string(runes[:maxChars])
}

func redactSecretAssignment(input string) string {
	if key, _, ok := strings.Cut(input, "="); ok {
		return strings.TrimSpace(key) + "=[REDACTED]"
	}
	if key, _, ok := strings.Cut(input, ":"); ok {
		return strings.TrimSpace(key) + ": [REDACTED]"
	}
	return "[REDACTED]"
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

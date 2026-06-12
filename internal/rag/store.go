package rag

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"yemaka/internal/models"

	_ "modernc.org/sqlite"
)

const (
	defaultDocumentInventoryLimit = 200
	maxDocumentInventoryLimit     = 500

	DocumentStatusIndexed = "indexed"
	DocumentStatusMissing = "missing"
	DocumentStatusStale   = "stale"
	DocumentStatusUnknown = "unknown"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, databasePath string) (*Store, error) {
	if databasePath == "" {
		return nil, fmt.Errorf("rag database path is empty")
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open rag sqlite database: %w", err)
	}
	store := &Store{db: db}
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

func (s *Store) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = DefaultConfig().TopK
	}
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			c.id,
			c.document_id,
			c.path,
			c.content,
			c.created_at,
			bm25(rag_fts) AS rank
		FROM rag_fts
		JOIN rag_chunks c ON rag_fts.chunk_id = c.id
		WHERE rag_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, topK)
	if err != nil {
		return nil, fmt.Errorf("search rag: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(
			&result.ChunkID,
			&result.DocumentID,
			&result.Path,
			&result.Content,
			&result.CreatedAt,
			&result.Rank,
		); err != nil {
			return nil, fmt.Errorf("scan rag result: %w", err)
		}
		result.Source = "fts"
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read rag results: %w", err)
	}
	return results, nil
}

func (s *Store) ListChunks(ctx context.Context, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = DefaultConfig().TopK
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, document_id, path, content
		FROM rag_chunks
		ORDER BY created_at DESC, path ASC, chunk_index ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list rag chunks: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.ChunkID, &result.DocumentID, &result.Path, &result.Content); err != nil {
			return nil, fmt.Errorf("scan rag chunk: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read rag chunks: %w", err)
	}
	return results, nil
}

func (s *Store) ListDocuments(ctx context.Context, limit int) ([]DocumentInventoryItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("rag store is unavailable")
	}
	limit = boundedDocumentInventoryLimit(limit)
	return s.listDocuments(ctx, limit)
}

func (s *Store) listDocuments(ctx context.Context, limit int) ([]DocumentInventoryItem, error) {
	query := `
		SELECT
			d.id,
			COALESCE(d.profile_id, ''),
			d.workspace_root,
			d.path,
			d.content_hash,
			d.size_bytes,
			COALESCE(d.mime_type, ''),
			d.indexed_at,
			d.updated_at,
			COUNT(DISTINCT c.id) AS chunk_count,
			COUNT(e.chunk_id) AS embedding_count
		FROM rag_documents d
		LEFT JOIN rag_chunks c ON c.document_id = d.id
		LEFT JOIN rag_embeddings e ON e.chunk_id = c.id
		GROUP BY d.id
		ORDER BY d.updated_at DESC, d.path ASC
	`
	var args []any
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list rag documents: %w", err)
	}
	defer rows.Close()

	var items []DocumentInventoryItem
	for rows.Next() {
		var item DocumentInventoryItem
		if err := rows.Scan(
			&item.ID,
			&item.ProfileID,
			&item.WorkspaceRoot,
			&item.Path,
			&item.ContentHash,
			&item.SizeBytes,
			&item.MimeType,
			&item.IndexedAt,
			&item.UpdatedAt,
			&item.ChunkCount,
			&item.EmbeddingCount,
		); err != nil {
			return nil, fmt.Errorf("scan rag document inventory: %w", err)
		}
		if err := s.markDocumentInventoryStatus(ctx, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read rag document inventory: %w", err)
	}
	return items, nil
}

func (s *Store) PruneMissingDocuments(ctx context.Context, dryRun bool) (DocumentPruneResult, error) {
	if s == nil || s.db == nil {
		return DocumentPruneResult{}, fmt.Errorf("rag store is unavailable")
	}
	inventory, err := s.listDocuments(ctx, 0)
	if err != nil {
		return DocumentPruneResult{}, err
	}
	result := DocumentPruneResult{DryRun: dryRun}
	for _, item := range inventory {
		if !item.Missing {
			continue
		}
		result.DocumentsMatched++
		result.ChunksMatched += item.ChunkCount
		result.EmbeddingsMatched += item.EmbeddingCount
		result.DocumentIDs = append(result.DocumentIDs, item.ID)
		ftsRows, err := s.countDocumentFTSRows(ctx, item.ID)
		if err != nil {
			return result, err
		}
		result.FTSRowsMatched += ftsRows
	}
	if dryRun || len(result.DocumentIDs) == 0 {
		return result, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin rag prune transaction: %w", err)
	}
	defer tx.Rollback()

	for _, documentID := range result.DocumentIDs {
		ftsRemoved, err := execRowsAffected(ctx, tx, `DELETE FROM rag_fts WHERE document_id = ?`, documentID)
		if err != nil {
			return result, fmt.Errorf("delete missing rag fts rows: %w", err)
		}
		result.FTSRowsRemoved += ftsRemoved

		embeddingsRemoved, err := execRowsAffected(ctx, tx, `
			DELETE FROM rag_embeddings
			WHERE chunk_id IN (SELECT id FROM rag_chunks WHERE document_id = ?)
		`, documentID)
		if err != nil {
			return result, fmt.Errorf("delete missing rag embeddings: %w", err)
		}
		result.EmbeddingsRemoved += embeddingsRemoved

		chunksRemoved, err := execRowsAffected(ctx, tx, `DELETE FROM rag_chunks WHERE document_id = ?`, documentID)
		if err != nil {
			return result, fmt.Errorf("delete missing rag chunks: %w", err)
		}
		result.ChunksRemoved += chunksRemoved

		documentsRemoved, err := execRowsAffected(ctx, tx, `DELETE FROM rag_documents WHERE id = ?`, documentID)
		if err != nil {
			return result, fmt.Errorf("delete missing rag document: %w", err)
		}
		result.DocumentsRemoved += documentsRemoved
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit rag prune transaction: %w", err)
	}
	return result, nil
}

func (s *Store) SearchWithConfig(ctx context.Context, query string, cfg Config, embedder models.EmbeddingRuntime) ([]SearchResult, error) {
	cfg = applyConfigDefaults(cfg)
	candidateLimit := rerankCandidateLimit(cfg)
	ftsResults, err := s.Search(ctx, query, candidateLimit)
	if err != nil {
		return nil, err
	}
	if !cfg.Embeddings.Enabled {
		return rerankResults(query, ftsResults, cfg), nil
	}
	vectorResults, err := s.SearchEmbeddings(ctx, query, cfg, embedder)
	if err != nil {
		if len(ftsResults) > 0 {
			return rerankResults(query, ftsResults, cfg), nil
		}
		return nil, err
	}
	merged := mergeSearchResults(ftsResults, vectorResults, candidateLimit)
	return rerankResults(query, merged, cfg), nil
}

func (s *Store) SearchEmbeddings(ctx context.Context, query string, cfg Config, embedder models.EmbeddingRuntime) ([]SearchResult, error) {
	cfg = applyConfigDefaults(cfg)
	if !cfg.Embeddings.Enabled {
		return nil, nil
	}
	if embedder == nil {
		return nil, fmt.Errorf("embedding runtime is unavailable")
	}
	if cfg.Embeddings.Provider != "ollama" {
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Embeddings.Provider)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	response, err := embedder.Embed(ctx, models.EmbeddingRequest{
		Model: cfg.Embeddings.Model,
		Input: []string{trimTo(query, cfg.Embeddings.MaxTextChars)},
	})
	if err != nil {
		return nil, err
	}
	if len(response.Embeddings) == 0 {
		return nil, nil
	}
	return s.searchVector(ctx, response.Embeddings[0], cfg)
}

func (s *Store) BuildContext(ctx context.Context, query string, topK int, maxChars int) (Context, error) {
	cfg := DefaultConfig()
	cfg.TopK = topK
	results, err := s.SearchWithConfig(ctx, query, cfg, nil)
	if err != nil {
		return Context{}, err
	}
	return contextFromResults(results, maxChars), nil
}

func (s *Store) BuildContextWithConfig(ctx context.Context, query string, cfg Config, embedder models.EmbeddingRuntime, maxChars int) (Context, error) {
	results, err := s.SearchWithConfig(ctx, query, cfg, embedder)
	if err != nil {
		return Context{}, err
	}
	return contextFromResults(results, maxChars), nil
}

func contextFromResults(results []SearchResult, maxChars int) Context {
	if maxChars <= 0 {
		maxChars = 10000
	}

	var builder strings.Builder
	sources := make([]string, 0, len(results))
	seenSources := map[string]bool{}
	remaining := maxChars
	for _, result := range results {
		if remaining <= 0 {
			break
		}
		excerpt := trimTo(result.Content, remaining)
		if excerpt == "" {
			continue
		}
		if !seenSources[result.Path] {
			sources = append(sources, result.Path)
			seenSources[result.Path] = true
		}
		fmt.Fprintf(&builder, "[%d] %s\n%s\n\n", len(sources), result.Path, excerpt)
		remaining -= len(excerpt)
	}

	if builder.Len() == 0 {
		return Context{}
	}
	return Context{
		Sources: sources,
		Text:    strings.TrimSpace("RETRIEVED CONTEXT:\n" + builder.String()),
	}
}

func (s *Store) EnsureEmbeddings(ctx context.Context, cfg Config, embedder models.EmbeddingRuntime) (EmbeddingIndexResult, error) {
	cfg = applyConfigDefaults(cfg)
	result := EmbeddingIndexResult{
		Enabled:  cfg.Embeddings.Enabled,
		Provider: cfg.Embeddings.Provider,
		Model:    cfg.Embeddings.Model,
	}
	if !cfg.Embeddings.Enabled {
		return result, nil
	}
	if embedder == nil {
		return result, fmt.Errorf("embedding runtime is unavailable")
	}
	if cfg.Embeddings.Provider != "ollama" {
		return result, fmt.Errorf("unsupported embedding provider: %s", cfg.Embeddings.Provider)
	}

	for {
		chunks, err := s.unembeddedChunks(ctx, cfg)
		if err != nil {
			return result, err
		}
		if len(chunks) == 0 {
			return result, nil
		}
		inputs := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			inputs = append(inputs, trimTo(chunk.Content, cfg.Embeddings.MaxTextChars))
		}
		response, err := embedder.Embed(ctx, models.EmbeddingRequest{
			Model: cfg.Embeddings.Model,
			Input: inputs,
		})
		if err != nil {
			return result, err
		}
		if len(response.Embeddings) != len(chunks) {
			return result, fmt.Errorf("embedding runtime returned %d embedding(s) for %d chunk(s)", len(response.Embeddings), len(chunks))
		}
		if len(response.Embeddings) > 0 {
			result.Dimensions = len(response.Embeddings[0])
		}
		stored, skipped, err := s.storeEmbeddings(ctx, cfg, chunks, response.Embeddings)
		if err != nil {
			return result, err
		}
		result.ChunksEmbedded += stored
		result.ChunksSkipped += skipped
		if stored == 0 && skipped > 0 {
			return result, fmt.Errorf("embedding runtime returned only empty vectors")
		}
	}
}

func (s *Store) migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS rag_documents (
			id TEXT PRIMARY KEY,
			profile_id TEXT,
			workspace_root TEXT NOT NULL,
			path TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			mime_type TEXT,
			indexed_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(workspace_root, path)
		);`,
		`CREATE TABLE IF NOT EXISTS rag_chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			path TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			content TEXT NOT NULL,
			start_byte INTEGER NOT NULL,
			end_byte INTEGER NOT NULL,
			token_estimate INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(document_id) REFERENCES rag_documents(id) ON DELETE CASCADE
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS rag_fts USING fts5(
			content,
			chunk_id UNINDEXED,
			document_id UNINDEXED,
			path UNINDEXED
		);`,
		`CREATE INDEX IF NOT EXISTS idx_rag_documents_path ON rag_documents(workspace_root, path);`,
		`CREATE INDEX IF NOT EXISTS idx_rag_chunks_document ON rag_chunks(document_id);`,
		`CREATE TABLE IF NOT EXISTS rag_embeddings (
			chunk_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			dimensions INTEGER NOT NULL,
			vector_json TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY(chunk_id, provider, model),
			FOREIGN KEY(chunk_id) REFERENCES rag_chunks(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_rag_embeddings_model ON rag_embeddings(provider, model);`,
	}
	for index, statement := range migrations {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply rag migration %d: %w", index+1, err)
		}
	}
	return nil
}

func (s *Store) existingDocument(ctx context.Context, root string, path string) (id string, hash string, err error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, content_hash FROM rag_documents
		WHERE workspace_root = ? AND path = ?
	`, root, path)
	if err := row.Scan(&id, &hash); err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil
		}
		return "", "", fmt.Errorf("query existing rag document: %w", err)
	}
	return id, hash, nil
}

func (s *Store) replaceDocument(ctx context.Context, doc documentToStore) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin rag transaction: %w", err)
	}
	defer tx.Rollback()

	if doc.ExistingID != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM rag_fts WHERE document_id = ?`, doc.ExistingID); err != nil {
			return 0, fmt.Errorf("delete old rag fts rows: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM rag_chunks WHERE document_id = ?`, doc.ExistingID); err != nil {
			return 0, fmt.Errorf("delete old rag chunks: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM rag_documents WHERE id = ?`, doc.ExistingID); err != nil {
			return 0, fmt.Errorf("delete old rag document: %w", err)
		}
	}

	now := timestamp()
	documentID := newID("doc")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO rag_documents (
			id, profile_id, workspace_root, path, content_hash,
			size_bytes, mime_type, indexed_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, documentID, doc.ProfileID, doc.Root, doc.Path, doc.Hash, doc.SizeBytes, doc.MimeType, now, now); err != nil {
		return 0, fmt.Errorf("insert rag document: %w", err)
	}

	for _, chunk := range doc.Chunks {
		chunkID := newID("chunk")
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO rag_chunks (
				id, document_id, path, chunk_index, content,
				start_byte, end_byte, token_estimate, created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, chunkID, documentID, doc.Path, chunk.Index, chunk.Content, chunk.StartByte, chunk.EndByte, chunk.TokenEstimate, now); err != nil {
			return 0, fmt.Errorf("insert rag chunk: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO rag_fts (content, chunk_id, document_id, path)
			VALUES (?, ?, ?, ?)
		`, chunk.Content, chunkID, documentID, doc.Path); err != nil {
			return 0, fmt.Errorf("insert rag fts row: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit rag transaction: %w", err)
	}
	return len(doc.Chunks), nil
}

func (s *Store) unembeddedChunks(ctx context.Context, cfg Config) ([]embeddedChunk, error) {
	limit := cfg.Embeddings.BatchSize
	if limit <= 0 {
		limit = DefaultConfig().Embeddings.BatchSize
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.content
		FROM rag_chunks c
		LEFT JOIN rag_embeddings e
			ON e.chunk_id = c.id AND e.provider = ? AND e.model = ?
		WHERE e.chunk_id IS NULL
		ORDER BY c.created_at, c.chunk_index
		LIMIT ?
	`, cfg.Embeddings.Provider, cfg.Embeddings.Model, limit)
	if err != nil {
		return nil, fmt.Errorf("query unembedded chunks: %w", err)
	}
	defer rows.Close()

	var chunks []embeddedChunk
	for rows.Next() {
		var chunk embeddedChunk
		if err := rows.Scan(&chunk.ID, &chunk.Content); err != nil {
			return nil, fmt.Errorf("scan unembedded chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read unembedded chunks: %w", err)
	}
	return chunks, nil
}

func (s *Store) storeEmbeddings(ctx context.Context, cfg Config, chunks []embeddedChunk, vectors [][]float64) (stored int, skipped int, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin embedding transaction: %w", err)
	}
	defer tx.Rollback()

	now := timestamp()
	for index, chunk := range chunks {
		vector := vectors[index]
		if len(vector) == 0 {
			skipped++
			continue
		}
		data, err := json.Marshal(vector)
		if err != nil {
			return stored, skipped, fmt.Errorf("encode embedding vector: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO rag_embeddings (
				chunk_id, provider, model, dimensions, vector_json, content_hash, created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, chunk.ID, cfg.Embeddings.Provider, cfg.Embeddings.Model, len(vector), string(data), contentHash(chunk.Content), now); err != nil {
			return stored, skipped, fmt.Errorf("insert rag embedding: %w", err)
		}
		stored++
	}
	if err := tx.Commit(); err != nil {
		return stored, skipped, fmt.Errorf("commit embedding transaction: %w", err)
	}
	return stored, skipped, nil
}

func (s *Store) searchVector(ctx context.Context, queryVector []float64, cfg Config) ([]SearchResult, error) {
	limit := cfg.Embeddings.CandidateLimit
	if limit <= 0 {
		limit = DefaultConfig().Embeddings.CandidateLimit
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			c.id,
			c.document_id,
			c.path,
			c.content,
			c.created_at,
			e.vector_json
		FROM rag_embeddings e
		JOIN rag_chunks c ON e.chunk_id = c.id
		WHERE e.provider = ? AND e.model = ?
		LIMIT ?
	`, cfg.Embeddings.Provider, cfg.Embeddings.Model, limit)
	if err != nil {
		return nil, fmt.Errorf("query rag embeddings: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var vectorJSON string
		if err := rows.Scan(
			&result.ChunkID,
			&result.DocumentID,
			&result.Path,
			&result.Content,
			&result.CreatedAt,
			&vectorJSON,
		); err != nil {
			return nil, fmt.Errorf("scan rag embedding: %w", err)
		}
		var vector []float64
		if err := json.Unmarshal([]byte(vectorJSON), &vector); err != nil {
			return nil, fmt.Errorf("decode rag embedding: %w", err)
		}
		similarity := cosineSimilarity(queryVector, vector)
		if similarity <= 0 {
			continue
		}
		result.Source = "embedding"
		result.Similarity = similarity
		result.Rank = 1 - similarity
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read rag embeddings: %w", err)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})
	candidateLimit := rerankCandidateLimit(cfg)
	if len(results) > candidateLimit {
		results = results[:candidateLimit]
	}
	return results, nil
}

func boundedDocumentInventoryLimit(limit int) int {
	if limit <= 0 {
		return defaultDocumentInventoryLimit
	}
	if limit > maxDocumentInventoryLimit {
		return maxDocumentInventoryLimit
	}
	return limit
}

func (s *Store) markDocumentInventoryStatus(ctx context.Context, item *DocumentInventoryItem) error {
	item.Status = DocumentStatusIndexed
	if item.WorkspaceRoot == uploadedDocumentRoot {
		item.ManagedSource = "upload"
		return nil
	}
	item.ManagedSource = "filesystem"
	currentHash, missing, err := documentSourceHash(ctx, item.WorkspaceRoot, item.Path)
	if err != nil {
		item.Status = DocumentStatusUnknown
		item.Reason = fmt.Sprintf("source status could not be checked: %v", err)
		return nil
	}
	if missing {
		item.Status = DocumentStatusMissing
		item.Missing = true
		item.Reason = "source file is missing"
		return nil
	}
	item.CurrentContentHash = currentHash
	if currentHash != "" && currentHash != item.ContentHash {
		item.Status = DocumentStatusStale
		item.Stale = true
		item.Reason = "source content hash differs from indexed hash"
	}
	return nil
}

func documentSourceHash(ctx context.Context, root string, path string) (string, bool, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return "", true, nil
		}
		return "", false, err
	}
	if isExtractableDocument(path) {
		extracted, err := extractDocument(ctx, root, path, DefaultConfig().MaxFileBytes)
		if err != nil {
			return "", false, err
		}
		return contentHash(extracted.Content), false, nil
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", true, nil
		}
		return "", false, err
	}
	return contentHash(string(data)), false, nil
}

func (s *Store) countDocumentFTSRows(ctx context.Context, documentID string) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM rag_fts WHERE document_id = ?
	`, documentID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count rag fts rows: %w", err)
	}
	return count, nil
}

func execRowsAffected(ctx context.Context, tx *sql.Tx, query string, args ...any) (int, error) {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

type documentToStore struct {
	ProfileID  string
	Root       string
	Path       string
	ExistingID string
	Hash       string
	SizeBytes  int64
	MimeType   string
	Chunks     []Chunk
}

type embeddedChunk struct {
	ID      string
	Content string
}

func buildFTSQuery(query string) string {
	terms := strings.FieldsFunc(query, func(r rune) bool {
		return !(r == '_' || unicode.IsDigit(r) || unicode.IsLetter(r))
	})
	clean := terms[:0]
	for _, term := range terms {
		term = strings.ToLower(strings.Trim(term, "_"))
		if term != "" && !isRAGQueryStopTerm(term) {
			clean = append(clean, quoteFTSTerm(term))
		}
	}
	return strings.Join(clean, " ")
}

var ragQueryStopTerms = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "can": true, "do": true, "does": true, "for": true,
	"from": true, "how": true, "i": true, "if": true, "in": true, "is": true,
	"it": true, "me": true, "my": true, "of": true, "on": true, "or": true,
	"the": true, "this": true, "to": true, "using": true, "use": true, "uses": true,
	"what": true, "when": true, "where": true, "which": true, "who": true, "why": true,
	"with": true, "your": true,

	// Retrieval mode words help the router, but they should not make FTS require
	// every matching document to literally say "local documents" or "RAG".
	"doc": true, "docs": true, "document": true, "documents": true, "local": true, "rag": true,
	"retrieved": true, "ingested": true, "source": true, "sources": true,
}

func isRAGQueryStopTerm(term string) bool {
	if len(term) < 2 {
		return true
	}
	return ragQueryStopTerms[term]
}

func quoteFTSTerm(term string) string {
	return `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
}

func mergeSearchResults(primary []SearchResult, secondary []SearchResult, topK int) []SearchResult {
	if topK <= 0 {
		topK = DefaultConfig().TopK
	}
	merged := make([]SearchResult, 0, topK)
	seen := map[string]bool{}
	add := func(results []SearchResult) {
		for _, result := range results {
			if len(merged) >= topK {
				return
			}
			if seen[result.ChunkID] {
				continue
			}
			seen[result.ChunkID] = true
			merged = append(merged, result)
		}
	}
	add(primary)
	add(secondary)
	return merged
}

func cosineSimilarity(left []float64, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot float64
	var leftNorm float64
	var rightNorm float64
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func newID(prefix string) string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%s_%x", prefix, time.Now().UTC().UnixNano())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

func trimTo(input string, maxChars int) string {
	input = strings.TrimSpace(input)
	if maxChars <= 0 || input == "" {
		return ""
	}
	if len(input) <= maxChars {
		return input
	}
	return strings.TrimSpace(input[:maxChars]) + "\n[truncated]"
}

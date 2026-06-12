package rag

import (
	"context"
	"fmt"
)

const bytesPerMiB = 1024 * 1024

type IndexStats struct {
	Documents                  int   `json:"documents"`
	Chunks                     int   `json:"chunks"`
	Embeddings                 int   `json:"embeddings"`
	IndexedBytes               int64 `json:"indexedBytes"`
	ChunkTextBytes             int64 `json:"chunkTextBytes"`
	EmbeddingDimensions        int   `json:"embeddingDimensions"`
	StoredEmbeddingVectorBytes int64 `json:"storedEmbeddingVectorBytes"`
}

type VectorDBResearchReport struct {
	Enabled                bool                `json:"enabled"`
	Provider               string              `json:"provider"`
	ResearchOnly           bool                `json:"researchOnly"`
	LocalOnly              bool                `json:"localOnly"`
	AllowBackgroundService bool                `json:"allowBackgroundService"`
	MaxRAMMB               int                 `json:"maxRamMb"`
	MaxStorageMB           int                 `json:"maxStorageMb"`
	MinBenefitPercent      int                 `json:"minBenefitPercent"`
	SQLiteFTSActive        bool                `json:"sqliteFtsActive"`
	SQLiteEmbeddingsActive bool                `json:"sqliteEmbeddingsActive"`
	Stats                  IndexStats          `json:"stats"`
	Candidates             []VectorDBCandidate `json:"candidates"`
	Approved               bool                `json:"approved"`
	Recommendation         string              `json:"recommendation"`
	Reasons                []string            `json:"reasons"`
}

type VectorDBCandidate struct {
	Name                      string `json:"name"`
	Status                    string `json:"status"`
	RequiresBackgroundService bool   `json:"requiresBackgroundService"`
	LocalOnly                 bool   `json:"localOnly"`
	EstimatedRAMMB            int    `json:"estimatedRamMb"`
	EstimatedStorageMB        int    `json:"estimatedStorageMb"`
	Reason                    string `json:"reason"`
}

func (s *Store) IndexStats(ctx context.Context) (IndexStats, error) {
	if s == nil || s.db == nil {
		return IndexStats{}, fmt.Errorf("rag store is unavailable")
	}
	var stats IndexStats
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM rag_documents
	`).Scan(&stats.Documents, &stats.IndexedBytes); err != nil {
		return IndexStats{}, fmt.Errorf("read rag document stats: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(length(content)), 0)
		FROM rag_chunks
	`).Scan(&stats.Chunks, &stats.ChunkTextBytes); err != nil {
		return IndexStats{}, fmt.Errorf("read rag chunk stats: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(MAX(dimensions), 0), COALESCE(SUM(length(vector_json)), 0)
		FROM rag_embeddings
	`).Scan(&stats.Embeddings, &stats.EmbeddingDimensions, &stats.StoredEmbeddingVectorBytes); err != nil {
		return IndexStats{}, fmt.Errorf("read rag embedding stats: %w", err)
	}
	return stats, nil
}

func (s *Store) EvaluateVectorDB(ctx context.Context, cfg Config) (VectorDBResearchReport, error) {
	cfg = applyConfigDefaults(cfg)
	stats, err := s.IndexStats(ctx)
	if err != nil {
		return VectorDBResearchReport{}, err
	}
	vectorCfg := cfg.VectorDB
	report := VectorDBResearchReport{
		Enabled:                vectorCfg.Enabled,
		Provider:               vectorCfg.Provider,
		ResearchOnly:           vectorCfg.ResearchOnly,
		LocalOnly:              vectorCfg.LocalOnly,
		AllowBackgroundService: vectorCfg.AllowBackgroundService,
		MaxRAMMB:               vectorCfg.MaxRAMMB,
		MaxStorageMB:           vectorCfg.MaxStorageMB,
		MinBenefitPercent:      vectorCfg.MinBenefitPercent,
		SQLiteFTSActive:        cfg.Enabled && cfg.Mode == "sqlite_fts",
		SQLiteEmbeddingsActive: cfg.Embeddings.Enabled && stats.Embeddings > 0,
		Stats:                  stats,
		Approved:               false,
		Recommendation:         "keep_sqlite_fts",
	}
	report.Candidates = []VectorDBCandidate{
		vectorCandidate("faiss", false, 128, 1, stats, vectorCfg),
		vectorCandidate("chroma", true, 384, 64, stats, vectorCfg),
		vectorCandidate("qdrant", true, 512, 128, stats, vectorCfg),
	}
	report.Reasons = vectorResearchReasons(report)
	return report, nil
}

func vectorCandidate(name string, requiresBackground bool, baseRAMMB int, baseStorageMB int, stats IndexStats, cfg VectorDBConfig) VectorDBCandidate {
	ramMB := baseRAMMB + vectorWorkingSetMB(stats)
	storageMB := baseStorageMB + bytesToMBCeil(stats.StoredEmbeddingVectorBytes)
	candidate := VectorDBCandidate{
		Name:                      name,
		Status:                    "research_only",
		RequiresBackgroundService: requiresBackground,
		LocalOnly:                 true,
		EstimatedRAMMB:            ramMB,
		EstimatedStorageMB:        storageMB,
		Reason:                    fmt.Sprintf("requires measured retrieval benefit of at least %d%% before approval", cfg.MinBenefitPercent),
	}
	if requiresBackground && !cfg.AllowBackgroundService {
		candidate.Status = "blocked"
		candidate.Reason = "background services are not allowed by the low-resource vector gate"
		return candidate
	}
	if cfg.MaxRAMMB > 0 && ramMB > cfg.MaxRAMMB {
		candidate.Status = "blocked"
		candidate.Reason = fmt.Sprintf("estimated RAM %d MB exceeds gate limit %d MB", ramMB, cfg.MaxRAMMB)
		return candidate
	}
	if cfg.MaxStorageMB > 0 && storageMB > cfg.MaxStorageMB {
		candidate.Status = "blocked"
		candidate.Reason = fmt.Sprintf("estimated storage %d MB exceeds gate limit %d MB", storageMB, cfg.MaxStorageMB)
		return candidate
	}
	return candidate
}

func vectorResearchReasons(report VectorDBResearchReport) []string {
	reasons := []string{
		"SQLite FTS5 remains the active default RAG path",
		"no external vector database is approved without measured low-end hardware benefit",
	}
	if !report.Enabled {
		reasons = append(reasons, "vector_db.enabled is false, so this is a research-only gate")
	} else {
		reasons = append(reasons, "vector_db.enabled is true but the gate still requires explicit implementation approval")
	}
	if !report.AllowBackgroundService {
		reasons = append(reasons, "background vector services are blocked by default")
	}
	if report.Stats.Chunks == 0 {
		reasons = append(reasons, "no indexed chunks exist yet, so there is no retrieval benefit to measure")
	}
	return reasons
}

func vectorWorkingSetMB(stats IndexStats) int {
	if stats.StoredEmbeddingVectorBytes > 0 {
		return bytesToMBCeil(stats.StoredEmbeddingVectorBytes)
	}
	if stats.Embeddings > 0 && stats.EmbeddingDimensions > 0 {
		return bytesToMBCeil(int64(stats.Embeddings * stats.EmbeddingDimensions * 8))
	}
	return 0
}

func bytesToMBCeil(value int64) int {
	if value <= 0 {
		return 0
	}
	return int((value + bytesPerMiB - 1) / bytesPerMiB)
}

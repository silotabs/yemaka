package rag

type Config struct {
	Enabled          bool
	Mode             string
	ChunkSize        int
	ChunkOverlap     int
	TopK             int
	MaxFileBytes     int64
	MaxTotalBytes    int64
	MaxChunksPerFile int
	Rerank           RerankConfig
	Embeddings       EmbeddingConfig
	VectorDB         VectorDBConfig
}

type RerankConfig struct {
	CandidateLimit  int
	SourceDiversity int
}

type EmbeddingConfig struct {
	Enabled        bool
	Provider       string
	Model          string
	BatchSize      int
	MaxTextChars   int
	CandidateLimit int
}

type VectorDBConfig struct {
	Enabled                bool
	Provider               string
	ResearchOnly           bool
	LocalOnly              bool
	AllowBackgroundService bool
	MaxRAMMB               int
	MaxStorageMB           int
	MinBenefitPercent      int
}

type IngestResult struct {
	Root           string
	FilesIndexed   int
	FilesSkipped   int
	FilesUnchanged int
	ChunksCreated  int
	BytesIndexed   int64
	SkippedReasons []string
}

type SearchResult struct {
	ChunkID     string
	DocumentID  string
	Path        string
	Content     string
	Rank        float64
	Source      string
	Similarity  float64
	CreatedAt   string
	Score       float64
	Explanation []string
}

type Context struct {
	Sources []string
	Text    string
}

type EmbeddingIndexResult struct {
	Enabled        bool
	Provider       string
	Model          string
	ChunksEmbedded int
	ChunksSkipped  int
	Dimensions     int
}

type DocumentInventoryItem struct {
	ID                 string
	ProfileID          string
	WorkspaceRoot      string
	Path               string
	ContentHash        string
	CurrentContentHash string
	SizeBytes          int64
	MimeType           string
	IndexedAt          string
	UpdatedAt          string
	ChunkCount         int
	EmbeddingCount     int
	ManagedSource      string
	Status             string
	Reason             string
	Missing            bool
	Stale              bool
}

type DocumentPruneResult struct {
	DryRun            bool
	DocumentsMatched  int
	ChunksMatched     int
	FTSRowsMatched    int
	EmbeddingsMatched int
	DocumentsRemoved  int
	ChunksRemoved     int
	FTSRowsRemoved    int
	EmbeddingsRemoved int
	DocumentIDs       []string
}

func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		Mode:             "sqlite_fts",
		ChunkSize:        700,
		ChunkOverlap:     100,
		TopK:             5,
		MaxFileBytes:     300000,
		MaxTotalBytes:    50000000,
		MaxChunksPerFile: 200,
		Rerank: RerankConfig{
			CandidateLimit:  24,
			SourceDiversity: 2,
		},
		Embeddings: EmbeddingConfig{
			Enabled:        false,
			Provider:       "ollama",
			Model:          "nomic-embed-text",
			BatchSize:      8,
			MaxTextChars:   2000,
			CandidateLimit: 200,
		},
		VectorDB: VectorDBConfig{
			Enabled:                false,
			Provider:               "research_only",
			ResearchOnly:           true,
			LocalOnly:              true,
			AllowBackgroundService: false,
			MaxRAMMB:               512,
			MaxStorageMB:           1024,
			MinBenefitPercent:      20,
		},
	}
}

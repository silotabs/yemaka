package rag

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func rerankResults(query string, results []SearchResult, cfg Config) []SearchResult {
	cfg = applyConfigDefaults(cfg)
	topK := cfg.TopK
	if topK <= 0 {
		topK = DefaultConfig().TopK
	}
	if len(results) == 0 {
		return nil
	}

	terms := queryTerms(query)
	phrase := strings.Join(terms, " ")
	scored := make([]SearchResult, 0, len(results))
	for _, result := range results {
		result.Score, result.Explanation = rerankScore(result, terms, phrase)
		scored = append(scored, result)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		if scored[i].Similarity != scored[j].Similarity {
			return scored[i].Similarity > scored[j].Similarity
		}
		if scored[i].Rank != scored[j].Rank {
			return scored[i].Rank < scored[j].Rank
		}
		if scored[i].Path != scored[j].Path {
			return scored[i].Path < scored[j].Path
		}
		return scored[i].ChunkID < scored[j].ChunkID
	})

	return applySourceDiversity(scored, topK, cfg.Rerank.SourceDiversity)
}

func rerankCandidateLimit(cfg Config) int {
	cfg = applyConfigDefaults(cfg)
	limit := cfg.Rerank.CandidateLimit
	if limit <= 0 {
		limit = DefaultConfig().Rerank.CandidateLimit
	}
	if limit < cfg.TopK {
		limit = cfg.TopK
	}
	return limit
}

func rerankScore(result SearchResult, terms []string, phrase string) (float64, []string) {
	var score float64
	var reasons []string
	source := strings.TrimSpace(result.Source)
	if source == "" {
		source = "fts"
	}
	if source == "embedding" && result.Similarity > 0 {
		boost := result.Similarity * 30
		score += boost
		reasons = append(reasons, "embedding_similarity")
	} else {
		boost := 4 / (1 + math.Abs(result.Rank))
		score += boost
		reasons = append(reasons, "fts_rank")
	}

	content := normalizeText(result.Content)
	path := normalizeText(result.Path)
	title := normalizeText(strings.TrimSuffix(filepath.Base(result.Path), filepath.Ext(result.Path)))

	if phrase != "" && strings.Contains(content, phrase) {
		score += 35
		reasons = append(reasons, "exact_phrase")
	}
	if phrase != "" && (strings.Contains(path, phrase) || strings.Contains(title, phrase)) {
		score += 14
		reasons = append(reasons, "path_phrase")
	}

	contentMatches := countTermMatches(content, terms)
	if contentMatches > 0 {
		score += float64(contentMatches) * 8
		reasons = append(reasons, "term_coverage")
	}
	pathMatches := countTermMatches(path+" "+title, terms)
	if pathMatches > 0 {
		score += float64(pathMatches) * 6
		reasons = append(reasons, "path_title")
	}
	if headingMatches(result.Content, terms, phrase) {
		score += 10
		reasons = append(reasons, "heading")
	}
	if boost := recencyBoost(result.CreatedAt); boost > 0 {
		score += boost
		reasons = append(reasons, "recency")
	}
	return score, reasons
}

func applySourceDiversity(results []SearchResult, topK int, capPerPath int) []SearchResult {
	if topK <= 0 || len(results) == 0 {
		return nil
	}
	if capPerPath <= 0 {
		capPerPath = len(results)
	}
	selected := make([]SearchResult, 0, topK)
	used := map[string]int{}
	picked := map[string]bool{}
	for _, result := range results {
		if len(selected) >= topK {
			return selected
		}
		if used[result.Path] >= capPerPath {
			continue
		}
		selected = append(selected, result)
		used[result.Path]++
		picked[result.ChunkID] = true
	}
	for _, result := range results {
		if len(selected) >= topK {
			return selected
		}
		if picked[result.ChunkID] {
			continue
		}
		selected = append(selected, result)
	}
	return selected
}

func queryTerms(query string) []string {
	seen := map[string]bool{}
	var terms []string
	for _, term := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !(r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z')
	}) {
		term = strings.Trim(term, "-_")
		if len(term) < 2 || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	return terms
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func countTermMatches(value string, terms []string) int {
	count := 0
	padded := " " + value + " "
	for _, term := range terms {
		if strings.Contains(padded, " "+term+" ") || strings.Contains(value, term) {
			count++
		}
	}
	return count
}

func headingMatches(content string, terms []string, phrase string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := normalizeText(strings.TrimLeft(trimmed, "#"))
		if phrase != "" && strings.Contains(heading, phrase) {
			return true
		}
		if countTermMatches(heading, terms) > 0 {
			return true
		}
	}
	return false
}

func recencyBoost(createdAt string) float64 {
	createdAt = strings.TrimSpace(createdAt)
	if createdAt == "" {
		return 0
	}
	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return 0
	}
	ageDays := time.Since(created).Hours() / 24
	if ageDays < 0 {
		ageDays = 0
	}
	return 1 / (1 + ageDays)
}

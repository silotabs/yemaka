package rag

import "strings"

type Chunk struct {
	Index         int
	Content       string
	StartByte     int
	EndByte       int
	TokenEstimate int
}

func ChunkText(content string, chunkSize int, overlap int, maxChunks int) []Chunk {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	sizeChars := chunkSizeToChars(chunkSize)
	overlapChars := chunkSizeToChars(overlap)
	if overlapChars >= sizeChars {
		overlapChars = sizeChars / 4
	}
	if sizeChars <= 0 {
		sizeChars = 2800
	}
	if maxChunks <= 0 {
		maxChunks = DefaultConfig().MaxChunksPerFile
	}

	var chunks []Chunk
	start := 0
	for start < len(content) && len(chunks) < maxChunks {
		end := start + sizeChars
		if end >= len(content) {
			end = len(content)
		} else {
			end = adjustChunkEnd(content, start, end)
		}

		text := strings.TrimSpace(content[start:end])
		if text != "" {
			chunks = append(chunks, Chunk{
				Index:         len(chunks),
				Content:       text,
				StartByte:     start,
				EndByte:       end,
				TokenEstimate: estimateTokens(text),
			})
		}

		if end >= len(content) {
			break
		}
		next := end - overlapChars
		if next <= start {
			next = end
		}
		start = next
	}
	return chunks
}

func chunkSizeToChars(value int) int {
	if value <= 0 {
		return 0
	}
	// The blueprint gives chunk sizes in approximate tokens. Multiplying by
	// four keeps chunking deterministic without loading a tokenizer.
	return value * 4
}

func adjustChunkEnd(content string, start int, end int) int {
	windowStart := end - 300
	if windowStart < start {
		windowStart = start
	}
	window := content[windowStart:end]
	if idx := strings.LastIndex(window, "\n\n"); idx >= 0 {
		return windowStart + idx
	}
	if idx := strings.LastIndex(window, "\n"); idx >= 0 {
		return windowStart + idx
	}
	if idx := strings.LastIndex(window, ". "); idx >= 0 {
		return windowStart + idx + 1
	}
	return end
}

func estimateTokens(content string) int {
	if content == "" {
		return 0
	}
	tokens := len(content) / 4
	if tokens == 0 {
		return 1
	}
	return tokens
}

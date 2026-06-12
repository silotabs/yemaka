package contextcore

import "unicode"

const (
	DefaultMaxBytes      = 12 * 1024
	DefaultMaxTokens     = 4096
	DefaultMinItemBytes  = 96
	DefaultMinItemTokens = 24
)

type Budget struct {
	MaxBytes      int
	MaxTokens     int
	MinItemBytes  int
	MinItemTokens int
	PerSource     map[Source]Limit
}

type Limit struct {
	MaxBytes  int
	MaxTokens int
}

type Usage struct {
	Bytes  int
	Tokens int
}

type budgetCounter struct {
	budget   Budget
	used     Usage
	bySource map[Source]Usage
}

func (b Budget) withDefaults() Budget {
	if b.MaxBytes <= 0 && b.MaxTokens <= 0 {
		b.MaxBytes = DefaultMaxBytes
		b.MaxTokens = DefaultMaxTokens
	}
	if b.MinItemBytes <= 0 {
		b.MinItemBytes = DefaultMinItemBytes
	}
	if b.MinItemTokens <= 0 {
		b.MinItemTokens = DefaultMinItemTokens
	}
	if len(b.PerSource) > 0 {
		copied := make(map[Source]Limit, len(b.PerSource))
		for source, limit := range b.PerSource {
			copied[source] = limit
		}
		b.PerSource = copied
	}
	return b
}

func newBudgetCounter(budget Budget) *budgetCounter {
	return &budgetCounter{
		budget:   budget.withDefaults(),
		bySource: map[Source]Usage{},
	}
}

func (c *budgetCounter) remaining(source Source) Limit {
	limit := Limit{
		MaxBytes:  remainingLimit(c.budget.MaxBytes, c.used.Bytes),
		MaxTokens: remainingLimit(c.budget.MaxTokens, c.used.Tokens),
	}
	if sourceLimit, ok := c.budget.PerSource[source]; ok {
		sourceUsed := c.bySource[source]
		limit.MaxBytes = minLimit(limit.MaxBytes, remainingLimit(sourceLimit.MaxBytes, sourceUsed.Bytes))
		limit.MaxTokens = minLimit(limit.MaxTokens, remainingLimit(sourceLimit.MaxTokens, sourceUsed.Tokens))
	}
	return limit
}

func (c *budgetCounter) add(source Source, bytes int, tokens int) {
	c.used.Bytes += bytes
	c.used.Tokens += tokens
	usage := c.bySource[source]
	usage.Bytes += bytes
	usage.Tokens += tokens
	c.bySource[source] = usage
}

func (l Limit) empty() bool {
	return l.MaxBytes < 0 || l.MaxTokens < 0
}

func (l Limit) fitsText(content string) bool {
	return l.fits(len(content), EstimateTokens(content))
}

func (l Limit) fits(bytes int, tokens int) bool {
	if l.empty() {
		return false
	}
	if l.MaxBytes > 0 && bytes > l.MaxBytes {
		return false
	}
	if l.MaxTokens > 0 && tokens > l.MaxTokens {
		return false
	}
	return true
}

func (l Limit) hasMinimum(budget Budget) bool {
	if l.empty() {
		return false
	}
	if l.MaxBytes > 0 && l.MaxBytes < budget.MinItemBytes {
		return false
	}
	if l.MaxTokens > 0 && l.MaxTokens < budget.MinItemTokens {
		return false
	}
	return true
}

func (l Limit) targetBytes() int {
	if l.empty() {
		return -1
	}
	target := l.MaxBytes
	if l.MaxTokens > 0 {
		tokenBytes := l.MaxTokens * 4
		target = minLimit(target, tokenBytes)
	}
	return target
}

func EstimateTokens(content string) int {
	if content == "" {
		return 0
	}
	wordCount := 0
	inWord := false
	punctuation := 0
	for _, r := range content {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			if !inWord {
				wordCount++
				inWord = true
			}
			continue
		}
		inWord = false
		if !unicode.IsSpace(r) {
			punctuation++
		}
	}
	byteEstimate := (len(content) + 3) / 4
	wordEstimate := wordCount + ((punctuation + 3) / 4)
	if byteEstimate > wordEstimate {
		return byteEstimate
	}
	if wordEstimate > 0 {
		return wordEstimate
	}
	return 1
}

func remainingLimit(max int, used int) int {
	if max <= 0 {
		return 0
	}
	left := max - used
	if left <= 0 {
		return -1
	}
	return left
}

func minLimit(a int, b int) int {
	if a < 0 || b < 0 {
		return -1
	}
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

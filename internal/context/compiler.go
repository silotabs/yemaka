package contextcore

import "strings"

// Source identifies the generic origin of a context item. These are deliberately
// broad primitives so domain packs, skills, extensions, and agents can adapt
// them without becoming part of the trusted core.
type Source string

const (
	SourceUnknown     Source = "unknown"
	SourceRequest     Source = "request"
	SourceTaskState   Source = "task_state"
	SourceMemory      Source = "memory"
	SourceKnowledge   Source = "knowledge"
	SourceRAG         Source = "rag"
	SourceSkills      Source = "skills"
	SourceTools       Source = "tools"
	SourcePolicy      Source = "policy"
	SourcePreferences Source = "preferences"
)

type Item struct {
	ID              string
	Source          Source
	Title           string
	Content         string
	Relevance       float64
	Priority        int
	Required        bool
	EstimatedTokens int
	Metadata        map[string]string
}

type Input struct {
	Request string
	Items   []Item
	Options Options
}

type Options struct {
	Budget           Budget
	SourcePriorities map[Source]int
	MinRelevance     float64
	Hooks            Hooks
}

type Hooks struct {
	Compress Reducer
	Truncate Reducer
}

type Reducer func(Item, Limit) (string, bool)

type Result struct {
	Request CompiledItem
	Items   []CompiledItem
	Dropped []DroppedItem
	Stats   Stats
}

type CompiledItem struct {
	ID             string
	Source         Source
	Title          string
	Content        string
	Relevance      float64
	Priority       int
	Required       bool
	Metadata       map[string]string
	OriginalBytes  int
	OriginalTokens int
	UsedBytes      int
	UsedTokens     int
	Compressed     bool
	Truncated      bool
}

type DroppedItem struct {
	ID             string
	Source         Source
	Title          string
	Reason         string
	Relevance      float64
	Priority       int
	Required       bool
	OriginalBytes  int
	OriginalTokens int
}

type Stats struct {
	OriginalBytes   int
	OriginalTokens  int
	UsedBytes       int
	UsedTokens      int
	ItemsConsidered int
	ItemsSelected   int
	ItemsDropped    int
	Compressed      bool
	Truncated       bool
	BySource        map[Source]Usage
}

type Compiler struct {
	options Options
}

func NewCompiler(options Options) Compiler {
	options.Budget = options.Budget.withDefaults()
	if options.MinRelevance < 0 {
		options.MinRelevance = 0
	}
	if options.MinRelevance > 1 {
		options.MinRelevance = 1
	}
	return Compiler{options: options}
}

func Compile(input Input) Result {
	return NewCompiler(input.Options).Compile(input)
}

func (c Compiler) Compile(input Input) Result {
	options := c.options
	counter := newBudgetCounter(options.Budget)
	result := Result{
		Stats: Stats{BySource: map[Source]Usage{}},
	}

	request := normalizeItem(Item{
		ID:        "request",
		Source:    SourceRequest,
		Title:     "User request",
		Content:   input.Request,
		Relevance: 1,
		Required:  true,
	}, 0)
	result.Request = c.compileRequiredRequest(request, counter)
	addCompiledStats(&result.Stats, result.Request)

	candidates := make([]rankedItem, 0, len(input.Items))
	for index, item := range input.Items {
		normalized := normalizeItem(item, index)
		if normalized.Content == "" {
			result.Dropped = append(result.Dropped, droppedFromItem(normalized.Item, "empty"))
			continue
		}
		if !normalized.Required && normalized.Relevance < options.MinRelevance {
			result.Dropped = append(result.Dropped, droppedFromItem(normalized.Item, "below_relevance_threshold"))
			continue
		}
		candidates = append(candidates, normalized)
	}
	sortRankedItems(candidates, options.SourcePriorities)

	for _, candidate := range candidates {
		result.Stats.ItemsConsidered++
		compiled, ok, reason := c.compileItem(candidate.Item, counter)
		if !ok {
			result.Dropped = append(result.Dropped, droppedFromItem(candidate.Item, reason))
			continue
		}
		result.Items = append(result.Items, compiled)
		addCompiledStats(&result.Stats, compiled)
	}

	result.Stats.ItemsSelected = len(result.Items)
	result.Stats.ItemsDropped = len(result.Dropped)
	result.Stats.UsedBytes = counter.used.Bytes
	result.Stats.UsedTokens = counter.used.Tokens
	for source, usage := range counter.bySource {
		result.Stats.BySource[source] = usage
	}
	return result
}

func (c Compiler) compileRequiredRequest(item rankedItem, counter *budgetCounter) CompiledItem {
	limit := counter.remaining(item.Source)
	content, compressed, truncated := c.reduceToFit(item.Item, limit)
	compiled := compiledFromItem(item.Item, content, compressed, truncated)
	counter.add(item.Source, compiled.UsedBytes, compiled.UsedTokens)
	return compiled
}

func (c Compiler) compileItem(item Item, counter *budgetCounter) (CompiledItem, bool, string) {
	limit := counter.remaining(item.Source)
	if limit.empty() {
		return CompiledItem{}, false, "budget_exhausted"
	}
	if !item.Required && !limit.hasMinimum(c.options.Budget) {
		return CompiledItem{}, false, "budget_below_minimum"
	}

	content, compressed, truncated := c.reduceToFit(item, limit)
	if strings.TrimSpace(content) == "" {
		return CompiledItem{}, false, "empty_after_reduction"
	}
	if !limit.fitsText(content) {
		return CompiledItem{}, false, "does_not_fit_budget"
	}

	compiled := compiledFromItem(item, content, compressed, truncated)
	counter.add(item.Source, compiled.UsedBytes, compiled.UsedTokens)
	return compiled, true, ""
}

func (c Compiler) reduceToFit(item Item, limit Limit) (string, bool, bool) {
	content := item.Content
	compressed := false
	truncated := false
	if limit.fitsText(content) {
		return content, compressed, truncated
	}

	compressor := c.options.Hooks.Compress
	if compressor == nil {
		compressor = DefaultCompress
	}
	if next, changed := compressor(item, limit); strings.TrimSpace(next) != "" {
		content = strings.TrimSpace(next)
		compressed = changed || content != item.Content
	}
	if limit.fitsText(content) {
		return content, compressed, truncated
	}

	truncator := c.options.Hooks.Truncate
	if truncator == nil {
		truncator = DefaultTruncate
	}
	truncateItem := item
	truncateItem.Content = content
	if next, changed := truncator(truncateItem, limit); strings.TrimSpace(next) != "" {
		content = strings.TrimSpace(next)
		truncated = changed || content != truncateItem.Content
	}
	if !limit.fitsText(content) {
		content = TrimToLimit(content, limit)
		truncated = truncated || content != item.Content
	}
	return content, compressed, truncated
}

func normalizeItem(item Item, index int) rankedItem {
	item.Source = normalizeSource(item.Source)
	item.Title = strings.TrimSpace(item.Title)
	item.Content = CompactWhitespace(item.Content)
	item.Relevance = NormalizeRelevance(item.Relevance)
	return rankedItem{Item: item, originalIndex: index}
}

func normalizeSource(source Source) Source {
	switch source {
	case SourceRequest, SourceTaskState, SourceMemory, SourceKnowledge, SourceRAG, SourceSkills, SourceTools, SourcePolicy, SourcePreferences:
		return source
	case "":
		return SourceUnknown
	default:
		return source
	}
}

func compiledFromItem(item Item, content string, compressed bool, truncated bool) CompiledItem {
	content = strings.TrimSpace(content)
	originalTokens := item.EstimatedTokens
	if originalTokens <= 0 {
		originalTokens = EstimateTokens(item.Content)
	}
	return CompiledItem{
		ID:             item.ID,
		Source:         item.Source,
		Title:          item.Title,
		Content:        content,
		Relevance:      item.Relevance,
		Priority:       item.Priority,
		Required:       item.Required,
		Metadata:       copyMetadata(item.Metadata),
		OriginalBytes:  len(item.Content),
		OriginalTokens: originalTokens,
		UsedBytes:      len(content),
		UsedTokens:     EstimateTokens(content),
		Compressed:     compressed,
		Truncated:      truncated,
	}
}

func droppedFromItem(item Item, reason string) DroppedItem {
	originalTokens := item.EstimatedTokens
	if originalTokens <= 0 {
		originalTokens = EstimateTokens(item.Content)
	}
	return DroppedItem{
		ID:             item.ID,
		Source:         item.Source,
		Title:          item.Title,
		Reason:         reason,
		Relevance:      item.Relevance,
		Priority:       item.Priority,
		Required:       item.Required,
		OriginalBytes:  len(item.Content),
		OriginalTokens: originalTokens,
	}
}

func addCompiledStats(stats *Stats, item CompiledItem) {
	stats.OriginalBytes += item.OriginalBytes
	stats.OriginalTokens += item.OriginalTokens
	stats.Compressed = stats.Compressed || item.Compressed
	stats.Truncated = stats.Truncated || item.Truncated
}

func copyMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

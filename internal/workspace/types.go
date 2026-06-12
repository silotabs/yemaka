package workspace

import "time"

type Limits struct {
	MaxFilesScanned   int
	MaxFileBytes      int64
	MaxTotalScanBytes int64
	MaxSearchResults  int
	MaxContextFiles   int
	MaxContextChars   int
	IncludeHidden     bool
	FollowSymlinks    bool
}

type FileInfo struct {
	Path     string
	Size     int64
	Language string
	Kind     string
}

type SkipInfo struct {
	Path   string
	Reason string
}

type ScanResult struct {
	Root             string
	Files            []FileInfo
	Skipped          []SkipInfo
	LanguageCounts   map[string]int
	TopLevelDirs     []string
	ImportantFiles   []string
	Documentation    []string
	TotalIndexedSize int64
	LimitReached     bool
}

type ReadResult struct {
	Path    string
	Content string
	Size    int64
}

type SearchMatch struct {
	Path string
	Line int
	Text string
}

type ListEntry struct {
	Path  string
	Name  string
	IsDir bool
	Size  int64
}

type ListResult struct {
	Path    string
	Entries []ListEntry
	Skipped int
}

type StatResult struct {
	Path    string
	Exists  bool
	IsDir   bool
	Size    int64
	ModTime time.Time
}

type TreeEntry struct {
	Path  string
	Name  string
	IsDir bool
	Size  int64
	Depth int
}

type TreeResult struct {
	Path         string
	Entries      []TreeEntry
	Skipped      int
	LimitReached bool
}

type QuestionContext struct {
	Root    string
	Summary string
	Sources []string
	Text    string
}

func DefaultLimits() Limits {
	return Limits{
		MaxFilesScanned:   2000,
		MaxFileBytes:      200000,
		MaxTotalScanBytes: 20000000,
		MaxSearchResults:  20,
		MaxContextFiles:   6,
		MaxContextChars:   10000,
		IncludeHidden:     false,
		FollowSymlinks:    false,
	}
}

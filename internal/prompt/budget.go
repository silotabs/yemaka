package prompt

import "strings"

const (
	DefaultMaxChars        = 12000
	DefaultMinSectionChars = 240
)

type Options struct {
	MaxChars        int
	LowMemory       bool
	MinSectionChars int
	Disabled        bool
}

func applyDefaults(options Options) Options {
	if options.MaxChars <= 0 {
		options.MaxChars = DefaultMaxChars
	}
	if options.MinSectionChars <= 0 {
		options.MinSectionChars = DefaultMinSectionChars
	}
	if options.LowMemory && options.MaxChars > DefaultMaxChars {
		options.MaxChars = DefaultMaxChars
	}
	if options.MaxChars < 2000 {
		options.MaxChars = 2000
	}
	return options
}

func allocateSections(sections []Section, remaining int, options Options) []int {
	allocations := make([]int, len(sections))
	if len(sections) == 0 || remaining <= 0 {
		return allocations
	}
	desired := make([]int, len(sections))
	totalDesired := 0
	totalWeight := 0
	for index, section := range sections {
		cap := sectionCap(section.Name, options)
		if cap > len(section.Content) {
			cap = len(section.Content)
		}
		desired[index] = cap
		totalDesired += cap
		totalWeight += section.Priority
	}
	if totalDesired <= remaining {
		return desired
	}
	min := options.MinSectionChars
	if min*len(sections) > remaining {
		min = remaining / len(sections)
	}
	if min < 80 {
		min = 80
	}
	used := 0
	for index, section := range sections {
		if remaining-used <= 0 {
			break
		}
		base := min
		if base > desired[index] {
			base = desired[index]
		}
		if base > remaining-used {
			base = remaining - used
		}
		if section.Priority >= 80 || desired[index] <= min {
			allocations[index] = base
			used += base
		}
	}
	left := remaining - used
	if left <= 0 {
		return allocations
	}
	if totalWeight <= 0 {
		totalWeight = len(sections)
	}
	for left > 0 {
		progress := false
		for index, section := range sections {
			if allocations[index] >= desired[index] {
				continue
			}
			share := (left * section.Priority) / totalWeight
			if share <= 0 {
				share = 1
			}
			need := desired[index] - allocations[index]
			if share > need {
				share = need
			}
			if share > left {
				share = left
			}
			allocations[index] += share
			left -= share
			progress = true
			if left <= 0 {
				break
			}
		}
		if !progress {
			break
		}
	}
	return allocations
}

func sectionCap(name string, options Options) int {
	key := strings.ToLower(strings.TrimSpace(name))
	low := options.LowMemory
	switch key {
	case "profile memory":
		if low {
			return 900
		}
		return 1400
	case "task memory":
		if low {
			return 1200
		}
		return 1800
	case "retrieved context":
		if low {
			return 2800
		}
		return 5200
	case "active skill":
		return 240
	case "skill instructions":
		if low {
			return 1600
		}
		return 2600
	default:
		if low {
			return 1000
		}
		return 1800
	}
}

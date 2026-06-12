package safety

import (
	"fmt"
	"strings"
)

func UnifiedDiff(path string, before string, after string) string {
	if before == after {
		return fmt.Sprintf("--- %s\n+++ %s\n(no changes)\n", path, path)
	}

	beforeLines := splitLines(before)
	afterLines := splitLines(after)
	table := lcsTable(beforeLines, afterLines)

	var builder strings.Builder
	fmt.Fprintf(&builder, "--- %s\n", path)
	fmt.Fprintf(&builder, "+++ %s\n", path)
	writeDiff(&builder, beforeLines, afterLines, table, len(beforeLines), len(afterLines))
	return builder.String()
}

func splitLines(input string) []string {
	if input == "" {
		return nil
	}
	lines := strings.SplitAfter(input, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func lcsTable(left []string, right []string) [][]int {
	table := make([][]int, len(left)+1)
	for i := range table {
		table[i] = make([]int, len(right)+1)
	}
	for i := len(left) - 1; i >= 0; i-- {
		for j := len(right) - 1; j >= 0; j-- {
			if left[i] == right[j] {
				table[i][j] = table[i+1][j+1] + 1
				continue
			}
			if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	return table
}

func writeDiff(builder *strings.Builder, before []string, after []string, table [][]int, i int, j int) {
	if i > 0 && j > 0 && before[i-1] == after[j-1] {
		writeDiff(builder, before, after, table, i-1, j-1)
		builder.WriteString(" ")
		builder.WriteString(before[i-1])
		return
	}
	if j > 0 && (i == 0 || table[i][j-1] >= table[i-1][j]) {
		writeDiff(builder, before, after, table, i, j-1)
		builder.WriteString("+")
		builder.WriteString(after[j-1])
		return
	}
	if i > 0 && (j == 0 || table[i][j-1] < table[i-1][j]) {
		writeDiff(builder, before, after, table, i-1, j)
		builder.WriteString("-")
		builder.WriteString(before[i-1])
	}
}

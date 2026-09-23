package tools

import "strings"

// linePage is one page of a line-paged text, as every line-paged tool reports it.
type linePage struct {
	StartLine     int     `json:"start_line"`
	ReturnedLines int     `json:"returned_lines"`
	TotalLines    int     `json:"total_lines"`
	Truncated     bool    `json:"truncated"`
	NextStartLine *int    `json:"next_start_line"`
	Content       *string `json:"content"`
}

// newLinePage describes page, which starts at the 0-based startIndex of lines.
func newLinePage(lines []string, startIndex int, page []string) linePage {
	readThrough := startIndex + len(page)
	content := strings.Join(page, "\n")
	result := linePage{
		StartLine:     startIndex + 1,
		ReturnedLines: len(page),
		TotalLines:    len(lines),
		Truncated:     readThrough < len(lines),
		Content:       &content,
	}
	if result.Truncated {
		next := readThrough + 1
		result.NextStartLine = &next
	}
	return result
}

// pageLines takes lineCount lines from the 1-based startLine, capped at maxLines.
func pageLines(lines []string, startLine, lineCount, maxLines int) linePage {
	startIndex := min(max(0, startLine-1), len(lines))
	count := max(1, min(lineCount, maxLines))
	end := min(startIndex+count, len(lines))
	return newLinePage(lines, startIndex, lines[startIndex:end])
}

// tailLines takes the last maxLines lines, or all of them when there are fewer.
func tailLines(lines []string, maxLines int) linePage {
	startIndex := max(0, len(lines)-maxLines)
	return newLinePage(lines, startIndex, lines[startIndex:])
}

// splitLines splits text into lines the way Python's str.splitlines does for "\n" and "\r\n"
// line breaks: a trailing line break does not start another line.
func splitLines(text string) []string {
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}

// collectPages fetches one page of a collection when top is set, or every remaining page when
// it is not, so that an unbounded listing is never silently truncated to the API's first page.
// The returned cursor is nil when nothing remains.
func collectPages[T any](top int, cursor string, fetch func(top int, cursor string) ([]T, string, error)) ([]T, *string, error) {
	if top > 0 {
		rows, next, err := fetch(top, cursor)
		if err != nil || next == "" {
			return rows, nil, err
		}
		return rows, &next, nil
	}

	rows := []T{}
	for {
		page, next, err := fetch(0, cursor)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, page...)
		if next == "" {
			return rows, nil, nil
		}
		cursor = next
	}
}

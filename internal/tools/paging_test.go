package tools

import (
	"errors"
	"slices"
	"testing"
)

func TestPageLines(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}

	page := pageLines(lines, 2, 2, 100)
	if page.StartLine != 2 || page.ReturnedLines != 2 || *page.Content != "b\nc" || !page.Truncated || *page.NextStartLine != 4 {
		t.Errorf("middle page: %+v", page)
	}

	page = pageLines(lines, 4, 10, 100)
	if page.ReturnedLines != 2 || page.Truncated || page.NextStartLine != nil {
		t.Errorf("last page: %+v", page)
	}

	page = pageLines(lines, 1, 10, 3)
	if page.ReturnedLines != 3 || *page.NextStartLine != 4 {
		t.Errorf("capped page: %+v", page)
	}

	page = pageLines(lines, 99, 10, 100)
	if page.ReturnedLines != 0 || page.TotalLines != 5 || page.Truncated {
		t.Errorf("past the end: %+v", page)
	}
}

func TestTailLines(t *testing.T) {
	page := tailLines([]string{"a", "b", "c", "d"}, 2)
	if page.StartLine != 3 || *page.Content != "c\nd" || page.Truncated {
		t.Errorf("tail: %+v", page)
	}
}

func TestSplitLines(t *testing.T) {
	if got := splitLines("a\r\nb\n"); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("splitLines = %q", got)
	}
	if got := splitLines(""); len(got) != 0 {
		t.Errorf("splitLines of empty text = %q", got)
	}
}

func TestCollectPages(t *testing.T) {
	pages := map[string]struct {
		rows []int
		next string
	}{"": {[]int{1, 2}, "p2"}, "p2": {[]int{3}, "p3"}, "p3": {[]int{4}, ""}}
	fetch := func(top int, cursor string) ([]int, string, error) {
		page := pages[cursor]
		return page.rows, page.next, nil
	}

	rows, next, err := collectPages(0, "", fetch)
	if err != nil || !slices.Equal(rows, []int{1, 2, 3, 4}) || next != nil {
		t.Errorf("all pages: %v %v %v", rows, next, err)
	}

	rows, next, err = collectPages(2, "", fetch)
	if err != nil || !slices.Equal(rows, []int{1, 2}) || next == nil || *next != "p2" {
		t.Errorf("one page: %v %v %v", rows, next, err)
	}

	failure := errors.New("boom")
	_, _, err = collectPages(0, "", func(int, string) ([]int, string, error) { return nil, "", failure })
	if !errors.Is(err, failure) {
		t.Errorf("error: got %v", err)
	}
}

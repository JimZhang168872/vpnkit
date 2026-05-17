package viewport

import (
	"strings"
	"testing"
)

func TestTruncateDisplay_NoTruncationWhenShortEnough(t *testing.T) {
	if got := TruncateDisplay("hello", 10); got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestTruncateDisplay_ASCIIAddsEllipsis(t *testing.T) {
	got := TruncateDisplay("abcdefghij", 5)
	// Should end with … and total display width ≤ 5.
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected trailing …, got %q", got)
	}
}

func TestTruncateDisplay_CJKWideChars(t *testing.T) {
	// 你好世界 = 4 chars * 2 cols = 8 cols display width.
	// maxWidth=5 → only 你 (2 cols) + … (1 col) = 3 cols fits.
	got := TruncateDisplay("你好世界", 5)
	if displayWidth(got) > 5 {
		t.Errorf("truncated string %q exceeds max width 5 (got %d cols)", got, displayWidth(got))
	}
}

func TestTruncateDisplay_EmptyAndZeroWidth(t *testing.T) {
	if got := TruncateDisplay("", 10); got != "" {
		t.Errorf("empty input → empty output, got %q", got)
	}
	if got := TruncateDisplay("abc", 0); got != "" {
		t.Errorf("zero width → empty, got %q", got)
	}
}

func TestTruncateDisplay_VerySmallWidthFallsBackToEmpty(t *testing.T) {
	// maxWidth=1 with multi-byte CJK input: "你"=2 cols, "…"=1 col → "…" wins.
	got := TruncateDisplay("你好", 1)
	if got != "…" && got != "" {
		t.Errorf("got %q, want either … or empty", got)
	}
}

func TestWindow_FitsWhenSmallerThanMax(t *testing.T) {
	start, end := Window(5, 0, 10)
	if start != 0 || end != 5 {
		t.Errorf("got (%d,%d), want (0,5)", start, end)
	}
}

func TestWindow_NoOverflowOnExactFit(t *testing.T) {
	start, end := Window(10, 0, 10)
	if start != 0 || end != 10 {
		t.Errorf("got (%d,%d), want (0,10)", start, end)
	}
}

func TestWindow_CursorAtTopShowsFromZero(t *testing.T) {
	start, end := Window(100, 0, 10)
	if start != 0 || end != 10 {
		t.Errorf("cursor=0: got (%d,%d), want (0,10)", start, end)
	}
}

func TestWindow_CursorWithinFirstPageNoScroll(t *testing.T) {
	start, end := Window(100, 5, 10)
	if start != 0 || end != 10 {
		t.Errorf("cursor=5 within first 10: got (%d,%d), want (0,10)", start, end)
	}
}

func TestWindow_CursorPastFirstPageScrolls(t *testing.T) {
	// cursor=15 in list of 100, max 10. Cursor must be visible.
	start, end := Window(100, 15, 10)
	if cursor := 15; cursor < start || cursor >= end {
		t.Errorf("cursor=15 not visible in window (%d,%d)", start, end)
	}
}

func TestWindow_CursorAtBottomClampsToLast(t *testing.T) {
	start, end := Window(100, 99, 10)
	if end != 100 {
		t.Errorf("cursor=last: end should be 100, got %d", end)
	}
	if start != 90 {
		t.Errorf("cursor=last: start should be 90 (100-10), got %d", start)
	}
}

func TestWindow_MaxRowsZeroOrNegativeReturnsEmpty(t *testing.T) {
	start, end := Window(100, 50, 0)
	if start != 50 || end != 50 {
		t.Errorf("maxRows=0: got (%d,%d), want (50,50)", start, end)
	}
	start, end = Window(100, 50, -5)
	if start != 50 || end != 50 {
		t.Errorf("maxRows=-5: got (%d,%d), want (50,50)", start, end)
	}
}

func TestWindow_TotalZeroReturnsEmpty(t *testing.T) {
	start, end := Window(0, 0, 10)
	if start != 0 || end != 0 {
		t.Errorf("total=0: got (%d,%d), want (0,0)", start, end)
	}
}

func TestWindow_CursorOutOfRangeClampsToBounds(t *testing.T) {
	// cursor > total — defensive: don't panic, return last page.
	start, end := Window(20, 50, 10)
	if end != 20 || start != 10 {
		t.Errorf("cursor out of range: got (%d,%d), want (10,20)", start, end)
	}
	// negative cursor — return first page.
	start, end = Window(100, -3, 10)
	if start != 0 || end != 10 {
		t.Errorf("negative cursor: got (%d,%d), want (0,10)", start, end)
	}
}

func TestIndicator(t *testing.T) {
	if got := Indicator(0, 100, 10, 0); got != "[1-10/100]" {
		t.Errorf("got %q, want [1-10/100]", got)
	}
	if got := Indicator(90, 100, 10, 95); got != "[91-100/100]" {
		t.Errorf("got %q, want [91-100/100]", got)
	}
	// total=0 → empty indicator suppressed
	if got := Indicator(0, 0, 10, 0); got != "" {
		t.Errorf("total=0: got %q, want empty", got)
	}
	// fits in one page → still show "1-N/N"
	if got := Indicator(0, 5, 10, 0); got != "[1-5/5]" {
		t.Errorf("got %q, want [1-5/5]", got)
	}
}

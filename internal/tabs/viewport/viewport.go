// Package viewport is a tiny pure helper for slicing list-style models so
// they keep the cursor row visible inside a bounded terminal height.
//
// Tabs were rendering all rows into a lipgloss.Style with a fixed Height,
// which causes lipgloss to clip from the bottom — selected rows and even
// the footer hint vanish off-screen once content exceeds height. The fix
// is per-tab: compute Window(total, cursor, maxRows) → slice your rows →
// render only the visible slice. Add Indicator(...) somewhere visible so
// the user knows there's more above/below.
//
// No state lives here; it's all functions of (total, cursor, maxRows).
package viewport

import "fmt"

// Window returns [start, end) indices into a hypothetical row slice of
// length `total`, sized to fit at most `maxRows` rows while keeping
// `cursor` visible. Cursor is clamped to [0, total); maxRows ≤ 0 yields
// an empty window.
//
// Strategy: simple "page-locked" window. Once the cursor leaves the
// current page, jump the window so the cursor is on the LAST visible
// row going down or FIRST visible row going up. This avoids the
// disorienting "scroll one line at a time" behavior of some terminals
// while still keeping the cursor in view at all times.
func Window(total, cursor, maxRows int) (start, end int) {
	if total <= 0 || maxRows <= 0 {
		// Return a zero-width window pinned to a sensible position.
		if cursor < 0 {
			cursor = 0
		}
		if cursor > total {
			cursor = total
		}
		return cursor, cursor
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}
	// Page index: cursor // maxRows. Window starts at page * maxRows.
	page := cursor / maxRows
	start = page * maxRows
	end = start + maxRows
	if end > total {
		end = total
		// Pull start backward so we still show maxRows when possible (last page).
		if end-maxRows >= 0 {
			start = end - maxRows
		} else {
			start = 0
		}
	}
	return start, end
}

// Indicator renders "[m-n/total]" for a footer. Returns empty string when
// total is 0 (nothing to indicate). Cursor is informational only — it
// affects nothing; callers can pass 0.
func Indicator(start, total, maxRows, cursor int) string {
	if total <= 0 {
		return ""
	}
	_, end := Window(total, cursor, maxRows)
	// Display 1-indexed range.
	displayStart := start + 1
	if displayStart < 1 {
		displayStart = 1
	}
	return fmt.Sprintf("[%d-%d/%d]", displayStart, end, total)
}

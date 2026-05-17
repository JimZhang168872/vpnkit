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

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// TruncateDisplay shortens s so its rendered terminal display width is
// ≤ maxWidth columns (CJK / emoji / etc. count as 2 cols each via
// lipgloss.Width). An ellipsis "…" is appended when truncated.
//
// Caller must do this for every row before joining into a lipgloss
// container — otherwise lipgloss soft-wraps long rows into multiple lines
// and the row count blows up height bounds, which is how the Rules tab
// was overflowing and pushing the sidebar off-screen.
func TruncateDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 || s == "" {
		return ""
	}
	if displayWidth(s) <= maxWidth {
		return s
	}
	const marker = "…"
	markerW := displayWidth(marker)
	if markerW > maxWidth {
		return ""
	}
	budget := maxWidth - markerW
	runes := []rune(s)
	width := 0
	cut := 0
	for i, r := range runes {
		w := displayWidth(string(r))
		if width+w > budget {
			cut = i
			break
		}
		width += w
		cut = i + 1
	}
	return string(runes[:cut]) + marker
}

// displayWidth wraps lipgloss.Width to keep the import set local and so we
// can swap it out later (e.g. for runewidth) without touching call sites.
func displayWidth(s string) int { return lipgloss.Width(s) }

// FocusDot returns the focus-indicator marker for a panel header. Bright
// pink ● when the panel has the input focus, dim gray ○ otherwise. Always
// includes a trailing space so callers can use it as a prefix.
//
// Every panel in the TUI uses this same helper so dot color, glyph, and
// position are consistent across the app (Bug N follow-up).
func FocusDot(focused bool) string {
	if focused {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("● ")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○ ")
}

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

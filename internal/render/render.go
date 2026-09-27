// Package render produces human- and machine-readable output: aligned tables
// with either ASCII or Unicode borders, ANSI colors that honor --color, and
// JSON for scripting.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wubinstu/adrm/internal/model"
)

// ANSI escape codes.
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
)

// Style controls table/color output.
type Style struct {
	Color bool // emit ANSI colors
	ASCII bool // ASCII-only borders and symbols (English/plain mode)
}

// Cell is one table entry.
type Cell struct {
	Text  string
	Color string // ANSI code, applied only when Style.Color is on
	Align int    // -1 left, 0 auto (text left, numbers right), 1 right
}

// Table renders a bordered table to a writer.
type Table struct {
	Style   Style
	Headers []string
	Rows    [][]Cell
	Align   []int
	w       io.Writer
}

// NewTable creates a table writing to w.
func NewTable(w io.Writer, style Style, headers []string, align []int) *Table {
	return &Table{Style: style, Headers: headers, Align: align, w: w}
}

// Add appends a row.
func (t *Table) Add(cells ...Cell) { t.Rows = append(t.Rows, cells) }

// Write renders the table. An empty table prints a short notice instead.
func (t *Table) WriteEmpty(text string) error {
	if len(t.Rows) == 0 {
		_, err := fmt.Fprintln(t.w, text)
		return err
	}
	return t.Write()
}

var borderChars = map[bool]struct{ h, v, tl, tr, bl, br, cross, teeDown, teeUp, teeRight, teeLeft string }{
	false: {"─", "│", "┌", "┐", "└", "┘", "┼", "┬", "┴", "├", "┤"},
	true:  {"-", "|", "+", "+", "+", "+", "+", "+", "+", "+", "+"},
}

func (t *Table) Write() error {
	cols := len(t.Headers)
	widths := make([]int, cols)
	for i, h := range t.Headers {
		widths[i] = displayWidth(h)
	}
	plain := make([][]string, len(t.Rows))
	for r, row := range t.Rows {
		plain[r] = make([]string, cols)
		for i := 0; i < cols && i < len(row); i++ {
			plain[r][i] = row[i].Text
			if w := displayWidth(plain[r][i]); w > widths[i] {
				widths[i] = w
			}
		}
	}
	b := borderChars[t.Style.ASCII]
	line := func(l, m, r string) string {
		var sb strings.Builder
		sb.WriteString(l)
		for i := range widths {
			if i > 0 {
				sb.WriteString(m)
			}
			sb.WriteString(strings.Repeat(b.h, widths[i]+2))
		}
		sb.WriteString(r)
		return sb.String()
	}
	// header
	fmt.Fprintln(t.w, line(b.tl, b.teeDown, b.tr))
	var hb strings.Builder
	for i, h := range t.Headers {
		if i > 0 {
			hb.WriteString(b.v)
		}
		hb.WriteString(" " + pad(h, widths[i]) + " ")
	}
	fmt.Fprintln(t.w, hb.String())
	fmt.Fprintln(t.w, line(b.teeRight, b.cross, b.teeLeft))
	// rows
	for r, row := range t.Rows {
		var sb strings.Builder
		for i := 0; i < cols; i++ {
			if i > 0 {
				sb.WriteString(b.v)
			}
			text := plain[r][i]
			align := 0
			if i < len(t.Align) {
				align = t.Align[i]
			}
			if align == 0 && i < len(row) {
				align = row[i].Align
			}
			padded := padAlign(text, widths[i], align)
			if t.Style.Color && i < len(row) && row[i].Color != "" {
				padded = row[i].Color + padded + Reset
			}
			sb.WriteString(" " + padded + " ")
		}
		fmt.Fprintln(t.w, sb.String())
	}
	fmt.Fprintln(t.w, line(b.bl, b.teeUp, b.br))
	return nil
}

func pad(s string, width int) string {
	if n := width - displayWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func padAlign(s string, width int, align int) string {
	switch align {
	case 1:
		if n := width - displayWidth(s); n > 0 {
			return strings.Repeat(" ", n) + s
		}
		return s
	case -1:
		return pad(s, width)
	default:
		return pad(s, width)
	}
}

// displayWidth approximates the terminal width of s, treating wide (CJK)
// runes as two cells and ignoring ANSI escapes.
func displayWidth(s string) int {
	s = stripANSI(s)
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func runeWidth(r rune) int {
	if r == utf8.RuneError {
		return 1
	}
	if (r >= 0x1100 && r <= 0x115F) || // Hangul Jamo
		(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) || // CJK Radicals .. Yi
		(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0xFE30 && r <= 0xFE6F) || // CJK Compatibility Forms
		(r >= 0xFF00 && r <= 0xFF60) || // Fullwidth Forms
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x3FFFD) { // CJK Ext B+
		return 2
	}
	return 1
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inEsc {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEsc = false
			}
			continue
		}
		if c == 0x1b {
			inEsc = true
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// FormatTime renders a unix timestamp with the configured layout.
func FormatTime(ts int64, layout string) string {
	if layout == "" {
		layout = "2006-01-02 15:04:05"
	}
	return time.Unix(ts, 0).Local().Format(layout)
}

// FormatLeft renders the time remaining until ts (or "expired"), pure ASCII.
func FormatLeft(ts, now int64) string {
	if ts <= now {
		return "expired"
	}
	d := ts - now
	switch {
	case d >= 86400:
		return fmt.Sprintf("%dd", d/86400)
	case d >= 3600:
		return fmt.Sprintf("%dh", d/3600)
	case d >= 60:
		return fmt.Sprintf("%dm", d/60)
	default:
		return fmt.Sprintf("%ds", d)
	}
}

// StateColor maps a bin state to an ANSI color.
func StateColor(state string) string {
	switch state {
	case "recycled":
		return Blue
	case "exception":
		return Red
	}
	return ""
}

// OpColor maps a reflog op to an ANSI color.
func OpColor(op string) string {
	switch op {
	case model.OpRecycle:
		return Blue
	case model.OpRestore:
		return Green
	case model.OpPurge, model.OpExpire:
		return Yellow
	case model.OpException:
		return Red
	case model.OpReset, model.OpEmpty:
		return Magenta
	}
	return ""
}

// JSON prints v as indented JSON.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TimeSpec is a resolved time filter. Lo/Hi are optional bounds; when Exact is
// true both are set and the row must match the timestamp exactly.
type TimeSpec struct {
	Lo, Hi *int64
	Exact  bool
}

// Match reports whether ts satisfies the spec.
func (s *TimeSpec) Match(ts int64) bool {
	if s == nil {
		return true
	}
	if s.Exact && (s.Lo == nil || s.Hi == nil) {
		return true
	}
	if s.Lo != nil && ts < *s.Lo {
		return false
	}
	if s.Hi != nil && ts > *s.Hi {
		return false
	}
	return true
}

// SizeSpec is a resolved size filter. Lo/Hi are optional bounds; Exact means
// the size must equal Lo (== Hi).
type SizeSpec struct {
	Lo, Hi *int64
	Exact  bool
}

// Match reports whether n satisfies the spec.
func (s *SizeSpec) Match(n int64) bool {
	if s == nil {
		return true
	}
	if s.Exact && s.Lo != nil {
		return n == *s.Lo
	}
	if s.Lo != nil && n < *s.Lo {
		return false
	}
	if s.Hi != nil && n > *s.Hi {
		return false
	}
	return true
}

var unitSecs = map[byte]int64{
	's': 1,
	'm': 60,
	'h': 3600,
	'd': 86400,
	// No months or years on purpose: use --until for long horizons.
}

// ParseDuration parses "3d4h5m" style durations (no sign, no whitespace).
func ParseDuration(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	var total int64
	num := strings.Builder{}
	seen := map[byte]bool{}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			num.WriteByte(c)
			continue
		}
		mult, ok := unitSecs[c]
		if !ok {
			return 0, fmt.Errorf("invalid duration %q: unknown unit %q (want s/m/h/d)", s, string(c))
		}
		if num.Len() == 0 {
			return 0, fmt.Errorf("invalid duration %q: missing number before %q", s, string(c))
		}
		if seen[c] {
			return 0, fmt.Errorf("invalid duration %q: repeated unit %q", s, string(c))
		}
		seen[c] = true
		n, err := strconv.ParseInt(num.String(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: number too large", s)
		}
		total += n * mult
		num.Reset()
	}
	if num.Len() > 0 {
		return 0, fmt.Errorf("invalid duration %q: trailing number without unit", s)
	}
	// A zero duration is allowed on purpose: "+0s" means "expire right now".
	return total, nil
}

// ParseDurationSign parses "+3d4h5m" / "-1d" style relative timestamps.
func ParseDurationSign(s string) (int64, error) {
	sign := int64(1)
	body := s
	if strings.HasPrefix(body, "+") {
		body = body[1:]
	} else if strings.HasPrefix(body, "-") {
		sign = -1
		body = body[1:]
	}
	d, err := ParseDuration(body)
	if err != nil {
		return 0, err
	}
	return sign * d, nil
}

// parseDateTime accepts flexible separators (/ _ -) and optional time.
func parseDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if len(s) == 8 && allDigits(s) { // YYYYMMDD
		t, err := time.ParseInLocation("20060102", s, time.Local)
		if err == nil {
			return t, nil
		}
	}
	if t, err := time.ParseInLocation("2006-1-2 15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	for _, norm := range []string{"2006/1/2 15:04:05", "2006_1_2 15:04:05"} {
		if t, err := time.ParseInLocation(norm, s, time.Local); err == nil {
			return t, nil
		}
	}
	// normalize separators to '-' then rely on Go's flexible month/day parsing
	n := strings.NewReplacer("/", "-", "_", "-").Replace(s)
	if t, err := time.ParseInLocation("2006-1-2 15:04:05", n, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-1-2 15:04", n, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-1-2", n, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid datetime %q (want e.g. 2027/01/01 or 2027-1-1 03:45:01)", s)
}

// ParseTimeArg resolves values used by --until / --mtime / --older-than /
// --newer-than and friends. Accepted forms:
//
//	3d4h5m                 exactly now-3d4h5m (seconds ignored)
//	+3d5m7s                at or after now-3d5m7s
//	-121d                  at or before now-121d
//	2027-1-1 03:45:01      exactly that moment
//	+2027/1/1 3:45         at or after that moment
//	-2027_01_01            at or before that moment
func ParseTimeArg(s string, now int64) (*TimeSpec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty time expression")
	}
	sign := byte(0)
	body := s
	if strings.HasPrefix(body, "+") {
		sign = '+'
		body = body[1:]
	} else if strings.HasPrefix(body, "-") {
		sign = '-'
		body = body[1:]
	}
	if body == "" {
		return nil, fmt.Errorf("invalid time expression %q", s)
	}
	// Try relative duration first.
	if !looksLikeDate(body) {
		d, err := ParseDurationSign(appendSign(sign, body))
		if err != nil {
			return nil, err
		}
		// "+3d" and "-3d" both point at the moment 3 days ago; the sign only
		// decides which side of that moment is matched.
		at := now - abs64(d)
		lo, hi := at, at
		switch sign {
		case '+': // newer than / at or after
			return &TimeSpec{Lo: &lo}, nil
		case '-': // older than / at or before
			return &TimeSpec{Hi: &hi}, nil
		default:
			return &TimeSpec{Lo: &lo, Hi: &hi, Exact: true}, nil
		}
	}
	t, err := parseDateTime(body)
	if err != nil {
		return nil, err
	}
	ts := t.Unix()
	lo, hi := ts, ts
	switch sign {
	case '+':
		return &TimeSpec{Lo: &lo}, nil
	case '-':
		return &TimeSpec{Hi: &hi}, nil
	default:
		return &TimeSpec{Lo: &lo, Hi: &hi, Exact: true}, nil
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func appendSign(sign byte, body string) string {
	switch sign {
	case '+':
		return "+" + body
	case '-':
		return "-" + body
	}
	return body
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

func looksLikeDate(s string) bool {
	if strings.ContainsAny(s, "-_/") {
		return true
	}
	if strings.Contains(s, ":") {
		return true
	}
	digits := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			digits++
		}
	}
	// A bare 8-digit blob is treated as YYYYMMDD, otherwise as something else.
	return digits == 8
}

var sizeUnits = []struct {
	suffix string
	factor int64
}{
	{"t", 1 << 40},
	{"g", 1 << 30},
	{"m", 1 << 20},
	{"k", 1 << 10},
	{"b", 1},
}

// ParseSizeArg resolves "10t20g30m40k50b", "+30m50b", "-1g" style expressions.
func ParseSizeArg(s string) (*SizeSpec, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil, fmt.Errorf("empty size expression")
	}
	exact := true
	body := s
	if strings.HasPrefix(body, "+") {
		exact = false
		body = body[1:]
	} else if strings.HasPrefix(body, "-") {
		exact = false
		body = body[1:]
	}
	// Parse "10g20m" style sums; suffix must be attached to a number.
	var total int64
	num := strings.Builder{}
	matched := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c >= '0' && c <= '9' {
			num.WriteByte(c)
			continue
		}
		f, ok := unitFactor(c)
		if !ok {
			return nil, fmt.Errorf("invalid size %q: unknown unit %q (want b/k/m/g/t)", s, string(c))
		}
		if num.Len() == 0 {
			return nil, fmt.Errorf("invalid size %q: missing number before %q", s, string(c))
		}
		n, err := strconv.ParseInt(num.String(), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid size %q: number too large", s)
		}
		total += n * f
		num.Reset()
		matched = true
	}
	if num.Len() > 0 || !matched {
		return nil, fmt.Errorf("invalid size %q (want e.g. 10g, +1m, 10t20g30m40k50b)", s)
	}
	if exact {
		v := total
		return &SizeSpec{Lo: &v, Hi: &v, Exact: true}, nil
	}
	if strings.HasPrefix(s, "+") {
		v := total
		return &SizeSpec{Lo: &v}, nil
	}
	v := total
	return &SizeSpec{Hi: &v}, nil
}

func unitFactor(c byte) (int64, bool) {
	for _, u := range sizeUnits {
		if u.suffix[0] == c {
			return u.factor, true
		}
	}
	return 0, false
}

// FormatSize renders bytes in a compact, pure-ASCII form (safe for every
// terminal and both languages): "0 B", "512 B", "4.0K", "1.5G".
func FormatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(n)/float64(div), "KMGTPE"[exp])
}

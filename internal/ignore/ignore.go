// Package ignore implements gitignore-style pattern matching for the adrm
// ignore file. Patterns are matched against the path of a file relative to
// the ignore file's base directory (the adrm home by default), except for
// patterns without any slash, which also match a bare file name at any depth.
package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ruleKind int

const (
	kindBasename ruleKind = iota // no slash: match the bare file name
	kindRelative                 // inner slash: match abs path or path below $HOME
	kindAbsolute                 // leading slash: match the absolute path
)

type rule struct {
	negate  bool
	dirOnly bool
	kind    ruleKind
	re      *regexp.Regexp
	src     string
}

// Matcher evaluates paths against the compiled ruleset.
//
// Matching is intentionally more forgiving than gitignore, because this is a
// single global ignore file for the trash: every pattern is tried against
//   - the absolute path,
//   - the path relative to the user's home directory,
//   - the bare file name (for patterns without a slash).
//
// A pattern anchored with a leading "/" therefore means "rooted at the
// filesystem root" (e.g. /var/log/*.log), which is what people expect from a
// global ignore file; patterns with an inner slash (Downloads/*.iso) match
// below $HOME as well as absolute paths.
type Matcher struct {
	rules []rule
	base  string
	home  string
}

// New creates an empty matcher; base is the adrm home, home the user home.
func New(base, home string) *Matcher {
	if base == "" {
		base = "."
	}
	return &Matcher{base: base, home: home}
}

// LoadFile reads a gitignore-style file. A missing file yields an empty matcher.
func LoadFile(base, path string) (*Matcher, error) {
	m := New(base, userHomeDir())
	if path == "" {
		return m, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m.Add(sc.Text())
	}
	return m, sc.Err()
}

// Empty reports whether the matcher has no active rules.
func (m *Matcher) Empty() bool { return len(m.rules) == 0 }

// Add compiles one pattern line. Blank lines and '#' comments are skipped, as
// are malformed patterns (they never match).
func (m *Matcher) Add(line string) {
	raw := line
	if strings.HasPrefix(line, `\#`) || strings.HasPrefix(line, `\!`) {
		line = line[1:]
	} else {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			return
		}
	}
	r := rule{src: raw}
	if strings.HasPrefix(line, "!") {
		r.negate = true
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		r.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		// A leading slash anchors at the filesystem root; keep it in the
		// pattern so it matches absolute paths.
		r.kind = kindAbsolute
	} else if strings.Contains(line, "/") {
		r.kind = kindRelative
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	rx, err := regexp.Compile("^(?:" + globToRegexp(line) + ")$")
	if err != nil {
		return // never matches
	}
	r.re = rx
	m.rules = append(m.rules, r)
}

// Match reports whether absPath (with isDir) is ignored. The last matching
// rule wins, so a later "!keep" pattern can re-include a file.
func (m *Matcher) Match(absPath string, isDir bool) bool {
	if len(m.rules) == 0 {
		return false
	}
	abs := filepath.ToSlash(absPath)
	base := filepath.Base(abs)
	rel := ""
	if m.home != "" {
		if r, err := filepath.Rel(m.home, absPath); err == nil && !strings.HasPrefix(r, "..") {
			rel = filepath.ToSlash(r)
		}
	}
	ignored := false
	for _, r := range m.rules {
		if r.dirOnly && !isDir {
			continue
		}
		matched := false
		switch r.kind {
		case kindBasename:
			matched = r.re.MatchString(base)
		case kindAbsolute:
			matched = r.re.MatchString(abs)
			if !matched && rel != "" {
				matched = r.re.MatchString(rel)
			}
		default: // kindRelative
			matched = r.re.MatchString(abs)
			if !matched && rel != "" {
				matched = r.re.MatchString(rel)
			}
		}
		if matched {
			ignored = !r.negate
		}
	}
	return ignored
}

func userHomeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// globToRegexp converts a gitignore glob to a regular expression body.
func globToRegexp(pat string) string {
	var b strings.Builder
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch c {
		case '*':
			if i+1 < len(pat) && pat[i+1] == '*' {
				b.WriteString(".*")
				i++
				// "**/" means zero or more directories
				if i+1 < len(pat) && pat[i+1] == '/' {
					b.WriteString("/?")
					i++
				}
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '[':
			j := i + 1
			neg := ""
			if j < len(pat) && (pat[j] == '!' || pat[j] == '^') {
				neg = "^"
				j++
			}
			for j < len(pat) && pat[j] != ']' {
				j++
			}
			if j >= len(pat) {
				b.WriteString(regexp.QuoteMeta("["))
				continue
			}
			b.WriteString("[" + neg + pat[i+1:j] + "]")
			i = j
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '\\':
			b.WriteString(regexp.QuoteMeta(string(c)))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

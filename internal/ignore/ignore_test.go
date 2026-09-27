package ignore

import "testing"

func TestMatch(t *testing.T) {
	m := New("/home/u/.adrm", "/home/u")
	lines := []string{
		"# comment",
		"",
		"*.tmp",
		"secret/",
		"/anchored.txt",
		"build/**",
		"!keep.tmp",
		"node_modules",
		"Downloads/*.iso",
		"/var/log/*.log",
	}
	for _, l := range lines {
		m.Add(l)
	}
	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"/home/u/a.tmp", false, true},
		{"/home/u/sub/a.tmp", false, true},
		{"/home/u/keep.tmp", false, false}, // negated later
		{"/home/u/secret", true, true},
		{"/home/u/secret", false, false}, // dir-only rule
		{"/home/u/x/secret", true, true},
		{"/anchored.txt", false, true},                // leading "/" = filesystem root
		{"/home/u/anchored.txt", false, false},
		{"/home/u/sub/anchored.txt", false, false},
		{"/home/u/build/out/x.o", false, true},
		// "build/**" matches the contents of build, not build itself (gitignore semantics)
		{"/home/u/build", true, false},
		{"/home/u/node_modules", true, true},
		{"/home/u/src/node_modules", true, true},
		{"/home/u/readme.md", false, false},
		{"/other/a.tmp", false, true}, // basename rules match anywhere
		{"/home/u/Downloads/x.iso", false, true},   // relative to $HOME
		{"/home/u/sub/Downloads/x.iso", false, false}, // not anchored at $HOME root
		{"/other/y.iso", false, false},
		{"/var/log/syslog.log", false, true},     // absolute pattern
		{"/var/log/nginx/access.log", false, false}, // * does not cross /
	}
	for _, c := range cases {
		if got := m.Match(c.path, c.isDir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestMatchGlobChars(t *testing.T) {
	m := New("/base/.adrm", "/base")
	m.Add("file?.txt")
	m.Add("[abc].log")
	m.Add("a+b.txt")
	m.Add("do.ts")
	cases := []struct {
		path string
		want bool
	}{
		{"/base/file1.txt", true},
		{"/base/file12.txt", false},
		{"/base/a.log", true},
		{"/base/d.log", false},
		{"/base/a+b.txt", true},
		{"/base/do.ts", true},
	}
	for _, c := range cases {
		if got := m.Match(c.path, false); got != c.want {
			t.Errorf("Match(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestEmptyAndLastMatchWins(t *testing.T) {
	m := New("/base/.adrm", "/base")
	if !m.Empty() {
		t.Error("fresh matcher should be empty")
	}
	if m.Match("/base/x", false) {
		t.Error("empty matcher must not match")
	}
	m.Add("*.log")
	m.Add("!important.log")
	if !m.Match("/base/x.log", false) {
		t.Error("x.log should be ignored")
	}
	if m.Match("/base/important.log", false) {
		t.Error("important.log should be re-included by the negation")
	}
}

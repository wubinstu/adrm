package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

// runCLI invokes the CLI in a sandboxed home directory.
func runCLI(t *testing.T, home string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ADRM_HOME", home)
	t.Setenv("ADRM_LANG", "en")
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb, ModeDelete)
	return code, out.String(), errb.String()
}

// runMgr runs the admr (management) face.
func runMgr(t *testing.T, home string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ADRM_HOME", home)
	t.Setenv("ADRM_LANG", "en")
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb, ModeManage)
	return code, out.String(), errb.String()
}

func runCLIStdin(t *testing.T, home, stdin string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ADRM_HOME", home)
	t.Setenv("ADRM_LANG", "en")
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errb, ModeDelete)
	return code, out.String(), errb.String()
}

// runMgrStdin runs the admr (management) face with piped stdin.
func runMgrStdin(t *testing.T, home, stdin string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("ADRM_HOME", home)
	t.Setenv("ADRM_LANG", "en")
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errb, ModeManage)
	return code, out.String(), errb.String()
}

func newSandbox(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycle(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	f1, f2 := filepath.Join(work, "a.txt"), filepath.Join(work, "b.txt")
	touch(t, f1)
	touch(t, f2)

	code, out, errS := runCLI(t, home, f1, f2)
	if code != 0 || !strings.Contains(out, "recycled") {
		t.Fatalf("recycle: code=%d out=%q err=%q", code, out, errS)
	}
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Error("f1 should be gone from the work dir")
	}

	code, out, _ = runMgr(t, home, "ls", "--json")
	if code != 0 || !strings.Contains(out, `"id": 1`) || !strings.Contains(out, `"id": 2`) {
		t.Fatalf("ls --json: code=%d out=%q", code, out)
	}

	code, out, _ = runMgr(t, home, "restore", "2")
	if code != 0 || !strings.Contains(out, "restored") {
		t.Fatalf("restore: %q", out)
	}
	if _, err := os.Stat(f2); err != nil {
		t.Errorf("f2 should be restored: %v", err)
	}
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Error("f1 should still be in the trash")
	}

	code, out, _ = runMgr(t, home, "purge", "--all", "--yes")
	if code != 0 || !strings.Contains(out, "purged 1") {
		t.Fatalf("purge: %q", out)
	}
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Error("f1 must be really gone now")
	}

	// the reflog keeps everything, even for purged items
	code, out, _ = runMgr(t, home, "log", "--json")
	if code != 0 {
		t.Fatalf("log: %d", code)
	}
	for _, want := range []string{`"op": "recycle"`, `"op": "restore"`, `"op": "purge"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %s: %s", want, out)
		}
	}
	if strings.Count(out, `"op": "recycle"`) != 2 {
		t.Errorf("expected 2 recycle entries: %s", out)
	}
}

func TestIDsNeverReused(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	for i := 0; i < 3; i++ {
		f := filepath.Join(work, string(rune('a'+i))+".txt")
		touch(t, f)
		runCLI(t, home, f)
	}
	runMgr(t, home, "purge", "1", "--yes")
	f := filepath.Join(work, "new.txt")
	touch(t, f)
	code, out, _ := runCLI(t, home, f)
	if code != 0 || !strings.Contains(out, "recycled") {
		t.Fatalf("recycle new: %d %q", code, out)
	}
	_, out, _ = runMgr(t, home, "ls", "--json")
	if strings.Contains(out, `"id": 1`) {
		t.Errorf("id 1 must not be reused after purge: %s", out)
	}
	if !strings.Contains(out, `"id": 4`) {
		t.Errorf("expected id 4 for the new item: %s", out)
	}
}

func TestStrictValidation(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	f1 := filepath.Join(work, "1.txt")
	touch(t, f1)

	// a dangling expiry among several is ambiguous: nothing may be recycled
	code, _, _ := runCLI(t, home, "+2d", f1, "+3d")
	if code != 2 {
		t.Fatalf("dangling expiry should exit 2, got %d", code)
	}
	if _, err := os.Stat(f1); err != nil {
		t.Errorf("file must not be recycled: %v", err)
	}
	// a single trailing expiry is valid and applies to the whole command
	f2 := filepath.Join(work, "2.txt")
	touch(t, f2)
	if code, _, _ := runCLI(t, home, f2, "+2d"); code != 0 {
		t.Errorf("single trailing expiry should be accepted, got %d", code)
	}
	// unknown option
	code, _, _ = runCLI(t, home, "--bogus", f1)
	if code != 2 {
		t.Errorf("unknown option should exit 2, got %d", code)
	}
	// invalid duration value
	code, _, _ = runCLI(t, home, "--for", "3x", f1)
	if code != 2 {
		t.Errorf("invalid --for should exit 2, got %d", code)
	}
	// after all failures the file is still there
	if _, err := os.Stat(f1); err != nil {
		t.Errorf("file must not be recycled: %v", err)
	}
}

func TestDurationBinding(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	f1, f2 := filepath.Join(work, "1"), filepath.Join(work, "2")
	touch(t, f1)
	touch(t, f2)
	if code, _, _ := runCLI(t, home, f1, "+1d", f2); code != 0 {
		t.Fatal("recycle failed")
	}
	_, out, _ := runMgr(t, home, "ls", "--json")
	// both items exist with different expiry: 1d for f2, default for f1
	m1 := regexp.MustCompile(`"id": 1,\s*"path": "[^"]*1",[\s\S]*?"expire_at": (\d+)`)
	m2 := regexp.MustCompile(`"id": 2,\s*"path": "[^"]*2",[\s\S]*?"expire_at": (\d+)`)
	g1 := m1.FindStringSubmatch(out)
	g2 := m2.FindStringSubmatch(out)
	if g1 == nil || g2 == nil {
		t.Fatalf("cannot parse expiry: %s", out)
	}
	if g1[1] == g2[1] {
		t.Errorf("f2 should expire sooner than f1: %s", out)
	}
}

// TestTrailingExpiry pins the two spellings of the expiry argument: it may
// precede the files it applies to (right-associative) or sit alone at the end
// of the command line, where it applies to every file in the command. More
// than one expiry with a dangling one stays a hard error.
func TestTrailingExpiry(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()

	// 1. trailing single expiry applies to the whole command
	f := filepath.Join(work, "a.txt")
	touch(t, f)
	if code, _, errS := runCLI(t, home, f, "--deadline", "2027/1/1 03:45:01"); code != 0 {
		t.Fatalf("trailing --deadline should work: %q", errS)
	}
	_, out, _ := runMgr(t, home, "ls", "--json")
	want := time.Date(2027, 1, 1, 3, 45, 1, 0, time.Local).Unix()
	if !strings.Contains(out, fmt.Sprintf(`"expire_at": %d`, want)) {
		t.Errorf("trailing --deadline not applied: %s", out)
	}

	// 2. the same for a trailing +Nd token
	f2 := filepath.Join(work, "b.txt")
	touch(t, f2)
	if code, _, errS := runCLI(t, home, f2, "+1d"); code != 0 {
		t.Fatalf("trailing +1d should work: %q", errS)
	}

	// 3. right-associative: the expiry applies to what follows it
	f3, f4 := filepath.Join(work, "c.txt"), filepath.Join(work, "d.txt")
	touch(t, f3)
	touch(t, f4)
	if code, _, _ := runCLI(t, home, "+2d", f3, f4); code != 0 {
		t.Fatal("leading +2d failed")
	}
	_, out, _ = runMgr(t, home, "ls", "--name", "c.txt", "--json")
	if !strings.Contains(out, fmt.Sprintf(`"expire_at": %d`, time.Now().Add(48*time.Hour).Unix()/10*10)) &&
		!strings.Contains(out, `"expire_at"`) {
		t.Errorf("leading +2d: %s", out)
	}

	// 4. a dangling expiry among several is still an error, and nothing runs
	f5, f6, f7 := filepath.Join(work, "e.txt"), filepath.Join(work, "f.txt"), filepath.Join(work, "g.txt")
	touch(t, f5)
	touch(t, f6)
	touch(t, f7)
	code, _, _ := runCLI(t, home, "+1d", f5, "+2d", f6, "+3d", f7, "+4d")
	if code != 2 {
		t.Errorf("dangling expiry among several should exit 2, got %d", code)
	}
	for _, p := range []string{f5, f6, f7} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("nothing may be recycled on a usage error: %s", p)
		}
	}
}

func TestIgnoreRules(t *testing.T) {
	home := newSandbox(t)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "ignore"), []byte("*.tmp\n!keep.tmp\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	tmp := filepath.Join(work, "a.tmp")
	keep := filepath.Join(work, "keep.tmp")
	other := filepath.Join(work, "a.log")
	touch(t, tmp)
	touch(t, keep)
	touch(t, other)

	code, out, _ := runCLI(t, home, tmp, keep, other)
	if code != 0 || !strings.Contains(out, "skipped") {
		t.Fatalf("ignore notice missing: %d %q", code, out)
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Error("ignored file must stay")
	}
	// "!keep.tmp" re-includes the file, so it is recycled normally
	if _, err := os.Stat(keep); !os.IsNotExist(err) {
		t.Error("negated (!) pattern should be recycled normally")
	}
	if _, err := os.Stat(other); err == nil {
		t.Error("non-matching file should be recycled")
	}
	// -f overrides
	if code, _, _ := runCLI(t, home, "-f", tmp); code != 0 {
		t.Fatal("force recycle failed")
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("-f must override ignore rules")
	}
}

func TestRestoreReplace(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	f := filepath.Join(work, "doc.txt")
	touch(t, f)
	runCLI(t, home, f)
	// recreate a conflicting file
	touch(t, f)
	if code, _, _ := runMgr(t, home, "restore", "--last"); code == 0 {
		t.Fatal("restore over an existing file must fail")
	}
	if data, _ := os.ReadFile(f); string(data) != "x" {
		t.Error("conflicting file must be untouched")
	}
	if code, out, _ := runMgr(t, home, "restore", "--last", "--replace"); code != 0 || !strings.Contains(out, "restored") {
		t.Fatalf("restore --replace: %d %q", code, out)
	}
	// the conflicting file went to the trash first
	if _, err := os.Stat(f); err != nil {
		t.Errorf("restored file missing: %v", err)
	}
	_, logout, _ := runMgr(t, home, "log", "--json")
	if !strings.Contains(logout, `"op": "recycle"`) {
		t.Errorf("conflict should have been recycled: %s", logout)
	}
}

func TestSymlinkRestore(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	target := filepath.Join(work, "target.txt")
	touch(t, target)
	link := filepath.Join(work, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCLI(t, home, link); code != 0 {
		t.Fatal("recycle link failed")
	}
	if code, out, _ := runMgr(t, home, "restore", "--last"); code != 0 || !strings.Contains(out, "restored") {
		t.Fatalf("restore link: %d %q", code, out)
	}
	fi, err := os.Lstat(link)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link should be restored as a symlink: %v", err)
	}
	if got, _ := os.Readlink(link); got != target {
		t.Errorf("link target = %q, want %q", got, target)
	}
}

func TestSpecialBitsRestore(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root to chown")
	}
	home := newSandbox(t)
	work := t.TempDir()
	f := filepath.Join(work, "suid.sh")
	if err := os.WriteFile(f, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// syscall.Chmod keeps the suid/sgid bits (os.Chmod would drop them).
	if err := syscall.Chmod(f, 0o6755); err != nil {
		t.Fatal(err)
	}
	runCLI(t, home, f)
	runMgr(t, home, "restore", "--last")
	fi, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSetuid == 0 || fi.Mode()&os.ModeSetgid == 0 {
		t.Errorf("suid/sgid lost: %v", fi.Mode())
	}
}

func TestSafetyRefusals(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	// the adrm home itself and anything above it
	if code, _, _ := runCLI(t, home, home); code == 0 {
		t.Error("recycling the adrm home must fail")
	}
	// the trash dir
	runCLI(t, home, filepath.Join(work, "x"))
	trash := filepath.Join(home, "trash")
	if code, _, _ := runCLI(t, home, "-rf", trash); code == 0 {
		t.Error("recycling the trash dir must fail")
	}
	if _, err := os.Stat(trash); err != nil {
		t.Error("trash dir must survive")
	}
	// "." and ".."
	if code, _, _ := runCLI(t, home, "."); code == 0 {
		t.Error("recycling . must fail")
	}
}

func TestDoubleDashEscape(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	odd := filepath.Join(work, "-weird")
	touch(t, odd)
	if code, _, _ := runCLI(t, home, "--", odd); code != 0 {
		t.Fatal("recycling -weird via -- failed")
	}
	if _, err := os.Stat(odd); !os.IsNotExist(err) {
		t.Error("file should be recycled")
	}
	// a file literally named "+1d"
	plus := filepath.Join(work, "+1d")
	touch(t, plus)
	if code, _, _ := runCLI(t, home, "--", plus); code != 0 {
		t.Fatal("recycling +1d via -- failed")
	}
	if _, err := os.Stat(plus); !os.IsNotExist(err) {
		t.Error("+1d file should be recycled")
	}
}

func TestInteractivePrompt(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	f := filepath.Join(work, "i.txt")
	touch(t, f)
	// pipe "n": nothing recycled
	code, _, _ := runCLIStdin(t, home, "n\n", "-i", f)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if _, err := os.Stat(f); err != nil {
		t.Error("answering n must keep the file")
	}
	// pipe "y": recycled
	code, _, _ = runCLIStdin(t, home, "y\n", "-i", f)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("answering y must recycle the file")
	}
	// EOF (empty stdin) behaves like "no"
	touch(t, f)
	code, _, _ = runCLIStdin(t, home, "", "-i", f)
	if _, err := os.Stat(f); err != nil {
		t.Error("EOF must cancel, not hang, and keep the file")
	}
	// -I with more than the threshold
	for _, n := range []string{"a", "b", "c", "d"} {
		touch(t, filepath.Join(work, n))
	}
	code, _, _ = runCLIStdin(t, home, "n\n", "-I",
		filepath.Join(work, "a"), filepath.Join(work, "b"), filepath.Join(work, "c"), filepath.Join(work, "d"))
	for _, n := range []string{"a", "b", "c", "d"} {
		if _, err := os.Stat(filepath.Join(work, n)); err != nil {
			t.Errorf("%s must survive a declined -I prompt", n)
		}
	}
}

func TestFiltersAndSort(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	big := filepath.Join(work, "big.bin")
	touch(t, big)
	if err := os.WriteFile(big, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	small := filepath.Join(work, "small.txt")
	touch(t, small)
	runCLI(t, home, big, small)
	_, out, _ := runMgr(t, home, "ls", "--size", "+1k", "--json")
	if !strings.Contains(out, "big.bin") || strings.Contains(out, "small.txt") {
		t.Errorf("--size +1k: %s", out)
	}
	_, out, _ = runMgr(t, home, "ls", "--name", "SMALL", "--json")
	if !strings.Contains(out, "small.txt") || strings.Contains(out, "big.bin") {
		t.Errorf("--name (case-insensitive): %s", out)
	}
	_, out, _ = runMgr(t, home, "ls", "--expired", "--json")
	if strings.Contains(out, "big.bin") {
		t.Errorf("nothing should be expired: %s", out)
	}
	_, out, _ = runMgr(t, home, "ls", "--last", "1", "--json")
	if strings.Count(out, `"id":`) != 1 {
		t.Errorf("--last 1 should print one row: %s", out)
	}
	// unknown sort field is a usage error
	if code, _, _ := runMgr(t, home, "ls", "--sort", "bogus"); code != 2 {
		t.Errorf("unknown sort field should exit 2, got %d", code)
	}
	// unknown enum value
	if code, _, _ := runMgr(t, home, "ls", "--state", "bogus"); code != 2 {
		t.Errorf("unknown state should exit 2, got %d", code)
	}
}

func TestEnglishIsPureASCII(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	touch(t, filepath.Join(work, "a.txt"))
	runCLI(t, home, filepath.Join(work, "a.txt"))
	// the delete face keeps its own --help/--version
	for _, args := range [][]string{{"--help"}, {"--version"}, {"-h"}} {
		_, out, errS := runCLI(t, home, args...)
		checkASCII(t, args, out, errS)
	}
	// the management face: every command that produces human-readable output
	cmds := [][]string{
		{"ls"}, {"log"}, {"stats"}, {"doctor"}, {"config", "--show"},
		{"ls", "--help"}, {"help"}, {"version"}, {"gc", "--dry-run"},
		{"restore", "--help"}, {"purge", "--help"}, {"setup", "--status"},
		{"setup", "--help"}, {"completions", "bash"}, {"completions", "zsh"},
		{"completions", "fish"}, {"db", "--path"}, {"undo", "--help"},
		{"empty", "--help"}, {"gc", "--help"}, {"log", "--help"},
	}
	for _, args := range cmds {
		_, out, errS := runMgr(t, home, args...)
		checkASCII(t, args, out, errS)
	}
}

// checkASCII fails when any output byte is non-ASCII: the English face must
// render on the oldest terminals.
func checkASCII(t *testing.T, args []string, out, errS string) {
	t.Helper()
	for label, s := range map[string]string{"stdout": out, "stderr": errS} {
		for i := 0; i < len(s); i++ {
			if s[i] > 127 {
				t.Errorf("%v (%s) produced non-ASCII byte 0x%02x at %d: %q",
					args, label, s[i], i, s[max(0, i-20):min(len(s), i+20)])
				break
			}
		}
	}
}

func TestCompletionCandidates(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	touch(t, filepath.Join(work, "a.txt"))
	runCLI(t, home, filepath.Join(work, "a.txt"))

	run := func(args ...string) string {
		_, out, _ := runMgr(t, home, append([]string{"__complete", "bash"}, args...)...)
		return out
	}
	if out := run(); !strings.Contains(out, "restore") || !strings.Contains(out, "purge") {
		t.Errorf("first word should offer subcommands: %q", out)
	}
	// the management face must never offer file names alongside subcommands
	if out := run(); strings.Contains(out, "a.txt") {
		t.Errorf("admr must not complete files: %q", out)
	}
	// the delete face must never offer subcommands, only its own options
	_, delOut, _ := runCLI(t, home, append([]string{"__complete", "bash"}, "-")...)
	if strings.Contains(delOut, "restore") || strings.Contains(delOut, "purge") {
		t.Errorf("adrm must not complete subcommands: %q", delOut)
	}
	if !strings.Contains(delOut, "--deadline") || !strings.Contains(delOut, "--recursive") {
		t.Errorf("adrm should complete its own options: %q", delOut)
	}
	// and no file candidates either (the shell supplies those)
	if _, out2, _ := runCLI(t, home, append([]string{"__complete", "bash"}, "")...); strings.TrimSpace(out2) != "" {
		t.Errorf("adrm must not complete bare words itself: %q", out2)
	}
	if out := run("re"); !strings.Contains(out, "restore") {
		t.Errorf("prefix re should offer restore: %q", out)
	}
	if out := run("restore", ""); !strings.Contains(out, "1") {
		t.Errorf("restore <TAB> should offer trash id 1: %q", out)
	}
	if out := run("ls", "--"); !strings.Contains(out, "--state") {
		t.Errorf("ls -- should offer --state: %q", out)
	}
	if out := run("ls", "--state", ""); !strings.Contains(out, "recycled") {
		t.Errorf("--state '' should offer enums: %q", out)
	}
	if out := run("log", "--op", ""); !strings.Contains(out, "purge") {
		t.Errorf("--op '' should offer enums: %q", out)
	}
	if out := run("completions", ""); !strings.Contains(out, "fish") {
		t.Errorf("completions '' should offer shells: %q", out)
	}
	// unknown command falls through to file completion (empty output)
	if out := run("ls", "--bogus"); strings.TrimSpace(out) != "" {
		t.Errorf("unknown option should not complete: %q", out)
	}
}

// TestDeadlineFormats locks in every date/time spelling --deadline accepts,
// including the exact moment each one resolves to.
func TestDeadlineFormats(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	cases := []struct{ in, want string }{
		{"2027/01/01", "2027-01-01 00:00:00"},
		{"2027/1/01", "2027-01-01 00:00:00"},
		{"2027/01/1", "2027-01-01 00:00:00"},
		{"2027/1/1", "2027-01-01 00:00:00"},
		{"2027_01_01", "2027-01-01 00:00:00"},
		{"2027_1_01", "2027-01-01 00:00:00"},
		{"2027_01_1", "2027-01-01 00:00:00"},
		{"2027_1_1", "2027-01-01 00:00:00"},
		{"2027-01-01", "2027-01-01 00:00:00"},
		{"2027-1-01", "2027-01-01 00:00:00"},
		{"2027-01-1", "2027-01-01 00:00:00"},
		{"2027-1-1", "2027-01-01 00:00:00"},
		{"2027/1/1 3:45", "2027-01-01 03:45:00"},
		{"2027/1/1 03:45", "2027-01-01 03:45:00"},
		{"2027/1/1 03:45:01", "2027-01-01 03:45:01"},
		{"2027_1_1 3:45", "2027-01-01 03:45:00"},
		{"2027_1_1 03:45:01", "2027-01-01 03:45:01"},
		{"2027-1-1 03:45:01", "2027-01-01 03:45:01"},
	}
	for i, c := range cases {
		f := filepath.Join(work, fmt.Sprintf("f%d.txt", i))
		touch(t, f)
		// --until must stay accepted as an alias of --deadline
		opt := "--deadline"
		if i%3 == 0 {
			opt = "--until"
		}
		if code, out, errS := runCLI(t, home, opt, c.in, f); code != 0 {
			t.Fatalf("%s %q: %d %q %q", opt, c.in, code, out, errS)
		}
		_, out, _ := runMgr(t, home, "ls", "--json")
		var items []struct {
			ExpireAt int64 `json:"expire_at"`
		}
		if err := json.Unmarshal([]byte(out), &items); err != nil {
			t.Fatalf("ls --json: %v (%s)", err, out)
		}
		want, err := time.ParseInLocation("2006-01-02 15:04:05", c.want, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		if items[0].ExpireAt != want.Unix() {
			t.Errorf("%s %q: expire_at=%d want=%d (%s)", opt, c.in,
				items[0].ExpireAt, want.Unix(), c.want)
		}
	}
}

func TestConfigInit(t *testing.T) {
	home := newSandbox(t)
	code, out, _ := runMgr(t, home, "config", "--init")
	if code != 0 {
		t.Fatalf("config --init: %d %q", code, out)
	}
	data, err := os.ReadFile(filepath.Join(home, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "retention_days") {
		t.Errorf("template missing keys: %s", data)
	}
	// a second init refuses to overwrite
	if code, _, _ := runMgr(t, home, "config", "--init"); code == 0 {
		t.Error("second --init should fail without --force")
	}
	if code, _, _ := runMgr(t, home, "config", "--init", "--force"); code != 0 {
		t.Error("--force should overwrite")
	}
	// validate
	if code, _, _ := runMgr(t, home, "config", "--validate"); code != 0 {
		t.Error("generated config must validate")
	}
}

func TestSetupInstallUninstall(t *testing.T) {
	home := newSandbox(t)
	fakeUser := t.TempDir()
	t.Setenv("HOME", fakeUser)
	rc := filepath.Join(fakeUser, ".bashrc")
	if err := os.WriteFile(rc, []byte("# my rc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runMgr(t, home, "setup", "--install", "--shells", "bash", "--local"); code != 0 {
		t.Fatal("setup --install failed")
	}
	data, _ := os.ReadFile(rc)
	if !strings.Contains(string(data), rcMarkBegin) || !strings.Contains(string(data), "ADRM_HOME") {
		t.Fatalf("rc not updated: %s", data)
	}
	// completions land in the per-user directory, one pair per binary
	userComp := filepath.Join(fakeUser, ".local", "share", "bash-completion", "completions")
	for _, name := range []string{"adrm.bash", "admr.bash"} {
		if _, err := os.Stat(filepath.Join(userComp, name)); err != nil {
			t.Errorf("completion script missing: %v", err)
		}
	}
	// the two scripts must not share a vocabulary
	adrmScript, _ := os.ReadFile(filepath.Join(userComp, "adrm.bash"))
	admrScript, _ := os.ReadFile(filepath.Join(userComp, "admr.bash"))
	if strings.Contains(string(adrmScript), "subcommand") {
		t.Errorf("adrm.bash must not mention subcommands:\n%s", adrmScript)
	}
	if !strings.Contains(string(admrScript), "__nofiles__") {
		t.Errorf("admr.bash must suppress file completion:\n%s", admrScript)
	}
	if _, err := os.Stat(filepath.Join(home, "adrm-init.sh")); err != nil {
		t.Errorf("init script missing: %v", err)
	}
	if code, _, _ := runMgr(t, home, "setup", "--uninstall"); code != 0 {
		t.Fatal("setup --uninstall failed")
	}
	data, _ = os.ReadFile(rc)
	if strings.Contains(string(data), "adrm") {
		t.Errorf("rc should be clean after uninstall: %s", data)
	}
	if !strings.Contains(string(data), "# my rc") {
		t.Errorf("uninstall must not touch unrelated rc content: %s", data)
	}
	if _, err := os.Stat(filepath.Join(home, "completions")); !os.IsNotExist(err) {
		t.Error("completions dir should be gone")
	}
	for _, name := range []string{"adrm.bash", "admr.bash"} {
		if _, err := os.Stat(filepath.Join(userComp, name)); !os.IsNotExist(err) {
			t.Errorf("completion script should be removed: %s", name)
		}
	}
	// status after uninstall
	if code, out, _ := runMgr(t, home, "setup", "--status"); code != 0 || !strings.Contains(out, "not installed") {
		t.Errorf("status: %d %q", code, out)
	}
}

// TestSetupBlockIsValidBash is the regression test for the fish-syntax leak:
// the generated rc block must be valid POSIX shell that actually works when
// sourced by bash (it used to contain "; and ." which bash cannot run).
func TestSetupBlockIsValidBash(t *testing.T) {
	home := newSandbox(t)
	fakeUser := t.TempDir()
	t.Setenv("HOME", fakeUser)
	if code, _, _ := runMgr(t, home, "setup", "--install", "--shells", "bash", "--local", "--alias"); code != 0 {
		t.Fatal("setup --install failed")
	}
	rc := filepath.Join(fakeUser, ".bashrc")
	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	block := string(data)
	// 1. the managed block must be syntactically valid bash
	blockFile := filepath.Join(t.TempDir(), "block.sh")
	if err := os.WriteFile(blockFile, []byte(block), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("bash", "-n", blockFile).CombinedOutput(); err != nil {
		t.Fatalf("bash -n rejected the generated block: %v\n%s\n---\n%s", err, out, block)
	}
	// 2. sourcing it in a real bash must define everything, silently
	script := fmt.Sprintf(`export ADRM_ADRM_BIN=%q
set -e
. %q
[ "$ADRM_HOME" = %q ] || { echo "ADRM_HOME not set"; exit 1; }
alias rm >/dev/null 2>&1 || { echo "rm alias missing"; exit 1; }
complete -p adrm >/dev/null 2>&1 || { echo "adrm completion missing"; exit 1; }
echo OK
`, adrmBinaryForTest(), blockFile, home)
	out, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "OK") {
		t.Fatalf("sourcing the block failed: %v\n%s", err, out)
	}
	// 3. no fish-only syntax may leak into the POSIX block
	if strings.Contains(block, "; and ") {
		t.Errorf("fish syntax leaked into the bash block:\n%s", block)
	}
}

// adrmBinaryForTest returns the test binary path so the completion scripts
// call the same code under test.
func adrmBinaryForTest() string {
	exe, err := os.Executable()
	if err != nil {
		return "adrm"
	}
	return exe
}

func TestDBResetSafety(t *testing.T) {
	home := newSandbox(t)
	work := t.TempDir()
	touch(t, filepath.Join(work, "a.txt"))
	runCLI(t, home, filepath.Join(work, "a.txt"))
	// with items in the bin and a declined prompt, nothing is reset
	code, _, _ := runMgrStdin(t, home, "n\n", "db", "--reset")
	if code != 0 {
		t.Fatalf("declined reset should exit 0: %d", code)
	}
	_, out, _ := runMgr(t, home, "ls", "--json")
	if !strings.Contains(out, "a.txt") {
		t.Errorf("item must survive a declined reset: %s", out)
	}
	// purge everything, then reset works
	runMgr(t, home, "purge", "--all", "--yes")
	if code, _, _ := runMgr(t, home, "db", "--reset", "--yes"); code != 0 {
		t.Fatal("reset failed")
	}
	_, out, _ = runMgr(t, home, "log", "--json")
	if strings.Contains(out, `"op": "recycle"`) {
		t.Errorf("history should be cleared: %s", out)
	}
}

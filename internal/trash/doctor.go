package trash

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/model"
)

// DoctorReport is the result of a self-check.
type DoctorReport struct {
	Checks    []DoctorCheck
	Orphans   []string // files in the trash directory not referenced by the db
	Missing   []string // db rows whose trash copy vanished
	SizeSkips []string // db rows whose recorded size differs from disk
}

// DoctorCheck is one named check result.
type DoctorCheck struct {
	Name   string
	OK     bool
	Detail string
}

// Add appends a check.
func (r *DoctorReport) Add(name string, ok bool, format string, args ...any) {
	r.Checks = append(r.Checks, DoctorCheck{Name: name, OK: ok, Detail: fmt.Sprintf(format, args...)})
}

// Doctor verifies that the database, the trash directory and the shell
// integration all agree, and finds orphaned files.
func (e *Engine) Doctor() (*DoctorReport, error) {
	rep := &DoctorReport{}
	// 1. database integrity
	if err := e.St.Check(); err != nil {
		rep.Add("database integrity", false, "%v", err)
	} else {
		rep.Add("database integrity", true, "%s", e.Cfg.Database)
	}
	// 2. every bin row must exist on disk
	items, err := e.St.ListBin()
	if err != nil {
		return rep, err
	}
	live := map[string]bool{}
	for i := range items {
		it := items[i]
		live[it.TrashPath] = true
		if _, err := os.Lstat(it.TrashPath); err != nil {
			rep.Missing = append(rep.Missing, fmt.Sprintf("#%d %s (%s)", it.ID, it.OrigPath, it.TrashPath))
		} else if m, err := captureSize(it.TrashPath); err == nil && m != it.Size {
			rep.SizeSkips = append(rep.SizeSkips, fmt.Sprintf("#%d %s: recorded %s, on disk %s",
				it.ID, it.OrigPath, model.FormatSize(it.Size), model.FormatSize(m)))
		}
	}
	if len(rep.Missing) == 0 {
		rep.Add("trash copies", true, "%d item(s) present", len(items))
	} else {
		rep.Add("trash copies", false, "%d item(s) missing from disk", len(rep.Missing))
	}
	// 3. orphan files in the trash directory
	referenced := live
	err = filepath.WalkDir(e.Cfg.TrashDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !referenced[p] {
			rep.Orphans = append(rep.Orphans, p)
		}
		return nil
	})
	if err != nil {
		rep.Add("trash directory", false, "%v", err)
	} else {
		rep.Add("trash directory", true, "%s", e.Cfg.TrashDir)
	}
	if len(rep.Orphans) == 0 {
		rep.Add("orphans", true, "none")
	} else {
		rep.Add("orphans", false, "%d file(s) not tracked by the database", len(rep.Orphans))
	}
	if len(rep.SizeSkips) == 0 {
		rep.Add("recorded sizes", true, "match")
	} else {
		rep.Add("recorded sizes", false, "%d mismatch(es)", len(rep.SizeSkips))
	}
	// 4. home directory permissions
	fi, err := os.Stat(e.Cfg.Home)
	if err != nil {
		rep.Add("home directory", false, "%v", err)
	} else if fi.Mode().Perm() != 0o700 {
		rep.Add("home directory", false, "%s has permissions %04o (want 0700)", e.Cfg.Home, fi.Mode().Perm())
	} else {
		rep.Add("home directory", true, "%s (0700)", e.Cfg.Home)
	}
	// 5. config file
	if _, err := os.Stat(e.Cfg.ConfigFile); err != nil {
		rep.Add("config file", true, "not present, using defaults ('adrm config --init' to create)")
	} else {
		rep.Add("config file", true, "%s", e.Cfg.ConfigFile)
	}
	return rep, nil
}

// Print renders a report.
func (r *DoctorReport) Print(print func(string, ...any)) {
	okMark, badMark := "[ok]", "[!!]"
	print(i18n.Tr("adrm 自检报告:\n", "adrm self-check:\n"))
	for _, c := range r.Checks {
		mark := okMark
		if !c.OK {
			mark = badMark
		}
		print("  %s %-18s %s\n", mark, c.Name, c.Detail)
	}
	if len(r.Missing) > 0 {
		print(i18n.Tr("  数据库中记录但磁盘上不存在的项:\n", "  items recorded in the database but missing on disk:\n"))
		for _, m := range r.Missing {
			print("    %s\n", m)
		}
		print(i18n.Tr("  这些记录会被 'adrm purge <id>' 清掉, 或用 'adrm db --reset' 重置。\n",
			"  remove them with 'adrm purge <id>' or reset everything with 'adrm db --reset'.\n"))
	}
	if len(r.SizeSkips) > 0 {
		print(i18n.Tr("  大小不匹配的记录(仅供参考):\n", "  size mismatches (informational):\n"))
		for _, m := range r.SizeSkips {
			print("    %s\n", m)
		}
	}
}

// PurgeOrphans removes orphan files found by Doctor, recording each removal in
// the reflog.
func (e *Engine) PurgeOrphans(orphans []string) error {
	var failures []string
	for _, p := range orphans {
		if err := os.RemoveAll(p); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		_ = e.St.AppendReflog(model.Reflog{
			Ts: e.Now(), Op: model.OpPurge, TrashPath: p, Detail: "orphan cleanup via doctor",
		})
	}
	sort.Strings(orphans)
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func captureSize(p string) (int64, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	if fi.IsDir() {
		return dirSizeOf(p)
	}
	return fi.Size(), nil
}

func dirSizeOf(root string) (int64, error) {
	var total int64
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total, nil
}

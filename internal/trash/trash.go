// Package trash implements the recycle engine: moving files into the trash
// batch directory, restoring them with their original attributes, purging
// them for real, and the lazy garbage collector for expired items.
package trash

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wubinstu/adrm/internal/config"
	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/ignore"
	"github.com/wubinstu/adrm/internal/model"
	"github.com/wubinstu/adrm/internal/store"
	"github.com/wubinstu/adrm/internal/x"
)

// Engine holds everything needed to run trash operations.
type Engine struct {
	Cfg  *config.Config
	St   *store.Store
	Ig   *ignore.Matcher
	In   io.Reader
	Out  io.Writer
	Err  io.Writer
	Now  func() int64
	Lang string
}

// New builds an engine. The ignore matcher is loaded from the config.
func New(cfg *config.Config, st *store.Store) (*Engine, error) {
	ig, err := ignore.LoadFile(cfg.Home, cfg.IgnoreFile)
	if err != nil {
		return nil, fmt.Errorf("cannot load ignore file %s: %w", cfg.IgnoreFile, err)
	}
	return &Engine{Cfg: cfg, St: st, Ig: ig, Now: time.Now().Unix}, nil
}

// Target is one recycle operand: a path plus the moment it expires.
type Target struct {
	Path     string
	ExpireAt int64
}

// RecycleOpts mirrors the rm-compatible flags.
type RecycleOpts struct {
	Force          bool
	Interactive    bool // -i
	InteractiveN   bool // -I
	Verbose        bool
	Recursive      bool
	Dir            bool // -d
	DryRun         bool
	Yes            bool
	NoPreserveRoot bool
}

func (e *Engine) printf(format string, args ...any) {
	fmt.Fprintf(e.Out, format, args...)
}

func (e *Engine) errorf(format string, args ...any) {
	fmt.Fprintf(e.Err, format, args...)
}

// Confirm asks a yes/no question. A non-tty stdin is still read (so
// `echo n | adrm -I ...` works); EOF means "no" and never hangs.
func (e *Engine) Confirm(question string, def bool) bool {
	e.errorf("%s ", question)
	br := bufio.NewReader(e.In)
	line, err := br.ReadString('\n')
	if err != nil && line == "" {
		e.errorf("<eof>\n")
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	case "":
		return def
	}
	return false
}

// Recycle is the main entry point of the default (rm-compatible) mode.
// All targets are validated before anything is moved: a malformed command
// never touches the file system.
func (e *Engine) Recycle(targets []Target, opt RecycleOpts) error {
	if len(targets) == 0 {
		return fmt.Errorf("%s", i18n.Tr("没有指定要回收的文件", "no files given"))
	}
	now := e.Now()

	// ---- phase 1: validate everything ----
	type planItem struct {
		path     string // absolute cleaned path
		meta     *x.Meta
		expireAt int64
	}
	var plans []planItem
	var ignored []string
	var failed []string

	for _, t := range targets {
		if c := filepath.Clean(t.Path); c == "." || c == ".." {
			failed = append(failed, fmt.Sprintf(i18n.Tr("拒绝回收 '%s': 非法路径(rm 同样拒绝)", "refusing to remove '%s': illegal path (rm refuses too)"), t.Path))
			continue
		}
		p, err := filepath.Abs(t.Path)
		if err != nil {
			failed = append(failed, t.Path+": "+err.Error())
			continue
		}
		p = filepath.Clean(p)
		if err := e.checkSafety(p, opt); err != nil {
			failed = append(failed, err.Error())
			continue
		}
		fi, err := os.Lstat(p)
		if err != nil {
			if os.IsNotExist(err) {
				if opt.Force {
					continue // rm -f: silent success
				}
				failed = append(failed, fmt.Sprintf(i18n.Tr("无法回收 '%s': 文件不存在", "cannot remove '%s': No such file or directory"), p))
				continue
			}
			failed = append(failed, fmt.Sprintf(i18n.Tr("无法回收 '%s': %v", "cannot remove '%s': %v"), p, err))
			continue
		}
		isDir := fi.IsDir()
		if isDir && !opt.Recursive && !opt.Dir {
			failed = append(failed, fmt.Sprintf(i18n.Tr("无法回收 '%s': 是一个目录(需要 -r)", "cannot remove '%s': Is a directory (use -r)"), p))
			continue
		}
		if isDir && opt.Dir && !opt.Recursive {
			if entries, err := os.ReadDir(p); err == nil && len(entries) > 0 {
				failed = append(failed, fmt.Sprintf(i18n.Tr("无法回收 '%s': 目录非空(需要 -r)", "cannot remove '%s': Directory not empty (use -r)"), p))
				continue
			}
		}
		// ignore rules
		if e.Ig.Match(p, isDir) && !opt.Force {
			ignored = append(ignored, p)
			continue
		}
		// write permission on the parent directory
		if !opt.Force && !x.WritableDir(filepath.Dir(p)) {
			if !e.Confirm(fmt.Sprintf(i18n.Tr("remove write-protected %s '%s'?", "remove write-protected %s '%s'?"), kindOf(fi), p), false) {
				failed = append(failed, fmt.Sprintf(i18n.Tr("已取消: '%s'", "cancelled: '%s'"), p))
				continue
			}
		}
		meta, err := x.Capture(p)
		if err != nil {
			failed = append(failed, fmt.Sprintf(i18n.Tr("无法读取 '%s' 的元数据: %v", "cannot read metadata of '%s': %v"), p, err))
			continue
		}
		plans = append(plans, planItem{path: p, meta: meta, expireAt: t.ExpireAt})
	}

	for _, p := range ignored {
		e.printf(i18n.Tr("已跳过(命中 ignore 规则): %s  (用 -f 强制执行)\n", "skipped (matches ignore rule): %s  (use -f to override)\n"), p)
	}
	if len(failed) > 0 {
		return &OpError{Failures: failed}
	}
	if len(plans) == 0 {
		if len(ignored) > 0 {
			return nil
		}
		return nil
	}

	// ---- phase 2: interactive prompts ----
	if opt.Interactive {
		var kept []planItem
		for _, pl := range plans {
			if !e.Confirm(fmt.Sprintf(i18n.Tr("remove %s '%s'?", "remove %s '%s'?"), kindOfPath(pl.path), pl.path), false) {
				continue
			}
			kept = append(kept, pl)
		}
		plans = kept
		if len(plans) == 0 {
			fmt.Fprint(e.Out, i18n.Tr("已取消, 没有回收任何文件。\n", "cancelled, nothing recycled.\n"))
			return nil
		}
	} else if opt.InteractiveN && len(plans) > e.Cfg.PromptThreshold {
		if !e.Confirm(fmt.Sprintf(i18n.Tr("remove %d arguments?", "remove %d arguments?"), len(plans)), false) {
			fmt.Fprint(e.Out, i18n.Tr("已取消, 没有回收任何文件。\n", "cancelled, nothing recycled.\n"))
			return nil
		}
	}

	// ---- phase 3: dry run ----
	if opt.DryRun {
		fmt.Fprint(e.Out, i18n.Tr("演练模式, 不会实际操作:\n", "dry run, nothing will be changed:\n"))
		for _, pl := range plans {
			e.printf("  %s  (%s, %s)\n", pl.path, model.FormatSize(pl.meta.Size),
				time.Unix(pl.expireAt, 0).Local().Format(e.Cfg.DateFormat))
		}
		return nil
	}

	// ---- phase 4: move files into a fresh batch directory ----
	batchDir, err := e.newBatchDir(now)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tr("无法创建回收站批次目录: %v", "cannot create trash batch dir: %v", err))
	}
	items := make([]model.Item, 0, len(plans))
	var problems []string
	for _, pl := range plans {
		dst := uniquePath(batchDir, filepath.Base(pl.path))
		method, err := x.Move(pl.path, dst, e.Cfg.CopyFallback)
		if err != nil {
			problems = append(problems, fmt.Sprintf(i18n.Tr("回收 '%s' 失败: %v", "recycling '%s' failed: %v"), pl.path, err))
			continue
		}
		owner, group := x.LookupOwner(pl.meta.UID, pl.meta.GID)
		items = append(items, model.Item{
			OrigPath: pl.path, IsDir: pl.meta.IsDir, Size: pl.meta.Size, Mode: pl.meta.Mode,
			UID: pl.meta.UID, GID: pl.meta.GID, Owner: owner, Group: group,
			Mtime: pl.meta.Mtime, TrashPath: dst, RecycledAt: now, ExpireAt: pl.expireAt,
			Method: method,
		})
	}
	if len(items) == 0 {
		if len(problems) > 0 {
			return &OpError{Failures: problems}
		}
		return nil
	}
	if err := e.St.RecycleItems(items); err != nil {
		// The files are inside the trash but unknown to the database; do not
		// lose them silently: keep them and tell the user.
		return fmt.Errorf("%s", i18n.Tr("回收了文件但写入数据库失败: %v\n  可用 'adrm doctor' 检查回收站中的孤立项", "recycled files but failed to record them: %v\n  run 'adrm doctor' to inspect orphans", err))
	}
	for _, it := range items {
		if opt.Verbose {
			e.printf(i18n.Tr("已回收 '%s' -> %s\n", "recycled '%s' -> %s\n"), it.OrigPath, it.TrashPath)
		}
		e.printf(i18n.Tr("已回收 '%s' (id=%d, %s, %s)\n", "recycled '%s' (id=%d, %s, %s)\n"),
			it.OrigPath, it.ID, model.FormatSize(it.Size), leftIn(it.ExpireAt, now))
	}
	// opportunistic garbage collection of already-expired items
	if e.Cfg.AutoGC && !opt.DryRun {
		if n, _ := e.GC(GCOpts{Quiet: false, Yes: true, Auto: true}); n > 0 {
			e.printf(i18n.Tr("(顺手清理了 %d 个过期项, 见 'adrm log --op expire')\n", "(auto-purged %d expired item(s); see 'adrm log --op expire')\n"), n)
		}
	}
	if len(problems) > 0 {
		return &OpError{Failures: problems}
	}
	return nil
}

// checkSafety refuses operations that would destroy adrm itself or the system.
func (e *Engine) checkSafety(p string, opt RecycleOpts) error {
	if p == "/" {
		if opt.NoPreserveRoot || !e.Cfg.PreserveRoot {
			return nil // explicit opt-in (CLI flag or config)
		}
		return fmt.Errorf("%s", i18n.Tr("拒绝回收 '/': 使用 --no-preserve-root 强制(极度危险!)", "refusing to remove '/': use --no-preserve-root at your own risk"))
	}
	if isAncestorOrSame(p, e.Cfg.TrashDir) {
		return fmt.Errorf("%s", i18n.Tr("拒绝回收 '%s': 它在回收站内部", "refusing to remove '%s': it is inside the trash directory", p))
	}
	if isAncestorOrSame(p, e.Cfg.Home) {
		return fmt.Errorf("%s", i18n.Tr("拒绝回收 '%s': 它是 adrm 主目录或其父目录", "refusing to remove '%s': it is the adrm home or one of its parents", p))
	}
	return nil
}

func isAncestorOrSame(path, dir string) bool {
	if path == dir {
		return true
	}
	return strings.HasPrefix(dir, path+string(os.PathSeparator))
}

// newBatchDir creates a chronologically named, unique batch directory such as
// trash/20260505-134426_000123. The id suffix keeps names unique even within
// the same second and across restarts.
func (e *Engine) newBatchDir(now int64) (string, error) {
	if err := os.MkdirAll(e.Cfg.TrashDir, 0o700); err != nil {
		return "", err
	}
	maxID, _ := e.St.MaxBinID()
	base := time.Unix(now, 0).Local().Format("20060102-150405")
	for n := maxID + 1; ; n++ {
		dir := filepath.Join(e.Cfg.TrashDir, fmt.Sprintf("%s_%06d", base, n))
		if _, err := os.Lstat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o700); err == nil {
				return dir, nil
			}
		}
	}
}

// uniquePath returns a non-existing path inside dir that keeps the original
// base name visible ("report.pdf", "report.pdf.1", ...).
func uniquePath(dir, name string) string {
	if name == "" || name == "." || name == ".." {
		name = "unnamed"
	}
	p := filepath.Join(dir, name)
	if _, err := os.Lstat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s.%d%s", stem, i, ext))
		if _, err := os.Lstat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

func kindOf(fi os.FileInfo) string {
	mode := fi.Mode()
	switch {
	case mode&os.ModeDir != 0:
		return i18n.Tr("directory", "directory")
	case mode&os.ModeSymlink != 0:
		return i18n.Tr("symlink", "symlink")
	case mode&os.ModeDevice != 0:
		return i18n.Tr("device", "device")
	case mode&os.ModeNamedPipe != 0:
		return i18n.Tr("fifo", "fifo")
	case mode&os.ModeSocket != 0:
		return i18n.Tr("socket", "socket")
	case mode&0100 != 0:
		return i18n.Tr("executable", "executable")
	default:
		return i18n.Tr("regular file", "regular file")
	}
}

func kindOfPath(p string) string {
	fi, err := os.Lstat(p)
	if err != nil {
		return "file"
	}
	return kindOf(fi)
}

func leftIn(expireAt, now int64) string {
	if expireAt <= now {
		return i18n.Tr("已过期", "expired")
	}
	d := expireAt - now
	if d >= 86400 {
		return fmt.Sprintf(i18n.Tr("%d 天后过期", "expires in %dd"), d/86400)
	}
	if d >= 3600 {
		return fmt.Sprintf(i18n.Tr("%d 小时后过期", "expires in %dh"), d/3600)
	}
	return fmt.Sprintf(i18n.Tr("%d 分钟后过期", "expires in %dm"), d/60)
}

// OpError collects per-file failures so a command can report everything that
// went wrong instead of stopping at the first problem.
type OpError struct {
	Failures []string
}

func (o *OpError) Error() string {
	if len(o.Failures) == 1 {
		return o.Failures[0]
	}
	return fmt.Sprintf("%d operations failed:\n  - %s", len(o.Failures), strings.Join(o.Failures, "\n  - "))
}

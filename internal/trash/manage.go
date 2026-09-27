package trash

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/model"
	"github.com/wubinstu/adrm/internal/store"
	"github.com/wubinstu/adrm/internal/x"
)

// RestoreOpts controls restore behavior.
type RestoreOpts struct {
	Replace bool // recycle an existing target file instead of failing
	DryRun  bool
}

// Restore puts items back at their original path with all recorded attributes
// (owner, group, permission bits including suid/sgid/sticky, mtime).
func (e *Engine) Restore(ids []int64, opt RestoreOpts) error {
	now := e.Now()
	var failures []string
	for _, id := range ids {
		it, err := e.St.GetBin(id)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if opt.DryRun {
			e.printf("would restore #%d: %s -> %s\n", it.ID, it.TrashPath, it.OrigPath)
			continue
		}
		if err := e.restoreOne(it, opt, now); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return &OpError{Failures: failures}
	}
	return nil
}

func (e *Engine) restoreOne(it *model.Item, opt RestoreOpts, now int64) error {
	if _, err := os.Lstat(it.TrashPath); err != nil {
		_ = e.St.MarkException(it.ID, fmt.Sprintf("trash copy missing: %v", err))
		return fmt.Errorf("%s", i18n.Tr("还原 #%d 失败: 回收站中的文件不存在 (%s)", "restore #%d failed: the trash copy is missing (%s)", it.ID, it.TrashPath))
	}
	// target conflict
	if _, err := os.Lstat(it.OrigPath); err == nil {
		if !opt.Replace {
			return fmt.Errorf("%s", i18n.Tr("还原 #%d 失败: '%s' 已存在 (用 --replace 先把已存在的文件回收到回收站)", "restore #%d failed: '%s' already exists (use --replace to recycle the existing file first)", it.ID, it.OrigPath))
		}
		if err := e.Recycle([]Target{{Path: it.OrigPath, ExpireAt: now + int64(e.Cfg.RetentionDays)*86400}}, RecycleOpts{Force: true}); err != nil {
			return fmt.Errorf("%s", i18n.Tr("--replace: 回收冲突文件 '%s' 失败: %v", "--replace: recycling the conflicting '%s' failed: %v", it.OrigPath, err))
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("%s", i18n.Tr("还原 #%d 失败: 无法检查目标路径: %v", "restore #%d failed: cannot inspect target: %v", it.ID, err))
	}
	if err := os.MkdirAll(filepath.Dir(it.OrigPath), 0o755); err != nil {
		_ = e.St.MarkException(it.ID, fmt.Sprintf("cannot recreate parent: %v", err))
		return fmt.Errorf("%s", i18n.Tr("还原 #%d 失败: 无法创建父目录: %v", "restore #%d failed: cannot recreate parent directory: %v", it.ID, err))
	}
	isLink := false
	if fi, err := os.Lstat(it.TrashPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		isLink = true
	}
	method, err := x.Move(it.TrashPath, it.OrigPath, e.Cfg.CopyFallback)
	if err != nil {
		_ = e.St.MarkException(it.ID, fmt.Sprintf("move back failed: %v", err))
		return fmt.Errorf("%s", i18n.Tr("还原 #%d 失败: %v", "restore #%d failed: %v", it.ID, err))
	}
	detail := "method=" + method
	if isLink {
		detail += "; symlink"
	}
	if err := x.RestoreMeta(it, isLink); err != nil {
		// The file is back in place; keep the operation successful but warn.
		fmt.Fprint(e.Err, i18n.Tr("警告: #%d 部分属性未能还原: %v\n", "warning: could not fully restore attributes of #%d: %v\n"), it.ID, err)
		detail = detail + "; attributes partially restored: " + err.Error()
	}
	if it.IsDir {
		detail += "; dir"
	}
	if err := e.St.LeaveBin(it.ID, model.OpRestore, now, detail); err != nil {
		return err
	}
	e.printf(i18n.Tr("已还原 '%s' (id=%d)\n", "restored '%s' (id=%d)\n"), it.OrigPath, it.ID)
	return nil
}

// PurgeOpts controls destructive cleanup.
type PurgeOpts struct {
	Yes    bool
	DryRun bool
}

// Purge permanently deletes the given items from the trash.
func (e *Engine) Purge(ids []int64, opt PurgeOpts) error {
	return e.purge(ids, model.OpPurge, opt,
		i18n.Tr("彻底清理 %d 项?", "permanently delete %d item(s)?"),
		i18n.Tr("已取消, 没有清理任何文件。", "cancelled, nothing purged."))
}

func (e *Engine) purge(ids []int64, op string, opt PurgeOpts, question, cancelMsg string) error {
	var items []*model.Item
	var failures []string
	for _, id := range ids {
		it, err := e.St.GetBin(id)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		if len(failures) > 0 {
			return &OpError{Failures: failures}
		}
		return nil
	}
	now := e.Now()
	if opt.DryRun {
		fmt.Fprint(e.Out, i18n.Tr("演练模式, 将彻底删除:\n", "dry run, would permanently delete:\n"))
		for _, it := range items {
			e.printf("  #%d %s (%s)\n", it.ID, it.OrigPath, model.FormatSize(it.Size))
		}
		return nil
	}
	if !opt.Yes {
		e.printf(i18n.Tr("即将彻底删除以下 %d 项(不可恢复):\n", "about to permanently delete %d item(s) (cannot be undone):\n"), len(items))
		for _, it := range items {
			e.printf("  #%d %s\n", it.ID, it.OrigPath)
		}
		if !e.Confirm(fmt.Sprintf(question, len(items)), false) {
			e.printf("%s\n", cancelMsg)
			return nil
		}
	}
	for _, it := range items {
		if err := os.RemoveAll(it.TrashPath); err != nil {
			_ = e.St.MarkException(it.ID, fmt.Sprintf("purge failed: %v", err))
			failures = append(failures, fmt.Sprintf(i18n.Tr("彻底清理 #%d 失败: %v", "purge #%d failed: %v"), it.ID, err))
			continue
		}
		if err := e.St.LeaveBin(it.ID, op, now, ""); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return &OpError{Failures: failures}
	}
	e.printf(i18n.Tr("已彻底清理 %d 项。\n", "purged %d item(s).\n"), len(items))
	return nil
}

// GCOpts controls the expired-item collector.
type GCOpts struct {
	DryRun bool
	Yes    bool
	Quiet  bool
	Auto   bool // triggered implicitly by another command: no confirmation
}

// GC purges every item whose retention deadline has passed. It returns the
// number of purged items.
func (e *Engine) GC(opt GCOpts) (int, error) {
	now := e.Now()
	items, err := e.St.ListBin()
	if err != nil {
		return 0, err
	}
	var expired []*model.Item
	for i := range items {
		if items[i].ExpireAt <= now {
			expired = append(expired, &items[i])
		}
	}
	if len(expired) == 0 {
		if !opt.Quiet {
			fmt.Fprint(e.Out, i18n.Tr("没有过期的回收项。\n", "no expired items.\n"))
		}
		return 0, nil
	}
	if opt.DryRun {
		e.printf(i18n.Tr("演练模式, %d 个过期项将被清理:\n", "dry run, %d expired item(s) would be purged:\n"), len(expired))
		for _, it := range expired {
			e.printf("  #%d %s (%s)\n", it.ID, it.OrigPath, model.FormatSize(it.Size))
		}
		return 0, nil
	}
	if !opt.Yes && !opt.Auto {
		e.printf(i18n.Tr("%d 个回收项已过期:\n", "%d trash item(s) have expired:\n"), len(expired))
		for _, it := range expired {
			e.printf("  #%d %s\n", it.ID, it.OrigPath)
		}
		if !e.Confirm(i18n.Tr("全部彻底清理这 %d 个过期项?", "permanently delete all %d expired item(s)?"), false) {
			fmt.Fprint(e.Out, i18n.Tr("已取消。\n", "cancelled.\n"))
			return 0, nil
		}
	}
	n := 0
	var failures []string
	for _, it := range expired {
		if err := os.RemoveAll(it.TrashPath); err != nil {
			_ = e.St.MarkException(it.ID, fmt.Sprintf("expire purge failed: %v", err))
			failures = append(failures, fmt.Sprintf(i18n.Tr("清理过期项 #%d 失败: %v", "purging expired item #%d failed: %v"), it.ID, err))
			continue
		}
		if err := e.St.LeaveBin(it.ID, model.OpExpire, now, "retention deadline reached"); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		n++
	}
	if !opt.Quiet {
		e.printf(i18n.Tr("已清理 %d 个过期项。\n", "purged %d expired item(s).\n"), n)
	}
	if len(failures) > 0 {
		return n, &OpError{Failures: failures}
	}
	return n, nil
}

// Empty removes every item currently in the trash.
func (e *Engine) Empty(opt PurgeOpts) error {
	items, err := e.St.ListBin()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Fprint(e.Out, i18n.Tr("回收站已经是空的。\n", "the trash bin is already empty.\n"))
		return nil
	}
	var ids []int64
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	if !opt.Yes {
		e.printf(i18n.Tr("将彻底清空回收站中的 %d 项(不可恢复):\n", "about to permanently empty %d item(s) (cannot be undone):\n"), len(items))
		for i := range items {
			e.printf("  #%d %s\n", items[i].ID, items[i].OrigPath)
		}
		if !e.Confirm(i18n.Tr("确认清空回收站?", "really empty the trash bin?"), false) {
			fmt.Fprint(e.Out, i18n.Tr("已取消。\n", "cancelled.\n"))
			return nil
		}
	}
	err = e.purge(ids, model.OpEmpty, PurgeOpts{Yes: true, DryRun: opt.DryRun},
		i18n.Tr("彻底清空 %d 项?", "permanently empty %d item(s)?"),
		i18n.Tr("已取消。", "cancelled."))
	if err == nil {
		_ = e.St.AppendReflog(model.Reflog{Ts: e.Now(), Op: model.OpEmpty, Detail: fmt.Sprintf("emptied %d items", len(items))})
	}
	return err
}

// Undo restores the most recently recycled item.
func (e *Engine) Undo(dryRun bool) error {
	items, err := e.St.ListBin()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Fprint(e.Out, i18n.Tr("回收站是空的, 没有可撤销的操作。\n", "the trash bin is empty, nothing to undo.\n"))
		return nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return e.Restore([]int64{items[0].ID}, RestoreOpts{DryRun: dryRun})
}

// LatestID returns the id of the most recently recycled item (0 if empty).
func (e *Engine) LatestID() (int64, error) {
	items, err := e.St.ListBin()
	if err != nil {
		return 0, err
	}
	var max int64
	for i := range items {
		if items[i].ID > max {
			max = items[i].ID
		}
	}
	return max, nil
}

// Pick interactively asks the user to choose items from the last n entries.
func (e *Engine) Pick(n int) ([]int64, error) {
	items, err := e.St.ListBin()
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	if len(items) > n {
		items = items[:n]
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s", i18n.Tr("回收站是空的", "the trash bin is empty"))
	}
	e.printf(i18n.Tr("最近的 %d 个回收项:\n", "the %d most recent trash item(s):\n"), len(items))
	for _, it := range items {
		mark := ""
		if it.Exception() {
			mark = i18n.Tr(" [异常]", " [exception]")
		}
		e.printf("  %d) %s (%s, %s)%s\n", it.ID, it.OrigPath, model.FormatSize(it.Size),
			time.Unix(it.RecycledAt, 0).Local().Format(e.Cfg.DateFormat), mark)
	}
	fmt.Fprint(e.Err, i18n.Tr("输入要操作的编号(空格分隔, 回车取消): ", "enter ids to act on (space separated, Enter to cancel): "))
	line, err := readLine(e.In)
	if err != nil || strings.TrimSpace(line) == "" {
		fmt.Fprint(e.Out, i18n.Tr("已取消。\n", "cancelled.\n"))
		return nil, nil
	}
	return store.ParseIDSelectors(strings.Fields(line))
}

func readLine(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	return br.ReadString('\n')
}

package trash

import (
	"os"
	"path/filepath"

	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/model"
)

// Stats summarizes the trash bin and its history.
type Stats struct {
	InBin     int
	Expired   int
	Exception int
	DirCount  int
	TotalSize int64
	DiskUsed  int64
	Oldest    int64
	Newest    int64
	History   int
	BinCaps   map[string]int
}

// Collect gathers statistics.
func (e *Engine) Stats() (*Stats, error) {
	items, err := e.St.ListBin()
	if err != nil {
		return nil, err
	}
	now := e.Now()
	s := &Stats{BinCaps: map[string]int{}, Oldest: now, Newest: 0}
	for i := range items {
		it := items[i]
		s.InBin++
		s.TotalSize += it.Size
		if it.IsDir {
			s.DirCount++
		}
		if it.ExpireAt <= now {
			s.Expired++
		}
		if it.Exception() {
			s.Exception++
		}
		if it.RecycledAt < s.Oldest {
			s.Oldest = it.RecycledAt
		}
		if it.RecycledAt > s.Newest {
			s.Newest = it.RecycledAt
		}
	}
	log, err := e.St.ListReflog()
	if err != nil {
		return nil, err
	}
	s.History = len(log)
	for _, e2 := range log {
		s.BinCaps[e2.Op]++
	}
	// actual disk usage of the trash directory
	_ = filepath.WalkDir(e.Cfg.TrashDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				s.DiskUsed += info.Size()
			}
		}
		return nil
	})
	return s, nil
}

// PrintStats renders statistics to the engine output.
func (e *Engine) PrintStats(s *Stats, print func(string, ...any)) {
	print(i18n.Tr("回收站: %s\n", "trash bin: %s\n"), e.Cfg.TrashDir)
	print(i18n.Tr("  当前项数: %d (目录 %d, 文件 %d)\n", "  items now: %d (dirs %d, files %d)\n"),
		s.InBin, s.DirCount, s.InBin-s.DirCount)
	print(i18n.Tr("  其中: 过期 %d, 异常 %d\n", "  of which: expired %d, exception %d\n"), s.Expired, s.Exception)
	print(i18n.Tr("  记录总大小: %s, 实际占用: %s\n", "  recorded size: %s, on-disk: %s\n"),
		model.FormatSize(s.TotalSize), model.FormatSize(s.DiskUsed))
	print(i18n.Tr("  历史记录: %d 条 ( recycle %d / restore %d / purge %d / expire %d / exception %d / reset %d / empty %d )\n",
		"  history: %d entries ( recycle %d / restore %d / purge %d / expire %d / exception %d / reset %d / empty %d )\n"),
		s.History, s.BinCaps[model.OpRecycle], s.BinCaps[model.OpRestore], s.BinCaps[model.OpPurge],
		s.BinCaps[model.OpExpire], s.BinCaps[model.OpException], s.BinCaps[model.OpReset], s.BinCaps[model.OpEmpty])
}

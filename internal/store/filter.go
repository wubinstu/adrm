package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/wubinstu/adrm/internal/model"
)

// ParseIDSelectors turns positional id arguments into a list of ids.
// Accepted forms: "10", "1-20", "1,3,5-8", or any comma/space separated mix.
func ParseIDSelectors(tokens []string) ([]int64, error) {
	var ids []int64
	seen := map[int64]bool{}
	add := func(id int64) error {
		if id <= 0 {
			return fmt.Errorf("invalid id %d (ids are positive numbers)", id)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
		return nil
	}
	for _, tok := range tokens {
		for _, part := range strings.Split(tok, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.Contains(part, "-") {
				bounds := strings.SplitN(part, "-", 2)
				lo, err1 := strconv.ParseInt(strings.TrimSpace(bounds[0]), 10, 64)
				hi, err2 := strconv.ParseInt(strings.TrimSpace(bounds[1]), 10, 64)
				if err1 != nil || err2 != nil || lo <= 0 || hi < lo {
					return nil, fmt.Errorf("invalid id range %q (want e.g. 1-20)", part)
				}
				for i := lo; i <= hi; i++ {
					if err := add(i); err != nil {
						return nil, err
					}
				}
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid id %q (want e.g. 12 or 1-20)", part)
			}
			if err := add(id); err != nil {
				return nil, err
			}
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no item ids given (see 'adrm ls')")
	}
	return ids, nil
}

// BinState returns the state of a bin item: "recycled" or "exception".
func BinState(it *model.Item) string {
	if it.Exception() {
		return "exception"
	}
	return "recycled"
}

// ApplyBinFilter filters and sorts bin items. now is the current unix time.
func ApplyBinFilter(items []model.Item, f *model.Filter, now int64) []model.Item {
	if f == nil {
		return items
	}
	want := map[string]bool{}
	for _, s := range f.States {
		want[s] = true
	}
	out := make([]model.Item, 0, len(items))
	for i := range items {
		it := items[i]
		if len(want) > 0 && !want[BinState(&it)] {
			continue
		}
		if f.Expired && it.ExpireAt > now {
			continue
		}
		if len(f.IDs) > 0 && !containsID(f.IDs, it.ID) {
			continue
		}
		if f.Name != "" && !strings.Contains(strings.ToLower(it.OrigPath), strings.ToLower(f.Name)) {
			continue
		}
		if f.PathPrefix != "" && !strings.HasPrefix(it.OrigPath, f.PathPrefix) {
			continue
		}
		if f.RecycledLo != nil && it.RecycledAt < *f.RecycledLo {
			continue
		}
		if f.RecycledHi != nil && it.RecycledAt > *f.RecycledHi {
			continue
		}
		if !f.Mtime.Match(it.Mtime) {
			continue
		}
		if !f.Size.Match(it.Size) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// ApplyReflogFilter filters and sorts reflog entries.
func ApplyReflogFilter(entries []model.Reflog, f *model.Filter) []model.Reflog {
	if f == nil {
		return entries
	}
	want := map[string]bool{}
	for _, s := range f.Ops {
		want[s] = true
	}
	out := make([]model.Reflog, 0, len(entries))
	for _, e := range entries {
		if len(want) > 0 && !want[e.Op] {
			continue
		}
		if len(f.IDs) > 0 && !containsID(f.IDs, e.ItemID) {
			continue
		}
		if f.Name != "" && !strings.Contains(strings.ToLower(e.OrigPath), strings.ToLower(f.Name)) {
			continue
		}
		if f.PathPrefix != "" && !strings.HasPrefix(e.OrigPath, f.PathPrefix) {
			continue
		}
		if f.OpLo != nil && e.Ts < *f.OpLo {
			continue
		}
		if f.OpHi != nil && e.Ts > *f.OpHi {
			continue
		}
		out = append(out, e)
	}
	return out
}

func containsID(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// SortBin orders items by the given keys (first key has highest priority).
func SortBin(items []model.Item, keys []model.SortKey) {
	if len(keys) == 0 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		for _, k := range keys {
			c := binCompare(&a, &b, k.Field)
			if c == 0 {
				continue
			}
			if k.Desc {
				return c > 0
			}
			return c < 0
		}
		return false
	})
}

func binCompare(a, b *model.Item, field string) int {
	switch strings.ToLower(field) {
	case "id":
		return cmpI64(a.ID, b.ID)
	case "path", "fname", "name":
		return strings.Compare(strings.ToLower(a.OrigPath), strings.ToLower(b.OrigPath))
	case "size", "fsize":
		return cmpI64(a.Size, b.Size)
	case "mtime", "fdate":
		return cmpI64(a.Mtime, b.Mtime)
	case "recycled", "rdate":
		return cmpI64(a.RecycledAt, b.RecycledAt)
	case "expire", "cdate":
		return cmpI64(a.ExpireAt, b.ExpireAt)
	case "state":
		return strings.Compare(BinState(a), BinState(b))
	}
	return 0
}

// SortReflog orders history entries by the given keys.
func SortReflog(entries []model.Reflog, keys []model.SortKey) {
	if len(keys) == 0 {
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		for _, k := range keys {
			c := reflogCompare(&a, &b, k.Field)
			if c == 0 {
				continue
			}
			if k.Desc {
				return c > 0
			}
			return c < 0
		}
		return false
	})
}

func reflogCompare(a, b *model.Reflog, field string) int {
	switch strings.ToLower(field) {
	case "seq", "id":
		return cmpI64(a.Seq, b.Seq)
	case "ts", "time", "date":
		return cmpI64(a.Ts, b.Ts)
	case "op":
		return strings.Compare(a.Op, b.Op)
	case "item", "item_id":
		return cmpI64(a.ItemID, b.ItemID)
	case "path", "fname", "name":
		return strings.Compare(strings.ToLower(a.OrigPath), strings.ToLower(b.OrigPath))
	case "size":
		return cmpI64(a.Size, b.Size)
	}
	return 0
}

func cmpI64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// BaseName returns the last path component (used for name matching and display).
func BaseName(p string) string { return filepath.Base(p) }

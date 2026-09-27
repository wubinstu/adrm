package store

import (
	"path/filepath"
	"testing"

	"github.com/wubinstu/adrm/internal/model"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "adrm.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestBinReflogLifecycle(t *testing.T) {
	st := openTemp(t)
	items := []model.Item{
		{OrigPath: "/x/a", Size: 10, TrashPath: "/t/a", RecycledAt: 100, ExpireAt: 200, Mode: 0644},
		{OrigPath: "/x/b", IsDir: true, Size: 20, TrashPath: "/t/b", RecycledAt: 101, ExpireAt: 201, Mode: 0755},
	}
	if err := st.RecycleItems(items); err != nil {
		t.Fatalf("RecycleItems: %v", err)
	}
	if items[0].ID == 0 || items[1].ID == 0 {
		t.Fatal("ids should be assigned")
	}
	bin, err := st.ListBin()
	if err != nil || len(bin) != 2 {
		t.Fatalf("ListBin: %v (%d rows)", err, len(bin))
	}
	// Leaving the bin removes the row but keeps the reflog entry.
	if err := st.LeaveBin(items[0].ID, model.OpRestore, 300, "detail"); err != nil {
		t.Fatalf("LeaveBin: %v", err)
	}
	bin, _ = st.ListBin()
	if len(bin) != 1 || bin[0].ID != items[1].ID {
		t.Fatalf("after restore: %+v", bin)
	}
	log, err := st.ListReflog()
	if err != nil {
		t.Fatalf("ListReflog: %v", err)
	}
	if len(log) != 3 {
		t.Fatalf("reflog should have 3 entries (2 recycle + 1 restore), got %d", len(log))
	}
	if log[2].Op != model.OpRestore || log[2].ItemID != items[0].ID {
		t.Errorf("last reflog entry: %+v", log[2])
	}
	// ids are never reused
	maxID, err := st.MaxBinID()
	if err != nil || maxID != items[1].ID {
		t.Fatalf("MaxBinID = %d (%v), want %d", maxID, err, items[1].ID)
	}
	next := []model.Item{{OrigPath: "/x/c", TrashPath: "/t/c", RecycledAt: 400, ExpireAt: 500}}
	if err := st.RecycleItems(next); err != nil {
		t.Fatalf("RecycleItems #2: %v", err)
	}
	if next[0].ID <= maxID {
		t.Errorf("new item got id %d, but %d was already used (ids must never be reused)", next[0].ID, maxID)
	}
}

func TestReset(t *testing.T) {
	st := openTemp(t)
	if err := st.RecycleItems([]model.Item{{OrigPath: "/x/a", TrashPath: "/t/a", RecycledAt: 1, ExpireAt: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := st.Reset("test"); err != nil {
		t.Fatal(err)
	}
	bin, _ := st.ListBin()
	log, _ := st.ListReflog()
	if len(bin) != 0 {
		t.Errorf("bin should be empty, got %d", len(bin))
	}
	if len(log) != 1 || log[0].Op != model.OpReset {
		t.Errorf("reflog should contain exactly the reset entry, got %+v", log)
	}
}

func TestApplyFilters(t *testing.T) {
	st := openTemp(t)
	now := int64(1000000)
	_ = st.RecycleItems([]model.Item{
		{ID: 1, OrigPath: "/x/a.tmp", Size: 100, TrashPath: "/t/1", RecycledAt: now - 100, ExpireAt: now + 100},
		{ID: 2, OrigPath: "/x/b.log", Size: 1 << 30, TrashPath: "/t/2", RecycledAt: now - 5000, ExpireAt: now - 1},
		{ID: 3, OrigPath: "/x/a.tmp", IsDir: true, Size: 5, TrashPath: "/t/3", RecycledAt: now - 10, ExpireAt: now + 99999},
	})
	bin, _ := st.ListBin()

	got := ApplyBinFilter(bin, &model.Filter{Name: "a.tmp"}, now)
	if len(got) != 2 {
		t.Errorf("--name a.tmp: got %d rows, want 2", len(got))
	}
	got = ApplyBinFilter(bin, &model.Filter{Expired: true}, now)
	if len(got) != 1 || got[0].ID != 2 {
		t.Errorf("--expired: %+v", got)
	}
	lo, hi := now-200, now-50
	got = ApplyBinFilter(bin, &model.Filter{RecycledLo: &lo, RecycledHi: &hi}, now)
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("time window: %+v", got)
	}
	spec := &model.SizeSpec{Lo: &[]int64{1 << 20}[0]}
	got = ApplyBinFilter(bin, &model.Filter{Size: spec}, now)
	if len(got) != 1 || got[0].ID != 2 {
		t.Errorf("--size +1m: %+v", got)
	}
	got = ApplyBinFilter(bin, &model.Filter{IDs: []int64{1, 3}}, now)
	if len(got) != 2 {
		t.Errorf("ids: %d", len(got))
	}
	got = ApplyBinFilter(bin, &model.Filter{PathPrefix: "/x/a"}, now)
	if len(got) != 2 {
		t.Errorf("--path /x/a: %d", len(got))
	}
}

func TestSortBin(t *testing.T) {
	items := []model.Item{
		{ID: 3, Size: 1},
		{ID: 1, Size: 30},
		{ID: 2, Size: 10},
	}
	SortBin(items, []model.SortKey{{Field: "size", Desc: true}})
	if items[0].ID != 1 || items[1].ID != 2 || items[2].ID != 3 {
		t.Errorf("sort by size desc: %+v", items)
	}
	SortBin(items, []model.SortKey{{Field: "id"}})
	if items[0].ID != 1 || items[1].ID != 2 || items[2].ID != 3 {
		t.Errorf("sort by id asc: %+v", items)
	}
}

func TestParseIDSelectors(t *testing.T) {
	ids, err := ParseIDSelectors([]string{"5", "1-3", "7,9-10"})
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{5, 1, 2, 3, 7, 9, 10}
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got %v, want %v", ids, want)
		}
	}
	for _, bad := range [][]string{{"0"}, {"x"}, {"3-1"}, {}} {
		if _, err := ParseIDSelectors(bad); err == nil {
			t.Errorf("ParseIDSelectors(%v) should fail", bad)
		}
	}
}

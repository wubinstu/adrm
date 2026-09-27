package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wubinstu/adrm/internal/config"
	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/model"
	"github.com/wubinstu/adrm/internal/render"
	"github.com/wubinstu/adrm/internal/store"
	"github.com/wubinstu/adrm/internal/trash"
)

// runSubcommand dispatches a management subcommand.
func (a *app) runSubcommand(cmd string, args []string) int {
	var code int
	switch cmd {
	case "ls":
		code = a.cmdList(args, false)
	case "log":
		code = a.cmdList(args, true)
	case "restore":
		code = a.cmdRestore(args)
	case "undo":
		code = a.cmdUndo(args)
	case "purge":
		code = a.cmdPurge(args)
	case "gc":
		code = a.cmdGC(args)
	case "empty":
		code = a.cmdEmpty(args)
	case "stats":
		code = a.cmdStats(args)
	case "config":
		code = a.cmdConfig(args)
	case "completions":
		code = a.cmdCompletions(args)
	case "setup":
		code = a.cmdSetup(args)
	case "doctor":
		code = a.cmdDoctor(args)
	case "db":
		code = a.cmdDB(args)
	default:
		fmt.Fprintf(a.errw, "adrm: %s\n", i18n.Tr("未知命令 %q", "unknown command %q", cmd))
		code = 2
	}
	return code
}

func (a *app) fail(err error) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(a.errw, "adrm: %v\n", err)
	var ue *usageError
	if errors.As(err, &ue) {
		return 2
	}
	return 1
}

// style builds the render style from config + tty detection.
func (a *app) style() render.Style {
	color := false
	switch a.cfg.Color {
	case "always":
		color = true
	case "never":
		color = false
	default:
		if f, ok := a.out.(*os.File); ok && xIsTerminal(f) {
			color = true
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		color = false
	}
	return render.Style{Color: color, ASCII: i18n.Current() == i18n.EN}
}

// cmdList implements ls (bin table) and log (reflog table).
func (a *app) cmdList(args []string, forLog bool) int {
	p := newParser()
	if err := p.parse(args, listSpecs); err != nil {
		return a.fail(err)
	}
	if p.has("help") {
		helpCmd := "ls"
		if forLog {
			helpCmd = "log"
		}
		a.printHelp([]string{helpCmd})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()

	f, err := a.buildFilter(p, forLog)
	if err != nil {
		return a.fail(err)
	}
	style := a.style()
	now := time.Now().Unix()

	if forLog {
		entries, err := eng.Reflog()
		if err != nil {
			return a.fail(err)
		}
		entries = store.ApplyReflogFilter(entries, f)
		entries = a.orderLog(entries, f)
		if p.has("json") {
			return a.fail(render.JSON(a.out, logJSON(entries)))
		}
		if err := a.renderLog(entries, style); err != nil {
			return a.fail(err)
		}
		return 0
	}

	items, err := eng.Bin()
	if err != nil {
		return a.fail(err)
	}
	items = store.ApplyBinFilter(items, f, now)
	items = a.orderBin(items, f)
	if p.has("json") {
		return a.fail(render.JSON(a.out, binJSON(items, now)))
	}
	if err := a.renderBin(items, style, now); err != nil {
		return a.fail(err)
	}
	return 0
}

// orderBin applies --last, the default newest-first order, user sorts and the
// max_list cap. Nothing is truncated silently: the cap prints a notice.
func (a *app) orderBin(items []model.Item, f *model.Filter) []model.Item {
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	if f != nil && f.Last > 0 && len(items) > f.Last {
		items = items[:f.Last]
	}
	if f != nil && len(f.Sort) > 0 {
		store.SortBin(items, f.Sort)
	}
	if len(items) > a.cfg.MaxList {
		n := len(items) - a.cfg.MaxList
		fmt.Fprintf(a.out, i18n.Tr("... 已省略 %d 行 (max_list=%d, 用 --last N 精确控制)\n",
			"... %d row(s) omitted (max_list=%d, use --last N to control)\n"), n, a.cfg.MaxList)
		items = items[:a.cfg.MaxList]
	}
	return items
}

// orderLog is orderBin for the reflog table.
func (a *app) orderLog(entries []model.Reflog, f *model.Filter) []model.Reflog {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Seq > entries[j].Seq })
	if f != nil && f.Last > 0 && len(entries) > f.Last {
		entries = entries[:f.Last]
	}
	if f != nil && len(f.Sort) > 0 {
		store.SortReflog(entries, f.Sort)
	}
	if len(entries) > a.cfg.MaxList {
		n := len(entries) - a.cfg.MaxList
		fmt.Fprintf(a.out, i18n.Tr("... 已省略 %d 行 (max_list=%d, 用 --last N 精确控制)\n",
			"... %d row(s) omitted (max_list=%d, use --last N to control)\n"), n, a.cfg.MaxList)
		entries = entries[:a.cfg.MaxList]
	}
	return entries
}

// ---- rendering ----

var binColumnHeaders = map[string][2]string{
	"id":       {"编号", "id"},
	"path":     {"原路径", "path"},
	"size":     {"大小", "size"},
	"mtime":    {"修改时间", "mtime"},
	"recycled": {"回收时间", "recycled"},
	"expire":   {"过期时间", "expires"},
	"left":     {"剩余", "left"},
	"state":    {"状态", "state"},
	"mode":     {"权限", "mode"},
	"owner":    {"属主", "owner"},
	"trash":    {"回收站路径", "trash path"},
}

var logColumnHeaders = map[string][2]string{
	"seq":    {"序号", "seq"},
	"ts":     {"时间", "time"},
	"op":     {"操作", "op"},
	"id":     {"项", "item"},
	"path":   {"原路径", "path"},
	"size":   {"大小", "size"},
	"detail": {"详情", "detail"},
}

func headerLabel(m map[string][2]string, col string) string {
	pair := m[col]
	if i18n.Current() == i18n.ZH {
		return pair[0]
	}
	return pair[1]
}

func (a *app) renderBin(items []model.Item, style render.Style, now int64) error {
	cols := a.cfg.LSColumns
	if err := validateColumns("ls_columns", cols, binColumnHeaders); err != nil {
		return err
	}
	headers := make([]string, len(cols))
	aligns := make([]int, len(cols))
	for i, c := range cols {
		headers[i] = headerLabel(binColumnHeaders, c)
		aligns[i] = binAlign(c)
	}
	t := render.NewTable(a.out, style, headers, aligns)
	for _, it := range items {
		row := make([]render.Cell, 0, len(cols))
		for _, c := range cols {
			row = append(row, a.binCell(c, it, now))
		}
		t.Add(row...)
	}
	_ = t.WriteEmpty(i18n.Tr("回收站是空的。", "the trash bin is empty."))
	return nil
}

// validateColumns rejects unknown column names in the config.
func validateColumns(key string, cols []string, known map[string][2]string) error {
	for _, c := range cols {
		if _, ok := known[c]; !ok {
			names := make([]string, 0, len(known))
			for k := range known {
				names = append(names, k)
			}
			sort.Strings(names)
			return usageErr("%s", i18n.Tr("配置 %s 中的列 %q 不存在 (可用: %s)",
				"column %q in config %s does not exist (available: %s)", c, key, strings.Join(names, " ")))
		}
	}
	return nil
}

func binAlign(col string) int {
	switch col {
	case "id", "size", "left":
		return 1
	}
	return -1
}

func (a *app) binCell(col string, it model.Item, now int64) render.Cell {
	switch col {
	case "id":
		return render.Cell{Text: strconv.FormatInt(it.ID, 10), Align: 1}
	case "path":
		name := it.OrigPath
		if it.IsDir {
			name += "/"
		}
		return render.Cell{Text: name}
	case "size":
		return render.Cell{Text: model.FormatSize(it.Size), Align: 1}
	case "mtime":
		return render.Cell{Text: render.FormatTime(it.Mtime, a.cfg.DateFormat)}
	case "recycled":
		return render.Cell{Text: render.FormatTime(it.RecycledAt, a.cfg.DateFormat)}
	case "expire":
		c := ""
		if it.ExpireAt <= now {
			c = render.Red
		}
		return render.Cell{Text: render.FormatTime(it.ExpireAt, a.cfg.DateFormat), Color: c}
	case "left":
		return render.Cell{Text: render.FormatLeft(it.ExpireAt, now), Align: 1}
	case "state":
		st := store.BinState(&it)
		return render.Cell{Text: st, Color: render.StateColor(st)}
	case "mode":
		return render.Cell{Text: fmt.Sprintf("%04o", it.Mode)}
	case "owner":
		return render.Cell{Text: it.Owner + ":" + it.Group}
	case "trash":
		return render.Cell{Text: it.TrashPath}
	}
	return render.Cell{Text: i18n.Tr("<未知列 %s>", "<unknown column %s>"), Color: render.Red}
}

func (a *app) renderLog(entries []model.Reflog, style render.Style) error {
	cols := a.cfg.LogColumns
	if err := validateColumns("log_columns", cols, logColumnHeaders); err != nil {
		return err
	}
	headers := make([]string, len(cols))
	aligns := make([]int, len(cols))
	for i, c := range cols {
		headers[i] = headerLabel(logColumnHeaders, c)
		if c == "seq" || c == "id" || c == "size" {
			aligns[i] = 1
		} else {
			aligns[i] = -1
		}
	}
	t := render.NewTable(a.out, style, headers, aligns)
	for _, e := range entries {
		row := make([]render.Cell, 0, len(cols))
		for _, c := range cols {
			row = append(row, a.logCell(c, e))
		}
		t.Add(row...)
	}
	_ = t.WriteEmpty(i18n.Tr("没有历史记录。", "no history yet."))
	return nil
}

func (a *app) logCell(col string, e model.Reflog) render.Cell {
	switch col {
	case "seq":
		return render.Cell{Text: strconv.FormatInt(e.Seq, 10), Align: 1}
	case "ts":
		return render.Cell{Text: render.FormatTime(e.Ts, a.cfg.DateFormat)}
	case "op":
		return render.Cell{Text: e.Op, Color: render.OpColor(e.Op)}
	case "id":
		txt := "-"
		if e.ItemID > 0 {
			txt = strconv.FormatInt(e.ItemID, 10)
		}
		return render.Cell{Text: txt, Align: 1}
	case "path":
		return render.Cell{Text: orDash(e.OrigPath)}
	case "size":
		return render.Cell{Text: model.FormatSize(e.Size), Align: 1}
	case "detail":
		return render.Cell{Text: orDash(e.Detail), Color: render.Dim}
	}
	return render.Cell{Text: i18n.Tr("<未知列 %s>", "<unknown column %s>")}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ---- JSON ----

type binItemJSON struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	SizeHuman  string `json:"size_human"`
	Mode       uint32 `json:"mode"`
	ModeOctal  string `json:"mode_octal"`
	UID        int    `json:"uid"`
	GID        int    `json:"gid"`
	Owner      string `json:"owner"`
	Group      string `json:"group"`
	Mtime      int64  `json:"mtime"`
	MtimeHuman string `json:"mtime_human"`
	RecycledAt int64  `json:"recycled_at"`
	ExpireAt   int64  `json:"expire_at"`
	Expired    bool   `json:"expired"`
	State      string `json:"state"`
	TrashPath  string `json:"trash_path"`
}

func binJSON(items []model.Item, now int64) []binItemJSON {
	out := make([]binItemJSON, 0, len(items))
	for _, it := range items {
		out = append(out, binItemJSON{
			ID: it.ID, Path: it.OrigPath, IsDir: it.IsDir, Size: it.Size,
			SizeHuman: model.FormatSize(it.Size), Mode: it.Mode, ModeOctal: fmt.Sprintf("%04o", it.Mode),
			UID: it.UID, GID: it.GID, Owner: it.Owner, Group: it.Group,
			Mtime: it.Mtime, MtimeHuman: render.FormatTime(it.Mtime, "2006-01-02 15:04:05"),
			RecycledAt: it.RecycledAt, ExpireAt: it.ExpireAt, Expired: it.ExpireAt <= now,
			State: store.BinState(&it), TrashPath: it.TrashPath,
		})
	}
	return out
}

type logEntryJSON struct {
	Seq       int64  `json:"seq"`
	Ts        int64  `json:"ts"`
	TsHuman   string `json:"ts_human"`
	Op        string `json:"op"`
	ItemID    int64  `json:"item_id"`
	Path      string `json:"path"`
	TrashPath string `json:"trash_path"`
	Size      int64  `json:"size"`
	Detail    string `json:"detail"`
}

func logJSON(entries []model.Reflog) []logEntryJSON {
	out := make([]logEntryJSON, 0, len(entries))
	for _, e := range entries {
		out = append(out, logEntryJSON{
			Seq: e.Seq, Ts: e.Ts, TsHuman: render.FormatTime(e.Ts, "2006-01-02 15:04:05"),
			Op: e.Op, ItemID: e.ItemID, Path: e.OrigPath, TrashPath: e.TrashPath,
			Size: e.Size, Detail: e.Detail,
		})
	}
	return out
}

// ---- restore / purge / undo / gc / empty ----

func (a *app) cmdRestore(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(dangerSpecs, filterSpecs)); err != nil {
		return a.fail(err)
	}
	if p.has("help") {
		a.printHelp([]string{"restore"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()
	ids, err := a.resolveIDs(p, eng)
	if err != nil {
		return a.fail(err)
	}
	if len(ids) == 0 {
		fmt.Fprintln(a.out, i18n.Tr("已取消。", "cancelled."))
		return 0
	}
	if err := eng.Restore(ids, trash.RestoreOpts{Replace: p.has("replace"), DryRun: p.has("dry-run")}); err != nil {
		return a.fail(err)
	}
	return 0
}

func (a *app) cmdUndo(args []string) int {
	p := newParser()
	if err := p.parse(args, commonSpecs); err != nil {
		return a.fail(err)
	}
	if p.has("help") {
		a.printHelp([]string{"undo"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()
	if err := eng.Undo(p.has("dry-run")); err != nil {
		return a.fail(err)
	}
	return 0
}

func (a *app) cmdPurge(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(dangerSpecs, filterSpecs)); err != nil {
		return a.fail(err)
	}
	if p.has("help") {
		a.printHelp([]string{"purge"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()
	ids, err := a.resolveIDs(p, eng)
	if err != nil {
		return a.fail(err)
	}
	if len(ids) == 0 {
		fmt.Fprintln(a.out, i18n.Tr("已取消。", "cancelled."))
		return 0
	}
	if err := eng.Purge(ids, trash.PurgeOpts{Yes: p.has("yes"), DryRun: p.has("dry-run")}); err != nil {
		return a.fail(err)
	}
	return 0
}

// resolveIDs turns --last/--all/--pick/positional ids into an id list.
func (a *app) resolveIDs(p *parser, eng *trash.Engine) ([]int64, error) {
	switch {
	case p.has("all"):
		items, err := eng.Bin()
		if err != nil {
			return nil, err
		}
		var ids []int64
		for i := range items {
			ids = append(ids, items[i].ID)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return ids, nil
	case p.has("last"):
		id, err := eng.LatestID()
		if err != nil {
			return nil, err
		}
		if id == 0 {
			return nil, fmt.Errorf("%s", i18n.Tr("回收站是空的", "the trash bin is empty"))
		}
		return []int64{id}, nil
	case p.has("pick"):
		return eng.Pick(15)
	default:
		if len(p.pos) == 0 {
			return nil, fmt.Errorf("%s", i18n.Tr("没有指定 id (用 'adrm ls' 查看, 或 --last/--all/--pick)",
				"no ids given (see 'adrm ls', or use --last/--all/--pick)"))
		}
		return store.ParseIDSelectors(p.pos)
	}
}

func (a *app) cmdGC(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(commonSpecs, []optSpec{
		{long: "dry-run"}, {long: "yes", short: "y"}, {long: "quiet"},
	})); err != nil {
		return a.fail(err)
	}
	if p.has("help") {
		a.printHelp([]string{"gc"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()
	if _, err := eng.GC(trash.GCOpts{
		DryRun: p.has("dry-run"), Yes: p.has("yes"), Quiet: p.has("quiet"),
	}); err != nil {
		return a.fail(err)
	}
	return 0
}

func (a *app) cmdEmpty(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(commonSpecs, []optSpec{{long: "yes", short: "y"}, {long: "dry-run"}})); err != nil {
		return a.fail(err)
	}
	if p.has("help") {
		a.printHelp([]string{"empty"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()
	if err := eng.Empty(trash.PurgeOpts{Yes: p.has("yes"), DryRun: p.has("dry-run")}); err != nil {
		return a.fail(err)
	}
	return 0
}

// ---- stats ----

func (a *app) cmdStats(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(commonSpecs, []optSpec{{long: "json"}})); err != nil {
		return a.fail(err)
	}
	if p.has("help") {
		a.printHelp([]string{"stats"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()
	s, err := eng.Stats()
	if err != nil {
		return a.fail(err)
	}
	if p.has("json") {
		return a.fail(render.JSON(a.out, map[string]any{
			"home": a.cfg.Home, "trash_dir": a.cfg.TrashDir, "database": a.cfg.Database,
			"in_bin": s.InBin, "expired": s.Expired, "exception": s.Exception,
			"dirs": s.DirCount, "files": s.InBin - s.DirCount,
			"recorded_size": s.TotalSize, "disk_used": s.DiskUsed,
			"history": s.History, "ops": s.BinCaps,
		}))
	}
	eng.PrintStats(s, a.printf)
	return 0
}

func (a *app) printf(format string, args ...any) { fmt.Fprintf(a.out, format, args...) }

// ---- config ----

func (a *app) cmdConfig(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(commonSpecs, []optSpec{
		{long: "init"}, {long: "show"}, {long: "edit"}, {long: "path"}, {long: "validate"}, {long: "force"},
	})); err != nil {
		return a.fail(err)
	}
	if p.has("help") || (!p.has("init") && !p.has("show") && !p.has("edit") && !p.has("path") && !p.has("validate")) {
		a.printHelp([]string{"config"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	switch {
	case p.has("init"):
		if err := config.WriteTemplate(a.cfg.ConfigFile, p.has("force")); err != nil {
			return a.fail(err)
		}
		fmt.Fprintf(a.out, i18n.Tr("已写入默认配置: %s\n", "wrote default config: %s\n"), a.cfg.ConfigFile)
		return 0
	case p.has("path"):
		fmt.Fprintln(a.out, a.cfg.ConfigFile)
		return 0
	case p.has("validate"):
		if _, err := config.Load(a.cfg.Home); err != nil {
			return a.fail(err)
		}
		fmt.Fprintf(a.out, i18n.Tr("配置有效: %s\n", "config is valid: %s\n"), a.cfg.ConfigFile)
		return 0
	case p.has("edit"):
		return a.editFile(a.cfg.ConfigFile)
	case p.has("show"):
		a.showConfig()
		return 0
	}
	return 0
}

func (a *app) showConfig() {
	fmt.Fprintf(a.out, i18n.Tr("# 生效配置 (来源: %s)\n", "# effective config (source: %s)\n"), a.cfg.ConfigFile)
	rows := [][2]string{
		{"home", a.cfg.Home},
		{"trash_dir", a.cfg.TrashDir},
		{"database", a.cfg.Database},
		{"ignore_file", a.cfg.IgnoreFile},
		{"retention_days", strconv.Itoa(a.cfg.RetentionDays)},
		{"auto_gc", strconv.FormatBool(a.cfg.AutoGC)},
		{"prompt_threshold", strconv.Itoa(a.cfg.PromptThreshold)},
		{"max_list", strconv.Itoa(a.cfg.MaxList)},
		{"preserve_root", strconv.FormatBool(a.cfg.PreserveRoot)},
		{"copy_fallback", strconv.FormatBool(a.cfg.CopyFallback)},
		{"lang", a.cfg.Lang + " -> " + i18n.Current()},
		{"color", a.cfg.Color},
		{"date_format", a.cfg.DateFormat},
		{"ls_columns", strings.Join(a.cfg.LSColumns, ",")},
		{"log_columns", strings.Join(a.cfg.LogColumns, ",")},
	}
	style := a.style()
	t := render.NewTable(a.out, style, []string{
		headerLabel(map[string][2]string{"key": {"键", "key"}}, "key"),
		headerLabel(map[string][2]string{"value": {"值", "value"}}, "value"),
	}, []int{-1, -1})
	for _, r := range rows {
		t.Add(render.Cell{Text: r[0]}, render.Cell{Text: r[1]})
	}
	_ = t.Write()
}

func (a *app) editFile(path string) int {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.in, a.out, a.errw
	if err := cmd.Run(); err != nil {
		return a.fail(fmt.Errorf("%s", i18n.Tr("编辑器退出: %v", "editor failed: %v", err)))

	}
	return 0
}

// ---- db ----

func (a *app) cmdDB(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(commonSpecs, []optSpec{{long: "reset"}, {long: "yes", short: "y"}})); err != nil {
		return a.fail(err)
	}
	if p.has("help") || !p.has("reset") {
		a.printHelp([]string{"db"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()
	items, err := eng.Bin()
	if err != nil {
		return a.fail(err)
	}
	if len(items) > 0 && !p.has("yes") {
		fmt.Fprintf(a.out, i18n.Tr("回收站中还有 %d 项(数据库只记录历史, 不跟踪这些文件):\n",
			"there are still %d item(s) in the trash (the database tracks history, not these files):\n"), len(items))
		for _, it := range items {
			fmt.Fprintf(a.out, "  #%d %s\n", it.ID, it.OrigPath)
		}
		if !eng.Confirm(i18n.Tr("先彻底清空回收站再重置数据库?", "empty the trash first, then reset the database?"), false) {
			fmt.Fprintln(a.out, i18n.Tr("已取消。", "cancelled."))
			return 0
		}
		if err := eng.Empty(trash.PurgeOpts{Yes: true}); err != nil {
			return a.fail(err)
		}
	}
	if err := eng.ResetDB(); err != nil {
		return a.fail(err)
	}
	fmt.Fprintln(a.out, i18n.Tr("数据库已重置(历史记录也已清空)。", "database reset (history cleared as well)."))
	return 0
}

// ---- doctor ----

func (a *app) cmdDoctor(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(commonSpecs, []optSpec{{long: "json"}})); err != nil {
		return a.fail(err)
	}
	if p.has("help") {
		a.printHelp([]string{"doctor"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	eng, err := a.engine()
	if err != nil {
		return a.fail(err)
	}
	defer a.closeStore()
	rep, err := eng.Doctor()
	if err != nil {
		return a.fail(err)
	}
	if p.has("json") {
		return a.fail(render.JSON(a.out, rep))
	}
	rep.Print(a.printf)
	if len(rep.Orphans) > 0 {
		fmt.Fprintf(a.out, i18n.Tr("发现 %d 个回收站中的孤立文件(数据库没有记录):\n",
			"found %d orphan file(s) in the trash (not tracked by the database):\n"), len(rep.Orphans))
		for _, o := range rep.Orphans {
			fmt.Fprintf(a.out, "  %s\n", o)
		}
		if eng.Confirm(i18n.Tr("彻底清理这些孤立文件?", "permanently delete these orphan files?"), false) {
			if err := eng.PurgeOrphans(rep.Orphans); err != nil {
				return a.fail(err)
			}
			fmt.Fprintln(a.out, i18n.Tr("孤立文件已清理。", "orphans removed."))
		}
	}
	allOK := true
	for _, c := range rep.Checks {
		if !c.OK {
			allOK = false
		}
	}
	if allOK && len(rep.Orphans) == 0 && len(rep.Missing) == 0 {
		fmt.Fprintln(a.out, i18n.Tr("一切正常。", "all good."))
	}
	return 0
}

// ---- helpers ----

func xIsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

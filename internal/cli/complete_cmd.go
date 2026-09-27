package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/wubinstu/adrm/internal/complete"
	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/model"
	"github.com/wubinstu/adrm/internal/store"
)

// cmdCompletions prints or installs completion scripts.
func (a *app) cmdCompletions(args []string) int {
	p := newParser()
	if err := p.parse(args, concatSpecs(commonSpecs, []optSpec{{long: "out", takesValue: true}})); err != nil {
		return a.fail(err)
	}
	if p.has("help") || len(p.pos) == 0 {
		a.printHelp([]string{"completions"})
		return 0
	}
	shell := strings.ToLower(p.pos[0])
	if shell != "bash" && shell != "zsh" && shell != "fish" {
		return a.fail(usageErr("%s", i18n.Tr("completions 只支持 bash|zsh|fish, 得到 %q", "completions supports bash|zsh|fish, got %q", shell)))
	}
	var script string
	switch shell {
	case "zsh":
		script = complete.Zsh()
	case "fish":
		script = complete.Fish()
	default:
		script = complete.Bash()
	}
	if out := p.val("out"); out != "" {
		path := filepath.Join(out, complete.FileName(shell))
		if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
			return a.fail(err)
		}
		fmt.Fprintf(a.out, i18n.Tr("已写入 %s\n", "wrote %s\n"), path)
		fmt.Fprintf(a.out, i18n.Tr("在 ~/.bashrc 或 ~/.zshrc 中加入: source %s\n", "add to ~/.bashrc or ~/.zshrc: source %s\n"), path)
		return 0
	}
	fmt.Fprint(a.out, script)
	return 0
}

// runComplete is the hidden engine behind shell completion. It may open the
// database to offer trash ids and original file names as candidates.
func (a *app) runComplete(args []string) int {
	shell := "bash"
	if len(args) > 0 {
		shell = strings.ToLower(args[0])
		args = args[1:]
	}
	var cur string
	var prefix []string
	if len(args) > 0 {
		cur = args[len(args)-1]
		prefix = args[:len(args)-1]
	} else {
		prefix = nil
	}
	cands := a.candidates(prefix, cur)
	if shell == "bash" {
		for _, c := range cands {
			fmt.Fprintln(a.out, c.name)
		}
		return 0
	}
	for _, c := range cands {
		if c.desc != "" {
			fmt.Fprintf(a.out, "%s\t%s\n", c.name, c.desc)
		} else {
			fmt.Fprintln(a.out, c.name)
		}
	}
	return 0
}

type candidate struct {
	name string
	desc string
}

// candidates decides what to offer given the words before the cursor.
func (a *app) candidates(prefix []string, cur string) []candidate {
	// completing the very first word: subcommands
	if len(prefix) == 0 {
		return commandCandidates()
	}
	cmd := prefix[0]
	if !subcommands[cmd] && cmd != "help" && cmd != "version" {
		// recycle mode: only options when typing "-"
		if strings.HasPrefix(cur, "-") {
			return recycleOptionCandidates(cur)
		}
		return nil
	}
	// inside a subcommand
	if strings.HasPrefix(cur, "-") {
		return optionCandidates(cmd, cur)
	}
	// value of the preceding option?
	if len(prefix) >= 1 {
		prev := prefix[len(prefix)-1]
		if vals, ok := enumCandidates(prev, cmd); ok {
			return filterPrefix(vals, cur)
		}
		switch prev {
		case "--name":
			return a.nameCandidates(cur)
		case "--path":
			return a.pathCandidates(cur)
		}
	}
	// positional candidates
	switch cmd {
	case "restore", "purge", "ls", "log":
		return a.idCandidates(cur)
	case "completions":
		return filterPrefix([]candidate{{"bash", ""}, {"zsh", ""}, {"fish", ""}}, cur)
	case "help":
		return filterPrefix(commandCandidates(), cur)
	}
	return nil
}

func filterPrefix(cs []candidate, cur string) []candidate {
	var out []candidate
	for _, c := range cs {
		if strings.HasPrefix(c.name, cur) {
			out = append(out, c)
		}
	}
	return out
}

func commandCandidates() []candidate {
	cmds := []string{"ls", "log", "restore", "undo", "purge", "gc", "empty", "stats",
		"config", "completions", "setup", "doctor", "db", "help", "version"}
	out := make([]candidate, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, candidate{name: c, desc: i18n.Tr("子命令", "command")})
	}
	return out
}

func recycleOptionCandidates(cur string) []candidate {
	opts := []struct{ name, desc string }{
		{"--force", "-f"}, {"--interactive", "-i"}, {"--recursive", "-r -R"},
		{"--dir", "-d"}, {"--verbose", "-v"}, {"--yes", "-y"}, {"--dry-run", ""},
		{"--preserve-root", ""}, {"--no-preserve-root", ""},
		{"--for", "expiry duration"}, {"--until", "expiry deadline"},
		{"--home", ""}, {"--lang", ""}, {"--color", ""}, {"--help", "-h"},
	}
	var out []candidate
	for _, o := range opts {
		if strings.HasPrefix(o.name, cur) {
			out = append(out, candidate{name: o.name, desc: o.desc})
		}
	}
	return out
}

var cmdOptionSpecs = map[string][]struct{ name, desc string }{
	"ls": {
		{"--state", "recycled|exception"}, {"--op", ""}, {"--name", "substring"},
		{"--path", "prefix"}, {"--older-than", "dur|date"}, {"--newer-than", "dur|date"},
		{"--mtime", "+3d|-3d|date"}, {"--size", "+1g|-1k|10g20m"},
		{"--since", "log only"}, {"--until", "log only"}, {"--last", "N"},
		{"--sort", "field:asc|desc"}, {"--json", ""}, {"--expired", ""},
		{"--color", "auto|always|never"}, {"--lang", "zh|en"}, {"--home", "path"}, {"--help", ""},
	},
	"log": {
		{"--op", "recycle|restore|purge|..."}, {"--state", ""}, {"--name", "substring"},
		{"--path", "prefix"}, {"--since", "dur|date"}, {"--until", "dur|date"},
		{"--older-than", ""}, {"--newer-than", ""}, {"--mtime", ""}, {"--size", ""},
		{"--last", "N"}, {"--sort", "field:asc|desc"}, {"--json", ""},
		{"--color", "auto|always|never"}, {"--lang", "zh|en"}, {"--home", "path"}, {"--help", ""},
	},
	"restore": {
		{"--last", "most recent"}, {"--all", ""}, {"--pick", "interactive"},
		{"--replace", "recycle conflicts"}, {"--dry-run", ""},
		{"--state", ""}, {"--op", ""}, {"--name", ""}, {"--path", ""}, {"--expired", ""},
		{"--older-than", ""}, {"--newer-than", ""}, {"--mtime", ""}, {"--size", ""},
		{"--sort", ""}, {"--json", ""},
		{"--color", "auto|always|never"}, {"--lang", "zh|en"}, {"--home", "path"}, {"--help", ""},
	},
	"purge": {
		{"--last", "most recent"}, {"--all", ""}, {"--yes", "-y"}, {"--dry-run", ""},
		{"--state", ""}, {"--op", ""}, {"--name", ""}, {"--path", ""}, {"--expired", ""},
		{"--older-than", ""}, {"--newer-than", ""}, {"--mtime", ""}, {"--size", ""},
		{"--last", "N"}, {"--sort", ""}, {"--json", ""},
		{"--color", "auto|always|never"}, {"--lang", "zh|en"}, {"--home", "path"}, {"--help", ""},
	},
	"gc":          {{"--dry-run", ""}, {"--yes", "-y"}, {"--quiet", ""}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""}},
	"empty":       {{"--yes", "-y"}, {"--dry-run", ""}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""}},
	"stats":       {{"--json", ""}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""}},
	"config":      {{"--init", ""}, {"--show", ""}, {"--edit", ""}, {"--path", ""}, {"--validate", ""}, {"--force", ""}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""}},
	"completions": {{"--out", "dir"}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""}},
	"setup": {
		{"--install", ""}, {"--uninstall", ""}, {"--status", ""},
		{"--alias", ""}, {"--no-alias", ""}, {"--completion", ""}, {"--no-completion", ""},
		{"--cron", ""}, {"--no-cron", ""}, {"--purge-home", ""}, {"--yes", "-y"},
		{"--shells", "bash,zsh,fish"}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""},
	},
	"doctor":  {{"--json", ""}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""}},
	"db":      {{"--reset", ""}, {"--yes", "-y"}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""}},
	"undo":    {{"--dry-run", ""}, {"--lang", ""}, {"--color", ""}, {"--home", "path"}, {"--help", ""}},
	"help":    {},
	"version": {},
}

func optionCandidates(cmd, cur string) []candidate {
	specs := cmdOptionSpecs[cmd]
	var out []candidate
	for _, o := range specs {
		if strings.HasPrefix(o.name, cur) {
			out = append(out, candidate{name: o.name, desc: o.desc})
		}
	}
	// always allow the global options
	for _, o := range []struct{ name, desc string }{
		{"--home", "path"}, {"--lang", "zh|en"}, {"--color", "auto|always|never"}, {"--help", ""},
	} {
		if strings.HasPrefix(o.name, cur) && !hasCandidate(out, o.name) {
			out = append(out, candidate{name: o.name, desc: o.desc})
		}
	}
	return out
}

func hasCandidate(cs []candidate, name string) bool {
	for _, c := range cs {
		if c.name == name {
			return true
		}
	}
	return false
}

func enumCandidates(prev, cmd string) ([]candidate, bool) {
	switch prev {
	case "--state":
		return []candidate{{"recycled", ""}, {"exception", ""}}, true
	case "--op":
		return []candidate{
			{"recycle", ""}, {"restore", ""}, {"purge", ""}, {"expire", ""},
			{"exception", ""}, {"reset", ""}, {"empty", ""},
		}, true
	case "--color":
		return []candidate{{"auto", ""}, {"always", ""}, {"never", ""}}, true
	case "--lang":
		return []candidate{{"zh", ""}, {"en", ""}}, true
	case "--shells":
		return []candidate{{"bash", ""}, {"zsh", ""}, {"fish", ""}}, true
	case "--sort":
		fields := []string{"id", "path", "size", "mtime", "recycled", "expire", "state"}
		if cmd == "log" {
			fields = []string{"seq", "ts", "op", "item", "path", "size"}
		}
		out := []candidate{}
		for _, f := range fields {
			out = append(out, candidate{name: f + ":asc"}, candidate{name: f + ":desc"})
		}
		return out, true
	}
	return nil, false
}

// idCandidates offers the ids of items currently in the trash.
func (a *app) idCandidates(cur string) []candidate {
	items, err := a.storeItemsForCompletion()
	if err != nil || len(items) == 0 {
		return nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	var out []candidate
	for _, it := range items {
		name := strconv.FormatInt(it.ID, 10)
		if !strings.HasPrefix(name, cur) {
			continue
		}
		desc := it.OrigPath
		if it.IsDir {
			desc += "/"
		}
		out = append(out, candidate{name: name, desc: desc})
		if len(out) >= 50 {
			break
		}
	}
	return out
}

// nameCandidates offers base names seen in the database for --name.
func (a *app) nameCandidates(cur string) []candidate {
	items, err := a.storeItemsForCompletion()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []candidate
	for i := len(items) - 1; i >= 0 && len(out) < 20; i-- {
		base := filepath.Base(items[i].OrigPath)
		if seen[base] || !strings.HasPrefix(base, cur) {
			continue
		}
		seen[base] = true
		out = append(out, candidate{name: base, desc: "trashed name"})
	}
	return out
}

// pathCandidates offers parent directories seen in the database for --path.
func (a *app) pathCandidates(cur string) []candidate {
	items, err := a.storeItemsForCompletion()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []candidate
	for _, it := range items {
		dir := filepath.Dir(it.OrigPath)
		if seen[dir] || !strings.HasPrefix(dir, cur) {
			continue
		}
		seen[dir] = true
		out = append(out, candidate{name: dir, desc: "trashed path"})
		if len(out) >= 20 {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name > out[j].name })
	return out
}

// storeItemsForCompletion opens the store just to read ids/names. Failures are
// silent: completion simply falls back to file names.
func (a *app) storeItemsForCompletion() ([]model.Item, error) {
	if a.st != nil {
		return a.st.ListBin()
	}
	if _, err := os.Stat(a.cfg.Database); err != nil {
		return nil, err // never create a database from completion
	}
	st, err := store.Open(a.cfg.Database)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	return st.ListBin()
}

// ---- per-command help ----

func cmdHelp(cmd string) string {
	switch cmd {
	case "ls":
		return i18n.Tr(`用法: adrm ls [过滤] [排序] [选项]

列出当前回收站中的文件(只有还在回收站里的才列出; 历史见 'adrm log')。

过滤:
  <id...>          1, 5, 1-20
  --state recycled|exception
  --name PAT       原路径包含 PAT (忽略大小写)
  --path PREFIX    原路径前缀
  --expired        只看过期的
  --older-than D   回收时间早于 (3d4h5m 或 "2027-1-1")
  --newer-than D   回收时间晚于
  --mtime SPEC     原始修改时间匹配 (+3d 之后 / -3d 之前 / 无符号=恰好 / 绝对日期)
  --size SPEC      大小匹配 (+1g / -1k / 10g20m)
  --last N         最近 N 条
排序:
  --sort field[:asc|:desc],...   字段: id path size mtime recycled expire state
选项:
  --json           JSON 输出
  --color MODE     auto|always|never
  --lang zh|en

示例:
  adrm ls --state exception
  adrm ls --path ~/projects --last 20
  adrm ls --sort size:desc
  adrm ls --expired --json
`, `usage: adrm ls [filters] [sort] [options]

list what is currently in the trash bin (history lives in 'adrm log').

filters:
  <id...>          1, 5, 1-20
  --state recycled|exception
  --name PAT        original path contains PAT (case-insensitive)
  --path PREFIX     original path prefix
  --expired         only expired items
  --older-than D    recycled before (3d4h5m or "2027-1-1")
  --newer-than D    recycled after
  --mtime SPEC      original mtime (+3d after / -3d before / exact / date)
  --size SPEC       size (+1g / -1k / 10g20m)
  --last N          most recent N entries
sort:
  --sort field[:asc|:desc],...   fields: id path size mtime recycled expire state
options:
  --json           JSON output
  --color MODE     auto|always|never
  --lang zh|en

examples:
  adrm ls --state exception
  adrm ls --path ~/projects --last 20
  adrm ls --sort size:desc
  adrm ls --expired --json
`)
	case "log":
		return i18n.Tr(`用法: adrm log [过滤] [选项]

操作历史(类似 git reflog): 只追加, 永不修改。即使对应文件早已被彻底
清理, 记录依然保留, 可以追溯 "我删过什么"。

过滤:
  --op recycle|restore|purge|expire|exception|reset|empty
  --name PAT / --path PREFIX
  --since D --until D     按操作时间
  --older-than / --newer-than   按回收时间
  --last N
  <id...>      只看与这些 item id 相关的历史
排序: --sort seq|ts|op|item|path|size[:asc|:desc]
选项: --json

示例:
  adrm log --op purge --last 20
  adrm log --since 7d
  adrm log --name report.pdf
`, `usage: adrm log [filters] [options]

operation history (like git reflog): append-only, never modified. Records
survive the file they describe.

filters:
  --op recycle|restore|purge|expire|exception|reset|empty
  --name PAT / --path PREFIX
  --since D --until D     by operation time
  --older-than / --newer-than    by recycle time
  --last N
  <id...>      history for these item ids
sort: --sort seq|ts|op|item|path|size[:asc|:desc]
options: --json

examples:
  adrm log --op purge --last 20
  adrm log --since 7d
  adrm log --name report.pdf
`)
	case "restore":
		return i18n.Tr(`用法: adrm restore <id...> | --last | --all | --pick [选项]

把文件还原到原始路径, 恢复属主/属组/完整权限位(含 suid/sgid/sticky)与
修改时间。

选项:
  --last         还原最近回收的一个 (同 'adrm undo')
  --all          还原全部
  --pick         从最近 15 项中交互选择
  --replace      目标已存在时, 先把已存在的文件回收进回收站再还原
  --dry-run      只演练

示例:
  adrm restore 42
  adrm restore 1-5 --replace
  adrm undo
  adrm restore --pick
`, `usage: adrm restore <id...> | --last | --all | --pick [options]

restore items to their original path with owner, group, full permission
bits (including suid/sgid/sticky) and mtime.

options:
  --last         restore the most recently recycled item (same as 'adrm undo')
  --all          restore everything
  --pick         choose interactively from the 15 most recent
  --replace      when the target exists, recycle the existing file first
  --dry-run      show what would happen

examples:
  adrm restore 42
  adrm restore 1-5 --replace
  adrm undo
  adrm restore --pick
`)
	case "purge":
		return i18n.Tr(`用法: adrm purge <id...> | --last | --all [选项]

从回收站中彻底删除(真正删除, 不可恢复, 历史记录保留)。

选项:
  --last / --all / --dry-run / --yes

示例:
  adrm purge 42
  adrm purge --all --yes
  adrm purge 1-10 --dry-run
`, `usage: adrm purge <id...> | --last | --all [options]

permanently delete items from the trash (real deletion, unrecoverable;
the history entry stays).

options:
  --last / --all / --dry-run / --yes

examples:
  adrm purge 42
  adrm purge --all --yes
  adrm purge 1-10 --dry-run
`)
	case "gc":
		return i18n.Tr(`用法: adrm gc [--dry-run] [--yes] [--quiet]

清理所有已过有效期的回收项。adrm 没有守护进程; 过期清理是惰性的——只有
运行 gc (或任何命令在 auto_gc=true 时顺带) 才会真正删除。

示例:
  adrm gc --dry-run
  adrm gc
`, `usage: adrm gc [--dry-run] [--yes] [--quiet]

purge every item past its retention deadline. adrm has no daemon; expiry is
lazy: items are only removed when gc runs (or opportunistically when auto_gc
is on).

examples:
  adrm gc --dry-run
  adrm gc
`)
	case "empty":
		return i18n.Tr(`用法: adrm empty [--yes] [--dry-run]

清空回收站(彻底删除所有项)。历史记录保留。

示例: adrm empty --yes
`, `usage: adrm empty [--yes] [--dry-run]

empty the whole trash bin (permanently deletes every item). History is kept.

examples: adrm empty --yes
`)
	case "stats":
		return i18n.Tr(`用法: adrm stats [--json]

回收站统计: 项数/大小/过期数/历史条数等。

示例: adrm stats
`, `usage: adrm stats [--json]

trash statistics: counts, sizes, expired items, history entries.

examples: adrm stats
`)
	case "config":
		return i18n.Tr(`用法: adrm config (--init|--show|--edit|--path|--validate) [--force]

  --init       生成带完整注释的默认配置文件 (--force 覆盖)
  --show       显示生效配置
  --edit       用 $EDITOR 编辑
  --path       打印配置文件路径
  --validate   校验配置文件

示例:
  adrm config --init
  adrm config --show
`, `usage: adrm config (--init|--show|--edit|--path|--validate) [--force]

  --init       write the annotated default config (--force to overwrite)
  --show       show the effective config
  --edit       edit with $EDITOR
  --path       print the config file path
  --validate   validate the config file

examples:
  adrm config --init
  adrm config --show
`)
	case "completions":
		return i18n.Tr(`用法: adrm completions <bash|zsh|fish> [--out DIR]

输出补全脚本。动态候选(回收站 id、文件名)由脚本调用 'adrm __complete'
实时查询数据库得到。

示例:
  adrm completions bash > ~/.local/share/bash-completion/completions/adrm
  adrm completions zsh --out ~/.zfunc
  adrm completions fish > ~/.config/fish/completions/adrm.fish
  或一键管理: adrm setup --install
`, `usage: adrm completions <bash|zsh|fish> [--out DIR]

print a completion script. dynamic candidates (trash ids, file names) are
queried live from the database via 'adrm __complete'.

examples:
  adrm completions bash > ~/.local/share/bash-completion/completions/adrm
  adrm completions zsh --out ~/.zfunc
  adrm completions fish > ~/.config/fish/completions/adrm.fish
  or let adrm manage it: adrm setup --install
`)
	case "setup":
		return i18n.Tr(`用法: adrm setup (--install|--uninstall|--status) [选项]

安装/卸载 shell 集成: rm 别名 + bash/zsh/fish 补全。所有写入内容都带
标记块, 卸载时精确移除, 绝不动你其他的配置。

选项:
  --alias / --no-alias          是否添加 rm 别名 (默认添加)
  --completion / --no-completion 是否安装补全 (默认安装)
  --shells bash,zsh,fish         指定 shell (默认按 $SHELL 探测)
  --purge-home                  卸载时连同回收站数据一起删除 (需确认)
  --cron                        显示可选的定时清理配置方法

示例:
  adrm setup --install
  adrm setup --uninstall
  adrm setup --uninstall --purge-home
  adrm setup --status
`, `usage: adrm setup (--install|--uninstall|--status) [options]

install/remove shell integration: the rm alias and bash/zsh/fish completion.
Everything adrm writes is wrapped in marker blocks, so uninstall removes
exactly what install added and nothing else.

options:
  --alias / --no-alias            add the rm alias (default: add)
  --completion / --no-completion  install completion (default: install)
  --shells bash,zsh,fish          which shells (default: detect from $SHELL)
  --purge-home                    also delete the trash data on uninstall
  --cron                          show optional scheduled-cleanup recipes

examples:
  adrm setup --install
  adrm setup --uninstall
  adrm setup --uninstall --purge-home
  adrm setup --status
`)
	case "doctor":
		return i18n.Tr(`用法: adrm doctor

自检: 数据库完整性, 数据库中每条记录与磁盘的对账, 回收站中的孤立文件,
主目录权限等。

示例: adrm doctor
`, `usage: adrm doctor

self-check: database integrity, reconciliation of every database row with
the disk, orphan files in the trash, home directory permissions.

examples: adrm doctor
`)
	case "db":
		return i18n.Tr(`用法: adrm db --reset [--yes] | --path

  --reset   清空数据库(当前表 + reflog 历史)。回收站非空时会先列出并询问
            是否先彻底清空回收站。
  --path    打印数据库路径

示例:
  adrm db --path
  adrm db --reset
`, `usage: adrm db --reset [--yes] | --path

  --reset   clear the database (current items + reflog history). when the
            trash is not empty, list it and ask whether to empty it first.
  --path    print the database path

examples:
  adrm db --path
  adrm db --reset
`)
	case "undo":
		return i18n.Tr(`用法: adrm undo [--dry-run]

还原最近回收的一个文件/目录 (等价于 'adrm restore --last')。

示例: adrm undo
`, `usage: adrm undo [--dry-run]

restore the most recently recycled item (same as 'adrm restore --last').

examples: adrm undo
`)
	}
	return helpText()
}

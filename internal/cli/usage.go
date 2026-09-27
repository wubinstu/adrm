package cli

import (
	"fmt"

	"github.com/wubinstu/adrm/internal/i18n"
)

// usageText is the short usage shown when adrm is invoked without arguments.
// The English variant is strictly ASCII so it renders on any terminal.
func usageText() string {
	return i18n.Tr(`adrm `+version+` - 更安全的 rm: 删除即回收, 可随时还原

用法 / usage:
  adrm [选项] <文件...> [+持续时间|--for 时间|--until 日期]   回收(默认模式)
  adrm <子命令> [选项]                                       管理回收站

子命令 / commands:
  ls, log, restore, undo, purge, gc, empty, stats, config,
  completions, setup, doctor, db, help, version

示例 / examples:
  adrm report.pdf                回收到回收站
  adrm -rf old-project/          整体回收目录
  adrm +7d notes/                7 天后过期
  adrm ls                        查看回收站
  adrm restore 12                还原第 12 项
  adrm purge 1-5                 彻底清理第 1 到 5 项

更多帮助 / more help: 'adrm help'
`, `adrm `+version+` - a safer rm: deletes go to the trash, restorable anytime

usage:
  adrm [options] <files...> [+dur|--for dur|--until date]   recycle (default mode)
  adrm <command> [options]                                 manage the trash

commands:
  ls, log, restore, undo, purge, gc, empty, stats, config,
  completions, setup, doctor, db, help, version

examples:
  adrm report.pdf                recycle a file
  adrm -rf old-project/          recycle a directory
  adrm +7d notes/                expire in 7 days
  adrm ls                        list the trash
  adrm restore 12                restore item 12
  adrm purge 1-5                 purge items 1 to 5

more help: 'adrm help'
`)
}

// helpText is the full help. The English variant is strictly ASCII.
func helpText() string {
	return i18n.Tr(`adrm `+version+` - 带回收站功能的 rm 替代品

概述
  adrm 把 "删除" 变成 "回收": 文件被移动到回收站而不是真正删除,
  之后可以列出/还原/彻底清理。单个静态可执行文件, 无需守护进程,
  过期清理是惰性的(运行 gc 或任何命令时顺带进行)。

用法
  adrm [选项] <文件...> [有效期...]     回收模式(默认, 兼容 rm 参数)
  adrm <子命令> [选项]                  管理命令

回收模式选项 (rm 兼容)
  -f, --force          忽略不存在/无提示/无视 ignore 规则
  -i                   逐个询问 (rm 行为)
  -I                   一次回收超过阈值(默认 3 个)时询问一次
  -r, -R, --recursive  递归回收目录
  -d, --dir            回收空目录
  -v, --verbose        详细输出
  -y, --yes            所有询问默认回答 "是"
      --dry-run        只演练不实操
      --               之后的参数一律视为文件名
      --preserve-root  拒绝回收 "/" (默认开启; --no-preserve-root 关闭)
      --home PATH      指定 adrm 主目录 (等同 ADRM_HOME)
      --lang zh|en     界面语言
      --color MODE     auto|always|never

有效期 (向右亲和, 严格校验, 整条命令先校验后执行)
  +6d +20h +1s +5m     持续时间, 可组合如 +3d4h5m; 作用于其后所有文件
  --for 6d             同 +6d
  --until "2027/1/1 03:45:01"   绝对截止时间 (/ _ - 均可作分隔符)
  未指定则用配置 retention_days (默认 30 天)
  悬空的时间参数会报错, 且任何文件都不会被回收

管理命令
  ls                列出当前回收站 (过滤/排序/--json/--color)
  log               操作历史 (只增不改, 类似 git reflog)
  restore <id...>   还原; --last 最近一个; --all 全部; --replace 冲突时先回收已存在文件; --pick 交互选择
  undo              还原最近回收的一个 (= restore --last)
  purge <id...>     彻底清理; --last/--all/--yes
  gc                清理过期项 (--dry-run/--yes/--quiet)
  empty             清空回收站 (--yes)
  stats             统计信息
  config            --init 生成配置; --show 生效配置; --edit; --path; --validate
  completions        bash|zsh|fish 输出补全脚本 (--out DIR 写入文件)
  setup             --install/--uninstall/--status 安装或移除 shell 集成
  doctor            自检: 数据库/磁盘对账/孤儿文件/权限
  db                --reset 重置数据库; --path 显示数据库路径
  help [命令]       帮助
  version           版本

过滤 (ls/log/restore/purge 通用, 可组合为 AND)
  <id 列表>          1, 5, 1-20
  --state recycled|exception     按状态 (ls)
  --op recycle|restore|purge|expire|exception|reset|empty   按操作 (log)
  --expired          仅过期项
  --name PAT         原路径包含 PAT (忽略大小写)
  --path PREFIX      原路径前缀
  --older-than D     --newer-than D     按回收时间 (3d4h5m 或绝对日期)
  --mtime SPEC       按原始修改时间 (+3d 之后, -3d 之前, 无符号=恰好)
  --size SPEC        按大小 (+1g, -1k, 10g20m)
  --since D --until D    按操作时间 (log)
  --last N           最近 N 条
  --sort field:asc|desc,...   排序 (id/path/size/mtime/recycled/expire/state; log: seq/ts/op/item)

配置与环境变量
  ADRM_HOME   主目录 (默认 ~/.adrm)
  ADRM_DB     数据库路径      ADRM_TRASH  回收站目录
  ADRM_CONFIG 配置文件        ADRM_IGNORE 忽略规则文件
  ADRM_LANG   zh|en          ADRM_COLOR  auto|always|never
  配置文件: $ADRM_HOME/config (key = value, 'adrm config --init' 生成带注释模板)

安装与卸载
  curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
  adrm setup --install      # 装 shell 集成 (alias + 补全)
  adrm setup --uninstall    # 干净卸载 (只删自己写的内容)

仓库: https://github.com/wubinstu/adrm
`, `adrm `+version+` - an rm replacement with a trash bin

overview
  adrm turns "delete" into "recycle": files move into a trash directory
  instead of being destroyed, and can be listed, restored or purged later.
  One static binary, no daemon; expired items are collected lazily.

usage
  adrm [options] <files...> [expiry...]    recycle mode (default, rm-compatible)
  adrm <command> [options]                 management commands

recycle mode options (rm compatible)
  -f, --force          ignore missing files, skip prompts, override ignore rules
  -i                   prompt for each file (rm behavior)
  -I                   prompt once when more than the threshold (3) files are given
  -r, -R, --recursive  recycle directories recursively
  -d, --dir            recycle empty directories
  -v, --verbose        verbose output
  -y, --yes            answer yes to every prompt
      --dry-run        show what would happen
      --               treat everything after it as a file name
      --preserve-root  refuse to recycle "/" (default; --no-preserve-root disables)
      --home PATH      adrm home directory (same as ADRM_HOME)
      --lang zh|en     interface language
      --color MODE     auto|always|never

expiry (right-associative, strictly validated, whole command checked first)
  +6d +20h +1s +5m     durations, combinable, e.g. +3d4h5m; applies to all
                       following files until the next expiry argument
  --for 6d            same as +6d
  --until "2027/1/1 03:45:01"   absolute deadline (/ _ - separators allowed)
  when omitted, config retention_days (default 30) is used
  a dangling expiry argument is an error and nothing is recycled

management commands
  ls                list the current trash (filters/sort/--json/--color)
  log               operation history (append-only, like git reflog)
  restore <id...>   restore; --last latest; --all all; --replace recycles the
                    conflicting file first; --pick interactive choose
  undo              restore the most recently recycled item (restore --last)
  purge <id...>     permanently delete; --last/--all/--yes
  gc                purge expired items (--dry-run/--yes/--quiet)
  empty             empty the trash bin (--yes)
  stats             summary statistics
  config            --init writes a config; --show effective config; --edit;
                    --path; --validate
  completions       bash|zsh|fish scripts (--out DIR writes files)
  setup             --install/--uninstall/--status manage shell integration
  doctor            self-check: database, disk reconciliation, orphans, perms
  db                --reset clears the database; --path prints its path
  help [command]    help
  version           version

filters (shared by ls/log/restore/purge, combined with AND)
  <id list>         1, 5, 1-20
  --state recycled|exception                 by state (ls)
  --op recycle|restore|purge|expire|exception|reset|empty   by op (log)
  --expired         only expired items
  --name PAT        original path contains PAT (case-insensitive)
  --path PREFIX     original path prefix
  --older-than D --newer-than D   by recycle time (3d4h5m or absolute date)
  --mtime SPEC      by original mtime (+3d after, -3d before, none=exact)
  --size SPEC       by size (+1g, -1k, 10g20m)
  --since D --until D    by operation time (log)
  --last N          most recent N entries
  --sort field:asc|desc,...   sort keys (id/path/size/mtime/recycled/expire/
                              state; log: seq/ts/op/item)

config and environment
  ADRM_HOME   home directory (default ~/.adrm)
  ADRM_DB     database file    ADRM_TRASH  trash directory
  ADRM_CONFIG config file      ADRM_IGNORE ignore rules file
  ADRM_LANG   zh|en            ADRM_COLOR  auto|always|never
  config file: $ADRM_HOME/config (key = value; 'adrm config --init' writes an
  annotated template)

install and uninstall
  curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
  adrm setup --install      # shell integration (alias + completion)
  adrm setup --uninstall    # clean removal (only removes what adrm added)

repo: https://github.com/wubinstu/adrm
`)
}

// printHelp prints full help or per-command help.
func (a *app) printHelp(args []string) {
	if len(args) > 0 && subcommands[args[0]] {
		fmt.Fprint(a.out, cmdHelp(args[0]))
		return
	}
	fmt.Fprint(a.out, helpText())
}

# adrm — 带回收站功能的 rm 替代品

> `rm` 太危险：删掉就没了。adrm 把"删除"变成"回收"——文件被移进回收站，
> 随时可以列出、还原、彻底清理。单个静态可执行文件，无需守护进程，
> bash/zsh/fish 补全 + 一键安装 + 干净卸载。

[English](#english) · [安装](#安装) · [快速上手](#快速上手) · [命令参考](#命令参考)

---

## 特性

- **删除即回收**：`rename` 移动，瞬间完成；跨文件系统自动回退为"复制+删除"并做完整性校验
- **单文件零守护**：一个完全静态的二进制（纯 Go + 内嵌 SQLite），没有 daemon、没有 cron 依赖
- **惰性过期**：每个文件可设有效期；过期项只在运行 `adrm gc`（或任何命令顺带 `auto_gc`）时才真正删除
- **完整还原**：属主/属组/权限位（含 suid/sgid/sticky）/mtime 全部还原；目标冲突时可 `--replace` 先把冲突文件回收
- **双数据库表**：
  - `bin` 表——当前回收站里的东西（还原/清理后行即删除，id 全局单调永不复用）
  - `reflog` 表——像 `git reflog` 一样的操作历史，**只增不改**，哪怕对应文件早已被彻底清理也能翻出记录
- **强大的查询**：按 id/名字/路径/时间/大小/状态过滤，多字段排序，表格/JSON 输出，可配置列
- **shell 补全**：bash/zsh/fish 三套内嵌脚本，**动态候选直连数据库**——输 `adrm restore <TAB>` 直接列出回收站里的 id 和文件名
- **zh/en 双语**：`lang = en` 时所有人类可读输出严格 ASCII（老终端/串口/Windows 默认控制台也能看），`zh` 时使用中文
- **rm 参数兼容**：`-f -i -I -r -R -d -v --` 以及 rm 的提示/退出码语义
- **严格校验**：整条命令行先通过验证才执行；悬空的持续时间参数会报错且任何文件都不会被动

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
```

安装脚本会：

1. 询问二进制安装位置（默认 `~/.local/bin`）、adrm 主目录（默认 `~/.adrm`）、是否创建 `rm` 别名（默认是）
2. 从 GitHub Releases 下载对应平台的静态二进制并**校验 SHA256**
3. 调用 `adrm setup --install` 写入 shell 集成（带标记块，卸载时可精确移除）
4. 生成默认配置并跑一次 `adrm doctor` 自检

> `rm` 别名只作用于交互式 shell，脚本里的 `rm` 仍然是 `/bin/rm`，互不影响。
> 想用真正的删除：`command rm ...`，或先回收再 `adrm purge`。

**手动安装**：

```bash
# 从 Releases 下载对应平台二进制到 PATH
install -m0755 adrm-linux-x86_64 ~/.local/bin/adrm

adrm setup --install          # shell 集成(别名+补全)
adrm config --init            # 生成带注释的默认配置
```

**卸载**（干净，只删 adrm 自己写的东西）：

```bash
adrm setup --uninstall                 # 移除 shell 集成(rc 标记块/补全/init)
bash install.sh --uninstall            # 连二进制一起删
bash install.sh --uninstall --purge-home   # 连同回收站数据一起删(二次确认)
```

## 快速上手

```bash
$ adrm report.pdf                      # 回收一个文件
recycled '/home/u/report.pdf' (id=1, 2.1M, expires in 30d)

$ adrm -rf old-project/                # 整体回收目录
$ adrm +7d build.log                   # 7 天后过期
$ adrm --until "2027/01/01" archive/   # 到 2027-01-01 才过期
$ adrm ls                              # 看看回收站里有什么
+----+---------------------------+-------+---------------------+---------------------+---------------------+-------+
 id | path                       | size  | mtime               | recycled            | expires             | left  |
+----+---------------------------+-------+---------------------+---------------------+---------------------+-------+
  2 | /home/u/build.log           | 12K   | 2026-05-01 09:12:00 | 2026-05-05 10:00:00 | 2026-05-12 10:00:00 |  7d   |
  1 | /home/u/report.pdf          | 2.1M  | 2026-04-30 18:44:10 | 2026-05-05 10:00:00 | 2026-06-04 10:00:00 | 30d   |
+----+---------------------------+-------+---------------------+---------------------+---------------------+-------+

$ adrm undo                            # 还原最近删的一个
restored '/home/u/build.log' (id=2)

$ adrm restore 1                       # 按 id 还原
$ adrm purge 1                         # 彻底清理(真删, 不可恢复)
$ adrm log --name report               # 翻历史记录
```

## 设计取舍（重要）

| 问题 | 决策 | 理由 |
|---|---|---|
| 单用户还是多用户？ | **单用户**（可见范围 = 本用户），用 `ADRM_HOME`/`--home` 支持多个独立回收站 | 回收站里是刚删的敏感内容，不该被别人浏览；还原要恢复 uid/gid，共享回收站对非 root 用户必然大量失败 |
| 单个可执行文件还是守护进程？ | **单文件，零守护进程**，惰性 GC | 守护进程对个人工具是过度设计（休眠/容器/重启全是坑）；截止日期是显式意图，运行命令时顺带清理即可 |
| 语言 | Go（纯 Go SQLite，CGO 关闭，完全静态） | 单二进制、跨 linux/macOS、供应链干净（唯一依赖是 SQLite 本身） |
| 旧版接口 | **全部废弃**，v2 全新 CLI | v1 的 `--query/--restore/--clean` 混在 rm 参数里又难记又难补全 |

## 命令参考

### 默认模式：回收（rm 兼容）

```
adrm [选项] <文件/目录...> [有效期...]
```

| 选项 | 说明 |
|---|---|
| `-f, --force` | 忽略不存在文件/跳过询问/无视 ignore 规则 |
| `-i` | 逐个询问（rm 行为） |
| `-I` | 一次回收超过阈值（默认 3 个）时询问一次 |
| `-r, -R, --recursive` | 递归回收目录 |
| `-d, --dir` | 回收空目录 |
| `-v, --verbose` | 详细输出 |
| `-y, --yes` | 所有询问默认"是" |
| `--dry-run` | 只演练不实操 |
| `--` | 之后一律视为文件名（`adrm -- -file`） |
| `--preserve-root` / `--no-preserve-root` | 拒绝回收 `/`（默认开启） |
| `--home PATH`、`--lang zh|en`、`--color MODE` | 全局选项 |

**有效期（向右亲和，严格校验）**：

```bash
adrm +6d a b                  # a、b 都 6 天后过期
adrm a +1d b +2h c            # a 用默认(30天), b 1天, c 2小时
adrm a +3d4h5m                # 组合单位
adrm --for 6d a b             # 同 +6d
adrm --until "2027/1/1 03:45:01" a   # 绝对截止(支持 / _ - 分隔)
adrm a +2d                    # ❌ 报错: 行尾持续时间没有对应文件, 任何文件都不会被回收
```

### 管理命令

| 命令 | 说明 |
|---|---|
| `adrm ls [过滤] [排序]` | 当前回收站（bin 表） |
| `adrm log [过滤]` | 操作历史（reflog，只增不改） |
| `adrm restore <id...> \| --last \| --all \| --pick` | 还原（`--replace` 冲突时先回收已存在文件） |
| `adrm undo` | = `restore --last` |
| `adrm purge <id...> \| --last \| --all` | 彻底清理（真删） |
| `adrm gc [--dry-run] [--yes] [--quiet]` | 清理所有过期项 |
| `adrm empty [--yes]` | 清空回收站 |
| `adrm stats` | 统计 |
| `adrm config --init\|--show\|--edit\|--path\|--validate` | 配置 |
| `adrm completions bash\|zsh\|fish [--out DIR]` | 输出补全脚本 |
| `adrm setup --install\|--uninstall\|--status` | shell 集成管理 |
| `adrm doctor` | 自检（数据库完整性/孤儿对账/权限） |
| `adrm db --reset\|--path` | 数据库维护 |

### 过滤器（ls / log / restore / purge 通用，可组合 = AND）

| 过滤 | 示例 | 说明 |
|---|---|---|
| 位置参数 id | `adrm purge 1 5 10`、`1-20`、`1,3,5-8` | 支持区间 |
| `--state` | `--state recycled\|exception` | ls 按状态 |
| `--op` | `log --op purge` | log 按操作类型 |
| `--expired` | `ls --expired` | 已过期还没清理的 |
| `--name` | `--name report` | 原路径子串，忽略大小写 |
| `--path` | `--path ~/projects` | 原路径前缀 |
| `--older-than` / `--newer-than` | `--older-than 3d`、`--older-than "2027-1-1"` | 按回收时间 |
| `--mtime` | `--mtime +3d` / `-3d` / 无符号=恰好 | 按原始修改时间 |
| `--size` | `--size +1g` / `-1k` / `10g20m` | 按大小 |
| `--since` / `--until` | `log --since 7d` | log 按操作时间 |
| `--last N` | `ls --last 10` | 最近 N 条 |
| `--sort` | `--sort size:desc,id:asc` | 多字段排序，按出现优先级 |
| `--json` / `--color` | | 输出控制 |

## 配置与环境变量

`~/.adrm/config`（`adrm config --init` 生成带注释模板）：

```ini
retention_days = 30        # 默认有效期(天)
auto_gc = true             # 命令运行时顺带清理过期项(有产出才提示)
prompt_threshold = 3       # -I 的询问阈值
preserve_root = true
copy_fallback = true       # 跨盘时复制+删除兜底
lang = auto                # auto|zh|en, en 模式输出纯 ASCII
color = auto               # auto|always|never (尊重 NO_COLOR)
ls_columns = id, path, size, mtime, recycled, expire, left
log_columns = seq, ts, op, id, path, detail
max_list = 500             # 列表输出上限(超出会打印省略提示, 不静默截断)
```

| 环境变量 | 作用 |
|---|---|
| `ADRM_HOME` | 主目录（默认 `~/.adrm`），换它就换了整个回收站 |
| `ADRM_DB` / `ADRM_TRASH` / `ADRM_CONFIG` / `ADRM_IGNORE` | 覆盖各个文件路径 |
| `ADRM_LANG` / `ADRM_COLOR` | 覆盖语言/颜色 |
| `ADRM_AUTO_GC=0` / `ADRM_RETENTION_DAYS=7` | 覆盖行为配置 |

## ignore 规则（gitignore 语法）

`~/.adrm/ignore`，命中则默认只提示不回收（`-f` 无视）。匹配比 gitignore 更直观：
每个模式依次对 **绝对路径 / `$HOME` 相对路径 / 裸文件名** 尝试匹配。

```gitignore
*.tmp                  # 任何 .tmp
secret/                # 任何名为 secret 的目录
/var/log/*.log         # 前导 / = 文件系统绝对路径
Downloads/*.iso        # $HOME/Downloads 下的 iso
build/**               # build 目录内的所有内容
!keep.tmp              # 取反: keep.tmp 重新参与正常回收
```

## 回收站里长什么样

```
~/.adrm/                            # 0700
├── config                          # 配置
├── ignore                          # gitignore 规则
├── adrm.db                         # SQLite (WAL, busy_timeout)
├── trash/
│   └── 20260505-134426_000123/     # 批次目录: 时间_id, 可读可逛
│       ├── report.pdf
│       └── old-project/
├── completions/{adrm.bash,_adrm,adrm.fish}
└── adrm-init.sh                    # shell 集成入口
```

数据库两张表：

- **`bin`**：当前回收站。进出回收站即删行；id 用 AUTOINCREMENT 全局分配，**永不复用**
- **`reflog`**：操作历史。每次 bin 变更都在**同一事务**里追加一条
  （recycle/restore/purge/expire/exception/reset/empty），永不修改、永不删除——
  文件没了，记录还在

## shell 补全

```bash
adrm completions bash > ~/.local/share/bash-completion/completions/adrm
adrm completions zsh  --out ~/.zfunc     # 并在 ~/.zshrc 加: fpath=(~/.zfunc $fpath); compinit
adrm completions fish > ~/.config/fish/completions/adrm.fish
# 或让 adrm 自己管:
adrm setup --install
```

补全内容：子命令、全部选项（带简短说明）、枚举值（`--state`/`--op`/`--color`/`--lang`）、
排序字段，以及**动态候选**：`adrm restore <TAB>` 列出回收站里的 id（附带原路径），
`--name <TAB>` 列出历史文件名，`--path <TAB>` 列出历史目录。bash/zsh 下同时对 `rm`
注册补全（因为 bash 的补全按字面单词查找，不会展开 alias）。

## 安全语义

- 拒绝回收 `/`（`--no-preserve-root` 显式关闭）、`.`/`..`、adrm 主目录及其父目录、回收站自身
- 回收站内的一切路径以数据库为准，绝不用原名反推——重名自动加 `.1` `.2` 后缀，不会互相覆盖
- 跨文件系统：先 `rename`，失败则复制（保留属性）→ 校验大小 → 才删原文件，原文件永不凭空消失
- 进程在"文件已移动、DB 未写"之间被杀：`adrm doctor` 能发现孤儿，可交互清理或登记
- 还原前检查目标冲突：默认拒绝，`--replace` 先把冲突文件**回收**再还原——任何数据都不会静默丢失

## 开发

```bash
go build -o adrm .            # 构建
go test ./...                 # 单元测试(表驱动, 含 en 纯 ASCII 断言)
bash tests/integration.sh     # 85+ 项端到端测试(含 tmpfs 跨盘/并发/补全/安装卸载)
```

代码结构：`internal/{cli,config,ignore,model,store,trash,x,render,i18n,complete}`，
唯一外部依赖 `modernc.org/sqlite`（纯 Go 静态编译）。

---

<a id="english"></a>

## English

**adrm** is a safer `rm`: everything you delete moves into a per-user trash bin and can be
listed, restored, or purged later. One static binary, no daemon, bash/zsh/fish completion,
one-line install, clean uninstall.

```bash
# install
curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash

# use
adrm report.pdf            # recycle (default mode, rm-compatible flags)
adrm -rf old-project/      # recycle a directory recursively
adrm +7d notes/            # expire in 7 days
adrm ls                    # list the trash bin
adrm undo                  # restore the most recently recycled item
adrm restore 12 --replace  # restore by id, recycling conflicts first
adrm purge 1-5             # permanently delete items 1..5
adrm gc                    # purge everything past its deadline
adrm log --since 7d        # operation history (append-only, like git reflog)

# manage
adrm config --init         # annotated default config
adrm completions bash      # print a completion script
adrm setup --uninstall     # clean removal of shell integration
```

Key properties:

- **Trash semantics**: `rename` first, copy+delete fallback across filesystems (with a size
  check before the original is deleted). Restore replays owner, group, full permission bits
  (suid/sgid/sticky) and mtime.
- **Two tables**: `bin` (current trash; rows are removed when items leave; ids never reused)
  and `reflog` (append-only history that survives the files it describes).
- **Lazy expiry**: `auto_gc` collects expired items when any command runs; `adrm gc` does it
  explicitly. No daemon anywhere.
- **Language**: `lang = en` guarantees pure-ASCII output on every terminal; `zh` uses Chinese.
- **Filters**: `--state/--op/--name/--path/--older-than/--newer-than/--mtime/--size/--since/
  --until/--last`, multi-key `--sort`, `--json`, configurable columns.
- **Completion**: embedded bash/zsh/fish scripts with live candidates read straight from the
  database (trash ids, names, paths) via a hidden `adrm __complete` subcommand.

License: MIT.

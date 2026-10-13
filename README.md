# adrm — 带回收站功能的 rm 替代品

`adrm` 把「删除」变成「回收」：文件被移进回收站而不是被销毁，之后随时可以查看、还原或彻底清理。

项目由**两个二进制**组成，分工明确：

| 二进制 | 作用 | 补全行为 |
|--------|------|----------|
| `adrm` | 删除文件，参数兼容 `/bin/rm` | 只补全**文件路径**（和 `rm` 一样） |
| `admr` | 管理回收站（查看/还原/清理/配置） | 只补全**子命令**（和 `git` 一样） |

这样 `rm <TAB>` 永远不会和子命令混在一起，两个词表严格分离。

- 单个静态可执行文件，**无守护进程**，纯 Go + 内嵌 SQLite（CGO 关闭，glibc 无关）
- **每用户独立**：程序运行时自动使用当前用户的 `~/.adrm`，二进制和补全全局唯一
- MIT 许可证

---

## 功能总览

| 功能 | 命令 | 说明 |
|------|------|------|
| 回收文件/目录 | `adrm <文件...>` | rm 兼容参数 `-f -i -I -r -R -d -v --`，整条命令先校验后执行 |
| 设置有效期 | `+3d` / `--for 3d` / `--deadline "2027/1/1"` | 向右亲和；未指定默认 30 天 |
| 查看回收站 | `admr ls` | 表格/JSON，可按名称、大小、时间、状态过滤排序 |
| 操作历史 | `admr log` | 只增不改的流水账（类似 git reflog），文件真删后记录仍在 |
| 还原 | `admr restore` / `admr undo` | 还原属主、属组、权限位（含 suid/sgid/sticky）、时间戳 |
| 彻底清理 | `admr purge` / `gc` / `empty` | 单项、按条件、清空、清过期 |
| 统计 | `admr stats` | 当前项数、占用、历史操作计数 |
| 配置 | `admr config --init` | 生成带注释的配置模板 |
| 补全 | `admr completions bash\|zsh\|fish` | 内嵌三种 shell 补全脚本 |
| 安装集成 | `admr setup --install` | 全局补全、可选 rm 别名、可选定时清理 |
| 自检 | `admr doctor` | 数据库完整性、磁盘对账、孤立文件、权限 |
| 数据库 | `admr db --path / --reset` | 位置查询、安全重置 |

---

## 安装

```bash
curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
```

默认行为（`root` 或 `/usr/bin` 可写时）：

- 两个二进制安装到 `/usr/bin/adrm`、`/usr/bin/admr`
- 补全脚本安装到系统目录（bash/zsh/fish 各自的全局目录），所有用户可用
- **不**创建 `rm` 别名，`rm` 行为完全不变
- 每个用户第一次运行时自动创建自己的 `~/.adrm`

可选附加（按需追加参数）：

```bash
bash install.sh --alias    # 交互式 shell 里 alias rm=adrm，并接管 rm 的补全
bash install.sh --timer    # 安装 systemd --user 定时清理（每天一次）
bash install.sh -y         # 全部使用默认值，不提问
```

环境变量覆盖：`ADRM_VERSION`（版本）、`ADRM_INSTALL_DIR`（安装目录）、
`ADRM_HOME`（家目录）、`ADRM_ALIAS`、`ADRM_TIMER`、`ADRM_SHELLS`。

> 想先看看再决定：`bash install.sh --help`

---

## 快速上手

```bash
adrm report.pdf
```

```
已回收 '/home/you/report.pdf' (id=1, 19 B, 30 天后过期)
没有过期的回收项。
```

```bash
adrm -r old-project/
```

```
已回收 '/home/you/old-project' (id=2, 4 B, 30 天后过期)
没有过期的回收项。
```

```bash
admr ls
```

```
┌──────┬──────────────────────────┬──────┬─────────────────────┬─────────────────────┬─────────────────────┬──────┐
 编号 │ 原路径                   │ 大小 │ 修改时间            │ 回收时间            │ 过期时间            │ 剩余 
├──────┼──────────────────────────┼──────┼─────────────────────┼─────────────────────┼─────────────────────┼──────┤
    2 │ /home/you/old-project/   │  4 B │ 2026-10-13 17:41:46 │ 2026-10-13 17:41:46 │ 2026-11-12 17:41:46 │  30d 
    1 │ /home/you/report.pdf     │ 19 B │ 2026-10-13 17:41:46 │ 2026-10-13 17:41:46 │ 2026-11-12 17:41:46 │  30d 
└──────┴──────────────────────────┴──────┴─────────────────────┴─────────────────────┴─────────────────────┴──────┘
```

```bash
admr restore 1
```

```
已还原 '/home/you/report.pdf' (id=1)
```

目录在列表里带结尾 `/`，一眼区分文件和目录。

---

## 第一章 · 删除文件（adrm）

### rm 兼容选项

| 选项 | 含义 |
|------|------|
| `-f`, `--force` | 忽略不存在的文件、跳过询问、无视 ignore 规则 |
| `-i`, `--interactive` | 逐个询问（与 `/bin/rm` 行为一致） |
| `-I` | 一次回收超过阈值（默认 3 个）时询问一次 |
| `-r`, `-R`, `--recursive` | 递归回收目录 |
| `-d`, `--dir` | 回收空目录 |
| `-v`, `--verbose` | 详细输出 |
| `-y`, `--yes` | 所有询问默认回答「是」 |
| `--dry-run` | 只演练不实操 |
| `--preserve-root` / `--no-preserve-root` | 是否拒绝回收 `/`（默认拒绝） |
| `--home PATH` | 指定 adrm 主目录（等同 `ADRM_HOME`） |
| `--lang zh\|en` | 界面语言 |
| `--color auto\|always\|never` | 颜色模式 |

### 示例

```bash
adrm -rf old-project/
```

```bash
adrm -i file1 file2
```

```
remove regular empty file '/home/you/file1'? y
已回收 '/home/you/file1' (id=3, 0 B, 30 天后过期)
remove regular empty file '/home/you/file2'? n
```

（提示文案与 `/bin/rm` 一致：`remove regular empty file 'x'?`，`y` 回收、`n` 跳过。）

```bash
adrm -- -weird-name
```

`--` 之后的参数一律视为文件名，用来删除以 `-` 或 `+` 开头的文件。

### 整条命令先校验后执行

命令行有任何错误（未知选项、时间参数悬空、非法值）时**直接退出码 2，一个文件都不会动**：

```bash
adrm +1d a.txt +2d b.txt +3d
```

```
adrm: 错误: 行尾的持续时间参数没有对应的文件 (持续时间向右亲和, 必须后接文件)
```

---

## 第二章 · 有效期

回收站里的东西不会永久保存。默认 30 天后过期（`retention_days` 可改），过期后由
`admr gc` 或安装的定时器清理。

### 三种写法

| 写法 | 例子 | 含义 |
|------|------|------|
| 持续时间 | `+3d`、`+20h`、`+1s`、`+5m` | 相对当前时间，可组合 `+3d4h5m` |
| `--for` | `--for 6d` | 同 `+6d` |
| `--deadline` | `--deadline "2027/1/1"` | 绝对截止时间 |

没有「月」和「年」单位：更长时间请写 `+730d`，或用 `--deadline`。

### 绑定规则（向右亲和）

有效期参数作用于它**后面**的文件，直到遇到下一个有效期参数：

```bash
adrm +1d a.txt b.txt +2d c.txt
```

```
已回收 '/home/you/a.txt' (id=1, 0 B, 1 天后过期)
已回收 '/home/you/b.txt' (id=2, 0 B, 1 天后过期)
已回收 '/home/you/c.txt' (id=3, 0 B, 2 天后过期)
```

单独位于命令末尾的一个有效期参数作用于整条命令的所有文件：

```bash
adrm a.txt --deadline "2027/01/01"
```

```
已回收 '/home/you/a.txt' (id=4, 0 B, 80 天后过期)
```

> 多个有效期参数 + 末尾还有一个悬空的，属于歧义，报错退出（见上节）。

### `--deadline` 支持的日期时间格式

分隔符 `/` `_` `-` 任意混用；时间部分可省略（默认 00:00:00），也可只写 `HH:MM`。

```bash
adrm file1 --deadline "2027/01/01"          有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027/1/01"           有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027/01/1"           有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027/1/1"            有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027_01_01"          有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027_1_01"           有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027_01_1"           有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027_1_1"            有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027-01-01"          有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027-1-01"           有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027-01-1"           有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027-1-1"            有效直到 2027年1月1日 00:00:00
adrm file1 --deadline "2027/1/1 3:45"       有效直到 2027年1月1日 03:45:00
adrm file1 --deadline "2027/1/1 03:45"      有效直到 2027年1月1日 03:45:00
adrm file1 --deadline "2027/1/1 03:45:01"   有效直到 2027年1月1日 03:45:01
adrm file1 --deadline "2027_1_1 3:45"       有效直到 2027年1月1日 03:45:00
adrm file1 --deadline "2027_1_1 03:45"      有效直到 2027年1月1日 03:45:00
adrm file1 --deadline "2027_1_1 03:45:01"   有效直到 2027年1月1日 03:45:01
adrm file1 --deadline "2027-1-1 3:45"       有效直到 2027年1月1日 03:45:00
adrm file1 --deadline "2027-1-1 03:45"      有效直到 2027年1月1日 03:45:00
adrm file1 --deadline "2027-1-1 03:45:01"   有效直到 2027年1月1日 03:45:01
```

（`--until` 是 `--deadline` 的等价别名，两种拼写都可以。）

### 过期清理

没有守护进程，过期文件不会被实时删除。清理发生在：

- 手动 `admr gc`
- 运行其他命令时顺带惰性清理（有产出才提示一行）
- 安装了 `--timer` 时由 systemd 每天执行一次

```bash
admr gc
```

```
2 个回收项已过期:
  #2 /home/you/old.iso
  #3 /home/you/cache.tmp
全部彻底清理这 2 个过期项? y
已清理 2 个过期项。
```

```bash
admr gc --dry-run
```

```
演练模式, 1 个过期项将被清理:
  #4 /home/you/exp2.txt (0 B)
```

```bash
admr gc --yes
```

（`--yes` 跳过询问直接清理，适合脚本和定时器。）

---

## 第三章 · 查看回收站（admr ls）

```bash
admr ls
```

默认列出当前回收站所有项，最新在上。

```bash
admr ls --state exception
```

```bash
admr ls --expired
```

```bash
admr ls --json
```

```json
[
  {
    "id": 1,
    "path": "/home/you/report.pdf",
    "is_dir": false,
    "size": 19,
    "size_human": "19 B",
    "mode": 420,
    "mode_octal": "0644",
    "uid": 1000,
    "gid": 1000,
    "owner": "you",
    "group": "you",
    "mtime": "2026-10-13T17:41:46+08:00",
    "recycled_at": "2026-10-13T17:41:46+08:00",
    "expire_at": "2026-11-12T17:41:46+08:00",
    "state": "recycled",
    "trash_path": "/home/you/.adrm/trash/20261013-174146_000001/report.pdf"
  }
]
```

JSON 适合脚本消费；列集合可在配置文件里自定义（`columns`）。

---

## 第四章 · 操作历史（admr log）

回收站只反映「现在有什么」，`log` 是**只增不改**的流水账：每次回收、还原、清理、
过期、重置都记一条，文件被真正删除后记录依然在。

```bash
admr log
```

```
┌──────┬─────────────────────┬─────────┬────┬──────────────────────────┬───────────────┐
 序号 │ 时间                │ 操作    │ 项 │ 原路径                   │ 详情          
├──────┼─────────────────────┼─────────┼────┼──────────────────────────┼───────────────┤
    2 │ 2026-10-13 17:41:46 │ recycle │  2 │ /home/you/old-project    │ method=rename 
    1 │ 2026-10-13 17:41:46 │ recycle │  1 │ /home/you/report.pdf     │ method=rename 
└──────┴─────────────────────┴─────────┴────┴──────────────────────────┴───────────────┘
```

```bash
admr log --op purge
```

```bash
admr log --since 3d --deadline 2027/1/1
```

```bash
admr log --name report --json
```

编号（`seq`）单调递增、永不复用，可以放心用来引用历史。

---

## 第五章 · 还原（admr restore / undo）

还原会把文件放回原始路径，并恢复**属主、属组、全部权限位（含 suid/sgid/sticky）、
修改时间**。

```bash
admr restore 1
```

```
已还原 '/home/you/report.pdf' (id=1)
```

```bash
admr undo
```

```
已还原 '/home/you/z.txt' (id=3)
```

`undo` 等价于 `restore --last`，还原最近回收的那一个。

其他用法：

```bash
admr restore --last          # 最近一个
admr restore 1 3 5           # 多个编号
admr restore 1-20            # 编号区间
admr restore --all           # 全部
admr restore --pick          # 交互式勾选
```

目标位置已有同名文件时默认拒绝，加 `--replace` 会**先把冲突文件回收、再还原选中项**：

```bash
admr restore 1 --replace
```

```
已回收 '/home/you/report.pdf' (id=9, 19 B, 30 天后过期)
没有过期的回收项。
已还原 '/home/you/report.pdf' (id=1)
```

跨文件系统（例如回收站和原文件不在同一分区）时自动走「复制 + 校验大小 + 删除原文件」，
不会出现「复制失败但源文件已删」的事故。

---

## 第六章 · 彻底清理（purge / gc / empty）

```bash
admr purge 2
```

```
已彻底清理 1 项。
```

```bash
admr purge --last
```

```bash
admr purge 1-5 --yes
```

```bash
admr purge --name "*.tmp" --dry-run
```

```bash
admr empty
```

```
将彻底清空回收站中的 3 项(不可恢复):
  #1 /home/you/report.pdf
  #2 /home/you/old-project
  #5 /home/you/notes.tmp
确认清空回收站? y
已彻底清理 3 项。
```

```bash
admr empty --yes
```

清理后 `bin` 表对应行被删除，但 `log` 里的历史记录保留。

---

## 第七章 · 过滤与排序

`ls` / `log` / `restore` / `purge` 共享同一套过滤器，可任意组合（逻辑 AND）。

| 过滤器 | 例子 | 含义 |
|--------|------|------|
| 编号 | `1`、`1,5`、`1-20` | 位置参数，选中回收站编号 |
| `--state` | `recycled`、`exception` | 按状态（ls） |
| `--op` | `recycle`、`restore`、`purge`、`expire`、`reset`、`empty` | 按操作（log） |
| `--expired` | — | 仅过期项 |
| `--name` | `report` | 原路径包含该串（忽略大小写） |
| `--path` | `/home/you/` | 原路径前缀 |
| `--older-than` / `--newer-than` | `3d`、`2027/1/1` | 按回收时间 |
| `--mtime` | `+3d`、`-3d`、`3d4h` | 按原文件修改时间 |
| `--size` | `+1g`、`-1k`、`10g20m` | 按大小 |
| `--since` / `--deadline` | `3d`、`2027/1/1` | 按操作时间（log） |
| `--last` | `10` | 最近 N 条 |
| `--sort` | `size:desc,id:asc` | 多字段排序 |

```bash
admr ls --size +1k --name pdf --sort size:desc
```

```bash
admr log --op restore --last 5
```

时间参数接受与 `--deadline` 相同的全部日期格式（见第二章表格），
以及 `3d4h5m` 这类持续时间和带方向的 `+3d`（之后）/ `-3d`（之前）。

---

## 第八章 · ignore 规则

`~/.adrm/ignore` 使用 **gitignore 语法**。命中规则的文件**只提示、不回收**，
除非加 `-f` 无视规则。

```gitignore
# ~/.adrm/ignore
*.tmp
node_modules/
/important-project/
```

```bash
adrm notes.tmp
```

```
已跳过(命中 ignore 规则): /home/you/notes.tmp  (用 -f 强制执行)
```

```bash
adrm -f notes.tmp
```

```
已回收 '/home/you/notes.tmp' (id=7, 12 B, 30 天后过期)
```

支持 `!` 取反、`**` 通配、行尾 `/` 表示目录、`#` 注释。

---

## 第九章 · 配置与环境变量

```bash
admr config --init      # 生成带完整注释的配置模板
admr config --show      # 查看生效配置
admr config --edit      # 用 $EDITOR 编辑
admr config --path      # 配置文件路径
admr config --validate  # 校验
```

常用配置项（`~/.adrm/config`，`key = value`）：

| 键 | 默认 | 说明 |
|----|------|------|
| `retention_days` | `30` | 默认保留天数 |
| `prompt_threshold` | `3` | `-I` 询问阈值 |
| `auto_gc` | `true` | 运行其他命令时顺带清理过期项 |
| `columns` | 内置默认 | `ls` 显示哪些列、顺序如何 |
| `lang` | `auto` | `auto` / `zh` / `en` |
| `color` | `auto` | `auto` / `always` / `never` |
| `trash_dir` / `database` / `ignore_file` | 主目录下 | 可改到别处 |

环境变量（优先级高于配置文件）：

| 变量 | 作用 |
|------|------|
| `ADRM_HOME` | 主目录（默认 `~/.adrm`） |
| `ADRM_DB` | 数据库路径 |
| `ADRM_TRASH` | 回收站目录 |
| `ADRM_CONFIG` | 配置文件路径 |
| `ADRM_IGNORE` | ignore 规则路径 |
| `ADRM_LANG` | `zh` / `en` |
| `ADRM_COLOR` | `auto` / `always` / `never` |
| `ADRM_AUTO_GC` | `0` 关闭惰性清理 |
| `ADRM_PROMPT_THRESHOLD` | `-I` 阈值 |

`en` 模式下所有人读输出都是纯 ASCII（表格边框用 `+---+`），老终端/串口也能正常显示。

---

## 第十章 · shell 补全

安装时已写入系统目录，开新终端即可用。也可以手动取用脚本：

```bash
admr completions bash
admr completions zsh
admr completions fish
admr completions bash --out ~/.local/share/bash-completion/completions
```

补全内容：

| 位置 | 补全内容 |
|------|----------|
| `adrm <TAB>` | 当前目录**文件**（和 `rm` 一致） |
| `adrm -<TAB>` | adrm 自己的选项 |
| `admr <TAB>` | **子命令**（ls/log/restore/...） |
| `admr restore <TAB>` | 回收站编号（实时读数据库，附原路径） |
| `admr ls --name <TAB>` | 数据库里的历史文件名 |
| `admr ls --path <TAB>` | 数据库里的历史目录 |
| `admr ls --state <TAB>` | 枚举值 |
| `admr config --<TAB>` | 该子命令的选项 |

子命令和文件**永远不在同一屏出现**：`adrm` 只出文件，`admr` 只出子命令。

---

## 第十一章 · rm 别名（可选）

默认不装别名，`rm` 保持原样。想要「删除即回收」的完整体验：

```bash
admr setup --install --alias
```

```bash
bash install.sh --alias
```

效果（写入你的 shell rc，带标记块，可干净移除）：

```bash
alias rm='adrm'
complete -r rm 2>/dev/null || true
complete -o default -o filenames -F _adrm_complete rm
```

即：交互式 shell 里 `rm` 变成 `adrm`，同时 `rm <TAB>` 的文件补全改由 adrm 提供。

几点说明：

- 别名只在**交互式 shell** 生效；脚本里的 `#!/bin/bash` 调用 `rm` 不受影响
- 任何时候可以用 `command rm xxx` 或 `/bin/rm xxx` 走原生 rm
- 逃生通道：`admr purge --all --yes` 清空回收站，`admr setup --uninstall` 移除集成

---

## 第十二章 · systemd 定时清理（可选）

```bash
admr setup --install --timer
```

生成并启用：

- `~/.config/systemd/user/adrm-gc.service`（`ExecStart=/usr/bin/admr gc --quiet`）
- `~/.config/systemd/user/adrm-gc.timer`（`OnCalendar=daily`，`Persistent=true`）

```bash
systemctl --user list-timers adrm-gc.timer
```

没有 systemd 用户会话时会写入单元文件并提示手动启用，不会报错中断安装。
卸载时 `admr setup --uninstall` 会一并停用并删除。

---

## 第十三章 · 自检、统计与数据库

```bash
admr doctor
```

```
adrm 自检报告:
  [ok] database integrity /home/you/.adrm/adrm.db
  [ok] trash copies       2 item(s) present
  [ok] trash directory    /home/you/.adrm/trash
  [ok] orphans            none
  [ok] recorded sizes     match
  [ok] home directory     /home/you/.adrm (0700)
  [ok] config file        not present, using defaults ('adrm config --init' to create)
一切正常。
```

```bash
admr stats
```

```
回收站: /home/you/.adrm/trash
  当前项数: 2 (目录 1, 文件 1)
  其中: 过期 0, 异常 0
  记录总大小: 23 B, 实际占用: 4.0 KB
  历史记录: 2 条 ( recycle 2 / restore 0 / purge 0 / expire 0 / exception 0 / reset 0 / empty 0 )
```

```bash
admr db --path
```

```
/home/you/.adrm/adrm.db
```

```bash
admr db --reset
```

回收站非空时会先列出并询问是否先彻底清空，空则直接重置。数据库使用 WAL 模式 +
`busy_timeout`，多个 adrm 进程并发写不会互相踩。

---

## 子命令速查表

| 命令 | 作用 |
|------|------|
| `admr ls` | 列出当前回收站 |
| `admr log` | 操作历史（只增不改） |
| `admr restore <id...>` | 还原；`--last` / `--all` / `--replace` / `--pick` |
| `admr undo` | 还原最近回收的一项 |
| `admr purge <id...>` | 彻底清理；`--last` / `--all` / `--yes` |
| `admr gc` | 清理过期项；`--dry-run` / `--yes` / `--quiet` |
| `admr empty` | 清空回收站；`--yes` |
| `admr stats` | 统计信息 |
| `admr config` | `--init` / `--show` / `--edit` / `--path` / `--validate` |
| `admr completions` | 输出补全脚本；`--out DIR` |
| `admr setup` | `--install` / `--uninstall` / `--status` / `--alias` / `--timer` |
| `admr doctor` | 自检 |
| `admr db` | `--path` / `--reset` |
| `admr help [命令]` | 帮助 |
| `admr version` | 版本 |

任何命令加 `--help` 看该命令的详细帮助。

---

## 卸载

```bash
bash install.sh --uninstall              # 保留 ~/.adrm（回收站数据）
bash install.sh --uninstall --purge-home # 连回收站一起删
```

或只用二进制：

```bash
admr setup --uninstall             # rc 块、补全脚本、timer
admr setup --uninstall --purge-home # 再加 ~/.adrm
```

只删自己写的内容：rc 文件里带 `# >>> adrm >>>` / `# <<< adrm <<<` 标记的块被整段移除，
其他行一概不动。

---

## 设计取舍

- **单用户、多回收站**：每个用户有自己的 `~/.adrm`（0700）。回收站含敏感文件名和内容，
  不该被别人读；还原属主需要权限；SQLite 单写者。用 `ADRM_HOME` 或 `--home` 可切换回收站。
- **两张表**：`bin` 记录「当前回收站里有什么」（还原/清理后行即删除），`log` 是只增的
  流水账。编号全局分配、永不复用，所以历史引用永远稳定。
- **惰性 GC，无守护进程**：过期不实时删除，`gc` / 其他命令顺带 / 可选 timer 三者取其一。
- **严格校验后执行**：命令行有问题就整体退出，不会「删了一半才发现参数错了」。
- **`trash_path` 是唯一真相**：文件在磁盘上的实际位置以数据库记录为准，doctor 会做对账。

---

## 开发

```bash
go build -o adrm .            # 删除面孔
go build -o admr ./cmd/admr # 管理面孔
go test ./...
bash tests/integration.sh     # 85 项集成测试
```

唯一外部依赖是 `modernc.org/sqlite`（纯 Go 转译的 SQLite），因此可以 `CGO_ENABLED=0`
全静态构建，不依赖 glibc 版本。

代码结构：`internal/cli`（参数解析与命令分发）、`internal/store`（双表 SQLite）、
`internal/trash`（回收/还原/清理引擎）、`internal/model`（时间/大小语法）、
`internal/ignore`（gitignore 匹配）、`internal/render`（表格与颜色）、
`internal/i18n`（中英文）、`internal/complete`（补全脚本）。

---

## English

**adrm** turns `rm` into a recycle bin. Two binaries, one vocabulary each:
`adrm` deletes (rm-compatible, completes file names), `admr` manages the trash
(completes subcommands).

```bash
curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
```

```bash
adrm report.pdf                 # recycle
adrm -r old-project/            # recycle a directory
adrm file1 --deadline "2027/1/1" # keep until a date
admr ls                        # what is in the trash
admr restore 1                 # put it back
admr purge 1                   # delete for real
admr log                       # append-only history
```

Install options: `--alias` (alias rm=adrm and take over rm's completion),
`--timer` (systemd --user daily cleanup), `-y` (accept defaults).
Uninstall: `bash install.sh --uninstall [--purge-home]`.

See the Chinese sections above for the full documentation, including every
`--deadline` date/time spelling and the complete filter/sort reference.

---

## 许可证

MIT，见 [LICENSE](LICENSE)。

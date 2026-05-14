# adrm - Advance Remover

A safe alternative to `/bin/rm` that moves files to a trash directory instead of permanently deleting them. Features SQLite-backed tracking, time-based auto-cleanup, powerful query/restore/clean operations, and interactive confirmation modes.

## Features

- **Safe deletion**: Files are moved to a trash directory, not permanently deleted
- **SQLite database**: Full tracking of original paths, permissions, ownership, timestamps
- **Time-based cleanup**: Set per-file expiration with flexible duration/absolute deadline syntax
- **Query/Restore/Clean**: Powerful filtering with SQL-like conditions
- **Interactive modes**: `-i` (per-file) and `-I` (batch) confirmation prompts
- **Permission preservation**: Restores original permissions, ownership, and timestamps (including suid/setgid/sticky bits)
- **Ignore rules**: gitignore-style patterns to skip certain files
- **Colored output**: Visual status indicators in query results
- **Shell completion**: Bash and zsh tab-completion for all options
- **Configurable**: Config file, custom home directory, customizable query columns

## Install

### Quick install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh > /tmp/adrm_install.sh && bash /tmp/adrm_install.sh && /bin/rm /tmp/adrm_install.sh
```

The installer will:
1. Download the `adrm` binary to your chosen location (default: `~/.local/bin`)
2. Download the completion script to `$ADRM_HOME/adrm-completion.sh`
3. Generate `$ADRM_HOME/adrm-init.sh` with `ADRM_HOME`, `rm` alias, and completion sourcing
4. Add a `source "$ADRM_HOME/adrm-init.sh"` line to your `~/.bashrc` or `~/.zshrc`
5. Auto-run `adrm --default` if no config file exists

After installation, open a new shell or run `source ~/.bashrc` (or `~/.zshrc`).

### Manual install

```bash
# 1. Download binary from GitHub Releases and place it in your PATH
cp adrm-linux-x86_64 ~/.local/bin/adrm
chmod +x ~/.local/bin/adrm

# 2. Create adrm home directory
mkdir -p ~/.adrm

# 3. Download completion script
curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/adrm-completion.sh \
    -o ~/.adrm/adrm-completion.sh

# 4. Add to your ~/.bashrc (or ~/.zshrc):
export ADRM_HOME="$HOME/.adrm"
alias rm="$HOME/.local/bin/adrm"
source "$HOME/.adrm/adrm-completion.sh"

# 5. Generate default config
adrm --default
```

### Local install (project-scoped)

You can restrict adrm to a specific project directory, so that `rm` is only overridden inside that project. All adrm data — config, database, trash — stays inside the project, making it fully self-contained and portable.

Using [direnv](https://direnv.net/):

```bash
# Inside your project directory:
cd /path/to/my-project

# Create .envrc
cat > .envrc << 'EOF'
export ADRM_HOME="$PWD/.adrm"
alias rm="$(command -v adrm)"
source "$(command -v adrm-completion.sh 2>/dev/null || echo /path/to/adrm-completion.sh)"
EOF

# Allow direnv to load it
direnv allow
```

This creates `<project>/.adrm/` containing everything adrm needs:

```
my-project/
├── .adrm/
│   ├── config          # project-specific config
│   ├── files.db        # project-local database
│   └── trash/          # project-local trash
├── .envrc
└── ...
```

When you `cd` into the project, `rm` is aliased to `adrm` with all data stored in `<project>/.adrm/`. When you leave, the original `rm` is restored. No global state is affected.

You can also do this without direnv by manually setting the environment in a subshell or script:

```bash
ADRM_HOME=/path/to/my-project/.adrm alias rm=adrm
```

### Build from source

```bash
mkdir build && cd build
cmake .. -DCMAKE_BUILD_TYPE=Release
cmake --build .
sudo cmake --install .
```

**Dependencies**: CMake 3.14+, GCC 12+ (C++23), SQLite3 development library.

On Debian/Ubuntu: `sudo apt install cmake g++ libsqlite3-dev`

### Uninstall

1. Delete the binary: `/bin/rm -f ~/.local/bin/adrm`
2. Delete the adrm home directory (contains database and trash — **verify no needed files remain**): `/bin/rm -rf ~/.adrm`
3. Remove the `# >>> adrm >>>` ... `# <<< adrm <<<` block from `~/.bashrc` or `~/.zshrc`
4. The completion script automatically saves the original `rm` completion on load; after removing the source line, `rm` completion reverts to the system default on next shell launch
5. Start a new shell

## Quick Start

```bash
# Generate default config
adrm --default

# Recycle files (default cleanup: 30 days)
adrm myfile.txt

# With custom expiration
adrm myfile.txt +7d              # 7 days from now
adrm myfile.txt +3d12h30m        # 3 days 12 hours 30 minutes
adrm --deadline "2027-01-01" myfile.txt  # absolute deadline

# Recycle directory recursively
adrm -r mydir/

# Recycle multiple files with different durations (right-associative)
adrm +3d file1 file2 +7d file3   # file1,2: 3 days; file3: 7 days

# Interactive mode
adrm -i file1 file2              # prompt for each file
adrm -I file1 file2 file3 file4  # prompt once if >3 files

# Force: ignore nonexistent files and ignore rules
adrm -f file1

# Handle filenames starting with -
adrm -- -file_with_dash

# Remove empty directories
adrm -d emptydir/

# Verbose output
adrm -v file1 file2
```

## Query / Restore / Clean

```bash
# Query records
adrm --query                     # recent 10 records
adrm --query-all                 # all records
adrm --query --state recycled    # only recycled files
adrm --query --fname "test"      # name contains "test"
adrm --query --fsize +1g         # files larger than 1GB
adrm --query --sort-asc id       # sort by id ascending
adrm --query --sort-des fsize --sort-asc id  # multi-field sort
adrm --query --color             # colored output

# Restore files
adrm --restore                   # restore most recent file
adrm --restore --id 5            # restore by ID
adrm --restore --fname "test"    # restore by name pattern
adrm --restore-all               # restore all recycled files

# Permanently delete (clean)
adrm --clean                     # clean most recent file
adrm --clean --id 5              # clean by ID
adrm --clean-all                 # clean all recycled files
adrm --clear                     # clean all expired files (interactive)
```

## Command-Line Options

### Remove options (compatible with /bin/rm)

| Option | Description |
|--------|-------------|
| `-f` | Force: ignore nonexistent files, bypass ignore rules |
| `-i` | Interactive: prompt before every removal |
| `-I` | Interactive once: prompt when removing >3 files |
| `-r`, `-R`, `--recursive` | Remove directories recursively |
| `-d` | Remove empty directories |
| `-v` | Verbose output |
| `--` | End of options; treat rest as filenames |

### Time specifiers (right-associative)

Duration applies to all following files until the next duration:

```
adrm +3d file1 file2 +7d file3
# file1: 3 days, file2: 3 days, file3: 7 days
```

Duration format: `+NdNhNmNs` (e.g., `+3d12h30m`, `+1s`, `+5m`)

Available units: `d` (days), `h` (hours), `m` (minutes), `s` (seconds). No months or years — use `+730d` for approximately 2 years, or `--deadline` for exact dates.

Trailing duration without files after it is an error.

Absolute deadline: `--deadline "YYYY-MM-DD HH:MM:SS"` (time part optional, supports `/` and `_` separators)

```
--deadline "2027-01-01"
--deadline "2027/01/01 03:45"
--deadline "2027_01_01 03:45:01"
```

Default cleanup: 30 days (configurable in `~/.adrm/config`).

No daemon runs in the background — expired files are only cleaned when you run `adrm --clear`.

### Operations

| Command | Description |
|---------|-------------|
| `--query` | Show recent records (default 10). Accepts filters and sort options. |
| `--query-all` | Show all records. Accepts filters and sort options. |
| `--restore` | Restore the most recent recycled file. Accepts filters. |
| `--restore-all` | Restore all recycled files |
| `--clean` | Permanently delete the most recent recycled file. Accepts filters. |
| `--clean-all` | Permanently delete all recycled files |
| `--clear` | Clean all expired files (with interactive confirmation if any exist) |
| `--reset-db` | Reset the database (only if no recycled files remain) |
| `--default` | Generate default configuration file |
| `--help` | Show help |
| `--version` | Show version |

### Filter conditions (for --query, --restore, --clean)

| Filter | Example | Description |
|--------|---------|-------------|
| `--id N` | `--id 10` | Exact ID match |
| `--id +N` | `--id +10` | ID >= 10 |
| `--id -N` | `--id -10` | ID <= 10 |
| `--items N` | `--items 20` | Limit to N most recent records |
| `--fname PAT` | `--fname "test"` | Name contains pattern (case-insensitive) |
| `--fdate DUR` | `--fdate +30d` | File modification date filter |
| `--fsize SPEC` | `--fsize +1g` | File size filter |
| `--rdate DUR` | `--rdate +7d` | Recycle date filter |
| `--cdate DUR` | `--cdate -30d` | Cleanup date filter |
| `--state ST` | `--state recycled` | Status filter |

Size format: `NgNmNkNb` with `+`/`-` prefix (e.g., `+1g`, `-10m50b`, `10t20g30m40k50b`)

Filters combine with logical AND:

```bash
adrm --query --state recycled --fsize -1g --id +5
# Shows records that are recycled AND <=1GB AND ID>=5
```

### Sort options (for --query, --query-all)

| Option | Description |
|--------|-------------|
| `--sort-asc FIELD` | Sort ascending |
| `--sort-des FIELD` | Sort descending |

Fields: `id`, `fname`, `fdate`, `fsize`, `rdate`, `cdate`, `state`

Multiple sort options supported, applied in order of appearance:

```bash
adrm --query --sort-asc id --sort-des cdate
```

### Display options (for --query, --query-all)

| Option | Description |
|--------|-------------|
| `--color` | Colorize status column |
| `--items N` | Limit displayed records |

### Color legend (with --color)

| Status | Color |
|--------|-------|
| recycled | Blue |
| restored | Green |
| cleaned | Yellow |
| exception | Red |

## Shell Completion

adrm includes bash and zsh tab-completion. The installer sets this up automatically.

For manual setup, source the completion file in your shell config:

```bash
source "/path/to/adrm-completion.sh"
```

**Completion behavior:**

- `rm file<TAB>` / `adrm file<TAB>` — completes filenames only
- `rm -<TAB>` / `adrm -<TAB>` — completes short and long options (`-f`, `-i`, `-I`, `-r`, `-R`, `-d`, `-v`, `--help`, `--query`, ...)
- `rm --sort-asc <TAB>` — completes sort fields (`id`, `fname`, `fdate`, `fsize`, `rdate`, `cdate`, `state`)
- `rm --state <TAB>` — completes states (`recycled`, `restored`, `cleaned`, `exception`)
- `/bin/rm <TAB>` — falls back to the system default `rm` completion (not adrm)

The original `rm` completion is saved on load. After uninstalling (removing the source line), `rm` reverts to the system default on next shell launch.

## Configuration

Config file: `~/.adrm/config` (or `$ADRM_HOME/config`)

```ini
# Default cleanup period in days
default_cleanup_days = 30

# Paths (relative to adrm home or absolute)
trash_dir = trash
database_file = files.db
ignore_file = ignore

# Query display columns (comma-separated)
query_columns = id, original_path, original_size, recycle_time, cleanup_time, status

# Default query limit
query_default_limit = 10
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `ADRM_HOME` | Custom adrm home directory (default: `~/.adrm`) |

### Ignore rules

File: `~/.adrm/ignore` (gitignore-style patterns)

```
*.tmp
*.log
build/
```

Ignored files are skipped with a message unless `-f` is used.

## Database Schema

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Auto-increment primary key |
| original_path | TEXT | Original absolute path (dirs end with `/`) |
| original_mtime | TEXT | Original modification time |
| original_size_human | TEXT | Human-readable size |
| original_size_bytes | INTEGER | Size in bytes |
| original_perm | INTEGER | Regular permission bits (octal) |
| original_special_perm | INTEGER | Special permission bits (suid/setgid/sticky) |
| original_uid | INTEGER | Owner UID |
| original_gid | INTEGER | Group GID |
| original_owner | TEXT | Owner name |
| original_group | TEXT | Group name |
| trash_path | TEXT | Trash path (UUID-based filename) |
| recycle_time | TEXT | Recycle timestamp |
| cleanup_time | TEXT | Scheduled cleanup timestamp |
| status | TEXT | `recycled` / `restored` / `cleaned` / `exception` |

## License

MIT License

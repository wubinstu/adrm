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
- **Configurable**: Config file, custom home directory, customizable query columns

## Install

### Quick install

```bash
curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
```

The installer will ask for:
1. Binary install location (default: `~/.local/bin`)
2. adrm home directory (default: `~/.adrm/`)
3. Whether to create a `rm` alias in your shell config

### Build from source

```bash
mkdir build && cd build
cmake .. -DCMAKE_BUILD_TYPE=Release
cmake --build .
sudo cmake --install .
```

**Dependencies**: CMake 3.14+, GCC 12+ (C++23), SQLite3 development library.

On Debian/Ubuntu: `sudo apt install cmake g++ libsqlite3-dev`

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

# Recycle multiple files with different durations
adrm +3d file1 file2 +7d file3   # file1,2: 3 days; file3: 7 days

# Interactive mode
adrm -i file1 file2              # prompt for each file
adrm -I file1 file2 file3 file4  # prompt once if >3 files

# Query records
adrm --query                     # recent 10 records
adrm --query-all                 # all records
adrm --query --state recycled    # only recycled files
adrm --query --fname "test"      # name contains "test"
adrm --query --fsize +1g         # files larger than 1GB
adrm --query --sort-asc id       # sort by id ascending
adrm --query --color             # colored output

# Restore files
adrm --restore                   # restore most recent file
adrm --restore --id 5            # restore by ID
adrm --restore --fname "test"    # restore by name pattern
adrm --restore-all               # restore all

# Clean files (permanently delete)
adrm --clean                     # clean most recent file
adrm --clean --id 5              # clean by ID
adrm --clean-all                 # clean all recycled files
adrm --clear                     # clean all expired files

# Other
adrm --reset-db                  # reset database
adrm -f nonexistent              # force, skip missing files
adrm -- -file_with_dash          # handle filenames starting with -
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

Absolute deadline: `--deadline "YYYY-MM-DD HH:MM:SS"` (time part optional, supports `/` and `_` separators)

Default cleanup: 30 days (configurable in `~/.adrm/config`).

### Operations

| Command | Description |
|---------|-------------|
| `--query` | Show recent records (default 10) |
| `--query-all` | Show all records |
| `--restore` | Restore most recent recycled file |
| `--restore-all` | Restore all recycled files |
| `--clean` | Permanently delete most recent recycled file |
| `--clean-all` | Permanently delete all recycled files |
| `--clear` | Clean all expired files (interactive confirmation) |
| `--reset-db` | Reset the database |
| `--default` | Generate default configuration |
| `--help` | Show help |
| `--version` | Show version |

### Filter conditions (for --query, --restore, --clean)

| Filter | Example | Description |
|--------|---------|-------------|
| `--id N` | `--id 10` | Exact ID match |
| `--id +N` | `--id +10` | ID >= 10 |
| `--id -N` | `--id -10` | ID <= 10 |
| `--items N` | `--items 20` | Limit to N records |
| `--fname PAT` | `--fname "test"` | Name contains pattern (case-insensitive) |
| `--fdate DUR` | `--fdate +30d` | File modification date filter |
| `--fsize SPEC` | `--fsize +1g` | File size filter |
| `--rdate DUR` | `--rdate +7d` | Recycle date filter |
| `--cdate DUR` | `--cdate -30d` | Cleanup date filter |
| `--state ST` | `--state recycled` | Status filter |

Size format: `NgNmNkNb` with `+`/`-` prefix (e.g., `+1g`, `-10m50b`, `10t20g30m40k50b`)

Filters combine with logical AND: `--state recycled --fsize -1g --id +5`

### Sort options (for --query, --query-all)

| Option | Description |
|--------|-------------|
| `--sort-asc FIELD` | Sort ascending |
| `--sort-des FIELD` | Sort descending |
| `--color` | Colorize status column |

Fields: `id`, `fname`, `fdate`, `fsize`, `rdate`, `cdate`, `state`

Multiple sort options supported, applied in order of appearance.

### Color legend (with --color)

| Status | Color |
|--------|-------|
| recycled | Blue |
| restored | Green |
| cleaned | Yellow |
| exception | Red |

## Configuration

Config file: `~/.adrm/config` (or `$SAFE_RM_HOME/config`)

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
| `SAFE_RM_HOME` | Custom adrm home directory (default: `~/.adrm`) |

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

# adrm - Advance Remover

Move files to trash instead of permanently deleting them, with SQLite-backed tracking, time-based cleanup, and powerful query/restore/clean operations.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/wubinstu/adrm/main/install.sh | bash
```

## Quick Start

```bash
# Move file to trash (with auto-cleanup in 7 days)
adrm myfile.txt +7d

# Move directory recursively
adrm -r mydir/

# Query recent recycled files
adrm --query

# Restore the most recent file
adrm --restore

# Restore by filename pattern
adrm --restore --fname "test"

# Clean all recycled files
adrm --clean-all

# Clean expired files
adrm --clear
```

## Build from Source

```bash
mkdir build && cd build
cmake .. -DCMAKE_BUILD_TYPE=Release
cmake --build .
sudo cmake --install .
```

Dependencies: CMake 3.14+, GCC 13+ (C23/C++23), SQLite3 development library.

## License

GPL-3.0

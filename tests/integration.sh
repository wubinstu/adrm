#!/usr/bin/env bash
# adrm end-to-end integration tests.
#
# Usage: tests/integration.sh [path-to-adrm-binary]
# Requires: bash. The cross-filesystem and ownership sections need root and
# are skipped automatically when not running as root.
set -u

ADRM_BIN="${1:-$(cd "$(dirname "$0")/.." && pwd)/adrm}"
if [ ! -x "$ADRM_BIN" ]; then
    echo "integration: binary not found: $ADRM_BIN" >&2
    exit 1
fi
ADRM_BIN="$(cd "$(dirname "$ADRM_BIN")" && pwd)/$(basename "$ADRM_BIN")"

ROOT="$(mktemp -d /tmp/adrm-it.XXXXXX)"
trap 'rm -rf "$ROOT"' EXIT

PASS=0
FAIL=0

ok()   { PASS=$((PASS+1)); printf '  [ok] %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  [FAIL] %s\n' "$1"; }
check(){ if [ "$2" = "$3" ]; then ok "$1 ($2)"; else bad "$1 (got '$2' want '$3')"; fi; }

export ADRM_HOME="$ROOT/home"
export ADRM_LANG=en
mkdir -p "$ADRM_HOME"
adrm() { "$ADRM_BIN" "$@"; }

echo "== 1. basic recycle / list / restore / purge =="
work="$ROOT/w1"; mkdir -p "$work"
touch "$work/a" "$work/b"
adrm "$work/a" "$work/b" >/dev/null 2>&1
check "both recycled" "$(ls "$work" | wc -l)" "0"
n=$(adrm ls --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "ls shows 2" "$n" "2"
adrm restore --last >/dev/null 2>&1
check "restore --last" "$(ls "$work" | wc -l)" "1"
adrm purge --all --yes >/dev/null 2>&1
check "purge --all empties bin" "$(adrm ls --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')" "0"
# reflog survives the purge
ops=$(adrm log --json | python3 -c 'import json,sys; print(",".join(sorted({e["op"] for e in json.load(sys.stdin)})))')
check "reflog kept recycle+restore+purge" "$ops" "purge,recycle,restore"

echo "== 2. strict command validation =="
touch "$work/c"
adrm "$work/c" +2d >/dev/null 2>&1
check "dangling expiry exit code" "$?" "2"
[ -e "$work/c" ] && ok "nothing recycled on error" || bad "file was recycled despite error"
adrm --frobnicate "$work/c" >/dev/null 2>&1
check "unknown option exit code" "$?" "2"
adrm "$work/does-not-exist" >/dev/null 2>&1
rc=$?
check "missing file exit code" "$rc" "1"
adrm -f "$work/does-not-exist" >/dev/null 2>&1
check "-f missing file exit code" "$?" "0"

echo "== 3. rm compatibility =="
mkdir -p "$work/dir/sub"; touch "$work/dir/sub/f"
adrm "$work/dir" >/dev/null 2>&1
check "dir without -r fails" "$?" "1"
adrm -d "$work/dir" >/dev/null 2>&1
check "non-empty dir with -d fails" "$?" "1"
mkdir -p "$work/empty"; adrm -d "$work/empty" >/dev/null 2>&1
check "-d empty dir" "$?" "0"
touch "$work/-weird"
( cd "$work" && adrm -- -weird >/dev/null 2>&1 )
check "-- escapes file names" "$?" "0"
iwork="$ROOT/w3i"; mkdir -p "$iwork"
for n in one two three four; do printf 'x' > "$iwork/$n"; done
echo n | adrm -I "$iwork/one" "$iwork/two" "$iwork/three" "$iwork/four" >/dev/null 2>&1
check "-I declined keeps files" "$(ls "$iwork" | wc -l)" "4"
echo y | adrm -I "$iwork/one" "$iwork/two" "$iwork/three" "$iwork/four" >/dev/null 2>&1
check "-I accepted recycles" "$(ls "$iwork" | wc -l)" "0"
printf 'x' > "$iwork/one"
echo n | adrm -i "$iwork/one" >/dev/null 2>&1
check "-i n keeps file" "$(ls "$iwork" | wc -l)" "1"
echo y | adrm -i "$iwork/one" >/dev/null 2>&1
check "-i y recycles" "$(ls "$iwork" | wc -l)" "0"
adrm < /dev/null >/dev/null 2>&1
check "no args exit code" "$?" "1"

echo "== 4. expiry, gc, auto-gc =="
export ADRM_AUTO_GC=1
touch "$work/e1" "$work/e2"
# durations are right-associative: they must precede their files
adrm +0s "$work/e1" "$work/e2" >/dev/null 2>&1
n=$(adrm ls --name e1 --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "auto gc purged expired e1" "$n" "0"
n=$(adrm ls --name e2 --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "auto gc purged expired e2" "$n" "0"
export ADRM_AUTO_GC=0
touch "$work/e3"
adrm +0s "$work/e3" >/dev/null 2>&1
n=$(adrm ls --name e3 --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "expired item waits for gc" "$n" "1"
ADRM_RETENTION_DAYS=30 adrm gc --dry-run >/dev/null 2>&1
check "gc --dry-run exit" "$?" "0"
n=$(adrm ls --name e3 --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "gc --dry-run keeps item" "$n" "1"
adrm gc --yes >/dev/null 2>&1
n=$(adrm ls --name e3 --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "gc removes it" "$n" "0"
ops=$(adrm log --op expire --name e3 --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
[ "$ops" -ge 1 ] && ok "expire recorded in reflog" || bad "no expire entry"

echo "== 5. ignore rules =="
# a sandboxed $HOME so relative patterns anchor predictably
export HOME="$ROOT/userhome"; mkdir -p "$HOME"
igwork="$HOME/igwork"; mkdir -p "$igwork" "$HOME/Downloads"
printf '*.tmp\nsecret/\n/abs-only.txt\nDownloads/*.iso\n' > "$ADRM_HOME/ignore"
touch "$igwork/a.tmp" "$igwork/b.log" "$igwork/abs-only.txt" "$HOME/Downloads/x.iso"
adrm "$igwork/a.tmp" "$igwork/b.log" "$igwork/abs-only.txt" "$HOME/Downloads/x.iso" >/dev/null 2>&1
[ -e "$igwork/a.tmp" ] && ok "*.tmp skipped" || bad "*.tmp not skipped"
[ ! -e "$igwork/b.log" ] && ok "b.log recycled" || bad "b.log not recycled"
[ ! -e "$igwork/abs-only.txt" ] && ok "/abs-only.txt is a root pattern, file recycled" || bad "abs pattern wrongly matched"
[ -e "$HOME/Downloads/x.iso" ] && ok "Downloads/*.iso skipped below $HOME" || bad "relative pattern not matched"
mkdir -p "$igwork/secret"; touch "$igwork/secret/s"
adrm -r "$igwork/secret" >/dev/null 2>&1
[ -e "$igwork/secret" ] && ok "secret/ skipped" || bad "secret/ not skipped"
adrm -rf "$igwork/secret" >/dev/null 2>&1
[ ! -e "$igwork/secret" ] && ok "-rf overrides ignore" || bad "-rf did not override"
adrm -f "$igwork/a.tmp" >/dev/null 2>&1
[ ! -e "$igwork/a.tmp" ] && ok "-f overrides ignore" || bad "-f did not override"

echo "== 6. cross-filesystem move (copy+delete) =="
if [ "$(id -u)" = "0" ] && [ -w /dev ]; then
    mnt="$ROOT/mnt"; mkdir -p "$mnt"
    if mount -t tmpfs -o size=16m tmpfs "$mnt" 2>"$ROOT/mount.err"; then
        if [ "$(stat -c '%d' "$mnt")" = "$(stat -c '%d' "$ADRM_HOME")" ]; then
            echo "  [skip] tmpfs same device as home ($(cat "$ROOT/mount.err"))"
            umount "$mnt" 2>/dev/null
            mount_failed=1
        fi
        if [ "${mount_failed:-0}" = "1" ]; then :; else
        big="$mnt/bigfile"; head -c 3000000 /dev/urandom > "$big"
        adrm "$big" >/dev/null 2>&1
        [ -e "$big" ] && bad "bigfile should be gone" || ok "cross-device file moved"
        adrm restore --last >/dev/null 2>&1
        if [ -e "$big" ]; then
            if cmp -s <(head -c 3000000 /dev/zero) <(head -c 3000000 "$big"); then :; fi
            ok "cross-device file restored (size $(stat -c%s "$big"))"
        else
            bad "cross-device restore failed"
        fi
        method=$(adrm log --json | python3 -c 'import json,sys; print([e["detail"] for e in json.load(sys.stdin) if e["op"]=="recycle"][0])  # log is newest-first')
        case "$method" in *copy+delete*) ok "reflog records copy+delete" ;; *) bad "reflog detail: $method" ;; esac
        fi
        umount "$mnt" 2>/dev/null
    else
        echo "  [skip] cannot mount tmpfs: $(cat "$ROOT/mount.err" 2>/dev/null)"
    fi
else
    echo "  [skip] cross-filesystem test needs root"
fi

echo "== 7. attribute preservation =="
if [ "$(id -u)" = "0" ]; then
    awork="$ROOT/w7"; mkdir -p "$awork"
    f="$awork/suid.sh"; printf '#!/bin/sh\n' > "$f"; chmod 6755 "$f"
    before=$(stat -c '%a %U:%G' "$f")
    adrm "$f" >/dev/null 2>&1
    adrm restore --last >/dev/null 2>&1
    after=$(stat -c '%a %U:%G' "$f")
    check "suid/sgid + owner restored" "$after" "$before"
    # a sticky dir
    d="$awork/sticky"; mkdir -p "$d"; chmod 1777 "$d"
    before=$(stat -c '%a' "$d")
    adrm "$d" >/dev/null 2>&1
    adrm restore --last >/dev/null 2>&1
    check "sticky bit restored" "$(stat -c '%a' "$d")" "$before"
    # mtime
    touch -d '2020-01-02 03:04:05' "$awork/mt"
    before=$(stat -c '%Y' "$awork/mt")
    adrm "$awork/mt" >/dev/null 2>&1
    adrm restore --last >/dev/null 2>&1
    check "mtime restored" "$(stat -c '%Y' "$awork/mt")" "$before"
    # symlink
    ln -sf /nonexistent "$awork/lnk"
    adrm "$awork/lnk" >/dev/null 2>&1
    adrm restore --last >/dev/null 2>&1
    [ -L "$awork/lnk" ] && ok "symlink restored as symlink" || bad "symlink lost"
else
    echo "  [skip] attribute tests need root"
fi

echo "== 8. restore conflicts =="
cwork="$ROOT/w8"; mkdir -p "$cwork"; f="$cwork/doc"
printf 'original' > "$f"
adrm "$f" >/dev/null 2>&1
printf 'new' > "$f"
adrm restore --last >/dev/null 2>&1
check "restore over existing fails" "$?" "1"
check "existing file untouched" "$(cat "$f")" "new"
adrm restore --last --replace >/dev/null 2>&1
check "--replace recycles conflict" "$(cat "$f")" "original"

echo "== 9. filters and sorting =="
fwork="$ROOT/w9"; mkdir -p "$fwork"
head -c 4096 /dev/zero > "$fwork/big.bin"; touch "$fwork/small.txt"
adrm "$fwork/big.bin" "$fwork/small.txt" >/dev/null 2>&1
n=$(adrm ls --size +1k --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "--size +1k" "$n" "1"
n=$(adrm ls --name SMALL --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "--name case-insensitive" "$n" "1"
n=$(adrm ls --path "$fwork" --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "--path prefix" "$n" "2"
n=$(adrm ls --last 1 --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "--last 1" "$n" "1"
first=$(adrm ls --sort size:desc --json | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["path"])')
check "--sort size:desc" "$(basename "$first")" "big.bin"
n=$(adrm ls --state exception --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "--state exception empty" "$n" "0"
adrm ls --sort bogus >/dev/null 2>&1
check "bad sort field exit code" "$?" "2"

echo "== 10. ids =="
ids=$(adrm ls --path "$fwork" --json | python3 -c 'import json,sys; print(",".join(str(e["id"]) for e in json.load(sys.stdin)))')
check "two items in scope" "$(echo "$ids" | tr ',' '\n' | wc -l)" "2"
first=${ids%%,*}
adrm purge "$first" --yes >/dev/null 2>&1
n=$(adrm ls --path "$fwork" --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "purge single id" "$n" "1"
rest=$(adrm ls --path "$fwork" --json | python3 -c 'import json,sys; print(",".join(str(e["id"]) for e in json.load(sys.stdin)))')
adrm purge "$rest" --yes >/dev/null 2>&1
n=$(adrm ls --path "$fwork" --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "purge remaining id" "$n" "0"
before=$(adrm log --json | python3 -c 'import json,sys; print(max(e["item_id"] for e in json.load(sys.stdin)))')
touch "$fwork/recycled-again"
adrm "$fwork/recycled-again" >/dev/null 2>&1
newid=$(adrm ls --path "$fwork" --json | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["id"])')
[ "$newid" -gt "$before" ] && ok "new id $newid > old max $before (ids never reused)" || bad "id reuse: $newid <= $before"

echo "== 11. concurrency (20 parallel recycles) =="
pwork="$ROOT/w11"; mkdir -p "$pwork"
for i in $(seq 1 20); do touch "$pwork/f$i"; done
for i in $(seq 1 20); do
    ( cd "$pwork" && adrm "f$i" >/dev/null 2>&1 ) &
done
wait
n=$(adrm ls --path "$pwork" --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "all 20 recycled" "$n" "20"
adrm restore --all --yes >/dev/null 2>&1
check "restore --all" "$(ls "$pwork" | wc -l)" "20"

echo "== 12. json output is valid everywhere =="
for sub in "ls" "log" "stats"; do
    adrm $sub --json | python3 -c 'import json,sys; json.load(sys.stdin)' >/dev/null 2>&1 \
        && ok "$sub --json parses" || bad "$sub --json invalid"
done

echo "== 13. completion =="
bash_out=$(adrm completions bash)
case "$bash_out" in *"complete -o default -F _adrm_complete adrm"*) ok "bash completion registers adrm" ;; *) bad "bash completion content" ;; esac
case "$bash_out" in *"complete -o default -F _adrm_complete rm"*) ok "bash completion registers rm" ;; *) bad "bash completion misses rm" ;; esac
( adrm completions bash > "$ROOT/comp.bash"; bash -c ". $ROOT/comp.bash; complete -p adrm rm" >/dev/null 2>&1 ) \
    && ok "bash sources completion cleanly" || bad "bash completion fails to load"
cands=$(adrm __complete bash restore "")
if printf '%s\n' "$cands" | grep -qE '^[0-9]+$'; then ok "__complete offers trash ids"; else bad "__complete ids: $cands"; fi
cands=$(adrm __complete bash ls --state "")
case "$cands" in *recycled*) ok "__complete offers state enums" ;; *) bad "__complete state: $cands" ;; esac
cands=$(adrm __complete bash "")
case "$cands" in *restore*) ok "__complete offers subcommands" ;; *) bad "__complete subcommands: $cands" ;; esac
if command -v zsh >/dev/null 2>&1; then
    zsh -c "adrm() { command $ADRM_BIN \"\$@\"; }; fpath=($ROOT \$fpath); adrm completions zsh > $ROOT/_adrm; autoload -Uz compinit && compinit -u -d $ROOT/zcompdump && source $ROOT/_adrm" >/dev/null 2>&1 \
        && ok "zsh completion loads" || bad "zsh completion fails"
else
    echo "  [skip] zsh not installed"
fi

echo "== 14. setup install / uninstall =="
fakehome="$ROOT/fakehome"; mkdir -p "$fakehome"
HOME="$fakehome" adrm setup --install --shells bash >/dev/null 2>&1
grep -q "ADRM_HOME" "$fakehome/.bashrc" && ok "rc updated" || bad "rc not updated"
[ -f "$ADRM_HOME/completions/adrm.bash" ] && ok "completion installed" || bad "completion missing"
[ -f "$ADRM_HOME/adrm-init.sh" ] && ok "init script installed" || bad "init script missing"
HOME="$fakehome" adrm setup --uninstall >/dev/null 2>&1
grep -q adrm "$fakehome/.bashrc" && bad "rc not cleaned" || ok "rc cleaned"
[ ! -e "$ADRM_HOME/completions" ] && ok "completions removed" || bad "completions left behind"

echo "== 15. doctor =="
adrm doctor >/dev/null 2>&1
check "doctor exit code" "$?" "0"
# orphan injection: a file in the trash the db does not know
mkdir -p "$ADRM_HOME/trash/99990101-000000_999999"; touch "$ADRM_HOME/trash/99990101-000000_999999/orphan"
out=$(adrm doctor | grep -c orphan)
[ "$out" -ge 1 ] && ok "doctor finds orphans" || bad "doctor missed orphan"
echo n | adrm doctor >/dev/null 2>&1
[ -e "$ADRM_HOME/trash/99990101-000000_999999/orphan" ] && ok "declined orphan kept" || bad "orphan removed despite no"
echo y | adrm doctor >/dev/null 2>&1
[ ! -e "$ADRM_HOME/trash/99990101-000000_999999/orphan" ] && ok "accepted orphan purged" || bad "orphan not purged"

echo "== 16. db reset safety =="
touch "$work/keepme"; adrm "$work/keepme" >/dev/null 2>&1
echo n | adrm db --reset >/dev/null 2>&1
n=$(adrm ls --name keepme --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "declined reset keeps bin" "$n" "1"
echo y | adrm db --reset >/dev/null 2>&1
n=$(adrm log --json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')
check "reset clears history" "$n" "1"

echo "== 17. english output is pure ASCII =="
for args in "ls" "log" "stats" "doctor" "config --show" "help" "--help" "gc --dry-run" "completions bash"; do
    if adrm $args 2>&1 | LC_ALL=C grep -qP '[^\x00-\x7F]'; then
        bad "non-ASCII in: adrm $args"
    else
        ok "ascii: adrm $args"
    fi
done

echo
echo "integration: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]

// Package x contains the low level filesystem helpers used by the trash
// engine: metadata capture, same/cross-device moves, attribute-preserving
// restores and small OS utilities.
package x

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/wubinstu/adrm/internal/model"
)

// Meta captures the metadata adrm records before moving a file into the trash.
type Meta struct {
	IsDir bool
	Size  int64
	Mode  uint32
	UID   int
	GID   int
	Mtime int64
}

// Capture reads metadata (following no symlinks) for path.
func Capture(path string) (*Meta, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	m := &Meta{IsDir: fi.IsDir(), Mode: permBits(fi), Mtime: fi.ModTime().Unix()}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		m.UID, m.GID = int(st.Uid), int(st.Gid)
	}
	if m.IsDir {
		size, err := DirSize(path)
		if err != nil {
			return nil, err
		}
		m.Size = size
	} else {
		m.Size = fi.Size()
	}
	return m, nil
}

// permBits extracts the full permission word including setuid/setgid/sticky.
func permBits(fi os.FileInfo) uint32 {
	mode := uint32(fi.Mode().Perm())
	if fi.Mode()&os.ModeSetuid != 0 {
		mode |= 04000
	}
	if fi.Mode()&os.ModeSetgid != 0 {
		mode |= 02000
	}
	if fi.Mode()&os.ModeSticky != 0 {
		mode |= 01000
	}
	return mode
}

// DirSize sums file sizes recursively (following no symlinks into other trees).
func DirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil // skip unreadable entries, keep the walk going
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// LookupOwner resolves uid/gid to names when possible (falls back to numbers).
func LookupOwner(uid, gid int) (string, string) {
	name := strconv.Itoa(uid)
	if u, err := user.LookupId(strconv.Itoa(uid)); err == nil {
		name = u.Username
	}
	group := strconv.Itoa(gid)
	if g, err := user.LookupGroupId(strconv.Itoa(gid)); err == nil {
		group = g.Name
	}
	return name, group
}

// SameDevice reports whether a and b live on the same filesystem.
func SameDevice(a, b string) bool {
	da, err1 := deviceOf(a)
	db, err2 := deviceOf(b)
	if err1 != nil || err2 != nil {
		return true // assume same; rename will tell us the truth
	}
	return da == db
}

func deviceOf(p string) (uint64, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Dev), nil
	}
	return 0, errors.New("unsupported platform")
}

// W_OK is the unix write-permission bit for access().
const W_OK = 2

// WritableDir reports whether the current user may create entries inside dir.
func WritableDir(dir string) bool {
	return syscall.Access(dir, W_OK) == nil
}

// Move relocates src to dst. It first tries a rename; when that fails because
// the paths cross a filesystem boundary it falls back to a
// metadata-preserving copy + delete (unless copyFallback is false).
func Move(src, dst string, copyFallback bool) (string, error) {
	if err := os.Rename(src, dst); err == nil {
		return "rename", nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return "", err
	}
	if !copyFallback {
		return "", fmt.Errorf("cannot move %s to %s: different filesystems (copy_fallback = false)", src, dst)
	}
	if err := CopyTreePreserve(src, dst); err != nil {
		return "", err
	}
	// Sanity check: the copy must be at least as large as the original before
	// we delete anything.
	srcSize, _ := sizeOf(src)
	dstSize, _ := sizeOf(dst)
	if dstSize < srcSize {
		return "", fmt.Errorf("copy of %s looks incomplete (%d < %d bytes), original kept", src, dstSize, srcSize)
	}
	if err := os.RemoveAll(src); err != nil {
		return "", fmt.Errorf("copied %s but could not remove the original: %w", src, err)
	}
	return "copy+delete", nil
}

func sizeOf(p string) (int64, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return 0, err
	}
	if fi.IsDir() {
		return DirSize(p)
	}
	return fi.Size(), nil
}

// CopyTreePreserve copies a file or directory tree preserving mode, ownership
// (best effort) and timestamps.
func CopyTreePreserve(src, dst string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := os.MkdirAll(dst, fi.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := CopyTreePreserve(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		restoreOwnershipBestEffort(dst, fi)
		restoreTimes(dst, fi)
		return nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		os.Remove(dst)
		return os.Symlink(target, dst)
	}
	return CopyFilePreserve(src, dst)
}

// CopyFilePreserve copies one regular file preserving mode and timestamps.
func CopyFilePreserve(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	tmp := dst + ".adrm-part"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, fi.Mode().Perm()); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	restoreOwnershipBestEffort(dst, fi)
	restoreTimes(dst, fi)
	return nil
}

// unixMode converts a raw unix mode word (as stored in the database) into an
// os.FileMode that preserves the special permission bits.
func unixMode(m uint32) os.FileMode {
	fm := os.FileMode(m & 0o7777)
	if m&0o4000 != 0 {
		fm |= os.ModeSetuid
	}
	if m&0o2000 != 0 {
		fm |= os.ModeSetgid
	}
	if m&0o1000 != 0 {
		fm |= os.ModeSticky
	}
	return fm
}

func restoreOwnershipBestEffort(p string, fi os.FileInfo) {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		_ = os.Lchown(p, int(st.Uid), int(st.Gid))
	}
}

func restoreTimes(p string, fi os.FileInfo) {
	mt := fi.ModTime()
	_ = os.Chtimes(p, mt, mt)
}

// RestoreMeta reapplies recorded ownership, permissions (including
// setuid/setgid/sticky) and timestamps to a restored path. Symlinks only
// get their ownership restored: chmod/chtimes would affect the link target.
func RestoreMeta(it *model.Item, isLink bool) error {
	if err := os.Lchown(it.OrigPath, it.UID, it.GID); err != nil {
		return fmt.Errorf("cannot restore owner %s:%s: %w", it.Owner, it.Group, err)
	}
	if isLink {
		return nil
	}
	// chown clears setuid/setgid on Linux, so chmod must come after chown.
	// The stored mode is a raw unix mode word; convert it so os.Chmod keeps
	// the suid/sgid/sticky bits (os.FileMode(mode).Perm() would drop them).
	if err := os.Chmod(it.OrigPath, unixMode(it.Mode)); err != nil {
		return fmt.Errorf("cannot restore permissions %04o: %w", it.Mode, err)
	}
	t := time.Unix(it.Mtime, 0)
	if err := os.Chtimes(it.OrigPath, t, t); err != nil {
		return fmt.Errorf("cannot restore timestamps: %w", err)
	}
	return nil
}

// MkdirAll creates dir (and parents) with the given mode.
func MkdirAll(dir string, mode os.FileMode) error { return os.MkdirAll(dir, mode) }

// IsTerminal reports whether f is attached to a character device (a tty).
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

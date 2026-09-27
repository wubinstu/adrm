// Package store persists adrm state in a single SQLite database:
//
//	bin    - rows for items currently inside the trash (row removed on exit)
//	reflog - append-only history of every operation, never deleted
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/wubinstu/adrm/internal/model"
)

// SchemaVersion is stored in PRAGMA user_version; incompatible databases are
// refused instead of silently migrated (v2 is a fresh start by design).
const SchemaVersion = 2

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path and makes sure the
// schema exists. WAL + busy_timeout give us crash safety and cross-process
// locking so concurrent adrm runs serialize instead of corrupting.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("empty database path")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("cannot create database directory %s: %w", dir, err)
		}
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // single writer: simplest correct concurrency model
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	// Does the file already contain tables from a different (older) adrm?
	var hasBin, hasReflog bool
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		switch name {
		case "bin":
			hasBin = true
		case "reflog":
			hasReflog = true
		}
	}
	rows.Close()
	if (hasBin || hasReflog) && version == 0 {
		return fmt.Errorf("database %s looks like an old adrm v1 database\n"+
			"  v2 uses a fresh schema. Point adrm at a new home (ADRM_HOME) or run 'adrm db --reset'", s.path())
	}
	if version != 0 && version != SchemaVersion {
		return fmt.Errorf("database schema version %d is not supported by this adrm (want %d)", version, SchemaVersion)
	}
	if _, err := s.db.Exec(schemaDDL); err != nil {
		return fmt.Errorf("cannot create schema: %w", err)
	}
	if _, err := s.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", SchemaVersion)); err != nil {
		return err
	}
	return nil
}

func (s *Store) path() string {
	var p string
	_ = s.db.QueryRow("PRAGMA database_list").Scan(new(int), new(string), &p)
	return p
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

const schemaDDL = `
CREATE TABLE IF NOT EXISTS bin (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  orig_path    TEXT NOT NULL,
  is_dir       INTEGER NOT NULL,
  size         INTEGER NOT NULL,
  mode         INTEGER NOT NULL,
  uid          INTEGER NOT NULL,
  gid          INTEGER NOT NULL,
  owner        TEXT,
  groupname    TEXT,
  mtime        INTEGER NOT NULL,
  trash_path   TEXT NOT NULL UNIQUE,
  recycled_at  INTEGER NOT NULL,
  expire_at    INTEGER NOT NULL,
  method       TEXT,
  note         TEXT
);
CREATE INDEX IF NOT EXISTS ix_bin_expire ON bin(expire_at);
CREATE INDEX IF NOT EXISTS ix_bin_orig   ON bin(orig_path);
CREATE TABLE IF NOT EXISTS reflog (
  seq        INTEGER PRIMARY KEY AUTOINCREMENT,
  ts         INTEGER NOT NULL,
  op         TEXT NOT NULL,
  item_id    INTEGER,
  orig_path  TEXT,
  trash_path TEXT,
  size       INTEGER,
  detail     TEXT
);
CREATE INDEX IF NOT EXISTS ix_reflog_ts  ON reflog(ts);
CREATE INDEX IF NOT EXISTS ix_reflog_op  ON reflog(op);
CREATE INDEX IF NOT EXISTS ix_reflog_id  ON reflog(item_id);
`

// RecycleItems inserts items into bin and appends one reflog row per item in a
// single transaction, so the file system and the history can never disagree.
func (s *Store) RecycleItems(items []model.Item) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i := range items {
		it := &items[i]
		res, err := tx.Exec(`INSERT INTO bin
		 (orig_path,is_dir,size,mode,uid,gid,owner,groupname,mtime,trash_path,recycled_at,expire_at,method,note)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			it.OrigPath, b2i(it.IsDir), it.Size, int64(it.Mode), it.UID, it.GID, it.Owner, it.Group,
			it.Mtime, it.TrashPath, it.RecycledAt, it.ExpireAt, it.Method, "")
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		it.ID = id
		if err := s.reflogTx(tx, model.Reflog{
			Ts: it.RecycledAt, Op: model.OpRecycle, ItemID: id,
			OrigPath: it.OrigPath, TrashPath: it.TrashPath, Size: it.Size,
			Detail: "method=" + it.Method,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LeaveBin removes a row from the bin table and appends a reflog entry in one
// transaction. It is used by restore / purge / gc / empty.
func (s *Store) LeaveBin(id int64, op string, ts int64, detail string) error {
	var origPath, trashPath string
	var size int64
	err := s.db.QueryRow(`SELECT orig_path, trash_path, size FROM bin WHERE id=?`, id).Scan(&origPath, &trashPath, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("item %d is no longer in the trash bin", id)
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM bin WHERE id=?`, id); err != nil {
		return err
	}
	if err := s.reflogTx(tx, model.Reflog{
		Ts: ts, Op: op, ItemID: id,
		OrigPath: origPath, TrashPath: trashPath, Size: size, Detail: detail,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkException records a note on a bin item and appends an exception entry.
func (s *Store) MarkException(id int64, note string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE bin SET note=? WHERE id=?`, note, id); err != nil {
		return err
	}
	var origPath, trashPath string
	var size int64
	_ = tx.QueryRow(`SELECT orig_path, trash_path, size FROM bin WHERE id=?`, id).Scan(&origPath, &trashPath, &size)
	if err := s.reflogTx(tx, model.Reflog{
		Ts: time.Now().Unix(), Op: model.OpException, ItemID: id,
		OrigPath: origPath, TrashPath: trashPath, Size: size, Detail: note,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearException removes the exception note (after the user fixed it).
func (s *Store) ClearException(id int64) error {
	_, err := s.db.Exec(`UPDATE bin SET note='' WHERE id=?`, id)
	return err
}

func (s *Store) reflogTx(tx *sql.Tx, e model.Reflog) error {
	_, err := tx.Exec(`INSERT INTO reflog (ts,op,item_id,orig_path,trash_path,size,detail) VALUES (?,?,?,?,?,?,?)`,
		e.Ts, e.Op, i2z(e.ItemID), s2n(e.OrigPath), s2n(e.TrashPath), e.Size, s2n(e.Detail))
	return err
}

// AppendReflog records an entry that has no bin row (reset, empty, orphans).
func (s *Store) AppendReflog(e model.Reflog) error {
	if e.Ts == 0 {
		e.Ts = time.Now().Unix()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.reflogTx(tx, e); err != nil {
		return err
	}
	return tx.Commit()
}

// MaxBinID returns the largest id ever used in bin (0 when empty), which is
// what we use to name new trash batches so ids stay unique forever.
func (s *Store) MaxBinID() (int64, error) {
	var id int64
	err := s.db.QueryRow(`SELECT COALESCE(MAX(id),0) FROM bin`).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	// AUTOINCREMENT guarantees monotonicity even after deletes, but if the
	// sqlite_sequence row is ahead (rows deleted) we still must not reuse.
	var seq int64
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(seq),0) FROM sqlite_sequence WHERE name='bin'`).Scan(&seq)
	if seq > id {
		id = seq
	}
	return id, nil
}

// ListBin returns every bin row (filtering happens in Go, see ApplyFilter).
func (s *Store) ListBin() ([]model.Item, error) {
	rows, err := s.db.Query(`SELECT id,orig_path,is_dir,size,mode,uid,gid,owner,groupname,mtime,
	 trash_path,recycled_at,expire_at,method,note FROM bin`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Item
	for rows.Next() {
		var it model.Item
		var isDir int64
		var mode int64
		var owner, group, method, note sql.NullString
		if err := rows.Scan(&it.ID, &it.OrigPath, &isDir, &it.Size, &mode, &it.UID, &it.GID,
			&owner, &group, &it.Mtime, &it.TrashPath, &it.RecycledAt, &it.ExpireAt, &method, &note); err != nil {
			return nil, err
		}
		it.IsDir = isDir != 0
		it.Mode = uint32(mode)
		it.Owner, it.Group = owner.String, group.String
		it.Method, it.Note = method.String, note.String
		out = append(out, it)
	}
	return out, rows.Err()
}

// GetBin returns one bin row by id.
func (s *Store) GetBin(id int64) (*model.Item, error) {
	items, err := s.ListBin()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("no trash item with id %d", id)
}

// ListReflog returns the full history (oldest first); filtering happens in Go.
func (s *Store) ListReflog() ([]model.Reflog, error) {
	rows, err := s.db.Query(`SELECT seq,ts,op,item_id,orig_path,trash_path,size,detail FROM reflog ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Reflog
	for rows.Next() {
		var e model.Reflog
		var itemID sql.NullInt64
		var origPath, trashPath, detail sql.NullString
		var size sql.NullInt64
		if err := rows.Scan(&e.Seq, &e.Ts, &e.Op, &itemID, &origPath, &trashPath, &size, &detail); err != nil {
			return nil, err
		}
		e.ItemID = itemID.Int64
		e.OrigPath, e.TrashPath = origPath.String, trashPath.String
		e.Size, e.Detail = size.Int64, detail.String
		out = append(out, e)
	}
	return out, rows.Err()
}

// Reset empties both tables and records a reflog entry.
func (s *Store) Reset(detail string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM bin`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM reflog`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO reflog (ts,op,item_id,detail) VALUES (?,?,NULL,?)`,
		time.Now().Unix(), model.OpReset, s2n(detail)); err != nil {
		return err
	}
	return tx.Commit()
}

// Check validates database integrity.
func (s *Store) Check() error {
	var res string
	if err := s.db.QueryRow(`PRAGMA integrity_check`).Scan(&res); err != nil {
		return err
	}
	if res != "ok" {
		return fmt.Errorf("integrity check: %s", res)
	}
	return nil
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func i2z(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func s2n(s string) any {
	if s == "" {
		return nil
	}
	return s
}

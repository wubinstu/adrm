// Package config loads the adrm configuration file and environment overrides.
//
// Precedence (highest first): command line flags > ADRM_* environment
// variables > config file > built-in defaults.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	// Paths
	Home       string
	TrashDir   string
	Database   string
	IgnoreFile string
	ConfigFile string

	// Behavior
	RetentionDays   int
	AutoGC          bool
	PromptThreshold int
	MaxList         int
	PreserveRoot    bool
	CopyFallback    bool

	// Presentation
	Lang       string
	Color      string
	DateFormat string
	LSColumns  []string
	LogColumns []string
}

// Default returns the built-in configuration for a given home directory.
func Default(home string) *Config {
	return &Config{
		Home:            home,
		TrashDir:        filepath.Join(home, "trash"),
		Database:        filepath.Join(home, "adrm.db"),
		IgnoreFile:      filepath.Join(home, "ignore"),
		ConfigFile:      filepath.Join(home, "config"),
		RetentionDays:   30,
		AutoGC:          true,
		PromptThreshold: 3,
		MaxList:         500,
		PreserveRoot:    true,
		CopyFallback:    true,
		Lang:            "auto",
		Color:           "auto",
		DateFormat:      "2006-01-02 15:04:05",
		LSColumns:       []string{"id", "path", "size", "mtime", "recycled", "expire", "left"},
		LogColumns:      []string{"seq", "ts", "op", "id", "path", "detail"},
	}
}

// Load reads the configuration for the given home directory. A missing config
// file is not an error; `adrm config --init` writes one.
func Load(home string) (*Config, error) {
	if home == "" {
		home = DefaultHome()
	}
	home = absPath(home)
	c := Default(home)
	if err := c.loadFile(filepath.Join(home, "config")); err != nil {
		return nil, err
	}
	c.applyEnv()
	c.derive()
	return c, nil
}

// DefaultHome returns $ADRM_HOME or ~/.adrm.
func DefaultHome() string {
	if v := os.Getenv("ADRM_HOME"); v != "" {
		return absPath(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".adrm"
	}
	return filepath.Join(home, ".adrm")
}

func (c *Config) applyEnv() {
	if v := os.Getenv("ADRM_HOME"); v != "" {
		c.Home = absPath(v)
	}
	if v := os.Getenv("ADRM_TRASH"); v != "" {
		c.TrashDir = absPath(v)
	}
	if v := os.Getenv("ADRM_DB"); v != "" {
		c.Database = absPath(v)
	}
	if v := os.Getenv("ADRM_CONFIG"); v != "" {
		c.ConfigFile = absPath(v)
	}
	if v := os.Getenv("ADRM_IGNORE"); v != "" {
		c.IgnoreFile = absPath(v)
	}
	if v := os.Getenv("ADRM_LANG"); v != "" {
		c.Lang = v
	}
	if v := os.Getenv("ADRM_COLOR"); v != "" {
		c.Color = v
	}
	if v := os.Getenv("ADRM_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.RetentionDays = n
		}
	}
	if v := os.Getenv("ADRM_AUTO_GC"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.AutoGC = b
		}
	}
	if v := os.Getenv("ADRM_PROMPT_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.PromptThreshold = n
		}
	}
}

// derive fills in path defaults that the config file left unset and expands ~.
func (c *Config) derive() {
	if c.Home == "" {
		c.Home = DefaultHome()
	}
	c.Home = absPath(c.Home)
	if c.TrashDir == "" {
		c.TrashDir = filepath.Join(c.Home, "trash")
	} else {
		c.TrashDir = absPath(c.TrashDir)
	}
	if c.Database == "" {
		c.Database = filepath.Join(c.Home, "adrm.db")
	} else {
		c.Database = absPath(c.Database)
	}
	if c.IgnoreFile == "" {
		c.IgnoreFile = filepath.Join(c.Home, "ignore")
	} else {
		c.IgnoreFile = absPath(c.IgnoreFile)
	}
	if c.ConfigFile == "" {
		c.ConfigFile = filepath.Join(c.Home, "config")
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = 30
	}
	if c.PromptThreshold <= 0 {
		c.PromptThreshold = 3
	}
	if c.MaxList <= 0 {
		c.MaxList = 500
	}
	if len(c.LSColumns) == 0 {
		c.LSColumns = Default(c.Home).LSColumns
	}
	if len(c.LogColumns) == 0 {
		c.LogColumns = Default(c.Home).LogColumns
	}
}

// EnsureHome creates the adrm home directory with strict permissions and
// tightens permissions if the directory already exists with a looser mode:
// it holds the index of everything the user deleted.
func (c *Config) EnsureHome() error {
	if err := tightenDir(c.Home); err != nil {
		return err
	}
	return tightenDir(c.TrashDir)
}

func tightenDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 { // group/other may access
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("cannot tighten permissions of %s (%04o): %w", dir, perm, err)
		}
	}
	return nil
}

// loadFile parses a simple "key = value" file. Unknown keys produce an error
// so typos in hand-edited configs are caught early.
func (c *Config) loadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read config %s: %w", path, err)
	}
	defer f.Close()
	if c.ConfigFile == "" {
		c.ConfigFile = path
	}
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			return fmt.Errorf("%s:%d: expected 'key = value', got %q", path, lineNo, line)
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if err := c.set(key, val, path, lineNo); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (c *Config) set(key, val, path string, lineNo int) error {
	switch key {
	case "home":
		c.Home = unquote(val)
	case "trash_dir":
		c.TrashDir = unquote(val)
	case "database", "db":
		c.Database = unquote(val)
	case "ignore_file":
		c.IgnoreFile = unquote(val)
	case "retention_days":
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return fmt.Errorf("%s:%d: retention_days must be a positive integer, got %q", path, lineNo, val)
		}
		c.RetentionDays = n
	case "auto_gc":
		b, err := parseBool(val)
		if err != nil {
			return fmt.Errorf("%s:%d: %v", path, lineNo, err)
		}
		c.AutoGC = b
	case "prompt_threshold":
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return fmt.Errorf("%s:%d: prompt_threshold must be >= 0, got %q", path, lineNo, val)
		}
		c.PromptThreshold = n
	case "max_list":
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return fmt.Errorf("%s:%d: max_list must be a positive integer, got %q", path, lineNo, val)
		}
		c.MaxList = n
	case "preserve_root":
		b, err := parseBool(val)
		if err != nil {
			return fmt.Errorf("%s:%d: %v", path, lineNo, err)
		}
		c.PreserveRoot = b
	case "copy_fallback":
		b, err := parseBool(val)
		if err != nil {
			return fmt.Errorf("%s:%d: %v", path, lineNo, err)
		}
		c.CopyFallback = b
	case "lang":
		c.Lang = unquote(val)
	case "color":
		c.Color = unquote(val)
	case "date_format":
		c.DateFormat = unquote(val)
	case "ls_columns":
		c.LSColumns = parseList(val)
	case "log_columns":
		c.LogColumns = parseList(val)
	default:
		return fmt.Errorf("%s:%d: unknown config key %q (see 'adrm config --show')", path, lineNo, key)
	}
	return nil
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(unquote(v)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean %q (want true/false)", v)
}

func parseList(v string) []string {
	v = strings.TrimSpace(unquote(v))
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, part := range strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	}) {
		part = unquote(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	// strip trailing inline comment for unquoted scalars
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func absPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				p = home
			} else if strings.HasPrefix(p, "~/") {
				p = filepath.Join(home, p[2:])
			}
		}
	}
	p = os.ExpandEnv(p)
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

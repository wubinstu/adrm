package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/model"
	"github.com/wubinstu/adrm/internal/store"
)

// optSpec describes one accepted option.
type optSpec struct {
	long       string
	short      string
	takesValue bool
	multi      bool // repeatable option (--sort)
}

// parser is a tiny strict option parser shared by all subcommands.
type parser struct {
	multi  map[string][]string
	single map[string]string
	bools  map[string]bool
	pos    []string
}

func newParser() *parser {
	return &parser{
		multi:  map[string][]string{},
		single: map[string]string{},
		bools:  map[string]bool{},
	}
}

func (p *parser) has(long string) bool { return p.bools[long] }

func (p *parser) val(long string) string { return p.single[long] }

func (p *parser) vals(long string) []string { return p.multi[long] }

// parse consumes args per specs. It stops option parsing at "--", handles
// clustered short flags (-rf), "--opt=value" and "--opt value" forms, and
// errors out on unknown options.
func (p *parser) parse(args []string, specs []optSpec) error {
	byLong := map[string]optSpec{}
	byShort := map[string]optSpec{}
	for _, s := range specs {
		byLong[s.long] = s
		if s.short != "" {
			byShort[s.short] = s
		}
	}
	stopOpts := false
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if !stopOpts && tok == "--" {
			stopOpts = true
			continue
		}
		if !stopOpts && strings.HasPrefix(tok, "--") {
			name, val := tok[2:], ""
			hasVal := false
			if eq := strings.Index(name, "="); eq >= 0 {
				val, hasVal = name[eq+1:], true
				name = name[:eq]
			}
			spec, ok := byLong[name]
			if !ok {
				return usageErr("%s", i18n.Tr("未知选项 --%s (见 'adrm help')", "unknown option --%s (see 'adrm help')", name))
			}
			if spec.takesValue {
				if !hasVal {
					if i+1 >= len(args) {
						return usageErr("%s", i18n.Tr("选项 --%s 需要一个值", "option --%s requires a value", name))
					}
					i++
					val = args[i]
				}
				if spec.multi {
					p.multi[spec.long] = append(p.multi[spec.long], val)
				} else {
					p.single[spec.long] = val
				}
			} else {
				if hasVal {
					return usageErr("%s", i18n.Tr("选项 --%s 不接受值", "option --%s does not take a value", name))
				}
				p.bools[spec.long] = true
			}
			continue
		}
		if !stopOpts && strings.HasPrefix(tok, "-") && len(tok) > 1 {
			for j := 1; j < len(tok); j++ {
				c := string(tok[j])
				spec, ok := byShort[c]
				if !ok {
					return usageErr("%s", i18n.Tr("无效选项 -%s (见 'adrm help')", "invalid option -%s (see 'adrm help')", c))
				}
				if spec.takesValue {
					rest := tok[j+1:]
					if rest == "" {
						if i+1 >= len(args) {
							return usageErr("%s", i18n.Tr("选项 -%s 需要一个值", "option -%s requires a value", c))
						}
						i++
						rest = args[i]
					}
					if spec.multi {
						p.multi[spec.long] = append(p.multi[spec.long], rest)
					} else {
						p.single[spec.long] = rest
					}
					break
				}
				p.bools[spec.long] = true
			}
			continue
		}
		p.pos = append(p.pos, tok)
	}
	return nil
}

// usageError marks command-line mistakes; the CLI exits with code 2 for them.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usageErr(format string, args ...any) error {
	return &usageError{msg: fmt.Sprintf(format, args...)}
}

// commonSpecs are accepted by every subcommand.
var commonSpecs = []optSpec{
	{long: "home", takesValue: true},
	{long: "lang", takesValue: true},
	{long: "color", takesValue: true},
	{long: "help", short: "h"},
}

// filterSpecs are the filters shared by ls/log/restore/purge.
var filterSpecs = concatSpecs(commonSpecs, []optSpec{
	{long: "state", takesValue: true},
	{long: "op", takesValue: true},
	{long: "name", takesValue: true},
	{long: "path", takesValue: true},
	{long: "older-than", takesValue: true},
	{long: "newer-than", takesValue: true},
	{long: "mtime", takesValue: true},
	{long: "size", takesValue: true},
	{long: "since", takesValue: true},
	{long: "until", takesValue: true},
	{long: "sort", takesValue: true, multi: true},
	{long: "json"},
	{long: "expired"},
})

// listSpecs adds --last N (a count) on top of the filters for ls/log.
var listSpecs = concatSpecs(filterSpecs, []optSpec{{long: "last", takesValue: true}})

var dangerSpecs = concatSpecs(commonSpecs, []optSpec{
	{long: "last"},
	{long: "all"},
	{long: "yes", short: "y"},
	{long: "dry-run"},
	{long: "quiet"},
	{long: "replace"},
	{long: "pick"},
})

// concatSpecs joins several option sets.
func concatSpecs(sets ...[]optSpec) []optSpec {
	n := 0
	for _, s := range sets {
		n += len(s)
	}
	out := make([]optSpec, 0, n)
	for _, s := range sets {
		out = append(out, s...)
	}
	return out
}

// applyCommon honors --lang/--color/--home after parsing.
func (a *app) applyCommon(p *parser) error {
	if v := p.val("lang"); v != "" {
		a.applyLang(a.cfg.Lang, v, "")
	}
	if v := p.val("color"); v != "" {
		switch v {
		case "auto", "always", "never":
			a.cfg.Color = v
		default:
			return usageErr("%s", i18n.Tr("--color 只接受 auto|always|never, 得到 %q", "--color accepts auto|always|never, got %q", v))
		}
	}
	return nil
}

// buildFilter assembles a model.Filter from parsed options.
func (a *app) buildFilter(p *parser, forLog bool) (*model.Filter, error) {
	f := &model.Filter{}
	now := time.Now().Unix()
	if len(p.pos) > 0 {
		ids, err := store.ParseIDSelectors(p.pos)
		if err != nil {
			return nil, err
		}
		f.IDs = ids
	}
	if v := p.val("state"); v != "" {
		states, err := parseEnum(v, "state", []string{"recycled", "exception"})
		if err != nil {
			return nil, err
		}
		f.States = states
	}
	if v := p.val("op"); v != "" {
		ops, err := parseEnum(v, "op", []string{"recycle", "restore", "purge", "expire", "exception", "reset", "empty"})
		if err != nil {
			return nil, err
		}
		f.Ops = ops
	}
	if v := p.val("name"); v != "" {
		f.Name = v
	}
	if v := p.val("path"); v != "" {
		abs, err := filepath.Abs(expandHome(v))
		if err != nil {
			return nil, err
		}
		f.PathPrefix = filepath.Clean(abs)
	}
	if v := p.val("older-than"); v != "" {
		b, err := timeBound(v, now, true)
		if err != nil {
			return nil, err
		}
		f.RecycledHi = b
	}
	if v := p.val("newer-than"); v != "" {
		b, err := timeBound(v, now, false)
		if err != nil {
			return nil, err
		}
		f.RecycledLo = b
	}
	if v := p.val("since"); v != "" {
		b, err := timeBound(v, now, false)
		if err != nil {
			return nil, err
		}
		f.OpLo = b
	}
	if v := p.val("until"); v != "" {
		b, err := timeBound(v, now, true)
		if err != nil {
			return nil, err
		}
		f.OpHi = b
	}
	if v := p.val("mtime"); v != "" {
		spec, err := model.ParseTimeArg(v, now)
		if err != nil {
			return nil, err
		}
		f.Mtime = spec
	}
	if v := p.val("size"); v != "" {
		spec, err := model.ParseSizeArg(v)
		if err != nil {
			return nil, err
		}
		f.Size = spec
	}
	if p.has("expired") {
		f.Expired = true
	}
	if v := p.val("last"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, usageErr("%s", i18n.Tr("--last 需要一个正整数, 得到 %q", "--last wants a positive integer, got %q", v))
		}
		f.Last = n
	}
	sorts, err := a.parseSorts(p, forLog)
	if err != nil {
		return nil, err
	}
	f.Sort = sorts
	return f, nil
}

var binSortFields = map[string]bool{
	"id": true, "path": true, "fname": true, "name": true, "size": true, "fsize": true,
	"mtime": true, "fdate": true, "recycled": true, "rdate": true,
	"expire": true, "cdate": true, "state": true,
}

var logSortFields = map[string]bool{
	"seq": true, "ts": true, "time": true, "date": true, "op": true,
	"item": true, "item_id": true, "path": true, "fname": true, "name": true, "size": true,
}

func (a *app) parseSorts(p *parser, forLog bool) ([]model.SortKey, error) {
	allowed := binSortFields
	if forLog {
		allowed = logSortFields
	}
	var keys []model.SortKey
	for _, s := range p.vals("sort") {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			field, dir := part, false
			if c := strings.LastIndex(part, ":"); c >= 0 {
				field = strings.TrimSpace(part[:c])
				switch strings.ToLower(strings.TrimSpace(part[c+1:])) {
				case "asc":
					dir = false
				case "desc":
					dir = true
				default:
					return nil, usageErr("%s", i18n.Tr("--sort 方向只支持 asc|desc, 得到 %q", "--sort direction must be asc|desc, got %q", part[c+1:]))
				}
			}
			if !allowed[strings.ToLower(field)] {
				return nil, usageErr("%s", i18n.Tr("--sort 字段 %q 不存在 (见 'adrm help')", "--sort field %q does not exist (see 'adrm help')", field))
			}
			keys = append(keys, model.SortKey{Field: strings.ToLower(field), Desc: dir})
		}
	}
	return keys, nil
}

func parseEnum(v, name string, allowed []string) ([]string, error) {
	set := map[string]bool{}
	for _, a := range allowed {
		set[a] = true
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		if !set[part] {
			return nil, usageErr("%s", i18n.Tr("%s 的有效值: %s, 得到 %q", "valid values for %s: %s, got %q",
				name, strings.Join(allowed, "|"), part))
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil, usageErr("%s", i18n.Tr("%s 需要一个值", "%s needs a value", name))
	}
	return out, nil
}

func timeBound(arg string, now int64, useHi bool) (*int64, error) {
	spec, err := model.ParseTimeArg(arg, now)
	if err != nil {
		return nil, err
	}
	if useHi {
		if spec.Hi != nil {
			return spec.Hi, nil
		}
		return spec.Lo, nil
	}
	if spec.Lo != nil {
		return spec.Lo, nil
	}
	return spec.Hi, nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~") {
		if home, err := userHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			if strings.HasPrefix(p, "~/") {
				return filepath.Join(home, p[2:])
			}
		}
	}
	return p
}

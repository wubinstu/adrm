package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/model"
	"github.com/wubinstu/adrm/internal/trash"
)

// runRecycle implements the default rm-compatible mode.
//
// Grammar: [options] <file...> [expiry...]
//   - an expiry token (+3d, --for 6d, --until DATE) applies to every following
//     file until the next expiry token (right-associative);
//   - the whole command line is validated before anything is moved;
//   - a dangling expiry token (no file after it) is an error.
func (a *app) runRecycle(args []string) int {
	now := time.Now().Unix()
	defaultExpire := now + int64(a.cfg.RetentionDays)*86400
	var targets []trash.Target
	opt := trash.RecycleOpts{}

	curExpire := defaultExpire
	pendingExpiry := false // an expiry token not yet followed by a file
	stopOpts := false

	err := func() error {
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
				switch name {
				case "force":
					opt.Force = true
				case "recursive":
					opt.Recursive = true
				case "dir":
					opt.Dir = true
				case "verbose":
					opt.Verbose = true
				case "yes":
					opt.Yes = true
				case "dry-run":
					opt.DryRun = true
				case "interactive":
					opt.Interactive = true
				case "preserve-root":
					// default; accepted for rm compatibility
				case "no-preserve-root":
					opt.NoPreserveRoot = true
				case "help":
					a.printHelp(nil)
					return errDone
				case "version":
					fmt.Fprintf(a.out, "adrm %s\n", version)
					return errDone
				case "lang":
					if !hasVal {
						if i+1 >= len(args) {
							return usageErr("%s", i18n.Tr("--lang 需要一个值", "--lang requires a value"))
						}
						i++
						val = args[i]
					}
					switch strings.ToLower(val) {
					case "zh", "en":
						a.applyLang(a.cfg.Lang, val, "")
					default:
						return usageErr("%s", i18n.Tr("--lang 只接受 zh|en, 得到 %q", "--lang accepts zh|en, got %q", val))
					}
				case "color":
					if !hasVal {
						if i+1 >= len(args) {
							return usageErr("%s", i18n.Tr("--color 需要一个值", "--color requires a value"))
						}
						i++
						val = args[i]
					}
					switch val {
					case "auto", "always", "never":
						a.cfg.Color = val
					default:
						return usageErr("%s", i18n.Tr("--color 只接受 auto|always|never, 得到 %q", "--color accepts auto|always|never, got %q", val))
					}
				case "home":
					if !hasVal {
						if i+1 >= len(args) {
							return usageErr("%s", i18n.Tr("--home 需要一个值", "--home requires a value"))
						}
						i++
						val = args[i]
					}
					// already applied by the config prescan
				case "for":
					if !hasVal {
						if i+1 >= len(args) {
							return usageErr("%s", i18n.Tr("--for 需要一个持续时间, 如 6d", "--for requires a duration, e.g. 6d"))
						}
						i++
						val = args[i]
					}
					d, err := model.ParseDurationSign(val)
					if err != nil {
						return usageErr("%s", i18n.Tr("--for 的持续时间无效: %v", "invalid duration for --for: %v", err))
					}
					curExpire = now + d
					pendingExpiry = true
				case "until":
					if !hasVal {
						if i+1 >= len(args) {
							return usageErr("%s", i18n.Tr("--until 需要一个日期时间", "--until requires a date/time"))
						}
						i++
						val = args[i]
					}
					spec, err := model.ParseTimeArg(val, now)
					if err != nil {
						return usageErr("%s", i18n.Tr("--until 的时间无效: %v", "invalid time for --until: %v", err))
					}
					ts := int64(0)
					if spec.Hi != nil {
						ts = *spec.Hi
					} else if spec.Lo != nil {
						ts = *spec.Lo
					}
					curExpire = ts
					pendingExpiry = true
				default:
					return usageErr("%s", i18n.Tr("未知选项 --%s (见 'adrm help')", "unknown option --%s (see 'adrm help')", name))
				}
				continue
			}
			if !stopOpts && strings.HasPrefix(tok, "-") && len(tok) > 1 {
				for j := 1; j < len(tok); j++ {
					switch tok[j] {
					case 'f':
						opt.Force = true
					case 'i':
						opt.Interactive = true
					case 'I':
						opt.InteractiveN = true
					case 'r', 'R':
						opt.Recursive = true
					case 'd':
						opt.Dir = true
					case 'v':
						opt.Verbose = true
					case 'y':
						opt.Yes = true
					case 'h':
						a.printHelp(nil)
						return errDone
					default:
						return usageErr("%s", i18n.Tr("无效选项 -%s (见 'adrm help')", "invalid option -%s (see 'adrm help')", string(tok[j])))
					}
				}
				continue
			}
			// operand: file, or a [+-]duration expiry token
			if !stopOpts && strings.HasPrefix(tok, "+") {
				d, err := model.ParseDurationSign(tok)
				if err != nil {
					// not a duration: treat as a file name starting with '+'
					targets = append(targets, trash.Target{Path: tok, ExpireAt: curExpire})
					pendingExpiry = false
					continue
				}
				curExpire = now + d
				pendingExpiry = true
				continue
			}
			targets = append(targets, trash.Target{Path: tok, ExpireAt: curExpire})
			pendingExpiry = false
		}
		return nil
	}()
	if err == errDone {
		return 0
	}
	if err != nil {
		fmt.Fprintf(a.errw, "adrm: %v\n", err)
		return 2
	}
	if pendingExpiry {
		fmt.Fprintf(a.errw, "adrm: %s\n", i18n.Tr("错误: 行尾的持续时间参数没有对应的文件 (持续时间向右亲和, 必须后接文件)",
			"error: trailing expiry argument has no file (durations are right-associative and must be followed by files)"))
		return 2
	}
	if len(targets) == 0 {
		a.printUsage()
		return 1
	}
	eng, err := a.engine()
	if err != nil {
		fmt.Fprintf(a.errw, "adrm: %v\n", err)
		return 2
	}
	defer a.closeStore()
	if err := eng.Recycle(targets, opt); err != nil {
		fmt.Fprintf(a.errw, "adrm: %v\n", err)
		return 1
	}
	return 0
}

// errDone is a sentinel meaning "already handled, exit 0".
var errDone = fmt.Errorf("done")

// userHomeDir is a thin wrapper for testability.
func userHomeDir() (string, error) { return os.UserHomeDir() }

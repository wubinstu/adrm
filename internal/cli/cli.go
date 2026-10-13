// Package cli implements the adrm command line interface: the rm-compatible
// default mode plus all management subcommands, strict argument validation,
// and the hidden __complete entry point used by the shell completions.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wubinstu/adrm/internal/config"
	"github.com/wubinstu/adrm/internal/i18n"
	"github.com/wubinstu/adrm/internal/store"
	"github.com/wubinstu/adrm/internal/trash"
)

// version is the adrm version. Release builds override it via
// -ldflags "-X github.com/wubinstu/adrm/internal/cli.version=<tag>";
// local builds report "dev".
var version = "dev"

var subcommands = map[string]bool{
	"ls": true, "log": true, "restore": true, "undo": true, "purge": true,
	"gc": true, "empty": true, "stats": true, "config": true, "completions": true,
	"setup": true, "doctor": true, "db": true,
}

// Mode selects which face of the CLI is running. adrm and adrmm are two
// binaries built from the same tree so that shell completion can keep the
// two vocabularies strictly apart: adrm completes file paths (like /bin/rm),
// adrmm completes subcommands (like git).
type Mode int

const (
	// ModeDelete is the rm-compatible binary: adrm <files...>
	ModeDelete Mode = iota
	// ModeManage is the trash-management binary: adrmm <subcommand> ...
	ModeManage
)

// app holds process-wide state for one CLI invocation.
type app struct {
	mode           Mode
	cfg            *config.Config
	st             *store.Store
	eng            *trash.Engine
	in             io.Reader
	out, errw      io.Writer
	storeAttempted bool
}

// Name is the program name as the user typed it.
func (a *app) Name() string {
	if a.mode == ModeManage {
		return "adrmm"
	}
	return "adrm"
}

// Run executes the CLI and returns the process exit code. mode picks the
// binary face (see Mode).
func Run(args []string, in io.Reader, out, errw io.Writer, mode Mode) int {
	a := &app{mode: mode, in: in, out: out, errw: errw}

	// Bootstrap language as early as possible from config + locale so that
	// even early messages are localized.
	home := prescanHome(args)
	cfg, err := config.Load(home)
	if err != nil {
		fmt.Fprintf(errw, "adrm: %v\n", err)
		return 2
	}
	a.cfg = cfg
	a.applyLang(resolveLang(cfg.Lang), "", "")

	if len(args) == 0 {
		a.printUsage()
		return 1
	}

	if a.mode == ModeManage {
		return a.runManage(args)
	}

	// ModeDelete: pure rm-compatible behavior. No subcommands exist here, so
	// "adrm ls" recycles a file literally named ls (use ./ls for cwd files).
	if cmd, idx := scanCommand(args); cmd == "__complete" {
		return a.runComplete(args[idx+1:])
	}
	if hasFlag(args, "--help", "-h") {
		a.printHelp(nil)
		return 0
	}
	if hasFlag(args, "--version", "-V") {
		fmt.Fprintf(out, "adrm %s\n", version)
		return 0
	}
	return a.runRecycle(args)
}

// runManage dispatches adrmm: the first non-option token is the subcommand.
func (a *app) runManage(args []string) int {
	cmd, cmdIdx := scanCommand(args)
	if cmdIdx < 0 {
		if hasFlag(args, "--help", "-h") {
			a.printHelp(nil)
			return 0
		}
		if hasFlag(args, "--version", "-V") {
			fmt.Fprintf(a.out, "adrmm %s\n", version)
			return 0
		}
		a.printUsage()
		return 1
	}
	switch cmd {
	case "help":
		a.printHelp(restAfter(args, cmdIdx))
		return 0
	case "version":
		fmt.Fprintf(a.out, "adrmm %s\n", version)
		return 0
	case "__complete":
		return a.runComplete(args[cmdIdx+1:])
	}
	if subcommands[cmd] {
		return a.runSubcommand(cmd, append(args[:cmdIdx:cmdIdx], args[cmdIdx+1:]...))
	}
	return a.fail(usageErr("%s", i18n.Tr("未知子命令: %s (输入 'adrmm help' 查看全部)", "unknown command: %s (try 'adrmm help')", cmd)))
}

// scanCommand finds the first non-option token (the subcommand, if any),
// skipping the values of global value-taking options. It returns -1 when
// only options were given or when "--" ends option parsing.
func scanCommand(args []string) (string, int) {
	valueOpts := map[string]bool{"--lang": true, "--color": true, "--home": true}
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if tok == "--" {
			return "", -1
		}
		if strings.HasPrefix(tok, "--") {
			if eq := strings.Index(tok, "="); eq >= 0 {
				continue
			}
			if valueOpts[tok] {
				i++
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && len(tok) > 1 {
			continue
		}
		return tok, i
	}
	return "", -1
}

func hasFlag(args []string, names ...string) bool {
	for _, a := range args {
		for _, n := range names {
			if a == n {
				return true
			}
		}
	}
	return false
}

func restAfter(args []string, i int) []string {
	if i+1 >= len(args) {
		return nil
	}
	return args[i+1:]
}

// engine lazily builds the store + trash engine.
func (a *app) engine() (*trash.Engine, error) {
	if a.eng != nil {
		return a.eng, nil
	}
	if a.storeAttempted {
		return nil, fmt.Errorf("store unavailable")
	}
	a.storeAttempted = true
	if err := a.cfg.EnsureHome(); err != nil {
		return nil, err
	}
	st, err := store.Open(a.cfg.Database)
	if err != nil {
		return nil, err
	}
	a.st = st
	eng, err := trash.New(a.cfg, st)
	if err != nil {
		return nil, err
	}
	eng.In, eng.Out, eng.Err = a.in, a.out, a.errw
	a.eng = eng
	return eng, nil
}

func (a *app) closeStore() {
	if a.st != nil {
		_ = a.st.Close()
	}
}

// prescanHome finds --home / --home=... before "--" so the config file can be
// located before full parsing happens.
func prescanHome(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			return ""
		}
		switch {
		case args[i] == "--home" && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(args[i], "--home="):
			return strings.TrimPrefix(args[i], "--home=")
		}
	}
	return ""
}

// applyLang resolves the language: CLI flag > env ADRM_LANG > config > locale.
func (a *app) applyLang(cfgLang, cliLang, env string) {
	switch {
	case cliLang != "":
		i18n.Set(cliLang)
	case env != "":
		i18n.Set(env)
	case cfgLang == "auto" || cfgLang == "":
		i18n.SetAuto(localeOf())
	default:
		i18n.Set(cfgLang)
	}
}

func resolveLang(cfgLang string) string {
	if cfgLang == "" {
		return "auto"
	}
	return cfgLang
}

func localeOf() string {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

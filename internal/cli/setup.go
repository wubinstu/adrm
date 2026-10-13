package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wubinstu/adrm/internal/complete"
	"github.com/wubinstu/adrm/internal/i18n"
)

// markers bracket everything adrm writes into shell rc files, so uninstall
// can remove exactly what install added and nothing else.
const (
	rcMarkBegin = "# >>> adrm >>>"
	rcMarkEnd   = "# <<< adrm <<<"
)

// shellProfile describes one supported shell and its rc file.
type shellProfile struct {
	Shell string
	Rc    string
	Fish  bool
}

// shellProfiles lists the rc files adrm manages.
func shellProfiles(home string) []shellProfile {
	return []shellProfile{
		{"bash", filepath.Join(home, ".bashrc"), false},
		{"zsh", filepath.Join(home, ".zshrc"), false},
		{"fish", filepath.Join(home, ".config", "fish", "config.fish"), true},
	}
}

// globalCompletionDirs returns the system-wide completion directories that
// exist (or can be created) for each shell. A nil entry means "not
// available"; the caller then falls back to a per-user directory.
func globalCompletionDirs() map[string]string {
	candidates := map[string][]string{
		// bash-completion lazy-loads from the share dir; the /etc variant is
		// the fallback on systems without the share layout.
		"bash": {"/usr/share/bash-completion/completions", "/etc/bash_completion.d"},
		"zsh":  {"/usr/share/zsh/site-functions", "/usr/local/share/zsh/site-functions"},
		"fish": {"/usr/share/fish/vendor_completions.d", "/usr/local/share/fish/vendor_completions.d"},
	}
	out := map[string]string{}
	for shell, dirs := range candidates {
		for _, d := range dirs {
			if isDir(d) {
				out[shell] = d
				break
			}
			if canCreate(d) {
				out[shell] = d
				break
			}
		}
	}
	return out
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// canCreate reports whether the parent of p is writable, i.e. mkdir -p p
// would succeed for the current user.
func canCreate(p string) bool {
	parent := filepath.Dir(p)
	for parent != "/" && parent != "." {
		if isDir(parent) {
			return unixWritable(parent)
		}
		parent = filepath.Dir(parent)
	}
	return false
}

func unixWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".adrm-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// userCompletionDirs are the per-user fallbacks.
func userCompletionDirs(home string) map[string]string {
	return map[string]string{
		"bash": filepath.Join(home, ".local", "share", "bash-completion", "completions"),
		"zsh":  filepath.Join(home, ".zfunc"),
		"fish": filepath.Join(home, ".config", "fish", "completions"),
	}
}

// completionInstallPlan decides where each shell's completion files go.
type completionPlan struct {
	// Dirs maps shell -> directory (global when available, else per-user).
	Dirs map[string]string
	// Global reports whether at least one shell got a system-wide directory.
	Global bool
	// LocalDirs holds the per-user fallback for shells without a global dir.
	LocalDirs map[string]string
}

func planCompletions(home string) completionPlan {
	glob := globalCompletionDirs()
	loc := userCompletionDirs(home)
	plan := completionPlan{Dirs: map[string]string{}, LocalDirs: loc}
	for _, sh := range []string{"bash", "zsh", "fish"} {
		if d, ok := glob[sh]; ok {
			plan.Dirs[sh] = d
			plan.Global = true
		} else {
			plan.Dirs[sh] = loc[sh]
		}
	}
	return plan
}

// completionFiles returns the two scripts (adrm + admr) for a shell.
func completionFiles(shell string) map[string]string {
	switch shell {
	case "zsh":
		return map[string]string{
			complete.FileName("zsh", "adrm"): complete.Zsh("adrm"),
			complete.FileName("zsh", "admr"): complete.Zsh("admr"),
		}
	case "fish":
		return map[string]string{
			complete.FileName("fish", "adrm"): complete.Fish("adrm"),
			complete.FileName("fish", "admr"): complete.Fish("admr"),
		}
	default:
		return map[string]string{
			complete.FileName("bash", "adrm"): complete.Bash("adrm"),
			complete.FileName("bash", "admr"): complete.Bash("admr"),
		}
	}
}

// cmdSetup installs or removes the shell integration.
func (a *app) cmdSetup(args []string) int {
	p := newParser()
	specs := concatSpecs(commonSpecs, []optSpec{
		{long: "install"}, {long: "uninstall"}, {long: "status"},
		{long: "alias"}, {long: "no-alias"},
		{long: "completion"}, {long: "no-completion"},
		{long: "timer"}, {long: "no-timer"},
		{long: "local"},
		{long: "purge-home"}, {long: "yes", short: "y"},
		{long: "shells", takesValue: true},
	})
	if err := p.parse(args, specs); err != nil {
		return a.fail(err)
	}
	if p.has("help") || (!p.has("install") && !p.has("uninstall") && !p.has("status")) {
		a.printHelp([]string{"setup"})
		return 0
	}
	if err := a.applyCommon(p); err != nil {
		return a.fail(err)
	}
	switch {
	case p.has("install"):
		return a.setupInstall(p)
	case p.has("uninstall"):
		return a.setupUninstall(p)
	case p.has("status"):
		return a.setupStatus(p)
	}
	return 0
}

func (a *app) setupInstall(p *parser) int {
	if err := a.cfg.EnsureHome(); err != nil {
		return a.fail(err)
	}
	home, err := userHomeDir()
	if err != nil {
		return a.fail(err)
	}
	// The rm alias is opt-in: by default adrm is reachable as /usr/bin/adrm
	// and rm keeps its normal behavior.
	wantAlias := p.has("alias") && !p.has("no-alias")
	wantCompletion := !p.has("no-completion")
	wantTimer := p.has("timer") && !p.has("no-timer")
	shells := a.selectedShells(p)
	forceLocal := p.has("local")

	plan := planCompletions(home)
	if forceLocal {
		plan = completionPlan{Dirs: map[string]string{}, Global: false, LocalDirs: userCompletionDirs(home)}
		for _, sh := range shells {
			plan.Dirs[sh] = plan.LocalDirs[sh]
		}
	}

	// 1. completion scripts: one pair (adrm + admr) per shell
	if wantCompletion {
		for _, sh := range shells {
			dir := plan.Dirs[sh]
			if dir == "" {
				continue
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return a.fail(err)
			}
			for name, script := range completionFiles(sh) {
				path := filepath.Join(dir, name)
				if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
					return a.fail(err)
				}
				fmt.Fprintf(a.out, i18n.Tr("已写入补全脚本: %s\n", "wrote completion script: %s\n"), path)
			}
		}
	}

	// 2. init file sourced by every shell block
	initPath := filepath.Join(a.cfg.Home, "adrm-init.sh")
	var init strings.Builder
	fmt.Fprintf(&init, "# adrm shell integration (generated by 'admr setup --install')\n")
	fmt.Fprintf(&init, "export ADRM_HOME=%q\n", a.cfg.Home)
	if wantAlias {
		fmt.Fprintf(&init, "alias rm='adrm'\n")
	}
	if err := os.WriteFile(initPath, []byte(init.String()), 0o644); err != nil {
		return a.fail(err)
	}
	fmt.Fprintf(a.out, i18n.Tr("已写入初始化脚本: %s\n", "wrote init script: %s\n"), initPath)

	// 3. rc file blocks (ADRM_HOME, optional alias, optional rm completion
	//    takeover, and the zsh compinit bootstrap when needed)
	for _, prof := range shellProfiles(home) {
		if !containsStr(shells, prof.Shell) {
			continue
		}
		if _, err := os.Stat(prof.Rc); err != nil && !os.IsNotExist(err) {
			continue
		}
		block := rcBlock(prof, a.cfg.Home, wantAlias, wantCompletion, plan, forceLocal)
		if err := appendRcBlock(prof.Rc, block); err != nil {
			return a.fail(err)
		}
		fmt.Fprintf(a.out, i18n.Tr("已更新 %s\n", "updated %s\n"), prof.Rc)
	}

	// 4. optional systemd --user timer for lazy gc
	if wantTimer {
		a.installTimer()
	}
	if plan.Global && !forceLocal {
		fmt.Fprintln(a.out, i18n.Tr("补全已安装到系统目录, 所有用户可用。",
			"completions installed system-wide, available to every user."))
	}
	fmt.Fprintln(a.out, i18n.Tr("安装完成。打开新终端或执行 'source ~/.bashrc' 后生效。",
		"done. open a new shell or run 'source ~/.bashrc' to activate."))
	return 0
}

func (a *app) selectedShells(p *parser) []string {
	if v := p.val("shells"); v != "" {
		var out []string
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(strings.ToLower(s))
			if s == "bash" || s == "zsh" || s == "fish" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	// auto-detect from $SHELL, default bash
	if sh := os.Getenv("SHELL"); sh != "" {
		base := filepath.Base(sh)
		switch base {
		case "zsh", "fish":
			return []string{base}
		}
	}
	return []string{"bash"}
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// rcBlock renders the managed block for one shell profile.
func rcBlock(prof shellProfile, home string, wantAlias, wantCompletion bool, plan completionPlan, forceLocal bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", rcMarkBegin)
	fmt.Fprintf(&b, "# adrm: shell integration managed by 'admr setup' (remove with 'admr setup --uninstall')\n")
	if prof.Fish {
		fmt.Fprintf(&b, "set -gx ADRM_HOME %q\n", home)
		if wantAlias {
			fmt.Fprintf(&b, "alias rm='adrm'\n")
			// take rm's completion over from /usr/bin/rm
			fmt.Fprintf(&b, "complete -c rm -w adrm\n")
		}
		if wantCompletion && forceLocal {
			// only the per-user fallback needs an explicit source line
			fish := filepath.Join(plan.Dirs["fish"], complete.FileName("fish", "adrm"))
			fmt.Fprintf(&b, "test -f %q; and source %q\n", fish, fish)
			fishm := filepath.Join(plan.Dirs["fish"], complete.FileName("fish", "admr"))
			fmt.Fprintf(&b, "test -f %q; and source %q\n", fishm, fishm)
		}
	} else {
		// POSIX shells (bash/zsh): "[ -r f ] && . f" -- never fish's "; and".
		fmt.Fprintf(&b, "export ADRM_HOME=%q\n", home)
		init := filepath.Join(home, "adrm-init.sh")
		fmt.Fprintf(&b, "[ -r %q ] && . %q\n", init, init)
		if wantCompletion {
			switch prof.Shell {
			case "zsh":
				// make sure the completion function exists before it is used,
				// and that compdef is available for the per-user fpath dir
				fmt.Fprintf(&b, "(( $+functions[compdef] )) || { autoload -Uz compinit && compinit -u }\n")
				if forceLocal {
					fmt.Fprintf(&b, "fpath=(%q $fpath)\n", plan.Dirs["zsh"])
				}
				zsh := filepath.Join(plan.Dirs["zsh"], complete.FileName("zsh", "adrm"))
				fmt.Fprintf(&b, "[ -r %q ] && . %q\n", zsh, zsh)
				zshm := filepath.Join(plan.Dirs["zsh"], complete.FileName("zsh", "admr"))
				fmt.Fprintf(&b, "[ -r %q ] && . %q\n", zshm, zshm)
			default:
				// source unconditionally (not only for --local): the system
				// completion may be loaded lazily, and _adrm_complete must
				// exist before it is bound to rm below.
				bash := filepath.Join(plan.Dirs["bash"], complete.FileName("bash", "adrm"))
				fmt.Fprintf(&b, "[ -r %q ] && . %q\n", bash, bash)
				bashm := filepath.Join(plan.Dirs["bash"], complete.FileName("bash", "admr"))
				fmt.Fprintf(&b, "[ -r %q ] && . %q\n", bashm, bashm)
			}
		}
		if wantAlias {
			switch prof.Shell {
			case "zsh":
				// take rm's completion over from /usr/bin/rm
				fmt.Fprintf(&b, "compdef _adrm rm 2>/dev/null\n")
			default:
				// "|| true" keeps the block safe under 'set -e': complete -r
				// returns 1 when rm has no completion registered yet.
				fmt.Fprintf(&b, "complete -r rm 2>/dev/null || true\n")
				fmt.Fprintf(&b, "complete -o default -o filenames -F _adrm_complete rm\n")
			}
		}
	}
	fmt.Fprintf(&b, "%s\n", rcMarkEnd)
	return b.String()
}

// appendRcBlock removes an existing adrm block and appends the new one.
func appendRcBlock(rc, block string) error {
	data, err := os.ReadFile(rc)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	clean := removeRcBlock(string(data))
	sep := ""
	if len(clean) > 0 && !strings.HasSuffix(clean, "\n") {
		sep = "\n"
	}
	out := clean + sep + block
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(rc), 0o755); err != nil {
		return err
	}
	return os.WriteFile(rc, []byte(out), 0o644)
}

// removeRcBlock strips a previously installed adrm block (markers included).
func removeRcBlock(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inBlock := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == rcMarkBegin {
			inBlock = true
			continue
		}
		if t == rcMarkEnd {
			inBlock = false
			continue
		}
		if !inBlock {
			out = append(out, ln)
		}
	}
	res := strings.Join(out, "\n")
	// collapse triple blank lines left behind
	for strings.Contains(res, "\n\n\n") {
		res = strings.ReplaceAll(res, "\n\n\n", "\n\n")
	}
	return strings.TrimRight(res, "\n") + "\n"
}

func (a *app) setupUninstall(p *parser) int {
	home, err := userHomeDir()
	if err != nil {
		return a.fail(err)
	}
	// 1. rc blocks
	for _, prof := range shellProfiles(home) {
		data, err := os.ReadFile(prof.Rc)
		if err != nil {
			continue
		}
		if !strings.Contains(string(data), rcMarkBegin) {
			continue
		}
		clean := removeRcBlock(string(data))
		if err := os.WriteFile(prof.Rc, []byte(clean), 0o644); err != nil {
			return a.fail(err)
		}
		fmt.Fprintf(a.out, i18n.Tr("已清理 %s\n", "cleaned %s\n"), prof.Rc)
	}
	// 2. completion scripts, wherever they were written
	removed := 0
	for _, dirs := range []map[string]string{globalCompletionDirs(), userCompletionDirs(home)} {
		for _, dir := range dirs {
			for _, names := range []map[string]string{
				completionFiles("bash"), completionFiles("zsh"), completionFiles("fish"),
			} {
				for name := range names {
					path := filepath.Join(dir, name)
					if err := os.Remove(path); err == nil {
						removed++
						fmt.Fprintf(a.out, i18n.Tr("已删除补全脚本: %s\n", "removed completion script: %s\n"), path)
					}
				}
			}
		}
	}
	// per-user copy under the adrm home (legacy location)
	for _, sh := range []string{"bash", "zsh", "fish"} {
		for name := range completionFiles(sh) {
			path := filepath.Join(a.cfg.Home, "completions", name)
			if err := os.Remove(path); err == nil {
				removed++
				fmt.Fprintf(a.out, i18n.Tr("已删除补全脚本: %s\n", "removed completion script: %s\n"), path)
			}
		}
	}
	if removed == 0 {
		fmt.Fprintln(a.out, i18n.Tr("没有找到已安装的补全脚本。", "no installed completion scripts found."))
	}
	if err := os.Remove(filepath.Join(a.cfg.Home, "adrm-init.sh")); err == nil {
		fmt.Fprintln(a.out, i18n.Tr("已删除初始化脚本 adrm-init.sh。", "removed the init script adrm-init.sh."))
	}
	if err := os.Remove(filepath.Join(a.cfg.Home, "completions")); err == nil {
		fmt.Fprintln(a.out, i18n.Tr("已删除补全目录。", "removed the completions directory."))
	}
	// 3. systemd user timer
	a.uninstallTimer()
	// 4. optionally the whole home (the trash itself)
	if p.has("purge-home") {
		if !p.has("yes") {
			fmt.Fprintf(a.out, i18n.Tr("警告: 将删除整个 adrm 主目录 %s (含回收站内所有文件!)\n",
				"warning: about to delete the whole adrm home %s (including everything in the trash!)\n"), a.cfg.Home)
			eng, err := a.engine()
			if err != nil {
				return a.fail(err)
			}
			if !eng.Confirm(i18n.Tr("确认删除?", "really delete?"), false) {
				fmt.Fprintln(a.out, i18n.Tr("已取消(回收站数据保留)。", "cancelled (trash data kept)."))
				return 0
			}
		}
		if err := os.RemoveAll(a.cfg.Home); err != nil {
			return a.fail(err)
		}
		fmt.Fprintf(a.out, i18n.Tr("已删除 %s\n", "removed %s\n"), a.cfg.Home)
	}
	fmt.Fprintln(a.out, i18n.Tr("卸载完成。如需删除二进制本身, 请手动移除安装路径下的 adrm/admr。",
		"uninstall done. to remove the binaries themselves, delete adrm/admr from your install directory."))
	return 0
}

func (a *app) setupStatus(p *parser) int {
	home, err := userHomeDir()
	if err != nil {
		return a.fail(err)
	}
	fmt.Fprintf(a.out, i18n.Tr("adrm 主目录: %s\n", "adrm home: %s\n"), a.cfg.Home)
	if _, err := os.Stat(a.cfg.ConfigFile); err == nil {
		fmt.Fprintf(a.out, i18n.Tr("配置文件: %s\n", "config file: %s\n"), a.cfg.ConfigFile)
	} else {
		fmt.Fprintln(a.out, i18n.Tr("配置文件: 不存在 (运行 'admr config --init' 生成)", "config file: missing (run 'admr config --init')"))
	}
	plan := planCompletions(home)
	for _, sh := range []string{"bash", "zsh", "fish"} {
		dir := plan.Dirs[sh]
		if dir == "" {
			continue
		}
		where := i18n.Tr("用户目录", "user dir")
		if plan.Global && dir == globalCompletionDirs()[sh] {
			where = i18n.Tr("系统目录", "system dir")
		}
		for name := range completionFiles(sh) {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				fmt.Fprintf(a.out, i18n.Tr("补全 %s: %s [已安装, %s]\n", "completion %s: %s [installed, %s]\n"), sh, path, where)
			}
		}
	}
	for _, prof := range shellProfiles(home) {
		data, err := os.ReadFile(prof.Rc)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), rcMarkBegin) {
			fmt.Fprintf(a.out, i18n.Tr("shell 集成: %s [已安装]\n", "shell integration: %s [installed]\n"), prof.Rc)
		} else if _, err := os.Stat(prof.Rc); err == nil {
			fmt.Fprintf(a.out, i18n.Tr("shell 集成: %s [未安装]\n", "shell integration: %s [not installed]\n"), prof.Rc)
		}
	}
	if u, ok := timerUnitPaths(); ok {
		for _, p := range u {
			if _, err := os.Stat(p); err == nil {
				fmt.Fprintf(a.out, i18n.Tr("定时清理: %s [已安装]\n", "scheduled cleanup: %s [installed]\n"), p)
			}
		}
		if enabled, _ := timerEnabled(); enabled {
			fmt.Fprintln(a.out, i18n.Tr("定时清理: systemd --user adrm-gc.timer [已启用]",
				"scheduled cleanup: systemd --user adrm-gc.timer [enabled]"))
		}
	}
	return 0
}

// installTimer installs a systemd --user timer that runs `admr gc --quiet`
// daily. It degrades gracefully when systemd or the user bus is unavailable.
func (a *app) installTimer() {
	paths, ok := timerUnitPaths()
	if !ok {
		fmt.Fprintln(a.out, i18n.Tr("未找到 systemd 用户目录, 跳过定时清理安装。",
			"no systemd user directory found, skipping the timer."))
		a.printTimerHints()
		return
	}
	bin := "admr"
	if exe, err := os.Executable(); err == nil {
		bin = exe
	}
	service := fmt.Sprintf(`[Unit]
Description=adrm trash bin cleanup

[Service]
Type=oneshot
ExecStart=%s gc --quiet
`, bin)
	timer := `[Unit]
Description=Daily adrm trash bin cleanup

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
`
	for path, content := range map[string]string{paths[0]: service, paths[1]: timer} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			a.fail(err)
			return
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			a.fail(err)
			return
		}
		fmt.Fprintf(a.out, i18n.Tr("已写入 %s\n", "wrote %s\n"), path)
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		fmt.Fprintln(a.out, i18n.Tr("未找到 systemctl, 已写入单元文件但未启用。",
			"systemctl not found; unit files written but not enabled."))
		return
	}
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		fmt.Fprintf(a.out, i18n.Tr("daemon-reload 失败(%v), 未启用定时器。\n", "daemon-reload failed (%v), timer not enabled.\n"), err)
		_ = out
		return
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", "adrm-gc.timer").CombinedOutput(); err != nil {
		fmt.Fprintf(a.out, i18n.Tr("启用定时器失败(可能需要登录会话/systemd --user): %v\n",
			"failed to enable the timer (a login session with systemd --user may be needed): %v\n"), err)
		_ = out
		return
	}
	fmt.Fprintln(a.out, i18n.Tr("已启用 systemd --user 定时清理 (每天一次)。",
		"enabled the systemd --user cleanup timer (runs daily)."))
}

func (a *app) uninstallTimer() {
	paths, ok := timerUnitPaths()
	if !ok {
		return
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		_, _ = exec.Command("systemctl", "--user", "disable", "--now", "adrm-gc.timer").CombinedOutput()
		_, _ = exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput()
	}
	for _, p := range paths {
		if err := os.Remove(p); err == nil {
			fmt.Fprintf(a.out, i18n.Tr("已删除 %s\n", "removed %s\n"), p)
		}
	}
}

// timerUnitPaths returns the service and timer unit paths for this user.
func timerUnitPaths() ([]string, bool) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		h, err := userHomeDir()
		if err != nil {
			return nil, false
		}
		base = filepath.Join(h, ".config")
	}
	dir := filepath.Join(base, "systemd", "user")
	return []string{
		filepath.Join(dir, "adrm-gc.service"),
		filepath.Join(dir, "adrm-gc.timer"),
	}, true
}

func timerEnabled() (bool, error) {
	out, err := exec.Command("systemctl", "--user", "is-enabled", "adrm-gc.timer").CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), "enabled"), nil
}

func (a *app) printTimerHints() {
	fmt.Fprintln(a.out, i18n.Tr("可选定时清理 (手动执行其一即可):",
		"optional scheduled cleanup (run one of these):"))
	fmt.Fprintln(a.out, "  systemd: systemd-run --user --on-calendar=daily admr gc --quiet")
	fmt.Fprintln(a.out, "  cron:    (crontab -l; echo '0 9 * * * admr gc --quiet') | crontab -")
}

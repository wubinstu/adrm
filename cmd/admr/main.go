// Command admr manages the adrm trash bin: listing, restoring, purging,
// statistics, shell integration and self-checks. It is the companion of the
// rm-compatible adrm binary and keeps the two vocabularies (subcommands vs
// file paths) strictly apart so shell completion never mixes them.
package main

import (
	"os"

	"github.com/wubinstu/adrm/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, cli.ModeManage))
}

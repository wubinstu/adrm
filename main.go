// Command adrm is a safer replacement for rm: everything you delete goes to a
// per-user trash directory and can be listed, restored, or purged later.
package main

import (
	"os"

	"github.com/wubinstu/adrm/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

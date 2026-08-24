// Command yass serves an existing yass spec set to a coding agent.
//
// The behaviour of every subcommand is specified by the .yass.yaml files in
// this directory; root.yass.yaml is the entry point.
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/shakefu/yass/cli/internal/yass"
)

func main() {
	// Suppress the default SIGPIPE disposition so a write to a closed stdout
	// returns EPIPE to us instead of killing the process with a signal, which
	// would select an exit status outside root@ExitPolicy's table.
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
	os.Exit(yass.Main(os.Args[1:], os.Stdout, os.Stderr))
}

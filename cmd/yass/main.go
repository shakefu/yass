package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"golang.org/x/term"

	"github.com/shakefu/yass/internal/cli"
	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/exitcode"
)

func main() {
	// Set up panic recovery.
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("internal error: %v", r)
			fmt.Fprint(os.Stderr, yerrors.FormatError("yass", 0, yerrors.CodeInternalUncaught, msg))
			os.Exit(exitcode.Processing)
		}
	}()

	// Set up signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGPIPE, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGPIPE:
				os.Exit(exitcode.Success)
			case syscall.SIGINT:
				os.Stderr.Sync()
				os.Exit(exitcode.SigInt)
			case syscall.SIGTERM:
				os.Stderr.Sync()
				os.Exit(exitcode.SigTerm)
			}
		}
	}()

	// Determine if stdout is a TTY.
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// Get terminal width.
	termWidth := 80
	if isTTY {
		if colStr := os.Getenv("COLUMNS"); colStr != "" {
			if cols, err := strconv.Atoi(colStr); err == nil && cols > 0 {
				termWidth = cols
			} else {
				// COLUMNS is set but invalid; try OS query.
				if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
					termWidth = w
				}
			}
		} else {
			// No COLUMNS env var; query OS.
			if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
				termWidth = w
			}
		}
	}

	// Dispatch.
	code := cli.Dispatch(os.Args[1:], os.Stdout, os.Stderr, isTTY, termWidth)
	os.Exit(code)
}

package cli

import (
	"fmt"
	"io"
	"strings"

	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/exitcode"
	"github.com/shakefu/yass/internal/list"
	"github.com/shakefu/yass/internal/query"
	"github.com/shakefu/yass/internal/validate"
	"github.com/shakefu/yass/internal/version"
)

// knownSubcommands lists the valid subcommand names.
var knownSubcommands = []string{"validate", "query", "list"}

// Usage is the help text printed for --help or on error.
const Usage = `Usage: yass <command> [arguments]

Commands:
  validate    Validate .yass.yaml files
  list        List specs in .yass.yaml files
  query       Query a spec by name

Flags:
  --help      Show this help
  --version   Show version
`

// Dispatch parses global flags, selects a subcommand, and dispatches.
// args = os.Args[1:] (everything after the program name).
// Returns an exit code.
func Dispatch(args []string, stdout io.Writer, stderr io.Writer, isTTY bool, termWidth int) int {
	// 1. If --help appears anywhere in args, print usage to stdout and exit 0.
	for _, arg := range args {
		if arg == "--help" {
			fmt.Fprint(stdout, Usage)
			return exitcode.Success
		}
	}

	// 2. If --version appears anywhere in args, print version to stdout and exit 0.
	for _, arg := range args {
		if arg == "--version" {
			fmt.Fprintf(stdout, "yass %s\n", version.Version)
			return exitcode.Success
		}
	}

	// 3. Handle "--" as end-of-options: everything after it is positional.
	var preDash []string
	var postDash []string
	seenDashDash := false
	for _, arg := range args {
		if arg == "--" && !seenDashDash {
			seenDashDash = true
			continue
		}
		if seenDashDash {
			postDash = append(postDash, arg)
		} else {
			preDash = append(preDash, arg)
		}
	}

	// 4. Validate all args (check ALL args including those after subcommand).
	// Walk through preDash args in order. The first positional is the subcommand.
	subcommandIdx := -1

	for i, arg := range preDash {
		// a. Empty string.
		if arg == "" {
			emitError(stderr, yerrors.CodeArgvEmptyArgument,
				yerrors.Messages[yerrors.CodeArgvEmptyArgument])
			return exitcode.Usage
		}

		// b. Bare "-".
		if arg == "-" {
			emitError(stderr, yerrors.CodeArgvStdinDash,
				yerrors.Messages[yerrors.CodeArgvStdinDash])
			return exitcode.Usage
		}

		// c. Short flags (single dash + letters/digits).
		if isShortFlag(arg) {
			emitError(stderr, yerrors.CodeArgvShortFlag,
				fmt.Sprintf(yerrors.Messages[yerrors.CodeArgvShortFlag], arg))
			return exitcode.Usage
		}

		// d. Flags starting with -- (not --help/--version/--).
		if strings.HasPrefix(arg, "--") {
			lower := strings.ToLower(arg)
			if lower == "--help" || lower == "--version" {
				// Exact match was already caught in steps 1-2, so this is a case mismatch.
				emitError(stderr, yerrors.CodeArgvCaseMismatch,
					fmt.Sprintf(yerrors.Messages[yerrors.CodeArgvCaseMismatch], arg))
				return exitcode.Usage
			}
			// Check for flag abbreviation: a proper prefix of a known flag.
			if isFlagAbbreviation(lower) {
				emitError(stderr, yerrors.CodeArgvAbbreviation,
					fmt.Sprintf(yerrors.Messages[yerrors.CodeArgvAbbreviation], arg))
				return exitcode.Usage
			}
			emitError(stderr, yerrors.CodeArgvUnknownFlag,
				fmt.Sprintf(yerrors.Messages[yerrors.CodeArgvUnknownFlag], arg))
			return exitcode.Usage
		}

		// This is a positional arg. First positional is the subcommand.
		if subcommandIdx == -1 {
			subcommandIdx = i
			// Don't break; continue checking remaining preDash for invalid
			// flags/args (spec says validate ALL args).
			continue
		}

		// Subsequent positionals after the subcommand in preDash are subcommand
		// args passed through without flag validation.
	}

	// Also validate postDash args for empty string and bare "-".
	for _, arg := range postDash {
		if arg == "" {
			emitError(stderr, yerrors.CodeArgvEmptyArgument,
				yerrors.Messages[yerrors.CodeArgvEmptyArgument])
			return exitcode.Usage
		}
		if arg == "-" {
			emitError(stderr, yerrors.CodeArgvStdinDash,
				yerrors.Messages[yerrors.CodeArgvStdinDash])
			return exitcode.Usage
		}
	}

	// e. Validate the subcommand and dispatch.
	if subcommandIdx >= 0 {
		sub := preDash[subcommandIdx]
		remaining := append(preDash[subcommandIdx+1:], postDash...)
		return validateAndDispatch(sub, remaining, stdout, stderr, isTTY, termWidth)
	}

	// No subcommand was found in preDash. If postDash has args, the first
	// is the subcommand candidate (everything after "--" is positional).
	if len(postDash) > 0 {
		sub := postDash[0]
		return validateAndDispatch(sub, postDash[1:], stdout, stderr, isTTY, termWidth)
	}

	// f. No subcommand found at all.
	emitError(stderr, yerrors.CodeArgvNoSubcommand,
		yerrors.Messages[yerrors.CodeArgvNoSubcommand])
	return exitcode.Usage
}

// validateAndDispatch validates the subcommand name and dispatches.
func validateAndDispatch(sub string, remaining []string, stdout, stderr io.Writer, isTTY bool, termWidth int) int {
	if isKnownSubcommand(sub) {
		return dispatchSubcommand(sub, remaining, stdout, stderr, isTTY, termWidth)
	}

	// Case mismatch: lowercased matches a known subcommand but not exact.
	if isKnownSubcommandCaseInsensitive(sub) {
		emitError(stderr, yerrors.CodeArgvCaseMismatch,
			fmt.Sprintf(yerrors.Messages[yerrors.CodeArgvCaseMismatch], sub))
		return exitcode.Usage
	}

	// Abbreviation: is a prefix of exactly one known subcommand.
	if isSubcommandAbbreviation(sub) {
		emitError(stderr, yerrors.CodeArgvAbbreviation,
			fmt.Sprintf(yerrors.Messages[yerrors.CodeArgvAbbreviation], sub))
		return exitcode.Usage
	}

	// Unknown subcommand.
	emitError(stderr, yerrors.CodeArgvUnknownSubcommand,
		fmt.Sprintf(yerrors.Messages[yerrors.CodeArgvUnknownSubcommand], sub))
	return exitcode.Usage
}

// dispatchSubcommand calls the appropriate subcommand handler.
func dispatchSubcommand(sub string, remaining []string, stdout, stderr io.Writer, isTTY bool, termWidth int) int {
	switch sub {
	case "validate":
		return validate.Run(remaining, stdout, stderr)
	case "list":
		return list.Run(remaining, stdout, stderr, isTTY, termWidth)
	case "query":
		return query.Run(remaining, stdout, stderr)
	default:
		emitError(stderr, yerrors.CodeInternalUncaught,
			fmt.Sprintf(yerrors.Messages[yerrors.CodeInternalUncaught], "unreachable dispatch"))
		return exitcode.Processing
	}
}

// emitError writes an error line to stderr.
func emitError(stderr io.Writer, code, message string) {
	fmt.Fprint(stderr, yerrors.FormatError("yass", 0, code, message))
}

// isShortFlag returns true for single-dash flags like -v, -h, -abc.
func isShortFlag(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	// Must be single dash followed by alphanumeric characters.
	if arg[1] == '-' {
		return false
	}
	for _, c := range arg[1:] {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// isKnownSubcommand checks if a name matches a known subcommand exactly.
func isKnownSubcommand(name string) bool {
	for _, s := range knownSubcommands {
		if name == s {
			return true
		}
	}
	return false
}

// isKnownSubcommandCaseInsensitive checks if a name matches a known subcommand
// when lowercased, but not by exact match.
func isKnownSubcommandCaseInsensitive(name string) bool {
	lower := strings.ToLower(name)
	for _, s := range knownSubcommands {
		if lower == s && name != s {
			return true
		}
	}
	return false
}

// isSubcommandAbbreviation checks if a name is a prefix of any known subcommand.
func isSubcommandAbbreviation(name string) bool {
	lower := strings.ToLower(name)
	for _, s := range knownSubcommands {
		if strings.HasPrefix(s, lower) && lower != s {
			return true
		}
	}
	return false
}

// knownFlags lists the valid long flag names.
var knownFlags = []string{"--help", "--version"}

// isFlagAbbreviation checks if a lowercased flag is a proper prefix of any known flag.
// The flag must start with "--" and have at least 3 characters (i.e., "--" + at least one letter).
func isFlagAbbreviation(lower string) bool {
	if len(lower) < 3 {
		return false
	}
	for _, f := range knownFlags {
		if strings.HasPrefix(f, lower) && lower != f {
			return true
		}
	}
	return false
}

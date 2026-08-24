package yass

import (
	"strings"
	"unicode/utf8"
)

// Subcommands is the recognized set, in synopsis order.
var Subcommands = []string{"overview", "root", "query", "list", "find", "refs", "validate", "lint", "docs"}

// DefaultSubcommand is what an argument vector naming no subcommand yields.
// Such a vector is well formed rather than malformed: a reader who does not yet
// know how to drive the program is oriented instead of rejected.
const DefaultSubcommand = "overview"

var subcommandList = strings.Join(Subcommands, ", ")

// Invocation is what argument parsing yields: the starting directory, the
// subcommand token, that subcommand's parsed options, and its operands in
// argument order.
type Invocation struct {
	StartDir   string
	Subcommand string
	Operands   []string

	Raw     bool
	Filters []string
	SlotOpt string
	HasSlot bool
	In      bool
	Out     bool

	Help    bool
	Version bool
}

// optionSpec describes one subcommand option the synopsis lists.
type optionSpec struct {
	takesValue bool
}

var subcommandOptions = map[string]map[string]optionSpec{
	"overview": {},
	"root":     {},
	"query":    {"--raw": {}},
	"list":     {"--filter": {takesValue: true}},
	"find":     {"--slot": {takesValue: true}},
	"refs":     {"--in": {}, "--out": {}},
	"validate": {},
	"lint":     {},
	"docs":     {},
}

// operandRange fixes how many operands each subcommand requires and accepts;
// a max of -1 means unbounded.
var operandRange = map[string][2]int{
	"overview": {0, 0},
	"root":     {0, 1},
	"query":    {1, -1},
	"list":     {0, -1},
	"find":     {1, 1},
	"refs":     {1, 1},
	"validate": {0, -1},
	"lint":     {0, -1},
	"docs":     {0, 1},
}

func unknownOption(opt string) error {
	return fail(ExitUsage, "yass.args.unknown_option", "", 0, "unknown option "+opt)
}

// globalPhase is the result of scanning the arguments before the subcommand
// token, where the global options are the only options accepted.
type globalPhase struct {
	err          error // the first malformed-vector finding, if any
	rest         []string
	haveSub      bool
	endOfOptions bool
}

// ParseArgs turns the argument vector after the program name into an
// Invocation, and owns every way that vector can be malformed.
func ParseArgs(args []string) (*Invocation, error) {
	inv := &Invocation{StartDir: "."}
	for _, a := range args {
		if !utf8.ValidString(a) {
			// Reject the vector rather than replace or drop the bytes.
			return nil, unknownOption("(argument is not valid utf-8)")
		}
	}

	g := scanGlobal(inv, args)

	// --help takes precedence over every other option and over the subcommand
	// token; a malformed vector is reported next; --version last.
	if inv.Help {
		return inv, nil
	}
	if g.err != nil {
		return nil, g.err
	}
	if inv.Version {
		return inv, nil
	}
	if !g.haveSub {
		// No subcommand token: yield the default one, carrying no operand.
		inv.Subcommand = DefaultSubcommand
		return inv, nil
	}
	if !contains(Subcommands, inv.Subcommand) {
		return nil, fail(ExitUsage, "yass.args.unknown_subcommand", "", 0,
			"unknown subcommand "+inv.Subcommand+", expected one of "+subcommandList)
	}
	if g.endOfOptions {
		// Every argument after `--` is an operand, even when it begins with `-`.
		inv.Operands = append(inv.Operands, g.rest...)
		return inv, checkOperands(inv)
	}
	if err := parseSubcommandArgs(inv, g.rest); err != nil {
		return nil, err
	}
	return inv, nil
}

// scanGlobal consumes the global options and the subcommand token.
func scanGlobal(inv *Invocation, args []string) globalPhase {
	var g globalPhase
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			g.endOfOptions = true
			if i+1 < len(args) {
				inv.Subcommand = args[i+1]
				g.haveSub = true
				g.rest = args[i+2:]
			}
			return g
		case a == "--help" || a == "-h":
			inv.Help = true
		case a == "--version" || a == "-V":
			inv.Version = true
		case a == "-C":
			if i+1 >= len(args) {
				if g.err == nil {
					g.err = fail(ExitUsage, "yass.args.missing_value", "", 0, "missing value for option -C")
				}
				return g
			}
			i++
			inv.StartDir = args[i]
		case strings.HasPrefix(a, "-C="):
			inv.StartDir = a[len("-C="):]
		case strings.HasPrefix(a, "-") && a != "-":
			if g.err == nil {
				g.err = unknownOption(a)
			}
		default:
			inv.Subcommand = a
			g.haveSub = true
			g.rest = args[i+1:]
			return g
		}
	}
	return g
}

func parseSubcommandArgs(inv *Invocation, args []string) error {
	opts := subcommandOptions[inv.Subcommand]
	endOfOptions := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !endOfOptions {
			if a == "--" {
				endOfOptions = true
				continue
			}
			if strings.HasPrefix(a, "-") && a != "-" {
				name, value, hasValue := a, "", false
				if eq := strings.Index(a, "="); eq >= 0 {
					name, value, hasValue = a[:eq], a[eq+1:], true
				}
				spec, known := opts[name]
				if !known {
					return unknownOption(name)
				}
				if spec.takesValue {
					if !hasValue {
						if i+1 >= len(args) {
							return fail(ExitUsage, "yass.args.missing_value", "", 0, "missing value for option "+name)
						}
						i++
						value = args[i]
					}
				} else if hasValue {
					return unknownOption(name)
				}
				applyOption(inv, name, value)
				continue
			}
		}
		inv.Operands = append(inv.Operands, a)
	}
	return checkOperands(inv)
}

func applyOption(inv *Invocation, name, value string) {
	switch name {
	case "--raw":
		inv.Raw = true
	case "--filter":
		inv.Filters = append(inv.Filters, value)
	case "--slot":
		inv.SlotOpt = value
		inv.HasSlot = true
	case "--in":
		inv.In = true
	case "--out":
		inv.Out = true
	}
}

func checkOperands(inv *Invocation) error {
	for _, op := range inv.Operands {
		if op == "" {
			return fail(ExitUsage, "yass.args.empty", "", 0, "operand is the empty string")
		}
	}
	r := operandRange[inv.Subcommand]
	if len(inv.Operands) < r[0] {
		return fail(ExitUsage, "yass.args.missing_operand", "", 0,
			"missing operand for subcommand "+inv.Subcommand)
	}
	if r[1] >= 0 && len(inv.Operands) > r[1] {
		return fail(ExitUsage, "yass.args.extra_operand", "", 0,
			"unexpected operand "+inv.Operands[r[1]])
	}
	return nil
}

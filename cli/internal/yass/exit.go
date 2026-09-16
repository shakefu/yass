package yass

// Exit statuses, per root@ExitPolicy. Exactly one is returned per invocation
// and, when more than one condition applies, the higher number wins.
const (
	ExitOK          = 0 // the command completed and reported no finding
	ExitFindings    = 1 // a finding about the spec set
	ExitUsage       = 2 // the argument vector is malformed
	ExitUnresolved  = 3 // well formed but names something absent
	ExitNoRoot      = 4 // no root.yass.yaml at or above the starting path
	ExitEnvironment = 5 // a path or stream failed, or an internal guarantee did not hold
)

// The yass language version this program implements is fixed at v1.
const (
	programName     = "yass"
	languageVersion = "v1"
)

// programVersion is the version reported by --version. It is a var, not a
// const, so release builds can stamp it: script/build passes -ldflags
// "-X github.com/shakefu/yass/cli/internal/yass.programVersion=<semver>".
// Builds without the stamp report "dev".
var programVersion = "dev"

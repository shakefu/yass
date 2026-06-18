package exitcode

// Exit codes used by the yass CLI.
const (
	Success = 0   // Run completed without error
	Processing = 1   // A validation or processing rule was violated
	Usage      = 2   // An argv-parse or file-input failure
	SigInt     = 130 // Process received SIGINT
	SigTerm    = 143 // Process received SIGTERM
)

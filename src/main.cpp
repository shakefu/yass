// M9 — real CLI entry point.
//
// main() wires the process up to run_dispatch (src/dispatch.hpp):
//   - installs signal handlers (cli.Dispatch INVARIANT / cli.ExitCode):
//       SIGINT  -> flush, exit 130 (no summary line)
//       SIGTERM -> flush, exit 143 (no summary line)
//       SIGPIPE -> exit 0 cleanly, writing nothing further
//   - sets stdout line-buffered regardless of TTY (cli.Dispatch INVARIANT)
//   - computes the list terminal width:
//       not a TTY                              -> 0 (run_list emits full rows)
//       a TTY, $COLUMNS set and parses to > 0  -> that value
//       a TTY, else ioctl(TIOCGWINSZ) ws_col>0 -> ws_col
//       a TTY, else                            -> 80
//   - calls run_dispatch(args, std::cout, std::cerr, width) and returns it.
//
// run_dispatch never calls std::exit and returns only 0/1/2; the signal paths
// supply 130/143; SIGPIPE supplies 0. No other exit code is reachable, per
// cli.ExitCode.

#include <csignal>
#include <cstdio>
#include <cstdlib>
#include <iostream>
#include <string>
#include <vector>

#include <unistd.h>

#ifdef __has_include
#if __has_include(<sys/ioctl.h>)
#include <sys/ioctl.h>
#endif
#endif

#include "diag.hpp"
#include "dispatch.hpp"

namespace {

// SIGINT / SIGTERM handler: flush any pending output, then exit with the mapped
// code WITHOUT writing a summary line. fflush(nullptr) flushes all C stdio
// buffers (std::cout is kept synced with stdio); _exit avoids re-entering
// non-async-signal-safe C++ destructors. The 130/143 mapping itself lives in
// yass::dispatch::signal_exit_code so it can be unit-tested.
extern "C" void on_terminating_signal(int sig) {
    std::fflush(nullptr);
    _exit(yass::dispatch::signal_exit_code(sig));
}

// SIGPIPE handler: the reader closed early. Exit 0 cleanly and write nothing
// further (cli.Dispatch INVARIANT: handle SIGPIPE by exiting 0 cleanly).
extern "C" void on_sigpipe(int) { _exit(yass::diag::ExitCode::SUCCESS); }

void install_signal_handlers() {
    struct sigaction sa;
    sa.sa_flags = 0;
    sigemptyset(&sa.sa_mask);

    sa.sa_handler = on_terminating_signal;
    sigaction(SIGINT, &sa, nullptr);
    sigaction(SIGTERM, &sa, nullptr);

    sa.sa_handler = on_sigpipe;
    sigaction(SIGPIPE, &sa, nullptr);
}

// Determine the terminal width run_list should use. Returns 0 when stdout is not
// a TTY (full, untruncated rows). Otherwise prefers $COLUMNS (when it parses to
// a positive integer), then the OS window size, then a fallback of 80.
int compute_list_tty_width() {
    if (!::isatty(STDOUT_FILENO)) {
        return 0;
    }
    if (const char* cols = std::getenv("COLUMNS")) {
        // Parse strictly: a clean positive integer wins; anything else falls
        // through to the OS query.
        char* end = nullptr;
        long v = std::strtol(cols, &end, 10);
        if (end != cols && end != nullptr && *end == '\0' && v > 0) {
            return static_cast<int>(v);
        }
    }
#ifdef TIOCGWINSZ
    struct winsize ws;
    if (::ioctl(STDOUT_FILENO, TIOCGWINSZ, &ws) == 0 && ws.ws_col > 0) {
        return static_cast<int>(ws.ws_col);
    }
#endif
    return 80;
}

}  // namespace

int main(int argc, char** argv) {
    install_signal_handlers();

    // Line-buffer stdout regardless of TTY (cli.Dispatch INVARIANT). Keep
    // std::cout synced with C stdio so the signal handler's fflush(nullptr)
    // reaches everything written so far.
    static char stdout_buf[BUFSIZ];
    std::setvbuf(stdout, stdout_buf, _IOLBF, sizeof(stdout_buf));

    int width = compute_list_tty_width();

    std::vector<std::string> args;
    args.reserve(argc > 0 ? static_cast<std::size_t>(argc - 1) : 0);
    for (int i = 1; i < argc; ++i) {
        args.emplace_back(argv[i]);
    }

    int code = yass::dispatch::run_dispatch(args, std::cout, std::cerr, width);
    std::cout.flush();
    return code;
}

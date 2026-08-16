#pragma once

// PTY harness for the cli.list TTY-truncation differential tests.
//
// The reference `yass list` truncates the description ONLY when stdout is a
// terminal, and reads the terminal width from COLUMNS (or the OS). To exercise
// that path against the reference we must run it with stdout attached to a
// pseudo-terminal, not a pipe (the proc.hpp harness gives a pipe, so the
// reference takes its non-TTY branch there). This helper allocates a pty via
// forkpty(), runs the reference in the child with COLUMNS=W in the environment,
// and captures everything the child writes to the pty master.
//
// IMPORTANT — pty line discipline: a pty master in cooked mode translates each
// '\n' the child writes into "\r\n" (ONLCR). The reference emits rows terminated
// by a single '\n'; under the pty those arrive as "\r\n". run_under_pty() strips
// the inserted '\r' before each '\n' so the captured bytes match the in-process
// runner's stdout (which writes to a std::ostringstream, no line discipline). We
// set the slave terminal width via TIOCSWINSZ as well, so the reference's OS
// width query (the COLUMNS-unset fallback) would also see W — but every caller
// also sets COLUMNS=W, which the spec prefers.
//
// On a host where a pty cannot be allocated (sandbox without /dev/ptmx, etc.),
// run_under_pty() returns ok=false and the caller SKIPS the differential check
// gracefully (a doctest MESSAGE), rather than failing.

#include <cerrno>
#include <csignal>
#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

#include <fcntl.h>
#include <sys/ioctl.h>
#include <sys/wait.h>
#include <termios.h>
#include <unistd.h>

#if defined(__APPLE__)
#include <util.h>  // forkpty / openpty on macOS
#else
#include <pty.h>  // forkpty / openpty on glibc
#endif

extern char** environ;

namespace yass::test {

struct PtyResult {
    bool ok = false;     // false => pty could not be allocated; SKIP the check.
    int exit_code = 0;
    std::string out;     // child's pty output with inserted '\r' before '\n' removed.
    std::string raw;     // child's pty output verbatim (for debugging).
};

namespace pty_detail {

// Strip a single '\r' immediately preceding each '\n' (undo ONLCR), leaving any
// other carriage returns intact.
inline std::string strip_cr_before_lf(const std::string& in) {
    std::string out;
    out.reserve(in.size());
    for (std::size_t i = 0; i < in.size(); ++i) {
        if (in[i] == '\r' && i + 1 < in.size() && in[i + 1] == '\n') {
            continue;  // drop the carriage return; the '\n' is emitted next iter.
        }
        out.push_back(in[i]);
    }
    return out;
}

}  // namespace pty_detail

// Run `argv` (argv[0] = program path) with stdout (and stdin/stderr) attached to
// a pty whose window is `width` columns by `rows` rows, in working directory
// `cwd` (empty = inherit), with extra environment variables `env` merged onto the
// inherited environment (typically {{"COLUMNS", std::to_string(width)}}). Returns
// the child's exit code and the bytes it wrote to the pty.
inline PtyResult run_under_pty(const std::vector<std::string>& argv, const std::string& cwd,
                               int width, int rows = 24,
                               const std::vector<std::pair<std::string, std::string>>& env = {}) {
    PtyResult result;
    if (argv.empty()) return result;

    struct winsize ws;
    std::memset(&ws, 0, sizeof(ws));
    ws.ws_col = static_cast<unsigned short>(width);
    ws.ws_row = static_cast<unsigned short>(rows);

    int master_fd = -1;
    pid_t pid = ::forkpty(&master_fd, nullptr, nullptr, &ws);
    if (pid < 0) {
        // No pty available in this environment -> caller SKIPS.
        result.ok = false;
        return result;
    }

    if (pid == 0) {
        // -------- child: stdin/stdout/stderr are already the pty slave --------
        if (!cwd.empty()) {
            if (::chdir(cwd.c_str()) != 0) ::_exit(127);
        }

        // Merge extra env onto the inherited environment (override matching keys).
        std::vector<std::string> env_storage;
        for (char** e = environ; e && *e; ++e) {
            bool overridden = false;
            const char* eq = std::strchr(*e, '=');
            if (eq) {
                std::size_t key_len = static_cast<std::size_t>(eq - *e);
                for (const auto& kv : env) {
                    if (kv.first.size() == key_len &&
                        std::strncmp(*e, kv.first.c_str(), key_len) == 0) {
                        overridden = true;
                        break;
                    }
                }
            }
            if (!overridden) env_storage.emplace_back(*e);
        }
        for (const auto& kv : env) env_storage.emplace_back(kv.first + "=" + kv.second);

        std::vector<char*> envp;
        envp.reserve(env_storage.size() + 1);
        for (auto& s : env_storage) envp.push_back(s.data());
        envp.push_back(nullptr);

        std::vector<std::string> arg_storage(argv.begin(), argv.end());
        std::vector<char*> cargv;
        cargv.reserve(arg_storage.size() + 1);
        for (auto& a : arg_storage) cargv.push_back(a.data());
        cargv.push_back(nullptr);

        ::execve(cargv[0], cargv.data(), envp.data());
        ::_exit(127);  // execve only returns on error.
    }

    // -------- parent: drain the pty master until the child exits/EOF --------
    char buf[65536];
    for (;;) {
        ssize_t n = ::read(master_fd, buf, sizeof(buf));
        if (n > 0) {
            result.raw.append(buf, static_cast<std::size_t>(n));
            continue;
        }
        if (n == 0) break;  // EOF: slave fully closed.
        if (errno == EINTR) continue;
        // On Linux, read() on the master after the child exits yields EIO; treat
        // it as EOF rather than an error.
        break;
    }
    ::close(master_fd);

    int status = 0;
    pid_t w;
    do {
        w = ::waitpid(pid, &status, 0);
    } while (w < 0 && errno == EINTR);
    if (WIFEXITED(status)) {
        result.exit_code = WEXITSTATUS(status);
    } else if (WIFSIGNALED(status)) {
        result.exit_code = 128 + WTERMSIG(status);
    }

    result.out = pty_detail::strip_cr_before_lf(result.raw);
    result.ok = true;
    return result;
}

}  // namespace yass::test

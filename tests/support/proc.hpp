#pragma once

#include <cerrno>
#include <csignal>
#include <cstring>
#include <stdexcept>
#include <string>
#include <utility>
#include <vector>

#include <fcntl.h>
#include <poll.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

extern char** environ;

namespace yass::test {

struct ProcResult {
    int exit_code = 0;
    std::string out;
    std::string err;
    bool signaled = false;
    int signal = 0;
};

namespace detail {

inline void set_nonblocking(int fd) {
    int flags = ::fcntl(fd, F_GETFL, 0);
    if (flags == -1) {
        flags = 0;
    }
    ::fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

// Drain a non-blocking fd into `sink` until it would block or hits EOF.
// Returns false once EOF is observed (fd should be closed by caller).
inline bool drain_fd(int fd, std::string& sink, bool& eof) {
    char buf[65536];
    for (;;) {
        ssize_t n = ::read(fd, buf, sizeof(buf));
        if (n > 0) {
            sink.append(buf, static_cast<size_t>(n));
            continue;
        }
        if (n == 0) {
            eof = true;
            return false;
        }
        // n < 0
        if (errno == EAGAIN || errno == EWOULDBLOCK) {
            return true;  // no more data right now
        }
        if (errno == EINTR) {
            continue;
        }
        // Unexpected error: treat as EOF to avoid spinning.
        eof = true;
        return false;
    }
}

}  // namespace detail

// Run `argv` (argv[0] = program path) with given cwd (empty = inherit),
// optional extra env vars merged onto the inherited environment, and optional
// stdin bytes. Captures stdout and stderr SEPARATELY and fully without
// deadlocking on large output. Returns exit status / signal information.
inline ProcResult run(const std::vector<std::string>& argv,
                      const std::string& cwd = "",
                      const std::vector<std::pair<std::string, std::string>>& env = {},
                      const std::string& stdin_data = "") {
    if (argv.empty()) {
        throw std::invalid_argument("yass::test::run: argv must not be empty");
    }

    int in_pipe[2];   // parent writes child's stdin: [0]=read(child), [1]=write(parent)
    int out_pipe[2];  // child writes stdout:        [0]=read(parent), [1]=write(child)
    int err_pipe[2];  // child writes stderr:        [0]=read(parent), [1]=write(child)

    if (::pipe(in_pipe) != 0) {
        throw std::runtime_error(std::string("pipe(stdin) failed: ") + std::strerror(errno));
    }
    if (::pipe(out_pipe) != 0) {
        ::close(in_pipe[0]);
        ::close(in_pipe[1]);
        throw std::runtime_error(std::string("pipe(stdout) failed: ") + std::strerror(errno));
    }
    if (::pipe(err_pipe) != 0) {
        ::close(in_pipe[0]);
        ::close(in_pipe[1]);
        ::close(out_pipe[0]);
        ::close(out_pipe[1]);
        throw std::runtime_error(std::string("pipe(stderr) failed: ") + std::strerror(errno));
    }

    pid_t pid = ::fork();
    if (pid < 0) {
        for (int fd : {in_pipe[0], in_pipe[1], out_pipe[0], out_pipe[1], err_pipe[0], err_pipe[1]}) {
            ::close(fd);
        }
        throw std::runtime_error(std::string("fork failed: ") + std::strerror(errno));
    }

    if (pid == 0) {
        // -------- child --------
        ::dup2(in_pipe[0], STDIN_FILENO);
        ::dup2(out_pipe[1], STDOUT_FILENO);
        ::dup2(err_pipe[1], STDERR_FILENO);

        for (int fd : {in_pipe[0], in_pipe[1], out_pipe[0], out_pipe[1], err_pipe[0], err_pipe[1]}) {
            ::close(fd);
        }

        if (!cwd.empty()) {
            if (::chdir(cwd.c_str()) != 0) {
                ::_exit(127);
            }
        }

        // Merge extra env onto inherited environ.
        std::vector<std::string> env_storage;
        std::vector<char*> envp;
        for (char** e = environ; e && *e; ++e) {
            // Skip inherited vars that are being overridden.
            bool overridden = false;
            const char* eq = std::strchr(*e, '=');
            if (eq) {
                size_t key_len = static_cast<size_t>(eq - *e);
                for (const auto& kv : env) {
                    if (kv.first.size() == key_len &&
                        std::strncmp(*e, kv.first.c_str(), key_len) == 0) {
                        overridden = true;
                        break;
                    }
                }
            }
            if (!overridden) {
                env_storage.emplace_back(*e);
            }
        }
        for (const auto& kv : env) {
            env_storage.emplace_back(kv.first + "=" + kv.second);
        }
        envp.reserve(env_storage.size() + 1);
        for (auto& s : env_storage) {
            envp.push_back(s.data());
        }
        envp.push_back(nullptr);

        std::vector<std::string> arg_storage(argv.begin(), argv.end());
        std::vector<char*> cargv;
        cargv.reserve(arg_storage.size() + 1);
        for (auto& a : arg_storage) {
            cargv.push_back(a.data());
        }
        cargv.push_back(nullptr);

        ::execve(cargv[0], cargv.data(), envp.data());
        // execve only returns on error.
        ::_exit(127);
    }

    // -------- parent --------
    ::close(in_pipe[0]);
    ::close(out_pipe[1]);
    ::close(err_pipe[1]);

    int stdin_fd = in_pipe[1];
    int stdout_fd = out_pipe[0];
    int stderr_fd = err_pipe[0];

    detail::set_nonblocking(stdout_fd);
    detail::set_nonblocking(stderr_fd);
    detail::set_nonblocking(stdin_fd);

    ProcResult result;

    size_t stdin_written = 0;
    bool stdin_done = stdin_data.empty();
    if (stdin_done) {
        ::close(stdin_fd);
        stdin_fd = -1;
    }

    bool out_eof = false;
    bool err_eof = false;

    while (!out_eof || !err_eof || !stdin_done) {
        struct pollfd fds[3];
        int nfds = 0;

        int out_idx = -1, err_idx = -1, in_idx = -1;
        if (!out_eof) {
            fds[nfds].fd = stdout_fd;
            fds[nfds].events = POLLIN;
            fds[nfds].revents = 0;
            out_idx = nfds++;
        }
        if (!err_eof) {
            fds[nfds].fd = stderr_fd;
            fds[nfds].events = POLLIN;
            fds[nfds].revents = 0;
            err_idx = nfds++;
        }
        if (!stdin_done) {
            fds[nfds].fd = stdin_fd;
            fds[nfds].events = POLLOUT;
            fds[nfds].revents = 0;
            in_idx = nfds++;
        }

        if (nfds == 0) {
            break;
        }

        int pr = ::poll(fds, static_cast<nfds_t>(nfds), -1);
        if (pr < 0) {
            if (errno == EINTR) {
                continue;
            }
            break;
        }

        if (out_idx >= 0 && (fds[out_idx].revents & (POLLIN | POLLHUP | POLLERR))) {
            if (!detail::drain_fd(stdout_fd, result.out, out_eof)) {
                ::close(stdout_fd);
                out_eof = true;
            }
        }
        if (err_idx >= 0 && (fds[err_idx].revents & (POLLIN | POLLHUP | POLLERR))) {
            if (!detail::drain_fd(stderr_fd, result.err, err_eof)) {
                ::close(stderr_fd);
                err_eof = true;
            }
        }
        if (in_idx >= 0 && (fds[in_idx].revents & (POLLOUT | POLLHUP | POLLERR))) {
            size_t remaining = stdin_data.size() - stdin_written;
            ssize_t wn = ::write(stdin_fd, stdin_data.data() + stdin_written, remaining);
            if (wn > 0) {
                stdin_written += static_cast<size_t>(wn);
            } else if (wn < 0 && errno != EAGAIN && errno != EWOULDBLOCK && errno != EINTR) {
                // Pipe broke (child closed stdin); stop writing.
                stdin_written = stdin_data.size();
            }
            if (stdin_written >= stdin_data.size()) {
                ::close(stdin_fd);
                stdin_fd = -1;
                stdin_done = true;
            }
        }
    }

    if (stdin_fd != -1) {
        ::close(stdin_fd);
    }

    int status = 0;
    pid_t w;
    do {
        w = ::waitpid(pid, &status, 0);
    } while (w < 0 && errno == EINTR);

    if (WIFSIGNALED(status)) {
        result.signaled = true;
        result.signal = WTERMSIG(status);
        result.exit_code = 128 + result.signal;
    } else if (WIFEXITED(status)) {
        result.exit_code = WEXITSTATUS(status);
    }

    return result;
}

}  // namespace yass::test

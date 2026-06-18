#pragma once

// RAII temporary-directory tree builder for filesystem tests (M4 cli.fs).
//
// Creates a unique temporary directory under $TMPDIR (or /tmp) on construction
// and recursively removes it on destruction. Provides small convenience helpers
// for materializing files, directories, and symlinks inside the tree so tests
// can build the exact on-disk shapes the cli.shared specs describe.
//
// The tree's root is an ABSOLUTE, lexically-canonical path. On macOS $TMPDIR is
// usually under /var/folders which is itself reached through the /var -> private
// symlink; tests that need the *real* (symlink-resolved) root for "under cwd"
// reasoning should use root_real(). For most tests the lexical root() is what
// you hand to the code under test.

#include <atomic>
#include <chrono>
#include <filesystem>
#include <fstream>
#include <string>
#include <system_error>

#if !defined(_WIN32)
#include <unistd.h>
#endif

namespace yass::test {

class TmpTree {
   public:
    TmpTree() {
        namespace fs = std::filesystem;
        std::error_code ec;
        fs::path base = fs::temp_directory_path(ec);
        if (ec || base.empty()) {
            base = fs::path("/tmp");
        }
        // Make a unique name. Combine pid, a process-lifetime counter, and a
        // clock tick so concurrent test cases never collide.
        static std::atomic<unsigned long long> counter{0};
        unsigned long long n = counter.fetch_add(1);
        auto now = static_cast<unsigned long long>(
            std::chrono::steady_clock::now().time_since_epoch().count());
        std::string name = "yass_fs_test_" + std::to_string(
                                                  static_cast<unsigned long long>(
#if defined(_WIN32)
                                                      0
#else
                                                      ::getpid()
#endif
                                                      )) +
                           "_" + std::to_string(n) + "_" + std::to_string(now);
        root_ = (base / name).lexically_normal();
        fs::create_directories(root_, ec);
    }

    ~TmpTree() {
        std::error_code ec;
        std::filesystem::remove_all(root_, ec);
    }

    TmpTree(const TmpTree&) = delete;
    TmpTree& operator=(const TmpTree&) = delete;

    // The lexical (possibly symlinked-prefix) absolute root.
    const std::filesystem::path& root() const { return root_; }

    // The realpath-resolved root (symlinks in the prefix collapsed). Useful when
    // a test must reason about whether a discovered path is lexically "under" a
    // cwd that the code under test also canonicalizes the same way.
    std::filesystem::path root_real() const {
        std::error_code ec;
        auto r = std::filesystem::canonical(root_, ec);
        return ec ? root_ : r;
    }

    // Create the directory (and parents) at root()/rel. Returns the abs path.
    std::filesystem::path mkdir(const std::string& rel) const {
        std::filesystem::path p = root_ / rel;
        std::error_code ec;
        std::filesystem::create_directories(p, ec);
        return p;
    }

    // Create a file at root()/rel with the given content (parents created).
    // Returns the abs path.
    std::filesystem::path write(const std::string& rel,
                                const std::string& content = "x") const {
        std::filesystem::path p = root_ / rel;
        std::error_code ec;
        std::filesystem::create_directories(p.parent_path(), ec);
        std::ofstream os(p, std::ios::binary | std::ios::trunc);
        os << content;
        return p;
    }

    // Create a symlink at root()/rel pointing at `target` (parents created).
    // Returns the abs path of the link. `target` may be relative or absolute.
    std::filesystem::path symlink(const std::string& rel,
                                  const std::filesystem::path& target) const {
        std::filesystem::path p = root_ / rel;
        std::error_code ec;
        std::filesystem::create_directories(p.parent_path(), ec);
        // Decide file vs dir symlink kind from the target if it exists; default
        // to a plain symlink which works for both on POSIX.
        std::filesystem::create_symlink(target, p, ec);
        return p;
    }

   private:
    std::filesystem::path root_;
};

}  // namespace yass::test

/*
 * test_list.c - Tests for the list subcommand
 */
#include "tinytest.h"
#include "../src/list.h"
#include "../src/error.h"

#include <dirent.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>
#include <util.h>
#include <fcntl.h>
#include <termios.h>

/* ---- helpers ---- */

static char saved_cwd[4096];

static void save_cwd(void)
{
    getcwd(saved_cwd, sizeof(saved_cwd));
}

static void restore_cwd(void)
{
    if (saved_cwd[0])
        chdir(saved_cwd);
}

static void rmtree(const char *path)
{
    DIR *dp = opendir(path);
    if (!dp)
        return;
    struct dirent *ent;
    while ((ent = readdir(dp)) != NULL) {
        if (strcmp(ent->d_name, ".") == 0 || strcmp(ent->d_name, "..") == 0)
            continue;
        char child[4096];
        snprintf(child, sizeof(child), "%s/%s", path, ent->d_name);
        struct stat st;
        if (lstat(child, &st) == 0) {
            if (S_ISDIR(st.st_mode))
                rmtree(child);
            else
                unlink(child);
        }
    }
    closedir(dp);
    rmdir(path);
}

static void mkdirp(const char *path)
{
    char tmp[4096];
    snprintf(tmp, sizeof(tmp), "%s", path);
    for (char *p = tmp + 1; *p; p++) {
        if (*p == '/') {
            *p = '\0';
            mkdir(tmp, 0755);
            *p = '/';
        }
    }
    mkdir(tmp, 0755);
}

static void write_file(const char *path, const char *content)
{
    FILE *f = fopen(path, "w");
    if (f) {
        fputs(content, f);
        fclose(f);
    }
}

/*
 * Capture stdout from cmd_list into a buffer.
 * Returns the exit code. Sets *out to a malloc'd string (caller frees).
 */
static int capture_list(int argc, char **argv, char **out)
{
    fflush(stdout);

    /* Create a temp file to capture stdout */
    char tmpf[] = "/tmp/yass_test_list_stdout_XXXXXX";
    int fd = mkstemp(tmpf);
    if (fd < 0) {
        *out = strdup("");
        return -1;
    }

    int saved_stdout = dup(STDOUT_FILENO);
    dup2(fd, STDOUT_FILENO);
    close(fd);

    int rc = cmd_list(argc, argv);

    fflush(stdout);
    dup2(saved_stdout, STDOUT_FILENO);
    close(saved_stdout);

    /* Read captured output */
    FILE *f = fopen(tmpf, "r");
    if (!f) {
        *out = strdup("");
        unlink(tmpf);
        return rc;
    }
    fseek(f, 0, SEEK_END);
    long len = ftell(f);
    fseek(f, 0, SEEK_SET);
    char *buf = malloc((size_t)len + 1);
    if (buf) {
        size_t nread = fread(buf, 1, (size_t)len, f);
        buf[nread] = '\0';
    } else {
        buf = strdup("");
    }
    fclose(f);
    unlink(tmpf);

    *out = buf;
    return rc;
}

/*
 * Capture stderr from cmd_list into a buffer.
 * Returns the exit code. Sets *out_stdout and *out_stderr to malloc'd strings.
 */
static int capture_list_both(int argc, char **argv, char **out_stdout,
                             char **out_stderr)
{
    fflush(stdout);
    fflush(stderr);

    /* Temp files for both streams */
    char tmpout[] = "/tmp/yass_test_list_out_XXXXXX";
    char tmperr[] = "/tmp/yass_test_list_err_XXXXXX";
    int fd_out = mkstemp(tmpout);
    int fd_err = mkstemp(tmperr);
    if (fd_out < 0 || fd_err < 0) {
        *out_stdout = strdup("");
        *out_stderr = strdup("");
        return -1;
    }

    int saved_stdout = dup(STDOUT_FILENO);
    int saved_stderr = dup(STDERR_FILENO);
    dup2(fd_out, STDOUT_FILENO);
    dup2(fd_err, STDERR_FILENO);
    close(fd_out);
    close(fd_err);

    int rc = cmd_list(argc, argv);

    fflush(stdout);
    fflush(stderr);
    dup2(saved_stdout, STDOUT_FILENO);
    dup2(saved_stderr, STDERR_FILENO);
    close(saved_stdout);
    close(saved_stderr);

    /* Read captured stdout */
    FILE *f = fopen(tmpout, "r");
    if (f) {
        fseek(f, 0, SEEK_END);
        long len = ftell(f);
        fseek(f, 0, SEEK_SET);
        char *buf = malloc((size_t)len + 1);
        if (buf) {
            size_t nread = fread(buf, 1, (size_t)len, f);
            buf[nread] = '\0';
        } else {
            buf = strdup("");
        }
        fclose(f);
        *out_stdout = buf;
    } else {
        *out_stdout = strdup("");
    }
    unlink(tmpout);

    /* Read captured stderr */
    f = fopen(tmperr, "r");
    if (f) {
        fseek(f, 0, SEEK_END);
        long len = ftell(f);
        fseek(f, 0, SEEK_SET);
        char *buf = malloc((size_t)len + 1);
        if (buf) {
            size_t nread = fread(buf, 1, (size_t)len, f);
            buf[nread] = '\0';
        } else {
            buf = strdup("");
        }
        fclose(f);
        *out_stderr = buf;
    } else {
        *out_stderr = strdup("");
    }
    unlink(tmperr);

    return rc;
}

/* ---- tests ---- */

/*
 * List specs from a single file -> correct tab-separated output format.
 */
TEST(list_single_file)
{
    const char *base = "/tmp/yass_test_list_single";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/my.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: A test spec file\n"
        "version: v1\n"
        "---\n"
        "spec: my.Feature\n"
        "INPUT:\n"
        "- MUST: do something\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* Output should contain the file path, tab, spec name, tab, description */
    ASSERT_STR_CONTAINS(out, "my.yass.yaml");
    ASSERT_STR_CONTAINS(out, "\tmy.Feature\t");
    ASSERT_STR_CONTAINS(out, "A test spec file");
    /* Should end with LF */
    size_t len = strlen(out);
    ASSERT(len > 0);
    ASSERT_EQ(out[len - 1], '\n');

    free(out);
    rmtree(base);
}

/*
 * List from directory -> discovers and lists specs.
 */
TEST(list_from_directory)
{
    const char *base = "/tmp/yass_test_list_dir";
    rmtree(base);
    mkdirp(base);

    char f1[4096], f2[4096];
    snprintf(f1, sizeof(f1), "%s/alpha.yass.yaml", base);
    snprintf(f2, sizeof(f2), "%s/beta.yass.yaml", base);

    write_file(f1,
        "---\n"
        "description: Alpha file\n"
        "version: v1\n"
        "---\n"
        "spec: alpha.One\n"
        "INPUT:\n"
        "- MUST: first\n"
    );
    write_file(f2,
        "---\n"
        "description: Beta file\n"
        "version: v1\n"
        "---\n"
        "spec: beta.Two\n"
        "INPUT:\n"
        "- MUST: second\n"
    );

    char *args[] = { (char *)base };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* Both specs should appear */
    ASSERT_STR_CONTAINS(out, "alpha.One");
    ASSERT_STR_CONTAINS(out, "beta.Two");
    /* alpha should come before beta (sorted by path) */
    char *pos_alpha = strstr(out, "alpha.One");
    char *pos_beta = strstr(out, "beta.Two");
    ASSERT(pos_alpha < pos_beta);

    free(out);
    rmtree(base);
}

/*
 * Empty file set -> exit 0, no output.
 */
TEST(list_no_files)
{
    const char *base = "/tmp/yass_test_list_nofiles";
    rmtree(base);
    mkdirp(base);

    /* Empty directory with no .yass.yaml files */
    save_cwd();
    chdir(base);

    /* Create a .git marker so project root is found but no specs */
    char gitdir[4096];
    snprintf(gitdir, sizeof(gitdir), "%s/.git", base);
    mkdir(gitdir, 0755);

    char *args[] = { (char *)base };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_EQ(out, "");

    free(out);
    restore_cwd();
    rmtree(base);
}

/*
 * File with only preamble (no spec docs) -> no rows emitted.
 */
TEST(list_preamble_only)
{
    const char *base = "/tmp/yass_test_list_preamble";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/preamble.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: Just a preamble\n"
        "version: v1\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* No output since no specs */
    ASSERT_STR_EQ(out, "");

    free(out);
    rmtree(base);
}

/*
 * Description normalization: whitespace collapsing.
 * Runs of whitespace (including newlines, tabs) become single space.
 * Leading/trailing whitespace stripped.
 */
TEST(list_description_whitespace)
{
    const char *base = "/tmp/yass_test_list_ws";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/ws.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: >-\n"
        "  hello\n"
        "  world\n"
        "version: v1\n"
        "---\n"
        "spec: ws.Test\n"
        "INPUT:\n"
        "- MUST: test\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* YAML >- folded scalar produces "hello world" (space-joined) */
    ASSERT_STR_CONTAINS(out, "hello world");

    free(out);
    rmtree(base);
}

/*
 * Tab-separated fields: verify exactly two tabs per output line.
 */
TEST(list_tab_separated_fields)
{
    const char *base = "/tmp/yass_test_list_tabs";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/tab.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: Some description\n"
        "version: v1\n"
        "---\n"
        "spec: tab.Check\n"
        "INPUT:\n"
        "- MUST: verify\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);

    /* Count tabs in the output line */
    int tab_count = 0;
    for (char *p = out; *p; p++) {
        if (*p == '\t')
            tab_count++;
    }
    /* Should have exactly 2 tabs (field separators) */
    ASSERT_EQ(tab_count, 2);

    /* Verify format: path<TAB>name<TAB>desc\n */
    ASSERT_STR_CONTAINS(out, "\ttab.Check\t");
    ASSERT_STR_CONTAINS(out, "Some description");

    free(out);
    rmtree(base);
}

/*
 * File with parse error -> ErrorLine to stderr, exit 1.
 */
TEST(list_parse_error)
{
    const char *base = "/tmp/yass_test_list_error";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/bad.yass.yaml", base);
    write_file(spec, ":\n  :\n    - [\n");

    char *args[] = { spec };
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_list_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 1);
    ASSERT_NOT_NULL(out_stderr);
    /* ErrorLine should contain an error code */
    ASSERT_STR_CONTAINS(out_stderr, "yass.yaml.");

    free(out_stdout);
    free(out_stderr);
    rmtree(base);
}

/*
 * Multiple files sorted by Unicode code-point order on NFC-normalized path.
 */
TEST(list_multiple_files_sorted)
{
    const char *base = "/tmp/yass_test_list_sorted";
    rmtree(base);
    mkdirp(base);

    /* Create files with names that will sort in a specific order */
    char fa[4096], fb[4096], fc[4096];
    snprintf(fa, sizeof(fa), "%s/aaa.yass.yaml", base);
    snprintf(fb, sizeof(fb), "%s/bbb.yass.yaml", base);
    snprintf(fc, sizeof(fc), "%s/ccc.yass.yaml", base);

    write_file(fc,
        "---\n"
        "description: C file\n"
        "version: v1\n"
        "---\n"
        "spec: ccc.Spec\n"
        "INPUT:\n"
        "- MUST: c\n"
    );
    write_file(fa,
        "---\n"
        "description: A file\n"
        "version: v1\n"
        "---\n"
        "spec: aaa.Spec\n"
        "INPUT:\n"
        "- MUST: a\n"
    );
    write_file(fb,
        "---\n"
        "description: B file\n"
        "version: v1\n"
        "---\n"
        "spec: bbb.Spec\n"
        "INPUT:\n"
        "- MUST: b\n"
    );

    /* List from directory: files should be sorted aaa < bbb < ccc */
    char *args[] = { (char *)base };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);

    char *pos_a = strstr(out, "aaa.Spec");
    char *pos_b = strstr(out, "bbb.Spec");
    char *pos_c = strstr(out, "ccc.Spec");
    ASSERT_NOT_NULL(pos_a);
    ASSERT_NOT_NULL(pos_b);
    ASSERT_NOT_NULL(pos_c);
    ASSERT(pos_a < pos_b);
    ASSERT(pos_b < pos_c);

    free(out);
    rmtree(base);
}

/*
 * Multiple specs in one file -> all listed in document order.
 */
TEST(list_multiple_specs)
{
    const char *base = "/tmp/yass_test_list_multi";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/multi.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: Multi spec file\n"
        "version: v1\n"
        "---\n"
        "spec: multi.First\n"
        "INPUT:\n"
        "- MUST: one\n"
        "---\n"
        "spec: multi.Second\n"
        "INPUT:\n"
        "- MUST: two\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);

    /* Both specs should appear */
    ASSERT_STR_CONTAINS(out, "multi.First");
    ASSERT_STR_CONTAINS(out, "multi.Second");
    /* First should come before Second (document order) */
    char *pos1 = strstr(out, "multi.First");
    char *pos2 = strstr(out, "multi.Second");
    ASSERT(pos1 < pos2);

    free(out);
    rmtree(base);
}

/*
 * Empty description -> two tab separators, empty third field.
 */
TEST(list_empty_description)
{
    const char *base = "/tmp/yass_test_list_empty_desc";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/nodesc.yass.yaml", base);
    write_file(spec,
        "---\n"
        "version: v1\n"
        "---\n"
        "spec: nodesc.Test\n"
        "INPUT:\n"
        "- MUST: test\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* Should have two tabs in the output line */
    ASSERT_STR_CONTAINS(out, "\tnodesc.Test\t");
    /* Line should end with tab then newline (empty description) */
    char *line = strstr(out, "nodesc.Test\t\n");
    ASSERT_NOT_NULL(line);

    free(out);
    rmtree(base);
}

/*
 * Mixed: one good file and one bad file -> lists parseable specs, exit 1.
 */
TEST(list_mixed_good_bad)
{
    const char *base = "/tmp/yass_test_list_mixed";
    rmtree(base);
    mkdirp(base);

    char good[4096], bad[4096];
    snprintf(good, sizeof(good), "%s/good.yass.yaml", base);
    snprintf(bad, sizeof(bad), "%s/bad.yass.yaml", base);

    write_file(good,
        "---\n"
        "description: Good file\n"
        "version: v1\n"
        "---\n"
        "spec: good.Spec\n"
        "INPUT:\n"
        "- MUST: work\n"
    );
    write_file(bad, ":\n  :\n    - [\n");

    /* List from directory */
    char *args[] = { (char *)base };
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_list_both(1, args, &out_stdout, &out_stderr);
    /* Should exit 1 because of bad file */
    ASSERT_EQ(rc, 1);
    /* But should still list the good spec */
    ASSERT_STR_CONTAINS(out_stdout, "good.Spec");
    /* And stderr should have the error */
    ASSERT_STR_CONTAINS(out_stderr, "yass.yaml.");

    free(out_stdout);
    free(out_stderr);
    rmtree(base);
}

/*
 * Description with literal whitespace that needs collapsing:
 * tabs, newlines, multiple spaces all become single space.
 */
TEST(list_description_multiline)
{
    const char *base = "/tmp/yass_test_list_multiline";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/ml.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: |\n"
        "  line one\n"
        "  line two\n"
        "  line three\n"
        "version: v1\n"
        "---\n"
        "spec: ml.Test\n"
        "INPUT:\n"
        "- MUST: check\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* Newlines in literal block should be collapsed to spaces */
    ASSERT_STR_CONTAINS(out, "line one line two line three");

    free(out);
    rmtree(base);
}

/*
 * Test colon-in-path rejection.
 */
TEST(list_colon_in_path)
{
    char *args[] = { "path:with:colon" };
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_list_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    ASSERT_STR_CONTAINS(out_stderr, "yass.path.colon_in_path");
    free(out_stdout);
    free(out_stderr);
}

/*
 * Test discover returning fatal error (non-existent path).
 */
TEST(list_discover_error)
{
    char *args[] = { "/tmp/yass_nonexistent_path_for_list_xyz" };
    char *out_stdout = NULL;
    char *out_stderr = NULL;
    int rc = capture_list_both(1, args, &out_stdout, &out_stderr);
    ASSERT_EQ(rc, 2);
    free(out_stdout);
    free(out_stderr);
}

/*
 * Test replace_tabs function via a file path containing a tab.
 * This is hard to test directly since filesystem paths rarely have tabs,
 * but we can test that the output format is correct for a normal file.
 */
TEST(list_normal_output_format)
{
    const char *base = "/tmp/yass_test_list_format";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/fmt.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: A short description\n"
        "version: v1\n"
        "---\n"
        "spec: fmt.One\n"
        "INPUT:\n"
        "- MUST: do something\n"
        "---\n"
        "spec: fmt.Two\n"
        "RETURN:\n"
        "- MUST: return something\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* Two specs, both in document order */
    ASSERT_STR_CONTAINS(out, "fmt.One");
    ASSERT_STR_CONTAINS(out, "fmt.Two");
    /* Each line has exactly 2 tabs */
    char *p = out;
    int line_count = 0;
    while (*p) {
        int tabs = 0;
        while (*p && *p != '\n') {
            if (*p == '\t') tabs++;
            p++;
        }
        if (*p == '\n') { p++; line_count++; }
        ASSERT_EQ(tabs, 2);
    }
    ASSERT_EQ(line_count, 2);

    free(out);
    rmtree(base);
}

/*
 * Capture stdout from cmd_list using a pseudo-terminal so isatty() returns true.
 * Sets COLUMNS env var to control terminal width.
 * Returns the exit code. Sets *out to a malloc'd string (caller frees).
 */
static int capture_list_tty(int argc, char **argv, int columns, char **out)
{
    fflush(stdout);

    /* Set COLUMNS env var */
    char col_buf[32];
    snprintf(col_buf, sizeof(col_buf), "%d", columns);
    setenv("COLUMNS", col_buf, 1);

    /* Open a pseudo-terminal pair */
    int master_fd, slave_fd;
    if (openpty(&master_fd, &slave_fd, NULL, NULL, NULL) < 0) {
        *out = strdup("");
        unsetenv("COLUMNS");
        return -1;
    }

    /* Disable output processing on slave to avoid CR/LF translation */
    struct termios t;
    tcgetattr(slave_fd, &t);
    t.c_oflag &= ~OPOST;
    tcsetattr(slave_fd, TCSANOW, &t);

    /* Set master to non-blocking for reading */
    int flags = fcntl(master_fd, F_GETFL);
    fcntl(master_fd, F_SETFL, flags | O_NONBLOCK);

    int saved_stdout = dup(STDOUT_FILENO);
    dup2(slave_fd, STDOUT_FILENO);
    /* Keep slave_fd open until after reading from master */

    int rc = cmd_list(argc, argv);

    fflush(stdout);

    /* Read captured output from master BEFORE restoring stdout */
    char buf[8192];
    size_t total = 0;
    memset(buf, 0, sizeof(buf));

    while (total < sizeof(buf) - 1) {
        ssize_t n = read(master_fd, buf + total, sizeof(buf) - 1 - total);
        if (n <= 0)
            break;
        total += (size_t)n;
    }
    buf[total] = '\0';

    /* Now restore stdout and clean up */
    dup2(saved_stdout, STDOUT_FILENO);
    close(saved_stdout);
    close(slave_fd);
    close(master_fd);

    unsetenv("COLUMNS");

    *out = strdup(buf);
    return rc;
}

/*
 * TTY truncation: description fits without truncation.
 * Set a wide terminal so everything fits.
 */
TEST(list_tty_no_truncation)
{
    const char *base = "/tmp/yass_test_list_tty_notrunc";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/t.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: Short\n"
        "version: v1\n"
        "---\n"
        "spec: t.X\n"
        "INPUT:\n"
        "- MUST: x\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    /* Wide terminal: everything fits */
    int rc = capture_list_tty(1, args, 200, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "t.X");
    ASSERT_STR_CONTAINS(out, "Short");
    /* No truncation marker */
    ASSERT(strstr(out, "...") == NULL);

    free(out);
    rmtree(base);
}

/*
 * TTY truncation: description is truncated with "..." marker.
 * Set a narrow terminal width.
 */
TEST(list_tty_truncation)
{
    const char *base = "/tmp/yass_test_list_tty_trunc";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/t.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: This is a very long description that should be truncated when the terminal is narrow\n"
        "version: v1\n"
        "---\n"
        "spec: t.Y\n"
        "INPUT:\n"
        "- MUST: y\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    /* Narrow terminal: force truncation */
    int rc = capture_list_tty(1, args, 50, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "t.Y");
    /* Should have truncation marker */
    ASSERT_STR_CONTAINS(out, "...");

    free(out);
    rmtree(base);
}

/*
 * TTY truncation: terminal too narrow for even the marker.
 * fixed + marker_len >= term_width -> empty description field.
 */
TEST(list_tty_no_room_for_marker)
{
    const char *base = "/tmp/yass_test_list_tty_noroom";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/t.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: Something here\n"
        "version: v1\n"
        "---\n"
        "spec: t.Z\n"
        "INPUT:\n"
        "- MUST: z\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    /* Very narrow: path + name + separators already exceed width */
    int rc = capture_list_tty(1, args, 10, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "t.Z");
    /* Description should be empty, no truncation marker */
    ASSERT(strstr(out, "...") == NULL);

    free(out);
    rmtree(base);
}

/*
 * TTY with empty description: empty third field, no marker.
 */
TEST(list_tty_empty_description)
{
    const char *base = "/tmp/yass_test_list_tty_emptydesc";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/t.yass.yaml", base);
    write_file(spec,
        "---\n"
        "version: v1\n"
        "---\n"
        "spec: t.E\n"
        "INPUT:\n"
        "- MUST: e\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list_tty(1, args, 80, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_CONTAINS(out, "t.E");

    free(out);
    rmtree(base);
}

/*
 * Empty .yass.yaml file (triggers empty_file/empty_stream skip in list).
 */
TEST(list_empty_file_skip)
{
    const char *base = "/tmp/yass_test_list_emptyfile";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/empty.yass.yaml", base);
    write_file(spec, "");

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    /* Empty file should be silently skipped, exit 0 */
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_EQ(out, "");

    free(out);
    rmtree(base);
}

/*
 * File with only comments (empty stream) -> no rows, no error.
 */
TEST(list_comment_only_file)
{
    const char *base = "/tmp/yass_test_list_commentonly";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/comment.yass.yaml", base);
    write_file(spec, "# just a comment\n");

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    /* Comment-only file should be silently skipped, exit 0 */
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    ASSERT_STR_EQ(out, "");

    free(out);
    rmtree(base);
}

/*
 * Document without "spec" key -> skip (no output row for that doc).
 */
TEST(list_doc_without_spec_key)
{
    const char *base = "/tmp/yass_test_list_nospeckey";
    rmtree(base);
    mkdirp(base);

    char spec[4096];
    snprintf(spec, sizeof(spec), "%s/nokey.yass.yaml", base);
    write_file(spec,
        "---\n"
        "description: Has desc\n"
        "version: v1\n"
        "---\n"
        "notspec: something\n"
        "INPUT:\n"
        "- MUST: x\n"
        "---\n"
        "spec: nokey.Real\n"
        "INPUT:\n"
        "- MUST: y\n"
    );

    char *args[] = { spec };
    char *out = NULL;
    int rc = capture_list(1, args, &out);
    ASSERT_EQ(rc, 0);
    ASSERT_NOT_NULL(out);
    /* Only the doc with "spec" key should appear */
    ASSERT_STR_CONTAINS(out, "nokey.Real");
    /* The doc without "spec" key should not produce a row */
    ASSERT(strstr(out, "notspec") == NULL);

    free(out);
    rmtree(base);
}

/* ---- Suite registration ---- */

void run_suite_list(void)
{
    RUN_TEST(list_single_file);
    RUN_TEST(list_from_directory);
    RUN_TEST(list_no_files);
    RUN_TEST(list_preamble_only);
    RUN_TEST(list_description_whitespace);
    RUN_TEST(list_tab_separated_fields);
    RUN_TEST(list_parse_error);
    RUN_TEST(list_multiple_files_sorted);
    RUN_TEST(list_multiple_specs);
    RUN_TEST(list_empty_description);
    RUN_TEST(list_mixed_good_bad);
    RUN_TEST(list_description_multiline);
    RUN_TEST(list_colon_in_path);
    RUN_TEST(list_discover_error);
    RUN_TEST(list_normal_output_format);
    RUN_TEST(list_tty_no_truncation);
    RUN_TEST(list_tty_truncation);
    RUN_TEST(list_tty_no_room_for_marker);
    RUN_TEST(list_tty_empty_description);
    RUN_TEST(list_empty_file_skip);
    RUN_TEST(list_comment_only_file);
    RUN_TEST(list_doc_without_spec_key);
}

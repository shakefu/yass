/*
 * main.c - Entry point for the yass CLI
 *
 * Per cli.Dispatch spec:
 * - Parse global flags (--help, --version)
 * - Select and dispatch to subcommand
 * - Handle signals (SIGPIPE, SIGINT, SIGTERM)
 * - All output is UTF-8 with LF line endings
 * - Line-buffer stdout
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <unistd.h>

#include "yass.h"
#include "argv.h"
#include "error.h"
#include "validate.h"
#include "query.h"
#include "list.h"

/* Signal state */
static volatile sig_atomic_t g_sigint_received = 0;
static volatile sig_atomic_t g_sigterm_received = 0;

static void handle_sigpipe(int sig) {
    (void)sig;
    _exit(0);
}

static void handle_sigint(int sig) {
    (void)sig;
    g_sigint_received = 1;
    _exit(EXIT_SIGINT);
}

static void handle_sigterm(int sig) {
    (void)sig;
    g_sigterm_received = 1;
    _exit(EXIT_SIGTERM);
}

static void print_usage(FILE *out) {
    fprintf(out,
        "usage: yass <subcommand> [options]\n"
        "\n"
        "subcommands:\n"
        "  validate   Validate .yass.yaml files\n"
        "  query      Query a spec by name\n"
        "  list       List specs in .yass.yaml files\n"
        "\n"
        "flags:\n"
        "  --help     Print this help message\n"
        "  --version  Print the version\n");
}

static void print_version(void) {
    printf("yass %s\n", YASS_VERSION);
}

int main(int argc, char **argv) {
    /* Line-buffer stdout per spec */
    setvbuf(stdout, NULL, _IOLBF, 0);

    /* Set up signal handlers per spec */
    struct sigaction sa;
    memset(&sa, 0, sizeof(sa));

    sa.sa_handler = handle_sigpipe;
    sa.sa_flags = 0;
    sigemptyset(&sa.sa_mask);
    sigaction(SIGPIPE, &sa, NULL);

    sa.sa_handler = handle_sigint;
    sigaction(SIGINT, &sa, NULL);

    sa.sa_handler = handle_sigterm;
    sigaction(SIGTERM, &sa, NULL);

    /* Parse arguments */
    yass_args_t args;
    memset(&args, 0, sizeof(args));

    int rc = argv_parse(argc, argv, &args);
    if (rc != 0) {
        print_usage(stderr);
        return rc;
    }

    /* Handle global flags */
    if (args.help || args.command == CMD_HELP) {
        print_usage(stdout);
        return 0;
    }

    if (args.version || args.command == CMD_VERSION) {
        print_version();
        return 0;
    }

    /* Dispatch to subcommand */
    int exit_code = 0;
    switch (args.command) {
        case CMD_VALIDATE:
            exit_code = cmd_validate(args.positional_count, args.positionals);
            break;
        case CMD_QUERY:
            exit_code = cmd_query(args.positional_count, args.positionals);
            break;
        case CMD_LIST:
            exit_code = cmd_list(args.positional_count, args.positionals);
            break;
        default:
            /* Should not happen - argv_parse should have caught this */
            error_emit(NULL, 0, EC_INTERNAL_UNCAUGHT, "internal error: unexpected command state");
            exit_code = EXIT_PROCESSING;
            break;
    }

    return exit_code;
}

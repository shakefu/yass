/*
 * argv.h - Argument parsing per cli.Dispatch spec
 */
#ifndef YASS_ARGV_H
#define YASS_ARGV_H

#include <stdbool.h>

typedef enum {
    CMD_NONE = 0,
    CMD_HELP,
    CMD_VERSION,
    CMD_VALIDATE,
    CMD_QUERY,
    CMD_LIST,
} yass_command_t;

typedef struct {
    yass_command_t command;
    int positional_count;
    char **positionals;  /* non-owned pointers into argv */
    bool help;
    bool version;
} yass_args_t;

/*
 * Parse command-line arguments.
 * Returns 0 on success.
 * On error, emits an ErrorLine to stderr and returns the exit code (2).
 */
int argv_parse(int argc, char **argv, yass_args_t *args);

#endif /* YASS_ARGV_H */

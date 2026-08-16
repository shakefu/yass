/*
 * error.c - Error line formatting per cli.ErrorLine spec
 */
#include "error.h"
#include "path.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

char *error_format_path(const char *file)
{
    if (!file)
        return strdup("yass");
    return path_relative_to_cwd(file);
}

char *error_sanitize_message(const char *message)
{
    if (!message)
        return strdup("");
    size_t len = strlen(message);
    char *out = malloc(len + 1);
    if (!out)
        return NULL;
    for (size_t i = 0; i < len; i++) {
        if (message[i] == '\n' || message[i] == '\r')
            out[i] = ' ';
        else
            out[i] = message[i];
    }
    out[len] = '\0';
    return out;
}

void error_emit(const char *file, int line, const char *code,
                const char *message)
{
    char *path = error_format_path(file);
    char *msg = error_sanitize_message(message);

    if (line > 0)
        fprintf(stderr, "%s:%d: [%s] %s\n", path, line, code, msg);
    else
        fprintf(stderr, "%s: [%s] %s\n", path, code, msg);

    free(path);
    free(msg);
}

void error_counter_init(error_counter_t *ec)
{
    ec->count = 0;
}

void error_count(error_counter_t *ec)
{
    ec->count++;
}

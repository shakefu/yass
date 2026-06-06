/*
 * emit.c - YAML emitter for query output per cli.query.OutputProfile
 */
#include "emit.h"

#include <ctype.h>
#include <string.h>

/* ------------------------------------------------------------------ */
/*  Quoting helpers                                                    */
/* ------------------------------------------------------------------ */

/*
 * Case-insensitive match against a fixed token.
 */
static int ci_eq(const char *s, const char *tok) {
    for (; *tok; s++, tok++) {
        if (tolower((unsigned char)*s) != tolower((unsigned char)*tok))
            return 0;
    }
    return *s == '\0';
}

/*
 * Return 1 if `s` looks like a YAML 1.2 core-schema numeric literal:
 *   - decimal integer  (optional sign, digits)
 *   - hex   0x[0-9a-fA-F]+
 *   - octal 0o[0-7]+
 *   - float (optional sign, digits with at most one '.', optional e/E exponent)
 *   - .inf / -.inf / +.inf / .nan (case-insensitive)
 */
static int is_numeric(const char *s) {
    if (!s || !*s) return 0;

    const char *p = s;

    /* optional sign */
    if (*p == '+' || *p == '-') p++;

    /* .inf / .nan */
    if (ci_eq(p, ".inf") || ci_eq(p, ".nan"))
        return 1;

    /* hex 0x... */
    if (p[0] == '0' && (p[1] == 'x' || p[1] == 'X')) {
        p += 2;
        if (!*p) return 0;
        while (*p && isxdigit((unsigned char)*p)) p++;
        return *p == '\0';
    }

    /* octal 0o... */
    if (p[0] == '0' && (p[1] == 'o' || p[1] == 'O')) {
        p += 2;
        if (!*p) return 0;
        while (*p && *p >= '0' && *p <= '7') p++;
        return *p == '\0';
    }

    /* decimal integer or float */
    int has_digit = 0;
    int has_dot = 0;
    while (*p) {
        if (isdigit((unsigned char)*p)) {
            has_digit = 1;
            p++;
        } else if (*p == '.' && !has_dot) {
            has_dot = 1;
            p++;
        } else if ((*p == 'e' || *p == 'E') && has_digit) {
            p++;
            if (*p == '+' || *p == '-') p++;
            if (!isdigit((unsigned char)*p)) return 0;
            while (isdigit((unsigned char)*p)) p++;
            return *p == '\0';
        } else {
            break;
        }
    }
    return has_digit && *p == '\0';
}

/*
 * Check if a scalar value needs double-quoting per OutputProfile rules.
 *
 * Quote when:
 *   - Empty string
 *   - Contains ": " (colon-space)
 *   - Leading character in ?-*&!|>%@
 *   - Leading or trailing whitespace
 *   - Matches YAML 1.2 core-schema type tokens (case-insensitive):
 *       true, false, null, yes, no, on, off
 *   - Numeric literal (integer, float, hex, octal, infinity, nan)
 */
int emit_needs_quoting(const char *value) {
    if (!value || !*value)
        return 1;  /* empty string */

    size_t len = strlen(value);

    /* leading / trailing whitespace */
    if (value[0] == ' ' || value[0] == '\t' ||
        value[len - 1] == ' ' || value[len - 1] == '\t')
        return 1;

    /* leading special character */
    if (strchr("?-*&!|>%@", value[0]))
        return 1;

    /* contains ": " */
    if (strstr(value, ": "))
        return 1;

    /* core-schema type tokens (case-insensitive) */
    if (ci_eq(value, "true")  || ci_eq(value, "false") ||
        ci_eq(value, "null")  ||
        ci_eq(value, "yes")   || ci_eq(value, "no")    ||
        ci_eq(value, "on")    || ci_eq(value, "off"))
        return 1;

    /* numeric literal */
    if (is_numeric(value))
        return 1;

    return 0;
}

/* ------------------------------------------------------------------ */
/*  Double-quote escaping                                              */
/* ------------------------------------------------------------------ */

/*
 * Write `value` to `out` surrounded by double quotes, escaping
 * backslash, double-quote, newline, and tab.
 */
static void emit_double_quoted(FILE *out, const char *value) {
    fputc('"', out);
    for (const char *p = value; *p; p++) {
        switch (*p) {
            case '\\': fputs("\\\\", out); break;
            case '"':  fputs("\\\"", out); break;
            case '\n': fputs("\\n", out);  break;
            case '\t': fputs("\\t", out);  break;
            default:   fputc(*p, out);     break;
        }
    }
    fputc('"', out);
}

/* ------------------------------------------------------------------ */
/*  Scalar emission                                                    */
/* ------------------------------------------------------------------ */

static void emit_scalar(FILE *out, const char *value) {
    if (emit_needs_quoting(value)) {
        emit_double_quoted(out, value);
    } else {
        fputs(value, out);
    }
}

/* ------------------------------------------------------------------ */
/*  Recursive node / kv emission                                       */
/* ------------------------------------------------------------------ */

static void emit_kv(FILE *out, yass_yaml_kv_t *kv, int indent);
static void emit_node(FILE *out, yass_yaml_node_t *node, int indent);

/*
 * Emit a mapping's key-value pairs at the given indentation level.
 */
static void emit_mapping(FILE *out, yass_yaml_kv_t *kvs, int count, int indent) {
    for (int i = 0; i < count; i++) {
        emit_kv(out, &kvs[i], indent);
    }
}

/*
 * Emit a single key-value pair.
 */
static void emit_kv(FILE *out, yass_yaml_kv_t *kv, int indent) {
    /* Print indent */
    for (int i = 0; i < indent; i++) fputc(' ', out);

    /* Print key */
    fputs(kv->key, out);
    fputs(":", out);

    switch (kv->value_type) {
        case 0: /* scalar */
            fputc(' ', out);
            emit_scalar(out, kv->scalar_value);
            fputc('\n', out);
            break;

        case 1: /* sequence */
            fputc('\n', out);
            for (int i = 0; i < kv->sequence_count; i++) {
                emit_node(out, &kv->sequence_values[i], indent);
            }
            break;

        case 2: /* mapping */
            fputc('\n', out);
            emit_mapping(out, kv->mapping_values, kv->mapping_count, indent + 2);
            break;

        case 3: /* null */
            fputc('\n', out);
            break;

        default:
            fputc('\n', out);
            break;
    }
}

/*
 * Emit a sequence item (node).
 *
 * For mappings (the common obligation case):
 *   "- first_key: first_value\n"
 *   "  next_key: next_value\n"
 *
 * For scalars:
 *   "- value\n"
 */
static void emit_node(FILE *out, yass_yaml_node_t *node, int indent) {
    /* Print indent */
    for (int i = 0; i < indent; i++) fputc(' ', out);

    switch (node->type) {
        case 0: /* scalar */
            fputs("- ", out);
            emit_scalar(out, node->scalar_value);
            fputc('\n', out);
            break;

        case 2: /* mapping */
            if (node->mapping_count > 0) {
                /* First kv gets the "- " prefix */
                fputs("- ", out);
                fputs(node->mapping_values[0].key, out);
                fputs(": ", out);
                if (node->mapping_values[0].value_type == 0) {
                    emit_scalar(out, node->mapping_values[0].scalar_value);
                    fputc('\n', out);
                } else if (node->mapping_values[0].value_type == 3) {
                    fputc('\n', out);
                } else {
                    /* nested structure under first key of a list item */
                    fputc('\n', out);
                    if (node->mapping_values[0].value_type == 1) {
                        for (int j = 0; j < node->mapping_values[0].sequence_count; j++) {
                            emit_node(out, &node->mapping_values[0].sequence_values[j], indent + 2);
                        }
                    } else if (node->mapping_values[0].value_type == 2) {
                        emit_mapping(out, node->mapping_values[0].mapping_values,
                                     node->mapping_values[0].mapping_count, indent + 4);
                    }
                }

                /* Remaining kvs aligned at indent + 2 */
                for (int i = 1; i < node->mapping_count; i++) {
                    emit_kv(out, &node->mapping_values[i], indent + 2);
                }
            }
            break;

        case 1: /* sequence (nested) */
            fputs("- ", out);
            fputc('\n', out);
            for (int i = 0; i < node->sequence_count; i++) {
                emit_node(out, &node->sequence_values[i], indent + 2);
            }
            break;

        case 3: /* null */
            fputs("- ", out);
            fputc('\n', out);
            break;

        default:
            fputs("- ", out);
            fputc('\n', out);
            break;
    }
}

/* ------------------------------------------------------------------ */
/*  Public API                                                         */
/* ------------------------------------------------------------------ */

/*
 * Emit a spec document as a YAML fragment per OutputProfile.
 *
 * Returns 0 on success, non-zero on error.
 */
int emit_spec_fragment(FILE *out, yaml_doc_t *doc) {
    if (!out || !doc) return 1;

    /* Document start marker */
    fputs("---\n", out);

    /* Emit each top-level key-value pair */
    for (int i = 0; i < doc->kv_count; i++) {
        emit_kv(out, &doc->kvs[i], 0);
    }

    return 0;
}

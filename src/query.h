/*
 * query.h - Query subcommand per cli.query spec
 */
#ifndef YASS_QUERY_H
#define YASS_QUERY_H

/*
 * Run the query subcommand.
 * argc/argv are the positional arguments after "query".
 * Returns exit code.
 */
int cmd_query(int argc, char **argv);

#endif /* YASS_QUERY_H */

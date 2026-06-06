/*
 * list.h - List subcommand per cli.list spec
 */
#ifndef YASS_LIST_H
#define YASS_LIST_H

/*
 * Run the list subcommand.
 * argc/argv are the positional arguments after "list".
 * Returns exit code.
 */
int cmd_list(int argc, char **argv);

#endif /* YASS_LIST_H */

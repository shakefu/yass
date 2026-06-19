// Argument parsing and subcommand dispatch for yass.

use crate::errors::CliError;

/// The yass version string, pulled from Cargo.toml at compile time.
const VERSION: &str = env!("CARGO_PKG_VERSION");

/// Known subcommands.
const SUBCOMMANDS: &[&str] = &["validate", "list", "query"];

/// Known global flags.
const KNOWN_FLAGS: &[&str] = &["--help", "--version"];

/// Parsed CLI arguments ready for dispatch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ParsedArgs {
    Help,
    Version,
    Validate { paths: Vec<String> },
    List { path: Option<String> },
    Query { name: String, scope: Option<String> },
}

/// Usage text printed for --help.
const USAGE: &str = "\
usage: yass [--help|--version] <subcommand> [args...]

subcommands:
  validate [paths...]   Validate YAML spec files
  list [path]           List spec files and their contents
  query <name> [scope]  Query spec files for specific data

flags:
  --help                Show this help message
  --version             Show version information";

/// Return the usage string.
pub fn usage() -> &'static str {
    USAGE
}

/// Return the version string in "yass <version>" format.
pub fn version_string() -> String {
    format!("yass {}", VERSION)
}

/// Parse CLI arguments (excluding argv[0], the program name).
///
/// `args` should be &args[1..] -- everything after the binary name.
pub fn parse_args(args: &[String]) -> Result<ParsedArgs, CliError> {
    // Phase 1: Scan entire argv for --help (highest priority), then --version.
    if args.iter().any(|a| a == "--help") {
        return Ok(ParsedArgs::Help);
    }
    if args.iter().any(|a| a == "--version") {
        return Ok(ParsedArgs::Version);
    }

    // Phase 2: If no args at all, that's no_subcommand.
    if args.is_empty() {
        return Err(CliError::new(
            crate::errors::ARGV_NO_SUBCOMMAND,
            "no subcommand given",
        ));
    }

    // Phase 3: Validate each token before subcommand extraction.
    // We process args[0] as the subcommand candidate, then remaining args.
    let first = &args[0];

    // Check the first token for errors before treating it as a subcommand.
    check_token_errors(first)?;

    // If the first token starts with "--", it's a flag position but not --help/--version.
    if first.starts_with("--") {
        return Err(CliError::new(
            crate::errors::ARGV_UNKNOWN_FLAG,
            format!("unknown flag: {}", first),
        ));
    }

    // If the first token starts with "-" (short flag or bare dash) the check_token_errors
    // above would have caught it. This is a safety fallback.
    if first.starts_with('-') {
        return Err(CliError::new(
            crate::errors::ARGV_SHORT_FLAG,
            format!("short-form flags are not supported in v1: {}", first),
        ));
    }

    // Check if this is a known subcommand.
    let subcommand = first.as_str();
    if !SUBCOMMANDS.contains(&subcommand) {
        return Err(CliError::new(
            crate::errors::ARGV_UNKNOWN_SUBCOMMAND,
            format!("unknown subcommand: {}", subcommand),
        ));
    }

    // Phase 4: Parse remaining args for the subcommand.
    let rest = &args[1..];
    parse_subcommand_args(subcommand, rest)
}

/// Check a single token for structural errors (empty, bare dash, short flag,
/// case mismatch, abbreviation). Returns Ok(()) if the token is fine.
fn check_token_errors(token: &str) -> Result<(), CliError> {
    // Empty string argument.
    if token.is_empty() {
        return Err(CliError::new(
            crate::errors::ARGV_EMPTY_ARGUMENT,
            "empty argument",
        ));
    }

    // Bare "-" (stdin dash).
    if token == "-" {
        return Err(CliError::new(
            crate::errors::ARGV_STDIN_DASH,
            "stdin marker `-` is not supported; pass a file path",
        ));
    }

    // Short flag: starts with "-" but not "--".
    if token.starts_with('-') && !token.starts_with("--") {
        return Err(CliError::new(
            crate::errors::ARGV_SHORT_FLAG,
            format!("short-form flags are not supported in v1: {}", token),
        ));
    }

    // Case mismatch: check flags and subcommands.
    let lower = token.to_lowercase();

    // Check flag case mismatch (e.g., --Help, --VERSION).
    if token.starts_with("--") {
        for flag in KNOWN_FLAGS {
            if lower == *flag && token != *flag {
                return Err(CliError::new(
                    crate::errors::ARGV_CASE_MISMATCH,
                    format!("subcommand or flag case mismatch: {}", token),
                ));
            }
        }
    } else {
        // Check subcommand case mismatch (e.g., Validate, LIST).
        for sub in SUBCOMMANDS {
            if lower == *sub && token != *sub {
                return Err(CliError::new(
                    crate::errors::ARGV_CASE_MISMATCH,
                    format!("subcommand or flag case mismatch: {}", token),
                ));
            }
        }
    }

    // Abbreviation: token is a strict prefix of a subcommand or flag.
    if token.starts_with("--") {
        for flag in KNOWN_FLAGS {
            if *flag != token && flag.starts_with(token) {
                return Err(CliError::new(
                    crate::errors::ARGV_ABBREVIATION,
                    format!("abbreviations are not supported: {}", token),
                ));
            }
        }
    } else if !token.starts_with('-') && !token.is_empty() {
        for sub in SUBCOMMANDS {
            if *sub != token && sub.starts_with(token) {
                return Err(CliError::new(
                    crate::errors::ARGV_ABBREVIATION,
                    format!("abbreviations are not supported: {}", token),
                ));
            }
        }
    }

    Ok(())
}

/// Parse arguments after the subcommand has been identified.
/// Handles "--" end-of-options, flag validation, and positional extraction.
fn parse_subcommand_args(
    subcommand: &str,
    rest: &[String],
) -> Result<ParsedArgs, CliError> {
    let mut positionals: Vec<String> = Vec::new();
    let mut past_double_dash = false;

    for arg in rest {
        if past_double_dash {
            // After --, everything is a positional. Still check for empty and colon.
            check_positional(arg)?;
            positionals.push(arg.clone());
            continue;
        }

        if arg == "--" {
            past_double_dash = true;
            continue;
        }

        // Before --, validate the token for structural errors.
        check_token_errors(arg)?;

        // If it starts with "--", it's a flag (and not --help/--version since
        // those were already consumed in phase 1).
        if arg.starts_with("--") {
            return Err(CliError::new(
                crate::errors::ARGV_UNKNOWN_FLAG,
                format!("unknown flag: {}", arg),
            ));
        }

        // If it starts with "-", check_token_errors would have caught it,
        // but guard anyway.
        if arg.starts_with('-') {
            return Err(CliError::new(
                crate::errors::ARGV_SHORT_FLAG,
                format!("short-form flags are not supported in v1: {}", arg),
            ));
        }

        // It's a positional argument.
        check_positional(arg)?;
        positionals.push(arg.clone());
    }

    // Build the appropriate ParsedArgs variant.
    match subcommand {
        "validate" => Ok(ParsedArgs::Validate { paths: positionals }),
        "list" => {
            Ok(ParsedArgs::List {
                path: positionals.into_iter().next(),
            })
        }
        "query" => {
            if positionals.is_empty() {
                return Err(CliError::new(
                    crate::errors::QUERY_NAME_MISSING,
                    "missing spec name",
                ));
            }
            let name = positionals[0].clone();
            let scope = positionals.get(1).cloned();
            Ok(ParsedArgs::Query { name, scope })
        }
        _ => unreachable!("subcommand already validated"),
    }
}

/// Validate a positional argument (after subcommand extraction).
fn check_positional(arg: &str) -> Result<(), CliError> {
    if arg.is_empty() {
        return Err(CliError::new(
            crate::errors::ARGV_EMPTY_ARGUMENT,
            "empty argument",
        ));
    }
    if arg.contains(':') {
        return Err(CliError::new(
            crate::errors::PATH_COLON_IN_PATH,
            format!("path contains an unsupported colon character: {}", arg),
        ));
    }
    Ok(())
}

/// Dispatch CLI arguments to the appropriate subcommand.
/// Returns a process exit code.
pub fn dispatch(args: &[String]) -> i32 {
    // args[0] is the binary name; pass the rest to parse_args.
    let rest = if args.is_empty() { &args[..] } else { &args[1..] };

    match parse_args(rest) {
        Ok(ParsedArgs::Help) => {
            println!("{}", USAGE);
            0
        }
        Ok(ParsedArgs::Version) => {
            println!("yass {}", VERSION);
            0
        }
        Ok(ParsedArgs::Validate { paths }) => {
            let cwd = std::env::current_dir().unwrap_or_else(|_| std::path::PathBuf::from("."));
            crate::validate::run_validate(&paths, &cwd)
        }
        Ok(ParsedArgs::List { path }) => {
            let cwd = std::env::current_dir().unwrap_or_else(|_| std::path::PathBuf::from("."));
            crate::list::run_list(path.as_deref(), &cwd)
        }
        Ok(ParsedArgs::Query { name, scope }) => {
            let cwd = std::env::current_dir().unwrap_or_else(|_| std::path::PathBuf::from("."));
            crate::query::run_query(&name, scope.as_deref(), &cwd)
        }
        Err(e) => {
            let cwd = std::env::current_dir().unwrap_or_else(|_| std::path::PathBuf::from("."));
            crate::error_line::emit_error(&e, &cwd);
            e.exit_code
        }
    }
}

// ─── Tests ────────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;

    /// Helper to create a Vec<String> from string slices.
    fn args(strs: &[&str]) -> Vec<String> {
        strs.iter().map(|s| s.to_string()).collect()
    }

    // ── Help and Version ─────────────────────────────────────────────────

    #[test]
    fn help_flag_only() {
        let result = parse_args(&args(&["--help"]));
        assert_eq!(result, Ok(ParsedArgs::Help));
    }

    #[test]
    fn help_flag_before_subcommand() {
        let result = parse_args(&args(&["--help", "validate"]));
        assert_eq!(result, Ok(ParsedArgs::Help));
    }

    #[test]
    fn help_flag_after_subcommand() {
        let result = parse_args(&args(&["validate", "--help"]));
        assert_eq!(result, Ok(ParsedArgs::Help));
    }

    #[test]
    fn help_flag_among_many_args() {
        let result = parse_args(&args(&["validate", "foo.yaml", "--help", "bar.yaml"]));
        assert_eq!(result, Ok(ParsedArgs::Help));
    }

    #[test]
    fn version_flag_only() {
        let result = parse_args(&args(&["--version"]));
        assert_eq!(result, Ok(ParsedArgs::Version));
    }

    #[test]
    fn version_flag_after_subcommand() {
        let result = parse_args(&args(&["validate", "--version"]));
        assert_eq!(result, Ok(ParsedArgs::Version));
    }

    #[test]
    fn help_takes_priority_over_version() {
        // --help appears before --version
        let result = parse_args(&args(&["--help", "--version"]));
        assert_eq!(result, Ok(ParsedArgs::Help));
    }

    #[test]
    fn help_takes_priority_over_version_reversed() {
        // --version appears first, but --help is scanned first in the loop
        // because we check --help before --version.
        let result = parse_args(&args(&["--version", "--help"]));
        assert_eq!(result, Ok(ParsedArgs::Help));
    }

    // ── No Subcommand ────────────────────────────────────────────────────

    #[test]
    fn empty_args() {
        let result = parse_args(&args(&[]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.no_subcommand");
        assert_eq!(err.exit_code, 2);
    }

    // ── Unknown Subcommand ───────────────────────────────────────────────

    #[test]
    fn unknown_subcommand() {
        let result = parse_args(&args(&["frobnicate"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.unknown_subcommand");
        assert_eq!(err.exit_code, 2);
    }

    // ── Unknown Flag ─────────────────────────────────────────────────────

    #[test]
    fn unknown_flag_as_first_arg() {
        let result = parse_args(&args(&["--foo"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.unknown_flag");
        assert_eq!(err.exit_code, 2);
    }

    #[test]
    fn unknown_flag_after_subcommand() {
        let result = parse_args(&args(&["validate", "--foo"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.unknown_flag");
    }

    // ── Short Flag ───────────────────────────────────────────────────────

    #[test]
    fn short_flag_as_first_arg() {
        let result = parse_args(&args(&["-v"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.short_flag");
        assert_eq!(err.exit_code, 2);
    }

    #[test]
    fn short_flag_h() {
        let result = parse_args(&args(&["-h"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.short_flag");
    }

    #[test]
    fn short_flag_after_subcommand() {
        let result = parse_args(&args(&["validate", "-x"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.short_flag");
    }

    // ── Case Mismatch ────────────────────────────────────────────────────

    #[test]
    fn case_mismatch_subcommand_capitalized() {
        let result = parse_args(&args(&["Validate"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.case_mismatch");
        assert_eq!(err.exit_code, 2);
    }

    #[test]
    fn case_mismatch_subcommand_uppercase() {
        let result = parse_args(&args(&["LIST"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.case_mismatch");
    }

    #[test]
    fn case_mismatch_flag_help() {
        let result = parse_args(&args(&["--Help"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.case_mismatch");
    }

    #[test]
    fn case_mismatch_flag_version() {
        let result = parse_args(&args(&["--VERSION"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.case_mismatch");
    }

    // ── Abbreviation ─────────────────────────────────────────────────────

    #[test]
    fn abbreviation_val() {
        let result = parse_args(&args(&["val"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.abbreviation");
        assert_eq!(err.exit_code, 2);
    }

    #[test]
    fn abbreviation_quer() {
        let result = parse_args(&args(&["quer"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.abbreviation");
    }

    #[test]
    fn abbreviation_lis() {
        let result = parse_args(&args(&["lis"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.abbreviation");
    }

    #[test]
    fn abbreviation_flag_hel() {
        let result = parse_args(&args(&["--hel"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.abbreviation");
    }

    #[test]
    fn abbreviation_flag_ver() {
        let result = parse_args(&args(&["--ver"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.abbreviation");
    }

    // ── Empty Argument ───────────────────────────────────────────────────

    #[test]
    fn empty_argument_first_position() {
        let result = parse_args(&args(&[""]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.empty_argument");
        assert_eq!(err.exit_code, 2);
    }

    #[test]
    fn empty_argument_after_subcommand() {
        let result = parse_args(&args(&["validate", ""]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.empty_argument");
    }

    // ── Stdin Dash ───────────────────────────────────────────────────────

    #[test]
    fn bare_dash() {
        let result = parse_args(&args(&["-"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.stdin_dash");
        assert_eq!(err.exit_code, 2);
    }

    #[test]
    fn bare_dash_after_subcommand() {
        let result = parse_args(&args(&["validate", "-"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.stdin_dash");
    }

    // ── Colon in Path ────────────────────────────────────────────────────

    #[test]
    fn colon_in_path() {
        let result = parse_args(&args(&["validate", "foo:bar.yaml"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.path.colon_in_path");
        assert_eq!(err.exit_code, 2);
    }

    #[test]
    fn colon_in_path_after_double_dash() {
        let result = parse_args(&args(&["validate", "--", "foo:bar.yaml"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.path.colon_in_path");
    }

    // ── Double Dash (end-of-options) ─────────────────────────────────────

    #[test]
    fn double_dash_prevents_flag_errors() {
        // After --, "--foo" should be treated as a positional, not a flag.
        let result = parse_args(&args(&["validate", "--", "--foo"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Validate {
                paths: vec!["--foo".to_string()]
            })
        );
    }

    #[test]
    fn double_dash_prevents_short_flag_errors() {
        let result = parse_args(&args(&["validate", "--", "-x"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Validate {
                paths: vec!["-x".to_string()]
            })
        );
    }

    #[test]
    fn double_dash_prevents_abbreviation_errors() {
        // "val" would normally be an abbreviation error, but after -- it's positional.
        let result = parse_args(&args(&["validate", "--", "val"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Validate {
                paths: vec!["val".to_string()]
            })
        );
    }

    #[test]
    fn double_dash_still_checks_empty() {
        let result = parse_args(&args(&["validate", "--", ""]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.empty_argument");
    }

    #[test]
    fn double_dash_still_checks_colon() {
        let result = parse_args(&args(&["validate", "--", "a:b"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.path.colon_in_path");
    }

    // ── Validate Subcommand ──────────────────────────────────────────────

    #[test]
    fn validate_no_paths() {
        let result = parse_args(&args(&["validate"]));
        assert_eq!(result, Ok(ParsedArgs::Validate { paths: vec![] }));
    }

    #[test]
    fn validate_one_path() {
        let result = parse_args(&args(&["validate", "foo.yaml"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Validate {
                paths: vec!["foo.yaml".to_string()]
            })
        );
    }

    #[test]
    fn validate_multiple_paths() {
        let result = parse_args(&args(&["validate", "a.yaml", "b.yaml", "c.yaml"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Validate {
                paths: vec![
                    "a.yaml".to_string(),
                    "b.yaml".to_string(),
                    "c.yaml".to_string(),
                ]
            })
        );
    }

    // ── List Subcommand ──────────────────────────────────────────────────

    #[test]
    fn list_no_path() {
        let result = parse_args(&args(&["list"]));
        assert_eq!(result, Ok(ParsedArgs::List { path: None }));
    }

    #[test]
    fn list_one_path() {
        let result = parse_args(&args(&["list", "specs/"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::List {
                path: Some("specs/".to_string())
            })
        );
    }

    #[test]
    fn list_multiple_paths_accepted() {
        // list accepts extra positionals (they are ignored by the parser, validated by subcommand)
        let result = parse_args(&args(&["list", "a"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::List {
                path: Some("a".to_string())
            })
        );
    }

    // ── Query Subcommand ─────────────────────────────────────────────────

    #[test]
    fn query_name_only() {
        let result = parse_args(&args(&["query", "my_service"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Query {
                name: "my_service".to_string(),
                scope: None,
            })
        );
    }

    #[test]
    fn query_name_and_scope() {
        let result = parse_args(&args(&["query", "my_service", "specs/"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Query {
                name: "my_service".to_string(),
                scope: Some("specs/".to_string()),
            })
        );
    }

    #[test]
    fn query_no_name() {
        let result = parse_args(&args(&["query"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.query.name_missing");
    }

    #[test]
    fn query_extra_args_accepted() {
        // Extra positionals are accepted at parse level; query subcommand uses first two
        let result = parse_args(&args(&["query", "a", "b", "c"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Query {
                name: "a".to_string(),
                scope: Some("b".to_string()),
            })
        );
    }

    // ── Version String ───────────────────────────────────────────────────

    #[test]
    fn version_string_format() {
        let v = version_string();
        assert!(v.starts_with("yass "));
        assert!(v.len() > 5);
    }

    // ── Edge Cases ───────────────────────────────────────────────────────

    #[test]
    fn double_dash_alone_after_validate() {
        // Just "--" after validate, no positionals.
        let result = parse_args(&args(&["validate", "--"]));
        assert_eq!(result, Ok(ParsedArgs::Validate { paths: vec![] }));
    }

    #[test]
    fn multiple_double_dashes() {
        // Second "--" after the first should be treated as a positional.
        let result = parse_args(&args(&["validate", "--", "--"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Validate {
                paths: vec!["--".to_string()]
            })
        );
    }

    #[test]
    fn bare_dash_after_double_dash() {
        // "-" after "--" is a positional, but still not a valid path (no colon check fails?).
        // Actually bare "-" after -- is just a positional. check_positional only checks
        // empty and colon. So it should be fine as a positional.
        let result = parse_args(&args(&["validate", "--", "-"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Validate {
                paths: vec!["-".to_string()]
            })
        );
    }

    #[test]
    fn subcommand_is_not_abbreviation_of_itself() {
        // "list" should not be flagged as an abbreviation of "list".
        let result = parse_args(&args(&["list"]));
        assert_eq!(result, Ok(ParsedArgs::List { path: None }));
    }

    #[test]
    fn query_with_double_dash_separating_args() {
        let result = parse_args(&args(&["query", "--", "name_arg"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Query {
                name: "name_arg".to_string(),
                scope: None,
            })
        );
    }

    #[test]
    fn query_with_double_dash_and_two_positionals() {
        let result = parse_args(&args(&["query", "--", "name_arg", "scope_arg"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Query {
                name: "name_arg".to_string(),
                scope: Some("scope_arg".to_string()),
            })
        );
    }

    #[test]
    fn unknown_subcommand_does_not_match_abbreviation() {
        // "xyz" is not a prefix of any subcommand, so it should be unknown_subcommand.
        let result = parse_args(&args(&["xyz"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.unknown_subcommand");
    }

    #[test]
    fn case_mismatch_query() {
        let result = parse_args(&args(&["Query"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.case_mismatch");
    }

    #[test]
    fn abbreviation_l_is_prefix_of_list() {
        let result = parse_args(&args(&["l"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.abbreviation");
    }

    #[test]
    fn abbreviation_q_is_prefix_of_query() {
        let result = parse_args(&args(&["q"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.abbreviation");
    }

    #[test]
    fn abbreviation_v_is_prefix_of_validate() {
        let result = parse_args(&args(&["v"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.argv.abbreviation");
    }

    #[test]
    fn help_wins_over_errors() {
        // Even with bad args, --help anywhere means help.
        let result = parse_args(&args(&["--badstuff", "--help"]));
        assert_eq!(result, Ok(ParsedArgs::Help));
    }

    #[test]
    fn version_wins_over_errors() {
        let result = parse_args(&args(&["--version", "--badstuff"]));
        assert_eq!(result, Ok(ParsedArgs::Version));
    }

    #[test]
    fn validate_with_paths_via_double_dash() {
        let result = parse_args(&args(&["validate", "--", "file1.yaml", "file2.yaml"]));
        assert_eq!(
            result,
            Ok(ParsedArgs::Validate {
                paths: vec!["file1.yaml".to_string(), "file2.yaml".to_string()]
            })
        );
    }

    #[test]
    fn colon_in_query_name() {
        let result = parse_args(&args(&["query", "foo:bar"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.path.colon_in_path");
    }

    #[test]
    fn colon_in_list_path() {
        let result = parse_args(&args(&["list", "a:b"]));
        assert!(result.is_err());
        let err = result.unwrap_err();
        assert_eq!(err.code, "yass.path.colon_in_path");
    }
}

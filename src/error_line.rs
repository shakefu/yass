// ErrorLine formatting for yass diagnostic output.
//
// Formats machine-readable error lines per the cli.ErrorLine spec:
//   With source line:    <file>:<line>: [<code>] <message>
//   Without source line: <file>: [<code>] <message>

use std::fmt;
use std::path::{Path, PathBuf};

/// A single diagnostic error line emitted to stderr.
///
/// - `file`: the path the file was reached by (None => use literal "yass")
/// - `line`: 1-based line number of the offending YAML node (None => omit)
/// - `code`: machine-stable error code token from cli.errors
/// - `message`: human-readable message (newlines replaced with spaces on output)
pub struct ErrorLine<'a> {
    pub file: Option<PathBuf>,
    pub line: Option<usize>,
    pub code: &'a str,
    pub message: String,
}

impl<'a> ErrorLine<'a> {
    /// Create a new ErrorLine.
    pub fn new(
        file: Option<PathBuf>,
        line: Option<usize>,
        code: &'a str,
        message: String,
    ) -> Self {
        Self {
            file,
            line,
            code,
            message,
        }
    }
}

impl<'a> fmt::Display for ErrorLine<'a> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Determine the <file> portion.
        let file_str = match &self.file {
            None => "yass".to_string(),
            Some(path) => {
                // Use the current working directory at format time.
                match std::env::current_dir() {
                    Ok(cwd) => format_path(path, &cwd),
                    Err(_) => path_to_forward_slashes(path),
                }
            }
        };

        // Replace any newline characters in message with a single space.
        let sanitized_message = self.message.replace('\n', " ").replace('\r', " ");

        match self.line {
            Some(line) => write!(f, "{}:{}: [{}] {}\n", file_str, line, self.code, sanitized_message),
            None => write!(f, "{}: [{}] {}\n", file_str, self.code, sanitized_message),
        }
    }
}

/// Format a path relative to the given cwd according to the cli.ErrorLine spec:
///
/// - When the lexical absolute path starts with lexical absolute cwd + "/",
///   emit a relative path (without leading "./").
/// - When the file is directly inside cwd, emit basename only.
/// - When the file is not under cwd, emit the absolute path.
/// - Always use forward slashes.
/// - Never resolve symlinks.
pub fn format_path(path: &Path, cwd: &Path) -> String {
    // Get the lexical absolute form of both paths.
    // We use std::path::absolute (stabilized in Rust 1.79) to avoid resolving symlinks.
    let abs_path = lexical_absolute(path, cwd);
    let abs_cwd = lexical_absolute(cwd, cwd);

    let abs_path_str = path_to_forward_slashes(&abs_path);
    let abs_cwd_str = path_to_forward_slashes(&abs_cwd);

    // Check if the path begins with cwd + "/"
    let prefix = if abs_cwd_str.ends_with('/') {
        abs_cwd_str.clone()
    } else {
        format!("{}/", abs_cwd_str)
    };

    if abs_path_str.starts_with(&prefix) {
        // The relative portion after the cwd prefix.
        let relative = &abs_path_str[prefix.len()..];
        // This already has no leading "./" since we stripped the cwd prefix.
        relative.to_string()
    } else if abs_path_str == abs_cwd_str {
        // Edge case: path IS the cwd itself; use basename.
        abs_path
            .file_name()
            .map(|n| n.to_string_lossy().to_string())
            .unwrap_or_else(|| abs_path_str)
    } else {
        // Not under cwd — emit absolute path.
        abs_path_str
    }
}

/// Compute a lexical absolute path without resolving symlinks.
///
/// If the path is already absolute, return it as-is.
/// If the path is relative, join it to the given base.
fn lexical_absolute(path: &Path, base: &Path) -> PathBuf {
    if path.is_absolute() {
        path.to_path_buf()
    } else {
        base.join(path)
    }
}

/// Convert a path to a string using forward slashes on all platforms.
fn path_to_forward_slashes(path: &Path) -> String {
    let s = path.to_string_lossy();
    s.replace('\\', "/")
}

/// Emit a CliError to stderr as a formatted error line.
pub fn emit_error(error: &crate::errors::CliError, cwd: &Path) {
    let file_path = error.file.as_ref().map(PathBuf::from);
    let el = ErrorLine {
        file: file_path,
        line: error.line,
        code: error.code,
        message: error.message.clone(),
    };
    // Use format_path-aware display by manually formatting with the given cwd.
    let file_str = match &el.file {
        None => "yass".to_string(),
        Some(path) => format_path(path, cwd),
    };
    let sanitized_message = el.message.replace('\n', " ").replace('\r', " ");
    let output = match el.line {
        Some(line) => format!("{}:{}: [{}] {}\n", file_str, line, el.code, sanitized_message),
        None => format!("{}: [{}] {}\n", file_str, el.code, sanitized_message),
    };
    eprint!("{}", output);
}

/// Create a display path string from a file path and cwd.
pub fn make_display_path(file_path: &Path, cwd: &Path) -> String {
    format_path(file_path, cwd)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;

    // ---------------------------------------------------------------
    // ErrorLine Display tests
    // ---------------------------------------------------------------

    #[test]
    fn test_format_with_file_and_line() {
        let cwd = PathBuf::from("/home/user/project");
        let el = ErrorLine {
            file: Some(PathBuf::from("/home/user/project/spec/foo.yass.yaml")),
            line: Some(42),
            code: "yass.yaml.malformed",
            message: "YAML well-formedness error".to_string(),
        };
        // We test format_path separately; here test the overall format shape.
        let output = format_error_line_with_cwd(&el, &cwd);
        assert_eq!(
            output,
            "spec/foo.yass.yaml:42: [yass.yaml.malformed] YAML well-formedness error\n"
        );
    }

    #[test]
    fn test_format_without_line() {
        let cwd = PathBuf::from("/home/user/project");
        let el = ErrorLine {
            file: Some(PathBuf::from("/home/user/project/spec/foo.yass.yaml")),
            line: None,
            code: "yass.path.not_found",
            message: "path does not exist: spec/foo.yass.yaml".to_string(),
        };
        let output = format_error_line_with_cwd(&el, &cwd);
        assert_eq!(
            output,
            "spec/foo.yass.yaml: [yass.path.not_found] path does not exist: spec/foo.yass.yaml\n"
        );
    }

    #[test]
    fn test_format_without_file() {
        let el = ErrorLine {
            file: None,
            line: None,
            code: "yass.argv.no_subcommand",
            message: "no subcommand given".to_string(),
        };
        // When file is None, output uses "yass" regardless of cwd.
        let output = el.to_string();
        assert_eq!(
            output,
            "yass: [yass.argv.no_subcommand] no subcommand given\n"
        );
    }

    #[test]
    fn test_format_without_file_with_line() {
        // Even if line is set, if file is None we use "yass".
        let el = ErrorLine {
            file: None,
            line: Some(10),
            code: "yass.internal.uncaught",
            message: "internal error: something broke".to_string(),
        };
        let output = el.to_string();
        assert_eq!(
            output,
            "yass:10: [yass.internal.uncaught] internal error: something broke\n"
        );
    }

    #[test]
    fn test_newline_replacement_in_message() {
        let el = ErrorLine {
            file: None,
            line: None,
            code: "yass.internal.uncaught",
            message: "line one\nline two\nline three".to_string(),
        };
        let output = el.to_string();
        assert_eq!(
            output,
            "yass: [yass.internal.uncaught] line one line two line three\n"
        );
    }

    #[test]
    fn test_carriage_return_replacement_in_message() {
        let el = ErrorLine {
            file: None,
            line: None,
            code: "yass.internal.uncaught",
            message: "line one\r\nline two".to_string(),
        };
        let output = el.to_string();
        assert_eq!(
            output,
            "yass: [yass.internal.uncaught] line one  line two\n"
        );
    }

    // ---------------------------------------------------------------
    // format_path tests
    // ---------------------------------------------------------------

    #[test]
    fn test_relative_path_subdirectory() {
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/home/user/project/spec/foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "spec/foo.yass.yaml");
    }

    #[test]
    fn test_relative_path_deep_subdirectory() {
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/home/user/project/a/b/c/d.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "a/b/c/d.yass.yaml");
    }

    #[test]
    fn test_basename_only_when_in_cwd() {
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/home/user/project/foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "foo.yass.yaml");
    }

    #[test]
    fn test_absolute_path_when_outside_cwd() {
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/other/place/foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "/other/place/foo.yass.yaml");
    }

    #[test]
    fn test_absolute_path_sibling_directory() {
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/home/user/other/foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "/home/user/other/foo.yass.yaml");
    }

    #[test]
    fn test_absolute_path_parent_directory() {
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/home/user/foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "/home/user/foo.yass.yaml");
    }

    #[test]
    fn test_no_leading_dot_slash() {
        // A relative input path that is under cwd should not get "./" prepended.
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/home/user/project/foo.yass.yaml");
        let result = format_path(&path, &cwd);
        assert!(!result.starts_with("./"), "should not start with ./: {}", result);
    }

    #[test]
    fn test_forward_slashes() {
        // On all platforms, paths should use forward slashes.
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/home/user/project/spec/nested/foo.yass.yaml");
        let result = format_path(&path, &cwd);
        assert!(!result.contains('\\'), "should not contain backslash: {}", result);
        assert_eq!(result, "spec/nested/foo.yass.yaml");
    }

    #[test]
    fn test_cwd_prefix_no_false_match() {
        // /home/user/project-extra should NOT be considered under /home/user/project.
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("/home/user/project-extra/foo.yass.yaml");
        assert_eq!(
            format_path(&path, &cwd),
            "/home/user/project-extra/foo.yass.yaml"
        );
    }

    #[test]
    fn test_relative_input_path() {
        // When given a relative path, lexical_absolute joins it with cwd.
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("spec/foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "spec/foo.yass.yaml");
    }

    #[test]
    fn test_relative_input_basename() {
        let cwd = PathBuf::from("/home/user/project");
        let path = PathBuf::from("foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "foo.yass.yaml");
    }

    #[test]
    fn test_cwd_with_trailing_slash() {
        // Ensure trailing slash on cwd does not break anything.
        let cwd = PathBuf::from("/home/user/project/");
        let path = PathBuf::from("/home/user/project/spec/foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "spec/foo.yass.yaml");
    }

    // ---------------------------------------------------------------
    // Helper for tests that need a specific cwd without env mutation
    // ---------------------------------------------------------------

    /// Format an ErrorLine using a specific cwd (bypassing std::env::current_dir).
    fn format_error_line_with_cwd(el: &ErrorLine, cwd: &Path) -> String {
        let file_str = match &el.file {
            None => "yass".to_string(),
            Some(path) => format_path(path, cwd),
        };
        let sanitized_message = el.message.replace('\n', " ").replace('\r', " ");
        match el.line {
            Some(line) => format!("{}:{}: [{}] {}\n", file_str, line, el.code, sanitized_message),
            None => format!("{}: [{}] {}\n", file_str, el.code, sanitized_message),
        }
    }

    #[test]
    fn test_exactly_one_lf_terminator() {
        let el = ErrorLine {
            file: None,
            line: None,
            code: "yass.argv.no_subcommand",
            message: "no subcommand given".to_string(),
        };
        let output = el.to_string();
        assert!(output.ends_with('\n'), "must end with LF");
        assert!(!output.ends_with("\n\n"), "must not end with double LF");
        // Count newlines — should be exactly 1.
        assert_eq!(
            output.chars().filter(|c| *c == '\n').count(),
            1,
            "must contain exactly one LF"
        );
    }

    #[test]
    fn test_no_ansi_escape_codes() {
        let el = ErrorLine {
            file: Some(PathBuf::from("foo.yass.yaml")),
            line: Some(1),
            code: "yass.yaml.malformed",
            message: "YAML well-formedness error".to_string(),
        };
        let output = el.to_string();
        assert!(!output.contains('\x1b'), "must not contain ANSI escape");
    }

    #[test]
    fn test_root_cwd() {
        let cwd = PathBuf::from("/");
        let path = PathBuf::from("/foo.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "foo.yass.yaml");
    }

    #[test]
    fn test_root_cwd_nested() {
        let cwd = PathBuf::from("/");
        let path = PathBuf::from("/a/b/c.yass.yaml");
        assert_eq!(format_path(&path, &cwd), "a/b/c.yass.yaml");
    }
}

// Query subcommand: retrieves a spec by name and emits it as YAML.

use std::io::Write;
use std::path::{Path, PathBuf};

use unicode_normalization::UnicodeNormalization;
use yaml_rust2::yaml::Hash;
use yaml_rust2::Yaml;

use crate::error_line;
use crate::errors::{self, CliError};
use crate::list;
use crate::shared;
use crate::yaml_parse::{self, ParsedDoc};

/// A matched spec with its metadata.
struct MatchedSpec {
    file_path: PathBuf,
    spec_name: String,
    doc_index: usize,
    description: String,
}

/// Dispatch wrapper for argv compatibility.
pub fn run(args: &[String]) -> i32 {
    let cwd = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    // Skip "yass" and "query" prefixes; next is name, then optional scope.
    let name = match args.get(2) {
        Some(n) => n.as_str(),
        None => {
            let e = CliError::new(errors::QUERY_NAME_MISSING, "missing spec name");
            error_line::emit_error(&e, &cwd);
            return errors::EXIT_USAGE;
        }
    };
    let scope = args.get(3).map(|s| s.as_str());
    run_query(name, scope, &cwd)
}

/// Run the query subcommand. Returns an exit code.
pub fn run_query(name: &str, scope: Option<&str>, cwd: &Path) -> i32 {
    // 1. Validate name: empty -> name_blank
    if name.is_empty() {
        let e = CliError::new(errors::QUERY_NAME_BLANK, "spec name is blank");
        error_line::emit_error(&e, cwd);
        return errors::EXIT_USAGE;
    }

    // 2. Validate scope path first (scope errors take priority)
    if let Some(sp) = scope {
        let resolved = if Path::new(sp).is_absolute() {
            PathBuf::from(sp)
        } else {
            cwd.join(sp)
        };
        if !resolved.exists() {
            let e = CliError::new(
                errors::QUERY_SCOPE_NOT_FOUND,
                format!("scope path does not exist: {}", sp),
            );
            error_line::emit_error(&e, cwd);
            return errors::EXIT_USAGE;
        }
    }

    // 3. Find project root
    let project_root = match shared::find_project_root(cwd) {
        Ok(root) => root,
        Err(e) => {
            error_line::emit_error(&e, cwd);
            return e.exit_code;
        }
    };

    // 4. Discover spec files in scope
    let scope_path = scope.map(|s| {
        if Path::new(s).is_absolute() {
            PathBuf::from(s)
        } else {
            cwd.join(s)
        }
    });
    let scope_ref = scope_path.as_deref();

    let files = match shared::discover_spec_files(scope_ref, &project_root, cwd) {
        Ok(files) => files,
        Err(errs) => {
            for e in &errs {
                error_line::emit_error(e, cwd);
            }
            return errs[0].exit_code;
        }
    };

    // Check scope_empty: if scope was provided and zero files found
    if scope.is_some() && files.is_empty() {
        let e = CliError::new(
            errors::QUERY_SCOPE_EMPTY,
            format!("no .yass.yaml files found in scope: {}", scope.unwrap()),
        );
        error_line::emit_error(&e, cwd);
        return errors::EXIT_USAGE;
    }

    // 5. Look up spec by name
    let matches = name_lookup(name, &files, cwd);

    // 6. Based on match count
    match matches.len() {
        0 => {
            let e = CliError::new(
                errors::QUERY_NO_MATCH,
                format!("no spec matches: {}", name),
            );
            error_line::emit_error(&e, cwd);
            errors::EXIT_PROCESSING
        }
        1 => {
            // Single match -> emit fragment with CONFORMS inlining
            let matched = &matches[0];
            emit_fragment(matched, &project_root, cwd)
        }
        _ => {
            // Multi-match -> emit disambiguation rows
            let stdout = std::io::stdout();
            let mut out = stdout.lock();
            for m in &matches {
                let display_path = error_line::make_display_path(&m.file_path, cwd);
                let row = list::format_list_row(&display_path, &m.spec_name, &m.description);
                let _ = writeln!(out, "{}", row);
            }
            errors::EXIT_SUCCESS
        }
    }
}

/// Perform name lookup across discovered files.
fn name_lookup(query: &str, files: &[PathBuf], cwd: &Path) -> Vec<MatchedSpec> {
    let mut matches = Vec::new();

    // If query contains whitespace, treat as no-match (not blank error)
    if query.chars().any(|c| c.is_whitespace()) {
        return matches;
    }

    for file_path in files {
        // Resolve to absolute path for reading if needed
        let abs_path = if file_path.is_absolute() {
            file_path.clone()
        } else {
            cwd.join(file_path)
        };

        let bytes = match std::fs::read(&abs_path) {
            Ok(b) => b,
            Err(_) => continue,
        };
        let content = match std::str::from_utf8(&bytes) {
            Ok(s) => s,
            Err(_) => continue,
        };
        let docs = match yaml_parse::parse_documents(content) {
            Ok(d) => d,
            Err(_) => continue,
        };

        let description = extract_description(&docs);

        for (i, doc) in docs.iter().enumerate() {
            if let Yaml::Hash(h) = &doc.content {
                if let Some(spec_name) = h
                    .get(&Yaml::String("spec".to_string()))
                    .and_then(|v| v.as_str())
                {
                    if matches_name(spec_name, query) {
                        matches.push(MatchedSpec {
                            file_path: file_path.clone(),
                            spec_name: spec_name.to_string(),
                            doc_index: i,
                            description: description.clone(),
                        });
                    }
                }
            }
        }
    }

    matches
}

/// Check if a spec name matches the query.
/// Matches full name or any dot-aligned trailing suffix.
fn matches_name(spec_name: &str, query: &str) -> bool {
    // Exact match (case-sensitive byte comparison)
    if spec_name == query {
        return true;
    }
    // Trailing suffix match: query equals spec_name with zero or more leading
    // dot-separated components removed. Must be dot-aligned and reach end.
    if spec_name.len() > query.len() {
        let offset = spec_name.len() - query.len();
        if spec_name[offset..] == *query && spec_name.as_bytes()[offset - 1] == b'.' {
            return true;
        }
    }
    false
}

/// Extract and normalize preamble description.
fn extract_description(docs: &[ParsedDoc]) -> String {
    if docs.is_empty() {
        return String::new();
    }
    // The preamble is the first document that does NOT have a "spec" key
    if let Yaml::Hash(h) = &docs[0].content {
        if h.contains_key(&Yaml::String("spec".to_string())) {
            return String::new();
        }
        if let Some(desc) = h.get(&Yaml::String("description".to_string())) {
            if let Some(s) = desc.as_str() {
                return normalize_description(s);
            }
        }
    }
    String::new()
}

/// Normalize description whitespace.
fn normalize_description(desc: &str) -> String {
    let nfc: String = desc.nfc().collect();
    nfc.split_whitespace().collect::<Vec<&str>>().join(" ")
}

/// Emit a YAML fragment for a single matched spec.
fn emit_fragment(matched: &MatchedSpec, project_root: &Path, cwd: &Path) -> i32 {
    // Resolve to absolute path for reading
    let abs_path = if matched.file_path.is_absolute() {
        matched.file_path.clone()
    } else {
        cwd.join(&matched.file_path)
    };

    let bytes = match std::fs::read(&abs_path) {
        Ok(b) => b,
        Err(_) => {
            let e = CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("cannot read file: {}", matched.file_path.display()),
            );
            error_line::emit_error(&e, cwd);
            return errors::EXIT_PROCESSING;
        }
    };
    let content = match std::str::from_utf8(&bytes) {
        Ok(s) => s,
        Err(_) => {
            return errors::EXIT_PROCESSING;
        }
    };
    let docs = match yaml_parse::parse_documents(content) {
        Ok(d) => d,
        Err(_) => {
            return errors::EXIT_PROCESSING;
        }
    };

    if matched.doc_index >= docs.len() {
        return errors::EXIT_PROCESSING;
    }

    let doc = &docs[matched.doc_index];

    // Inline CONFORMS refs
    match inline_conforms(doc, &abs_path, project_root, cwd) {
        Ok(output) => {
            print!("{}", output);
            errors::EXIT_SUCCESS
        }
        Err(errs) => {
            for e in &errs {
                error_line::emit_error(e, cwd);
            }
            errors::EXIT_PROCESSING
        }
    }
}

/// Inline CONFORMS refs and emit the YAML fragment.
fn inline_conforms(
    doc: &ParsedDoc,
    file_path: &Path,
    project_root: &Path,
    cwd: &Path,
) -> Result<String, Vec<CliError>> {
    let hash = match &doc.content {
        Yaml::Hash(h) => h,
        _ => return Ok("---\n".to_string()),
    };

    let display_path = error_line::make_display_path(file_path, cwd);
    let mut errors = Vec::new();
    let mut output = String::from("---\n");

    // Emit spec key first
    if let Some(name) = hash.get(&Yaml::String("spec".to_string())) {
        output.push_str(&format!("spec: {}\n", emit_scalar(name)));
    }

    // Emit each slot in document order
    for (key, value) in hash.iter() {
        let key_str = match key.as_str() {
            Some(s) => s,
            None => continue,
        };
        if key_str == "spec" {
            continue;
        }
        if !crate::errors::SLOT_KEYWORDS.contains(&key_str) {
            continue;
        }

        output.push_str(&format!("{}:\n", key_str));

        if let Yaml::Array(obligations) = value {
            for ob in obligations {
                if let Yaml::Hash(ob_hash) = ob {
                    // Check for CONFORMS ref
                    let conforms_target = find_conforms_ref(ob_hash);
                    let has_normativity = has_normativity_key(ob_hash);
                    let carrier_when = get_when_guard(ob_hash);

                    if let Some(target) = &conforms_target {
                        // Resolve the CONFORMS ref
                        match resolve_conforms(target, file_path, project_root) {
                            Ok(inlined_obs) => {
                                if has_normativity {
                                    // Keep the normative obligation, strip CONFORMS
                                    emit_obligation(&mut output, ob_hash, true);
                                }
                                // Emit inlined obligations with provenance
                                for inlined_ob in &inlined_obs {
                                    output.push_str(&format!("# CONFORMS: {}\n", target));
                                    emit_inlined_obligation(
                                        &mut output,
                                        inlined_ob,
                                        carrier_when.as_deref(),
                                    );
                                }
                            }
                            Err(e) => {
                                errors.push(e.with_file(display_path.clone()));
                            }
                        }
                    } else {
                        // No CONFORMS -- emit obligation as-is
                        emit_obligation(&mut output, ob_hash, false);
                    }
                }
            }
        }
    }

    if !errors.is_empty() {
        return Err(errors);
    }

    // Ensure trailing LF
    if !output.ends_with('\n') {
        output.push('\n');
    }

    Ok(output)
}

/// Find a CONFORMS ref in an obligation hash.
fn find_conforms_ref(hash: &Hash) -> Option<String> {
    hash.get(&Yaml::String("CONFORMS".to_string()))
        .and_then(|v| v.as_str())
        .map(|s| s.to_string())
}

/// Check if obligation has a normativity keyword.
fn has_normativity_key(hash: &Hash) -> bool {
    for key in hash.keys() {
        if let Some(k) = key.as_str() {
            if crate::errors::NORMATIVITY_KEYWORDS.contains(&k) {
                return true;
            }
        }
    }
    false
}

/// Get the WHEN guard value from an obligation.
fn get_when_guard(hash: &Hash) -> Option<String> {
    hash.get(&Yaml::String("WHEN".to_string()))
        .and_then(|v| v.as_str())
        .map(|s| s.to_string())
}

/// Resolve a CONFORMS ref target and return the obligations from the referenced slot.
fn resolve_conforms(
    target: &str,
    file_path: &Path,
    project_root: &Path,
) -> Result<Vec<Yaml>, CliError> {
    // Parse the ref target
    let (path_part, spec_name, slot_part) = parse_ref_target(target);

    // CONFORMS must have a ::SLOT suffix
    let slot = match slot_part {
        Some(s) => s,
        None => {
            return Err(CliError::new(
                errors::QUERY_CONFORMS_NO_SLOT,
                format!("CONFORMS ref must address a slot in v1: {}", target),
            ));
        }
    };

    // Resolve the file and parse documents
    let target_docs = if let Some(path_token) = &path_part {
        let target_file = resolve_ref_path(path_token, file_path, project_root);
        let bytes = std::fs::read(&target_file).map_err(|_| {
            CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("unresolvable CONFORMS ref: {}", target),
            )
        })?;
        let content = std::str::from_utf8(&bytes).map_err(|_| {
            CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("unresolvable CONFORMS ref: {}", target),
            )
        })?;
        yaml_parse::parse_documents(content).map_err(|_| {
            CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("unresolvable CONFORMS ref: {}", target),
            )
        })?
    } else {
        // Same file
        let bytes = std::fs::read(file_path).map_err(|_| {
            CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("unresolvable CONFORMS ref: {}", target),
            )
        })?;
        let content = std::str::from_utf8(&bytes).map_err(|_| {
            CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("unresolvable CONFORMS ref: {}", target),
            )
        })?;
        yaml_parse::parse_documents(content).map_err(|_| {
            CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("unresolvable CONFORMS ref: {}", target),
            )
        })?
    };

    // Find the spec in the target documents
    let spec_doc = target_docs
        .iter()
        .find(|d| {
            if let Yaml::Hash(h) = &d.content {
                h.get(&Yaml::String("spec".to_string()))
                    .and_then(|v| v.as_str())
                    == Some(&spec_name)
            } else {
                false
            }
        })
        .ok_or_else(|| {
            CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("unresolvable CONFORMS ref: {}", target),
            )
        })?;

    // Extract the slot's obligations
    if let Yaml::Hash(h) = &spec_doc.content {
        if let Some(Yaml::Array(obs)) = h.get(&Yaml::String(slot.clone())) {
            Ok(obs.clone())
        } else {
            Err(CliError::new(
                errors::QUERY_CONFORMS_UNRESOLVED,
                format!("unresolvable CONFORMS ref: {}", target),
            ))
        }
    } else {
        Err(CliError::new(
            errors::QUERY_CONFORMS_UNRESOLVED,
            format!("unresolvable CONFORMS ref: {}", target),
        ))
    }
}

/// Parse a ref target string into (path, spec_name, slot).
fn parse_ref_target(target: &str) -> (Option<String>, String, Option<String>) {
    let (rest, slot) = if let Some(idx) = target.find("::") {
        (&target[..idx], Some(target[idx + 2..].to_string()))
    } else {
        (target, None)
    };

    let (path, spec) = if let Some(idx) = rest.find('@') {
        (Some(rest[..idx].to_string()), rest[idx + 1..].to_string())
    } else {
        (None, rest.to_string())
    };

    (path, spec, slot)
}

/// Resolve a cross-file ref path.
fn resolve_ref_path(path_token: &str, referencing_file: &Path, project_root: &Path) -> PathBuf {
    let with_ext = format!("{}.yass.yaml", path_token);
    if path_token.starts_with("./") || path_token.starts_with("../") {
        let dir = referencing_file.parent().unwrap_or(Path::new("."));
        dir.join(&with_ext)
    } else {
        project_root.join(&with_ext)
    }
}

/// Emit a single obligation to the output string.
fn emit_obligation(output: &mut String, hash: &Hash, strip_conforms: bool) {
    output.push_str("- ");
    let mut first = true;

    // Emit in order: normativity, WHEN, references
    // First pass: normativity keywords
    for (k, v) in hash.iter() {
        if let Some(ks) = k.as_str() {
            if crate::errors::NORMATIVITY_KEYWORDS.contains(&ks) {
                if !first {
                    output.push_str("  ");
                }
                output.push_str(&format!("{}: {}\n", ks, emit_scalar(v)));
                first = false;
            }
        }
    }

    // WHEN guard
    if let Some(when) = hash.get(&Yaml::String("WHEN".to_string())) {
        if !first {
            output.push_str("  ");
        }
        output.push_str(&format!("WHEN: {}\n", emit_scalar(when)));
        first = false;
    }

    // Reference relations
    for (k, v) in hash.iter() {
        if let Some(ks) = k.as_str() {
            if crate::errors::REFERENCE_KEYWORDS.contains(&ks) {
                if strip_conforms && ks == "CONFORMS" {
                    continue;
                }
                if !first {
                    output.push_str("  ");
                }
                output.push_str(&format!("{}: {}\n", ks, emit_scalar(v)));
                first = false;
            }
        }
    }
}

/// Emit an inlined obligation with optional carrier WHEN guard.
fn emit_inlined_obligation(output: &mut String, ob: &Yaml, carrier_when: Option<&str>) {
    if let Yaml::Hash(hash) = ob {
        output.push_str("- ");
        let mut first = true;

        // Normativity keywords first
        for (k, v) in hash.iter() {
            if let Some(ks) = k.as_str() {
                if crate::errors::NORMATIVITY_KEYWORDS.contains(&ks) {
                    if !first {
                        output.push_str("  ");
                    }
                    output.push_str(&format!("{}: {}\n", ks, emit_scalar(v)));
                    first = false;
                }
            }
        }

        // WHEN guard -- combine carrier and inner if both present
        let inner_when = get_when_guard(hash);
        let combined_when = match (carrier_when, inner_when.as_deref()) {
            (Some(outer), Some(inner)) => Some(format!("{} and {}", outer, inner)),
            (Some(outer), None) => Some(outer.to_string()),
            (None, Some(inner)) => Some(inner.to_string()),
            (None, None) => None,
        };
        if let Some(when) = combined_when {
            if !first {
                output.push_str("  ");
            }
            output.push_str(&format!("WHEN: {}\n", emit_yaml_string(&when)));
            first = false;
        }

        // Reference relations (do NOT inline CONFORMS from referenced obligations)
        for (k, v) in hash.iter() {
            if let Some(ks) = k.as_str() {
                if crate::errors::REFERENCE_KEYWORDS.contains(&ks) && ks != "CONFORMS" {
                    if !first {
                        output.push_str("  ");
                    }
                    output.push_str(&format!("{}: {}\n", ks, emit_scalar(v)));
                    first = false;
                }
            }
        }
    }
}

/// Emit a YAML scalar value according to the OutputProfile rules.
fn emit_scalar(value: &Yaml) -> String {
    match value {
        Yaml::String(s) => emit_yaml_string(s),
        Yaml::Integer(i) => i.to_string(),
        Yaml::Real(r) => r.clone(),
        Yaml::Boolean(b) => b.to_string(),
        Yaml::Null => "null".to_string(),
        _ => format!("{:?}", value),
    }
}

/// Emit a string scalar, quoting when necessary per OutputProfile.
fn emit_yaml_string(s: &str) -> String {
    if needs_quoting(s) {
        format!("\"{}\"", s.replace('\\', "\\\\").replace('"', "\\\""))
    } else {
        s.to_string()
    }
}

/// Check if a string scalar needs double-quoting.
fn needs_quoting(s: &str) -> bool {
    if s.is_empty() {
        return true;
    }

    // Contains ": " (colon-space)
    if s.contains(": ") {
        return true;
    }

    // Leading special characters: ? - * & ! | > % @
    let first = s.chars().next().unwrap();
    if "?-*&!|>%@".contains(first) {
        return true;
    }

    // Leading or trailing whitespace
    if s.starts_with(' ') || s.starts_with('\t') || s.ends_with(' ') || s.ends_with('\t') {
        return true;
    }

    // YAML 1.2 core schema type tokens
    let lower = s.to_lowercase();
    if matches!(
        lower.as_str(),
        "true" | "false" | "null" | "yes" | "no" | "on" | "off"
    ) {
        return true;
    }

    // Numeric literals
    if s.parse::<i64>().is_ok() || s.parse::<f64>().is_ok() {
        return true;
    }

    // Special YAML floats
    if matches!(
        lower.as_str(),
        ".inf" | "-.inf" | "+.inf" | ".nan"
    ) {
        return true;
    }

    // Hex, octal, binary prefixes
    if s.starts_with("0x") || s.starts_with("0o") || s.starts_with("0b") {
        return true;
    }

    // Contains newlines
    if s.contains('\n') || s.contains('\r') {
        return true;
    }

    false
}

// ─── Tests ───────────────────────────────────────────────────────────────────

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    // ---------------------------------------------------------------
    // Helper functions
    // ---------------------------------------------------------------

    /// Create a tempdir with .git marker (acts as project root).
    fn setup_root() -> TempDir {
        let tmp = TempDir::new().unwrap();
        fs::create_dir(tmp.path().join(".git")).unwrap();
        tmp
    }

    /// Write a file at rel path under base, creating parent dirs.
    fn write_file(base: &Path, rel: &str, content: &str) {
        let full = base.join(rel);
        if let Some(parent) = full.parent() {
            fs::create_dir_all(parent).unwrap();
        }
        fs::write(&full, content).unwrap();
    }

    /// Create a minimal spec file with one spec.
    fn simple_spec(spec_name: &str) -> String {
        format!(
            "---\ndescription: test desc\nversion: v1\n---\nspec: {}\nINPUT:\n- MUST: do something\n",
            spec_name
        )
    }

    /// Create a spec file with a preamble and multiple specs.
    fn multi_spec_file(desc: &str, specs: &[(&str, &str)]) -> String {
        let mut content = format!("---\ndescription: {}\nversion: v1\n", desc);
        for (name, body) in specs {
            content.push_str(&format!("---\nspec: {}\n{}", name, body));
        }
        content
    }

    // ---------------------------------------------------------------
    // matches_name unit tests
    // ---------------------------------------------------------------

    #[test]
    fn test_matches_name_exact() {
        assert!(matches_name("cli.Dispatch", "cli.Dispatch"));
    }

    #[test]
    fn test_matches_name_suffix_one_component() {
        assert!(matches_name("cli.Dispatch", "Dispatch"));
    }

    #[test]
    fn test_matches_name_suffix_two_components() {
        assert!(matches_name("pkg.sub.Spec", "sub.Spec"));
    }

    #[test]
    fn test_matches_name_suffix_last_component() {
        assert!(matches_name("pkg.sub.Spec", "Spec"));
    }

    #[test]
    fn test_matches_name_full_three_component() {
        assert!(matches_name("pkg.sub.Spec", "pkg.sub.Spec"));
    }

    #[test]
    fn test_matches_name_no_partial_substring() {
        // "ispatch" is a partial substring, not dot-aligned
        assert!(!matches_name("cli.Dispatch", "ispatch"));
    }

    #[test]
    fn test_matches_name_no_prefix_match() {
        // "cli" is a prefix, not a trailing suffix
        assert!(!matches_name("cli.Dispatch", "cli"));
    }

    #[test]
    fn test_matches_name_no_partial_component() {
        // "Dis" does not align to a dot boundary at the end
        assert!(!matches_name("cli.Dispatch", "Dis"));
    }

    #[test]
    fn test_matches_name_no_middle_component() {
        // "cli.Dis" is a prefix, not a suffix
        assert!(!matches_name("cli.Dispatch", "cli.Dis"));
    }

    #[test]
    fn test_matches_name_case_sensitive() {
        assert!(!matches_name("cli.Dispatch", "dispatch"));
        assert!(!matches_name("cli.Dispatch", "CLI.Dispatch"));
        assert!(!matches_name("cli.Dispatch", "DISPATCH"));
    }

    #[test]
    fn test_matches_name_single_component_exact() {
        assert!(matches_name("MySpec", "MySpec"));
    }

    #[test]
    fn test_matches_name_empty_query() {
        // Empty query should not match anything
        assert!(!matches_name("cli.Dispatch", ""));
    }

    #[test]
    fn test_matches_name_query_longer_than_spec() {
        assert!(!matches_name("Spec", "cli.Spec"));
    }

    #[test]
    fn test_matches_name_dot_at_start_of_spec() {
        // Edge case: spec name ".Spec" with query "Spec" is dot-aligned
        // (the character before "Spec" is '.'), so it matches.
        assert!(matches_name(".Spec", "Spec"));
    }

    // ---------------------------------------------------------------
    // needs_quoting unit tests
    // ---------------------------------------------------------------

    #[test]
    fn test_needs_quoting_plain_text() {
        assert!(!needs_quoting("hello world"));
        assert!(!needs_quoting("do something"));
        assert!(!needs_quoting("accept a spec name string"));
    }

    #[test]
    fn test_needs_quoting_colon_space() {
        assert!(needs_quoting("foo: bar"));
        assert!(needs_quoting("contains: colon space"));
    }

    #[test]
    fn test_needs_quoting_leading_special_chars() {
        assert!(needs_quoting("?question"));
        assert!(needs_quoting("-dash"));
        assert!(needs_quoting("*star"));
        assert!(needs_quoting("&anchor"));
        assert!(needs_quoting("!bang"));
        assert!(needs_quoting("|pipe"));
        assert!(needs_quoting(">greater"));
        assert!(needs_quoting("%percent"));
        assert!(needs_quoting("@at"));
    }

    #[test]
    fn test_needs_quoting_yaml_type_tokens() {
        assert!(needs_quoting("true"));
        assert!(needs_quoting("false"));
        assert!(needs_quoting("null"));
        assert!(needs_quoting("yes"));
        assert!(needs_quoting("no"));
        assert!(needs_quoting("on"));
        assert!(needs_quoting("off"));
        assert!(needs_quoting("True"));
        assert!(needs_quoting("FALSE"));
        assert!(needs_quoting("Yes"));
        assert!(needs_quoting("NULL"));
    }

    #[test]
    fn test_needs_quoting_numeric() {
        assert!(needs_quoting("42"));
        assert!(needs_quoting("3.14"));
        assert!(needs_quoting("0"));
        assert!(needs_quoting("-1"));
    }

    #[test]
    fn test_needs_quoting_special_yaml_floats() {
        assert!(needs_quoting(".inf"));
        assert!(needs_quoting(".nan"));
        assert!(needs_quoting("-.inf"));
        assert!(needs_quoting("+.inf"));
    }

    #[test]
    fn test_needs_quoting_hex_octal_binary() {
        assert!(needs_quoting("0x1F"));
        assert!(needs_quoting("0o777"));
        assert!(needs_quoting("0b1010"));
    }

    #[test]
    fn test_needs_quoting_leading_trailing_whitespace() {
        assert!(needs_quoting(" leading"));
        assert!(needs_quoting("trailing "));
        assert!(needs_quoting("\tleading tab"));
        assert!(needs_quoting("trailing tab\t"));
    }

    #[test]
    fn test_needs_quoting_empty_string() {
        assert!(needs_quoting(""));
    }

    #[test]
    fn test_needs_quoting_newlines() {
        assert!(needs_quoting("line1\nline2"));
        assert!(needs_quoting("line1\rline2"));
    }

    // ---------------------------------------------------------------
    // emit_yaml_string unit tests
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_yaml_string_plain() {
        assert_eq!(emit_yaml_string("hello world"), "hello world");
    }

    #[test]
    fn test_emit_yaml_string_quoted() {
        assert_eq!(emit_yaml_string("foo: bar"), "\"foo: bar\"");
    }

    #[test]
    fn test_emit_yaml_string_with_quotes() {
        // "say \"hello\"" does not trigger any quoting rule (no colon-space,
        // no leading special char, etc.) so it stays plain.
        assert_eq!(emit_yaml_string("say \"hello\""), "say \"hello\"");
    }

    #[test]
    fn test_emit_yaml_string_with_backslash() {
        // "path\to" does not trigger any quoting rule, stays plain.
        assert_eq!(emit_yaml_string("path\\to"), "path\\to");
    }

    #[test]
    fn test_emit_yaml_string_escapes_when_quoting_needed() {
        // "contains: colon space" triggers quoting; internal quotes/backslash escaped.
        assert_eq!(
            emit_yaml_string("has: \"quotes\""),
            "\"has: \\\"quotes\\\"\""
        );
    }

    // ---------------------------------------------------------------
    // parse_ref_target unit tests
    // ---------------------------------------------------------------

    #[test]
    fn test_parse_ref_target_simple() {
        let (p, s, sl) = parse_ref_target("Foo");
        assert_eq!(p, None);
        assert_eq!(s, "Foo");
        assert_eq!(sl, None);
    }

    #[test]
    fn test_parse_ref_target_with_slot() {
        let (p, s, sl) = parse_ref_target("Foo::RETURN");
        assert_eq!(p, None);
        assert_eq!(s, "Foo");
        assert_eq!(sl, Some("RETURN".to_string()));
    }

    #[test]
    fn test_parse_ref_target_cross_file_relative() {
        let (p, s, sl) = parse_ref_target("./cli@Foo::RETURN");
        assert_eq!(p, Some("./cli".to_string()));
        assert_eq!(s, "Foo");
        assert_eq!(sl, Some("RETURN".to_string()));
    }

    #[test]
    fn test_parse_ref_target_cross_file_parent() {
        let (p, s, sl) = parse_ref_target("../shared@Bar::INPUT");
        assert_eq!(p, Some("../shared".to_string()));
        assert_eq!(s, "Bar");
        assert_eq!(sl, Some("INPUT".to_string()));
    }

    #[test]
    fn test_parse_ref_target_cross_file_project_root() {
        let (p, s, sl) = parse_ref_target("spec/cli@Foo::ERROR");
        assert_eq!(p, Some("spec/cli".to_string()));
        assert_eq!(s, "Foo");
        assert_eq!(sl, Some("ERROR".to_string()));
    }

    #[test]
    fn test_parse_ref_target_dotted_spec_name() {
        let (p, s, sl) = parse_ref_target("cli.ErrorLine::RETURN");
        assert_eq!(p, None);
        assert_eq!(s, "cli.ErrorLine");
        assert_eq!(sl, Some("RETURN".to_string()));
    }

    #[test]
    fn test_parse_ref_target_cross_file_dotted_spec() {
        let (p, s, sl) = parse_ref_target("./cli@cli.ErrorLine::RETURN");
        assert_eq!(p, Some("./cli".to_string()));
        assert_eq!(s, "cli.ErrorLine");
        assert_eq!(sl, Some("RETURN".to_string()));
    }

    // ---------------------------------------------------------------
    // resolve_ref_path unit tests
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_ref_path_relative_dot() {
        let result = resolve_ref_path("./cli", Path::new("/project/spec/main.yass.yaml"), Path::new("/project"));
        assert_eq!(result, PathBuf::from("/project/spec/./cli.yass.yaml"));
    }

    #[test]
    fn test_resolve_ref_path_relative_dotdot() {
        let result = resolve_ref_path("../shared", Path::new("/project/spec/sub/main.yass.yaml"), Path::new("/project"));
        assert_eq!(result, PathBuf::from("/project/spec/sub/../shared.yass.yaml"));
    }

    #[test]
    fn test_resolve_ref_path_project_root() {
        let result = resolve_ref_path("spec/cli", Path::new("/project/other/main.yass.yaml"), Path::new("/project"));
        assert_eq!(result, PathBuf::from("/project/spec/cli.yass.yaml"));
    }

    // ---------------------------------------------------------------
    // normalize_description unit tests
    // ---------------------------------------------------------------

    #[test]
    fn test_normalize_description_whitespace() {
        assert_eq!(normalize_description("  hello   world  "), "hello world");
    }

    #[test]
    fn test_normalize_description_newlines() {
        assert_eq!(normalize_description("a\n  b\n  c"), "a b c");
    }

    #[test]
    fn test_normalize_description_empty() {
        assert_eq!(normalize_description(""), "");
    }

    #[test]
    fn test_normalize_description_tabs() {
        assert_eq!(normalize_description("a\t\tb"), "a b");
    }

    // ---------------------------------------------------------------
    // Full integration: single match
    // ---------------------------------------------------------------

    #[test]
    fn test_query_single_match_simple() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        let exit = run_query("MySpec", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_query_single_match_by_suffix() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("pkg.MySpec"));

        let exit = run_query("MySpec", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_query_single_match_by_two_component_suffix() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("org.pkg.MySpec"));

        let exit = run_query("pkg.MySpec", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_query_single_match_by_full_name() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("org.pkg.MySpec"));

        let exit = run_query("org.pkg.MySpec", None, root);
        assert_eq!(exit, 0);
    }

    // ---------------------------------------------------------------
    // Full integration: name lookup failures
    // ---------------------------------------------------------------

    #[test]
    fn test_query_no_match() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        let exit = run_query("NonExistent", None, root);
        assert_eq!(exit, 1);
    }

    #[test]
    fn test_query_no_match_partial_substring() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("cli.query"));

        // "uery" is not a dot-aligned suffix
        let exit = run_query("uery", None, root);
        assert_eq!(exit, 1);
    }

    #[test]
    fn test_query_no_match_prefix() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("cli.query"));

        // "cli" is a prefix, not a trailing suffix
        let exit = run_query("cli", None, root);
        assert_eq!(exit, 1);
    }

    #[test]
    fn test_query_name_blank_exit_2() {
        let tmp = setup_root();
        let root = tmp.path();

        let exit = run_query("", None, root);
        assert_eq!(exit, 2);
    }

    #[test]
    fn test_query_name_whitespace_is_no_match_not_blank() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        // Whitespace in name -> treat as no-match, not blank
        let exit = run_query("My Spec", None, root);
        assert_eq!(exit, 1); // no_match (exit 1), not blank (exit 2)
    }

    #[test]
    fn test_query_name_tab_is_no_match_not_blank() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        let exit = run_query("My\tSpec", None, root);
        assert_eq!(exit, 1);
    }

    // ---------------------------------------------------------------
    // Full integration: multi-match disambiguation
    // ---------------------------------------------------------------

    #[test]
    fn test_query_multi_match_exits_0() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "a.yass.yaml", &simple_spec("a.Spec"));
        write_file(root, "b.yass.yaml", &simple_spec("b.Spec"));

        let exit = run_query("Spec", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_query_multi_match_same_file() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = multi_spec_file(
            "test",
            &[
                ("ns1.Spec", "RETURN:\n- MUST: do thing 1\n"),
                ("ns2.Spec", "RETURN:\n- MUST: do thing 2\n"),
            ],
        );
        write_file(root, "test.yass.yaml", &content);

        let exit = run_query("Spec", None, root);
        assert_eq!(exit, 0);
    }

    // ---------------------------------------------------------------
    // Full integration: scope validation
    // ---------------------------------------------------------------

    #[test]
    fn test_query_scope_not_found_exit_2() {
        let tmp = setup_root();
        let root = tmp.path();

        let exit = run_query("anything", Some(root.join("nonexistent").to_str().unwrap()), root);
        assert_eq!(exit, 2);
    }

    #[test]
    fn test_query_scope_empty_exit_2() {
        let tmp = setup_root();
        let root = tmp.path();

        let empty_dir = root.join("empty_scope");
        fs::create_dir(&empty_dir).unwrap();

        let exit = run_query("anything", Some(empty_dir.to_str().unwrap()), root);
        assert_eq!(exit, 2);
    }

    #[test]
    fn test_query_scope_validation_before_name_lookup() {
        let tmp = setup_root();
        let root = tmp.path();

        // Even with a valid spec in the project, scope error takes priority
        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        let exit = run_query("MySpec", Some(root.join("nonexistent").to_str().unwrap()), root);
        assert_eq!(exit, 2); // scope error, not a match
    }

    #[test]
    fn test_query_scope_with_valid_file() {
        let tmp = setup_root();
        let root = tmp.path();

        let scope_dir = root.join("specs");
        fs::create_dir(&scope_dir).unwrap();
        write_file(root, "specs/test.yass.yaml", &simple_spec("scoped.Spec"));

        let exit = run_query("scoped.Spec", Some(scope_dir.to_str().unwrap()), root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_query_scope_filters_to_directory() {
        let tmp = setup_root();
        let root = tmp.path();

        // Spec at root should not be found when scope is a subdir
        write_file(root, "root.yass.yaml", &simple_spec("root.Spec"));

        let scope_dir = root.join("sub");
        fs::create_dir(&scope_dir).unwrap();
        // Empty scope dir -> scope_empty
        let exit = run_query("root.Spec", Some(scope_dir.to_str().unwrap()), root);
        assert_eq!(exit, 2);
    }

    // ---------------------------------------------------------------
    // CONFORMS inlining: same-file
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_same_file_reference_only() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: be awesome
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_conforms_same_file_normative_with_conforms() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- MUST: do my own thing
  CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: be contracted
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_conforms_same_file_multiple_inlined_obligations() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: first obligation
- SHOULD: second obligation
- MAY: third obligation
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);
    }

    // ---------------------------------------------------------------
    // CONFORMS inlining: cross-file
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_cross_file_relative() {
        let tmp = setup_root();
        let root = tmp.path();

        let main_spec = "\
---
description: main
version: v1
---
spec: my.Spec
RETURN:
- CONFORMS: ./contract@my.Contract::RETURN
";
        let contract_spec = "\
---
description: contract
version: v1
---
spec: my.Contract
RETURN:
- MUST: fulfill contract
";
        write_file(root, "main.yass.yaml", main_spec);
        write_file(root, "contract.yass.yaml", contract_spec);

        let exit = run_query("my.Spec", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_conforms_cross_file_project_root() {
        let tmp = setup_root();
        let root = tmp.path();

        let main_spec = "\
---
description: main
version: v1
---
spec: my.Spec
RETURN:
- CONFORMS: contracts/base@base.Contract::RETURN
";
        let contract_spec = "\
---
description: contract
version: v1
---
spec: base.Contract
RETURN:
- MUST: conform to base
";
        write_file(root, "specs/main.yass.yaml", main_spec);
        write_file(root, "contracts/base.yass.yaml", contract_spec);

        let exit = run_query("my.Spec", None, root);
        assert_eq!(exit, 0);
    }

    // ---------------------------------------------------------------
    // CONFORMS error cases
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_no_slot_suffix_exit_1() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 1);
    }

    #[test]
    fn test_conforms_unresolved_spec_exit_1() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: NonExistent::RETURN
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 1);
    }

    #[test]
    fn test_conforms_unresolved_file_exit_1() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: ./missing@Target::RETURN
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 1);
    }

    #[test]
    fn test_conforms_unresolved_slot_exit_1() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::NONEXISTENT
---
spec: Target
RETURN:
- MUST: be something
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 1);
    }

    // ---------------------------------------------------------------
    // Combined WHEN guards
    // ---------------------------------------------------------------

    #[test]
    fn test_combined_when_guards_both_present() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- WHEN: outer condition
  CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- WHEN: inner condition
  MUST: do something
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);
        // The combined WHEN should be "outer condition and inner condition"
    }

    #[test]
    fn test_carrier_when_no_inner_when() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- WHEN: outer condition
  CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: do something without when
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_inner_when_no_carrier_when() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- WHEN: inner condition
  MUST: do something
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);
    }

    // ---------------------------------------------------------------
    // USES and SEE refs are NOT inlined
    // ---------------------------------------------------------------

    #[test]
    fn test_uses_not_inlined() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
INPUT:
- MUST: do something
  USES: some.Ref
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_see_not_inlined() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
INPUT:
- MUST: do something
  SEE: some.Ref
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);
    }

    // ---------------------------------------------------------------
    // Output profile formatting
    // ---------------------------------------------------------------

    #[test]
    fn test_output_starts_with_document_marker() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: MySpec
INPUT:
- MUST: do something
";
        write_file(root, "test.yass.yaml", content);

        // We cannot easily capture stdout in a unit test without redirecting,
        // but we can test the inline_conforms function directly.
        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1]; // skip preamble

        let result = inline_conforms(
            doc,
            &root.join("test.yass.yaml"),
            root,
            root,
        )
        .unwrap();

        assert!(result.starts_with("---\n"));
    }

    #[test]
    fn test_output_no_trailing_dot_marker() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "---\ndescription: test\nversion: v1\n---\nspec: MySpec\nINPUT:\n- MUST: do something\n";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();
        assert!(!result.contains("..."));
    }

    #[test]
    fn test_output_ends_with_single_lf() {
        let content = "---\ndescription: test\nversion: v1\n---\nspec: MySpec\nINPUT:\n- MUST: do something\n";
        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "test.yass.yaml", content);

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();
        assert!(result.ends_with('\n'));
        assert!(!result.ends_with("\n\n"));
    }

    #[test]
    fn test_output_two_space_indent_for_obligations() {
        let content = "---\ndescription: test\nversion: v1\n---\nspec: MySpec\nINPUT:\n- MUST: do something\n  WHEN: a condition\n";
        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "test.yass.yaml", content);

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();
        // The continuation keys after "- " should be indented with 2 spaces
        assert!(result.contains("  WHEN:") || result.contains("- MUST:"));
    }

    #[test]
    fn test_output_quoting_special_values() {
        assert_eq!(emit_yaml_string("true"), "\"true\"");
        assert_eq!(emit_yaml_string("42"), "\"42\"");
        assert_eq!(emit_yaml_string("do something"), "do something");
    }

    #[test]
    fn test_output_preserves_key_ordering() {
        let content = "\
---
description: test
version: v1
---
spec: MySpec
INPUT:
- MUST: accept input
RETURN:
- MUST: return output
ERROR:
- MUST: handle errors
";
        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "test.yass.yaml", content);

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // Check that INPUT appears before RETURN, and RETURN before ERROR
        let input_pos = result.find("INPUT:").unwrap();
        let return_pos = result.find("RETURN:").unwrap();
        let error_pos = result.find("ERROR:").unwrap();
        assert!(input_pos < return_pos);
        assert!(return_pos < error_pos);
    }

    // ---------------------------------------------------------------
    // Provenance comments
    // ---------------------------------------------------------------

    #[test]
    fn test_provenance_comment_format() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: be awesome
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1]; // Source

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();
        assert!(
            result.contains("# CONFORMS: Target::RETURN"),
            "output should contain provenance comment, got:\n{}",
            result
        );
    }

    #[test]
    fn test_provenance_comment_at_column_zero() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: be awesome
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // Check that the provenance comment starts at column zero
        for line in result.lines() {
            if line.starts_with("# CONFORMS:") {
                assert!(
                    !line.starts_with(' '),
                    "provenance comment should be at column zero"
                );
            }
        }
    }

    #[test]
    fn test_provenance_comment_above_inlined_obligation() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: be awesome
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        let lines: Vec<&str> = result.lines().collect();
        // Find the provenance comment and check the next line is the obligation
        for (i, line) in lines.iter().enumerate() {
            if line.starts_with("# CONFORMS:") {
                assert!(
                    i + 1 < lines.len(),
                    "provenance comment should be followed by an obligation"
                );
                assert!(
                    lines[i + 1].starts_with("- "),
                    "line after provenance should start with '- ', got: {}",
                    lines[i + 1]
                );
            }
        }
    }

    // ---------------------------------------------------------------
    // Normative obligation with CONFORMS keeps original + appends
    // ---------------------------------------------------------------

    #[test]
    fn test_normative_with_conforms_keeps_original() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- MUST: do my own thing
  CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: be contracted
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // Should contain the original normative obligation (without CONFORMS)
        assert!(
            result.contains("MUST: do my own thing"),
            "should keep original obligation, got:\n{}",
            result
        );
        // Should also contain the inlined obligation
        assert!(
            result.contains("MUST: be contracted"),
            "should contain inlined obligation, got:\n{}",
            result
        );
        // Original should NOT have CONFORMS anymore (stripped)
        // The CONFORMS should only appear in provenance comments
        let conforms_count = result.matches("CONFORMS:").count();
        let provenance_count = result.matches("# CONFORMS:").count();
        // All CONFORMS occurrences should be in provenance comments
        assert_eq!(
            conforms_count, provenance_count,
            "CONFORMS should only appear in provenance comments, got:\n{}",
            result
        );
    }

    // ---------------------------------------------------------------
    // CONFORMS strips CONFORMS but keeps non-CONFORMS refs
    // ---------------------------------------------------------------

    #[test]
    fn test_strip_conforms_keeps_uses() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- MUST: do something
  CONFORMS: Target::RETURN
  USES: some.Other
---
spec: Target
RETURN:
- MUST: contracted
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // USES should still be present on the original obligation
        assert!(
            result.contains("USES: some.Other"),
            "USES should be preserved, got:\n{}",
            result
        );
    }

    // ---------------------------------------------------------------
    // No preamble or other specs in output
    // ---------------------------------------------------------------

    #[test]
    fn test_output_no_preamble() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: this should not appear
version: v1
---
spec: MySpec
INPUT:
- MUST: do something
---
spec: OtherSpec
INPUT:
- MUST: do other thing
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1]; // MySpec

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        assert!(!result.contains("description:"));
        assert!(!result.contains("version:"));
        assert!(!result.contains("OtherSpec"));
    }

    // ---------------------------------------------------------------
    // No recursion: CONFORMS in inlined obligations not resolved
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_no_recursive_resolution() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Middle::RETURN
---
spec: Middle
RETURN:
- MUST: middle obligation
  CONFORMS: Deep::RETURN
---
spec: Deep
RETURN:
- MUST: deep obligation
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("Source", None, root);
        assert_eq!(exit, 0);

        // The inlined obligation from Middle should NOT resolve its CONFORMS to Deep
        // (only one level deep)
    }

    // ---------------------------------------------------------------
    // Edge cases
    // ---------------------------------------------------------------

    #[test]
    fn test_query_case_sensitive_matching() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("cli.Query"));

        // Lowercase "query" should not match "cli.Query"
        let exit = run_query("query", None, root);
        assert_eq!(exit, 1);
    }

    #[test]
    fn test_query_no_trimming() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        // Name with leading space should not match (whitespace -> no-match)
        let exit = run_query(" MySpec", None, root);
        assert_eq!(exit, 1);
    }

    #[test]
    fn test_query_empty_scope_no_yass_files() {
        let tmp = setup_root();
        let root = tmp.path();

        // Create a directory with non-yass files
        let scope_dir = root.join("scope");
        fs::create_dir(&scope_dir).unwrap();
        fs::write(scope_dir.join("readme.md"), "not a spec").unwrap();
        fs::write(scope_dir.join("config.yaml"), "not a spec").unwrap();

        let exit = run_query("anything", Some(scope_dir.to_str().unwrap()), root);
        assert_eq!(exit, 2); // scope_empty
    }

    #[test]
    fn test_query_multiple_slots() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: FullSpec
INPUT:
- MUST: accept input
RETURN:
- MUST: return output
ERROR:
- MUST: handle errors
SIDE-EFFECT:
- MUST-NOT: modify files
INVARIANT:
- MUST: be consistent
";
        write_file(root, "test.yass.yaml", content);

        let exit = run_query("FullSpec", None, root);
        assert_eq!(exit, 0);
    }

    #[test]
    fn test_query_preserve_normativity_keywords() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: required thing
- SHOULD: recommended thing
- MUST-NOT: forbidden thing
- SHOULD-NOT: discouraged thing
- MAY: optional thing
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        assert!(result.contains("MUST: required thing"));
        assert!(result.contains("SHOULD: recommended thing"));
        assert!(result.contains("MUST-NOT: forbidden thing"));
        assert!(result.contains("SHOULD-NOT: discouraged thing"));
        assert!(result.contains("MAY: optional thing"));
    }

    #[test]
    fn test_obligation_key_order_normativity_when_ref() {
        let content = "---\nspec: T\nRETURN:\n- USES: ref\n  WHEN: cond\n  MUST: do thing\n";
        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[0];

        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "test.yass.yaml", content);

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // In the output, MUST should come before WHEN, and WHEN before USES
        let must_pos = result.find("MUST:").unwrap();
        let when_pos = result.find("WHEN:").unwrap();
        let uses_pos = result.find("USES:").unwrap();
        assert!(
            must_pos < when_pos,
            "MUST should come before WHEN in output"
        );
        assert!(
            when_pos < uses_pos,
            "WHEN should come before USES in output"
        );
    }

    // ---------------------------------------------------------------
    // extract_description tests
    // ---------------------------------------------------------------

    #[test]
    fn test_extract_description_from_preamble() {
        let content = "---\ndescription: A nice description\nversion: v1\n---\nspec: S\n";
        let docs = yaml_parse::parse_documents(content).unwrap();
        assert_eq!(extract_description(&docs), "A nice description");
    }

    #[test]
    fn test_extract_description_multiline() {
        let content = "---\ndescription: >\n  This is a\n  long description\nversion: v1\n---\nspec: S\n";
        let docs = yaml_parse::parse_documents(content).unwrap();
        assert_eq!(extract_description(&docs), "This is a long description");
    }

    #[test]
    fn test_extract_description_missing() {
        let content = "---\nversion: v1\n---\nspec: S\n";
        let docs = yaml_parse::parse_documents(content).unwrap();
        assert_eq!(extract_description(&docs), "");
    }

    #[test]
    fn test_extract_description_no_preamble() {
        // First document has a spec key -> no preamble
        let content = "---\nspec: S\nINPUT:\n- MUST: do thing\n";
        let docs = yaml_parse::parse_documents(content).unwrap();
        assert_eq!(extract_description(&docs), "");
    }

    // ---------------------------------------------------------------
    // Combined WHEN guard string format
    // ---------------------------------------------------------------

    #[test]
    fn test_combined_when_literal_and() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- WHEN: the sky is blue
  CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- WHEN: the grass is green
  MUST: do something
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // The combined WHEN should use " and " (literal, one space each side)
        assert!(
            result.contains("the sky is blue and the grass is green"),
            "combined WHEN should use ' and ', got:\n{}",
            result
        );
    }

    // ---------------------------------------------------------------
    // Disambiguation row format
    // ---------------------------------------------------------------

    #[test]
    fn test_disambiguation_format() {
        // Test list::format_list_row directly
        let row = list::format_list_row("spec/file.yass.yaml", "cli.Spec", "a description");
        assert_eq!(row, "spec/file.yass.yaml\tcli.Spec\ta description");
    }

    // ---------------------------------------------------------------
    // File reading edge cases in name_lookup
    // ---------------------------------------------------------------

    #[test]
    fn test_name_lookup_skips_unparseable_files() {
        let tmp = setup_root();
        let root = tmp.path();

        // Valid spec file
        write_file(root, "good.yass.yaml", &simple_spec("Good.Spec"));
        // Invalid YAML
        write_file(root, "bad.yass.yaml", "---\nkey: [\ninvalid\n");

        let exit = run_query("Good.Spec", None, root);
        assert_eq!(exit, 0);
    }

    // ---------------------------------------------------------------
    // emit_scalar tests
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_scalar_string() {
        assert_eq!(emit_scalar(&Yaml::String("hello".to_string())), "hello");
    }

    #[test]
    fn test_emit_scalar_integer() {
        assert_eq!(emit_scalar(&Yaml::Integer(42)), "42");
    }

    #[test]
    fn test_emit_scalar_boolean() {
        assert_eq!(emit_scalar(&Yaml::Boolean(true)), "true");
    }

    #[test]
    fn test_emit_scalar_null() {
        assert_eq!(emit_scalar(&Yaml::Null), "null");
    }

    // ---------------------------------------------------------------
    // Output content verification via inline_conforms
    // ---------------------------------------------------------------

    #[test]
    fn test_inline_conforms_simple_output() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Simple
INPUT:
- MUST: accept a name
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        assert_eq!(result, "---\nspec: Simple\nINPUT:\n- MUST: accept a name\n");
    }

    #[test]
    fn test_inline_conforms_with_when() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: WithWhen
RETURN:
- WHEN: condition is met
  MUST: do something
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        assert!(result.contains("MUST: do something"));
        assert!(result.contains("WHEN: condition is met"));
    }

    #[test]
    fn test_inline_conforms_quoted_scalar() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Quoted
RETURN:
- MUST: \"emit it: like this\"
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // The value "emit it: like this" contains ": " so should be quoted
        assert!(
            result.contains("\"emit it: like this\""),
            "should quote scalar with colon-space, got:\n{}",
            result
        );
    }

    // ---------------------------------------------------------------
    // run_query: FindProjectRoot failure
    // ---------------------------------------------------------------

    #[test]
    fn test_query_find_project_root_failure() {
        // A tempdir with no .git and no .yass.yaml files anywhere
        // up the tree should fail to find a project root.
        let tmp = TempDir::new().unwrap();
        let isolated = tmp.path().join("no_marker");
        fs::create_dir(&isolated).unwrap();

        // run_query from an isolated dir with no markers should get
        // exit code 2 (findroot.no_marker is a usage error).
        let exit = run_query("SomeSpec", None, &isolated);
        // On CI machines or local dev, there may be a .git above /tmp,
        // which would cause this to find a root and then fail with
        // no_match (exit 1) instead. Accept either outcome.
        assert!(
            exit == 2 || exit == 1,
            "expected exit 1 or 2 for no project root, got {}",
            exit
        );
    }

    // ---------------------------------------------------------------
    // run_query: scope with colon (colon_in_path validation)
    // ---------------------------------------------------------------

    #[test]
    fn test_query_scope_colon_in_path() {
        // A scope path containing a colon triggers scope_not_found
        // because such a path almost certainly does not exist.
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        let exit = run_query("MySpec", Some("foo:bar"), root);
        // "foo:bar" does not exist, so scope_not_found -> exit 2
        assert_eq!(exit, 2);
    }

    // ---------------------------------------------------------------
    // run_query: scope exists but has no .yass.yaml files
    // ---------------------------------------------------------------

    #[test]
    fn test_query_scope_exists_no_yass_files() {
        let tmp = setup_root();
        let root = tmp.path();

        // Create a scope dir with only non-spec files
        let scope = root.join("scope_dir");
        fs::create_dir(&scope).unwrap();
        fs::write(scope.join("readme.txt"), "just text").unwrap();
        fs::write(scope.join("config.yaml"), "not a spec").unwrap();

        let exit = run_query("anything", Some(scope.to_str().unwrap()), root);
        assert_eq!(exit, 2); // scope_empty
    }

    // ---------------------------------------------------------------
    // name_lookup: query with whitespace returns no matches
    // ---------------------------------------------------------------

    #[test]
    fn test_name_lookup_whitespace_returns_empty() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        let files = vec![root.join("test.yass.yaml")];
        let matches = name_lookup("My Spec", &files, root);
        assert!(matches.is_empty(), "whitespace query should return no matches");
    }

    #[test]
    fn test_name_lookup_newline_returns_empty() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "test.yass.yaml", &simple_spec("MySpec"));

        let files = vec![root.join("test.yass.yaml")];
        let matches = name_lookup("My\nSpec", &files, root);
        assert!(matches.is_empty(), "newline in query should return no matches");
    }

    // ---------------------------------------------------------------
    // name_lookup: unreadable file is skipped
    // ---------------------------------------------------------------

    #[test]
    fn test_name_lookup_skips_nonexistent_file() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "good.yass.yaml", &simple_spec("Good"));

        // Include a nonexistent file in the list -- should be silently skipped
        let files = vec![
            root.join("nonexistent.yass.yaml"),
            root.join("good.yass.yaml"),
        ];
        let matches = name_lookup("Good", &files, root);
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].spec_name, "Good");
    }

    // ---------------------------------------------------------------
    // name_lookup: invalid YAML file is skipped
    // ---------------------------------------------------------------

    #[test]
    fn test_name_lookup_skips_invalid_yaml() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "good.yass.yaml", &simple_spec("Good"));
        // Write something that is not valid YAML
        write_file(root, "bad.yass.yaml", "---\n[[[invalid yaml");

        let files = vec![
            root.join("bad.yass.yaml"),
            root.join("good.yass.yaml"),
        ];
        let matches = name_lookup("Good", &files, root);
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].spec_name, "Good");
    }

    // ---------------------------------------------------------------
    // name_lookup: non-UTF8 file is skipped
    // ---------------------------------------------------------------

    #[test]
    fn test_name_lookup_skips_non_utf8_file() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "good.yass.yaml", &simple_spec("Good"));

        // Write invalid UTF-8 bytes
        let bad_path = root.join("bad.yass.yaml");
        fs::write(&bad_path, &[0xFF, 0xFE, 0x00, 0x01]).unwrap();

        let files = vec![
            bad_path,
            root.join("good.yass.yaml"),
        ];
        let matches = name_lookup("Good", &files, root);
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0].spec_name, "Good");
    }

    // ---------------------------------------------------------------
    // emit_fragment: file read failure
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_fragment_file_read_failure() {
        let tmp = setup_root();
        let root = tmp.path();

        let matched = MatchedSpec {
            file_path: PathBuf::from("nonexistent.yass.yaml"),
            spec_name: "Ghost".to_string(),
            doc_index: 0,
            description: String::new(),
        };

        let exit = emit_fragment(&matched, root, root);
        assert_eq!(exit, 1); // EXIT_PROCESSING
    }

    // ---------------------------------------------------------------
    // emit_fragment: UTF-8 decode failure
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_fragment_utf8_failure() {
        let tmp = setup_root();
        let root = tmp.path();

        // Write invalid UTF-8 bytes to a file
        let bad_file = root.join("bad.yass.yaml");
        fs::write(&bad_file, &[0xFF, 0xFE, 0x80, 0x81]).unwrap();

        let matched = MatchedSpec {
            file_path: bad_file,
            spec_name: "Bad".to_string(),
            doc_index: 0,
            description: String::new(),
        };

        let exit = emit_fragment(&matched, root, root);
        assert_eq!(exit, 1); // EXIT_PROCESSING
    }

    // ---------------------------------------------------------------
    // emit_fragment: YAML parse failure
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_fragment_yaml_parse_failure() {
        let tmp = setup_root();
        let root = tmp.path();

        let bad_file = root.join("bad.yass.yaml");
        fs::write(&bad_file, "---\n[[[not valid yaml").unwrap();

        let matched = MatchedSpec {
            file_path: bad_file,
            spec_name: "Bad".to_string(),
            doc_index: 0,
            description: String::new(),
        };

        let exit = emit_fragment(&matched, root, root);
        assert_eq!(exit, 1); // EXIT_PROCESSING
    }

    // ---------------------------------------------------------------
    // emit_fragment: doc index out of bounds
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_fragment_doc_index_out_of_bounds() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "---\nspec: OnlySpec\nINPUT:\n- MUST: do thing\n";
        write_file(root, "test.yass.yaml", content);

        let matched = MatchedSpec {
            file_path: root.join("test.yass.yaml"),
            spec_name: "OnlySpec".to_string(),
            doc_index: 99, // way out of bounds
            description: String::new(),
        };

        let exit = emit_fragment(&matched, root, root);
        assert_eq!(exit, 1); // EXIT_PROCESSING
    }

    // ---------------------------------------------------------------
    // inline_conforms: reference-only CONFORMS (no normativity)
    // replaces entirely with inlined obligations
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_reference_only_replaces_with_inlined() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- MUST: be inlined
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1]; // Source

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // Should NOT contain a bare CONFORMS key (only in provenance comment)
        assert!(result.contains("# CONFORMS: Target::RETURN"));
        assert!(result.contains("MUST: be inlined"));
        // The original CONFORMS-only obligation should be gone; only inlined
        let lines: Vec<&str> = result.lines().collect();
        let obligation_lines: Vec<&&str> = lines.iter().filter(|l| l.starts_with("- ")).collect();
        assert_eq!(obligation_lines.len(), 1, "should have exactly one obligation (inlined)");
        assert!(obligation_lines[0].contains("MUST: be inlined"));
    }

    // ---------------------------------------------------------------
    // inline_conforms: normative with CONFORMS keeps both
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_normative_keeps_original_and_appends() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- MUST: my own obligation
  CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- SHOULD: from target
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // Both obligations should be present
        assert!(result.contains("MUST: my own obligation"));
        assert!(result.contains("SHOULD: from target"));
        // CONFORMS should only appear in provenance comment
        let conforms_count = result.matches("CONFORMS:").count();
        let provenance_count = result.matches("# CONFORMS:").count();
        assert_eq!(conforms_count, provenance_count);
    }

    // ---------------------------------------------------------------
    // inline_conforms: combined WHEN guards (outer + inner)
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_combined_when_guards_output_verified() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- WHEN: caller is authenticated
  CONFORMS: Target::RETURN
---
spec: Target
RETURN:
- WHEN: input is valid
  MUST: return success
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // Combined WHEN should be "caller is authenticated and input is valid"
        // The combined guard is emitted via emit_yaml_string, which only quotes
        // when the value triggers needs_quoting. This combined value is a plain
        // string, so it stays unquoted.
        assert!(
            result.contains("WHEN: caller is authenticated and input is valid"),
            "expected combined WHEN guard, got:\n{}",
            result
        );
        assert!(result.contains("MUST: return success"));
    }

    // ---------------------------------------------------------------
    // inline_conforms: CONFORMS ref without ::SLOT suffix
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_no_slot_returns_error() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: Target
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root);
        assert!(result.is_err());
        let errs = result.unwrap_err();
        assert_eq!(errs.len(), 1);
        assert_eq!(errs[0].code, errors::QUERY_CONFORMS_NO_SLOT);
    }

    // ---------------------------------------------------------------
    // inline_conforms: cross-file CONFORMS ref resolution
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_cross_file_inlines_correctly() {
        let tmp = setup_root();
        let root = tmp.path();

        let source_content = "\
---
description: source file
version: v1
---
spec: my.Source
INPUT:
- CONFORMS: ./contract@my.Contract::INPUT
";
        let contract_content = "\
---
description: contract file
version: v1
---
spec: my.Contract
INPUT:
- MUST: validate input
- SHOULD: sanitize input
";
        write_file(root, "source.yass.yaml", source_content);
        write_file(root, "contract.yass.yaml", contract_content);

        let docs = yaml_parse::parse_documents(source_content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(
            doc,
            &root.join("source.yass.yaml"),
            root,
            root,
        )
        .unwrap();

        assert!(result.contains("MUST: validate input"));
        assert!(result.contains("SHOULD: sanitize input"));
        assert!(result.contains("# CONFORMS: ./contract@my.Contract::INPUT"));
    }

    // ---------------------------------------------------------------
    // inline_conforms: CONFORMS ref target not found
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_unresolved_returns_error() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- CONFORMS: NonExistent::RETURN
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root);
        assert!(result.is_err());
        let errs = result.unwrap_err();
        assert_eq!(errs[0].code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // inline_conforms: non-CONFORMS references preserved on carrier
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_preserves_non_conforms_refs_on_carrier() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
RETURN:
- MUST: do something
  CONFORMS: Target::RETURN
  SEE: other.Spec
  USES: util.Helper
---
spec: Target
RETURN:
- MUST: contracted
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        // SEE and USES should be preserved on the original obligation
        assert!(result.contains("SEE: other.Spec"), "SEE should be preserved, got:\n{}", result);
        assert!(result.contains("USES: util.Helper"), "USES should be preserved, got:\n{}", result);
        // CONFORMS should only be in provenance comments
        let conforms_count = result.matches("CONFORMS:").count();
        let provenance_count = result.matches("# CONFORMS:").count();
        assert_eq!(conforms_count, provenance_count);
    }

    // ---------------------------------------------------------------
    // inline_conforms: multiple CONFORMS refs in different slots
    // ---------------------------------------------------------------

    #[test]
    fn test_conforms_multiple_slots() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = "\
---
description: test
version: v1
---
spec: Source
INPUT:
- CONFORMS: Target::INPUT
RETURN:
- CONFORMS: Target::RETURN
---
spec: Target
INPUT:
- MUST: accept input
RETURN:
- MUST: produce output
";
        write_file(root, "test.yass.yaml", content);

        let docs = yaml_parse::parse_documents(content).unwrap();
        let doc = &docs[1];

        let result = inline_conforms(doc, &root.join("test.yass.yaml"), root, root).unwrap();

        assert!(result.contains("MUST: accept input"));
        assert!(result.contains("MUST: produce output"));
        // Should have provenance comments for both slots
        assert!(result.contains("# CONFORMS: Target::INPUT"));
        assert!(result.contains("# CONFORMS: Target::RETURN"));
    }

    // ---------------------------------------------------------------
    // Output profile: plain scalar (no quoting needed)
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_scalar_plain_no_quoting() {
        let val = Yaml::String("do something simple".to_string());
        assert_eq!(emit_scalar(&val), "do something simple");
    }

    // ---------------------------------------------------------------
    // Output profile: scalar with ": " -> double-quoted
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_scalar_colon_space_quoted() {
        let val = Yaml::String("key: value".to_string());
        assert_eq!(emit_scalar(&val), "\"key: value\"");
    }

    // ---------------------------------------------------------------
    // Output profile: scalar with leading special char -> double-quoted
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_scalar_leading_special_quoted() {
        assert_eq!(emit_scalar(&Yaml::String("*starred".to_string())), "\"*starred\"");
        assert_eq!(emit_scalar(&Yaml::String("&anchored".to_string())), "\"&anchored\"");
        assert_eq!(emit_scalar(&Yaml::String("!tagged".to_string())), "\"!tagged\"");
        assert_eq!(emit_scalar(&Yaml::String("@mentioned".to_string())), "\"@mentioned\"");
    }

    // ---------------------------------------------------------------
    // Output profile: boolean/numeric values -> double-quoted
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_scalar_boolean_like_quoted() {
        assert_eq!(emit_scalar(&Yaml::String("true".to_string())), "\"true\"");
        assert_eq!(emit_scalar(&Yaml::String("false".to_string())), "\"false\"");
        assert_eq!(emit_scalar(&Yaml::String("yes".to_string())), "\"yes\"");
        assert_eq!(emit_scalar(&Yaml::String("no".to_string())), "\"no\"");
    }

    #[test]
    fn test_emit_scalar_numeric_like_quoted() {
        assert_eq!(emit_scalar(&Yaml::String("42".to_string())), "\"42\"");
        assert_eq!(emit_scalar(&Yaml::String("3.14".to_string())), "\"3.14\"");
        assert_eq!(emit_scalar(&Yaml::String("0".to_string())), "\"0\"");
    }

    // ---------------------------------------------------------------
    // Output profile: empty string -> double-quoted
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_scalar_empty_string_quoted() {
        assert_eq!(emit_scalar(&Yaml::String(String::new())), "\"\"");
    }

    // ---------------------------------------------------------------
    // Output profile: scalar with newlines -> double-quoted
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_scalar_newlines_quoted() {
        assert_eq!(
            emit_scalar(&Yaml::String("line1\nline2".to_string())),
            "\"line1\nline2\""
        );
    }

    // ---------------------------------------------------------------
    // Output profile: emit_scalar for non-string types
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_scalar_real() {
        assert_eq!(emit_scalar(&Yaml::Real("3.14".to_string())), "3.14");
    }

    #[test]
    fn test_emit_scalar_null_value() {
        assert_eq!(emit_scalar(&Yaml::Null), "null");
    }

    #[test]
    fn test_emit_scalar_boolean_value() {
        assert_eq!(emit_scalar(&Yaml::Boolean(false)), "false");
    }

    #[test]
    fn test_emit_scalar_array_debug() {
        let val = Yaml::Array(vec![Yaml::Integer(1)]);
        let result = emit_scalar(&val);
        // Should fall through to Debug format
        assert!(result.contains("Array"), "expected Debug format, got: {}", result);
    }

    // ---------------------------------------------------------------
    // Multi-match disambiguation: multiple specs with same suffix
    // across files, verify output format and ordering
    // ---------------------------------------------------------------

    #[test]
    fn test_multi_match_across_files_returns_matches() {
        let tmp = setup_root();
        let root = tmp.path();

        write_file(root, "b_file.yass.yaml", &simple_spec("ns1.Widget"));
        write_file(root, "a_file.yass.yaml", &simple_spec("ns2.Widget"));

        // Both should match suffix "Widget"
        let exit = run_query("Widget", None, root);
        assert_eq!(exit, 0); // multi-match exits 0
    }

    #[test]
    fn test_multi_match_same_file_different_specs() {
        let tmp = setup_root();
        let root = tmp.path();

        let content = multi_spec_file(
            "multiple widgets",
            &[
                ("alpha.Widget", "RETURN:\n- MUST: do alpha\n"),
                ("beta.Widget", "RETURN:\n- MUST: do beta\n"),
                ("gamma.Widget", "RETURN:\n- MUST: do gamma\n"),
            ],
        );
        write_file(root, "widgets.yass.yaml", &content);

        let exit = run_query("Widget", None, root);
        assert_eq!(exit, 0); // multi-match exits 0
    }

    // ---------------------------------------------------------------
    // Disambiguation row format: tab-separated
    // ---------------------------------------------------------------

    #[test]
    fn test_disambiguation_format_tab_separated() {
        let row = list::format_list_row("path/to/file.yass.yaml", "ns.Spec", "a description here");
        // Should be tab-separated
        let parts: Vec<&str> = row.split('\t').collect();
        assert_eq!(parts.len(), 3);
        assert_eq!(parts[0], "path/to/file.yass.yaml");
        assert_eq!(parts[1], "ns.Spec");
        assert_eq!(parts[2], "a description here");
    }

    // ---------------------------------------------------------------
    // inline_conforms: non-hash document returns minimal output
    // ---------------------------------------------------------------

    #[test]
    fn test_inline_conforms_non_hash_document() {
        let tmp = setup_root();
        let root = tmp.path();

        // Create a ParsedDoc with non-hash content (e.g., a plain scalar)
        let doc = yaml_parse::ParsedDoc {
            start_line: 1,
            content: Yaml::String("just a string".to_string()),
            raw_keys: vec![],
        };

        let result = inline_conforms(&doc, &root.join("test.yass.yaml"), root, root).unwrap();
        assert_eq!(result, "---\n");
    }

    // ---------------------------------------------------------------
    // extract_description: empty docs vec
    // ---------------------------------------------------------------

    #[test]
    fn test_extract_description_empty_docs() {
        let docs: Vec<yaml_parse::ParsedDoc> = vec![];
        assert_eq!(extract_description(&docs), "");
    }

    // ---------------------------------------------------------------
    // extract_description: description is not a string
    // ---------------------------------------------------------------

    #[test]
    fn test_extract_description_non_string_description() {
        let content = "---\ndescription: 42\nversion: v1\n---\nspec: S\n";
        let docs = yaml_parse::parse_documents(content).unwrap();
        // description value is an integer, not a string
        assert_eq!(extract_description(&docs), "");
    }

    // ---------------------------------------------------------------
    // has_normativity_key: positive and negative
    // ---------------------------------------------------------------

    #[test]
    fn test_has_normativity_key_positive() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));
        assert!(has_normativity_key(&hash));
    }

    #[test]
    fn test_has_normativity_key_negative() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("CONFORMS".to_string()), Yaml::String("ref".to_string()));
        assert!(!has_normativity_key(&hash));
    }

    #[test]
    fn test_has_normativity_key_all_keywords() {
        for keyword in crate::errors::NORMATIVITY_KEYWORDS {
            let mut hash = Hash::new();
            hash.insert(Yaml::String(keyword.to_string()), Yaml::String("val".to_string()));
            assert!(has_normativity_key(&hash), "should detect normativity keyword: {}", keyword);
        }
    }

    // ---------------------------------------------------------------
    // get_when_guard: present and absent
    // ---------------------------------------------------------------

    #[test]
    fn test_get_when_guard_present() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("WHEN".to_string()), Yaml::String("condition".to_string()));
        assert_eq!(get_when_guard(&hash), Some("condition".to_string()));
    }

    #[test]
    fn test_get_when_guard_absent() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));
        assert_eq!(get_when_guard(&hash), None);
    }

    // ---------------------------------------------------------------
    // find_conforms_ref: present and absent
    // ---------------------------------------------------------------

    #[test]
    fn test_find_conforms_ref_present() {
        let mut hash = Hash::new();
        hash.insert(
            Yaml::String("CONFORMS".to_string()),
            Yaml::String("Target::RETURN".to_string()),
        );
        assert_eq!(find_conforms_ref(&hash), Some("Target::RETURN".to_string()));
    }

    #[test]
    fn test_find_conforms_ref_absent() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));
        assert_eq!(find_conforms_ref(&hash), None);
    }

    // ---------------------------------------------------------------
    // resolve_conforms: cross-file ref with missing file
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_conforms_missing_cross_file() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "source.yass.yaml", "---\nspec: S\n");

        let result = resolve_conforms(
            "./nonexistent@Target::RETURN",
            &root.join("source.yass.yaml"),
            root,
        );
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // resolve_conforms: cross-file ref with non-UTF8 target
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_conforms_non_utf8_target_file() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "source.yass.yaml", "---\nspec: S\n");
        // Write non-UTF8 content to target file
        fs::write(root.join("target.yass.yaml"), &[0xFF, 0xFE]).unwrap();

        let result = resolve_conforms(
            "./target@Target::RETURN",
            &root.join("source.yass.yaml"),
            root,
        );
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // resolve_conforms: cross-file ref with malformed YAML
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_conforms_malformed_yaml_target() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "source.yass.yaml", "---\nspec: S\n");
        write_file(root, "target.yass.yaml", "---\n[[[invalid");

        let result = resolve_conforms(
            "./target@Target::RETURN",
            &root.join("source.yass.yaml"),
            root,
        );
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // resolve_conforms: same-file non-UTF8
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_conforms_same_file_non_utf8() {
        let tmp = setup_root();
        let root = tmp.path();
        // Write non-UTF8 content
        fs::write(root.join("bad.yass.yaml"), &[0xFF, 0xFE]).unwrap();

        let result = resolve_conforms(
            "Target::RETURN",
            &root.join("bad.yass.yaml"),
            root,
        );
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // resolve_conforms: same-file malformed YAML
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_conforms_same_file_malformed_yaml() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "bad.yass.yaml", "---\n[[[invalid");

        let result = resolve_conforms(
            "Target::RETURN",
            &root.join("bad.yass.yaml"),
            root,
        );
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // resolve_conforms: same-file unreadable
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_conforms_same_file_unreadable() {
        let tmp = setup_root();
        let root = tmp.path();

        let result = resolve_conforms(
            "Target::RETURN",
            &root.join("nonexistent.yass.yaml"),
            root,
        );
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // resolve_conforms: spec found but content is not a hash
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_conforms_spec_not_found_in_target() {
        let tmp = setup_root();
        let root = tmp.path();
        // Target file has a spec but with wrong name
        write_file(root, "target.yass.yaml", "---\nspec: WrongName\nRETURN:\n- MUST: do thing\n");

        let result = resolve_conforms(
            "./target@RightName::RETURN",
            &root.join("source.yass.yaml"),
            root,
        );
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // resolve_conforms: spec found but slot not declared
    // ---------------------------------------------------------------

    #[test]
    fn test_resolve_conforms_slot_not_declared() {
        let tmp = setup_root();
        let root = tmp.path();
        write_file(root, "test.yass.yaml", "---\nspec: Target\nRETURN:\n- MUST: do thing\n");

        let result = resolve_conforms(
            "Target::INPUT",
            &root.join("test.yass.yaml"),
            root,
        );
        assert!(result.is_err());
        assert_eq!(result.unwrap_err().code, errors::QUERY_CONFORMS_UNRESOLVED);
    }

    // ---------------------------------------------------------------
    // needs_quoting: edge cases
    // ---------------------------------------------------------------

    #[test]
    fn test_needs_quoting_on_off() {
        assert!(needs_quoting("on"));
        assert!(needs_quoting("off"));
        assert!(needs_quoting("On"));
        assert!(needs_quoting("OFF"));
    }

    #[test]
    fn test_needs_quoting_regular_word() {
        assert!(!needs_quoting("hello"));
        assert!(!needs_quoting("MySpec"));
        assert!(!needs_quoting("cli.validate"));
    }

    #[test]
    fn test_needs_quoting_hex_prefix() {
        assert!(needs_quoting("0x1F"));
        assert!(needs_quoting("0xFF"));
    }

    #[test]
    fn test_needs_quoting_octal_prefix() {
        assert!(needs_quoting("0o777"));
    }

    #[test]
    fn test_needs_quoting_binary_prefix() {
        assert!(needs_quoting("0b1010"));
    }

    // ---------------------------------------------------------------
    // emit_yaml_string: escaping when quoting is triggered
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_yaml_string_escapes_backslash_in_quoted() {
        // A string with both ": " (triggers quoting) and backslash
        let result = emit_yaml_string("path: C:\\Users\\foo");
        assert_eq!(result, "\"path: C:\\\\Users\\\\foo\"");
    }

    #[test]
    fn test_emit_yaml_string_escapes_double_quotes_in_quoted() {
        // A string with ": " and embedded double quotes
        let result = emit_yaml_string("say: \"hello\"");
        assert_eq!(result, "\"say: \\\"hello\\\"\"");
    }

    // ---------------------------------------------------------------
    // emit_obligation: verify output structure
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_obligation_basic() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));

        let mut output = String::new();
        emit_obligation(&mut output, &hash, false);

        assert_eq!(output, "- MUST: do thing\n");
    }

    #[test]
    fn test_emit_obligation_with_when_and_ref() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));
        hash.insert(Yaml::String("WHEN".to_string()), Yaml::String("condition".to_string()));
        hash.insert(Yaml::String("USES".to_string()), Yaml::String("other.Spec".to_string()));

        let mut output = String::new();
        emit_obligation(&mut output, &hash, false);

        assert!(output.starts_with("- "));
        assert!(output.contains("MUST: do thing"));
        assert!(output.contains("WHEN: condition"));
        assert!(output.contains("USES: other.Spec"));
    }

    #[test]
    fn test_emit_obligation_strip_conforms() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));
        hash.insert(Yaml::String("CONFORMS".to_string()), Yaml::String("Target::RETURN".to_string()));
        hash.insert(Yaml::String("USES".to_string()), Yaml::String("other.Spec".to_string()));

        let mut output = String::new();
        emit_obligation(&mut output, &hash, true);

        assert!(output.contains("MUST: do thing"));
        assert!(output.contains("USES: other.Spec"));
        assert!(!output.contains("CONFORMS:"), "CONFORMS should be stripped when strip_conforms=true");
    }

    // ---------------------------------------------------------------
    // emit_inlined_obligation: verify output structure
    // ---------------------------------------------------------------

    #[test]
    fn test_emit_inlined_obligation_no_carrier_when() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));
        let ob = Yaml::Hash(hash);

        let mut output = String::new();
        emit_inlined_obligation(&mut output, &ob, None);

        assert_eq!(output, "- MUST: do thing\n");
    }

    #[test]
    fn test_emit_inlined_obligation_with_carrier_when() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));
        let ob = Yaml::Hash(hash);

        let mut output = String::new();
        emit_inlined_obligation(&mut output, &ob, Some("outer condition"));

        assert!(output.contains("MUST: do thing"));
        // "outer condition" does not trigger needs_quoting, so it stays unquoted
        assert!(output.contains("WHEN: outer condition"));
    }

    #[test]
    fn test_emit_inlined_obligation_non_hash_ignored() {
        let ob = Yaml::String("not a hash".to_string());

        let mut output = String::new();
        emit_inlined_obligation(&mut output, &ob, None);

        assert_eq!(output, "", "non-hash obligations should produce no output");
    }

    #[test]
    fn test_emit_inlined_obligation_strips_conforms_from_inlined() {
        let mut hash = Hash::new();
        hash.insert(Yaml::String("MUST".to_string()), Yaml::String("do thing".to_string()));
        hash.insert(Yaml::String("CONFORMS".to_string()), Yaml::String("Deep::RETURN".to_string()));
        let ob = Yaml::Hash(hash);

        let mut output = String::new();
        emit_inlined_obligation(&mut output, &ob, None);

        assert!(output.contains("MUST: do thing"));
        assert!(!output.contains("CONFORMS:"), "CONFORMS should not appear in inlined obligations");
    }

    // ---------------------------------------------------------------
    // run (argv dispatch): missing name
    // ---------------------------------------------------------------

    #[test]
    fn test_run_missing_name() {
        let args: Vec<String> = vec!["yass".to_string(), "query".to_string()];
        let exit = run(&args);
        assert_eq!(exit, 2); // EXIT_USAGE for missing spec name
    }

    // ---------------------------------------------------------------
    // Scope: relative scope path
    // ---------------------------------------------------------------

    #[test]
    fn test_query_scope_relative_path() {
        let tmp = setup_root();
        let root = tmp.path();

        let scope = root.join("sub");
        fs::create_dir(&scope).unwrap();
        write_file(root, "sub/test.yass.yaml", &simple_spec("sub.Spec"));

        // Use relative scope path "sub"
        let exit = run_query("sub.Spec", Some("sub"), root);
        assert_eq!(exit, 0);
    }

    // ---------------------------------------------------------------
    // Scope: absolute scope path
    // ---------------------------------------------------------------

    #[test]
    fn test_query_scope_absolute_path() {
        let tmp = setup_root();
        let root = tmp.path();

        let scope = root.join("abs_scope");
        fs::create_dir(&scope).unwrap();
        write_file(root, "abs_scope/test.yass.yaml", &simple_spec("abs.Spec"));

        let abs_scope = scope.to_str().unwrap();
        let exit = run_query("abs.Spec", Some(abs_scope), root);
        assert_eq!(exit, 0);
    }
}

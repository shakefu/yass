// YAML parsing utilities for yass.
//
// Handles low-level YAML 1.2 parsing and validation using yaml-rust2.
// Provides UTF-8, BOM, empty-file checks, and multi-document parsing
// with anchor/alias/tag rejection and duplicate-key detection.

use std::collections::HashSet;

use yaml_rust2::parser::{Event, MarkedEventReceiver, Parser};
use yaml_rust2::scanner::Marker;
use yaml_rust2::Yaml;

/// Error type for YAML parsing failures.
///
/// Variants are ordered by the priority in which they should be reported
/// (at most one error per file).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum YamlError {
    /// File content is not valid UTF-8.
    NotUtf8,
    /// File starts with a UTF-8 BOM (EF BB BF).
    HasBom,
    /// File is zero bytes.
    EmptyFile,
    /// YAML syntax is malformed.
    Malformed {
        line: usize,
        col: usize,
        message: String,
    },
    /// A duplicate key was found in a mapping.
    DuplicateKey {
        line: usize,
        col: usize,
        key: String,
    },
    /// An anchor, alias, or explicit tag was found.
    AnchorOrAlias {
        line: usize,
        col: usize,
        message: String,
    },
}

impl std::fmt::Display for YamlError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            YamlError::NotUtf8 => write!(f, "file is not valid UTF-8"),
            YamlError::HasBom => write!(f, "file starts with a UTF-8 BOM"),
            YamlError::EmptyFile => write!(f, "file is empty (zero bytes)"),
            YamlError::Malformed { line, col, message } => {
                write!(f, "malformed YAML at line {} col {}: {}", line, col, message)
            }
            YamlError::DuplicateKey { line, col, key } => {
                write!(
                    f,
                    "duplicate key '{}' at line {} col {}",
                    key, line, col
                )
            }
            YamlError::AnchorOrAlias { line, col, message } => {
                write!(
                    f,
                    "anchor/alias/tag at line {} col {}: {}",
                    line, col, message
                )
            }
        }
    }
}

/// A parsed YAML document with position metadata.
#[derive(Debug, Clone)]
pub struct ParsedDoc {
    /// 1-based line number where the document starts in the file.
    pub start_line: usize,
    /// The parsed YAML value.
    pub content: Yaml,
    /// Keys found in this document with their 1-based line numbers.
    /// Useful for error reporting on key-related issues.
    pub raw_keys: Vec<(String, usize)>,
}

/// Check if bytes are valid UTF-8.
///
/// Returns the content as a `&str` on success, or `YamlError::NotUtf8` on failure.
pub fn check_utf8(content: &[u8]) -> Result<&str, YamlError> {
    std::str::from_utf8(content).map_err(|_| YamlError::NotUtf8)
}

/// Check for UTF-8 BOM (byte sequence EF BB BF / U+FEFF).
///
/// Returns `Ok(())` if no BOM is present, or `YamlError::HasBom` if one is found.
pub fn check_bom(content: &str) -> Result<(), YamlError> {
    if content.starts_with('\u{FEFF}') {
        Err(YamlError::HasBom)
    } else {
        Ok(())
    }
}

/// Check if file content is zero bytes.
///
/// Returns `Ok(())` if the file has content, or `YamlError::EmptyFile` if empty.
pub fn check_empty(content: &[u8]) -> Result<(), YamlError> {
    if content.is_empty() {
        Err(YamlError::EmptyFile)
    } else {
        Ok(())
    }
}

/// Convert a byte offset to a 1-based line number.
pub fn get_line_number(content: &str, byte_offset: usize) -> usize {
    let clamped = byte_offset.min(content.len());
    content[..clamped].bytes().filter(|&b| b == b'\n').count() + 1
}

/// Event receiver that checks for anchors, aliases, explicit tags,
/// and duplicate keys while building YAML documents.
struct YassEventReceiver {
    /// Completed documents.
    docs: Vec<ParsedDoc>,
    /// Stack of (Yaml node being built, anchor_id).
    doc_stack: Vec<(Yaml, usize)>,
    /// Stack of current keys (one per active mapping).
    key_stack: Vec<Yaml>,
    /// Map from anchor ID to resolved Yaml value.
    anchor_map: std::collections::BTreeMap<usize, Yaml>,

    /// The 1-based line of the current DocumentStart.
    current_doc_start_line: usize,
    /// Collected raw keys for the current document.
    current_raw_keys: Vec<(String, usize)>,

    /// Stack of sets tracking keys seen in each active mapping level.
    /// Each entry is a set of (key_string, line) for duplicate detection.
    mapping_key_sets: Vec<HashSet<String>>,

    /// First error encountered (we stop on the first one).
    error: Option<YamlError>,
}

impl YassEventReceiver {
    fn new() -> Self {
        YassEventReceiver {
            docs: Vec::new(),
            doc_stack: Vec::new(),
            key_stack: Vec::new(),
            anchor_map: std::collections::BTreeMap::new(),
            current_doc_start_line: 1,
            current_raw_keys: Vec::new(),
            mapping_key_sets: Vec::new(),
            error: None,
        }
    }

    fn on_event_impl(&mut self, ev: Event, mark: Marker) -> Result<(), YamlError> {
        match ev {
            Event::StreamStart | Event::StreamEnd | Event::Nothing => {}
            Event::DocumentStart => {
                // mark.line() is 1-based in yaml-rust2
                self.current_doc_start_line = mark.line();
                self.current_raw_keys = Vec::new();
            }
            Event::DocumentEnd => {
                let yaml = match self.doc_stack.len() {
                    0 => Yaml::BadValue,
                    1 => self.doc_stack.pop().unwrap().0,
                    _ => unreachable!(),
                };
                self.docs.push(ParsedDoc {
                    start_line: self.current_doc_start_line,
                    content: yaml,
                    raw_keys: std::mem::take(&mut self.current_raw_keys),
                });
            }
            Event::Alias(_id) => {
                return Err(YamlError::AnchorOrAlias {
                    line: mark.line(),
                    col: mark.col() + 1,
                    message: "alias is not allowed".to_string(),
                });
            }
            Event::Scalar(v, style, aid, tag) => {
                // Reject anchors
                if aid > 0 {
                    return Err(YamlError::AnchorOrAlias {
                        line: mark.line(),
                        col: mark.col() + 1,
                        message: "anchor is not allowed".to_string(),
                    });
                }
                // Reject explicit tags (non-default)
                if let Some(ref t) = tag {
                    return Err(YamlError::AnchorOrAlias {
                        line: mark.line(),
                        col: mark.col() + 1,
                        message: format!("explicit tag '{}{}' is not allowed", t.handle, t.suffix),
                    });
                }

                let node = if style != yaml_rust2::scanner::TScalarStyle::Plain {
                    Yaml::String(v)
                } else {
                    Yaml::from_str(&v)
                };

                // If we are inside a mapping and we are in key position,
                // track the key for duplicate detection.
                if let Some(cur_key) = self.key_stack.last() {
                    if cur_key.is_badvalue() {
                        // We are about to set this scalar as a key.
                        let key_str = yaml_to_key_string(&node);
                        self.current_raw_keys
                            .push((key_str.clone(), mark.line()));

                        // Duplicate detection
                        if let Some(key_set) = self.mapping_key_sets.last_mut() {
                            if !key_set.insert(key_str.clone()) {
                                return Err(YamlError::DuplicateKey {
                                    line: mark.line(),
                                    col: mark.col() + 1,
                                    key: key_str,
                                });
                            }
                        }
                    }
                }

                self.insert_new_node((node, aid))?;
            }
            Event::SequenceStart(aid, tag) => {
                if aid > 0 {
                    return Err(YamlError::AnchorOrAlias {
                        line: mark.line(),
                        col: mark.col() + 1,
                        message: "anchor is not allowed".to_string(),
                    });
                }
                if let Some(ref t) = tag {
                    return Err(YamlError::AnchorOrAlias {
                        line: mark.line(),
                        col: mark.col() + 1,
                        message: format!("explicit tag '{}{}' is not allowed", t.handle, t.suffix),
                    });
                }
                self.doc_stack.push((Yaml::Array(Vec::new()), aid));
            }
            Event::SequenceEnd => {
                let node = self.doc_stack.pop().unwrap();
                self.insert_new_node(node)?;
            }
            Event::MappingStart(aid, tag) => {
                if aid > 0 {
                    return Err(YamlError::AnchorOrAlias {
                        line: mark.line(),
                        col: mark.col() + 1,
                        message: "anchor is not allowed".to_string(),
                    });
                }
                if let Some(ref t) = tag {
                    return Err(YamlError::AnchorOrAlias {
                        line: mark.line(),
                        col: mark.col() + 1,
                        message: format!("explicit tag '{}{}' is not allowed", t.handle, t.suffix),
                    });
                }
                self.doc_stack
                    .push((Yaml::Hash(yaml_rust2::yaml::Hash::new()), aid));
                self.key_stack.push(Yaml::BadValue);
                self.mapping_key_sets.push(HashSet::new());
            }
            Event::MappingEnd => {
                self.key_stack.pop().unwrap();
                self.mapping_key_sets.pop();
                let node = self.doc_stack.pop().unwrap();
                self.insert_new_node(node)?;
            }
        }
        Ok(())
    }

    fn insert_new_node(&mut self, node: (Yaml, usize)) -> Result<(), YamlError> {
        if node.1 > 0 {
            self.anchor_map.insert(node.1, node.0.clone());
        }
        if self.doc_stack.is_empty() {
            self.doc_stack.push(node);
        } else {
            let parent = self.doc_stack.last_mut().unwrap();
            match *parent {
                (Yaml::Array(ref mut v), _) => v.push(node.0),
                (Yaml::Hash(ref mut h), _) => {
                    let cur_key = self.key_stack.last_mut().unwrap();
                    if cur_key.is_badvalue() {
                        *cur_key = node.0;
                    } else {
                        let mut newkey = Yaml::BadValue;
                        std::mem::swap(&mut newkey, cur_key);
                        h.insert(newkey, node.0);
                    }
                }
                _ => unreachable!(),
            }
        }
        Ok(())
    }
}

impl MarkedEventReceiver for YassEventReceiver {
    fn on_event(&mut self, ev: Event, mark: Marker) {
        if self.error.is_some() {
            return;
        }
        if let Err(e) = self.on_event_impl(ev, mark) {
            self.error = Some(e);
        }
    }
}

/// Convert a Yaml value to a string representation for key tracking.
fn yaml_to_key_string(y: &Yaml) -> String {
    match y {
        Yaml::String(s) => s.clone(),
        Yaml::Integer(i) => i.to_string(),
        Yaml::Real(r) => r.clone(),
        Yaml::Boolean(b) => b.to_string(),
        Yaml::Null => "~".to_string(),
        _ => format!("{:?}", y),
    }
}

/// Parse a YAML multi-document stream.
///
/// Returns a list of parsed documents with their start line numbers,
/// or a single `YamlError` (at most one error per file, in priority order).
///
/// Checks performed:
/// - Reject anchors and aliases
/// - Reject explicit tags
/// - Detect duplicate keys within any mapping at any nesting level
/// - yes/no/on/off are treated as plain strings (YAML 1.2 core schema)
pub fn parse_documents(content: &str) -> Result<Vec<ParsedDoc>, YamlError> {
    let mut receiver = YassEventReceiver::new();
    let mut parser = Parser::new_from_str(content);

    match parser.load(&mut receiver, true) {
        Ok(()) => {}
        Err(scan_error) => {
            // If our receiver caught an error first, prefer that
            if let Some(e) = receiver.error {
                return Err(e);
            }
            return Err(YamlError::Malformed {
                line: scan_error.marker().line(),
                col: scan_error.marker().col() + 1,
                message: scan_error.info().to_string(),
            });
        }
    }

    // Check if receiver caught an error during loading
    if let Some(e) = receiver.error {
        return Err(e);
    }

    Ok(receiver.docs)
}

#[cfg(test)]
mod tests {
    use super::*;

    // ---------------------------------------------------------------
    // check_utf8
    // ---------------------------------------------------------------

    #[test]
    fn test_check_utf8_valid() {
        let content = b"hello: world";
        let result = check_utf8(content);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), "hello: world");
    }

    #[test]
    fn test_check_utf8_invalid() {
        // Invalid UTF-8 sequence
        let content: &[u8] = &[0xff, 0xfe, 0x00];
        let result = check_utf8(content);
        assert!(result.is_err());
        assert_eq!(result.unwrap_err(), YamlError::NotUtf8);
    }

    #[test]
    fn test_check_utf8_valid_multibyte() {
        let content = "key: \u{00e9}l\u{00e8}ve".as_bytes();
        let result = check_utf8(content);
        assert!(result.is_ok());
    }

    // ---------------------------------------------------------------
    // check_bom
    // ---------------------------------------------------------------

    #[test]
    fn test_check_bom_present() {
        let content = "\u{FEFF}key: value";
        let result = check_bom(content);
        assert!(result.is_err());
        assert_eq!(result.unwrap_err(), YamlError::HasBom);
    }

    #[test]
    fn test_check_bom_absent() {
        let content = "key: value";
        let result = check_bom(content);
        assert!(result.is_ok());
    }

    #[test]
    fn test_check_bom_raw_bytes() {
        // EF BB BF as bytes, then valid UTF-8
        let bytes: &[u8] = &[0xEF, 0xBB, 0xBF, b'a', b':', b' ', b'1'];
        let s = std::str::from_utf8(bytes).unwrap();
        let result = check_bom(s);
        assert!(result.is_err());
        assert_eq!(result.unwrap_err(), YamlError::HasBom);
    }

    // ---------------------------------------------------------------
    // check_empty
    // ---------------------------------------------------------------

    #[test]
    fn test_check_empty_zero_bytes() {
        let content: &[u8] = b"";
        let result = check_empty(content);
        assert!(result.is_err());
        assert_eq!(result.unwrap_err(), YamlError::EmptyFile);
    }

    #[test]
    fn test_check_empty_has_content() {
        let content: &[u8] = b"a: 1";
        let result = check_empty(content);
        assert!(result.is_ok());
    }

    // ---------------------------------------------------------------
    // get_line_number
    // ---------------------------------------------------------------

    #[test]
    fn test_get_line_number_first_line() {
        let content = "hello\nworld\nfoo";
        assert_eq!(get_line_number(content, 0), 1);
        assert_eq!(get_line_number(content, 4), 1);
    }

    #[test]
    fn test_get_line_number_second_line() {
        let content = "hello\nworld\nfoo";
        assert_eq!(get_line_number(content, 6), 2);
        assert_eq!(get_line_number(content, 10), 2);
    }

    #[test]
    fn test_get_line_number_third_line() {
        let content = "hello\nworld\nfoo";
        assert_eq!(get_line_number(content, 12), 3);
    }

    #[test]
    fn test_get_line_number_at_newline() {
        let content = "hello\nworld\nfoo";
        // At the newline character itself -- it is counted, so line 2
        assert_eq!(get_line_number(content, 6), 2);
    }

    #[test]
    fn test_get_line_number_beyond_end() {
        let content = "hello\nworld";
        // Offset beyond the string should clamp to end
        assert_eq!(get_line_number(content, 999), 2);
    }

    // ---------------------------------------------------------------
    // parse_documents: valid YAML
    // ---------------------------------------------------------------

    #[test]
    fn test_parse_single_document() {
        let yaml = "key: value\nother: 123\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 1);
        assert_eq!(docs[0].content["key"].as_str(), Some("value"));
        assert_eq!(docs[0].content["other"].as_i64(), Some(123));
    }

    #[test]
    fn test_parse_multi_document() {
        let yaml = "---\na: 1\n---\nb: 2\n---\nc: 3\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 3);
        assert_eq!(docs[0].content["a"].as_i64(), Some(1));
        assert_eq!(docs[1].content["b"].as_i64(), Some(2));
        assert_eq!(docs[2].content["c"].as_i64(), Some(3));
    }

    #[test]
    fn test_parse_multi_document_start_lines() {
        let yaml = "---\na: 1\n---\nb: 2\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 2);
        // First document starts at line 1 (the --- line)
        assert_eq!(docs[0].start_line, 1);
        // Second document starts at line 3
        assert_eq!(docs[1].start_line, 3);
    }

    #[test]
    fn test_parse_implicit_document() {
        let yaml = "foo: bar\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 1);
        assert_eq!(docs[0].content["foo"].as_str(), Some("bar"));
    }

    #[test]
    fn test_parse_nested_mapping() {
        let yaml = "outer:\n  inner: value\n  nested:\n    deep: 42\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 1);
        assert_eq!(docs[0].content["outer"]["inner"].as_str(), Some("value"));
        assert_eq!(docs[0].content["outer"]["nested"]["deep"].as_i64(), Some(42));
    }

    #[test]
    fn test_parse_sequence() {
        let yaml = "items:\n  - one\n  - two\n  - three\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 1);
        let items = docs[0].content["items"].as_vec().unwrap();
        assert_eq!(items.len(), 3);
        assert_eq!(items[0].as_str(), Some("one"));
    }

    // ---------------------------------------------------------------
    // parse_documents: yes/no/on/off as strings (YAML 1.2)
    // ---------------------------------------------------------------

    #[test]
    fn test_yes_no_on_off_as_strings() {
        // In YAML 1.2 core schema, yes/no/on/off are NOT booleans.
        // yaml-rust2 implements YAML 1.2, so these should be strings.
        let yaml = "a: yes\nb: no\nc: on\nd: off\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 1);
        let doc = &docs[0].content;
        // yaml-rust2 treats these as strings in YAML 1.2
        assert_eq!(doc["a"].as_str(), Some("yes"));
        assert_eq!(doc["b"].as_str(), Some("no"));
        assert_eq!(doc["c"].as_str(), Some("on"));
        assert_eq!(doc["d"].as_str(), Some("off"));
    }

    // ---------------------------------------------------------------
    // parse_documents: malformed YAML
    // ---------------------------------------------------------------

    #[test]
    fn test_malformed_yaml() {
        let yaml = "key: [\ninvalid yaml here\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::Malformed { line, .. } => {
                assert!(line > 0);
            }
            other => panic!("expected Malformed, got {:?}", other),
        }
    }

    #[test]
    fn test_malformed_yaml_bad_indentation() {
        let yaml = "a: 1\n  b: 2\n c: 3\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::Malformed { .. } => {}
            other => panic!("expected Malformed, got {:?}", other),
        }
    }

    // ---------------------------------------------------------------
    // parse_documents: duplicate keys
    // ---------------------------------------------------------------

    #[test]
    fn test_duplicate_key_top_level() {
        let yaml = "key: value1\nkey: value2\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::DuplicateKey { key, .. } => {
                assert_eq!(key, "key");
            }
            other => panic!("expected DuplicateKey, got {:?}", other),
        }
    }

    #[test]
    fn test_duplicate_key_nested() {
        let yaml = "outer:\n  inner: 1\n  inner: 2\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::DuplicateKey { key, .. } => {
                assert_eq!(key, "inner");
            }
            other => panic!("expected DuplicateKey, got {:?}", other),
        }
    }

    #[test]
    fn test_duplicate_key_different_levels_ok() {
        // Same key name at different nesting levels is fine
        let yaml = "key: 1\nouter:\n  key: 2\n";
        let result = parse_documents(yaml);
        assert!(result.is_ok());
    }

    #[test]
    fn test_no_duplicate_keys() {
        let yaml = "a: 1\nb: 2\nc: 3\n";
        let result = parse_documents(yaml);
        assert!(result.is_ok());
    }

    // ---------------------------------------------------------------
    // parse_documents: anchors and aliases
    // ---------------------------------------------------------------

    #[test]
    fn test_anchor_rejected() {
        let yaml = "a: &anchor value\nb: other\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::AnchorOrAlias { message, .. } => {
                assert!(message.contains("anchor"), "message: {}", message);
            }
            other => panic!("expected AnchorOrAlias, got {:?}", other),
        }
    }

    #[test]
    fn test_alias_rejected() {
        let yaml = "a: &x value\nb: *x\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::AnchorOrAlias { .. } => {}
            other => panic!("expected AnchorOrAlias, got {:?}", other),
        }
    }

    #[test]
    fn test_anchor_on_mapping_rejected() {
        let yaml = "a: &ref\n  x: 1\n  y: 2\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::AnchorOrAlias { .. } => {}
            other => panic!("expected AnchorOrAlias, got {:?}", other),
        }
    }

    #[test]
    fn test_explicit_tag_rejected() {
        let yaml = "a: !!str 123\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::AnchorOrAlias { message, .. } => {
                assert!(message.contains("tag"), "message: {}", message);
            }
            other => panic!("expected AnchorOrAlias, got {:?}", other),
        }
    }

    // ---------------------------------------------------------------
    // parse_documents: raw_keys tracking
    // ---------------------------------------------------------------

    #[test]
    fn test_raw_keys_collected() {
        let yaml = "foo: 1\nbar: 2\nbaz: 3\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 1);
        let keys: Vec<&str> = docs[0].raw_keys.iter().map(|(k, _)| k.as_str()).collect();
        assert!(keys.contains(&"foo"));
        assert!(keys.contains(&"bar"));
        assert!(keys.contains(&"baz"));
        assert_eq!(keys.len(), 3);
    }

    #[test]
    fn test_raw_keys_with_line_numbers() {
        let yaml = "foo: 1\nbar: 2\n";
        let docs = parse_documents(yaml).unwrap();
        let keys = &docs[0].raw_keys;
        // foo should be on line 1
        let foo_entry = keys.iter().find(|(k, _)| k == "foo").unwrap();
        assert_eq!(foo_entry.1, 1);
        // bar should be on line 2
        let bar_entry = keys.iter().find(|(k, _)| k == "bar").unwrap();
        assert_eq!(bar_entry.1, 2);
    }

    // ---------------------------------------------------------------
    // parse_documents: edge cases
    // ---------------------------------------------------------------

    #[test]
    fn test_empty_document() {
        let yaml = "---\n...\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 1);
    }

    #[test]
    fn test_document_with_only_scalar() {
        let yaml = "---\nhello\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 1);
        assert_eq!(docs[0].content.as_str(), Some("hello"));
    }

    #[test]
    fn test_multi_document_with_end_markers() {
        let yaml = "---\na: 1\n...\n---\nb: 2\n...\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs.len(), 2);
        assert_eq!(docs[0].content["a"].as_i64(), Some(1));
        assert_eq!(docs[1].content["b"].as_i64(), Some(2));
    }

    #[test]
    fn test_whitespace_only_content() {
        // Just whitespace, no actual YAML content -- yaml-rust2 parses this as
        // a single empty/null document
        let yaml = "   \n  \n";
        let result = parse_documents(yaml);
        assert!(result.is_ok());
    }

    #[test]
    fn test_comments_only() {
        let yaml = "# just a comment\n# another comment\n";
        let result = parse_documents(yaml);
        assert!(result.is_ok());
    }

    #[test]
    fn test_flow_mapping_duplicate_key() {
        let yaml = "{a: 1, a: 2}\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::DuplicateKey { key, .. } => {
                assert_eq!(key, "a");
            }
            other => panic!("expected DuplicateKey, got {:?}", other),
        }
    }

    #[test]
    fn test_integer_keys_no_duplicate() {
        let yaml = "1: one\n2: two\n3: three\n";
        let result = parse_documents(yaml);
        assert!(result.is_ok());
    }

    #[test]
    fn test_boolean_values_not_booleans() {
        // true/false ARE booleans in YAML 1.2
        let yaml = "a: true\nb: false\n";
        let docs = parse_documents(yaml).unwrap();
        assert_eq!(docs[0].content["a"].as_bool(), Some(true));
        assert_eq!(docs[0].content["b"].as_bool(), Some(false));
    }

    #[test]
    fn test_null_values() {
        let yaml = "a: null\nb: ~\n";
        let docs = parse_documents(yaml).unwrap();
        assert!(docs[0].content["a"].is_null());
        assert!(docs[0].content["b"].is_null());
    }

    #[test]
    fn test_display_not_utf8() {
        let e = YamlError::NotUtf8;
        assert_eq!(format!("{}", e), "file is not valid UTF-8");
    }

    #[test]
    fn test_display_has_bom() {
        let e = YamlError::HasBom;
        assert_eq!(format!("{}", e), "file starts with a UTF-8 BOM");
    }

    #[test]
    fn test_display_empty_file() {
        let e = YamlError::EmptyFile;
        assert_eq!(format!("{}", e), "file is empty (zero bytes)");
    }

    #[test]
    fn test_display_malformed() {
        let e = YamlError::Malformed {
            line: 3,
            col: 5,
            message: "bad syntax".to_string(),
        };
        assert_eq!(
            format!("{}", e),
            "malformed YAML at line 3 col 5: bad syntax"
        );
    }

    #[test]
    fn test_display_duplicate_key() {
        let e = YamlError::DuplicateKey {
            line: 2,
            col: 1,
            key: "foo".to_string(),
        };
        assert_eq!(format!("{}", e), "duplicate key 'foo' at line 2 col 1");
    }

    #[test]
    fn test_anchor_on_sequence_rejected() {
        let yaml = "items: &list\n  - 1\n  - 2\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::AnchorOrAlias { .. } => {}
            other => panic!("expected AnchorOrAlias, got {:?}", other),
        }
    }

    #[test]
    fn test_local_tag_rejected() {
        let yaml = "a: !custom value\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::AnchorOrAlias { message, .. } => {
                assert!(message.contains("tag"), "message: {}", message);
            }
            other => panic!("expected AnchorOrAlias, got {:?}", other),
        }
    }

    #[test]
    fn test_multiple_documents_second_has_error() {
        let yaml = "---\na: 1\n---\nb: &anchor value\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::AnchorOrAlias { .. } => {}
            other => panic!("expected AnchorOrAlias, got {:?}", other),
        }
    }

    #[test]
    fn test_deeply_nested_duplicate_key() {
        let yaml = "l1:\n  l2:\n    l3:\n      a: 1\n      a: 2\n";
        let result = parse_documents(yaml);
        assert!(result.is_err());
        match result.unwrap_err() {
            YamlError::DuplicateKey { key, .. } => {
                assert_eq!(key, "a");
            }
            other => panic!("expected DuplicateKey, got {:?}", other),
        }
    }
}

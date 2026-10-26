//! VDF text on keyvalues-parser, where the last duplicate key wins (decision 52).
//!
//! Steam writes `libraryfolders.vdf`, `appmanifest_<appid>.acf`,
//! `config.vdf` and each tool's `compatibilitytool.vdf` in this text form.
//! keyvalues-parser reads more real files than Go's line parser did: it
//! unescapes `\\`, keeps a pair that has a trailing `//` comment, and accepts
//! `"key" {` on one line. Those differences are accepted (decision 52).
//!
//! The parsed tree is a [`Node`]: a list of keys, each with every value it
//! had. Lookups take the last value of a key, as Go's map did, and check its
//! type after picking it. A key is matched exactly first and then without
//! regard to case, because Steam's own keys are not case sensitive.
//!
//! keyvalues-parser keeps keys in a sorted map, so a node does not know the
//! order of its keys in the file. [`Node::children`] sorts numeric keys by
//! value, which gives the file order of lists Steam numbers, such as the
//! libraries in `libraryfolders.vdf`.

use std::cmp::Ordering;
use std::io;
use std::path::{Path, PathBuf};

use keyvalues_parser::{Obj, Value};

/// One value in a VDF tree: a string or a nested node.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Item {
    /// A string value.
    Text(String),
    /// A nested node.
    Node(Node),
}

impl Item {
    /// The string, when this is one.
    pub fn as_text(&self) -> Option<&str> {
        match self {
            Item::Text(text) => Some(text),
            Item::Node(_) => None,
        }
    }

    /// The node, when this is one.
    pub fn as_node(&self) -> Option<&Node> {
        match self {
            Item::Node(node) => Some(node),
            Item::Text(_) => None,
        }
    }
}

/// A VDF node: its keys, each with every value it had in the file.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Node {
    entries: Vec<(String, Vec<Item>)>,
}

impl Node {
    /// The last value of `key`. An exact match wins over one that differs
    /// only in case.
    pub fn get(&self, key: &str) -> Option<&Item> {
        let exact = self.entries.iter().rev().find(|(k, _)| k == key);
        let entry = exact.or_else(|| {
            self.entries
                .iter()
                .rev()
                .find(|(k, _)| k.eq_ignore_ascii_case(key))
        })?;
        entry.1.last()
    }

    /// The last value of `key` when it is a node.
    pub fn node(&self, key: &str) -> Option<&Node> {
        self.get(key).and_then(Item::as_node)
    }

    /// The last value of `key` when it is a string.
    pub fn text(&self, key: &str) -> Option<&str> {
        self.get(key).and_then(Item::as_text)
    }

    /// The node at the end of a path of keys, such as
    /// `["InstallConfigStore", "Software", "Valve"]`.
    pub fn path(&self, keys: &[&str]) -> Option<&Node> {
        keys.iter().try_fold(self, |node, key| node.node(key))
    }

    /// Each key with its last value. Keys that are whole numbers come
    /// first, in numeric order, then the others in byte order.
    pub fn children(&self) -> Vec<(&str, &Item)> {
        let mut children: Vec<(&str, &Item)> = self
            .entries
            .iter()
            .filter_map(|(key, items)| Some((key.as_str(), items.last()?)))
            .collect();
        children.sort_by(|(a, _), (b, _)| numeric_first(a, b));
        children
    }

    /// Whether the node has no keys.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    fn from_obj(obj: &Obj<'_>) -> Node {
        Node {
            entries: obj
                .iter()
                .map(|(key, values)| (key.to_string(), values.iter().map(item).collect()))
                .collect(),
        }
    }
}

fn numeric_first(a: &str, b: &str) -> Ordering {
    match (a.parse::<u64>().ok(), b.parse::<u64>().ok()) {
        (Some(x), Some(y)) => x.cmp(&y).then_with(|| a.cmp(b)),
        (Some(_), None) => Ordering::Less,
        (None, Some(_)) => Ordering::Greater,
        (None, None) => a.cmp(b),
    }
}

fn item(value: &Value<'_>) -> Item {
    match value {
        Value::Str(text) => Item::Text(text.to_string()),
        Value::Obj(obj) => Item::Node(Node::from_obj(obj)),
    }
}

/// A VDF text that does not parse, with the parser's message.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("{0}")]
pub struct ParseError(pub String);

/// A VDF file that cannot be read or does not parse.
#[derive(Debug, thiserror::Error)]
pub enum VdfFileError {
    /// The file cannot be read.
    #[error("cannot read {}: {source}", path.display())]
    Read {
        /// The file.
        path: PathBuf,
        /// Why.
        source: io::Error,
    },
    /// The file does not parse.
    #[error("cannot parse {}: {message}", path.display())]
    Parse {
        /// The file.
        path: PathBuf,
        /// The parser's message.
        message: String,
    },
}

impl VdfFileError {
    /// Whether the file does not exist.
    pub fn is_not_found(&self) -> bool {
        matches!(self, VdfFileError::Read { source, .. } if source.kind() == io::ErrorKind::NotFound)
    }
}

/// Parses VDF text. The result holds the top-level key, so a manifest is
/// read with `parse(text)?.node("AppState")`, as in Go.
pub fn parse(text: &str) -> Result<Node, ParseError> {
    // A UTF-8 byte order mark is not part of the grammar.
    let text = text.strip_prefix('\u{feff}').unwrap_or(text);
    let vdf = keyvalues_parser::parse(text).map_err(|e| ParseError(e.to_string()))?;
    Ok(Node {
        entries: vec![(vdf.key.to_string(), vec![item(&vdf.value)])],
    })
}

/// Reads and parses a VDF file. Text that is not UTF-8 is read lossily.
pub fn read(path: &Path) -> Result<Node, VdfFileError> {
    let bytes = std::fs::read(path).map_err(|source| VdfFileError::Read {
        path: path.to_owned(),
        source,
    })?;
    parse(&String::from_utf8_lossy(&bytes)).map_err(|e| VdfFileError::Parse {
        path: path.to_owned(),
        message: e.0,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reads_a_manifest_like_go() {
        let text =
            "\"AppState\"\n{\n\t\"appid\"\t\t\"9100010\"\n\t\"name\"\t\t\"Glimmer Run\"\n}\n";
        let root = parse(text).unwrap();
        let app = root.node("AppState").unwrap();
        assert_eq!(app.text("appid"), Some("9100010"));
        assert_eq!(app.text("name"), Some("Glimmer Run"));
        assert_eq!(app.text("missing"), None);
        assert_eq!(app.node("name"), None, "a string is not a node");
    }

    #[test]
    fn the_last_duplicate_wins_and_its_type_decides() {
        let text = r#"
            "root"
            {
                "a" "first"
                "a" "second"
                "b" { "x" "1" }
                "b" "now text"
                "c" "text"
                "c" { "y" "2" }
            }
        "#;
        let root = parse(text).unwrap();
        let node = root.node("root").unwrap();
        assert_eq!(node.text("a"), Some("second"));
        assert_eq!(node.node("b"), None);
        assert_eq!(node.text("b"), Some("now text"));
        assert_eq!(node.node("c").unwrap().text("y"), Some("2"));
    }

    #[test]
    fn keys_match_exactly_first_then_without_case() {
        let text = r#"
            "Root"
            {
                "StateFlags" "4"
                "stateflags" "6"
                "Name" "x"
            }
        "#;
        let root = parse(text).unwrap();
        let node = root.node("root").unwrap();
        assert_eq!(node.text("StateFlags"), Some("4"));
        assert_eq!(node.text("stateflags"), Some("6"));
        assert_eq!(node.text("name"), Some("x"));
        assert_eq!(node.text("NAME"), Some("x"));
    }

    #[test]
    fn accepts_what_go_s_line_parser_dropped() {
        let text = r#"
            "compatibilitytools"
            {
              "compat_tools"
              {
                "GE-Proton11-7-x86_64" // Internal name of this tool
                {
                  "install_path" "." // a trailing comment
                  "display_name" "GE \"quoted\" C:\\Games"
                }
              }
            }
        "#;
        let root = parse(text).unwrap();
        let tool = root
            .path(&["compatibilitytools", "compat_tools", "GE-Proton11-7-x86_64"])
            .unwrap();
        assert_eq!(tool.text("install_path"), Some("."));
        assert_eq!(tool.text("display_name"), Some(r#"GE "quoted" C:\Games"#));
        let inline = parse("\"a\" { \"b\" { \"c\" \"d\" } }").unwrap();
        assert_eq!(inline.path(&["a", "b"]).unwrap().text("c"), Some("d"));
    }

    #[test]
    fn children_come_in_numeric_order() {
        let text = r#"
            "libraryfolders"
            {
                "10" { "path" "/ten" }
                "contentstatsid" "123"
                "2" { "path" "/two" }
                "0" { "path" "/zero" }
                "1" { "path" "/one" }
            }
        "#;
        let root = parse(text).unwrap();
        let keys: Vec<&str> = root
            .node("libraryfolders")
            .unwrap()
            .children()
            .into_iter()
            .map(|(k, _)| k)
            .collect();
        assert_eq!(keys, ["0", "1", "2", "10", "contentstatsid"]);
    }

    #[test]
    fn broken_text_is_an_error_and_a_bom_is_not() {
        assert!(parse("\"a\"\n{\n\"b\" \"c\"\n").is_err());
        assert!(parse("\"a\" }").is_err());
        let root = parse("\u{feff}\"a\" { \"b\" \"c\" }").unwrap();
        assert_eq!(root.node("a").unwrap().text("b"), Some("c"));
    }

    #[test]
    fn read_reports_missing_and_broken_files() {
        let dir = tempfile::tempdir().unwrap();
        let missing = read(&dir.path().join("none.vdf")).unwrap_err();
        assert!(missing.is_not_found(), "{missing}");
        let broken = dir.path().join("broken.vdf");
        std::fs::write(&broken, "\"a\" {").unwrap();
        let err = read(&broken).unwrap_err();
        assert!(!err.is_not_found());
        assert!(err.to_string().starts_with("cannot parse "), "{err}");
    }
}

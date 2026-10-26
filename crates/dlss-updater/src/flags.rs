//! Command-line flags read the way Go's `flag` package reads them, so the
//! workflows' `-json -type dlss -manifest data/manifest.json` and the Go
//! tool's help and error text keep working.
//!
//! - `-name` and `--name` are the same flag. A value follows as
//!   `-name value` or `-name=value`. A boolean takes only `-name=value`.
//! - Parsing stops at the first word that is not a flag, or after `--`.
//! - `-h` and `-help` print the usage and exit 0. Any other unknown flag, a
//!   missing value or a bad boolean prints the problem and the usage, and
//!   exits 2.

use std::collections::BTreeMap;

/// What a flag holds.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Kind {
    /// On or off. Its value is `true` or `false`.
    Bool,
    /// A string.
    Text,
}

/// One flag a command takes.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Flag {
    /// The name without dashes, such as `manifest`.
    pub name: &'static str,
    /// What it holds.
    pub kind: Kind,
    /// The help line.
    pub usage: &'static str,
    /// The value when the flag is not given. Empty is no default.
    pub default: &'static str,
}

/// Why the arguments did not parse.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FlagError {
    /// `-h` or `-help`: print the usage and exit 0.
    Help,
    /// A problem, Go's text: print it, then the usage, and exit 2.
    Usage(String),
}

/// Parsed flags and the words after them.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Parsed {
    values: BTreeMap<&'static str, String>,
    /// The words after the flags.
    pub rest: Vec<String>,
}

impl Parsed {
    /// A flag's value, its default when it was not given.
    pub fn text(&self, name: &str) -> &str {
        self.values.get(name).map_or("", String::as_str)
    }

    /// A boolean flag's value.
    pub fn on(&self, name: &str) -> bool {
        self.text(name) == "true"
    }
}

/// Parses `args` against `flags`, as Go's `FlagSet.Parse` does.
pub fn parse(flags: &[Flag], args: &[String]) -> Result<Parsed, FlagError> {
    let mut parsed = Parsed::default();
    for flag in flags {
        parsed.values.insert(flag.name, flag.default.to_owned());
    }
    let mut at = 0;
    while at < args.len() {
        let word = &args[at];
        if word.len() < 2 || !word.starts_with('-') {
            break;
        }
        at += 1;
        let mut name = &word[1..];
        if let Some(long) = name.strip_prefix('-') {
            if long.is_empty() {
                // `--` ends the flags.
                break;
            }
            name = long;
        }
        if name.is_empty() || name.starts_with('-') || name.starts_with('=') {
            return Err(FlagError::Usage(format!("bad flag syntax: {word}")));
        }
        // The first `=` after the first character splits off the value.
        let (name, value) = match name.char_indices().skip(1).find(|&(_, c)| c == '=') {
            Some((i, _)) => (&name[..i], Some(&name[i + 1..])),
            None => (name, None),
        };
        let Some(flag) = flags.iter().find(|f| f.name == name) else {
            if name == "h" || name == "help" {
                return Err(FlagError::Help);
            }
            return Err(FlagError::Usage(format!(
                "flag provided but not defined: -{name}"
            )));
        };
        let value = match (flag.kind, value) {
            (Kind::Bool, None) => "true".to_owned(),
            (Kind::Bool, Some(v)) => match parse_bool(v) {
                Some(b) => b.to_string(),
                None => {
                    return Err(FlagError::Usage(format!(
                        "invalid boolean value {v:?} for -{name}: parse error"
                    )));
                }
            },
            (Kind::Text, Some(v)) => v.to_owned(),
            (Kind::Text, None) => match args.get(at) {
                Some(v) => {
                    at += 1;
                    v.clone()
                }
                None => {
                    return Err(FlagError::Usage(format!("flag needs an argument: -{name}")));
                }
            },
        };
        parsed.values.insert(flag.name, value);
    }
    parsed.rest = args[at..].to_vec();
    Ok(parsed)
}

/// Go's `strconv.ParseBool`.
fn parse_bool(text: &str) -> Option<bool> {
    match text {
        "1" | "t" | "T" | "TRUE" | "true" | "True" => Some(true),
        "0" | "f" | "F" | "FALSE" | "false" | "False" => Some(false),
        _ => None,
    }
}

/// The usage text, as Go's default usage prints it: `Usage of <program>:`,
/// then every flag sorted by name.
pub fn usage(program: &str, flags: &[Flag]) -> String {
    let mut sorted: Vec<&Flag> = flags.iter().collect();
    sorted.sort_by_key(|f| f.name);
    let mut out = format!("Usage of {program}:\n");
    for flag in sorted {
        let mut line = format!("  -{}", flag.name);
        if flag.kind == Kind::Text {
            line.push_str(" string");
        }
        // A one-letter flag with no value name keeps its help on its line.
        line.push_str(if line.len() <= 4 { "\t" } else { "\n    \t" });
        line.push_str(&flag.usage.replace('\n', "\n    \t"));
        if !flag.default.is_empty() && flag.default != "false" {
            line.push_str(&format!(" (default {:?})", flag.default));
        }
        out.push_str(&line);
        out.push('\n');
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    const FLAGS: &[Flag] = &[
        Flag {
            name: "type",
            kind: Kind::Text,
            usage: "DLL type",
            default: "dlss",
        },
        Flag {
            name: "json",
            kind: Kind::Bool,
            usage: "Output as JSON",
            default: "false",
        },
        Flag {
            name: "x",
            kind: Kind::Bool,
            usage: "Short",
            default: "",
        },
    ];

    fn args(words: &[&str]) -> Vec<String> {
        words.iter().map(|w| (*w).to_owned()).collect()
    }

    fn ok(words: &[&str]) -> Parsed {
        parse(FLAGS, &args(words)).unwrap()
    }

    fn bad(words: &[&str]) -> String {
        match parse(FLAGS, &args(words)) {
            Err(FlagError::Usage(text)) => text,
            other => panic!("{words:?}: {other:?}"),
        }
    }

    #[test]
    fn single_and_double_dashes_and_both_value_forms() {
        let p = ok(&[]);
        assert_eq!((p.text("type"), p.on("json")), ("dlss", false));
        for words in [
            &["-json", "-type", "dlssg"][..],
            &["--json", "--type", "dlssg"][..],
            &["-json=true", "-type=dlssg"][..],
            &["--json=1", "--type=dlssg"][..],
        ] {
            let p = ok(words);
            assert_eq!((p.text("type"), p.on("json")), ("dlssg", true), "{words:?}");
        }
        assert!(
            !ok(&["-json", "-json=false"]).on("json"),
            "the last one wins"
        );
        assert_eq!(ok(&["-type="]).text("type"), "", "an empty value");
        assert_eq!(
            ok(&["-type", "-json"]).text("type"),
            "-json",
            "a value may look like a flag"
        );
    }

    #[test]
    fn parsing_stops_at_the_first_word_or_after_two_dashes() {
        assert_eq!(ok(&["add", "-json"]).rest, ["add", "-json"]);
        let p = ok(&["-json", "add", "-type", "x"]);
        assert!(p.on("json"));
        assert_eq!(p.rest, ["add", "-type", "x"]);
        let p = ok(&["--", "-json"]);
        assert!(!p.on("json"));
        assert_eq!(p.rest, ["-json"]);
        assert_eq!(
            ok(&["-", "-json"]).rest,
            ["-", "-json"],
            "a lone dash is a word"
        );
    }

    #[test]
    fn help_and_errors_carry_go_s_text() {
        for help in ["-h", "--h", "-help", "--help"] {
            assert_eq!(parse(FLAGS, &args(&[help])), Err(FlagError::Help), "{help}");
        }
        assert_eq!(
            bad(&["--invalid"]),
            "flag provided but not defined: -invalid"
        );
        assert_eq!(
            bad(&["-invalid=1"]),
            "flag provided but not defined: -invalid"
        );
        assert_eq!(bad(&["-type"]), "flag needs an argument: -type");
        assert_eq!(
            bad(&["-json=yes"]),
            r#"invalid boolean value "yes" for -json: parse error"#
        );
        assert_eq!(bad(&["-é=1"]), "flag provided but not defined: -é");
        for syntax in ["---json", "-=x", "--=x"] {
            assert_eq!(bad(&[syntax]), format!("bad flag syntax: {syntax}"));
        }
    }

    #[test]
    fn usage_lists_flags_by_name_with_their_defaults() {
        assert_eq!(
            usage("tool", FLAGS),
            "Usage of tool:\n  -json\n    \tOutput as JSON\n  -type string\n    \tDLL type (default \"dlss\")\n  -x\tShort\n"
        );
    }
}

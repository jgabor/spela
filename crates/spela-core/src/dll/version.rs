//! Version strings: the dotted compare Go uses everywhere, and the check a
//! manifest version passes before it goes into a cache path.
//!
//! The compare is Go's `CompareVersions` (`internal/dll/version.go`). The
//! DLL code, the release check, the TUI and `dlss-updater` all use it:
//!
//! - one leading lowercase `v` is stripped;
//! - the rest is split on `.`;
//! - each part is read as Go's `strconv.Atoi` reads it with the error
//!   ignored: an optional sign and decimal digits, 0 for anything else, and
//!   the end of the i64 range for a number too large;
//! - the shorter version is padded with zeros.
//!
//! So `3.7` equals `3.7.0`, `3-beta` reads as 0, and `abc.1` is below `0.2`.

use std::cmp::Ordering;
use std::num::IntErrorKind;

/// The most parts a manifest version may have, as in `310.5.3.0`.
pub const MAX_PARTS: usize = 4;

/// Compares two version strings the way Go spela does.
pub fn compare(a: &str, b: &str) -> Ordering {
    let a = parts(a);
    let b = parts(b);
    let len = a.len().max(b.len());
    let at = |v: &[i64], i: usize| v.get(i).copied().unwrap_or(0);
    (0..len)
        .map(|i| at(&a, i).cmp(&at(&b, i)))
        .find(|o| o.is_ne())
        .unwrap_or(Ordering::Equal)
}

/// Whether `available` is newer than `installed`, Go's `IsNewer`.
pub fn is_newer(installed: &str, available: &str) -> bool {
    compare(installed, available) == Ordering::Less
}

/// Whether a version may name a file in the DLL cache: 1 to 4 numbers
/// joined by dots, each only ASCII digits that fit in a `u32`. Go joined
/// any non-empty manifest version into the cache path, so a version with
/// path parts could leave the cache folder (decision 8).
pub fn is_valid(version: &str) -> bool {
    let mut count = 0;
    for part in version.split('.') {
        count += 1;
        let digits = !part.is_empty() && part.bytes().all(|b| b.is_ascii_digit());
        if count > MAX_PARTS || !digits || part.parse::<u32>().is_err() {
            return false;
        }
    }
    true
}

fn parts(version: &str) -> Vec<i64> {
    let version = version.strip_prefix('v').unwrap_or(version);
    version.split('.').map(atoi).collect()
}

/// Go's `strconv.Atoi` with its error ignored.
fn atoi(part: &str) -> i64 {
    match part.parse::<i64>() {
        Ok(n) => n,
        Err(e) => match e.kind() {
            IntErrorKind::PosOverflow => i64::MAX,
            IntErrorKind::NegOverflow => i64::MIN,
            _ => 0,
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn go_compare_table() {
        // internal/dll/version_test.go, TestCompareVersions.
        let cases = [
            ("3.7.20", "3.7.20", Ordering::Equal),
            ("3.7.19", "3.7.20", Ordering::Less),
            ("3.7.21", "3.7.20", Ordering::Greater),
            ("2.0.0", "3.0.0", Ordering::Less),
            ("3.6.0", "3.7.0", Ordering::Less),
            ("3.7", "3.7.1", Ordering::Less),
            ("3.7.1", "3.7", Ordering::Greater),
            ("3.7.0", "3.7", Ordering::Equal),
            ("v3.7.20", "3.7.20", Ordering::Equal),
            ("v1.0", "v2.0", Ordering::Less),
            ("3.7.20.1", "3.7.20.2", Ordering::Less),
            ("", "", Ordering::Equal),
            ("abc.1", "0.2", Ordering::Less),
        ];
        for (a, b, want) in cases {
            assert_eq!(compare(a, b), want, "{a:?} against {b:?}");
            assert_eq!(compare(b, a), want.reverse(), "{b:?} against {a:?}");
        }
    }

    #[test]
    fn go_is_newer_table() {
        // internal/dll/version_test.go, TestIsNewer.
        assert!(is_newer("3.7.10", "3.7.20"));
        assert!(!is_newer("3.7.20", "3.7.20"));
        assert!(!is_newer("3.7.20", "3.7.10"));
        assert!(is_newer("2.0.0", "3.0.0"));
    }

    #[test]
    fn parts_read_as_atoi_reads_them() {
        // A sign is part of the number.
        assert_eq!(compare("+3", "3"), Ordering::Equal);
        assert_eq!(compare("-1", "0"), Ordering::Less);
        // A syntax error is 0: a space, a suffix, a lone sign, an empty part.
        assert_eq!(compare(" 3", "0"), Ordering::Equal);
        assert_eq!(compare("3-beta", "0"), Ordering::Equal);
        assert_eq!(compare("+", "0"), Ordering::Equal);
        assert_eq!(compare("1..2", "1.0.2"), Ordering::Equal);
        // Too large is the end of the range, not 0.
        assert_eq!(
            compare("99999999999999999999", "9223372036854775807"),
            Ordering::Equal
        );
        assert_eq!(
            compare("-99999999999999999999", "-9223372036854775808"),
            Ordering::Equal
        );
        assert_eq!(compare("99999999999999999999", "1"), Ordering::Greater);
    }

    #[test]
    fn only_one_lowercase_v_is_stripped() {
        assert_eq!(compare("vv1", "0"), Ordering::Equal);
        assert_eq!(compare("V1", "0"), Ordering::Equal);
        assert_eq!(compare("v1", "1"), Ordering::Equal);
    }

    #[test]
    fn valid_versions_are_one_to_four_numbers() {
        for good in ["310", "310.5", "310.5.3", "310.5.3.0", "0", "007.1"] {
            assert!(is_valid(good), "{good}");
        }
        let bad = [
            "",
            "1.2.3.4.5",
            "v310.5.3",
            "310.5.",
            ".5",
            "1..2",
            "../../etc/passwd",
            "310/5",
            "310.5.3-beta",
            "+1",
            "-1",
            " 1",
            "4294967296",
            "١٢",
        ];
        for bad in bad {
            assert!(!is_valid(bad), "{bad}");
        }
    }
}

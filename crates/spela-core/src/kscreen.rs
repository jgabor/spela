//! Reading `kscreen-doctor -o`.
//!
//! The parser is Go's `parseOutputs` (`internal/display/vrr.go:137-206`),
//! kept closed: anything missing or repeated fails, so VRR never changes an
//! output spela did not read for sure. kscreen-doctor writes ANSI colour
//! even to a pipe, so colour codes are stripped first (verified:
//! `docs/phase0-checks.md`). Lines the parser does not need, such as modes,
//! geometry and HDR, are skipped.
//!
//! Only the process call lives in the binary. This is a pure function, so
//! the fixture in `tests/fixtures/kscreen` tests it.

use crate::backend::{DisplayError, Output};

/// The text `kscreen-doctor` prints on stdout when a set fails. Plasma 6.5
/// still exits 0 then (Go's `setPolicy`).
pub const APPLY_FAILED: &str = "applying config failed!";

/// Parses `kscreen-doctor -o` output into its outputs.
pub fn parse_outputs(text: &str) -> Result<Vec<Output>, DisplayError> {
    let fail = |m: String| Err(DisplayError::Failed(m));
    let mut outputs: Vec<Output> = Vec::new();
    // Which of enabled, connected, priority and Vrr the current output had.
    let mut seen = [false; 4];
    for line in strip_ansi(text).lines() {
        let fields: Vec<&str> = line.split_whitespace().collect();
        let Some(&key) = fields.first() else {
            continue;
        };
        if key == "Output:" {
            if let Some(last) = outputs.last()
                && seen.contains(&false)
            {
                return fail(format!("incomplete KDE discovery for {}", last.name));
            }
            let uuid = fields.get(3).copied().unwrap_or("");
            if fields.len() != 4
                || !is_uuid(uuid)
                || uuid == "00000000-0000-0000-0000-000000000000"
                || outputs.iter().any(|o| o.uuid == uuid)
            {
                return fail(format!(
                    "unsupported or ambiguous KDE output identity: {}",
                    line.trim()
                ));
            }
            outputs.push(Output {
                uuid: uuid.to_owned(),
                name: fields[2].to_owned(),
                enabled: false,
                connected: false,
                priority: 0,
                vrr: String::new(),
            });
            seen = [false; 4];
            continue;
        }
        let Some(current) = outputs.last_mut() else {
            continue;
        };
        let (slot, what) = match key {
            "enabled" | "disabled" => {
                current.enabled = key == "enabled";
                (0, "enabled")
            }
            "connected" | "disconnected" => {
                current.connected = key == "connected";
                (1, "connected")
            }
            "priority" => {
                match fields.get(1).map(|p| p.parse::<u32>()) {
                    Some(Ok(p)) if fields.len() == 2 => current.priority = p,
                    _ => return fail("unreadable KDE priority".into()),
                }
                (2, "priority")
            }
            "Vrr:" => {
                if fields.len() != 2 {
                    return fail("unreadable KDE VRR policy".into());
                }
                current.vrr = fields[1].to_lowercase();
                (3, "Vrr:")
            }
            _ => continue,
        };
        if seen[slot] {
            return fail(format!("ambiguous KDE {what}"));
        }
        seen[slot] = true;
    }
    match outputs.last() {
        None => fail("no KDE outputs discovered".into()),
        Some(last) if seen.contains(&false) => {
            fail(format!("incomplete KDE discovery for {}", last.name))
        }
        Some(_) => Ok(outputs),
    }
}

/// Whether `text` is a UUID in its 8-4-4-4-12 hex form.
fn is_uuid(text: &str) -> bool {
    let groups: Vec<&str> = text.split('-').collect();
    groups.len() == 5
        && groups
            .iter()
            .zip([8, 4, 4, 4, 12])
            .all(|(g, n)| g.len() == n && g.bytes().all(|b| b.is_ascii_hexdigit()))
}

/// Removes SGR colour codes, `ESC [ digits and ; m`, as Go's
/// `\x1b\[[0-9;]*m` does. Anything else is kept.
fn strip_ansi(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    let mut rest = text;
    while let Some(at) = rest.find("\x1b[") {
        out.push_str(&rest[..at]);
        let after = &rest[at + 2..];
        let len = after
            .bytes()
            .take_while(|b| b.is_ascii_digit() || *b == b';')
            .count();
        if after.as_bytes().get(len) == Some(&b'm') {
            rest = &after[len + 1..];
        } else {
            out.push_str("\x1b[");
            rest = after;
        }
    }
    out.push_str(rest);
    out
}

/// Whether a set's stdout reports Go's zero-exit failure, in any case.
pub fn set_failed(stdout: &str) -> bool {
    stdout.to_lowercase().contains(APPLY_FAILED)
}

#[cfg(test)]
mod tests {
    use super::*;

    const PRIMARY_UUID: &str = "11111111-2222-3333-4444-555555555555";
    const SECONDARY_UUID: &str = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee";

    fn primary() -> String {
        format!(
            "Output: 7 DP-3 {PRIMARY_UUID}\n\tenabled\n\tconnected\n\tpriority 1\n\tVrr: Automatic\n"
        )
    }

    fn secondary() -> String {
        format!(
            "Output: 1 HDMI-A-1 {SECONDARY_UUID}\n\tenabled\n\tconnected\n\tpriority 2\n\tVrr: Never\n"
        )
    }

    fn fixture() -> String {
        let path = concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../tests/fixtures/kscreen/doctor-o.txt"
        );
        std::fs::read_to_string(path).unwrap()
    }

    #[test]
    fn the_phase_0_capture_parses() {
        let outputs = parse_outputs(&fixture()).unwrap();
        assert_eq!(
            outputs,
            [Output {
                uuid: "4f1e2d3c-5b6a-4789-8abc-def012345601".into(),
                name: "DP-3".into(),
                enabled: true,
                connected: true,
                priority: 1,
                vrr: "never".into(),
            }]
        );
    }

    #[test]
    fn two_outputs_and_colour_codes() {
        let coloured = primary().replace("Output:", "\x1b[01;32mOutput: \x1b[0;0m")
            + "\tGeometry: 0,0 2560x1440\n\tHDR: enabled\n";
        let outputs = parse_outputs(&(secondary() + &coloured)).unwrap();
        assert_eq!(outputs.len(), 2);
        assert_eq!(outputs[1].vrr, "automatic");
        assert_eq!(outputs[0].priority, 2);
        assert_eq!(strip_ansi("a\x1b[1;2mb\x1b[xc"), "ab\x1b[xc");
    }

    #[test]
    fn unsafe_discovery_fails_closed() {
        // Go's TestApplyVRRRejectsUnsafeDiscoveryWithoutMutation, the cases
        // the parser itself refuses.
        let p = primary();
        let cases = [
            ("empty", String::new()),
            ("garbage", "not KDE".to_owned()),
            (
                "old format without UUID",
                p.replace(&format!(" {PRIMARY_UUID}"), ""),
            ),
            ("missing policy", p.replace("\tVrr: Automatic\n", "")),
            ("missing priority", p.replace("\tpriority 1\n", "")),
            (
                "invalid priority",
                p.replace("priority 1", "priority unknown"),
            ),
            ("negative priority", p.replace("priority 1", "priority -1")),
            ("missing connected", p.replace("\tconnected\n", "")),
            ("duplicate UUID", p.clone() + &p),
            (
                "null UUID",
                p.replace(PRIMARY_UUID, "00000000-0000-0000-0000-000000000000"),
            ),
            ("duplicate policy", p.clone() + "\tVrr: Never\n"),
            (
                "two words of policy",
                p.replace("Automatic", "Automatic now"),
            ),
            ("bad uuid", p.replace(PRIMARY_UUID, "1111-2222")),
        ];
        for (name, text) in cases {
            assert!(
                matches!(parse_outputs(&text), Err(DisplayError::Failed(_))),
                "{name}"
            );
        }
    }

    #[test]
    fn unknown_policies_parse_and_are_judged_later() {
        let outputs = parse_outputs(&primary().replace("Automatic", "incapable")).unwrap();
        assert_eq!(outputs[0].vrr, "incapable");
        assert_eq!(outputs[0].vrr_policy(), None);
    }

    #[test]
    fn zero_exit_failures() {
        assert!(set_failed("Applying config failed! Backend rejected\n"));
        assert!(!set_failed(""));
    }
}

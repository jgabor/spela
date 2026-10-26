//! KDE VRR around a launch: Go's `display.ApplyVRR`
//! (`internal/display/vrr.go:36-87`) over the [`Display`] trait.
//!
//! It runs in the user session, never under pkexec. It needs a Wayland
//! session with KDE in `XDG_CURRENT_DESKTOP`. Exactly one output must have
//! priority 1, and it must be enabled, connected and able to do VRR. A
//! primary already on the policy changes nothing and needs no restore.
//! Otherwise the restore is returned even when the set fails, because a
//! failed set may still have changed the display. The restore finds the
//! output again by its UUID and never re-targets the current primary.

use crate::backend::{Display, DisplayError, VrrPolicy};
use crate::env::{Env, var};

/// What a restore puts back: one output's policy before the launch.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct VrrRestore {
    /// The output's UUID.
    pub uuid: String,
    /// The output's connector name, for messages.
    pub name: String,
    /// Its policy before the launch.
    pub policy: VrrPolicy,
}

/// Whether the session is KDE on Wayland: `XDG_SESSION_TYPE` is `wayland`
/// and `XDG_CURRENT_DESKTOP`, split on `:`, names KDE in any case.
pub fn is_kde_wayland(env: &Env) -> bool {
    env.get(var::XDG_SESSION_TYPE) == Some("wayland")
        && env
            .get(var::XDG_CURRENT_DESKTOP)
            .unwrap_or("")
            .split(':')
            .any(|d| d.eq_ignore_ascii_case("KDE"))
}

/// Sets the primary output's policy. Returns the restore to register, if
/// any, and the outcome of the set. The caller keeps the restore even when
/// the outcome is an error.
pub fn apply(
    display: &mut dyn Display,
    env: &Env,
    policy: VrrPolicy,
) -> (Option<VrrRestore>, Result<(), DisplayError>) {
    if !is_kde_wayland(env) {
        return (
            None,
            Err(DisplayError::Unavailable(
                "VRR requires a KDE Wayland session".into(),
            )),
        );
    }
    let outputs = match display.outputs() {
        Ok(o) => o,
        Err(e) => return (None, Err(e)),
    };
    let mut primaries = outputs.iter().filter(|o| o.priority == 1);
    let primary = match (primaries.next(), primaries.next()) {
        (_, Some(_)) => {
            return (
                None,
                Err(DisplayError::Failed("ambiguous KDE primary display".into())),
            );
        }
        (Some(p), None) if p.enabled && p.connected => p,
        _ => {
            return (
                None,
                Err(DisplayError::Failed(
                    "no connected, enabled KDE primary display".into(),
                )),
            );
        }
    };
    let Some(current) = primary.vrr_policy() else {
        return (
            None,
            Err(DisplayError::Failed(format!(
                "primary display {} has unsupported or unreadable VRR policy {:?}",
                primary.name, primary.vrr
            ))),
        );
    };
    if current == policy {
        return (None, Ok(()));
    }
    let restore = VrrRestore {
        uuid: primary.uuid.clone(),
        name: primary.name.clone(),
        policy: current,
    };
    let result = display.set_vrr(&restore.uuid, policy);
    (Some(restore), result)
}

impl VrrRestore {
    /// Puts the policy back on the same output, after checking it is still
    /// connected, enabled and able to do VRR.
    pub fn run(&self, display: &mut dyn Display) -> Result<(), DisplayError> {
        let outputs = display.outputs()?;
        let Some(output) = outputs.iter().find(|o| o.uuid == self.uuid) else {
            return Err(DisplayError::Failed(format!(
                "original display {} ({}) disconnected; VRR not restored",
                self.name, self.uuid
            )));
        };
        if !output.connected || !output.enabled || output.vrr_policy().is_none() {
            return Err(DisplayError::Failed(format!(
                "original display {} is disconnected, disabled, or no longer supports VRR",
                self.name
            )));
        }
        display.set_vrr(&self.uuid, self.policy)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::backend::Output;
    use crate::fake::{DisplayFault, FakeDisplay};

    const PRIMARY: &str = "11111111-2222-3333-4444-555555555555";
    const SECONDARY: &str = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee";

    fn output(uuid: &str, name: &str, priority: u32, vrr: &str) -> Output {
        Output {
            uuid: uuid.into(),
            name: name.into(),
            enabled: true,
            connected: true,
            priority,
            vrr: vrr.into(),
        }
    }

    fn kde() -> Env {
        Env::from_pairs([
            (var::XDG_SESSION_TYPE, "wayland"),
            (var::XDG_CURRENT_DESKTOP, "kde"),
        ])
    }

    fn display() -> FakeDisplay {
        FakeDisplay::new(vec![
            output(SECONDARY, "HDMI-A-1", 2, "never"),
            output(PRIMARY, "DP-3", 1, "automatic"),
        ])
    }

    #[test]
    fn sets_the_primary_and_restores_it_by_uuid() {
        for policy in [VrrPolicy::Always, VrrPolicy::Never] {
            let d = display();
            let mut handle = d.clone();
            let (restore, result) = apply(&mut handle, &kde(), policy);
            result.unwrap();
            let restore = restore.expect("a restore");
            // The primary moves while the game runs: restore the old UUID.
            d.set_outputs(vec![
                output(SECONDARY, "HDMI-A-1", 1, "never"),
                output(PRIMARY, "DP-3", 2, policy.as_str()),
            ]);
            restore.run(&mut handle).unwrap();
            assert_eq!(
                d.sets(),
                [
                    (PRIMARY.to_owned(), policy),
                    (PRIMARY.to_owned(), VrrPolicy::Automatic)
                ]
            );
        }
    }

    #[test]
    fn the_same_policy_changes_nothing() {
        let d = display();
        let (restore, result) = apply(&mut d.clone(), &kde(), VrrPolicy::Automatic);
        assert!(restore.is_none());
        result.unwrap();
        assert!(d.sets().is_empty());
    }

    #[test]
    fn needs_a_kde_wayland_session() {
        for (session, desktop) in [
            ("x11", "KDE"),
            ("wayland", "GNOME"),
            ("", ""),
            ("wayland", "XFCE:KDEish"),
        ] {
            let env = Env::from_pairs([
                (var::XDG_SESSION_TYPE, session),
                (var::XDG_CURRENT_DESKTOP, desktop),
            ]);
            let d = display();
            let (restore, result) = apply(&mut d.clone(), &env, VrrPolicy::Always);
            assert!(restore.is_none() && result.is_err(), "{session} {desktop}");
            assert!(d.sets().is_empty());
        }
        assert!(is_kde_wayland(&Env::from_pairs([
            (var::XDG_SESSION_TYPE, "wayland"),
            (var::XDG_CURRENT_DESKTOP, "GNOME:KDE"),
        ])));
    }

    #[test]
    fn unsafe_selection_never_sets() {
        let mut disabled = output(PRIMARY, "DP-3", 1, "automatic");
        disabled.enabled = false;
        let mut unplugged = output(PRIMARY, "DP-3", 1, "automatic");
        unplugged.connected = false;
        let cases = [
            (
                "no primary",
                vec![output(SECONDARY, "HDMI-A-1", 2, "never")],
            ),
            (
                "two primaries",
                vec![
                    output(PRIMARY, "DP-3", 1, "automatic"),
                    output(SECONDARY, "HDMI-A-1", 1, "never"),
                ],
            ),
            ("disabled", vec![disabled]),
            ("disconnected", vec![unplugged]),
            ("incapable", vec![output(PRIMARY, "DP-3", 1, "incapable")]),
            (
                "unknown policy",
                vec![output(PRIMARY, "DP-3", 1, "adaptive")],
            ),
        ];
        for (name, outputs) in cases {
            let d = FakeDisplay::new(outputs);
            let (restore, result) = apply(&mut d.clone(), &kde(), VrrPolicy::Always);
            assert!(restore.is_none() && result.is_err(), "{name}");
            assert!(d.sets().is_empty(), "{name}");
        }
        let d = display();
        d.set_fault(Some(DisplayFault::TimedOut));
        let (restore, result) = apply(&mut d.clone(), &kde(), VrrPolicy::Always);
        assert_eq!((restore, result), (None, Err(DisplayError::TimedOut)));
    }

    #[test]
    fn a_failed_set_still_returns_its_restore() {
        let d = display();
        d.set_fault(Some(DisplayFault::SetFails));
        let (restore, result) = apply(&mut d.clone(), &kde(), VrrPolicy::Always);
        assert!(result.is_err());
        let restore = restore.expect("kept even though the set failed");
        assert_eq!(restore.policy, VrrPolicy::Automatic);
        assert_eq!(restore.uuid, PRIMARY);
    }

    #[test]
    fn restore_failures_never_target_a_replacement() {
        let restore = VrrRestore {
            uuid: PRIMARY.into(),
            name: "DP-3".into(),
            policy: VrrPolicy::Automatic,
        };
        let mut disabled = output(PRIMARY, "DP-3", 1, "always");
        disabled.enabled = false;
        let cases = [
            ("gone", vec![output(SECONDARY, "HDMI-A-1", 1, "never")]),
            (
                "replacement on the same connector",
                vec![output(SECONDARY, "DP-3", 1, "always")],
            ),
            ("disabled", vec![disabled]),
            ("incapable", vec![output(PRIMARY, "DP-3", 1, "incapable")]),
        ];
        for (name, outputs) in cases {
            let d = FakeDisplay::new(outputs);
            assert!(restore.run(&mut d.clone()).is_err(), "{name}");
            assert!(d.sets().is_empty(), "{name}");
        }
    }
}

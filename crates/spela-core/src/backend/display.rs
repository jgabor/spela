//! KDE VRR, applied in the user session through `kscreen-doctor`.

use std::fmt;

/// A VRR policy as KDE names it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum VrrPolicy {
    /// VRR off.
    Never,
    /// VRR on for everything.
    Always,
    /// VRR for full-screen apps only.
    Automatic,
}

impl VrrPolicy {
    /// The word `kscreen-doctor` takes.
    pub fn as_str(self) -> &'static str {
        match self {
            VrrPolicy::Never => "never",
            VrrPolicy::Always => "always",
            VrrPolicy::Automatic => "automatic",
        }
    }

    /// Parses the word, in any case.
    pub fn parse(text: &str) -> Option<VrrPolicy> {
        match text.trim().to_ascii_lowercase().as_str() {
            "never" => Some(VrrPolicy::Never),
            "always" => Some(VrrPolicy::Always),
            "automatic" => Some(VrrPolicy::Automatic),
            _ => None,
        }
    }
}

impl fmt::Display for VrrPolicy {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// One display output.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Output {
    /// KDE's stable id for the output.
    pub uuid: String,
    /// The connector name, such as `DP-1`.
    pub name: String,
    /// Whether the output is on.
    pub enabled: bool,
    /// The current policy, when the output supports VRR.
    pub vrr_policy: Option<VrrPolicy>,
}

/// Why a display call failed.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum DisplayError {
    /// No KDE session or no `kscreen-doctor`.
    #[error("KDE display control is not available: {0}")]
    Unavailable(String),
    /// `kscreen-doctor` did not answer in time.
    #[error("kscreen-doctor did not answer in time")]
    TimedOut,
    /// Any other failure.
    #[error("{0}")]
    Failed(String),
}

/// KDE's display settings.
pub trait Display: Send {
    /// Every output with its VRR state.
    fn outputs(&mut self) -> Result<Vec<Output>, DisplayError>;
    /// Sets one output's VRR policy.
    fn set_vrr(&mut self, uuid: &str, policy: VrrPolicy) -> Result<(), DisplayError>;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn policy_words_round_trip() {
        for p in [VrrPolicy::Never, VrrPolicy::Always, VrrPolicy::Automatic] {
            assert_eq!(VrrPolicy::parse(p.as_str()), Some(p));
            assert_eq!(p.to_string(), p.as_str());
        }
        assert_eq!(VrrPolicy::parse(" Always "), Some(VrrPolicy::Always));
        assert_eq!(VrrPolicy::parse("sometimes"), None);
    }
}

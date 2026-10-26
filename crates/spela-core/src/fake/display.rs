//! A scripted KDE display.

use std::sync::{Arc, Mutex, MutexGuard};

use crate::backend::display::{Display, DisplayError, Output, VrrPolicy};

/// A fault the fake display can show.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DisplayFault {
    /// No KDE session: every call is [`DisplayError::Unavailable`].
    Unavailable,
    /// `kscreen-doctor` never answers: every call is
    /// [`DisplayError::TimedOut`].
    TimedOut,
    /// Reads work and every `set_vrr` fails.
    SetFails,
}

#[derive(Debug)]
struct DisplayState {
    outputs: Vec<Output>,
    sets: Vec<(String, VrrPolicy)>,
    fault: Option<DisplayFault>,
}

/// Outputs in memory. `set_vrr` changes them and is recorded.
#[derive(Debug, Clone)]
pub struct FakeDisplay {
    state: Arc<Mutex<DisplayState>>,
}

impl FakeDisplay {
    /// A display with these outputs.
    pub fn new(outputs: Vec<Output>) -> Self {
        FakeDisplay {
            state: Arc::new(Mutex::new(DisplayState {
                outputs,
                sets: Vec::new(),
                fault: None,
            })),
        }
    }

    /// Two outputs: the primary, a VRR monitor on automatic, and a TV
    /// without VRR that is turned off.
    pub fn demo() -> Self {
        FakeDisplay::new(vec![
            Output {
                uuid: "6a1c4e1e-0000-4000-8000-000000000001".into(),
                name: "DP-1".into(),
                enabled: true,
                connected: true,
                priority: 1,
                vrr: "automatic".into(),
            },
            Output {
                uuid: "6a1c4e1e-0000-4000-8000-000000000002".into(),
                name: "HDMI-A-1".into(),
                enabled: false,
                connected: true,
                priority: 2,
                vrr: "incapable".into(),
            },
        ])
    }

    /// Turns a fault on, or off with `None`.
    pub fn set_fault(&self, fault: Option<DisplayFault>) {
        self.lock().fault = fault;
    }

    /// Replaces the outputs, as a display that is plugged in, unplugged or
    /// made primary while a game runs.
    pub fn set_outputs(&self, outputs: Vec<Output>) {
        self.lock().outputs = outputs;
    }

    /// Every successful `set_vrr` call, in order.
    pub fn sets(&self) -> Vec<(String, VrrPolicy)> {
        self.lock().sets.clone()
    }

    fn lock(&self) -> MutexGuard<'_, DisplayState> {
        self.state.lock().unwrap_or_else(|e| e.into_inner())
    }
}

impl Display for FakeDisplay {
    fn outputs(&mut self) -> Result<Vec<Output>, DisplayError> {
        let s = self.lock();
        match s.fault {
            Some(DisplayFault::Unavailable) => Err(DisplayError::Unavailable("fake".into())),
            Some(DisplayFault::TimedOut) => Err(DisplayError::TimedOut),
            _ => Ok(s.outputs.clone()),
        }
    }

    fn set_vrr(&mut self, uuid: &str, policy: VrrPolicy) -> Result<(), DisplayError> {
        let mut s = self.lock();
        match s.fault {
            Some(DisplayFault::Unavailable) => {
                return Err(DisplayError::Unavailable("fake".into()));
            }
            Some(DisplayFault::TimedOut) => return Err(DisplayError::TimedOut),
            Some(DisplayFault::SetFails) => {
                return Err(DisplayError::Failed("fake set failure".into()));
            }
            None => {}
        }
        let Some(output) = s.outputs.iter_mut().find(|o| o.uuid == uuid) else {
            return Err(DisplayError::Failed(format!("no output {uuid}")));
        };
        if output.vrr_policy().is_none() {
            return Err(DisplayError::Failed(format!(
                "{} does not support VRR",
                output.name
            )));
        }
        output.vrr = policy.as_str().to_owned();
        s.sets.push((uuid.to_owned(), policy));
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn set_vrr_changes_and_records() {
        let mut d = FakeDisplay::demo();
        let watcher = d.clone();
        let uuid = d.outputs().unwrap()[0].uuid.clone();
        d.set_vrr(&uuid, VrrPolicy::Always).unwrap();
        assert_eq!(
            d.outputs().unwrap()[0].vrr_policy(),
            Some(VrrPolicy::Always)
        );
        assert_eq!(watcher.sets(), [(uuid, VrrPolicy::Always)]);
    }

    #[test]
    fn unknown_or_non_vrr_outputs_fail() {
        let mut d = FakeDisplay::demo();
        assert!(d.set_vrr("nope", VrrPolicy::Never).is_err());
        let tv = d.outputs().unwrap()[1].uuid.clone();
        assert!(d.set_vrr(&tv, VrrPolicy::Never).is_err());
        assert!(d.sets().is_empty());
    }

    #[test]
    fn faults() {
        let mut d = FakeDisplay::demo();
        d.set_fault(Some(DisplayFault::TimedOut));
        assert_eq!(d.outputs(), Err(DisplayError::TimedOut));
        d.set_fault(Some(DisplayFault::SetFails));
        let uuid = d.outputs().unwrap()[0].uuid.clone();
        assert!(matches!(
            d.set_vrr(&uuid, VrrPolicy::Never),
            Err(DisplayError::Failed(_))
        ));
        d.set_fault(Some(DisplayFault::Unavailable));
        assert!(matches!(d.outputs(), Err(DisplayError::Unavailable(_))));
    }
}

//! The JSON lines between the parent and the root helper, version 1.
//!
//! The parent sends one batch:
//!
//! ```json
//! {"version": 1, "type": "apply", "steps": [{"step": "clock_offset", "value": 150}, {"step": "governor", "value": "performance"}]}
//! ```
//!
//! The helper answers one line per step, then `{"type": "ready"}` and waits
//! (`--serve`), or exits (`--once`). On `{"type": "restore"}`, or the end of
//! its input, the `--serve` helper answers one `restored` line per step it
//! puts back and then `{"type": "done"}`. A request it refuses gets one
//! `{"type": "error", "error": "..."}` line and changes nothing.
//!
//! Parsing is strict: an unknown key, a duplicate step, a value of the
//! wrong type or a number out of its type's range refuses the whole batch.
//! Whether a value suits this machine, such as a governor sysfs does not
//! list, is the helper's check per step, which skips that step only.

use serde_json::{Map, Value};

use crate::backend::{FanSetting, SettingValue, Step, StepOutcome, StepReport};

/// The protocol version both sides speak.
pub const VERSION: u64 = 1;

/// The longest line either side reads, in bytes. A batch is far shorter.
pub const MAX_LINE: usize = 16 * 1024;

/// Which helper loop a request is for.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Mode {
    /// `--serve`, for a launch: applies, waits and restores.
    Serve,
    /// `--once <request>`, for a live command: applies and exits.
    Once,
}

/// One line from the helper.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Reply {
    /// What happened to one step of the batch.
    Step(StepReport),
    /// Every step is done and the `--serve` helper waits.
    Ready,
    /// One step put back, or the error that stopped it.
    Restored {
        /// The setting.
        step: Step,
        /// What happened.
        result: Result<(), String>,
    },
    /// The restore is over; the helper exits.
    Done,
    /// The request was refused before any change.
    Error(String),
}

/// The batch for `request`'s steps as one line, without the newline.
pub fn encode_request(steps: &[(Step, SettingValue)]) -> String {
    let steps: Vec<Value> = steps
        .iter()
        .map(|(step, value)| {
            let mut m = Map::new();
            m.insert("step".into(), step.as_str().into());
            if *step != Step::LockedClocks {
                m.insert("value".into(), encode_value(value));
            }
            Value::Object(m)
        })
        .collect();
    let mut m = Map::new();
    m.insert("version".into(), VERSION.into());
    m.insert("type".into(), "apply".into());
    m.insert("steps".into(), Value::Array(steps));
    Value::Object(m).to_string()
}

/// The restore request, without the newline.
pub fn encode_restore() -> String {
    r#"{"type":"restore"}"#.to_owned()
}

/// Whether `line` is exactly the restore request.
pub fn is_restore(line: &str) -> bool {
    match serde_json::from_str::<Value>(line) {
        Ok(Value::Object(m)) => m.len() == 1 && m.get("type") == Some(&Value::from("restore")),
        _ => false,
    }
}

/// Reads a batch line: every step with its target, in apply order. `mode`
/// decides whether `locked_clocks` is allowed: only `--once` sends it,
/// because nothing restores it.
pub fn parse_request(line: &str, mode: Mode) -> Result<Vec<(Step, SettingValue)>, String> {
    let value: Value =
        serde_json::from_str(line).map_err(|e| format!("the request is not JSON: {e}"))?;
    let Value::Object(top) = value else {
        return Err("the request is not a JSON object".into());
    };
    only_keys(&top, &["version", "type", "steps"], "the request")?;
    match top.get("version").and_then(Value::as_u64) {
        Some(VERSION) => {}
        _ => return Err(format!("the request needs \"version\": {VERSION}")),
    }
    if top.get("type").and_then(Value::as_str) != Some("apply") {
        return Err("the request needs \"type\": \"apply\"".into());
    }
    let Some(Value::Array(raw)) = top.get("steps") else {
        return Err("the request needs a \"steps\" list".into());
    };
    if raw.is_empty() {
        return Err("the request has no steps".into());
    }
    let mut steps = Vec::with_capacity(raw.len());
    for item in raw {
        let Value::Object(m) = item else {
            return Err("a step is not a JSON object".into());
        };
        only_keys(m, &["step", "value"], "a step")?;
        let name = m
            .get("step")
            .and_then(Value::as_str)
            .ok_or("a step has no \"step\" name")?;
        let step = step_named(name).ok_or_else(|| format!("unknown step {name:?}"))?;
        if step == Step::LockedClocks && mode == Mode::Serve {
            return Err("locked_clocks is only for a one-shot change".into());
        }
        let target = match (step, m.get("value")) {
            (Step::LockedClocks, None) => SettingValue::DriverDefault,
            (Step::LockedClocks, Some(_)) => return Err("locked_clocks takes no value".into()),
            (_, None) => return Err(format!("{step} has no value")),
            (_, Some(v)) => decode_value(step, v)?,
        };
        if steps.iter().any(|(s, _)| *s == step) {
            return Err(format!("{step} is in the request twice"));
        }
        steps.push((step, target));
    }
    steps.sort_by_key(|(s, _)| *s);
    Ok(steps)
}

/// One reply as a line, without the newline.
pub fn encode_reply(reply: &Reply) -> String {
    let mut m = Map::new();
    match reply {
        Reply::Step(r) => {
            m.insert("type".into(), "step".into());
            m.insert("step".into(), r.step.as_str().into());
            m.insert("target".into(), encode_value(&r.target));
            match &r.outcome {
                StepOutcome::Applied { prior } => {
                    m.insert("outcome".into(), "applied".into());
                    m.insert("prior".into(), encode_value(prior));
                }
                StepOutcome::AppliedNoRestore => {
                    m.insert("outcome".into(), "applied_no_restore".into());
                }
                StepOutcome::Failed(e) => {
                    m.insert("outcome".into(), "failed".into());
                    m.insert("error".into(), e.as_str().into());
                }
                StepOutcome::Skipped(why) => {
                    m.insert("outcome".into(), "skipped".into());
                    m.insert("reason".into(), why.as_str().into());
                }
            }
        }
        Reply::Ready => {
            m.insert("type".into(), "ready".into());
        }
        Reply::Restored { step, result } => {
            m.insert("type".into(), "restored".into());
            m.insert("step".into(), step.as_str().into());
            match result {
                Ok(()) => {
                    m.insert("outcome".into(), "restored".into());
                }
                Err(e) => {
                    m.insert("outcome".into(), "failed".into());
                    m.insert("error".into(), e.as_str().into());
                }
            }
        }
        Reply::Done => {
            m.insert("type".into(), "done".into());
        }
        Reply::Error(e) => {
            m.insert("type".into(), "error".into());
            m.insert("error".into(), e.as_str().into());
        }
    }
    Value::Object(m).to_string()
}

/// Reads one reply line.
pub fn parse_reply(line: &str) -> Result<Reply, String> {
    let value: Value =
        serde_json::from_str(line).map_err(|e| format!("the reply is not JSON: {e}"))?;
    let Value::Object(m) = value else {
        return Err("the reply is not a JSON object".into());
    };
    let text = |key: &str| -> Result<String, String> {
        m.get(key)
            .and_then(Value::as_str)
            .map(str::to_owned)
            .ok_or_else(|| format!("the reply has no {key:?} text"))
    };
    let step = || -> Result<Step, String> {
        let name = text("step")?;
        step_named(&name).ok_or_else(|| format!("unknown step {name:?}"))
    };
    match text("type")?.as_str() {
        "step" => {
            let step = step()?;
            let target = decode_value(step, m.get("target").ok_or("the reply has no target")?)?;
            let outcome = match text("outcome")?.as_str() {
                "applied" => StepOutcome::Applied {
                    prior: decode_value(step, m.get("prior").ok_or("the reply has no prior")?)?,
                },
                "applied_no_restore" => StepOutcome::AppliedNoRestore,
                "failed" => StepOutcome::Failed(text("error")?),
                "skipped" => StepOutcome::Skipped(text("reason")?),
                other => return Err(format!("unknown outcome {other:?}")),
            };
            Ok(Reply::Step(StepReport {
                step,
                target,
                outcome,
            }))
        }
        "ready" => Ok(Reply::Ready),
        "restored" => {
            let step = step()?;
            let result = match text("outcome")?.as_str() {
                "restored" => Ok(()),
                "failed" => Err(text("error")?),
                other => return Err(format!("unknown outcome {other:?}")),
            };
            Ok(Reply::Restored { step, result })
        }
        "done" => Ok(Reply::Done),
        "error" => Ok(Reply::Error(text("error")?)),
        other => Err(format!("unknown reply type {other:?}")),
    }
}

/// The step a wire name stands for.
pub fn step_named(name: &str) -> Option<Step> {
    Step::ALL.into_iter().find(|s| s.as_str() == name)
}

fn only_keys(m: &Map<String, Value>, allowed: &[&str], what: &str) -> Result<(), String> {
    match m.keys().find(|k| !allowed.contains(&k.as_str())) {
        Some(k) => Err(format!("{what} has an unknown key {k:?}")),
        None => Ok(()),
    }
}

fn encode_value(value: &SettingValue) -> Value {
    match value {
        SettingValue::DriverDefault => "driver_default".into(),
        SettingValue::Mhz(v) => (*v).into(),
        SettingValue::Watts(v) => (*v).into(),
        SettingValue::Fan(FanSetting::Auto) => "auto".into(),
        SettingValue::Fan(FanSetting::Percent(p)) => (*p).into(),
        SettingValue::Governor(g) => g.as_str().into(),
        SettingValue::Smt(on) => (*on).into(),
    }
}

fn decode_value(step: Step, value: &Value) -> Result<SettingValue, String> {
    let bad = || format!("{value} is not a value for {step}");
    match step {
        Step::LockedClocks => match value.as_str() {
            Some("driver_default") => Ok(SettingValue::DriverDefault),
            _ => Err(bad()),
        },
        Step::ClockOffset | Step::MemoryOffset => value
            .as_i64()
            .and_then(|v| i32::try_from(v).ok())
            .map(SettingValue::Mhz)
            .ok_or_else(bad),
        Step::PowerLimit => value
            .as_u64()
            .and_then(|v| u32::try_from(v).ok())
            .map(SettingValue::Watts)
            .ok_or_else(bad),
        Step::Fan => match value {
            Value::String(s) if s == "auto" => Ok(SettingValue::Fan(FanSetting::Auto)),
            _ => value
                .as_u64()
                .and_then(|v| u8::try_from(v).ok())
                .map(|p| SettingValue::Fan(FanSetting::Percent(p)))
                .ok_or_else(bad),
        },
        Step::CpuGovernor => value
            .as_str()
            .map(|g| SettingValue::Governor(g.to_owned()))
            .ok_or_else(bad),
        Step::Smt => value.as_bool().map(SettingValue::Smt).ok_or_else(bad),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn all_steps() -> Vec<(Step, SettingValue)> {
        vec![
            (Step::LockedClocks, SettingValue::DriverDefault),
            (Step::ClockOffset, SettingValue::Mhz(-150)),
            (Step::MemoryOffset, SettingValue::Mhz(500)),
            (Step::PowerLimit, SettingValue::Watts(250)),
            (Step::Fan, SettingValue::Fan(FanSetting::Percent(70))),
            (
                Step::CpuGovernor,
                SettingValue::Governor("performance".into()),
            ),
            (Step::Smt, SettingValue::Smt(false)),
        ]
    }

    #[test]
    fn a_batch_round_trips_in_apply_order() {
        let line = encode_request(&all_steps());
        assert!(!line.contains('\n'));
        assert_eq!(parse_request(&line, Mode::Once).unwrap(), all_steps());
        // Out of order on the wire is put in apply order.
        let shuffled = r#"{"version":1,"type":"apply","steps":[{"step":"smt","value":true},{"step":"fan","value":"auto"},{"step":"clock_offset","value":150}]}"#;
        assert_eq!(
            parse_request(shuffled, Mode::Serve).unwrap(),
            [
                (Step::ClockOffset, SettingValue::Mhz(150)),
                (Step::Fan, SettingValue::Fan(FanSetting::Auto)),
                (Step::Smt, SettingValue::Smt(true)),
            ]
        );
    }

    #[test]
    fn strict_parsing_refuses_the_whole_batch() {
        let cases = [
            ("not json", "not JSON"),
            ("[]", "not a JSON object"),
            (r#"{"version":2,"type":"apply","steps":[]}"#, "version"),
            (r#"{"version":1,"type":"restore","steps":[]}"#, "\"apply\""),
            (r#"{"version":1,"type":"apply","steps":[]}"#, "no steps"),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"smt","value":true}],"gpu":1}"#,
                "unknown key \"gpu\"",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"smt","value":true,"gpu":1}]}"#,
                "unknown key \"gpu\"",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"voltage","value":1}]}"#,
                "unknown step",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"smt","value":"on"}]}"#,
                "not a value for smt",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"power_limit","value":-5}]}"#,
                "not a value for power_limit",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"clock_offset","value":3000000000}]}"#,
                "not a value for clock_offset",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"fan","value":300}]}"#,
                "not a value for fan",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"smt","value":true},{"step":"smt","value":false}]}"#,
                "twice",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"governor"}]}"#,
                "no value",
            ),
            (
                r#"{"version":1,"type":"apply","steps":[{"step":"locked_clocks"}]}"#,
                "only for a one-shot",
            ),
        ];
        for (line, needle) in cases {
            let e = parse_request(line, Mode::Serve).unwrap_err();
            assert!(e.contains(needle), "{line}: {e}");
        }
        let e = parse_request(
            r#"{"version":1,"type":"apply","steps":[{"step":"locked_clocks","value":"driver_default"}]}"#,
            Mode::Once,
        )
        .unwrap_err();
        assert!(e.contains("takes no value"), "{e}");
    }

    #[test]
    fn replies_round_trip() {
        let replies = [
            Reply::Step(StepReport {
                step: Step::CpuGovernor,
                target: SettingValue::Governor("performance".into()),
                outcome: StepOutcome::Applied {
                    prior: SettingValue::Governor("powersave".into()),
                },
            }),
            Reply::Step(StepReport {
                step: Step::LockedClocks,
                target: SettingValue::DriverDefault,
                outcome: StepOutcome::AppliedNoRestore,
            }),
            Reply::Step(StepReport {
                step: Step::Fan,
                target: SettingValue::Fan(FanSetting::Percent(70)),
                outcome: StepOutcome::Failed("not supported".into()),
            }),
            Reply::Step(StepReport {
                step: Step::Smt,
                target: SettingValue::Smt(false),
                outcome: StepOutcome::Skipped("SMT control is \"notsupported\"".into()),
            }),
            Reply::Ready,
            Reply::Restored {
                step: Step::PowerLimit,
                result: Ok(()),
            },
            Reply::Restored {
                step: Step::ClockOffset,
                result: Err("lost".into()),
            },
            Reply::Done,
            Reply::Error("bad".into()),
        ];
        for reply in replies {
            let line = encode_reply(&reply);
            assert!(!line.contains('\n'), "{line}");
            assert_eq!(parse_reply(&line).unwrap(), reply, "{line}");
        }
        assert!(parse_reply(r#"{"type":"surprise"}"#).is_err());
        assert!(parse_reply("{").is_err());
    }

    #[test]
    fn only_the_exact_restore_request_counts() {
        assert!(is_restore(&encode_restore()));
        assert!(is_restore(r#"{ "type": "restore" }"#));
        for line in [
            r#"{"type":"restore","steps":[]}"#,
            r#"{"type":"apply"}"#,
            "restore",
            "",
        ] {
            assert!(!is_restore(line), "{line}");
        }
    }
}

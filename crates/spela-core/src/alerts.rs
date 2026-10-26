//! The alert evaluator: GPU conditions worth a word in the TUI's top bar
//! and on Monitor.
//!
//! Ported from Go's `internal/overlay/alerts.go`, which decision 10 keeps
//! when the overlay goes. [`evaluate`] is a pure function of one sample.
//! The TUI's metrics thread runs it on each sweep and sends the alerts
//! with the metrics. `alerts_test.go` is the spec; its table is ported row
//! for row in the tests below.
//!
//! What changes from Go (decisions 8 and 60, architecture.md, alert
//! evaluator):
//!
//! - "Throttling" needs a thermal throttle reason from the driver. A
//!   temperature at or over the critical threshold with no reason is
//!   [`AlertKind::HighTemp`] at critical severity, labelled "High temp",
//!   with the message "GPU temperature high at T°C". Go said "GPU thermal
//!   throttling" and labelled it "Throttling".
//! - The power suggestion never goes past the GPU's maximum power limit,
//!   when the driver reports one. Go always proposed the limit plus 10 %.
//! - The graphics clock input, which Go filled in and never read, is gone.
//! - Go's suggestions used an em dash; these use a comma.

use std::fmt;

/// How urgent an alert is. Ordered: a later variant is more urgent.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Severity {
    /// Worth knowing. The top bar does not show it, as in Go.
    Info,
    /// Shown with ⚠.
    Warning,
    /// Shown with ✗.
    Critical,
}

impl Severity {
    /// Go's severity string: `info`, `warning` or `critical`.
    pub fn as_str(self) -> &'static str {
        match self {
            Severity::Info => "info",
            Severity::Warning => "warning",
            Severity::Critical => "critical",
        }
    }
}

impl fmt::Display for Severity {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// What an alert is about.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum AlertKind {
    /// The driver reports thermal throttling.
    ThermalThrottle,
    /// The temperature is at or over a threshold, with no throttle reason.
    HighTemp,
    /// The card runs at its power limit.
    PowerLimit,
    /// The fans run at their maximum.
    FanMaximum,
}

impl AlertKind {
    /// Go's type string: `thermal_throttle`, `power_limit` or
    /// `fan_maximum`. Go had one thermal type for both thermal alerts, so
    /// [`AlertKind::HighTemp`] keeps `thermal_throttle`.
    pub fn as_str(self) -> &'static str {
        match self {
            AlertKind::ThermalThrottle | AlertKind::HighTemp => "thermal_throttle",
            AlertKind::PowerLimit => "power_limit",
            AlertKind::FanMaximum => "fan_maximum",
        }
    }
}

impl fmt::Display for AlertKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// One active alert.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Alert {
    /// What it is about.
    pub kind: AlertKind,
    /// How urgent it is.
    pub severity: Severity,
    /// The full message, such as `GPU power-limited at 318W/320W`.
    pub message: String,
    /// What the user could do.
    pub suggestion: String,
}

impl Alert {
    /// The short word the top bar shows after the glyph, as Go's header
    /// labels it: `Throttling`, `High temp`, `Power limited` or `Fan max`.
    pub fn label(&self) -> &'static str {
        match self.kind {
            AlertKind::ThermalThrottle => "Throttling",
            AlertKind::HighTemp => "High temp",
            AlertKind::PowerLimit => "Power limited",
            AlertKind::FanMaximum => "Fan max",
        }
    }
}

/// The highest alert: the most severe, the first of those on a tie.
pub fn highest(alerts: &[Alert]) -> Option<&Alert> {
    let top = alerts.iter().map(|a| a.severity).max()?;
    alerts.iter().find(|a| a.severity == top)
}

/// When an alert fires.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct Thresholds {
    /// °C for a warning. Go's default: 80.
    pub thermal_warning: u32,
    /// °C for a critical alert. Go's default: 85.
    pub thermal_critical: u32,
    /// Percent of the power limit that counts as power-limited. Go's
    /// default: 95.
    pub power_margin_percent: f64,
    /// Fan percent that counts as maximum. Go's default: 95.
    pub fan_max_percent: u32,
}

impl Default for Thresholds {
    fn default() -> Thresholds {
        Thresholds {
            thermal_warning: 80,
            thermal_critical: 85,
            power_margin_percent: 95.0,
            fan_max_percent: 95,
        }
    }
}

/// What the evaluator looks at.
#[derive(Debug, Clone, Copy, Default, PartialEq)]
pub struct AlertInput {
    /// °C.
    pub temperature_c: u32,
    /// W.
    pub power_draw_w: f64,
    /// W, or 0 when unknown. With 0 only a power throttle reason raises a
    /// power alert.
    pub power_limit_w: f64,
    /// The highest power limit the GPU accepts in W, or 0 when unknown.
    /// The power suggestion never goes past it.
    pub power_max_limit_w: f64,
    /// Percent, or 0 when unknown.
    pub fan_percent: u32,
    /// The driver reports thermal throttling: hardware or software thermal
    /// slowdown.
    pub throttle_thermal: bool,
    /// The driver reports a power cap: the software power cap or the
    /// hardware power brake.
    pub throttle_power: bool,
}

/// The suggestion for a thermal alert at critical severity.
const COOL_DOWN: &str = "Improve case airflow or reduce power limit";

/// The active alerts for one sample, in Go's order: thermal, power, fan.
pub fn evaluate(input: &AlertInput, thresholds: &Thresholds) -> Vec<Alert> {
    let mut alerts = Vec::new();
    let t = input.temperature_c;
    if input.throttle_thermal {
        alerts.push(Alert {
            kind: AlertKind::ThermalThrottle,
            severity: Severity::Critical,
            message: format!("GPU thermal throttling at {t}°C"),
            suggestion: COOL_DOWN.into(),
        });
    } else if t >= thresholds.thermal_critical {
        alerts.push(Alert {
            kind: AlertKind::HighTemp,
            severity: Severity::Critical,
            message: format!("GPU temperature high at {t}°C"),
            suggestion: COOL_DOWN.into(),
        });
    } else if t >= thresholds.thermal_warning {
        alerts.push(Alert {
            kind: AlertKind::HighTemp,
            severity: Severity::Warning,
            message: format!("GPU temperature high at {t}°C"),
            suggestion: format!(
                "Approaching thermal limit, consider reducing power limit by {}W",
                suggested_power_reduction(input.power_draw_w)
            ),
        });
    }
    let limited = input.throttle_power
        || (input.power_limit_w > 0.0
            && input.power_draw_w / input.power_limit_w * 100.0 >= thresholds.power_margin_percent);
    if limited {
        alerts.push(Alert {
            kind: AlertKind::PowerLimit,
            severity: Severity::Warning,
            message: format!(
                "GPU power-limited at {:.0}W/{:.0}W",
                input.power_draw_w, input.power_limit_w
            ),
            suggestion: power_suggestion(input.power_limit_w, input.power_max_limit_w),
        });
    }
    if input.fan_percent >= thresholds.fan_max_percent {
        alerts.push(Alert {
            kind: AlertKind::FanMaximum,
            severity: Severity::Info,
            message: format!("GPU fans at {}%", input.fan_percent),
            suggestion: format!("GPU temperature: {t}°C, fans running at maximum capacity"),
        });
    }
    alerts
}

/// Go's power suggestion, the limit plus 10 %, capped at the maximum limit
/// when the driver reports one. A limit already at the maximum gets advice
/// it can follow.
fn power_suggestion(limit: f64, max: f64) -> String {
    let wanted = limit * 1.1;
    if max > 0.0 && limit >= max {
        return format!(
            "The power limit is already at the GPU's maximum of {max:.0}W; improve cooling instead"
        );
    }
    let target = if max > 0.0 { wanted.min(max) } else { wanted };
    format!("Increase power limit to {target:.0}W for potential FPS gain")
}

fn suggested_power_reduction(power_w: f64) -> u32 {
    let reduction = power_w * 0.1;
    if reduction < 10.0 {
        10
    } else {
        // Go's int(): the whole watts, cut toward zero.
        reduction as u32
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    struct Row {
        name: &'static str,
        input: AlertInput,
        thresholds: Thresholds,
        kinds: &'static [AlertKind],
        severities: &'static [Severity],
    }

    fn input(temp: u32, draw: f64, limit: f64, fan: u32) -> AlertInput {
        AlertInput {
            temperature_c: temp,
            power_draw_w: draw,
            power_limit_w: limit,
            fan_percent: fan,
            ..AlertInput::default()
        }
    }

    /// `alerts_test.go`'s `TestEvaluate`, row for row. The rows at 85 °C
    /// and over with no throttle reason expect `HighTemp` where Go
    /// expected its thermal-throttle type (decision 60).
    fn go_table() -> Vec<Row> {
        use AlertKind::*;
        use Severity::*;
        let d = Thresholds::default();
        vec![
            Row {
                name: "no alerts when metrics are normal",
                input: input(65, 200.0, 350.0, 50),
                thresholds: d,
                kinds: &[],
                severities: &[],
            },
            Row {
                name: "thermal warning at threshold",
                input: input(80, 250.0, 350.0, 70),
                thresholds: d,
                kinds: &[HighTemp],
                severities: &[Warning],
            },
            Row {
                name: "thermal critical at threshold",
                input: input(85, 300.0, 350.0, 90),
                thresholds: d,
                kinds: &[HighTemp],
                severities: &[Critical],
            },
            Row {
                name: "power limit alert",
                input: input(70, 340.0, 350.0, 60),
                thresholds: d,
                kinds: &[PowerLimit],
                severities: &[Warning],
            },
            Row {
                name: "fan maximum alert",
                input: input(75, 250.0, 350.0, 95),
                thresholds: d,
                kinds: &[FanMaximum],
                severities: &[Info],
            },
            Row {
                name: "multiple alerts: thermal critical + power limit + fan max",
                input: input(90, 345.0, 350.0, 100),
                thresholds: d,
                kinds: &[HighTemp, PowerLimit, FanMaximum],
                severities: &[Critical, Warning, Info],
            },
            Row {
                name: "custom thresholds",
                input: input(70, 200.0, 350.0, 80),
                thresholds: Thresholds {
                    thermal_warning: 65,
                    thermal_critical: 75,
                    power_margin_percent: 60.0,
                    fan_max_percent: 75,
                },
                kinds: &[HighTemp, FanMaximum],
                severities: &[Warning, Info],
            },
            Row {
                name: "zero power limit does not trigger power alert",
                input: input(60, 250.0, 0.0, 50),
                thresholds: d,
                kinds: &[],
                severities: &[],
            },
            Row {
                name: "thermal critical supersedes warning",
                input: input(90, 200.0, 350.0, 50),
                thresholds: d,
                kinds: &[HighTemp],
                severities: &[Critical],
            },
            Row {
                name: "NVML thermal throttle at low temperature",
                input: AlertInput {
                    throttle_thermal: true,
                    ..input(72, 200.0, 350.0, 60)
                },
                thresholds: d,
                kinds: &[ThermalThrottle],
                severities: &[Critical],
            },
            Row {
                name: "NVML power throttle below threshold",
                input: AlertInput {
                    throttle_power: true,
                    ..input(65, 200.0, 350.0, 50)
                },
                thresholds: d,
                kinds: &[PowerLimit],
                severities: &[Warning],
            },
            Row {
                name: "NVML thermal + power throttle",
                input: AlertInput {
                    throttle_thermal: true,
                    throttle_power: true,
                    ..input(78, 340.0, 350.0, 90)
                },
                thresholds: d,
                kinds: &[ThermalThrottle, PowerLimit],
                severities: &[Critical, Warning],
            },
        ]
    }

    #[test]
    fn evaluate_follows_the_go_table() {
        for row in go_table() {
            let alerts = evaluate(&row.input, &row.thresholds);
            let kinds: Vec<AlertKind> = alerts.iter().map(|a| a.kind).collect();
            let severities: Vec<Severity> = alerts.iter().map(|a| a.severity).collect();
            assert_eq!(kinds, row.kinds, "{}", row.name);
            assert_eq!(severities, row.severities, "{}", row.name);
            for a in &alerts {
                assert!(!a.message.is_empty(), "{}: message", row.name);
                assert!(!a.suggestion.is_empty(), "{}: suggestion", row.name);
                assert!(!a.suggestion.contains('\u{2014}'), "{}: em dash", row.name);
            }
        }
    }

    #[test]
    fn default_thresholds_are_gos() {
        let d = Thresholds::default();
        assert_eq!(d.thermal_warning, 80);
        assert_eq!(d.thermal_critical, 85);
        assert_eq!(d.power_margin_percent, 95.0);
        assert_eq!(d.fan_max_percent, 95);
    }

    #[test]
    fn severity_and_type_strings_are_gos() {
        assert_eq!(Severity::Info.to_string(), "info");
        assert_eq!(Severity::Warning.to_string(), "warning");
        assert_eq!(Severity::Critical.to_string(), "critical");
        assert_eq!(AlertKind::ThermalThrottle.to_string(), "thermal_throttle");
        assert_eq!(AlertKind::HighTemp.to_string(), "thermal_throttle");
        assert_eq!(AlertKind::PowerLimit.to_string(), "power_limit");
        assert_eq!(AlertKind::FanMaximum.to_string(), "fan_maximum");
    }

    #[test]
    fn heat_without_a_throttle_reason_is_high_temp() {
        let hot = input(86, 0.0, 0.0, 0);
        let alerts = evaluate(&hot, &Thresholds::default());
        assert_eq!(alerts[0].kind, AlertKind::HighTemp);
        assert_eq!(alerts[0].severity, Severity::Critical);
        assert_eq!(alerts[0].label(), "High temp");
        assert_eq!(alerts[0].message, "GPU temperature high at 86°C");
        let throttled = AlertInput {
            throttle_thermal: true,
            ..hot
        };
        let alerts = evaluate(&throttled, &Thresholds::default());
        assert_eq!(alerts[0].label(), "Throttling");
        assert_eq!(alerts[0].message, "GPU thermal throttling at 86°C");
        let warm = evaluate(&input(81, 50.0, 0.0, 0), &Thresholds::default());
        assert_eq!(
            warm[0].suggestion,
            "Approaching thermal limit, consider reducing power limit by 10W"
        );
        let warm = evaluate(&input(81, 318.0, 0.0, 0), &Thresholds::default());
        assert!(
            warm[0].suggestion.ends_with("by 31W"),
            "{}",
            warm[0].suggestion
        );
    }

    #[test]
    fn the_power_suggestion_stops_at_the_maximum_limit() {
        let at = |limit: f64, max: f64| AlertInput {
            power_max_limit_w: max,
            ..input(71, 318.0, limit, 40)
        };
        let s = |i: AlertInput| evaluate(&i, &Thresholds::default())[0].suggestion.clone();
        assert_eq!(
            s(at(320.0, 0.0)),
            "Increase power limit to 352W for potential FPS gain"
        );
        assert_eq!(
            s(at(320.0, 400.0)),
            "Increase power limit to 352W for potential FPS gain"
        );
        assert_eq!(
            s(at(320.0, 336.0)),
            "Increase power limit to 336W for potential FPS gain"
        );
        assert_eq!(
            s(at(320.0, 320.0)),
            "The power limit is already at the GPU's maximum of 320W; improve cooling instead"
        );
    }

    #[test]
    fn power_and_fan_and_the_highest() {
        let alerts = evaluate(&input(71, 318.0, 320.0, 100), &Thresholds::default());
        assert_eq!(alerts.len(), 2);
        assert_eq!(alerts[0].message, "GPU power-limited at 318W/320W");
        assert_eq!(alerts[1].kind, AlertKind::FanMaximum);
        assert_eq!(alerts[1].message, "GPU fans at 100%");
        assert_eq!(highest(&alerts).unwrap().label(), "Power limited");
        assert!(highest(&[]).is_none());
        let all = evaluate(&input(90, 345.0, 350.0, 100), &Thresholds::default());
        assert_eq!(highest(&all).unwrap().label(), "High temp");
    }
}

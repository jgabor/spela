//! The alert evaluator: GPU conditions worth a word in the TUI's top bar
//! and on Monitor.
//!
//! Ported from Go's `internal/overlay/alerts.go`, which decision 10 keeps
//! when the overlay goes. It is a pure function of one sample. The TUI's
//! metrics thread runs it on each sweep and sends the alerts with the
//! metrics.
//!
//! The one change from Go (decisions 8 and 60): a temperature at or over
//! the critical threshold with no thermal throttle reason from the driver
//! reads "High temp", not "Throttling". "Throttling" is kept for a real
//! throttle reason.

/// How urgent an alert is. Ordered: a later variant is more urgent.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum Severity {
    /// Worth knowing.
    Info,
    /// Shown with ⚠.
    Warning,
    /// Shown with ✗.
    Critical,
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
    alerts
        .iter()
        .rev()
        .max_by_key(|a| a.severity)
        .and_then(|top| alerts.iter().find(|a| a.severity == top.severity))
}

/// When an alert fires. Go's defaults.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct Thresholds {
    /// °C for a warning.
    pub thermal_warning: u32,
    /// °C for a critical alert.
    pub thermal_critical: u32,
    /// Percent of the power limit that counts as power-limited.
    pub power_margin_percent: f64,
    /// Fan percent that counts as maximum.
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
    /// W, or 0 when unknown.
    pub power_limit_w: f64,
    /// Percent, or 0 when unknown.
    pub fan_percent: u32,
    /// The driver reports thermal throttling.
    pub throttle_thermal: bool,
    /// The driver reports a power cap.
    pub throttle_power: bool,
}

/// The active alerts for one sample, in Go's order: thermal, power, fan.
pub fn evaluate(input: &AlertInput, thresholds: &Thresholds) -> Vec<Alert> {
    let mut alerts = Vec::new();
    let t = input.temperature_c;
    if input.throttle_thermal {
        alerts.push(Alert {
            kind: AlertKind::ThermalThrottle,
            severity: Severity::Critical,
            message: format!("GPU thermal throttling at {t}°C"),
            suggestion: "Improve case airflow or reduce power limit".into(),
        });
    } else if t >= thresholds.thermal_critical {
        alerts.push(Alert {
            kind: AlertKind::HighTemp,
            severity: Severity::Critical,
            message: format!("GPU temperature high at {t}°C"),
            suggestion: "Improve case airflow or reduce power limit".into(),
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
        let headroom = input.power_limit_w * 0.1;
        alerts.push(Alert {
            kind: AlertKind::PowerLimit,
            severity: Severity::Warning,
            message: format!(
                "GPU power-limited at {:.0}W/{:.0}W",
                input.power_draw_w, input.power_limit_w
            ),
            suggestion: format!(
                "Increase power limit to {:.0}W for potential FPS gain",
                input.power_limit_w + headroom
            ),
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

fn suggested_power_reduction(power_w: f64) -> u32 {
    let reduction = power_w * 0.1;
    if reduction < 10.0 {
        10
    } else {
        reduction as u32
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn calm_samples_raise_nothing() {
        let input = AlertInput {
            temperature_c: 54,
            power_draw_w: 182.0,
            power_limit_w: 320.0,
            fan_percent: 40,
            ..AlertInput::default()
        };
        assert!(evaluate(&input, &Thresholds::default()).is_empty());
    }

    #[test]
    fn heat_without_a_throttle_reason_is_high_temp() {
        let hot = AlertInput {
            temperature_c: 86,
            ..AlertInput::default()
        };
        let alerts = evaluate(&hot, &Thresholds::default());
        assert_eq!(alerts[0].kind, AlertKind::HighTemp);
        assert_eq!(alerts[0].severity, Severity::Critical);
        assert_eq!(alerts[0].label(), "High temp");
        let throttled = AlertInput {
            throttle_thermal: true,
            ..hot
        };
        assert_eq!(
            evaluate(&throttled, &Thresholds::default())[0].label(),
            "Throttling"
        );
    }

    #[test]
    fn power_and_fan_and_the_highest() {
        let input = AlertInput {
            temperature_c: 71,
            power_draw_w: 318.0,
            power_limit_w: 320.0,
            fan_percent: 100,
            ..AlertInput::default()
        };
        let alerts = evaluate(&input, &Thresholds::default());
        assert_eq!(alerts.len(), 2);
        assert_eq!(alerts[0].message, "GPU power-limited at 318W/320W");
        assert_eq!(
            alerts[0].suggestion,
            "Increase power limit to 352W for potential FPS gain"
        );
        assert_eq!(alerts[1].kind, AlertKind::FanMaximum);
        assert_eq!(highest(&alerts).unwrap().label(), "Power limited");
        assert!(highest(&[]).is_none());
    }
}

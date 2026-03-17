package overlay

import "fmt"

// AlertSeverity indicates the urgency of an alert.
type AlertSeverity int

const (
	AlertInfo AlertSeverity = iota
	AlertWarning
	AlertCritical
)

func (s AlertSeverity) String() string {
	switch s {
	case AlertInfo:
		return "info"
	case AlertWarning:
		return "warning"
	case AlertCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// AlertType identifies the category of alert.
type AlertType int

const (
	AlertThermalThrottle AlertType = iota
	AlertPowerLimit
	AlertFanMaximum
)

func (t AlertType) String() string {
	switch t {
	case AlertThermalThrottle:
		return "thermal_throttle"
	case AlertPowerLimit:
		return "power_limit"
	case AlertFanMaximum:
		return "fan_maximum"
	default:
		return "unknown"
	}
}

// Alert represents a detected performance condition.
type Alert struct {
	Type       AlertType
	Severity   AlertSeverity
	Message    string
	Suggestion string
}

// AlertThresholds configures the thresholds for alert detection.
type AlertThresholds struct {
	ThermalWarning  int     // °C, triggers warning. Default: 80
	ThermalCritical int     // °C, triggers critical. Default: 85
	PowerMarginPct  float64 // Percentage of power limit to trigger alert. Default: 95
	FanMaxPercent   int     // Fan speed % to consider "maximum". Default: 95
}

// DefaultThresholds returns sensible defaults for alert detection.
func DefaultThresholds() AlertThresholds {
	return AlertThresholds{
		ThermalWarning:  80,
		ThermalCritical: 85,
		PowerMarginPct:  95,
		FanMaxPercent:   95,
	}
}

// AlertInput contains the metrics needed for alert evaluation.
type AlertInput struct {
	Temperature   int
	PowerDraw     float64 // watts
	PowerLimit    float64 // watts
	GraphicsClock int     // MHz
	FanSpeed      int     // percentage 0-100

	// Throttle reasons from NVML. When true, these provide driver-confirmed
	// cause detection. When all false (nvidia-smi fallback), threshold-based
	// inference is used instead.
	ThrottleThermal bool // HW or SW thermal throttling active
	ThrottlePower   bool // SW power cap or HW power brake active
}

// Evaluate analyzes GPU metrics and returns any active alerts.
// It is a pure function with no side effects or state.
//
// When NVML throttle reasons are available (ThrottleThermal/ThrottlePower),
// they take precedence over threshold-based inference.
func Evaluate(input AlertInput, thresholds AlertThresholds) []Alert {
	var alerts []Alert

	// Thermal throttling detection
	if input.ThrottleThermal {
		alerts = append(alerts, Alert{
			Type:       AlertThermalThrottle,
			Severity:   AlertCritical,
			Message:    fmt.Sprintf("GPU thermal throttling at %d°C", input.Temperature),
			Suggestion: "Improve case airflow or reduce power limit",
		})
	} else if input.Temperature >= thresholds.ThermalCritical {
		alerts = append(alerts, Alert{
			Type:       AlertThermalThrottle,
			Severity:   AlertCritical,
			Message:    fmt.Sprintf("GPU thermal throttling at %d°C", input.Temperature),
			Suggestion: "Improve case airflow or reduce power limit",
		})
	} else if input.Temperature >= thresholds.ThermalWarning {
		alerts = append(alerts, Alert{
			Type:       AlertThermalThrottle,
			Severity:   AlertWarning,
			Message:    fmt.Sprintf("GPU temperature high at %d°C", input.Temperature),
			Suggestion: fmt.Sprintf("Approaching thermal limit — consider reducing power limit by %dW", suggestedPowerReduction(input.PowerDraw)),
		})
	}

	// Power limit detection
	if input.ThrottlePower {
		headroom := input.PowerLimit * 0.1
		alerts = append(alerts, Alert{
			Type:       AlertPowerLimit,
			Severity:   AlertWarning,
			Message:    fmt.Sprintf("GPU power-limited at %.0fW/%.0fW", input.PowerDraw, input.PowerLimit),
			Suggestion: fmt.Sprintf("Increase power limit to %.0fW for potential FPS gain", input.PowerLimit+headroom),
		})
	} else if input.PowerLimit > 0 {
		usagePct := (input.PowerDraw / input.PowerLimit) * 100
		if usagePct >= thresholds.PowerMarginPct {
			headroom := input.PowerLimit * 0.1
			alerts = append(alerts, Alert{
				Type:       AlertPowerLimit,
				Severity:   AlertWarning,
				Message:    fmt.Sprintf("GPU power-limited at %.0fW/%.0fW", input.PowerDraw, input.PowerLimit),
				Suggestion: fmt.Sprintf("Increase power limit to %.0fW for potential FPS gain", input.PowerLimit+headroom),
			})
		}
	}

	// Fan at maximum
	if input.FanSpeed >= thresholds.FanMaxPercent {
		alerts = append(alerts, Alert{
			Type:       AlertFanMaximum,
			Severity:   AlertInfo,
			Message:    fmt.Sprintf("GPU fans at %d%%", input.FanSpeed),
			Suggestion: fmt.Sprintf("GPU temperature: %d°C — fans running at maximum capacity", input.Temperature),
		})
	}

	return alerts
}

func suggestedPowerReduction(currentPower float64) int {
	reduction := currentPower * 0.1
	if reduction < 10 {
		return 10
	}
	return int(reduction)
}

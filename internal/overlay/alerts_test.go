package overlay

import "testing"

func TestEvaluate(t *testing.T) {
	defaults := DefaultThresholds()

	tests := []struct {
		name               string
		input              AlertInput
		thresholds         AlertThresholds
		expectedCount      int
		expectedTypes      []AlertType
		expectedSeverities []AlertSeverity
	}{
		{
			name: "no alerts when metrics are normal",
			input: AlertInput{
				Temperature: 65,
				PowerDraw:   200,
				PowerLimit:  350,
				FanSpeed:    50,
			},
			thresholds:    defaults,
			expectedCount: 0,
		},
		{
			name: "thermal warning at threshold",
			input: AlertInput{
				Temperature: 80,
				PowerDraw:   250,
				PowerLimit:  350,
				FanSpeed:    70,
			},
			thresholds:         defaults,
			expectedCount:      1,
			expectedTypes:      []AlertType{AlertThermalThrottle},
			expectedSeverities: []AlertSeverity{AlertWarning},
		},
		{
			name: "thermal critical at threshold",
			input: AlertInput{
				Temperature: 85,
				PowerDraw:   300,
				PowerLimit:  350,
				FanSpeed:    90,
			},
			thresholds:         defaults,
			expectedCount:      1,
			expectedTypes:      []AlertType{AlertThermalThrottle},
			expectedSeverities: []AlertSeverity{AlertCritical},
		},
		{
			name: "power limit alert",
			input: AlertInput{
				Temperature: 70,
				PowerDraw:   340,
				PowerLimit:  350,
				FanSpeed:    60,
			},
			thresholds:         defaults,
			expectedCount:      1,
			expectedTypes:      []AlertType{AlertPowerLimit},
			expectedSeverities: []AlertSeverity{AlertWarning},
		},
		{
			name: "fan maximum alert",
			input: AlertInput{
				Temperature: 75,
				PowerDraw:   250,
				PowerLimit:  350,
				FanSpeed:    95,
			},
			thresholds:         defaults,
			expectedCount:      1,
			expectedTypes:      []AlertType{AlertFanMaximum},
			expectedSeverities: []AlertSeverity{AlertInfo},
		},
		{
			name: "multiple alerts: thermal critical + power limit + fan max",
			input: AlertInput{
				Temperature: 90,
				PowerDraw:   345,
				PowerLimit:  350,
				FanSpeed:    100,
			},
			thresholds:         defaults,
			expectedCount:      3,
			expectedTypes:      []AlertType{AlertThermalThrottle, AlertPowerLimit, AlertFanMaximum},
			expectedSeverities: []AlertSeverity{AlertCritical, AlertWarning, AlertInfo},
		},
		{
			name: "custom thresholds",
			input: AlertInput{
				Temperature: 70,
				PowerDraw:   200,
				PowerLimit:  350,
				FanSpeed:    80,
			},
			thresholds: AlertThresholds{
				ThermalWarning:  65,
				ThermalCritical: 75,
				PowerMarginPct:  60,
				FanMaxPercent:   75,
			},
			expectedCount:      2,
			expectedTypes:      []AlertType{AlertThermalThrottle, AlertFanMaximum},
			expectedSeverities: []AlertSeverity{AlertWarning, AlertInfo},
		},
		{
			name: "zero power limit does not trigger power alert",
			input: AlertInput{
				Temperature: 60,
				PowerDraw:   250,
				PowerLimit:  0,
				FanSpeed:    50,
			},
			thresholds:    defaults,
			expectedCount: 0,
		},
		{
			name: "thermal critical supersedes warning",
			input: AlertInput{
				Temperature: 90,
				PowerDraw:   200,
				PowerLimit:  350,
				FanSpeed:    50,
			},
			thresholds:         defaults,
			expectedCount:      1,
			expectedTypes:      []AlertType{AlertThermalThrottle},
			expectedSeverities: []AlertSeverity{AlertCritical},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts := Evaluate(tt.input, tt.thresholds)
			if len(alerts) != tt.expectedCount {
				t.Fatalf("expected %d alerts, got %d: %v", tt.expectedCount, len(alerts), alerts)
			}
			for i, alert := range alerts {
				if i < len(tt.expectedTypes) && alert.Type != tt.expectedTypes[i] {
					t.Errorf("alert[%d]: expected type %s, got %s", i, tt.expectedTypes[i], alert.Type)
				}
				if i < len(tt.expectedSeverities) && alert.Severity != tt.expectedSeverities[i] {
					t.Errorf("alert[%d]: expected severity %s, got %s", i, tt.expectedSeverities[i], alert.Severity)
				}
				if alert.Message == "" {
					t.Errorf("alert[%d]: message should not be empty", i)
				}
				if alert.Suggestion == "" {
					t.Errorf("alert[%d]: suggestion should not be empty", i)
				}
			}
		})
	}
}

func TestDefaultThresholds(t *testing.T) {
	d := DefaultThresholds()
	if d.ThermalWarning != 80 {
		t.Errorf("expected ThermalWarning=80, got %d", d.ThermalWarning)
	}
	if d.ThermalCritical != 85 {
		t.Errorf("expected ThermalCritical=85, got %d", d.ThermalCritical)
	}
	if d.PowerMarginPct != 95 {
		t.Errorf("expected PowerMarginPct=95, got %f", d.PowerMarginPct)
	}
	if d.FanMaxPercent != 95 {
		t.Errorf("expected FanMaxPercent=95, got %d", d.FanMaxPercent)
	}
}

func TestAlertSeverityString(t *testing.T) {
	tests := []struct {
		severity AlertSeverity
		expected string
	}{
		{AlertInfo, "info"},
		{AlertWarning, "warning"},
		{AlertCritical, "critical"},
		{AlertSeverity(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.severity.String(); got != tt.expected {
			t.Errorf("AlertSeverity(%d).String() = %q, want %q", tt.severity, got, tt.expected)
		}
	}
}

func TestAlertTypeString(t *testing.T) {
	tests := []struct {
		alertType AlertType
		expected  string
	}{
		{AlertThermalThrottle, "thermal_throttle"},
		{AlertPowerLimit, "power_limit"},
		{AlertFanMaximum, "fan_maximum"},
		{AlertType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.alertType.String(); got != tt.expected {
			t.Errorf("AlertType(%d).String() = %q, want %q", tt.alertType, got, tt.expected)
		}
	}
}

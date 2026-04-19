package proton

import (
	"errors"
	"testing"
)

func TestParseDriverVersion_Shapes(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantMajor   int
		wantMinor   int
		wantPatch   int
		wantAvail   bool
		wantErr     bool
		wantMeetMin bool
	}{
		{
			name:      "three-component at minimum",
			raw:       "580.94.16",
			wantMajor: 580, wantMinor: 94, wantPatch: 16,
			wantAvail: true, wantMeetMin: true,
		},
		{
			name:      "two-component below minimum patch",
			raw:       "580.94",
			wantMajor: 580, wantMinor: 94, wantPatch: 0,
			wantAvail: true, wantMeetMin: false,
		},
		{
			name:      "beta with zero minor above minimum major",
			raw:       "585.0.0",
			wantMajor: 585, wantMinor: 0, wantPatch: 0,
			wantAvail: true, wantMeetMin: true,
		},
		{
			name:      "whitespace-padded from nvidia-smi fallback",
			raw:       "  580.94.16  ",
			wantMajor: 580, wantMinor: 94, wantPatch: 16,
			wantAvail: true, wantMeetMin: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseDriverVersion(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (v=%+v)", v)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Available != tt.wantAvail {
				t.Errorf("Available = %v, want %v", v.Available, tt.wantAvail)
			}
			if v.Major != tt.wantMajor || v.Minor != tt.wantMinor || v.Patch != tt.wantPatch {
				t.Errorf("components = %d.%d.%d, want %d.%d.%d",
					v.Major, v.Minor, v.Patch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
			if got := v.MeetsMinimum(); got != tt.wantMeetMin {
				t.Errorf("MeetsMinimum() = %v, want %v (raw %q vs min %q)",
					got, tt.wantMeetMin, tt.raw, MinDriverVersion)
			}
		})
	}
}

func TestParseDriverVersion_Empty_ReturnsUnavailable(t *testing.T) {
	v, err := ParseDriverVersion("")
	if !errors.Is(err, ErrDriverUnavailable) {
		t.Fatalf("err = %v, want ErrDriverUnavailable", err)
	}
	if v.Available {
		t.Errorf("empty input produced Available=true: %+v", v)
	}
	if v.MeetsMinimum() {
		t.Error("unavailable driver version should not meet minimum")
	}
	if v.String() != "unavailable" {
		t.Errorf("String() = %q, want %q", v.String(), "unavailable")
	}

	// Whitespace-only is also unavailable.
	if _, err := ParseDriverVersion("   "); !errors.Is(err, ErrDriverUnavailable) {
		t.Errorf("whitespace-only err = %v, want ErrDriverUnavailable", err)
	}
}

func TestParseDriverVersion_Garbage_ReturnsTypedError(t *testing.T) {
	v, err := ParseDriverVersion("not-a-version")
	if err == nil {
		t.Fatalf("want parse error, got nil (v=%+v)", v)
	}
	if errors.Is(err, ErrDriverUnavailable) {
		t.Errorf("garbage should not be reported as unavailable, got: %v", err)
	}
}

func TestDriverVersion_Compare(t *testing.T) {
	lo, _ := ParseDriverVersion("580.94.15")
	eq, _ := ParseDriverVersion("580.94.16")
	hi, _ := ParseDriverVersion("580.95.0")
	var unavail DriverVersion

	cases := []struct {
		a, b DriverVersion
		want int
		desc string
	}{
		{lo, eq, -1, "patch below minimum"},
		{eq, eq, 0, "identical"},
		{hi, eq, 1, "minor above minimum"},
		{unavail, eq, -1, "unavailable < available"},
		{eq, unavail, 1, "available > unavailable"},
		{unavail, unavail, 0, "both unavailable"},
	}
	for _, c := range cases {
		if got := c.a.Compare(c.b); got != c.want {
			t.Errorf("%s: %s.Compare(%s) = %d, want %d",
				c.desc, c.a.String(), c.b.String(), got, c.want)
		}
	}
}

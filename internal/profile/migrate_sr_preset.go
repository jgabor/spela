package profile

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// profileYAML unmarshals profiles including the legacy sr_model_preset field.
type profileYAML struct {
	Name      string          `yaml:"name,omitempty"`
	DLSS      dlssYAML        `yaml:"dlss,omitempty"`
	GPU       GPUSettings     `yaml:"gpu,omitempty"`
	CPU       CPUSettings     `yaml:"cpu,omitempty"`
	Proton    ProtonSettings  `yaml:"proton,omitempty"`
	Overlay   OverlaySettings `yaml:"overlay,omitempty"`
	Overrides map[string]bool `yaml:"overrides,omitempty"`
}

type dlssYAML struct {
	SRMode        DLSSMode   `yaml:"sr_mode,omitempty"`
	SRPreset      DLSSPreset `yaml:"sr_preset,omitempty"`
	SRModelPreset string     `yaml:"sr_model_preset,omitempty"`
	SROverride    bool       `yaml:"sr_override,omitempty"`
	RRMode        DLSSMode   `yaml:"rr_mode,omitempty"`
	RRPreset      DLSSPreset `yaml:"rr_preset,omitempty"`
	RROverride    bool       `yaml:"rr_override,omitempty"`
	FGEnabled     bool       `yaml:"fg_enabled,omitempty"`
	FGOverride    bool       `yaml:"fg_override,omitempty"`
	MultiFrame    int        `yaml:"multi_frame,omitempty"`
	Indicator     bool       `yaml:"indicator,omitempty"`
	FGIndicator   bool       `yaml:"fg_indicator,omitempty"`
}

func unmarshalProfileYAML(data []byte) (*Profile, error) {
	var doc profileYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	p := &Profile{
		Name:      doc.Name,
		GPU:       doc.GPU,
		CPU:       doc.CPU,
		Proton:    doc.Proton,
		Overlay:   doc.Overlay,
		Overrides: doc.Overrides,
		DLSS: DLSSSettings{
			SRMode:      doc.DLSS.SRMode,
			SRPreset:    doc.DLSS.SRPreset,
			SROverride:  doc.DLSS.SROverride,
			RRMode:      doc.DLSS.RRMode,
			RRPreset:    doc.DLSS.RRPreset,
			RROverride:  doc.DLSS.RROverride,
			FGEnabled:   doc.DLSS.FGEnabled,
			FGOverride:  doc.DLSS.FGOverride,
			MultiFrame:  doc.DLSS.MultiFrame,
			Indicator:   doc.DLSS.Indicator,
			FGIndicator: doc.DLSS.FGIndicator,
		},
	}
	migrateSRPreset(p, doc.DLSS.SRModelPreset)
	return p, nil
}

// NormalizeSRPreset canonicalizes user input for sr_preset.
func NormalizeSRPreset(s string) DLSSPreset {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if lower == "default" {
		return DLSSPresetDefault
	}
	if lower == "auto" {
		return DLSSPresetAuto
	}
	if len(s) == 1 {
		return DLSSPreset(strings.ToUpper(s))
	}
	return DLSSPreset(s)
}

func legacyModelPresetToSRPreset(s string) DLSSPreset {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto":
		return DLSSPresetAuto
	case "k":
		return DLSSPresetK
	case "l":
		return DLSSPresetL
	case "m":
		return DLSSPresetM
	default:
		return ""
	}
}

// migrateSRPreset folds legacy sr_model_preset into sr_preset and updates overrides.
// When both fields are set, the legacy model preset wins (matching prior apply precedence).
func migrateSRPreset(p *Profile, legacyModelPreset string) {
	if p == nil {
		return
	}

	if legacyModelPreset != "" {
		migrated := legacyModelPresetToSRPreset(legacyModelPreset)
		if migrated != "" {
			p.DLSS.SRPreset = migrated
		}
	}

	if p.DLSS.SRPreset != "" {
		p.DLSS.SRPreset = NormalizeSRPreset(string(p.DLSS.SRPreset))
	}

	const legacyField = "dlss.sr_model_preset"
	if p.Overrides == nil {
		return
	}
	if p.Overrides[legacyField] {
		if !p.Overrides[FieldDLSSSRPreset] {
			p.Overrides[FieldDLSSSRPreset] = true
		}
		delete(p.Overrides, legacyField)
	}
}

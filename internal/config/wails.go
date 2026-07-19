package config

// CatalogSection is the Wails-facing settings section descriptor.
type CatalogSection struct {
	ID      int             `json:"id"`
	Title   string          `json:"title"`
	Options []CatalogOption `json:"options"`
}

// CatalogOption is the Wails-facing option descriptor.
type CatalogOption struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Choices     []string `json:"choices,omitempty"`
}

// WailsCatalog returns the GUI-visible settings catalog.
func WailsCatalog() []CatalogSection {
	sections := Sections(VisibilityGUI)
	result := make([]CatalogSection, len(sections))
	for sectionIndex, section := range sections {
		result[sectionIndex] = CatalogSection{ID: int(section.ID), Title: section.Title, Options: make([]CatalogOption, len(section.Options))}
		for optionIndex, option := range section.Options {
			result[sectionIndex].Options[optionIndex] = CatalogOption{Key: option.JSONKey, Label: option.Label, Description: option.Description, Type: wailsType(option.Kind), Choices: option.Choices}
		}
	}
	return result
}

func wailsType(kind Kind) string {
	switch kind {
	case KindBool:
		return "toggle"
	case KindPath:
		return "path"
	default:
		return "select"
	}
}

//go:build dev || production || bindings

package settings

// CatalogSection is the Wails-facing settings section descriptor.
type CatalogSection struct {
	ID      int                `json:"id"`
	Title   string             `json:"title"`
	Options []CatalogOption    `json:"options"`
}

// CatalogOption is the Wails-facing option descriptor.
type CatalogOption struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Choices     []string `json:"choices,omitempty"`
}

// WailsCatalog returns the settings catalog for the GUI frontend.
func WailsCatalog() []CatalogSection {
	catalog := Catalog()
	out := make([]CatalogSection, len(catalog))
	for i, section := range catalog {
		opts := make([]CatalogOption, len(section.Options))
		for j, opt := range section.Options {
			opts[j] = CatalogOption{
				Key:         opt.JSONKey,
				Label:       opt.Label,
				Description: opt.Description,
				Type:        wailsOptionType(opt.Kind),
				Choices:     opt.Choices,
			}
		}
		out[i] = CatalogSection{
			ID:      int(section.ID),
			Title:   section.Title,
			Options: opts,
		}
	}
	return out
}

func wailsOptionType(kind Kind) string {
	switch kind {
	case KindBool:
		return "toggle"
	case KindPath:
		return "path"
	case KindEnum, KindInt:
		return "select"
	default:
		return "select"
	}
}

package config

import (
	"os"
	"strings"
	"testing"
)

// docsTableRows returns the cells of each Markdown table row in the section
// that starts at heading (the whole file when heading is empty).
func docsTableRows(t *testing.T, heading string) [][]string {
	t.Helper()
	data, err := os.ReadFile("../../docs/configuration.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if heading != "" {
		start := strings.Index(text, heading+"\n")
		if start < 0 {
			t.Fatalf("docs/configuration.md has no %q section", heading)
		}
		text = text[start+len(heading):]
		if end := strings.Index(text, "\n## "); end >= 0 {
			text = text[:end]
		}
	}
	var rows [][]string
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "|---") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), " | ")
		for index := range cells {
			cells[index] = strings.TrimSpace(cells[index])
		}
		rows = append(rows, cells)
	}
	return rows[1:] // the header row
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func TestConfigurationDocsMatchTheOptionCatalog(t *testing.T) {
	documented := map[string][]string{}
	for _, cells := range docsTableRows(t, "") {
		if len(cells) == 8 {
			documented[strings.Trim(cells[1], "`")] = cells
		}
	}
	for _, option := range Options() {
		cells, ok := documented[option.Key]
		if !ok {
			t.Errorf("%s is missing from docs/configuration.md", option.Key)
			continue
		}
		delete(documented, option.Key)
		for column, visibility := range map[int]Visibility{5: VisibilityCLI, 6: VisibilityTUI} {
			if want := yesNo(option.Visibility.Includes(visibility)); cells[column] != want {
				t.Errorf("%s column %d = %q, want %q", option.Key, column, cells[column], want)
			}
		}
	}
	for key := range documented {
		t.Errorf("docs/configuration.md documents %s, which is not an option", key)
	}
}

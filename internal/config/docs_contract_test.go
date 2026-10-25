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
		if len(cells) == 10 {
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
		for column, visibility := range map[int]Visibility{6: VisibilityCLI, 7: VisibilityTUI, 8: VisibilityGUI} {
			if want := yesNo(option.Visibility.Includes(visibility)); cells[column] != want {
				t.Errorf("%s column %d = %q, want %q", option.Key, column, cells[column], want)
			}
		}
	}
	for key := range documented {
		t.Errorf("docs/configuration.md documents %s, which is not an option", key)
	}
}

// Every option is in both interfaces, or the parity table names it with the
// reason it is in only one.
func TestSettingsParityTableMatchesBothInterfaces(t *testing.T) {
	rows := docsTableRows(t, "## Settings parity")
	terminal, graphical := Sections(VisibilityTUI), Sections(VisibilityGUI)
	if len(rows) != len(terminal) {
		t.Fatalf("parity table has %d groups, catalog has %d", len(rows), len(terminal))
	}
	for index, cells := range rows {
		if len(cells) != 3 {
			t.Fatalf("parity row %v must have three cells", cells)
		}
		if cells[0] != terminal[index].Title {
			t.Errorf("parity row %d is %q, want %q", index, cells[0], terminal[index].Title)
		}
		inTerminal := map[string]bool{}
		var both []string
		for _, option := range terminal[index].Options {
			inTerminal[option.Key] = true
			if !option.Visibility.Includes(VisibilityGUI) {
				t.Errorf("%s is in the TUI only; the parity table has no reason for it", option.Key)
			}
			both = append(both, option.Label)
		}
		if got := strings.Join(both, ", "); cells[1] != got {
			t.Errorf("%s in both = %q, want %q", cells[0], cells[1], got)
		}
		var graphicalOnly []string
		for _, option := range graphical[index].Options {
			if !inTerminal[option.Key] {
				graphicalOnly = append(graphicalOnly, option.Label)
			}
		}
		reasons := cells[2]
		for _, label := range graphicalOnly {
			if !strings.Contains(reasons, label+": ") {
				t.Errorf("%s is in the GUI only; the parity table gives no reason", label)
			}
		}
		if len(graphicalOnly) == 0 && reasons != "" {
			t.Errorf("%s lists a GUI-only option the catalog does not have: %q", cells[0], reasons)
		}
	}
}

package steam

import (
	"strings"
	"testing"
)

func TestParseVDFSimple(t *testing.T) {
	input := `"AppState"
{
	"appid"		"1091500"
	"name"		"Cyberpunk 2077"
	"StateFlags"		"4"
}`
	node, err := ParseVDF(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseVDF() error = %v", err)
	}

	appState := node.GetNode("AppState")
	if appState == nil {
		t.Fatal("expected AppState node")
	}
	if got := appState.GetString("appid"); got != "1091500" {
		t.Errorf("appid = %q, want %q", got, "1091500")
	}
	if got := appState.GetString("name"); got != "Cyberpunk 2077" {
		t.Errorf("name = %q, want %q", got, "Cyberpunk 2077")
	}
}

func TestParseVDFNested(t *testing.T) {
	input := `"libraryfolders"
{
	"0"
	{
		"path"		"/home/user/.steam/steam"
		"apps"
		{
			"1091500"		"12345"
		}
	}
}`
	node, err := ParseVDF(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseVDF() error = %v", err)
	}

	lf := node.GetNode("libraryfolders")
	if lf == nil {
		t.Fatal("expected libraryfolders node")
	}

	zero := lf.GetNode("0")
	if zero == nil {
		t.Fatal("expected node '0'")
	}
	if got := zero.GetString("path"); got != "/home/user/.steam/steam" {
		t.Errorf("path = %q, want %q", got, "/home/user/.steam/steam")
	}

	apps := zero.GetNode("apps")
	if apps == nil {
		t.Fatal("expected apps node")
	}
	if got := apps.GetString("1091500"); got != "12345" {
		t.Errorf("app 1091500 = %q, want %q", got, "12345")
	}
}

func TestParseVDFUnexpectedClosingBrace(t *testing.T) {
	input := `}`
	_, err := ParseVDF(strings.NewReader(input))
	if err == nil {
		t.Error("expected error for unexpected closing brace")
	}
}

func TestParseVDFComments(t *testing.T) {
	input := `// This is a comment
"root"
{
	// Another comment
	"key"		"value"
}`
	node, err := ParseVDF(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseVDF() error = %v", err)
	}

	root := node.GetNode("root")
	if root == nil {
		t.Fatal("expected root node")
	}
	if got := root.GetString("key"); got != "value" {
		t.Errorf("key = %q, want %q", got, "value")
	}
}

func TestGetNodeMissing(t *testing.T) {
	node := make(VDFNode)
	if got := node.GetNode("missing"); got != nil {
		t.Errorf("GetNode(missing) = %v, want nil", got)
	}
	if got := node.GetString("missing"); got != "" {
		t.Errorf("GetString(missing) = %q, want empty", got)
	}
}

func TestGetNodeWrongType(t *testing.T) {
	node := VDFNode{"key": "string_value"}
	if got := node.GetNode("key"); got != nil {
		t.Errorf("GetNode on string value = %v, want nil", got)
	}

	node2 := VDFNode{"key": make(VDFNode)}
	if got := node2.GetString("key"); got != "" {
		t.Errorf("GetString on node value = %q, want empty", got)
	}
}

func TestTokenizeLineUnquoted(t *testing.T) {
	tokens := tokenizeLine(`key value`)
	if len(tokens) != 2 || tokens[0] != "key" || tokens[1] != "value" {
		t.Errorf("tokenizeLine = %v, want [key value]", tokens)
	}
}

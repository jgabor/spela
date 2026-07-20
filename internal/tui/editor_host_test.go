package tui

import "testing"

func TestEditorHostSupportedValueKinds(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		spec EditorSpec
		from string
		want string
	}{
		{name: "bool", spec: EditorSpec{Key: "enabled", Kind: EditorBool}, from: "false", want: "true"},
		{name: "choice", spec: EditorSpec{Key: "level", Kind: EditorChoice, Choices: []string{"low", "high"}}, from: "low", want: "high"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var editor EditorHost
			editor.Begin(test.spec, test.from)
			editor.Cycle(1)
			if value, err := editor.Commit(); err != nil || value != test.want {
				t.Fatalf("commit = %q, %v; want %q", value, err, test.want)
			}
		})
	}
}

func TestEditorHostInvalidInputRetainsRecoverableDraft(t *testing.T) {
	t.Parallel()
	var editor EditorHost
	editor.Begin(EditorSpec{Key: "limit", Kind: EditorInteger}, "10")
	editor.Set("invalid")
	if _, err := editor.Commit(); err == nil {
		t.Fatal("invalid integer committed")
	}
	if !editor.Active() || editor.Value() != "invalid" || editor.Error() == nil {
		t.Fatalf("invalid draft was not retained: %#v", editor)
	}
}

func TestEditorHostCancelRestoresOriginalWithoutCommit(t *testing.T) {
	t.Parallel()
	var editor EditorHost
	editor.Begin(EditorSpec{Key: "path", Kind: EditorPath}, "/saved")
	editor.Set("/draft")
	editor.Cancel()
	if editor.Active() || editor.Value() != "/saved" || editor.Dirty() {
		t.Fatalf("cancelled editor = %#v", editor)
	}
}

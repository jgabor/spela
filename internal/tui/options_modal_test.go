package tui

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jgabor/spela/internal/config"
	"github.com/jgabor/spela/internal/nav"
)

func TestSettingsChangeStaysOffTheLiveConfigUntilSavedThenCommits(t *testing.T) {
	styles := NewStyles(DefaultTheme)
	live := config.Default()
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(live)
	modal.SyncNavSection(nav.SettingsStartup)
	modal.optionCursor = 0
	modal.cycleValue(1)
	if !live.RescanOnStartup || !modal.config.RescanOnStartup || modal.draft.RescanOnStartup {
		t.Fatal("Settings edit escaped the draft boundary")
	}

	modal.SetSaveConfig(func(*config.Config) error { return nil })
	modal, command := modal.save()
	message, ok := command().(optionsSavedMsg)
	if !ok || message.config.RescanOnStartup {
		t.Fatalf("save message = %#v", message)
	}
	layout := LayoutModel{styles: styles, config: live, pane: resourcePaneModel{settings: modal}, messageBar: NewMessageBar(styles)}
	layout, _ = layout.handleAppMessages(message, nil)
	if layout.config.RescanOnStartup || layout.pane.settings.draft.RescanOnStartup {
		t.Fatal("successful Settings save was not committed and kept visible")
	}
}

func TestSettingsTUIInventoryContainsOnlyRuntimeBackedControls(t *testing.T) {
	sections := config.Sections(config.VisibilityTUI)
	var keys []string
	for _, section := range sections {
		for _, option := range section.Options {
			keys = append(keys, option.Key)
		}
	}
	if got, want := strings.Join(keys, ","), "compact_mode,confirm_destructive,rescan_on_startup,auto_update_dlls,check_updates,steam_path,additional_library_paths,dll_cache_path,backup_path,shader_cache,auto_refresh_manifest,manifest_refresh_hours,dll_manifest_url,log_level"; got != want {
		t.Fatalf("visible Settings keys = %q, want %q", got, want)
	}
}

func TestSettingsDetailWrapsTheDescriptionIntoRemainingRows(t *testing.T) {
	modal := NewOptionsModal(NewStyles(DefaultTheme))
	modal.OpenEmbedded(config.Default())
	modal.SyncNavSection(nav.SettingsPaths)
	for index, option := range modal.sections[modal.sectionCursor].Options {
		if option.Key == "backup_path" {
			modal.optionCursor = index
		}
	}
	modal.SetSize(44, 20)
	view := ansi.Strip(modal.DetailView())
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Fatalf("line %q is wider than the pane", line)
		}
	}
	if flat := strings.Join(strings.Fields(view), " "); !strings.Contains(flat, "Earlier backups stay restorable.") {
		t.Fatalf("description was cut:\n%s", view)
	}
	modal.SetSize(44, 6)
	if lines := strings.Split(modal.DetailView(), "\n"); len(lines) > 6 {
		t.Fatalf("detail uses %d rows, want at most 6: %q", len(lines), lines)
	}
}

func TestSettingsDetailWrapsALongValue(t *testing.T) {
	configuration := config.Default()
	configuration.DLLManifestURL = "https://mirror.example.test/spela/releases/latest/download/manifest.json"
	modal := NewOptionsModal(NewStyles(DefaultTheme))
	modal.OpenEmbedded(configuration)
	modal.SyncNavSection(nav.SettingsDLLPolicy)
	for index, option := range modal.sections[modal.sectionCursor].Options {
		if option.Key == "dll_manifest_url" {
			modal.optionCursor = index
		}
	}
	modal.SetSize(44, 20)
	view := ansi.Strip(modal.DetailView())
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > 40 || strings.Contains(line, "…") {
			t.Fatalf("value line %q was cut", line)
		}
	}
	if flat := strings.ReplaceAll(view, "\n", ""); !strings.Contains(flat, configuration.DLLManifestURL) {
		t.Fatalf("value not shown in full:\n%s", view)
	}
}

func TestSettingsPathEditorKeepsErrorAndFitsThePane(t *testing.T) {
	modal := NewOptionsModal(NewStyles(DefaultTheme))
	modal.OpenEmbedded(config.Default())
	modal.SyncNavSection(nav.SettingsPaths)
	for index, option := range modal.sections[modal.sectionCursor].Options {
		if option.Key == "backup_path" {
			modal.optionCursor = index
		}
	}
	modal.SetSize(60, 20)
	modal, _ = modal.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	modal, _ = modal.Update(tea.KeyPressMsg{Code: 'b', Text: "backups"})
	if view := ansi.Strip(modal.DetailView()); strings.Contains(view, "…") {
		t.Fatalf("short value was truncated:\n%s", view)
	}
	modal, _ = modal.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	// A cursor blink or other non-key message must not hide the error.
	modal, _ = modal.Update(struct{}{})
	if view := modal.DetailView(); !strings.Contains(view, "backup_path must be an absolute path") {
		t.Fatalf("validation error not shown:\n%s", ansi.Strip(view))
	}
	if modal.draft.BackupPath != "" {
		t.Fatalf("relative backup path reached the draft: %q", modal.draft.BackupPath)
	}
}

func TestSettingsSaveFailureReturnsToStoredValueKeepsFocusAndShowsError(t *testing.T) {
	styles := NewStyles(DefaultTheme)
	live := config.Default()
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(live)
	modal.SyncNavSection(nav.SettingsStartup)
	modal.optionCursor = 0
	modal.cycleValue(1)
	modal.SetSaveConfig(func(*config.Config) error { return errors.New("read-only") })
	wantSection, wantOption := modal.sectionCursor, modal.optionCursor
	modal, command := modal.save()
	message := command().(optionsSaveErrorMsg)
	layout := LayoutModel{styles: styles, config: live, pane: resourcePaneModel{settings: modal}, messageBar: NewMessageBar(styles)}
	layout, _ = layout.handleAppMessages(message, nil)
	settings := layout.pane.settings
	if settings.saving || settings.modified || !settings.draft.RescanOnStartup || !settings.config.RescanOnStartup {
		t.Fatal("failed Settings save kept the unsaved value or changed the stored one")
	}
	if settings.sectionCursor != wantSection || settings.optionCursor != wantOption || settings.saveError == nil {
		t.Fatal("failed Settings save lost focus or inline error")
	}
	if view := stripANSI(settings.DetailView()); !strings.Contains(view, "On") || !strings.Contains(view, "Not saved: read-only") || strings.Contains(view, "Draft") {
		t.Fatalf("failed Settings Detail view:\n%s", view)
	}
}

func TestSettingsPathEditingPreservesPrintableInputAndDocumentsKeys(t *testing.T) {
	modal := NewOptionsModal(NewStyles(DefaultTheme))
	modal.OpenEmbedded(config.Default())
	modal.SyncNavSection(nav.SettingsPaths)
	modal, _ = modal.Update(keyMsg("enter"))
	modal.pathInput.SetValue("  /Steam Library/#1  ")
	if view := stripANSI(modal.DetailView()); !strings.Contains(view, "Tab control  Enter apply  Esc cancel") {
		t.Fatalf("path editor omitted key contract:\n%s", view)
	}
	modal, _ = modal.Update(keyMsg("enter"))
	if modal.draft.SteamPath != "  /Steam Library/#1  " {
		t.Fatalf("committed path = %q", modal.draft.SteamPath)
	}
	modal, _ = modal.Update(keyMsg("enter"))
	modal.pathInput.SetValue("discard me")
	modal, _ = modal.UpdateAction(ActionEditCancel)
	if modal.draft.SteamPath != "  /Steam Library/#1  " {
		t.Fatalf("cancel changed path to %q", modal.draft.SteamPath)
	}
}

func TestSettingsSavePreservesUnchangedSupportedYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	configuration := config.Default()
	configuration.Theme = "light"
	configuration.ConfirmDestructive = false
	configuration.AutoUpdateDLLs = true
	configuration.DLLCachePath = "/unchanged/cache"
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	modal := NewOptionsModal(NewStyles(DefaultTheme))
	modal.OpenEmbedded(loaded)
	modal.SyncNavSection(nav.SettingsStartup)
	modal.cycleValue(1)
	modal, command := modal.save()
	message := command().(optionsSavedMsg)
	modal.CompleteSave(message.config)
	restarted, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if restarted.RescanOnStartup || restarted.Theme != "light" || restarted.ConfirmDestructive || !restarted.AutoUpdateDLLs || restarted.DLLCachePath != "/unchanged/cache" {
		t.Fatalf("restart config lost values: %+v", restarted)
	}
	if _, err := os.Stat(home + "/spela/config.yaml"); err != nil {
		t.Fatalf("saved config missing: %v", err)
	}
}

func TestOptionsModal_EmbeddedShowsSingleSection(t *testing.T) {
	styles := NewStyles(DefaultTheme)
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(config.Default())
	modal.SyncNavSection(nav.SettingsStartup)

	view := modal.renderOptionsBody()
	if !strings.Contains(stripANSI(view), "Re-scan on st") {
		t.Fatalf("expected startup option in view, got %q", view)
	}
	if strings.Contains(view, "Steam path") {
		t.Fatal("paths section should not render when display section is active")
	}
}

func TestOptionsModal_SaveUsesInvocationSnapshot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	configuration := config.Default()
	modal := NewOptionsModal(NewStyles(DefaultTheme))
	modal.OpenEmbedded(configuration)
	modal.modified = true

	modal, command := modal.save()
	configuration.RescanOnStartup = false
	if _, ok := command().(optionsSavedMsg); !ok {
		t.Fatal("save command did not report success")
	}
	persisted, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !persisted.RescanOnStartup || !modal.saving {
		t.Fatalf("persisted live mutation or lost pending state: show hints=%v saving=%v", persisted.RescanOnStartup, modal.saving)
	}
}

func TestOptionsModal_ChangeDuringAWriteIsSavedAfterIt(t *testing.T) {
	styles := NewStyles(DefaultTheme)
	configuration := config.Default()
	modal := NewOptionsModal(styles)
	modal.OpenEmbedded(configuration)
	modal.SyncNavSection(nav.SettingsStartup)
	modal.optionCursor = 0
	var writes int
	modal.SetSaveConfig(func(*config.Config) error { writes++; return nil })

	modal, first := modal.UpdateAction(ActionDetailCycle)
	modal, second := modal.UpdateAction(ActionDetailCycle)
	if first == nil || second != nil || !modal.saving || configuration.RescanOnStartup != true {
		t.Fatal("a second change during a write was refused, started a parallel write, or reached the live config")
	}

	layout := LayoutModel{config: configuration, pane: resourcePaneModel{settings: modal}, messageBar: NewMessageBar(styles)}
	layout, commands := layout.handleAppMessages(first(), nil)
	if !layout.pane.settings.saving || !layout.pane.settings.modified {
		t.Fatal("the change made during the write was not scheduled")
	}
	var followUp tea.Cmd
	for _, command := range commands {
		if command != nil {
			if saved, ok := command().(optionsSavedMsg); ok {
				followUp = func() tea.Msg { return saved }
			}
		}
	}
	if followUp == nil {
		t.Fatal("no follow-up write after the first one finished")
	}
	layout, _ = layout.handleAppMessages(followUp(), nil)
	if layout.pane.settings.saving || layout.pane.settings.modified || writes != 2 || !configsEqual(layout.config, layout.pane.settings.config) {
		t.Fatalf("after both writes: saving=%v modified=%v writes=%d", layout.pane.settings.saving, layout.pane.settings.modified, writes)
	}
}

func TestOptionsModal_CompactWidthKeepsValuesOnOptionRows(t *testing.T) {
	modal := NewOptionsModal(NewStyles(DefaultTheme))
	modal.OpenEmbedded(config.Default())
	modal.SyncNavSection(nav.SettingsStartup)
	modal.SetSize(34, 14)

	view := stripANSI(modal.renderOptionsBody())
	for _, row := range []string{"Re-scan on st…: On"} {
		if !strings.Contains(view, row) {
			t.Errorf("compact settings missing row %q:\n%s", row, view)
		}
	}
	if strings.Contains(view, "Use the default, dark, or light theme.") {
		t.Fatal("compact settings should omit the long description")
	}
}

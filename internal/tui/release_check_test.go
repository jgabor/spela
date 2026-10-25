package tui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jgabor/spela/internal/release"
)

func TestReleaseCheckSetsTheHeaderNotice(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`[{"name":"v0.7.0"}]`))
	}))
	t.Cleanup(server.Close)
	originalURL := release.TagsURL
	release.TagsURL = server.URL
	t.Cleanup(func() { release.TagsURL = originalURL })

	if message := checkForRelease("dev")(); message != nil {
		t.Fatalf("development build produced %#v", message)
	}
	message, ok := checkForRelease("v0.6.2")().(releaseNoticeMsg)
	if !ok || message.text != "Spela v0.7.0 is available (you have v0.6.2)" {
		t.Fatalf("release message = %#v", message)
	}
	styles := NewStyles(DefaultTheme)
	layout := LayoutModel{styles: styles, header: NewHeader(styles), messageBar: NewMessageBar(styles)}
	layout, _ = layout.handleAppMessages(message, nil)
	layout.header.SetWidth(120)
	if !strings.Contains(stripANSI(layout.header.View()), "Spela v0.7.0 is available") {
		t.Fatal("header does not show the release notice")
	}
}

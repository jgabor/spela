package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func tagServer(t *testing.T, body string, status int) *atomic.Int32 {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	originalURL, originalClient := TagsURL, httpClient
	TagsURL, httpClient = server.URL, server.Client()
	t.Cleanup(func() { TagsURL, httpClient = originalURL, originalClient })
	return &requests
}

const tags = `[{"name":"v0.7.0-rc1"},{"name":"v0.6.2"},{"name":"latest"},{"name":"v0.6.10"},{"name":"v0.5.1"}]`

func TestCheckReportsTheNewestReleaseTagOncePerDay(t *testing.T) {
	requests := tagServer(t, tags, http.StatusOK)
	now := time.Now()
	notice, err := Check(context.Background(), "0.6.2", now)
	if err != nil || notice == nil || notice.Latest != "v0.6.10" || requests.Load() != 1 {
		t.Fatalf("Check = %+v, %v after %d requests; want v0.6.10 after 1", notice, err, requests.Load())
	}
	if got := notice.Text(); got != "Spela v0.6.10 is available (you have v0.6.2)" {
		t.Fatalf("Text() = %q", got)
	}
	// Within a day the stored result answers without a lookup.
	if notice, err := Check(context.Background(), "v0.6.2", now.Add(23*time.Hour)); err != nil || notice == nil || requests.Load() != 1 {
		t.Fatalf("second Check = %+v, %v after %d requests", notice, err, requests.Load())
	}
	// After an upgrade the stored result no longer shows a notice.
	if notice, _ := Check(context.Background(), "v0.6.10", now.Add(time.Hour)); notice != nil || requests.Load() != 1 {
		t.Fatalf("Check after upgrade = %+v after %d requests", notice, requests.Load())
	}
	if _, err := Check(context.Background(), "v0.6.2", now.Add(25*time.Hour)); err != nil || requests.Load() != 2 {
		t.Fatalf("Check a day later made %d requests, %v", requests.Load(), err)
	}
}

func TestCheckSkipsDevelopmentBuilds(t *testing.T) {
	requests := tagServer(t, tags, http.StatusOK)
	for _, version := range []string{"dev", "", "v0.6.2-12-g73b0b95-dirty", "0.5.1.r6.g087d1c4", "73b0b95"} {
		if notice, err := Check(context.Background(), version, time.Now()); notice != nil || err != nil {
			t.Errorf("Check(%q) = %+v, %v", version, notice, err)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("development builds made %d requests", requests.Load())
	}
}

func TestCheckRecordsAFailedLookupAndKeepsTheLastRelease(t *testing.T) {
	requests := tagServer(t, tags, http.StatusOK)
	now := time.Now()
	if _, err := Check(context.Background(), "v0.6.2", now); err != nil {
		t.Fatal(err)
	}
	failing := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(failing.Close)
	TagsURL = failing.URL
	notice, err := Check(context.Background(), "v0.6.2", now.Add(25*time.Hour))
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || notice == nil || notice.Latest != "v0.6.10" {
		t.Fatalf("failed lookup = %+v, %v; want the stored release and the error", notice, err)
	}
	if _, err := Check(context.Background(), "v0.6.2", now.Add(26*time.Hour)); err != nil || requests.Load() != 2 {
		t.Fatalf("a failed attempt was not recorded: %d requests, %v", requests.Load(), err)
	}
}

func TestCheckRejectsUnusableTagLists(t *testing.T) {
	for _, test := range []struct{ name, body string }{
		{"no release tags", `[{"name":"nightly"}]`},
		{"not JSON", `<html>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			tagServer(t, test.body, http.StatusOK)
			if notice, err := Check(context.Background(), "v0.6.2", time.Now()); notice != nil || err == nil {
				t.Fatalf("Check = %+v, %v; want an error and no notice", notice, err)
			}
		})
	}
}

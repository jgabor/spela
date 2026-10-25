package dll

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jgabor/spela/internal/config"
)

// manifestServer serves a manifest whose repository names the request path
// and counts requests.
func manifestServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path == "/failure" {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = writer.Write([]byte(`{"version":"1","repository":"` + request.URL.Path + `","dlls":{}}`))
	}))
	t.Cleanup(server.Close)
	originalClient := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = originalClient })
	return server, &requests
}

func saveManifestSettings(t *testing.T, url string, autoRefresh bool, hours int) {
	t.Helper()
	configuration := config.Default()
	configuration.DLLManifestURL, configuration.AutoRefreshManifest, configuration.ManifestRefreshHours = url, autoRefresh, hours
	if err := configuration.Save(); err != nil {
		t.Fatal(err)
	}
}

func cacheManifest(t *testing.T, repository, source string, fetchedAt time.Time) {
	t.Helper()
	if err := saveCachedManifest(&cachedManifest{Manifest: Manifest{Repository: repository}, FetchedAt: fetchedAt, SourceURL: source}); err != nil {
		t.Fatal(err)
	}
}

func TestGetManifestFollowsRefreshSettings(t *testing.T) {
	isolateDLLCache(t)
	server, requests := manifestServer(t)
	primary := server.URL + "/primary"

	check := func(name, wantRepository string, wantRequests int32) {
		t.Helper()
		requests.Store(0)
		manifest, err := GetManifest(false, "")
		if err != nil || manifest.Repository != wantRepository || requests.Load() != wantRequests {
			got := "<nil>"
			if manifest != nil {
				got = manifest.Repository
			}
			t.Fatalf("%s: repository %q, requests %d, error %v; want %q and %d requests", name, got, requests.Load(), err, wantRepository, wantRequests)
		}
	}

	saveManifestSettings(t, primary, true, 6)
	check("no cache fetches", "/primary", 1)
	cached, err := loadCachedManifest()
	if err != nil || cached.SourceURL != primary || time.Since(cached.FetchedAt) > time.Minute {
		t.Fatalf("cache did not record source and fetch time: %+v, %v", cached, err)
	}
	check("fresh cache is used", "/primary", 0)

	cacheManifest(t, "cached", primary, time.Now().Add(-2*time.Hour))
	check("younger than the interval", "cached", 0)
	saveManifestSettings(t, primary, true, 1)
	check("older than the interval", "/primary", 1)

	cacheManifest(t, "cached", primary, time.Now().Add(-30*24*time.Hour))
	saveManifestSettings(t, primary, false, 1)
	check("auto refresh off keeps an old cache", "cached", 0)

	saveManifestSettings(t, server.URL+"/other", false, 24)
	check("a changed source is fetched even with auto refresh off", "/other", 1)

	cacheManifest(t, "cached", server.URL+"/other", time.Now().Add(-48*time.Hour))
	saveManifestSettings(t, server.URL+"/failure", true, 24)
	check("a failed fetch falls back to the cache", "cached", 1)

	saveManifestSettings(t, primary, true, 0)
	cacheManifest(t, "cached", primary, time.Now().Add(-23*time.Hour))
	check("a zero interval uses 24 hours", "cached", 0)
}

func TestGetManifestTreatsAnOldCacheAsTheDefaultSource(t *testing.T) {
	isolateDLLCache(t)
	_, requests := manifestServer(t)
	// A cache written before fetch time and source were recorded.
	path := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "spela", ManifestCacheFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":"1","updated_at":"2020-01-01T00:00:00Z","repository":"legacy","dlls":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	saveManifestSettings(t, "", false, 24)
	if manifest, err := GetManifest(false, ""); err != nil || manifest.Repository != "legacy" || requests.Load() != 0 {
		t.Fatalf("old cache with auto refresh off = %+v, %v, %d requests", manifest, err, requests.Load())
	}
	onPolicy := manifestPolicy{url: DefaultManifestURL, autoRefresh: true, maxAge: ManifestMaxAge}
	if cached, err := loadCachedManifest(); err != nil || !cachedManifestCurrent(cached, onPolicy) {
		t.Fatalf("old cache written just now must count as fresh by its file time: %+v, %v", cached, err)
	}
	old := time.Now().Add(-2 * ManifestMaxAge)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if cached, err := loadCachedManifest(); err != nil || cachedManifestCurrent(cached, onPolicy) {
		t.Fatalf("old cache with an old file time must be stale when auto refresh is on: %+v, %v", cached, err)
	}
}

func TestManifestPolicyIgnoresAnUnsafeConfiguredURL(t *testing.T) {
	isolateDLLCache(t)
	saveManifestSettings(t, "http://example.test/manifest.json", true, 12)
	if policy := currentManifestPolicy(""); policy.url != DefaultManifestURL || policy.maxAge != 12*time.Hour {
		t.Fatalf("policy = %+v, want the default URL and 12 hours", policy)
	}
	saveManifestSettings(t, "https://example.test/manifest.json", true, 12)
	if policy := currentManifestPolicy(""); policy.url != "https://example.test/manifest.json" {
		t.Fatalf("policy URL = %q", policy.url)
	}
	if policy := currentManifestPolicy("https://override.test/m.json"); !strings.HasPrefix(policy.url, "https://override.test") {
		t.Fatalf("explicit URL did not override: %q", policy.url)
	}
}

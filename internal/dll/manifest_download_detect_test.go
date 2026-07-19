package dll

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/jgabor/spela/internal/game"
)

func isolateDLLCache(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
}

func TestManifestFetchCacheAndLookupContract(t *testing.T) {
	isolateDLLCache(t)
	manifestJSON := `{"version":"1","updated_at":"2026-07-18T12:00:00Z","repository":"fixture","dlls":{"dlss":[{"version":"3.8.10","filename":"nvngx_dlss.dll"}],"empty":[]}}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/failure" {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = writer.Write([]byte(manifestJSON))
	}))
	t.Cleanup(server.Close)
	originalClient := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = originalClient })

	manifest, err := FetchManifest(server.URL)
	if err != nil || manifest.Repository != "fixture" || manifest.UpdatedAt.IsZero() {
		t.Fatalf("FetchManifest = %+v, %v", manifest, err)
	}
	if latest := manifest.GetLatestDLL("dlss"); latest == nil || latest.Version != "3.8.10" {
		t.Fatalf("latest DLL = %+v", latest)
	}
	if manifest.GetLatestDLL("missing") != nil || manifest.GetLatestDLL("empty") != nil {
		t.Fatal("missing/empty manifest entries returned latest DLL")
	}
	if version := manifest.GetDLLVersion("dlss", "3.8.10"); version == nil || version.Filename != "nvngx_dlss.dll" {
		t.Fatalf("manifest version = %+v", version)
	}
	if manifest.GetDLLVersion("dlss", "0") != nil || manifest.GetDLLVersion("missing", "0") != nil {
		t.Fatal("unknown version unexpectedly resolved")
	}
	if names := manifest.ListDLLNames(); !reflect.DeepEqual(names, []string{"dlss", "empty"}) {
		t.Fatalf("manifest names = %v", names)
	}
	if err := SaveManifest(manifest); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadManifest()
	if err != nil || loaded.Repository != "fixture" {
		t.Fatalf("LoadManifest = %+v, %v", loaded, err)
	}
	loaded.UpdatedAt = time.Now()
	if err := SaveManifest(loaded); err != nil {
		t.Fatal(err)
	}
	if cached, err := GetManifest(false, server.URL+"/failure"); err != nil || cached.Repository != "fixture" {
		t.Fatalf("fresh cached manifest = %+v, %v", cached, err)
	}
	loaded.UpdatedAt = time.Now().Add(-2 * ManifestMaxAge)
	if err := SaveManifest(loaded); err != nil {
		t.Fatal(err)
	}
	if cached, err := GetManifest(false, server.URL+"/failure"); err != nil || cached.Repository != "fixture" {
		t.Fatalf("stale offline cached manifest = %+v, %v", cached, err)
	}
	if _, err := FetchManifest(server.URL + "/failure"); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("manifest HTTP error = %v", err)
	}
	cachePath := filepath.Join(os.Getenv("XDG_CACHE_HOME"), "spela", ManifestCacheFile)
	if err := os.WriteFile(cachePath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(); err == nil {
		t.Fatal("invalid cached manifest did not fail")
	}
}

func TestDownloadCacheProgressChecksumAndDirectoryContract(t *testing.T) {
	isolateDLLCache(t)
	payload := []byte("fixture DLL payload")
	digest := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/missing" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = writer.Write(payload)
	}))
	t.Cleanup(server.Close)
	originalClient := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = originalClient })

	entry := &DLL{Version: "3.8.10", URL: server.URL, SHA256: hex.EncodeToString(digest[:])}
	var downloaded, total int64
	path, err := acquireDLL(entry, "dlss", true, func(current, expected int64) {
		downloaded, total = current, expected
	})
	if err != nil || downloaded != int64(len(payload)) || total != int64(len(payload)) {
		t.Fatalf("download = path %q, progress %d/%d, error %v", path, downloaded, total, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string(payload) {
		t.Fatalf("downloaded payload = %q, %v", data, err)
	}
	if cached, err := acquireDLL(entry, "dlss", false, nil); err != nil || cached != path {
		t.Fatalf("valid cache = %q, %v", cached, err)
	}
	versions, err := ListCachedVersions("dlss")
	if err != nil || !reflect.DeepEqual(versions, []string{"3.8.10"}) {
		t.Fatalf("cached versions = %v, %v", versions, err)
	}
	if versions, err := ListCachedVersions("missing"); err != nil || versions != nil {
		t.Fatalf("missing cached versions = %v, %v", versions, err)
	}
	bad := &DLL{Version: "bad", URL: server.URL, SHA256: strings.Repeat("0", 64)}
	if _, err := acquireDLL(bad, "dlss", true, nil); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("checksum error = %v", err)
	}
	missing := &DLL{Version: "missing", URL: server.URL + "/missing"}
	if _, err := acquireDLL(missing, "dlss", true, nil); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("download HTTP error = %v", err)
	}
}

func TestAcquireDLLTimesOutWhileReadingBody(t *testing.T) {
	isolateDLLCache(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", "1024")
		writer.WriteHeader(http.StatusOK)
		writer.(http.Flusher).Flush()
		<-request.Context().Done()
	}))
	defer server.Close()
	originalClient := httpClient
	client := server.Client()
	client.Timeout = 50 * time.Millisecond
	httpClient = client
	t.Cleanup(func() { httpClient = originalClient })

	started := time.Now()
	_, err := acquireDLL(&DLL{Version: "1", URL: server.URL, SHA256: strings.Repeat("0", 64)}, "dlss", true, nil)
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("stalled body error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("stalled body timeout took %v", elapsed)
	}
}

func TestDLLDetectionCatalogueAndVersionResourceContract(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"NVNGX_DLSS.DLL", "libxess.dll", "ordinary.dll"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("not a PE"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	detected, err := ScanDirectory(directory)
	if err != nil || len(detected) != 2 || detected[0].Type != game.DLLTypeDLSS || detected[1].Type != game.DLLTypeXeSS {
		t.Fatalf("detected DLLs = %+v, %v", detected, err)
	}
	if missing, err := ScanDirectory(filepath.Join(directory, "missing")); err == nil || len(missing) != 0 {
		t.Fatalf("missing scan directory should fail = %+v, %v", missing, err)
	}
	types := KnownDLLTypes()
	if len(types) != 5 || types[0].ManifestKey != "dlss" || types[4].ManifestKey != "fsr" {
		t.Fatalf("known DLL catalogue = %+v", types)
	}
	types[0].ManifestKey = "changed"
	if KnownDLLTypes()[0].ManifestKey != "dlss" {
		t.Fatal("known DLL catalogue returned shared mutable storage")
	}

	resource := make([]byte, 80)
	name := utf16.Encode([]rune("VS_VERSION_INFO"))
	for index, value := range name {
		binary.LittleEndian.PutUint16(resource[index*2:], value)
	}
	signatureOffset := 40
	copy(resource[signatureOffset:], []byte{0xBD, 0x04, 0xEF, 0xFE})
	binary.LittleEndian.PutUint32(resource[signatureOffset+8:], uint32(3)<<16|8)
	binary.LittleEndian.PutUint32(resource[signatureOffset+12:], uint32(10)<<16)
	if version := extractVersionFromResource(resource); version != "3.8.10" {
		t.Fatalf("resource version = %q", version)
	}
	if extractVersionFromResource(nil) != "" || formatVersion(1, 2, 0, 0) != "1.2" || formatVersion(1, 2, 3, 4) != "1.2.3.4" {
		t.Fatal("version resource edge formatting changed")
	}
}

func TestScanDirectoryPropagatesNestedTraversalError(t *testing.T) {
	want := errors.New("injected nested traversal failure")
	detected, err := scanDirectory("/fixture", func(root string, visit fs.WalkDirFunc) error {
		return visit(filepath.Join(root, "nested"), nil, want)
	})
	if !errors.Is(err, want) || len(detected) != 0 {
		t.Fatalf("scanDirectory() = %+v, %v", detected, err)
	}
}

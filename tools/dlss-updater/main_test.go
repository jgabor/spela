package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestFetchLatestVersionUsesTechPowerUpPageAndRedirectContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != userAgent {
			t.Errorf("User-Agent = %q", request.Header.Get("User-Agent"))
		}
		switch request.Method {
		case http.MethodGet:
			_, _ = writer.Write([]byte(`<title>Fixture DLL 3.8.10 Download</title><input type="hidden" name="id" value="12345">`))
		case http.MethodPost:
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.Form.Get("id") != "12345" || request.Form.Get("server_id") != "27" {
				t.Errorf("download form = %v", request.Form)
			}
			writer.Header().Set("Location", "https://download.example.test/nvngx.dll")
			writer.WriteHeader(http.StatusFound)
		}
	}))
	t.Cleanup(server.Close)

	originalClient := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = originalClient })
	source := &DLLSource{
		Type: "dlss", TechPowerUp: server.URL, Filename: "nvngx_dlss.dll",
		TitlePattern: regexp.MustCompile(`<title>Fixture DLL (\d+\.\d+\.\d+) Download`),
	}
	latest, err := fetchLatestVersion(source)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Type != "dlss" || latest.Version != "3.8.10" || latest.Filename != "nvngx_dlss.dll" || latest.DownloadURL != "https://download.example.test/nvngx.dll" || latest.Source != "techpowerup.com" {
		t.Fatalf("latest version = %+v", latest)
	}
}

func TestTechPowerUpFailuresRemainActionable(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })
	source := &DLLSource{Type: "fixture", Filename: "fixture.dll", TitlePattern: regexp.MustCompile(`<title>Fixture (\d+\.\d+\.\d+)`)}
	for _, test := range []struct {
		name     string
		status   int
		body     string
		location string
		want     string
	}{
		{"HTTP status", http.StatusBadGateway, "", "", "HTTP 502"},
		{"missing version", http.StatusOK, `<input type="hidden" name="id" value="1">`, "", "could not find version"},
		{"missing file ID", http.StatusOK, `<title>Fixture 1.2.3`, "", "could not find file ID"},
		{"missing redirect", http.StatusOK, `<title>Fixture 1.2.3<input type="hidden" name="id" value="1">`, "", "expected redirect"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method == http.MethodPost {
					if test.location != "" {
						writer.Header().Set("Location", test.location)
					}
					writer.WriteHeader(http.StatusOK)
					return
				}
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			httpClient = server.Client()
			source.TechPowerUp = server.URL
			if _, err := fetchFromTechPowerUp(source); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestManifestVersionAndSourceContracts(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"dlls":{"dlss":[{"version":"v3.8.10"}],"empty":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := getCurrentVersion(manifest, "dlss"); err != nil || got != "3.8.10" {
		t.Fatalf("current version = %q, %v", got, err)
	}
	if got, err := getCurrentVersion(manifest, "empty"); err != nil || got != "" {
		t.Fatalf("empty version = %q, %v", got, err)
	}
	if _, err := getCurrentVersion(filepath.Join(t.TempDir(), "missing"), "dlss"); err == nil {
		t.Fatal("missing manifest did not fail")
	}
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := getCurrentVersion(invalid, "dlss"); err == nil {
		t.Fatal("invalid manifest did not fail")
	}
	for _, dllType := range []string{"dlss", "dlssg", "dlssd"} {
		if source := findSource(dllType); source == nil || source.Type != dllType {
			t.Errorf("findSource(%q) = %+v", dllType, source)
		}
	}
	if findSource("unknown") != nil {
		t.Fatal("unknown source unexpectedly resolved")
	}
}

func TestDownloadRedirectValidation(t *testing.T) {
	for _, test := range []struct {
		name     string
		status   int
		location string
		want     string
	}{
		{"success", http.StatusFound, "https://download.example.test/file", "https://download.example.test/file"},
		{"wrong status", http.StatusOK, "", "expected redirect"},
		{"missing location", http.StatusFound, "", "no redirect location"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				if test.location != "" {
					writer.Header().Set("Location", test.location)
				}
				writer.WriteHeader(test.status)
			}))
			defer server.Close()
			got, err := getTechPowerUpDownloadURL(server.URL, "42")
			if test.status == http.StatusFound && test.location != "" {
				if err != nil || got != test.want {
					t.Fatalf("download URL = %q, %v", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunCommandOutputExitAndFailureContracts(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"dlls":{"dlss":[{"version":"3.8.10"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	latest := func(version string) func(*DLLSource) (*LatestVersion, error) {
		return func(source *DLLSource) (*LatestVersion, error) {
			return &LatestVersion{Type: source.Type, Version: version, DownloadURL: "https://download", Filename: source.Filename, Source: "fixture"}, nil
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run("dlss-updater", []string{"--manifest", manifest, "--type", "dlss"}, &stdout, &stderr, latest("3.9.0")); code != 0 || !strings.Contains(stdout.String(), "New DLSS version") || !strings.Contains(stdout.String(), "Current version: 3.8.10") || stderr.Len() != 0 {
		t.Fatalf("new version run = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run("dlss-updater", []string{"--manifest", manifest, "--json"}, &stdout, &stderr, latest("3.8.10")); code != 2 || !strings.Contains(stdout.String(), `"is_new":false`) {
		t.Fatalf("current JSON run = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run("dlss-updater", []string{"--type", "unknown"}, &stdout, &stderr, latest("1")); code != 1 || !strings.Contains(stderr.String(), "Unknown DLL type") {
		t.Fatalf("unknown run = code %d, stderr %q", code, stderr.String())
	}
	stderr.Reset()
	fetchError := errors.New("offline")
	if code := run("dlss-updater", nil, &stdout, &stderr, func(*DLLSource) (*LatestVersion, error) { return nil, fetchError }); code != 1 || !strings.Contains(stderr.String(), "offline") {
		t.Fatalf("fetch failure run = code %d, stderr %q", code, stderr.String())
	}
}

func TestRunPreservesFlagHelpAndParseExitSemantics(t *testing.T) {
	const usage = "Usage of dlss-updater:\n" +
		"  -json\n" +
		"    \tOutput as JSON\n" +
		"  -manifest string\n" +
		"    \tPath to manifest.json (default \"data/manifest.json\")\n" +
		"  -type string\n" +
		"    \tDLL type to check (dlss, dlssg, dlssd) (default \"dlss\")\n"
	fetch := func(*DLLSource) (*LatestVersion, error) {
		t.Fatal("flag-only invocation unexpectedly fetched a DLL")
		return nil, nil
	}
	for _, helpFlag := range []string{"-h", "--help"} {
		var stdout, stderr bytes.Buffer
		if code := run("dlss-updater", []string{helpFlag}, &stdout, &stderr, fetch); code != 0 || stdout.String() != "" || stderr.String() != usage {
			t.Fatalf("%s = code %d, stdout %q, stderr %q", helpFlag, code, stdout.String(), stderr.String())
		}
	}
	var namedStdout, namedStderr bytes.Buffer
	if code := run("/tmp/custom-updater", []string{"-h"}, &namedStdout, &namedStderr, fetch); code != 0 || !strings.HasPrefix(namedStderr.String(), "Usage of /tmp/custom-updater:\n") {
		t.Fatalf("custom program name = code %d, stdout %q, stderr %q", code, namedStdout.String(), namedStderr.String())
	}
	var stdout, stderr bytes.Buffer
	wantError := "flag provided but not defined: -invalid\n" + usage
	if code := run("dlss-updater", []string{"--invalid"}, &stdout, &stderr, fetch); code != 2 || stdout.String() != "" || stderr.String() != wantError {
		t.Fatalf("invalid flag = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	nvidiaReleaseAPI = "https://api.github.com/repos/NVIDIA/DLSS/releases/latest"
)

type Manifest struct {
	Version   string           `json:"version"`
	UpdatedAt string           `json:"updated_at"`
	DLLs      map[string][]DLL `json:"dlls"`
}

type DLL struct {
	Version string `json:"version"`
}

type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type LatestVersion struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	IsNew       bool   `json:"is_new"`
	Source      string `json:"source"`
}

func main() {
	manifestPath := flag.String("manifest", "data/manifest.json", "Path to manifest.json")
	outputJSON := flag.Bool("json", false, "Output as JSON")
	flag.Parse()

	latest, err := fetchLatestVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch latest version: %v\n", err)
		os.Exit(1)
	}

	current, err := getCurrentVersion(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read manifest: %v\n", err)
	}

	latest.IsNew = current == "" || latest.Version != current

	if *outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(latest)
	} else {
		if latest.IsNew {
			fmt.Printf("New DLSS version available: %s\n", latest.Version)
			fmt.Printf("Download URL: %s\n", latest.DownloadURL)
			fmt.Printf("Source: %s\n", latest.Source)
			if current != "" {
				fmt.Printf("Current version: %s\n", current)
			}
		} else {
			fmt.Printf("DLSS is up to date: %s\n", latest.Version)
		}
	}

	if latest.IsNew {
		os.Exit(0)
	} else {
		os.Exit(2)
	}
}

func fetchLatestVersion() (*LatestVersion, error) {
	release, err := fetchGitHubRelease(nvidiaReleaseAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NVIDIA release: %w", err)
	}

	version := strings.TrimPrefix(release.TagName, "v")

	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, "windows") && strings.HasSuffix(asset.Name, ".zip") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no Windows demo zip found in release")
	}

	return &LatestVersion{
		Version:     version,
		DownloadURL: downloadURL,
		Source:      "github.com/NVIDIA/DLSS",
	}, nil
}

func fetchGitHubRelease(apiURL string) (*GitHubRelease, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("failed to parse release JSON: %w", err)
	}

	return &release, nil
}

func getCurrentVersion(manifestPath string) (string, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}

	dlls, ok := manifest.DLLs["dlss"]
	if !ok || len(dlls) == 0 {
		return "", nil
	}

	return strings.TrimPrefix(dlls[0].Version, "v"), nil
}

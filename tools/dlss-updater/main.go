package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

const (
	techPowerUpURL   = "https://www.techpowerup.com/download/nvidia-dlss-dll/"
	nvidiaReleaseAPI = "https://api.github.com/repos/NVIDIA/DLSS/releases/latest"
	userAgent        = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
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
	// Try TechPowerUp first
	latest, err := fetchFromTechPowerUp()
	if err == nil {
		return latest, nil
	}
	fmt.Fprintf(os.Stderr, "TechPowerUp failed: %v, falling back to GitHub\n", err)

	// Fallback to official NVIDIA GitHub
	return fetchFromNvidiaGitHub()
}

func fetchFromTechPowerUp() (*LatestVersion, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", techPowerUpURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)

	// Extract version from title
	titleRegex := regexp.MustCompile(`<title>NVIDIA DLSS DLL (\d+\.\d+\.\d+) Download`)
	matches := titleRegex.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find version in page title")
	}
	version := matches[1]

	// Extract file ID
	idRegex := regexp.MustCompile(`<input type="hidden" name="id" value="(\d+)"`)
	idMatches := idRegex.FindStringSubmatch(html)
	if len(idMatches) < 2 {
		return nil, fmt.Errorf("could not find file ID")
	}
	fileID := idMatches[1]

	// Get actual download URL
	downloadURL, err := getTechPowerUpDownloadURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("could not get download URL: %w", err)
	}

	return &LatestVersion{
		Version:     version,
		DownloadURL: downloadURL,
		Source:      "techpowerup.com",
	}, nil
}

func getTechPowerUpDownloadURL(fileID string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	data := url.Values{}
	data.Set("id", fileID)
	data.Set("server_id", "27") // TechPowerUp NL server

	req, err := http.NewRequest("POST", techPowerUpURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("expected redirect, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no redirect location")
	}

	return location, nil
}

func fetchFromNvidiaGitHub() (*LatestVersion, error) {
	req, err := http.NewRequest("GET", nvidiaReleaseAPI, nil)
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

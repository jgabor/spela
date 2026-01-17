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
	userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
)

type DLLSource struct {
	Type         string
	TechPowerUp  string
	Filename     string
	TitlePattern string
}

var dllSources = []DLLSource{
	{
		Type:         "dlss",
		TechPowerUp:  "https://www.techpowerup.com/download/nvidia-dlss-dll/",
		Filename:     "nvngx_dlss.dll",
		TitlePattern: `<title>NVIDIA DLSS DLL (\d+\.\d+\.\d+) Download`,
	},
	{
		Type:         "dlssg",
		TechPowerUp:  "https://www.techpowerup.com/download/nvidia-dlss-3-frame-generation-dll/",
		Filename:     "nvngx_dlssg.dll",
		TitlePattern: `<title>NVIDIA DLSS Frame Generation DLL (\d+\.\d+\.\d+) Download`,
	},
	{
		Type:         "dlssd",
		TechPowerUp:  "https://www.techpowerup.com/download/nvidia-dlss-3-ray-reconstruction-dll/",
		Filename:     "nvngx_dlssd.dll",
		TitlePattern: `<title>NVIDIA DLSS Ray Reconstruction DLL (\d+\.\d+\.\d+) Download`,
	},
}

type Manifest struct {
	Version   string           `json:"version"`
	UpdatedAt string           `json:"updated_at"`
	DLLs      map[string][]DLL `json:"dlls"`
}

type DLL struct {
	Version string `json:"version"`
}

type LatestVersion struct {
	Type        string `json:"type"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Filename    string `json:"filename"`
	IsNew       bool   `json:"is_new"`
	Source      string `json:"source"`
}

func main() {
	manifestPath := flag.String("manifest", "data/manifest.json", "Path to manifest.json")
	dllType := flag.String("type", "dlss", "DLL type to check (dlss, dlssg, dlssd)")
	outputJSON := flag.Bool("json", false, "Output as JSON")
	flag.Parse()

	source := findSource(*dllType)
	if source == nil {
		fmt.Fprintf(os.Stderr, "Unknown DLL type: %s\n", *dllType)
		os.Exit(1)
	}

	latest, err := fetchLatestVersion(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch latest version: %v\n", err)
		os.Exit(1)
	}

	current, err := getCurrentVersion(*manifestPath, *dllType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read manifest: %v\n", err)
	}

	latest.IsNew = current == "" || latest.Version != current

	if *outputJSON {
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(latest)
	} else {
		if latest.IsNew {
			fmt.Printf("New %s version available: %s\n", strings.ToUpper(source.Type), latest.Version)
			fmt.Printf("Download URL: %s\n", latest.DownloadURL)
			fmt.Printf("Filename: %s\n", latest.Filename)
			fmt.Printf("Source: %s\n", latest.Source)
			if current != "" {
				fmt.Printf("Current version: %s\n", current)
			}
		} else {
			fmt.Printf("%s is up to date: %s\n", strings.ToUpper(source.Type), latest.Version)
		}
	}

	if latest.IsNew {
		os.Exit(0)
	} else {
		os.Exit(2)
	}
}

func findSource(dllType string) *DLLSource {
	for i := range dllSources {
		if dllSources[i].Type == dllType {
			return &dllSources[i]
		}
	}
	return nil
}

func fetchLatestVersion(source *DLLSource) (*LatestVersion, error) {
	return fetchFromTechPowerUp(source)
}

func fetchFromTechPowerUp(source *DLLSource) (*LatestVersion, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", source.TechPowerUp, nil)
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
	titleRegex := regexp.MustCompile(source.TitlePattern)
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
	downloadURL, err := getTechPowerUpDownloadURL(source.TechPowerUp, fileID)
	if err != nil {
		return nil, fmt.Errorf("could not get download URL: %w", err)
	}

	return &LatestVersion{
		Type:        source.Type,
		Version:     version,
		DownloadURL: downloadURL,
		Filename:    source.Filename,
		Source:      "techpowerup.com",
	}, nil
}

func getTechPowerUpDownloadURL(baseURL, fileID string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	data := url.Values{}
	data.Set("id", fileID)
	data.Set("server_id", "27") // TechPowerUp NL server

	req, err := http.NewRequest("POST", baseURL, strings.NewReader(data.Encode()))
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

func getCurrentVersion(manifestPath, dllType string) (string, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}

	dlls, ok := manifest.DLLs[dllType]
	if !ok || len(dlls) == 0 {
		return "", nil
	}

	return strings.TrimPrefix(dlls[0].Version, "v"), nil
}

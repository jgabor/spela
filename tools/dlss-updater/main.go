package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
)

var (
	httpClient = &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}

	idRegex = regexp.MustCompile(`<input type="hidden" name="id" value="(\d+)"`)
)

type DLLSource struct {
	Type         string
	TechPowerUp  string
	Filename     string
	TitlePattern *regexp.Regexp
}

var dllSources = []DLLSource{
	{
		Type:         "dlss",
		TechPowerUp:  "https://www.techpowerup.com/download/nvidia-dlss-dll/",
		Filename:     "nvngx_dlss.dll",
		TitlePattern: regexp.MustCompile(`<title>NVIDIA DLSS DLL (\d+\.\d+\.\d+) Download`),
	},
	{
		Type:         "dlssg",
		TechPowerUp:  "https://www.techpowerup.com/download/nvidia-dlss-3-frame-generation-dll/",
		Filename:     "nvngx_dlssg.dll",
		TitlePattern: regexp.MustCompile(`<title>NVIDIA DLSS Frame Generation DLL (\d+\.\d+\.\d+) Download`),
	},
	{
		Type:         "dlssd",
		TechPowerUp:  "https://www.techpowerup.com/download/nvidia-dlss-3-ray-reconstruction-dll/",
		Filename:     "nvngx_dlssd.dll",
		TitlePattern: regexp.MustCompile(`<title>NVIDIA DLSS Ray Reconstruction DLL (\d+\.\d+\.\d+) Download`),
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
	os.Exit(run(os.Args[0], os.Args[1:], os.Stdout, os.Stderr, fetchLatestVersion))
}

func run(programName string, args []string, stdout, stderr io.Writer, fetch func(*DLLSource) (*LatestVersion, error)) int {
	flags := flag.NewFlagSet(programName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "data/manifest.json", "Path to manifest.json")
	dllType := flags.String("type", "dlss", "DLL type to check (dlss, dlssg, dlssd)")
	outputJSON := flags.Bool("json", false, "Output as JSON")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	source := findSource(*dllType)
	if source == nil {
		_, _ = fmt.Fprintf(stderr, "Unknown DLL type: %s\n", *dllType)
		return 1
	}

	latest, err := fetch(source)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Failed to fetch latest version: %v\n", err)
		return 1
	}

	current, err := getCurrentVersion(*manifestPath, *dllType)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Warning: could not read manifest: %v\n", err)
	}

	latest.IsNew = current == "" || latest.Version != current

	if *outputJSON {
		enc := json.NewEncoder(stdout)
		if err := enc.Encode(latest); err != nil {
			_, _ = fmt.Fprintf(stderr, "Failed to encode JSON: %v\n", err)
			return 1
		}
	} else {
		if latest.IsNew {
			_, _ = fmt.Fprintf(stdout, "New %s version available: %s\n", strings.ToUpper(source.Type), latest.Version)
			_, _ = fmt.Fprintf(stdout, "Download URL: %s\n", latest.DownloadURL)
			_, _ = fmt.Fprintf(stdout, "Filename: %s\n", latest.Filename)
			_, _ = fmt.Fprintf(stdout, "Source: %s\n", latest.Source)
			if current != "" {
				_, _ = fmt.Fprintf(stdout, "Current version: %s\n", current)
			}
		} else {
			_, _ = fmt.Fprintf(stdout, "%s is up to date: %s\n", strings.ToUpper(source.Type), latest.Version)
		}
	}

	if latest.IsNew {
		return 0
	}
	return 2
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
	req, err := http.NewRequest("GET", source.TechPowerUp, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
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
	matches := source.TitlePattern.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find version in page title")
	}
	version := matches[1]

	// Extract file ID
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
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
		},
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

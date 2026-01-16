package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

const (
	techPowerUpURL = "https://www.techpowerup.com/download/nvidia-dlss-dll/"
)

type Manifest struct {
	Version   string           `json:"version"`
	UpdatedAt string           `json:"updated_at"`
	DLLs      map[string][]DLL `json:"dlls"`
}

type DLL struct {
	Version string `json:"version"`
}

type LatestVersion struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	IsNew       bool   `json:"is_new"`
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
	resp, err := http.Get(techPowerUpURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)

	versionRegex := regexp.MustCompile(`/download/nvidia-dlss-dll/file\?id=(\d+)[^"]*"[^>]*>[\s\S]*?Version[:\s]+(\d+\.\d+\.\d+)`)
	matches := versionRegex.FindStringSubmatch(html)

	if len(matches) < 3 {
		altRegex := regexp.MustCompile(`<a[^>]+href="([^"]+dlss[^"]+)"[^>]*>[\s\S]{0,200}?(\d+\.\d+\.\d+)`)
		matches = altRegex.FindStringSubmatch(html)
		if len(matches) < 3 {
			altRegex2 := regexp.MustCompile(`(\d+\.\d+\.\d+)[\s\S]{0,50}?<a[^>]+href="/download/nvidia-dlss-dll/\?id=(\d+)"`)
			matches = altRegex2.FindStringSubmatch(html)
			if len(matches) >= 3 {
				return &LatestVersion{
					Version:     matches[1],
					DownloadURL: fmt.Sprintf("https://www.techpowerup.com/download/nvidia-dlss-dll/?id=%s", matches[2]),
				}, nil
			}
			return nil, fmt.Errorf("could not find version info in page")
		}
	}

	fileID := matches[1]
	version := matches[2]

	return &LatestVersion{
		Version:     version,
		DownloadURL: fmt.Sprintf("https://www.techpowerup.com/download/nvidia-dlss-dll/?id=%s", fileID),
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

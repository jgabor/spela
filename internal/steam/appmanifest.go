package steam

import (
	"os"
	"path/filepath"
	"strconv"
)

type AppManifest struct {
	AppID          uint64
	Name           string
	InstallDir     string
	StateFlags     int
	LastUpdated    int64
	SizeOnDisk     int64
	LibraryPath    string
	FullInstallDir string
}

func ParseAppManifest(path string) (*AppManifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	node, err := ParseVDF(f)
	if err != nil {
		return nil, err
	}

	appState := node.GetNode("AppState")
	if appState == nil {
		return nil, nil
	}

	appID, _ := strconv.ParseUint(appState.GetString("appid"), 10, 64)
	stateFlags, _ := strconv.Atoi(appState.GetString("StateFlags"))
	lastUpdated, _ := strconv.ParseInt(appState.GetString("LastUpdated"), 10, 64)
	sizeOnDisk, _ := strconv.ParseInt(appState.GetString("SizeOnDisk"), 10, 64)

	libraryPath := filepath.Dir(filepath.Dir(path))
	installDir := appState.GetString("installdir")

	return &AppManifest{
		AppID:          appID,
		Name:           appState.GetString("name"),
		InstallDir:     installDir,
		StateFlags:     stateFlags,
		LastUpdated:    lastUpdated,
		SizeOnDisk:     sizeOnDisk,
		LibraryPath:    libraryPath,
		FullInstallDir: filepath.Join(libraryPath, "steamapps", "common", installDir),
	}, nil
}

func (m *AppManifest) IsFullyInstalled() bool {
	return m.StateFlags == 4
}

func (m *AppManifest) CompatDataPath() string {
	return filepath.Join(m.LibraryPath, "steamapps", "compatdata", strconv.FormatUint(m.AppID, 10))
}

func (m *AppManifest) PrefixPath() string {
	return filepath.Join(m.CompatDataPath(), "pfx", "drive_c")
}

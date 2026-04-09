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
	defer func() { _ = f.Close() }()

	node, err := ParseVDF(f)
	if err != nil {
		return nil, err
	}

	appState := node.GetNode("AppState")
	if appState == nil {
		return nil, nil
	}

	appID, err := strconv.ParseUint(appState.GetString("appid"), 10, 64)
	if err != nil || appID == 0 {
		return nil, nil
	}
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

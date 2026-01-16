package steam

import (
	"os"
	"path/filepath"
	"time"

	"github.com/jgabor/spela/internal/dll"
	"github.com/jgabor/spela/internal/game"
)

type Library struct {
	Path  string
	Label string
	Apps  map[string]string
}

func FindSteamPath() string {
	home := os.Getenv("HOME")
	paths := []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".steam", "steam"),
	}

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return ""
}

func GetLibraries(steamPath string) ([]Library, error) {
	vdfPath := filepath.Join(steamPath, "steamapps", "libraryfolders.vdf")
	f, err := os.Open(vdfPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	node, err := ParseVDF(f)
	if err != nil {
		return nil, err
	}

	libraryFolders := node.GetNode("libraryfolders")
	if libraryFolders == nil {
		return nil, nil
	}

	var libraries []Library
	for key, value := range libraryFolders {
		if _, err := filepath.Abs(key); err != nil {
			continue
		}

		libNode, ok := value.(VDFNode)
		if !ok {
			continue
		}

		lib := Library{
			Path:  libNode.GetString("path"),
			Label: libNode.GetString("label"),
			Apps:  make(map[string]string),
		}

		if appsNode := libNode.GetNode("apps"); appsNode != nil {
			for appID, size := range appsNode {
				if s, ok := size.(string); ok {
					lib.Apps[appID] = s
				}
			}
		}

		if lib.Path != "" {
			libraries = append(libraries, lib)
		}
	}

	return libraries, nil
}

func ScanLibrary(lib Library) ([]*game.Game, error) {
	steamapps := filepath.Join(lib.Path, "steamapps")
	compatdata := filepath.Join(steamapps, "compatdata")

	manifests, err := filepath.Glob(filepath.Join(steamapps, "appmanifest_*.acf"))
	if err != nil {
		return nil, err
	}

	var games []*game.Game
	for _, manifestPath := range manifests {
		manifest, err := ParseAppManifest(manifestPath)
		if err != nil || manifest == nil {
			continue
		}

		if !manifest.IsFullyInstalled() {
			continue
		}

		g := &game.Game{
			AppID:       manifest.AppID,
			Name:        manifest.Name,
			InstallDir:  manifest.FullInstallDir,
			LibraryPath: lib.Path,
			ScannedAt:   time.Now(),
		}

		prefix := ScanProtonPrefix(compatdata, manifest.AppID)
		if prefix.IsValid {
			g.PrefixPath = prefix.Path
		}

		dlls, _ := dll.ScanDirectory(manifest.FullInstallDir)
		for _, d := range dlls {
			g.DLLs = append(g.DLLs, game.DetectedDLL{
				Path:    d.Path,
				Name:    d.Name,
				Type:    d.Type,
				Version: d.Version,
			})
		}

		games = append(games, g)
	}

	return games, nil
}

func ScanAllLibraries() (*game.Database, error) {
	steamPath := FindSteamPath()
	if steamPath == "" {
		return nil, nil
	}

	libraries, err := GetLibraries(steamPath)
	if err != nil {
		return nil, err
	}

	db := &game.Database{Games: make(map[uint64]*game.Game)}

	for _, lib := range libraries {
		games, err := ScanLibrary(lib)
		if err != nil {
			continue
		}

		for _, g := range games {
			db.AddGame(g)
		}
	}

	return db, nil
}

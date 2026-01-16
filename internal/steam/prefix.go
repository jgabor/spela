package steam

import (
	"os"
	"path/filepath"
	"strconv"
)

type ProtonPrefix struct {
	AppID   uint64
	Path    string
	DriveC  string
	IsValid bool
}

func ScanProtonPrefix(compatDataPath string, appID uint64) *ProtonPrefix {
	prefixPath := filepath.Join(compatDataPath, strconv.FormatUint(appID, 10))
	driveCPath := filepath.Join(prefixPath, "pfx", "drive_c")

	info, err := os.Stat(driveCPath)
	isValid := err == nil && info.IsDir()

	return &ProtonPrefix{
		AppID:   appID,
		Path:    prefixPath,
		DriveC:  driveCPath,
		IsValid: isValid,
	}
}

func (p *ProtonPrefix) ProgramFiles() string {
	return filepath.Join(p.DriveC, "Program Files")
}

func (p *ProtonPrefix) ProgramFilesX86() string {
	return filepath.Join(p.DriveC, "Program Files (x86)")
}

func (p *ProtonPrefix) Users() string {
	return filepath.Join(p.DriveC, "users")
}

func (p *ProtonPrefix) Windows() string {
	return filepath.Join(p.DriveC, "windows")
}

func FindDLLsInPrefix(prefix *ProtonPrefix, dllNames []string) map[string][]string {
	if !prefix.IsValid {
		return nil
	}

	results := make(map[string][]string)

	filepath.WalkDir(prefix.DriveC, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		name := d.Name()
		for _, dllName := range dllNames {
			if name == dllName {
				results[dllName] = append(results[dllName], path)
			}
		}
		return nil
	})

	return results
}

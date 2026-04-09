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

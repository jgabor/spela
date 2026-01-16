package dll

import (
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jgabor/spela/internal/game"
)

var knownDLLs = map[string]game.DLLType{
	"nvngx_dlss.dll":          game.DLLTypeDLSS,
	"nvngx_dlssg.dll":         game.DLLTypeDLSSG,
	"nvngx_dlssd.dll":         game.DLLTypeDLSSD,
	"libxess.dll":             game.DLLTypeXeSS,
	"amd_fidelityfx_vk.dll":   game.DLLTypeFSR,
	"amd_fidelityfx_dx12.dll": game.DLLTypeFSR,
}

func ScanDirectory(dir string) ([]game.DetectedDLL, error) {
	var results []game.DetectedDLL

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		name := strings.ToLower(d.Name())
		if dllType, ok := knownDLLs[name]; ok {
			version, _ := GetDLLVersion(path)
			results = append(results, game.DetectedDLL{
				Path:    path,
				Name:    d.Name(),
				Type:    dllType,
				Version: version,
			})
		}
		return nil
	})

	return results, err
}

func GetDLLVersion(path string) (string, error) {
	f, err := pe.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	for _, section := range f.Sections {
		if section.Name != ".rsrc" {
			continue
		}

		data, err := section.Data()
		if err != nil {
			return "", err
		}

		return extractVersionFromResource(data), nil
	}

	return "", nil
}

func extractVersionFromResource(data []byte) string {
	vsVersionInfo := []byte("VS_VERSION_INFO")
	idx := findBytes(data, vsVersionInfo)
	if idx == -1 {
		return ""
	}

	fixedFileInfo := []byte{0xBD, 0x04, 0xEF, 0xFE}
	idx = findBytes(data[idx:], fixedFileInfo)
	if idx == -1 {
		return ""
	}

	start := idx + 4
	if start+8 > len(data) {
		return ""
	}

	chunk := data[start : start+8]
	minor := binary.LittleEndian.Uint16(chunk[0:2])
	major := binary.LittleEndian.Uint16(chunk[2:4])
	build := binary.LittleEndian.Uint16(chunk[4:6])
	rev := binary.LittleEndian.Uint16(chunk[6:8])

	if major == 0 && minor == 0 {
		return ""
	}

	return formatVersion(major, minor, build, rev)
}

func findBytes(data, pattern []byte) int {
	for i := 0; i <= len(data)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if data[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func formatVersion(major, minor, build, rev uint16) string {
	if rev == 0 {
		if build == 0 {
			return fmt.Sprintf("%d.%d", major, minor)
		}
		return fmt.Sprintf("%d.%d.%d", major, minor, build)
	}
	return fmt.Sprintf("%d.%d.%d.%d", major, minor, build, rev)
}

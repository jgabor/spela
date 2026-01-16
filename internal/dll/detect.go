package dll

import (
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DLLType string

const (
	DLLTypeDLSS  DLLType = "dlss"
	DLLTypeDLSSG DLLType = "dlssg"
	DLLTypeDLSSD DLLType = "dlssd"
	DLLTypeXeSS  DLLType = "xess"
	DLLTypeFSR   DLLType = "fsr"
)

type DetectedDLL struct {
	Path    string
	Name    string
	Type    DLLType
	Version string
}

var knownDLLs = map[string]DLLType{
	"nvngx_dlss.dll":  DLLTypeDLSS,
	"nvngx_dlssg.dll": DLLTypeDLSSG,
	"nvngx_dlssd.dll": DLLTypeDLSSD,
	"libxess.dll":     DLLTypeXeSS,
	"amd_fidelityfx_vk.dll": DLLTypeFSR,
	"amd_fidelityfx_dx12.dll": DLLTypeFSR,
}

func ScanDirectory(dir string) ([]DetectedDLL, error) {
	var results []DetectedDLL

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		name := strings.ToLower(d.Name())
		if dllType, ok := knownDLLs[name]; ok {
			version, _ := GetDLLVersion(path)
			results = append(results, DetectedDLL{
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
	defer f.Close()

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

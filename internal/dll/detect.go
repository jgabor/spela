package dll

import (
	"debug/pe"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

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
	// VS_VERSION_INFO is stored as UTF-16LE in Windows PE files
	vsVersionInfoUTF16 := utf16.Encode([]rune("VS_VERSION_INFO"))
	vsBytes := make([]byte, len(vsVersionInfoUTF16)*2)
	for i, r := range vsVersionInfoUTF16 {
		binary.LittleEndian.PutUint16(vsBytes[i*2:], r)
	}

	vsIndex := findBytes(data, vsBytes)
	if vsIndex == -1 {
		return ""
	}

	// VS_FIXEDFILEINFO signature: 0xFEEF04BD
	fixedFileInfoSignature := []byte{0xBD, 0x04, 0xEF, 0xFE}
	relativeIndex := findBytes(data[vsIndex:], fixedFileInfoSignature)
	if relativeIndex == -1 {
		return ""
	}

	// VS_FIXEDFILEINFO structure layout:
	// offset 0:  dwSignature (4 bytes) - 0xFEEF04BD
	// offset 4:  dwStrucVersion (4 bytes)
	// offset 8:  dwFileVersionMS (4 bytes) - (major << 16) | minor
	// offset 12: dwFileVersionLS (4 bytes) - (build << 16) | revision
	signatureOffset := vsIndex + relativeIndex
	versionOffset := signatureOffset + 8
	if versionOffset+8 > len(data) {
		return ""
	}

	fileVersionMS := binary.LittleEndian.Uint32(data[versionOffset : versionOffset+4])
	fileVersionLS := binary.LittleEndian.Uint32(data[versionOffset+4 : versionOffset+8])

	major := uint16(fileVersionMS >> 16)
	minor := uint16(fileVersionMS & 0xFFFF)
	build := uint16(fileVersionLS >> 16)
	revision := uint16(fileVersionLS & 0xFFFF)

	if major == 0 && minor == 0 {
		return ""
	}

	return formatVersion(major, minor, build, revision)
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

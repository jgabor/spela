package dll

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jgabor/spela/internal/xdg"
)

func GetDLLCachePath(name, version string) string {
	return xdg.CachePath(filepath.Join("dlls", name, version+".dll"))
}

func GetDLLCacheDir(name string) string {
	return xdg.CachePath(filepath.Join("dlls", name))
}

func ListCachedVersions(name string) ([]string, error) {
	entries, err := os.ReadDir(GetDLLCacheDir(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".dll") {
			versions = append(versions, strings.TrimSuffix(entry.Name(), ".dll"))
		}
	}
	return versions, nil
}

func acquireDLL(entry *DLL, dllType string, allowDownload bool, progress func(int64, int64)) (string, error) {
	cachePath := GetDLLCachePath(dllType, entry.Version)
	if hash, err := fileSHA256(cachePath); err == nil && strings.EqualFold(hash, entry.SHA256) {
		return cachePath, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) && !allowDownload {
		return "", fmt.Errorf("read cached DLL: %w", err)
	} else if err == nil && !allowDownload {
		return "", fmt.Errorf("cached DLL checksum mismatch: expected %s, got %s", entry.SHA256, hash)
	}
	if !allowDownload {
		return "", fmt.Errorf("cached DLL not found: %s", cachePath)
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", fmt.Errorf("create cache directory: %w", err)
	}
	response, err := httpClient.Get(entry.URL)
	if err != nil {
		return "", fmt.Errorf("download DLL: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download DLL: HTTP %d", response.StatusCode)
	}

	hasher := sha256.New()
	err = writeAtomically(cachePath, 0o644, func(destination io.Writer) error {
		writer := io.Writer(io.MultiWriter(destination, hasher))
		if progress != nil {
			writer = &progressWriter{writer: writer, total: response.ContentLength, progress: progress}
		}
		if _, copyErr := io.Copy(writer, response.Body); copyErr != nil {
			return copyErr
		}
		actual := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actual, entry.SHA256) {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", entry.SHA256, actual)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("cache DLL: %w", err)
	}
	return cachePath, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type progressWriter struct {
	writer     io.Writer
	total      int64
	downloaded int64
	progress   func(int64, int64)
}

func (writer *progressWriter) Write(data []byte) (int, error) {
	written, err := writer.writer.Write(data)
	writer.downloaded += int64(written)
	writer.progress(writer.downloaded, writer.total)
	return written, err
}

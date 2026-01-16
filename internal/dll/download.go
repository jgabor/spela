package dll

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jgabor/spela/internal/xdg"
)

// ProgressCallback is called during download with bytes downloaded and total size.
// If total is -1, the total size is unknown.
type ProgressCallback func(downloaded, total int64)

func GetDLLCachePath(name, version string) string {
	return xdg.CachePath(filepath.Join("dlls", name, version+".dll"))
}

func IsCached(name, version string) bool {
	cachePath := GetDLLCachePath(name, version)
	_, err := os.Stat(cachePath)
	return err == nil
}

func DownloadDLL(dll *DLL, dllName string) (string, error) {
	return DownloadDLLWithProgress(dll, dllName, nil)
}

func DownloadDLLWithProgress(dll *DLL, dllName string, progress ProgressCallback) (string, error) {
	cachePath := GetDLLCachePath(dllName, dll.Version)

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	resp, err := http.Get(dll.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download DLL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download DLL: HTTP %d", resp.StatusCode)
	}

	tmpPath := cachePath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	hasher := sha256.New()
	writer := io.Writer(io.MultiWriter(out, hasher))

	total := resp.ContentLength
	if progress != nil {
		writer = &progressWriter{
			writer:   writer,
			total:    total,
			progress: progress,
		}
	}

	_, err = io.Copy(writer, resp.Body)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write DLL: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if dll.SHA256 != "" && actualHash != dll.SHA256 {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", dll.SHA256, actualHash)
	}

	if err := os.Rename(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to move DLL to cache: %w", err)
	}

	return cachePath, nil
}

type progressWriter struct {
	writer     io.Writer
	total      int64
	downloaded int64
	progress   ProgressCallback
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	pw.downloaded += int64(n)
	pw.progress(pw.downloaded, pw.total)
	return n, err
}

func GetOrDownloadDLL(manifest *Manifest, dllName, version string) (string, error) {
	var dll *DLL

	if version == "" || version == "latest" {
		dll = manifest.GetLatestDLL(dllName)
	} else {
		dll = manifest.GetDLLVersion(dllName, version)
	}

	if dll == nil {
		return "", fmt.Errorf("DLL not found: %s %s", dllName, version)
	}

	if IsCached(dllName, dll.Version) {
		return GetDLLCachePath(dllName, dll.Version), nil
	}

	return DownloadDLL(dll, dllName)
}

func ClearCache() error {
	cachePath := xdg.CachePath("dlls")
	return os.RemoveAll(cachePath)
}

func GetCacheSize() (int64, error) {
	cachePath := xdg.CachePath("dlls")
	var size int64

	err := filepath.Walk(cachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

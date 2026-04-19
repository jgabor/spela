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

// ProgressCallback is called during download with bytes downloaded and total size.
// If total is -1, the total size is unknown.
type ProgressCallback func(downloaded, total int64)

func GetDLLCachePath(name, version string) string {
	return xdg.CachePath(filepath.Join("dlls", name, version+".dll"))
}

// GetDLLCacheDir returns the per-type cache directory where downloaded DLL
// payloads are stored as "<version>.dll" files.
func GetDLLCacheDir(name string) string {
	return xdg.CachePath(filepath.Join("dlls", name))
}

// ListCachedVersions returns the list of cached DLL versions on disk for the
// given DLL type (manifest key, e.g. "dlss"). Versions are the file basenames
// with the ".dll" suffix trimmed. Returns an empty slice when the per-type
// cache directory does not yet exist — that is not an error, just "nothing
// has been downloaded yet".
func ListCachedVersions(name string) ([]string, error) {
	dir := GetDLLCacheDir(name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".dll") {
			continue
		}
		versions = append(versions, strings.TrimSuffix(name, ".dll"))
	}
	return versions, nil
}

func DownloadDLL(dll *DLL, dllName string) (string, error) {
	return DownloadDLLWithProgress(dll, dllName, nil)
}

func DownloadDLLWithProgress(dll *DLL, dllName string, progress ProgressCallback) (string, error) {
	cachePath := GetDLLCachePath(dllName, dll.Version)

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	resp, err := httpClient.Get(dll.URL)
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
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
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

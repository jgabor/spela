package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FixtureRoot stands for the case folder in every committed text file. A test
// copies a case into a temp folder and replaces this token with that folder.
const FixtureRoot = "/fixture-root"

// fixtureZone is a deliberate non-UTC offset, so the fixtures show how Go
// writes local times.
var fixtureZone = time.FixedZone("", 2*60*60)

// pinnedTime is the time written into every timestamp that Go lets a caller
// choose. Timestamps Go always takes from the clock are left as written.
var pinnedTime = time.Date(2026, 10, 26, 17, 2, 10, 123456789, fixtureZone)

// guardRoot holds the HOME and XDG folders the process uses outside a case.
var guardRoot string

// isolateProcess points HOME and every XDG variable at a new temp folder and
// fixes the local zone. It runs before any package reads the environment.
func isolateProcess() error {
	root, err := os.MkdirTemp("", "spela-goref-*")
	if err != nil {
		return err
	}
	guardRoot = root
	time.Local = fixtureZone
	if err := pointEnvironmentAt(filepath.Join(root, "guard")); err != nil {
		return err
	}
	// Belt and braces for the default HTTP transport: a request to any host
	// other than loopback would go to a proxy on a closed loopback port.
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		_ = os.Setenv(key, closedProxy)
	}
	_ = os.Unsetenv("NO_PROXY")
	_ = os.Unsetenv("no_proxy")
	for _, key := range []string{"SteamAppId", "SteamGameId", "SPELA_LOG", "XDG_STATE_HOME"} {
		_ = os.Unsetenv(key)
	}
	return nil
}

// pointEnvironmentAt sets HOME and the XDG folders under root, the layout a
// case uses: config/, data/, cache/, runtime/ and home/.
func pointEnvironmentAt(root string) error {
	values := map[string]string{
		"HOME":            filepath.Join(root, "home"),
		"XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_DATA_HOME":   filepath.Join(root, "data"),
		"XDG_CACHE_HOME":  filepath.Join(root, "cache"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "runtime"),
		"USER":            "fixture",
	}
	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return os.MkdirAll(filepath.Join(root, "runtime"), 0o700)
}

// fixtureCase is one folder of fixtures, built under a work folder and then
// copied into the output with its path replaced by FixtureRoot.
type fixtureCase struct {
	Name string
	Dir  string
}

// enterCase makes the case folder and points the environment at it.
func (w *fixtureWriter) enterCase(name string) (*fixtureCase, error) {
	dir := filepath.Join(w.work, filepath.FromSlash(name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := pointEnvironmentAt(dir); err != nil {
		return nil, err
	}
	c := &fixtureCase{Name: name, Dir: dir}
	w.cases = append(w.cases, c)
	return c, nil
}

// path joins elements under the case folder.
func (c *fixtureCase) path(elements ...string) string {
	return filepath.Join(append([]string{c.Dir}, elements...)...)
}

// skippedNames are written by Go as side effects and are not fixtures.
var skippedNames = map[string]bool{
	".mutation.lock": true,
	"games.lock":     true,
	"spela.pid":      true,
}

// textExtensions are the files whose content gets the FixtureRoot swap.
var textExtensions = map[string]bool{
	".yaml": true, ".yml": true, ".json": true, ".vdf": true, ".acf": true,
	".txt": true, ".md": true, ".out": true, ".err": true, "": true,
}

// isText reports whether a file is text, by extension and by content.
func isText(path string, data []byte) bool {
	if !textExtensions[strings.ToLower(filepath.Ext(path))] {
		return false
	}
	return !bytes.Contains(data, []byte{0})
}

// copyRelocated copies the tree at source into target and replaces every
// occurrence of from with to in text files. The runtime folder and lock files
// are left out.
func copyRelocated(source, target, from, to string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "runtime" && entry.IsDir() {
			return filepath.SkipDir
		}
		if skippedNames[entry.Name()] {
			return nil
		}
		destination := filepath.Join(target, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("unexpected symlink in fixture tree: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isText(path, data) {
			data = bytes.ReplaceAll(data, []byte(from), []byte(to))
		}
		mode := info.Mode().Perm()
		if mode&0o200 == 0 {
			// Keep a read-only file read-only in the copy too.
			mode = 0o444
		} else {
			mode = 0o644
		}
		return os.WriteFile(destination, data, mode)
	})
}

// writeText writes a file and its parent folders.
func writeText(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// writeBytes writes a binary file and its parent folders.
func writeBytes(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

var hashedBackupName = regexp.MustCompile(`^[0-9a-f]{64}-`)

// rehashBackups renames the sha256 backup files of a copied case so that
// each name is the hash Go computes for the relocated original path under
// FixtureRoot, as if Go had run in /fixture-root. Without this the names
// would hash the random generation folder and change on every run.
func rehashBackups(caseDir string) error {
	indexes, err := filepath.Glob(filepath.Join(caseDir, "data", "spela", "backups", "*", "backup.json"))
	if err != nil {
		return err
	}
	for _, indexPath := range indexes {
		data, err := os.ReadFile(indexPath)
		if err != nil {
			return err
		}
		var index struct {
			Files []struct {
				OriginalPath string `json:"original_path"`
				BackupPath   string `json:"backup_path"`
			} `json:"files"`
		}
		if err := json.Unmarshal(data, &index); err != nil {
			continue // an invalid index fixture stays as written
		}
		text := string(data)
		for _, file := range index.Files {
			name := filepath.Base(file.BackupPath)
			if !hashedBackupName.MatchString(name) || !strings.HasPrefix(file.BackupPath, FixtureRoot+"/") {
				continue
			}
			digest := sha256.Sum256([]byte(filepath.Clean(file.OriginalPath)))
			renamed := fmt.Sprintf("%x-%s", digest, filepath.Base(file.OriginalPath))
			if renamed == name {
				continue
			}
			oldPath := filepath.Join(caseDir, strings.TrimPrefix(file.BackupPath, FixtureRoot+"/"))
			newPath := filepath.Join(filepath.Dir(oldPath), renamed)
			if err := os.Rename(oldPath, newPath); err != nil {
				return err
			}
			text = strings.ReplaceAll(text, name, renamed)
		}
		if text != string(data) {
			if err := os.WriteFile(indexPath, []byte(text), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

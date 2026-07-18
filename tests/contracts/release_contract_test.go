package contracts

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type releaseWorkflow struct {
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		Steps []struct {
			Name string            `yaml:"name"`
			Uses string            `yaml:"uses"`
			Run  string            `yaml:"run"`
			With map[string]string `yaml:"with"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func TestReleaseWorkflowArtifactContract(t *testing.T) {
	repositoryRoot := contractRepositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(repositoryRoot, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow releaseWorkflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatalf("parse release workflow: %v", err)
	}
	if workflow.Permissions["contents"] != "write" {
		t.Fatalf("release contents permission = %q, want write", workflow.Permissions["contents"])
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse release workflow syntax tree: %v", err)
	}
	root := document.Content[0]
	trigger := yamlMappingValue(root, "on")
	push := yamlMappingValue(trigger, "push")
	tags := yamlMappingValue(push, "tags")
	if tags == nil || tags.Kind != yaml.SequenceNode || len(tags.Content) != 1 || tags.Content[0].Value != "v*" {
		t.Fatalf("release tag trigger = %v, want only v*", yamlNodeValues(tags))
	}
	build, ok := workflow.Jobs["build"]
	if !ok {
		t.Fatal("release workflow has no build job")
	}
	steps := make(map[string]struct {
		Uses string
		Run  string
		With map[string]string
	}, len(build.Steps))
	for _, step := range build.Steps {
		steps[step.Name] = struct {
			Uses string
			Run  string
			With map[string]string
		}{step.Uses, step.Run, step.With}
	}
	if got := nonemptyLines(steps["Build unified binary"].Run); !equalStrings(got, []string{"mage build", "mv spela spela-linux-amd64"}) {
		t.Fatalf("release build commands = %q", got)
	}
	if got := strings.TrimSpace(steps["Create checksums"].Run); got != "sha256sum spela-linux-amd64 > checksums.txt" {
		t.Fatalf("release checksum command = %q", got)
	}
	release := steps["Create Release"]
	if !strings.HasPrefix(release.Uses, "softprops/action-gh-release@") {
		t.Fatalf("release action = %q", release.Uses)
	}
	if got := nonemptyLines(release.With["files"]); !equalStrings(got, []string{"spela-linux-amd64", "checksums.txt"}) {
		t.Fatalf("published release files = %q", got)
	}
	if release.With["body_path"] != "release_notes.md" {
		t.Fatalf("release body path = %q", release.With["body_path"])
	}
}

func yamlMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func yamlNodeValues(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	values := make([]string, len(node.Content))
	for index, child := range node.Content {
		values[index] = child.Value
	}
	return values
}

func TestReleaseArtifactContractBuildsFromCleanArchiveAndVerifiesChecksum(t *testing.T) {
	repositoryRoot := contractRepositoryRoot(t)
	archive, err := exec.Command("git", "-C", repositoryRoot, "archive", "--format=tar", "HEAD").Output()
	if err != nil {
		t.Fatalf("create clean source archive: %v", err)
	}
	sourceRoot := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(strings.NewReader(string(archive)))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(sourceRoot, header.Name)
		if !strings.HasPrefix(filepath.Clean(path), sourceRoot+string(filepath.Separator)) {
			t.Fatalf("archive path escapes destination: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(path, os.FileMode(header.Mode))
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
				var file *os.File
				file, err = os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
				if err == nil {
					_, copyErr := io.Copy(file, reader)
					closeErr := file.Close()
					err = copyErr
					if err == nil {
						err = closeErr
					}
				}
			}
		}
		if err != nil {
			t.Fatalf("extract %s: %v", header.Name, err)
		}
	}

	command := exec.Command("mage", "build")
	command.Dir = sourceRoot
	command.Env = isolatedBuildEnvironment(t)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("clean archive mage build: %v\n%s", err, output)
	}
	binary := filepath.Join(sourceRoot, "spela")
	artifact := filepath.Join(sourceRoot, "spela-linux-amd64")
	if err := os.Rename(binary, artifact); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(artifact)
	if err != nil || info.Mode()&0o111 == 0 || info.Size() == 0 {
		t.Fatalf("release binary = info %+v, error %v", info, err)
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	checksumLine := fmt.Sprintf("%s  spela-linux-amd64\n", hex.EncodeToString(digest[:]))
	checksums := filepath.Join(sourceRoot, "checksums.txt")
	if err := os.WriteFile(checksums, []byte(checksumLine), 0o644); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(checksums)
	if err != nil || string(stored) != checksumLine {
		t.Fatalf("release checksum = %q, error %v", stored, err)
	}
	verified := sha256.Sum256(data)
	if verified != digest {
		t.Fatal("release checksum did not verify packaged binary")
	}
}

func contractRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func nonemptyLines(value string) []string {
	var lines []string
	for line := range strings.SplitSeq(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isolatedBuildEnvironment(t *testing.T) []string {
	t.Helper()
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, variable := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		value, err := exec.Command("go", "env", variable).Output()
		if err != nil {
			t.Fatal(err)
		}
		values[variable] = strings.TrimSpace(string(value))
	}
	state := t.TempDir()
	values["HOME"] = filepath.Join(state, "home")
	values["XDG_CONFIG_HOME"] = filepath.Join(state, "config")
	values["XDG_CACHE_HOME"] = filepath.Join(state, "cache")
	values["XDG_DATA_HOME"] = filepath.Join(state, "data")
	values["XDG_STATE_HOME"] = filepath.Join(state, "state")
	values["XDG_RUNTIME_DIR"] = filepath.Join(state, "runtime")
	values["BUN_INSTALL_CACHE_DIR"] = filepath.Join(realHome, ".bun", "install", "cache")
	for _, directory := range values {
		if filepath.IsAbs(directory) {
			_ = os.MkdirAll(directory, 0o755)
		}
	}
	environment := make([]string, 0, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := values[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range values {
		environment = append(environment, key+"="+value)
	}
	return environment
}

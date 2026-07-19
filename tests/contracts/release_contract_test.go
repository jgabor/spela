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
	if got := nonemptyLines(steps["Build unified binary"].Run); !equalStrings(got, []string{"go tool mage build", "mv spela spela-linux-amd64"}) {
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
	if got := strings.TrimSpace(steps["Extract release notes from CHANGELOG.md"].Run); got != `sh scripts/release-notes.sh "${{ github.ref_name }}" > release_notes.md` {
		t.Fatalf("release notes command = %q", got)
	}
	aur, ok := workflow.Jobs["aur-publish"]
	if !ok {
		t.Fatal("release workflow has no aur-publish job")
	}
	var published []string
	for _, step := range aur.Steps {
		if strings.HasPrefix(step.Uses, "KSXGitHub/github-actions-deploy-aur@") {
			published = append(published, step.With["pkgname"]+":"+step.With["pkgbuild"])
		}
	}
	if !equalStrings(published, []string{"spela:pkg/aur/PKGBUILD", "spela-git:pkg/aur/PKGBUILD-git"}) {
		t.Fatalf("AUR publications = %q", published)
	}
}

func TestWorkflowSyntax(t *testing.T) {
	repositoryRoot := contractRepositoryRoot(t)
	workflows, err := filepath.Glob(filepath.Join(repositoryRoot, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(workflows) == 0 {
		t.Fatal("no workflows found")
	}
	for _, path := range workflows {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document yaml.Node
		if err := yaml.Unmarshal(data, &document); err != nil {
			t.Errorf("parse %s: %v", filepath.Base(path), err)
			continue
		}
		if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
			t.Errorf("%s does not contain a workflow mapping", filepath.Base(path))
		}
	}
}

func TestReleaseBuildInputsContract(t *testing.T) {
	repositoryRoot := contractRepositoryRoot(t)
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repositoryRoot, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	goModule := read("go.mod")
	for _, pin := range []string{
		"github.com/magefile/mage v1.15.0",
		"github.com/wailsapp/wails/v2 v2.12.0",
		"tool (",
		"github.com/wailsapp/wails/v2/cmd/wails",
	} {
		if !strings.Contains(goModule, pin) {
			t.Errorf("go.mod does not pin %q", pin)
		}
	}
	if !strings.Contains(read("internal/gui/frontend/package.json"), `"packageManager": "bun@1.3.14"`) {
		t.Error("package.json does not pin Bun 1.3.14")
	}
	for _, workflowPath := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		workflow := read(workflowPath)
		if strings.Contains(workflow, "@latest") || strings.Contains(workflow, "bun-version: latest") {
			t.Errorf("%s contains an unpinned tool", workflowPath)
		}
		if strings.Contains(workflow, "hashFiles('**/bun.lock") || !strings.Contains(workflow, "hashFiles('internal/gui/frontend/bun.lock')") {
			t.Errorf("%s does not use the tracked Bun lockfile cache key", workflowPath)
		}
	}
	if !strings.Contains(read("cmd/spela/wails.json"), `"frontend:install": "bun install --frozen-lockfile"`) {
		t.Error("Wails frontend install is not frozen")
	}

	magefile := read("magefile.go")
	for _, removed := range []string{"type Release mg.Namespace", "func (Release)", "func (Aur) Publish", "func (Aur) Srcinfo", "findGitCliff", "opencode", "gh release"} {
		if strings.Contains(magefile, removed) {
			t.Errorf("magefile retains removed release publisher %q", removed)
		}
	}
	if got := strings.Count(magefile, `"go", "tool", "wails", "build"`); got != 2 {
		t.Errorf("pinned Wails build calls = %d, want 2 (frontend and coverage)", got)
	}
	if strings.Contains(magefile, `environment, "wails", "build"`) {
		t.Error("coverage requires an unpinned Wails executable on PATH")
	}
	for _, path := range []string{"pkg/aur/PKGBUILD", "pkg/aur/PKGBUILD-git"} {
		pkgbuild := read(path)
		if !strings.Contains(pkgbuild, "go tool mage build") {
			t.Errorf("%s does not use the canonical build target", path)
		}
		if !strings.Contains(pkgbuild, `SPELA_VERSION="$pkgver"`) {
			t.Errorf("%s does not preserve the package version in the binary", path)
		}
		if strings.Contains(pkgbuild, "bun run build") || strings.Contains(pkgbuild, "go build ") {
			t.Errorf("%s duplicates canonical build steps", path)
		}
	}
}

func TestReleaseNotesAreExactChangelogSection(t *testing.T) {
	repositoryRoot := contractRepositoryRoot(t)
	command := exec.Command("sh", filepath.Join(repositoryRoot, "scripts", "release-notes.sh"), "v0.6.0")
	command.Dir = repositoryRoot
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	notes := string(output)
	if !strings.Contains(notes, "This minor release makes the trusted profile loop visible") {
		t.Fatalf("unexpected release body: %q", notes)
	}
	if strings.Contains(notes, "## [0.6.0]") || strings.Contains(notes, "## [0.5.1]") || strings.Contains(notes, "This patch release hardens") {
		t.Fatalf("release body crosses changelog section boundary: %q", notes)
	}
	missing := exec.Command("sh", filepath.Join(repositoryRoot, "scripts", "release-notes.sh"), "v999.0.0")
	missing.Dir = repositoryRoot
	if err := missing.Run(); err == nil {
		t.Fatal("missing changelog version unexpectedly produced release notes")
	}
	for _, tag := range []string{
		"", "0.6.0", "v0.6", "v0.6.0.1", "v01.2.3", "v1.02.3", "v1.2.03",
		"v[0].6.0", "v0.*.0", "v0x6x0", "v1.2.3-rc.1", "v1.2.3+meta",
	} {
		command := exec.Command("sh", filepath.Join(repositoryRoot, "scripts", "release-notes.sh"), tag)
		command.Dir = repositoryRoot
		if err := command.Run(); err == nil {
			t.Errorf("malformed tag %q unexpectedly produced release notes", tag)
		}
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
	// Overlay current tracked and newly added source so this nonpublishing test
	// exercises the candidate tree before it has been committed.
	files, err := exec.Command("git", "-C", repositoryRoot, "ls-files", "--cached", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range strings.Split(strings.TrimSuffix(string(files), "\x00"), "\x00") {
		source := filepath.Join(repositoryRoot, name)
		data, readErr := os.ReadFile(source)
		if os.IsNotExist(readErr) {
			_ = os.Remove(filepath.Join(sourceRoot, name))
			continue
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		info, err := os.Stat(source)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(sourceRoot, name)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, info.Mode()); err != nil {
			t.Fatal(err)
		}
	}

	command := exec.Command("go", "tool", "mage", "build")
	command.Dir = sourceRoot
	command.Env = isolatedBuildEnvironment(t)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("clean archive canonical build: %v\n%s", err, output)
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

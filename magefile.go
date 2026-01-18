//go:build mage
// +build mage

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	binaryName  = "spela"
	frontendDir = "internal/gui/frontend"
)

var Default = Build

func findGitCliff() (string, error) {
	if path, err := exec.LookPath("git-cliff"); err == nil {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("git-cliff not found in PATH and cannot determine home directory")
	}

	candidates := []string{
		filepath.Join(home, ".cargo", "bin", "git-cliff"),
		filepath.Join(home, ".local", "bin", "git-cliff"),
		"/usr/local/bin/git-cliff",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("git-cliff not found. Install with: cargo install git-cliff")
}

func version() (string, error) {
	out, err := sh.Output("git", "describe", "--tags", "--always", "--dirty")
	if err != nil || out == "" {
		return "dev", nil
	}
	return out, nil
}

func ldflags() (string, error) {
	v, err := version()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("-s -w -X main.version=%s", v), nil
}

// Build builds the unified spela binary (frontend + backend)
func Build() error {
	mg.Deps(FrontendBuild)

	ldf, err := ldflags()
	if err != nil {
		return err
	}
	return sh.RunV("go", "build", "-tags", "production,webkit2_41", "-ldflags", ldf, "-o", binaryName, "./cmd/spela")
}

// FrontendBuild builds the Svelte frontend
func FrontendBuild() error {
	if err := runInDir(frontendDir, "bun", "install"); err != nil {
		return err
	}
	return runInDir(frontendDir, "bun", "run", "build")
}

func runInDir(dir string, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// Test runs all Go tests
func Test() error {
	return sh.RunV("go", "test", "-v", "./...")
}

// TestFrontend runs frontend tests
func TestFrontend() error {
	return runInDir(frontendDir, "bun", "run", "test")
}

// TestE2E runs Playwright e2e tests
func TestE2E() error {
	return runInDir(frontendDir, "bun", "run", "test:e2e")
}

// Lint runs golangci-lint
func Lint() error {
	return sh.RunV("golangci-lint", "run")
}

// Install installs the binary to GOPATH/bin
func Install() error {
	mg.Deps(FrontendBuild)

	ldf, err := ldflags()
	if err != nil {
		return err
	}
	return sh.RunV("go", "install", "-tags", "production,webkit2_41", "-ldflags", ldf, "./cmd/spela")
}

// Clean removes build artifacts
func Clean() error {
	if err := sh.Rm(binaryName); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := sh.Rm(filepath.Join(frontendDir, "dist")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Dev starts development mode with Vite hot-reload
func Dev() error {
	if err := runInDir(frontendDir, "bun", "install"); err != nil {
		return err
	}

	fmt.Println("Starting Vite dev server in background...")
	viteCmd := exec.Command("bun", "run", "dev")
	viteCmd.Dir = frontendDir
	viteCmd.Stdout = os.Stdout
	viteCmd.Stderr = os.Stderr
	if err := viteCmd.Start(); err != nil {
		return err
	}

	fmt.Println("Building Go binary with dev tag...")
	ldf, err := ldflags()
	if err != nil {
		viteCmd.Process.Kill()
		return err
	}
	if err := sh.RunV("go", "build", "-tags", "dev", "-ldflags", ldf, "-o", binaryName, "./cmd/spela"); err != nil {
		viteCmd.Process.Kill()
		return err
	}

	fmt.Println("Starting spela...")
	return sh.RunV("./"+binaryName, "gui")
}

// DevStop stops the Vite dev server
func DevStop() error {
	return sh.Run("pkill", "-f", "bun run dev")
}

type Release mg.Namespace

// Release performs a full release cycle
func (Release) Release() error {
	if dirty, err := isWorkingDirectoryDirty(); err != nil {
		return err
	} else if dirty {
		return fmt.Errorf("working directory is dirty, commit or stash changes first")
	}

	nextVersion, err := computeNextVersion()
	if err != nil {
		return fmt.Errorf("failed to compute next version: %w", err)
	}

	changelog, err := generateChangelog(nextVersion)
	if err != nil {
		return fmt.Errorf("failed to generate changelog: %w", err)
	}

	summary, err := generateReleaseSummary(changelog)
	if err != nil {
		fmt.Printf("Warning: failed to generate LLM summary: %v\n", err)
		summary = ""
	}

	summary, err = showPreviewAndConfirm(nextVersion, changelog, summary)
	if err != nil {
		return err
	}

	if err := updateChangelogFile(nextVersion, summary); err != nil {
		return fmt.Errorf("failed to update CHANGELOG.md: %w", err)
	}

	if err := commitTagPush(nextVersion); err != nil {
		return fmt.Errorf("failed to commit, tag, or push: %w", err)
	}

	fmt.Printf("\nRelease %s completed successfully!\n", nextVersion)
	return nil
}

// Rollback deletes the most recent tag and reverts the changelog commit
func (Release) Rollback() error {
	tag, err := getLatestTag()
	if err != nil {
		return fmt.Errorf("failed to get latest tag: %w", err)
	}

	fmt.Printf("This will delete tag %s and revert the changelog commit.\n", tag)
	fmt.Print("Continue? [y/N]: ")

	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		return fmt.Errorf("aborted")
	}

	if err := sh.RunV("git", "tag", "-d", tag); err != nil {
		return fmt.Errorf("failed to delete local tag: %w", err)
	}

	if err := sh.RunV("git", "revert", "--no-commit", "HEAD"); err != nil {
		return fmt.Errorf("failed to revert changelog commit: %w", err)
	}

	if err := sh.RunV("git", "commit", "-m", fmt.Sprintf("chore(release): rollback %s", tag)); err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	fmt.Printf("Rolled back %s locally. Run 'git push' to push the rollback.\n", tag)
	fmt.Println("If the tag was already pushed, run: git push origin :refs/tags/" + tag)
	return nil
}

// Redo re-releases the specified version (destructive: deletes remote tag and GitHub release)
func (Release) Redo(version string) error {
	if version == "" {
		return fmt.Errorf("version argument required, e.g., 'mage release:redo v0.1.0'")
	}

	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	fmt.Println("WARNING: This is a destructive operation!")
	fmt.Printf("This will delete the remote tag %s and its GitHub release.\n", version)
	fmt.Print("Type the version to confirm: ")

	var response string
	fmt.Scanln(&response)
	if response != version && response != strings.TrimPrefix(version, "v") {
		return fmt.Errorf("confirmation failed, aborted")
	}

	fmt.Printf("Deleting local tag %s...\n", version)
	sh.Run("git", "tag", "-d", version)

	fmt.Printf("Deleting remote tag %s...\n", version)
	sh.Run("git", "push", "origin", ":refs/tags/"+version)

	fmt.Printf("Deleting GitHub release %s...\n", version)
	sh.Run("gh", "release", "delete", version, "--yes")

	fmt.Println("Now running release with version override...")
	return releaseWithVersion(version)
}

func isWorkingDirectoryDirty() (bool, error) {
	out, err := sh.Output("git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

func computeNextVersion() (string, error) {
	tags, err := sh.Output("git", "tag", "-l", "v*")
	if err != nil || tags == "" {
		fmt.Print("No existing tags found. Enter initial version (e.g., v0.1.0): ")
		var v string
		fmt.Scanln(&v)
		if !strings.HasPrefix(v, "v") {
			v = "v" + v
		}
		return v, nil
	}

	gitCliff, err := findGitCliff()
	if err != nil {
		return "", err
	}
	out, err := sh.Output(gitCliff, "--bumped-version")
	if err != nil {
		return "", fmt.Errorf("git-cliff --bumped-version failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func generateChangelog(version string) (string, error) {
	gitCliff, err := findGitCliff()
	if err != nil {
		return "", err
	}
	out, err := sh.Output(gitCliff, "--unreleased", "--tag", version)
	if err != nil {
		return "", err
	}
	return out, nil
}

func generateReleaseSummary(changelog string) (string, error) {
	prompt := fmt.Sprintf(`You are writing a release summary for a GitHub release. Given the following changelog, write a brief 2-3 sentence summary that highlights the most important changes. Be concise and focus on user-facing features. Do not include markdown formatting.

Changelog:
%s

Summary:`, changelog)

	cmd := exec.Command("opencode", "run", "-m", "opencode/minimax-m2.1-free", prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("opencode failed: %w, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

func showPreviewAndConfirm(version, changelog, summary string) (string, error) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("Release Preview: %s\n", version)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\nChangelog:")
	fmt.Println(changelog)

	if summary != "" {
		fmt.Println("\nLLM-generated summary:")
		fmt.Println(summary)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\n[1] Proceed with release")
	fmt.Println("[2] Edit summary")
	fmt.Println("[3] Abort")
	fmt.Print("\nChoice: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		return summary, nil
	case "2":
		fmt.Print("Enter new summary (single line): ")
		newSummary, _ := reader.ReadString('\n')
		newSummary = strings.TrimSpace(newSummary)
		fmt.Printf("\nNew summary: %s\n", newSummary)
		fmt.Print("Proceed? [y/N]: ")
		confirm, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			return "", fmt.Errorf("aborted")
		}
		return newSummary, nil
	case "3":
		return "", fmt.Errorf("aborted")
	default:
		return "", fmt.Errorf("invalid choice")
	}
}

func updateChangelogFile(version, summary string) error {
	gitCliff, err := findGitCliff()
	if err != nil {
		return err
	}
	if err := sh.RunV(gitCliff, "--tag", version, "-o", "CHANGELOG.md"); err != nil {
		return err
	}

	if summary == "" {
		return nil
	}

	content, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		return fmt.Errorf("failed to read CHANGELOG.md: %w", err)
	}

	v := strings.TrimPrefix(version, "v")
	headerPattern := regexp.MustCompile(`(?m)(## \[` + regexp.QuoteMeta(v) + `\][^\n]*\n)`)
	loc := headerPattern.FindIndex(content)
	if loc == nil {
		return fmt.Errorf("could not find version header for %s in CHANGELOG.md", version)
	}

	headerEnd := loc[1]
	summaryBlock := "\n" + summary + "\n"
	newContent := string(content[:headerEnd]) + summaryBlock + string(content[headerEnd:])

	if err := os.WriteFile("CHANGELOG.md", []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("failed to write CHANGELOG.md: %w", err)
	}

	return nil
}

func commitTagPush(version string) error {
	if err := sh.RunV("git", "add", "CHANGELOG.md"); err != nil {
		return err
	}

	if err := sh.RunV("git", "commit", "-m", fmt.Sprintf("chore(release): %s", version)); err != nil {
		return err
	}

	if err := sh.RunV("git", "tag", "-a", version, "-m", fmt.Sprintf("Release %s", version)); err != nil {
		return err
	}

	if err := sh.RunV("git", "push"); err != nil {
		return err
	}

	return sh.RunV("git", "push", "origin", version)
}

func getLatestTag() (string, error) {
	out, err := sh.Output("git", "describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func releaseWithVersion(version string) error {
	changelog, err := generateChangelog(version)
	if err != nil {
		return fmt.Errorf("failed to generate changelog: %w", err)
	}

	summary, err := generateReleaseSummary(changelog)
	if err != nil {
		fmt.Printf("Warning: failed to generate LLM summary: %v\n", err)
		summary = ""
	}

	summary, err = showPreviewAndConfirm(version, changelog, summary)
	if err != nil {
		return err
	}

	if err := updateChangelogFile(version, summary); err != nil {
		return fmt.Errorf("failed to update CHANGELOG.md: %w", err)
	}

	if err := commitTagPush(version); err != nil {
		return fmt.Errorf("failed to commit, tag, or push: %w", err)
	}

	fmt.Printf("\nRelease %s completed successfully!\n", version)
	return nil
}

// ExtractReleaseNotes extracts release notes for a specific version from CHANGELOG.md
func ExtractReleaseNotes(version string) error {
	content, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		return fmt.Errorf("failed to read CHANGELOG.md: %w", err)
	}

	v := strings.TrimPrefix(version, "v")
	pattern := fmt.Sprintf(`(?s)## \[%s\][^\n]*\n(.*?)(?:\n## \[|\z)`, regexp.QuoteMeta(v))
	re := regexp.MustCompile(pattern)

	matches := re.FindSubmatch(content)
	if matches == nil {
		return fmt.Errorf("release notes for version %s not found in CHANGELOG.md", version)
	}

	notes := strings.TrimSpace(string(matches[1]))
	fmt.Println(notes)
	return nil
}

type Aur mg.Namespace

// Srcinfo generates .SRCINFO files from PKGBUILDs
func (Aur) Srcinfo() error {
	packages := []string{"pkg/aur/PKGBUILD", "pkg/aur/PKGBUILD-git"}
	for _, pkgbuild := range packages {
		dir := filepath.Dir(pkgbuild)
		base := filepath.Base(pkgbuild)

		srcinfo := ".SRCINFO"
		if base == "PKGBUILD-git" {
			srcinfo = ".SRCINFO-git"
		}

		out, err := sh.Output("bash", "-c", fmt.Sprintf("cd %s && makepkg --printsrcinfo -p %s", dir, base))
		if err != nil {
			return fmt.Errorf("failed to generate .SRCINFO for %s: %w", pkgbuild, err)
		}

		if err := os.WriteFile(filepath.Join(dir, srcinfo), []byte(out), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", srcinfo, err)
		}
		fmt.Printf("Generated %s/%s\n", dir, srcinfo)
	}
	return nil
}

// UpdateVersion updates PKGBUILD version and SHA256 checksum
func (Aur) UpdateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version argument required")
	}
	version = strings.TrimPrefix(version, "v")

	pkgbuild := "pkg/aur/PKGBUILD"
	content, err := os.ReadFile(pkgbuild)
	if err != nil {
		return fmt.Errorf("failed to read PKGBUILD: %w", err)
	}

	pkgverRe := regexp.MustCompile(`(?m)^pkgver=.*$`)
	content = pkgverRe.ReplaceAll(content, []byte(fmt.Sprintf("pkgver=%s", version)))

	tarballURL := fmt.Sprintf("https://github.com/jgabor/spela/archive/v%s.tar.gz", version)
	fmt.Printf("Fetching tarball checksum from %s...\n", tarballURL)

	tmpFile, err := os.CreateTemp("", "spela-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	maxRetries := 5
	var downloadErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		downloadErr = sh.Run("curl", "-sfL", "-o", tmpFile.Name(), tarballURL)
		if downloadErr == nil {
			break
		}
		if attempt < maxRetries {
			delay := time.Duration(attempt*attempt) * time.Second
			fmt.Printf("Download failed (attempt %d/%d), retrying in %v...\n", attempt, maxRetries, delay)
			time.Sleep(delay)
		}
	}
	if downloadErr != nil {
		return fmt.Errorf("failed to download tarball after %d attempts: %w", maxRetries, downloadErr)
	}

	checksumOut, err := sh.Output("sha256sum", tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}
	checksum := strings.Fields(checksumOut)[0]

	sha256Re := regexp.MustCompile(`(?m)^sha256sums=\('.*'\)$`)
	content = sha256Re.ReplaceAll(content, []byte(fmt.Sprintf("sha256sums=('%s')", checksum)))

	if err := os.WriteFile(pkgbuild, content, 0o644); err != nil {
		return fmt.Errorf("failed to write PKGBUILD: %w", err)
	}

	fmt.Printf("Updated PKGBUILD: pkgver=%s, sha256=%s\n", version, checksum)
	return nil
}

// Publish clones AUR repo, copies files, commits and pushes
func (Aur) Publish(packageName string) error {
	if packageName != "spela" && packageName != "spela-git" {
		return fmt.Errorf("package must be 'spela' or 'spela-git'")
	}

	tmpDir, err := os.MkdirTemp("", "aur-publish-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	aurURL := fmt.Sprintf("ssh://aur@aur.archlinux.org/%s.git", packageName)
	fmt.Printf("Cloning %s...\n", aurURL)
	if err := sh.Run("git", "clone", aurURL, tmpDir); err != nil {
		return fmt.Errorf("failed to clone AUR repo: %w", err)
	}

	var pkgbuildSrc, srcinfoSrc string
	if packageName == "spela" {
		pkgbuildSrc = "pkg/aur/PKGBUILD"
		srcinfoSrc = "pkg/aur/.SRCINFO"
	} else {
		pkgbuildSrc = "pkg/aur/PKGBUILD-git"
		srcinfoSrc = "pkg/aur/.SRCINFO-git"
	}

	pkgbuildContent, err := os.ReadFile(pkgbuildSrc)
	if err != nil {
		return fmt.Errorf("failed to read PKGBUILD: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "PKGBUILD"), pkgbuildContent, 0o644); err != nil {
		return fmt.Errorf("failed to write PKGBUILD: %w", err)
	}

	srcinfoContent, err := os.ReadFile(srcinfoSrc)
	if err != nil {
		return fmt.Errorf("failed to read .SRCINFO: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".SRCINFO"), srcinfoContent, 0o644); err != nil {
		return fmt.Errorf("failed to write .SRCINFO: %w", err)
	}

	pkgverRe := regexp.MustCompile(`(?m)^pkgver=(.*)$`)
	matches := pkgverRe.FindSubmatch(pkgbuildContent)
	pkgver := "unknown"
	if matches != nil {
		pkgver = string(matches[1])
	}

	gitCmd := func(args ...string) error {
		c := exec.Command("git", args...)
		c.Dir = tmpDir
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	}

	if err := gitCmd("add", "PKGBUILD", ".SRCINFO"); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	status, _ := sh.Output("git", "-C", tmpDir, "status", "--porcelain")
	if len(status) == 0 {
		fmt.Println("No changes to commit")
		return nil
	}

	if err := gitCmd("commit", "-m", fmt.Sprintf("Update to %s", pkgver)); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	if err := gitCmd("push"); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	fmt.Printf("Published %s version %s to AUR\n", packageName, pkgver)
	return nil
}

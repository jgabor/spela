// coverage-inventory verifies that every maintained executable Go source file
// occurs in at least one of the coverprofiles used by the canonical gate.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var productionRoots = []string{"cmd", "internal", "tools"}

type profilePaths []string

func (paths *profilePaths) String() string { return strings.Join(*paths, ",") }

func (paths *profilePaths) Set(path string) error {
	*paths = append(*paths, path)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("coverage-inventory", flag.ContinueOnError)
	var profiles profilePaths
	root := flags.String("root", ".", "repository root")
	flags.Var(&profiles, "profile", "Go coverprofile to include (repeatable)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return verifyInventory(*root, profiles)
}

func verifyInventory(root string, profiles []string) error {
	if len(profiles) == 0 {
		return errors.New("at least one --profile is required")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	module, err := readModulePath(filepath.Join(absoluteRoot, "go.mod"))
	if err != nil {
		return err
	}
	measured := make(map[string]bool)
	for _, profile := range profiles {
		if err := readProfileSources(profile, absoluteRoot, module, measured); err != nil {
			return err
		}
	}

	required, err := executableSources(absoluteRoot)
	if err != nil {
		return err
	}
	var missing []string
	for _, source := range required {
		if !measured[source] {
			missing = append(missing, source)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("executable Go sources absent from coverage profiles: %s", strings.Join(missing, ", "))
	}
	return nil
}

func executableSources(root string) ([]string, error) {
	var sources []string
	for _, productionRoot := range productionRoots {
		directory := filepath.Join(root, productionRoot)
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			syntax, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			executable := false
			ast.Inspect(syntax, func(node ast.Node) bool {
				switch body := node.(type) {
				case *ast.FuncDecl:
					executable = executable || body.Body != nil && len(body.Body.List) > 0
				case *ast.FuncLit:
					executable = executable || len(body.Body.List) > 0
				}
				return !executable
			})
			if executable {
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				sources = append(sources, filepath.ToSlash(relative))
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("inventory %s: %w", productionRoot, err)
		}
	}
	sort.Strings(sources)
	return sources, nil
}

func readModulePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open go.mod: %w", err)
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	return "", errors.New("module declaration not found in go.mod")
}

func readProfileSources(path, root, module string, sources map[string]bool) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open coverprofile %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			return fmt.Errorf("invalid coverprofile record in %s: %q", path, line)
		}
		source := filepath.ToSlash(line[:colon])
		switch {
		case strings.HasPrefix(source, module+"/"):
			source = strings.TrimPrefix(source, module+"/")
		case filepath.IsAbs(source):
			relative, err := filepath.Rel(root, source)
			if err != nil || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				continue
			}
			source = filepath.ToSlash(relative)
		}
		sources[source] = true
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read coverprofile %s: %w", path, err)
	}
	return nil
}

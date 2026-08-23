//go:build ignore

// collect-go-licenses emits legal notices for the modules linked into a target.
// It intentionally uses only the Go standard library so release builds do not
// require a separate license-scanner dependency.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxLegalFileBytes = 5 << 20

var legalFilename = regexp.MustCompile(`(?i)^(licen[cs]e|copying|copyright|notice|patents?)([._-].*)?$`)

type module struct {
	Path    string  `json:"Path"`
	Version string  `json:"Version"`
	Main    bool    `json:"Main"`
	Dir     string  `json:"Dir"`
	Replace *module `json:"Replace"`
}

type listedPackage struct {
	Standard bool    `json:"Standard"`
	Module   *module `json:"Module"`
}

type legalFile struct {
	Name string
	Text string
}

type moduleNotice struct {
	Path        string
	Version     string
	Replacement string
	Files       []legalFile
}

func main() {
	serverRoot := flag.String("server-root", "src/server", "directory containing go.mod")
	target := flag.String("target", "./cmd/sing-box-observability", "Go package to inspect")
	tags := flag.String("tags", "webui", "Go build tags")
	goos := flag.String("goos", "android", "target GOOS")
	goarch := flag.String("goarch", "arm64", "target GOARCH")
	inventoryPath := flag.String("inventory", "", "output GO-MODULES.txt path")
	noticesPath := flag.String("notices", "", "output GO-LICENSES.txt path")
	flag.Parse()

	if *inventoryPath == "" || *noticesPath == "" {
		fatalf("-inventory and -notices are required")
	}

	root, err := filepath.Abs(*serverRoot)
	if err != nil {
		fatalf("resolve server root: %v", err)
	}
	modules, err := linkedModules(root, *target, *tags, *goos, *goarch)
	if err != nil {
		fatalf("discover linked modules: %v", err)
	}
	if len(modules) == 0 {
		fatalf("no linked third-party Go modules were found")
	}

	if err := writeOutputs(*inventoryPath, *noticesPath, modules); err != nil {
		fatalf("write license outputs: %v", err)
	}
	fmt.Printf("Collected legal notices for %d linked Go modules.\n", len(modules))
}

func linkedModules(serverRoot, target, tags, goos, goarch string) ([]moduleNotice, error) {
	args := []string{"list", "-mod=readonly", "-deps", "-json"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, target)
	command := exec.Command("go", args...)
	command.Dir = serverRoot
	command.Env = replaceEnvironment(os.Environ(), map[string]string{
		"CGO_ENABLED": "0",
		"GOARCH":      goarch,
		"GOOS":        goos,
	})
	var stderr bytes.Buffer
	command.Stderr = &stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}

	unique := map[string]moduleNotice{}
	decoder := json.NewDecoder(stdout)
	for {
		var pkg listedPackage
		err := decoder.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		if pkg.Standard || pkg.Module == nil || pkg.Module.Main {
			continue
		}

		original := pkg.Module
		effective := original
		replacement := ""
		if original.Replace != nil {
			effective = original.Replace
			if effective.Version == "" {
				replacement = "local replacement"
			} else {
				replacement = strings.TrimSpace(effective.Path + " " + effective.Version)
			}
		}
		if effective.Dir == "" {
			return nil, fmt.Errorf("module %s %s has no downloaded source directory", original.Path, original.Version)
		}
		version := original.Version
		if version == "" {
			version = "(unversioned)"
		}
		key := original.Path + "@" + version
		if _, found := unique[key]; found {
			continue
		}
		files, err := legalFiles(effective.Dir)
		if err != nil {
			return nil, fmt.Errorf("module %s: %w", key, err)
		}
		if len(files) == 0 {
			return nil, fmt.Errorf(
				"module %s has no discoverable LICENSE, NOTICE, COPYING, COPYRIGHT, or PATENTS file",
				key,
			)
		}
		unique[key] = moduleNotice{
			Path:        original.Path,
			Version:     version,
			Replacement: replacement,
			Files:       files,
		}
	}
	if err := command.Wait(); err != nil {
		return nil, fmt.Errorf("go list failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	result := make([]moduleNotice, 0, len(unique))
	for _, item := range unique {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func legalFiles(directory string) ([]legalFile, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var result []legalFile
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !legalFilename.MatchString(entry.Name()) {
			continue
		}
		filename := filepath.Join(directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.Size() > maxLegalFileBytes {
			return nil, fmt.Errorf("legal file is unexpectedly large: %s", filename)
		}
		contents, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(contents) {
			return nil, fmt.Errorf("legal file is not UTF-8: %s", filename)
		}
		result = append(result, legalFile{Name: entry.Name(), Text: normalizeText(string(contents))})
	}
	return result, nil
}

func writeOutputs(inventoryPath, noticesPath string, modules []moduleNotice) error {
	var inventory strings.Builder
	for _, item := range modules {
		fmt.Fprintf(&inventory, "%s %s", item.Path, item.Version)
		if item.Replacement != "" {
			fmt.Fprintf(&inventory, " => %s", item.Replacement)
		}
		inventory.WriteByte('\n')
	}

	var notices strings.Builder
	notices.WriteString("GO THIRD-PARTY LICENSES\n\n")
	notices.WriteString("Generated deterministically from modules linked into the Android ARM64 webui build.\n")
	notices.WriteString("No build-machine paths are included.\n\n")
	for _, item := range modules {
		notices.WriteString(strings.Repeat("=", 80) + "\n")
		fmt.Fprintf(&notices, "%s %s\n", item.Path, item.Version)
		if item.Replacement != "" {
			fmt.Fprintf(&notices, "Replacement: %s\n", item.Replacement)
		}
		notices.WriteByte('\n')
		for _, file := range item.Files {
			notices.WriteString(strings.Repeat("-", 80) + "\n")
			notices.WriteString(file.Name + "\n")
			notices.WriteString(strings.Repeat("-", 80) + "\n")
			notices.WriteString(strings.TrimRight(file.Text, "\n") + "\n\n")
		}
	}

	if err := writeFile(inventoryPath, inventory.String()); err != nil {
		return err
	}
	return writeFile(noticesPath, notices.String())
}

func writeFile(filename, contents string) error {
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return err
	}
	return os.WriteFile(absolute, []byte(normalizeText(contents)), 0o644)
}

func normalizeText(value string) string {
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimRight(value, "\n") + "\n"
}

func replaceEnvironment(existing []string, replacements map[string]string) []string {
	result := make([]string, 0, len(existing)+len(replacements))
	for _, entry := range existing {
		key, _, found := strings.Cut(entry, "=")
		if found {
			if _, replaced := replacements[strings.ToUpper(key)]; replaced {
				continue
			}
		}
		result = append(result, entry)
	}
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, key+"="+replacements[key])
	}
	return result
}

func fatalf(format string, arguments ...any) {
	fmt.Fprintf(os.Stderr, "collect-go-licenses: "+format+"\n", arguments...)
	os.Exit(1)
}

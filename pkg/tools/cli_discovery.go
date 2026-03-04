// Package tools provides tool functionality for GoClaw.
package tools

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CliCategory represents the category of a CLI tool.
type CliCategory string

const (
	CliCategoryVersionControl CliCategory = "VersionControl"
	CliCategoryLanguage       CliCategory = "Language"
	CliCategoryPackageManager  CliCategory = "PackageManager"
	CliCategoryContainer      CliCategory = "Container"
	CliCategoryBuild          CliCategory = "Build"
	CliCategoryCloud          CliCategory = "Cloud"
)

// DiscoveredCli represents a discovered CLI tool with metadata.
type DiscoveredCli struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Version  string     `json:"version"`
	Category CliCategory `json:"category"`
}

// KnownCli represents a known CLI tool to scan for.
type KnownCli struct {
	Name        string
	VersionArgs []string
	Category    CliCategory
}

// Known CLI tools to scan for.
var knownClis = []KnownCli{
	{Name: "git", VersionArgs: []string{"--version"}, Category: CliCategoryVersionControl},
	{Name: "python", VersionArgs: []string{"--version"}, Category: CliCategoryLanguage},
	{Name: "python3", VersionArgs: []string{"--version"}, Category: CliCategoryLanguage},
	{Name: "node", VersionArgs: []string{"--version"}, Category: CliCategoryLanguage},
	{Name: "npm", VersionArgs: []string{"--version"}, Category: CliCategoryPackageManager},
	{Name: "pip", VersionArgs: []string{"--version"}, Category: CliCategoryPackageManager},
	{Name: "pip3", VersionArgs: []string{"--version"}, Category: CliCategoryPackageManager},
	{Name: "docker", VersionArgs: []string{"--version"}, Category: CliCategoryContainer},
	{Name: "cargo", VersionArgs: []string{"--version"}, Category: CliCategoryBuild},
	{Name: "make", VersionArgs: []string{"--version"}, Category: CliCategoryBuild},
	{Name: "kubectl", VersionArgs: []string{"version", "--client", "--short"}, Category: CliCategoryCloud},
	{Name: "rustc", VersionArgs: []string{"--version"}, Category: CliCategoryLanguage},
	{Name: "go", VersionArgs: []string{"version"}, Category: CliCategoryLanguage},
	{Name: "bun", VersionArgs: []string{"--version"}, Category: CliCategoryLanguage},
	{Name: "pnpm", VersionArgs: []string{"--version"}, Category: CliCategoryPackageManager},
	{Name: "yarn", VersionArgs: []string{"--version"}, Category: CliCategoryPackageManager},
	{Name: "ansible", VersionArgs: []string{"--version"}, Category: CliCategoryCloud},
	{Name: "terraform", VersionArgs: []string{"version"}, Category: CliCategoryCloud},
	{Name: "helm", VersionArgs: []string{"version"}, Category: CliCategoryCloud},
}

// DiscoverCliTools discovers available CLI tools on the system.
// Scans PATH for known tools and returns metadata for each found.
func DiscoverCliTools(additional []string, excluded []string) []DiscoveredCli {
	results := make([]DiscoveredCli, 0)
	excludedMap := make(map[string]bool)
	for _, e := range excluded {
		excludedMap[e] = true
	}

	for _, known := range knownClis {
		if excludedMap[known.Name] {
			continue
		}
		if cli := probeCli(known.Name, known.VersionArgs, known.Category); cli != nil {
			results = append(results, *cli)
		}
	}

	// Probe additional user-specified tools
	discoveredNames := make(map[string]bool)
	for _, r := range results {
		discoveredNames[r.Name] = true
	}

	for _, toolName := range additional {
		if excludedMap[toolName] || discoveredNames[toolName] {
			continue
		}
		if cli := probeCli(toolName, []string{"--version"}, CliCategoryBuild); cli != nil {
			results = append(results, *cli)
		}
	}

	return results
}

// probeCli probes a single CLI tool: check if it exists and get its version.
func probeCli(name string, versionArgs []string, category CliCategory) *DiscoveredCli {
	path := findExecutable(name)
	if path == "" {
		return nil
	}

	version := getVersion(name, versionArgs)

	return &DiscoveredCli{
		Name:     name,
		Path:     path,
		Version:  version,
		Category: category,
	}
}

// findExecutable finds an executable on PATH.
func findExecutable(name string) string {
	var whichCmd string
	if runtime.GOOS == "windows" {
		whichCmd = "where"
	} else {
		whichCmd = "which"
	}

	cmd := exec.Command(whichCmd, name)
	cmd.Stderr = nil
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return ""
	}

	path := strings.TrimSpace(lines[0])
	if path == "" {
		return ""
	}

	// Clean the path
	return filepath.Clean(path)
}

// getVersion gets the version string of a CLI tool.
func getVersion(name string, args []string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to extract error message
		errOutput := strings.TrimSpace(string(output))
		if errOutput != "" {
			// Return first line of error output
			lines := strings.Split(errOutput, "\n")
			if len(lines) > 0 {
				return strings.TrimSpace(lines[0])
			}
		}
		return ""
	}

	versionText := strings.TrimSpace(string(output))
	if versionText == "" {
		return ""
	}

	// Extract first line only
	lines := strings.Split(versionText, "\n")
	return strings.TrimSpace(lines[0])
}

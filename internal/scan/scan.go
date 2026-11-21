// Package scan finds what a project uses from the files in its folder: the dependencies its manifests name (with
// the versions its lock files pin), the images its Dockerfiles and Compose files run, the tools its configuration
// files belong to, and its programming languages. No AI and no network: only the files.
package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Term is one thing the project uses.
type Term struct {
	// Name is the product's name as people write it ("Laravel", "React", "PostgreSQL").
	Name string `json:"name"`
	// Package is the package or image it was found by ("laravel/framework"), empty when found by a file.
	Package string `json:"package,omitempty"`
	// Version is the version the files pin or ask for, empty when none.
	Version string `json:"version,omitempty"`
	// File is the file it was found in, relative to the project's folder.
	File string `json:"file"`
	// Dev is true when only development needs it (require-dev, devDependencies).
	Dev bool `json:"dev"`
}

// skippedDirs are never searched: dependencies, build output, caches.
var skippedDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true, "target": true, ".idea": true,
	".vscode": true, ".cache": true, "var": true, "tmp": true, "coverage": true, "__pycache__": true, ".venv": true,
	"venv": true, ".next": true, ".nuxt": true, "bin": true, "obj": true, ".gradle": true, "storage": true,
}

// maxDepth is how deep below the project's folder manifests are looked for (monorepos keep them in sub-folders).
const maxDepth = 4

// Folder scans a project's folder and returns its terms, each name once (the first file wins, a later one fills a
// missing version; a term is "dev" only when every file says so), sorted by name.
func Folder(root string) ([]Term, error) {
	var found []Term
	extensions := map[string]int{}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		relative, _ := filepath.Rel(root, path)
		depth := strings.Count(relative, string(filepath.Separator))

		if entry.IsDir() {
			if path == root {
				return nil
			}

			// A tool's folder counts also when it is not searched (.git).
			found = append(found, byFolder(root, relative, entry.Name())...)

			if skippedDirs[entry.Name()] || depth >= maxDepth {
				return filepath.SkipDir
			}

			return nil
		}

		extensions[languageKey(entry.Name())]++
		found = append(found, byFile(root, relative, entry.Name())...)

		return nil
	})

	if err != nil {
		return nil, err
	}

	found = append(found, languages(extensions)...)

	return merge(found), nil
}

// byFile reads the files that name what the project uses.
func byFile(root, relative, name string) []Term {
	path := filepath.Join(root, relative)
	lower := strings.ToLower(name)

	switch {
	case name == "composer.json":
		return composer(path, relative)
	case name == "package.json":
		return npm(path, relative)
	case name == "go.mod":
		return goModules(path, relative)
	case name == "requirements.txt" || (strings.HasPrefix(lower, "requirements") && strings.HasSuffix(lower, ".txt")):
		return pythonRequirements(path, relative)
	case name == "pyproject.toml":
		return pyproject(path, relative)
	case name == "Gemfile":
		return gemfile(path, relative)
	case name == "pom.xml":
		return maven(path, relative)
	case name == "build.gradle" || name == "build.gradle.kts":
		return gradle(path, relative)
	case strings.HasSuffix(lower, ".csproj"):
		return csproj(path, relative)
	case name == "Cargo.toml":
		return cargo(path, relative)
	case lower == "dockerfile" || strings.HasPrefix(lower, "dockerfile.") || strings.HasSuffix(lower, ".dockerfile"):
		return dockerfile(path, relative)
	case isComposeFile(lower):
		return composeFile(path, relative)
	}

	return toolsByFile(relative, name)
}

// readFile returns a file's content, empty when it cannot be read or is larger than 5 MB.
func readFile(path string) string {
	info, err := os.Stat(path)

	if err != nil || info.Size() > 5<<20 {
		return ""
	}

	content, err := os.ReadFile(path)

	if err != nil {
		return ""
	}

	return string(content)
}

// merge keeps each name once (case aside), sorted by name.
func merge(terms []Term) []Term {
	byName := map[string]*Term{}
	var order []string

	for _, term := range terms {
		term.Name = strings.TrimSpace(term.Name)

		if term.Name == "" {
			continue
		}

		key := strings.ToLower(term.Name)
		kept, ok := byName[key]

		if !ok {
			copied := term
			byName[key] = &copied
			order = append(order, key)

			continue
		}

		if kept.Version == "" {
			kept.Version = term.Version
		}

		if kept.Package == "" {
			kept.Package = term.Package
		}

		kept.Dev = kept.Dev && term.Dev
	}

	result := make([]Term, 0, len(order))

	for _, key := range order {
		result = append(result, *byName[key])
	}

	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result
}

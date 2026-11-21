package scan

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// versionPattern finds the first version number in a constraint ("^8.3" → "8.3", ">=3.10,<4" → "3.10").
var versionPattern = regexp.MustCompile(`\d+(?:\.\d+){0,3}`)

// versionOf is the first version number of a constraint or tag, empty when it has none.
func versionOf(constraint string) string {
	return versionPattern.FindString(strings.TrimPrefix(strings.TrimSpace(constraint), "v"))
}

// sortedKeys are a map's keys in order (Go's map order is random; the results must not be).
func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

// sibling is a file beside another one ("composer.lock" beside "composer.json").
func sibling(path, name string) string {
	return filepath.Join(filepath.Dir(path), name)
}

// composer reads composer.json and the exact versions of composer.lock.
func composer(path, relative string) []Term {
	var manifest struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}

	if json.Unmarshal([]byte(readFile(path)), &manifest) != nil {
		return nil
	}

	var lock struct {
		Packages    []struct{ Name, Version string } `json:"packages"`
		PackagesDev []struct{ Name, Version string } `json:"packages-dev"`
	}

	locked := map[string]string{}

	if json.Unmarshal([]byte(readFile(sibling(path, "composer.lock"))), &lock) == nil {
		for _, item := range append(lock.Packages, lock.PackagesDev...) {
			locked[strings.ToLower(item.Name)] = versionOf(item.Version)
		}
	}

	var terms []Term
	add := func(packages map[string]string, dev bool) {
		for _, name := range sortedKeys(packages) {
			constraint := packages[name]
			lower := strings.ToLower(name)

			switch {
			case lower == "php":
				terms = append(terms, Term{Name: "PHP", Version: versionOf(constraint), File: relative, Dev: dev})
			case strings.HasPrefix(lower, "ext-") || strings.HasPrefix(lower, "lib-") || !strings.Contains(lower, "/"):
				// PHP extensions and platform packages are no products of their own.
			default:
				version := locked[lower]

				if version == "" {
					version = versionOf(constraint)
				}

				terms = append(terms, Term{Name: displayName("composer", lower), Package: lower, Version: version, File: relative, Dev: dev})
			}
		}
	}

	add(manifest.Require, false)
	add(manifest.RequireDev, true)

	return append(terms, Term{Name: "Composer", File: relative, Dev: true})
}

// npm reads package.json and the exact versions of package-lock.json, yarn.lock or pnpm-lock.yaml.
func npm(path, relative string) []Term {
	var manifest struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Engines         map[string]string `json:"engines"`
	}

	if json.Unmarshal([]byte(readFile(path)), &manifest) != nil {
		return nil
	}

	locked := npmLocked(path)
	var terms []Term
	add := func(packages map[string]string, dev bool) {
		for _, name := range sortedKeys(packages) {
			constraint := packages[name]
			lower := strings.ToLower(name)
			version := locked[lower]

			if version == "" {
				version = versionOf(constraint)
			}

			terms = append(terms, Term{Name: displayName("npm", lower), Package: lower, Version: version, File: relative, Dev: dev})
		}
	}

	add(manifest.Dependencies, false)
	add(manifest.DevDependencies, true)

	if node, ok := manifest.Engines["node"]; ok {
		terms = append(terms, Term{Name: "Node.js", Version: versionOf(node), File: relative})
	} else {
		terms = append(terms, Term{Name: "Node.js", File: relative})
	}

	switch {
	case readFile(sibling(path, "yarn.lock")) != "":
		terms = append(terms, Term{Name: "Yarn", File: relative, Dev: true})
	case readFile(sibling(path, "pnpm-lock.yaml")) != "":
		terms = append(terms, Term{Name: "pnpm", File: relative, Dev: true})
	default:
		terms = append(terms, Term{Name: "npm", File: relative, Dev: true})
	}

	return terms
}

var (
	yarnEntry = regexp.MustCompile(`(?m)^"?(@?[^@\s"]+)@[^\n]*:\n\s+version:? "?([^"\s]+)"?`)
	pnpmEntry = regexp.MustCompile(`(?m)^\s{2}'?/?(@?[^@\s']+)@(\d[^:'(\s]*)`)
)

// npmLocked are the exact versions a lock file beside package.json pins, by package name.
func npmLocked(path string) map[string]string {
	locked := map[string]string{}
	var lock struct {
		Packages     map[string]struct{ Version string } `json:"packages"`
		Dependencies map[string]struct{ Version string } `json:"dependencies"`
	}

	if json.Unmarshal([]byte(readFile(sibling(path, "package-lock.json"))), &lock) == nil {
		for key, item := range lock.Packages {
			// Only the packages installed at the top ("node_modules/react", not nested ones).
			if name, ok := strings.CutPrefix(key, "node_modules/"); ok && !strings.Contains(name, "/node_modules/") {
				locked[strings.ToLower(name)] = item.Version
			}
		}

		for name, item := range lock.Dependencies {
			if _, ok := locked[strings.ToLower(name)]; !ok {
				locked[strings.ToLower(name)] = item.Version
			}
		}
	}

	for _, match := range yarnEntry.FindAllStringSubmatch(readFile(sibling(path, "yarn.lock")), -1) {
		if _, ok := locked[strings.ToLower(match[1])]; !ok {
			locked[strings.ToLower(match[1])] = match[2]
		}
	}

	for _, match := range pnpmEntry.FindAllStringSubmatch(readFile(sibling(path, "pnpm-lock.yaml")), -1) {
		if _, ok := locked[strings.ToLower(match[1])]; !ok {
			locked[strings.ToLower(match[1])] = match[2]
		}
	}

	return locked
}

var (
	goDirective = regexp.MustCompile(`(?m)^go\s+(\d+(?:\.\d+)*)`)
	goRequire   = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([a-zA-Z0-9.\-_~]+\.[a-z]{2,}/[^\s]+)\s+v(\d[^\s]*)(.*)$`)
)

// goModules reads go.mod: the Go version and the modules it requires directly.
func goModules(path, relative string) []Term {
	content := readFile(path)
	var terms []Term

	if match := goDirective.FindStringSubmatch(content); match != nil {
		terms = append(terms, Term{Name: "Go", Version: match[1], File: relative})
	}

	for _, match := range goRequire.FindAllStringSubmatch(content, -1) {
		if strings.Contains(match[3], "// indirect") {
			continue
		}

		lower := strings.ToLower(match[1])
		terms = append(terms, Term{Name: displayName("go", lower), Package: lower, Version: versionOf(match[2]), File: relative})
	}

	return terms
}

var requirementLine = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._\-]*)(?:\[[^\]]*\])?\s*(?:[=~<>!]=?\s*([0-9][^\s,;]*))?`)

// pythonRequirements reads a requirements file: one package a line.
func pythonRequirements(path, relative string) []Term {
	var terms []Term
	dev := strings.Contains(strings.ToLower(filepath.Base(path)), "dev") || strings.Contains(strings.ToLower(filepath.Base(path)), "test")

	for _, line := range strings.Split(readFile(path), "\n") {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") || strings.Contains(line, "://") {
			continue
		}

		if match := requirementLine.FindStringSubmatch(line); match != nil {
			lower := strings.ToLower(match[1])
			terms = append(terms, Term{Name: displayName("pypi", lower), Package: lower, Version: versionOf(match[2]), File: relative, Dev: dev})
		}
	}

	return append(terms, Term{Name: "Python", File: relative})
}

var (
	tomlSection      = regexp.MustCompile(`(?m)^\[([^\]]+)\]\s*$`)
	tomlKeyValue     = regexp.MustCompile(`(?m)^([A-Za-z0-9_.\-]+)\s*=\s*(.+)$`)
	tomlQuoted       = regexp.MustCompile(`"([^"]+)"`)
	pythonDependency = regexp.MustCompile(`(?s)dependencies\s*=\s*\[(.*?)\]`)
)

// tomlSections splits a TOML file into its sections' bodies, by name ("" for the top).
func tomlSections(content string) map[string]string {
	sections := map[string]string{}
	indexes := tomlSection.FindAllStringSubmatchIndex(content, -1)
	start, name := 0, ""

	for _, index := range indexes {
		sections[name] += content[start:index[0]]
		name, start = content[index[2]:index[3]], index[1]
	}

	sections[name] += content[start:]

	return sections
}

// pyproject reads pyproject.toml: [project] dependencies and requires-python, Poetry's dependencies.
func pyproject(path, relative string) []Term {
	sections := tomlSections(readFile(path))
	terms := []Term{{Name: "Python", File: relative}}

	if match := regexp.MustCompile(`requires-python\s*=\s*"([^"]+)"`).FindStringSubmatch(sections["project"]); match != nil {
		terms[0].Version = versionOf(match[1])
	}

	if match := pythonDependency.FindStringSubmatch(sections["project"]); match != nil {
		for _, quoted := range tomlQuoted.FindAllStringSubmatch(match[1], -1) {
			if requirement := requirementLine.FindStringSubmatch(quoted[1]); requirement != nil {
				lower := strings.ToLower(requirement[1])
				terms = append(terms, Term{Name: displayName("pypi", lower), Package: lower, Version: versionOf(requirement[2]), File: relative})
			}
		}
	}

	poetry := map[string]bool{"tool.poetry.dependencies": false, "tool.poetry.dev-dependencies": true, "tool.poetry.group.dev.dependencies": true}

	for _, section := range sortedKeys(poetry) {
		dev := poetry[section]

		for _, match := range tomlKeyValue.FindAllStringSubmatch(sections[section], -1) {
			lower := strings.ToLower(match[1])

			if lower == "python" {
				terms[0].Version = versionOf(match[2])

				continue
			}

			terms = append(terms, Term{Name: displayName("pypi", lower), Package: lower, Version: versionOf(match[2]), File: relative, Dev: dev})
		}
	}

	if sections["tool.poetry"] != "" {
		terms = append(terms, Term{Name: "Poetry", File: relative, Dev: true})
	}

	return terms
}

var (
	gemLine     = regexp.MustCompile(`(?m)^\s*gem\s+['"]([^'"]+)['"](?:\s*,\s*['"]([^'"]+)['"])?`)
	gemLocked   = regexp.MustCompile(`(?m)^    ([a-zA-Z0-9_\-]+) \(([0-9][^)]*)\)`)
	rubyVersion = regexp.MustCompile(`(?m)^\s*ruby\s+['"]([^'"]+)['"]`)
)

// gemfile reads a Gemfile and the exact versions of Gemfile.lock.
func gemfile(path, relative string) []Term {
	content := readFile(path)
	locked := map[string]string{}

	for _, match := range gemLocked.FindAllStringSubmatch(readFile(sibling(path, "Gemfile.lock")), -1) {
		locked[strings.ToLower(match[1])] = versionOf(match[2])
	}

	terms := []Term{{Name: "Ruby", File: relative}, {Name: "Bundler", File: relative, Dev: true}}

	if match := rubyVersion.FindStringSubmatch(content); match != nil {
		terms[0].Version = versionOf(match[1])
	}

	for _, match := range gemLine.FindAllStringSubmatch(content, -1) {
		lower := strings.ToLower(match[1])
		version := locked[lower]

		if version == "" {
			version = versionOf(match[2])
		}

		terms = append(terms, Term{Name: displayName("rubygems", lower), Package: lower, Version: version, File: relative})
	}

	return terms
}

var (
	mavenDependency = regexp.MustCompile(`(?s)<dependency>(.*?)</dependency>`)
	mavenParent     = regexp.MustCompile(`(?s)<parent>(.*?)</parent>`)
	xmlTag          = func(tag string) *regexp.Regexp {
		return regexp.MustCompile(`<` + tag + `>\s*([^<]+?)\s*</` + tag + `>`)
	}
	mavenGroup    = xmlTag("groupId")
	mavenArtifact = xmlTag("artifactId")
	mavenVersion  = xmlTag("version")
	mavenScope    = xmlTag("scope")
	javaVersion   = regexp.MustCompile(`<(?:java\.version|maven\.compiler\.(?:source|release))>\s*([0-9.]+)\s*<`)
)

// maven reads pom.xml: the parent (Spring Boot), the Java version and the dependencies.
func maven(path, relative string) []Term {
	content := readFile(path)
	terms := []Term{{Name: "Java", File: relative}, {Name: "Maven", File: relative, Dev: true}}

	if match := javaVersion.FindStringSubmatch(content); match != nil {
		terms[0].Version = match[1]
	}

	if parent := mavenParent.FindStringSubmatch(content); parent != nil {
		terms = append(terms, mavenTerm(parent[1], relative))
	}

	for _, dependency := range mavenDependency.FindAllStringSubmatch(content, -1) {
		terms = append(terms, mavenTerm(dependency[1], relative))
	}

	return terms
}

func mavenTerm(body, relative string) Term {
	field := func(pattern *regexp.Regexp) string {
		if match := pattern.FindStringSubmatch(body); match != nil {
			return match[1]
		}

		return ""
	}
	coordinates := strings.ToLower(field(mavenGroup) + ":" + field(mavenArtifact))
	version := field(mavenVersion)

	if strings.HasPrefix(version, "$") {
		version = ""
	}

	return Term{Name: displayName("maven", coordinates), Package: coordinates, Version: versionOf(version), File: relative, Dev: field(mavenScope) == "test"}
}

var (
	gradleDependency = regexp.MustCompile(`(?m)^\s*(\w+)\s*\(?\s*['"]([^'":]+):([^'":]+)(?::([^'"]+))?['"]`)
	gradlePlugin     = regexp.MustCompile(`(?m)id\s*\(?\s*['"]([^'"]+)['"]\s*\)?\s*version\s*['"]([^'"]+)['"]`)
)

// gradle reads build.gradle(.kts): plugins with versions and the dependencies.
func gradle(path, relative string) []Term {
	content := readFile(path)
	terms := []Term{{Name: "Gradle", File: relative, Dev: true}}

	if strings.HasSuffix(path, ".kts") {
		terms = append(terms, Term{Name: "Kotlin", File: relative})
	} else {
		terms = append(terms, Term{Name: "Java", File: relative})
	}

	for _, match := range gradlePlugin.FindAllStringSubmatch(content, -1) {
		lower := strings.ToLower(match[1])
		terms = append(terms, Term{Name: displayName("gradle-plugin", lower), Package: lower, Version: versionOf(match[2]), File: relative})
	}

	for _, match := range gradleDependency.FindAllStringSubmatch(content, -1) {
		configuration := strings.ToLower(match[1])

		if !strings.Contains(configuration, "implementation") && !strings.Contains(configuration, "api") && !strings.Contains(configuration, "compile") && !strings.Contains(configuration, "runtime") {
			continue
		}

		coordinates := strings.ToLower(match[2] + ":" + match[3])
		terms = append(terms, Term{Name: displayName("maven", coordinates), Package: coordinates, Version: versionOf(match[4]), File: relative, Dev: strings.HasPrefix(configuration, "test")})
	}

	return terms
}

var (
	dotnetFramework = regexp.MustCompile(`<TargetFrameworks?>\s*net(?:coreapp)?(\d+(?:\.\d+)?)`)
	dotnetPackage   = regexp.MustCompile(`<PackageReference\s+Include="([^"]+)"(?:\s+Version="([^"]+)")?`)
)

// csproj reads a .NET project file: the target framework and the NuGet packages.
func csproj(path, relative string) []Term {
	content := readFile(path)
	terms := []Term{{Name: "C#", File: relative}, {Name: ".NET", File: relative}}

	if match := dotnetFramework.FindStringSubmatch(content); match != nil {
		terms[1].Version = match[1]
	}

	for _, match := range dotnetPackage.FindAllStringSubmatch(content, -1) {
		lower := strings.ToLower(match[1])
		terms = append(terms, Term{Name: displayName("nuget", lower), Package: lower, Version: versionOf(match[2]), File: relative, Dev: strings.Contains(lower, "test") || strings.Contains(lower, "xunit") || strings.Contains(lower, "nunit")})
	}

	return terms
}

// cargo reads Cargo.toml: the dependencies, the dev-dependencies.
func cargo(path, relative string) []Term {
	sections := tomlSections(readFile(path))
	terms := []Term{{Name: "Rust", File: relative}, {Name: "Cargo", File: relative, Dev: true}}

	cargoSections := map[string]bool{"dependencies": false, "dev-dependencies": true, "build-dependencies": true}

	for _, section := range sortedKeys(cargoSections) {
		dev := cargoSections[section]

		for _, match := range tomlKeyValue.FindAllStringSubmatch(sections[section], -1) {
			lower := strings.ToLower(match[1])
			terms = append(terms, Term{Name: displayName("cargo", lower), Package: lower, Version: versionOf(match[2]), File: relative, Dev: dev})
		}
	}

	return terms
}

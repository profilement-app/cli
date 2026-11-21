package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// write puts files into a temporary project folder (path → content).
func write(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()

	for path, content := range files {
		full := filepath.Join(root, path)

		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

// byName is the found terms by name.
func byName(t *testing.T, root string) map[string]Term {
	t.Helper()
	terms, err := Folder(root)

	if err != nil {
		t.Fatal(err)
	}

	found := map[string]Term{}

	for _, term := range terms {
		found[term.Name] = term
	}

	return found
}

func expect(t *testing.T, found map[string]Term, name, version string, dev bool) {
	t.Helper()
	term, ok := found[name]

	if !ok {
		t.Fatalf("%s not found in %v", name, found)
	}

	if term.Version != version || term.Dev != dev {
		t.Errorf("%s: got version %q dev %v, want %q %v", name, term.Version, term.Dev, version, dev)
	}
}

func TestComposerWithItsLockedVersions(t *testing.T) {
	found := byName(t, write(t, map[string]string{
		"composer.json": `{"require": {"php": "^8.3", "ext-json": "*", "laravel/framework": "^13.0", "guzzlehttp/guzzle": "^7.8"},
			"require-dev": {"phpunit/phpunit": "^11.0"}}`,
		"composer.lock": `{"packages": [{"name": "laravel/framework", "version": "v13.35.1"}], "packages-dev": [{"name": "phpunit/phpunit", "version": "11.5.2"}]}`,
	}))

	expect(t, found, "PHP", "8.3", false)
	// The lock's exact version, the manifest's when the lock has none.
	expect(t, found, "Laravel", "13.35.1", false)
	expect(t, found, "Guzzle", "7.8", false)
	expect(t, found, "PHPUnit", "11.5.2", true)
	expect(t, found, "Composer", "", true)

	if _, ok := found["ext-json"]; ok {
		t.Error("a PHP extension is no term of its own")
	}

	if found["Laravel"].Package != "laravel/framework" || found["Laravel"].File != "composer.json" {
		t.Errorf("Laravel: %+v", found["Laravel"])
	}
}

func TestNpmWithPackageLockAndEngines(t *testing.T) {
	found := byName(t, write(t, map[string]string{
		"package.json": `{"dependencies": {"react": "^18.2.0", "@angular/core": "~17.0.0"}, "devDependencies": {"vite": "^5.0.0", "@vue/compiler-sfc": "3"},
			"engines": {"node": ">=20"}}`,
		"package-lock.json": `{"packages": {"": {}, "node_modules/react": {"version": "18.3.1"}, "node_modules/vite/node_modules/esbuild": {"version": "0.1"}}}`,
	}))

	expect(t, found, "React", "18.3.1", false)
	expect(t, found, "Angular", "17.0.0", false)
	expect(t, found, "Vite", "5.0.0", true)
	expect(t, found, "vue", "3", true)
	expect(t, found, "Node.js", "20", false)
	expect(t, found, "npm", "", true)
}

func TestDockerComposeImagesAndToolFiles(t *testing.T) {
	found := byName(t, write(t, map[string]string{
		"Dockerfile":                  "FROM --platform=linux/amd64 php:8.3-fpm-alpine AS base\nFROM node:20 AS assets\nFROM ${BASE}",
		"docker-compose.yml":          "services:\n  db:\n    image: mysql:8.4\n  cache:\n    image: 'docker.io/library/redis:7.2-alpine'\n  search:\n    image: opensearchproject/opensearch:2.11.0\n",
		".github/workflows/check.yml": "on: push",
		"phpstan.neon.dist":           "",
		"tailwind.config.js":          "",
		".git/HEAD":                   "ref: refs/heads/master",
		"node_modules/x/package.json": `{"dependencies": {"left-pad": "1"}}`,
	}))

	expect(t, found, "PHP", "8.3", false)
	expect(t, found, "Node.js", "20", false)
	expect(t, found, "MySQL", "8.4", false)
	expect(t, found, "Redis", "7.2", false)
	expect(t, found, "OpenSearch", "2.11.0", false)
	expect(t, found, "Docker", "", false)
	expect(t, found, "Docker Compose", "", false)
	expect(t, found, "GitHub Actions", "", true)
	expect(t, found, "PHPStan", "", true)
	expect(t, found, "Tailwind CSS", "", false)
	expect(t, found, "Git", "", true)

	// Dependencies' own folders are not searched.
	if _, ok := found["left-pad"]; ok {
		t.Error("node_modules was searched")
	}
}

func TestOtherEcosystems(t *testing.T) {
	found := byName(t, write(t, map[string]string{
		"go.mod":           "module example.com/x\n\ngo 1.22\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.1\n\tgolang.org/x/text v0.14.0 // indirect\n)\n",
		"requirements.txt": "# web\nDjango==5.0.1\nrequests>=2.31\n-r other.txt\n",
		"pyproject.toml":   "[project]\nrequires-python = \">=3.11\"\ndependencies = [\"fastapi>=0.110\", \"pydantic\"]\n",
		"Gemfile":          "ruby '3.3.0'\ngem 'rails', '~> 7.1'\n",
		"Gemfile.lock":     "GEM\n  specs:\n    rails (7.1.3)\n",
		"pom.xml":          "<project><parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId><version>3.2.1</version></parent><properties><java.version>21</java.version></properties><dependencies><dependency><groupId>org.junit.jupiter</groupId><artifactId>junit-jupiter</artifactId><scope>test</scope></dependency></dependencies></project>",
		"api/api.csproj":   `<Project><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup><ItemGroup><PackageReference Include="Microsoft.EntityFrameworkCore" Version="8.0.1" /></ItemGroup></Project>`,
		"Cargo.toml":       "[package]\nname = \"x\"\n\n[dependencies]\ntokio = { version = \"1.35\", features = [\"full\"] }\n\n[dev-dependencies]\nmockall = \"0.12\"\n",
	}))

	expect(t, found, "Go", "1.22", false)
	expect(t, found, "Gin", "1.9.1", false)
	expect(t, found, "Django", "5.0.1", false)
	expect(t, found, "FastAPI", "0.110", false)
	expect(t, found, "Python", "3.11", false)
	expect(t, found, "Ruby on Rails", "7.1.3", false)
	expect(t, found, "Ruby", "3.3.0", false)
	expect(t, found, "Spring Boot", "3.2.1", false)
	expect(t, found, "Java", "21", false)
	expect(t, found, "JUnit", "", true)
	expect(t, found, ".NET", "8.0", false)
	expect(t, found, "Entity Framework Core", "8.0.1", false)
	expect(t, found, "Tokio", "1.35", false)
	expect(t, found, "mockall", "0.12", true)

	if _, ok := found["text"]; ok {
		t.Error("an indirect Go module was sent")
	}
}

func TestLanguagesNeedSomeFilesAndATermIsDevOnlyWhenEveryFileSaysSo(t *testing.T) {
	found := byName(t, write(t, map[string]string{
		"src/a.php": "", "src/b.php": "", "src/c.php": "", "resources/views/x.blade.php": "", "tools/one.py": "",
		"package.json":      `{"devDependencies": {"typescript": "^5.4.0"}}`,
		"site/package.json": `{"dependencies": {"typescript": "^5.4.0"}}`,
	}))

	expect(t, found, "PHP", "", false)

	if _, ok := found["Python"]; ok {
		t.Error("one Python file makes no language of the project")
	}

	if _, ok := found["Blade"]; ok {
		t.Error("one Blade file makes no language of the project")
	}

	expect(t, found, "TypeScript", "5.4.0", false)
}

func TestFallbackNames(t *testing.T) {
	cases := map[string]string{
		"guzzlehttp/guzzle":       "guzzle",
		"laravel-lang/core":       "laravel-lang",
		"@vue/compiler-sfc":       "vue",
		"@tailwindcss/vite":       "tailwindcss",
		"@angular-devkit/core":    "angular-devkit",
		"github.com/jackc/pgx/v5": "pgx",
		"org.slf4j:slf4j-api":     "slf4j-api",
		"leftpad":                 "leftpad",
	}

	for pkg, want := range cases {
		if got := fallbackName(pkg); got != want {
			t.Errorf("fallbackName(%q) = %q, want %q", pkg, got, want)
		}
	}

	if got := displayName("maven", "org.springframework.boot:spring-boot-starter-web"); got != "Spring Boot" {
		t.Errorf("the longest prefix wins: got %q", got)
	}
}

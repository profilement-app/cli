package scan

import (
	"path/filepath"
	"strings"
)

// toolFiles are files whose name alone says a tool is used (the name in lower case; a "*" matches any rest).
var toolFiles = []struct {
	pattern string
	name    string
	dev     bool
}{
	{"phpstan.neon*", "PHPStan", true}, {"psalm.xml*", "Psalm", true}, {".php-cs-fixer*", "PHP-CS-Fixer", true},
	{"phpcs.xml*", "PHP_CodeSniffer", true}, {"phpunit.xml*", "PHPUnit", true}, {"rector.php", "Rector", true},
	{"pint.json", "Laravel Pint", true}, {"artisan", "Laravel", false}, {"symfony.lock", "Symfony", false},
	{"tsconfig.json", "TypeScript", false}, {"vite.config.*", "Vite", true}, {"webpack.config.*", "webpack", true},
	{"rollup.config.*", "Rollup", true}, {"tailwind.config.*", "Tailwind CSS", false}, {"postcss.config.*", "PostCSS", true},
	{".eslintrc*", "ESLint", true}, {"eslint.config.*", "ESLint", true}, {".prettierrc*", "Prettier", true},
	{"prettier.config.*", "Prettier", true}, {".stylelintrc*", "Stylelint", true}, {"jest.config.*", "Jest", true},
	{"vitest.config.*", "Vitest", true}, {"playwright.config.*", "Playwright", true}, {"cypress.config.*", "Cypress", true},
	{"babel.config.*", "Babel", true}, {".babelrc", "Babel", true}, {"angular.json", "Angular", false},
	{"next.config.*", "Next.js", false}, {"nuxt.config.*", "Nuxt", false}, {"svelte.config.*", "Svelte", false},
	{"astro.config.*", "Astro", false}, {".gitlab-ci.yml", "GitLab CI/CD", true}, {"jenkinsfile", "Jenkins", true},
	{"bitbucket-pipelines.yml", "Bitbucket Pipelines", true}, {"azure-pipelines.yml", "Azure Pipelines", true},
	{".travis.yml", "Travis CI", true}, {"makefile", "Make", true}, {"vagrantfile", "Vagrant", true},
	{"ansible.cfg", "Ansible", true}, {"chart.yaml", "Helm", true}, {"skaffold.yaml", "Skaffold", true},
	{"kustomization.yaml", "Kustomize", true}, {"serverless.yml", "Serverless Framework", true},
	{"netlify.toml", "Netlify", false}, {"vercel.json", "Vercel", false}, {"sonar-project.properties", "SonarQube", true},
	{".editorconfig", "EditorConfig", true}, {"renovate.json", "Renovate", true}, {"behat.yml*", "Behat", true},
	{"codeception.yml", "Codeception", true}, {"deptrac.yaml", "Deptrac", true}, {"captainhook.json", "CaptainHook", true},
	{"grumphp.yml", "GrumPHP", true}, {"deploy.php", "Deployer", true}, {"wp-config.php", "WordPress", false},
	{"pubspec.yaml", "Flutter", false}, {"podfile", "CocoaPods", true}, {"package.swift", "Swift", false},
	{"terraform.tf", "Terraform", true}, {"main.tf", "Terraform", true},
}

// toolFolders are folders whose name says a tool is used.
var toolFolders = map[string]struct {
	name string
	dev  bool
}{
	".git": {"Git", true}, ".ddev": {"DDEV", true}, ".lando": {"Lando", true}, ".husky": {"Husky", true},
	".storybook": {"Storybook", true}, ".terraform": {"Terraform", true}, ".devcontainer": {"Dev Containers", true},
	".circleci": {"CircleCI", true}, ".claude": {"Claude Code", true},
}

// toolsByFile are the tools a file's name says are used.
func toolsByFile(relative, name string) []Term {
	lower := strings.ToLower(name)

	// GitHub Actions: a workflow below .github/workflows.
	if strings.HasPrefix(filepath.ToSlash(relative), ".github/workflows/") && (strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")) {
		return []Term{{Name: "GitHub Actions", File: relative, Dev: true}}
	}

	for _, tool := range toolFiles {
		if matched, _ := filepath.Match(tool.pattern, lower); matched {
			return []Term{{Name: tool.name, File: relative, Dev: tool.dev}}
		}
	}

	return nil
}

// byFolder are the tools a folder's name says are used (the folder itself is not descended into for .git).
func byFolder(_, relative, name string) []Term {
	if tool, ok := toolFolders[name]; ok {
		return []Term{{Name: tool.name, File: relative, Dev: tool.dev}}
	}

	return nil
}

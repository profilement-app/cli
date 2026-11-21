package scan

import (
	"sort"
	"strings"
)

// languageFiles are the endings a language's files have (the longest ending is tried first: ".blade.php").
var languageFiles = map[string]string{
	".php": "PHP", ".blade.php": "Blade", ".twig": "Twig", ".js": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
	".jsx": "React", ".ts": "TypeScript", ".tsx": "TypeScript", ".vue": "Vue.js", ".svelte": "Svelte", ".py": "Python",
	".go": "Go", ".rb": "Ruby", ".java": "Java", ".kt": "Kotlin", ".kts": "Kotlin", ".scala": "Scala", ".cs": "C#",
	".fs": "F#", ".vb": "Visual Basic .NET", ".rs": "Rust", ".swift": "Swift", ".m": "Objective-C", ".dart": "Dart",
	".c": "C", ".h": "C", ".cpp": "C++", ".cc": "C++", ".hpp": "C++", ".scss": "Sass", ".sass": "Sass", ".less": "Less",
	".css": "CSS", ".html": "HTML", ".sql": "SQL", ".sh": "Bash", ".ps1": "PowerShell", ".lua": "Lua", ".pl": "Perl",
	".r": "R", ".ex": "Elixir", ".exs": "Elixir", ".erl": "Erlang", ".hs": "Haskell", ".clj": "Clojure", ".groovy": "Groovy",
	".graphql": "GraphQL", ".gql": "GraphQL", ".proto": "Protocol Buffers", ".tf": "Terraform", ".abap": "ABAP",
	".pas": "Delphi", ".dpr": "Delphi", ".dfm": "Delphi", ".xsl": "XSLT", ".xslt": "XSLT", ".md": "",
}

// minFiles is how many files of a language make it count (a stray script is no language of the project).
const minFiles = 3

// languageKey is the ending of a file's name that names its language, "" when none does.
func languageKey(name string) string {
	lower := strings.ToLower(name)

	if strings.HasSuffix(lower, ".blade.php") {
		return ".blade.php"
	}

	if index := strings.LastIndex(lower, "."); index > 0 {
		if _, ok := languageFiles[lower[index:]]; ok {
			return lower[index:]
		}
	}

	return ""
}

// languages are the languages of which the project has at least minFiles files.
func languages(counts map[string]int) []Term {
	byLanguage := map[string]int{}

	for ending, count := range counts {
		if language := languageFiles[ending]; language != "" {
			byLanguage[language] += count
		}
	}

	var terms []Term

	for language, count := range byLanguage {
		if count >= minFiles {
			terms = append(terms, Term{Name: language, File: "*" + endingsOf(language)})
		}
	}

	sort.Slice(terms, func(i, j int) bool { return terms[i].Name < terms[j].Name })

	return terms
}

// endingsOf is the first ending of a language, for the term's file ("*.php").
func endingsOf(language string) string {
	var endings []string

	for ending, name := range languageFiles {
		if name == language {
			endings = append(endings, ending)
		}
	}

	sort.Strings(endings)

	if len(endings) == 0 {
		return ""
	}

	return endings[0]
}

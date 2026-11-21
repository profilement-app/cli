// Package gitlog reads the signed-in person's own commits of a project's Git history and groups them into topics —
// a ticket ("SHOP-123"), an issue ("#42"), a conventional commit's scope ("feat(checkout): …"), else each commit's
// subject on its own (the same subject once) — that may become tasks. Noise ("wip", "typo", merges, version bumps)
// is left out. Nothing leaves the computer here: the person chooses which topics are sent.
package gitlog

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Commit is one commit of the history.
type Commit struct {
	Hash    string
	Date    string // YYYY-MM-DD
	Subject string
}

// Topic is a group of commits that may become a task.
type Topic struct {
	Title   string
	Key     string
	Kind    string // ticket, scope, commit
	Start   string
	End     string
	Commits []Commit
	// Suggested is true for a ticket's or a scope's topic (a single commit's is offered, not suggested).
	Suggested bool
}

// IsRepository says whether the folder is inside a Git working tree.
func IsRepository(dir string) bool {
	output, err := run(dir, "rev-parse", "--is-inside-work-tree")

	return err == nil && strings.TrimSpace(output) == "true"
}

// UserEmail is the e-mail address Git commits as in the folder.
func UserEmail(dir string) string {
	output, _ := run(dir, "config", "user.email")

	return strings.TrimSpace(output)
}

// Remote is the address of the folder's remote "origin" without credentials, empty when there is none.
func Remote(dir string) string {
	output, err := run(dir, "remote", "get-url", "origin")

	if err != nil {
		return ""
	}

	remote := strings.TrimSpace(output)

	// https://user:token@host/path → https://host/path
	if at := strings.LastIndex(remote, "@"); at > 0 && strings.Contains(remote[:at], "://") {
		remote = remote[:strings.Index(remote, "://")+3] + remote[at+1:]
	}

	return remote
}

// Branch is the folder's current branch, empty when detached.
func Branch(dir string) string {
	output, _ := run(dir, "rev-parse", "--abbrev-ref", "HEAD")

	if branch := strings.TrimSpace(output); branch != "HEAD" {
		return branch
	}

	return ""
}

// Commits are the author's commits on every branch, without merges, newest first.
func Commits(dir, author string) ([]Commit, error) {
	if strings.TrimSpace(author) == "" {
		return nil, fmt.Errorf("no author to read the commits of (git config user.email)")
	}

	output, err := run(dir, "log", "--all", "--no-merges", "--author="+author, "--date=short", "--pretty=format:%h%x1f%ad%x1f%s")

	if err != nil {
		return nil, err
	}

	var commits []Commit

	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(line, "\x1f", 3)

		if len(fields) == 3 {
			commits = append(commits, Commit{Hash: fields[0], Date: fields[1], Subject: strings.TrimSpace(fields[2])})
		}
	}

	return commits, nil
}

func run(dir string, arguments ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, arguments...)...)
	var output, errors bytes.Buffer
	command.Stdout, command.Stderr = &output, &errors

	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", arguments[0], strings.TrimSpace(errors.String()))
	}

	return output.String(), nil
}

var (
	ticketPattern       = regexp.MustCompile(`\b([A-Z][A-Z0-9]{1,9}-\d+)\b`)
	issuePattern        = regexp.MustCompile(`(?:^|[\s(\[])#(\d+)\b`)
	conventionalPattern = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?!?:\s*(.+)$`)
	noisePattern        = regexp.MustCompile(`(?i)^(?:wip\b|fixup!|squash!|merge\b|revert\b|typo|fix typo|bump\b|update (?:deps|dependencies|composer\.lock|package-lock)|chore\b|\.+$|-+$|test$|tmp$)`)
)

// Topics groups commits: by ticket, else issue, else conventional scope, else each subject on its own (noise left
// out); newest topic first.
func Topics(commits []Commit) []Topic {
	byKey := map[string]*Topic{}
	var order []string

	for _, commit := range commits {
		key, kind := topicKey(commit.Subject)

		if kind == "commit" {
			// A commit that says nothing is no task.
			if noisePattern.MatchString(strings.TrimSpace(commit.Subject)) || len([]rune(cleanSubject(commit.Subject))) < 4 {
				continue
			}

			key = "commit:" + strings.ToLower(cleanSubject(commit.Subject))
		}

		topic, ok := byKey[key]

		if !ok {
			topic = &Topic{Key: key, Kind: kind, Suggested: kind != "commit"}
			byKey[key] = topic
			order = append(order, key)
		}

		topic.Commits = append(topic.Commits, commit)
		topic.Start, topic.End = minDate(topic.Start, commit.Date), maxDate(topic.End, commit.Date)
	}

	topics := make([]Topic, 0, len(order))

	for _, key := range order {
		topic := byKey[key]
		topic.Title = title(*topic)
		topics = append(topics, *topic)
	}

	sort.SliceStable(topics, func(i, j int) bool { return topics[i].End > topics[j].End })

	return topics
}

// topicKey is the key and kind a commit's subject groups it by.
func topicKey(subject string) (string, string) {
	if match := ticketPattern.FindStringSubmatch(subject); match != nil {
		return match[1], "ticket"
	}

	if match := issuePattern.FindStringSubmatch(subject); match != nil {
		return "#" + match[1], "ticket"
	}

	if match := conventionalPattern.FindStringSubmatch(subject); match != nil && match[2] != "" {
		return "scope:" + strings.ToLower(match[2]), "scope"
	}

	return "", "commit"
}

// title names a topic: the ticket and what its earliest commit says, a scope and its earliest commit, a commit's
// subject.
func title(topic Topic) string {
	if topic.Kind == "commit" {
		return cleanSubject(topic.Commits[len(topic.Commits)-1].Subject)
	}

	// The earliest commit that says more than noise (commits are newest first).
	text := ""

	for index := len(topic.Commits) - 1; index >= 0; index-- {
		if cleaned := cleanSubject(topic.Commits[index].Subject); cleaned != "" && !noisePattern.MatchString(topic.Commits[index].Subject) {
			text = cleaned

			break
		}
	}

	label := topic.Key

	if topic.Kind == "scope" {
		label = capitalize(strings.TrimPrefix(topic.Key, "scope:"))
	}

	if text == "" {
		return label
	}

	return label + ": " + text
}

// cleanSubject drops the ticket, the issue and a conventional commit's prefix from a subject.
func cleanSubject(subject string) string {
	subject = ticketPattern.ReplaceAllString(subject, "")
	subject = issuePattern.ReplaceAllString(subject, " ")

	if match := conventionalPattern.FindStringSubmatch(strings.TrimSpace(subject)); match != nil {
		subject = match[3]
	}

	subject = strings.Trim(strings.TrimSpace(subject), ":-–[]() ")

	return capitalize(subject)
}

func capitalize(text string) string {
	first, size := utf8.DecodeRuneInString(text)

	if first == utf8.RuneError {
		return text
	}

	return string(unicode.ToUpper(first)) + text[size:]
}

func minDate(current, date string) string {
	if current == "" || date < current {
		return date
	}

	return current
}

func maxDate(current, date string) string {
	if date > current {
		return date
	}

	return current
}

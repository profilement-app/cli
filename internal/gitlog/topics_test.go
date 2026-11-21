package gitlog

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommitsAreGroupedByTicketIssueScopeElseBySubject(t *testing.T) {
	// Newest first, as git log gives them.
	commits := []Commit{
		{"a1", "2026-11-09", "SHOP-12: fix totals in the cart"},
		{"a2", "2026-11-08", "feat(search): facets for brands (#42)"},
		{"a3", "2026-11-05", "update admin grids"},
		{"a4", "2026-11-03", "fix(search): typo"},
		{"a5", "2026-10-20", "WIP SHOP-12"},
		{"a6", "2026-10-15", "[SHOP-12] Rework checkout"},
		{"a7", "2026-10-10", "Update admin grids"},
		{"a8", "2026-10-02", "chore: bump deps"},
		{"a9", "2026-10-01", "wip"},
	}

	topics := Topics(commits)
	byKey := map[string]Topic{}

	for _, topic := range topics {
		byKey[topic.Key] = topic
	}

	ticket := byKey["SHOP-12"]

	if len(ticket.Commits) != 3 || ticket.Start != "2026-10-15" || ticket.End != "2026-11-09" || !ticket.Suggested {
		t.Errorf("ticket: %+v", ticket)
	}

	// Named after its earliest commit that says more than noise.
	if ticket.Title != "SHOP-12: Rework checkout" {
		t.Errorf("ticket title %q", ticket.Title)
	}

	// An issue reference comes before the scope.
	if issue := byKey["#42"]; issue.Title != "#42: Facets for brands" || issue.Kind != "ticket" {
		t.Errorf("issue: %+v", issue)
	}

	if scope := byKey["scope:search"]; scope.Title != "Search: Typo" || len(scope.Commits) != 1 {
		t.Errorf("scope: %+v", scope)
	}

	// The same subject once, offered but not suggested; noise is no topic.
	grids := byKey["commit:update admin grids"]

	if grids.Title != "Update admin grids" || grids.Suggested || len(grids.Commits) != 2 || grids.Start != "2026-10-10" {
		t.Errorf("commit topic: %+v", grids)
	}

	if len(topics) != 4 {
		t.Errorf("%d topics, want ticket, issue, scope and one subject: %+v", len(topics), topics)
	}

	// Newest topic first.
	if topics[0].Key != "SHOP-12" {
		t.Errorf("first topic %q", topics[0].Key)
	}
}

func TestCommitsOfOneAuthorFromARealRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	dir := t.TempDir()
	git := func(env []string, arguments ...string) {
		command := exec.Command("git", append([]string{"-C", dir}, arguments...)...)
		command.Env = append(os.Environ(), env...)

		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", arguments, err, output)
		}
	}
	as := func(email string) []string {
		return []string{"GIT_AUTHOR_NAME=x", "GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_NAME=x", "GIT_COMMITTER_EMAIL=" + email, "GIT_AUTHOR_DATE=2026-10-01T10:00:00", "GIT_COMMITTER_DATE=2026-10-01T10:00:00"}
	}

	git(nil, "init", "-q", "-b", "main")
	git(nil, "config", "user.email", "me@example.com")
	git(nil, "remote", "add", "origin", "https://user:secret@example.com/acme/shop.git")

	for index, author := range []string{"me@example.com", "other@example.com", "me@example.com"} {
		if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte{byte(index)}, 0o644); err != nil {
			t.Fatal(err)
		}

		git(nil, "add", "file.txt")
		git(as(author), "commit", "-q", "-m", "ABC-1 change "+author)
	}

	if !IsRepository(dir) || UserEmail(dir) != "me@example.com" || Branch(dir) != "main" {
		t.Fatalf("repository: %v %q %q", IsRepository(dir), UserEmail(dir), Branch(dir))
	}

	// The remote without its credentials.
	if remote := Remote(dir); remote != "https://example.com/acme/shop.git" {
		t.Errorf("remote %q", remote)
	}

	commits, err := Commits(dir, UserEmail(dir))

	if err != nil || len(commits) != 2 || commits[0].Date != "2026-10-01" {
		t.Fatalf("commits %+v %v", commits, err)
	}
}

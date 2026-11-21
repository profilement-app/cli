package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/profilement-app/cli/internal/api"
	"github.com/profilement-app/cli/internal/config"
)

const token = "7|pfm_secret"

// profilement is a fake Profilement: it knows one token, two projects, and keeps the proposals it got.
type profilement struct {
	server    *httptest.Server
	proposals []api.Proposal
	requests  []string
}

func newProfilement(t *testing.T) *profilement {
	t.Helper()
	fake := &profilement{}
	fake.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fake.requests = append(fake.requests, request.Method+" "+request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")

		if request.Header.Get("Authorization") != "Bearer "+token {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"message": "Unauthenticated."}`))

			return
		}

		switch request.URL.Path {
		case "/api/v1/me":
			_, _ = writer.Write([]byte(`{"code": "anna", "name": "Anna Example", "company": {"code": "acme", "name": "Acme"}}`))
		case "/api/v1/projects":
			_, _ = writer.Write([]byte(`{"projects": [
				{"code": "portal", "name": "Portal", "customer": {"code": "acme", "name": "Acme"}, "starts_on": "2026-01-01", "ends_on": null},
				{"code": "shop", "name": "Shop", "customer": {"code": "globex", "name": "Globex"}, "starts_on": null, "ends_on": null}]}`))
		case "/api/v1/project-imports":
			var proposal api.Proposal
			body, _ := io.ReadAll(request.Body)

			if json.Unmarshal(body, &proposal) != nil {
				writer.WriteHeader(http.StatusUnprocessableEntity)

				return
			}

			fake.proposals = append(fake.proposals, proposal)
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`{"code": "x", "review_url": "` + fake.server.URL + `/account/project-imports/x", "terms": 1, "tasks": 0}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(fake.server.Close)

	return fake
}

// isolated gives the test a configuration file of its own and no settings from the environment.
func isolated(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("PROFILEMENT_CONFIG", path)
	t.Setenv("PROFILEMENT_URL", "")
	t.Setenv("PROFILEMENT_TOKEN", "")
	t.Setenv("PROFILEMENT_INSECURE_TLS", "")

	return path
}

// signedInTo stores the server and token as login would.
func signedInTo(t *testing.T, fake *profilement) {
	t.Helper()
	settings, _ := config.Load()
	settings.Server, settings.Token = fake.server.URL, token

	if err := settings.Save(); err != nil {
		t.Fatal(err)
	}
}

// project is a project folder with a composer.json naming Laravel.
func project(t *testing.T) string {
	t.Helper()
	folder := filepath.Join(t.TempDir(), "shop-backend")

	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(folder, "composer.json"), []byte(`{"require": {"php": "^8.3", "laravel/framework": "^13.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	return folder
}

// execute runs the command with the answers as input; its output and error.
func execute(answers string, arguments ...string) (string, error) {
	var out bytes.Buffer
	err := run(arguments, strings.NewReader(answers), &out)

	return out.String(), err
}

func TestLoginChecksTheTokenBeforeKeepingIt(t *testing.T) {
	path := isolated(t)
	fake := newProfilement(t)

	// A refused token is not kept.
	if _, err := execute("1|wrong\n", "login", "--server", fake.server.URL, "--token-stdin"); err == nil || !strings.Contains(err.Error(), "profilement login") {
		t.Fatalf("a wrong token: %v", err)
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("a refused token was kept")
	}

	// Asked for in the terminal: the server, then the token.
	out, err := execute(fake.server.URL+"\n"+token+"\n", "login")

	if err != nil || !strings.Contains(out, "Signed in as Anna Example (Acme)") {
		t.Fatalf("login: %q, %v", out, err)
	}

	settings, _ := config.Load()

	if settings.Token != token || settings.Server != fake.server.URL {
		t.Errorf("kept %+v", settings)
	}

	if out, err = execute("", "whoami"); err != nil || !strings.Contains(out, "Anna Example (Acme) on "+fake.server.URL) {
		t.Errorf("whoami: %q, %v", out, err)
	}

	// Logout forgets it; nothing works without it.
	if _, err = execute("", "logout"); err != nil {
		t.Fatal(err)
	}

	if _, err = execute("", "whoami"); err == nil || !strings.Contains(err.Error(), "not signed in") {
		t.Errorf("after logout: %v", err)
	}
}

func TestLoginRefusesAServerWithoutHttps(t *testing.T) {
	isolated(t)

	if _, err := execute(token, "login", "--server", "http://www.profilement.com", "--token-stdin"); err == nil || !strings.Contains(err.Error(), "only https") {
		t.Errorf("http server: %v", err)
	}
}

func TestAddSendsNothingUntilConfirmedThenTheFoldersTerms(t *testing.T) {
	isolated(t)
	fake := newProfilement(t)
	signedInTo(t, fake)
	folder := project(t)

	// The name asked (the folder's by default), then "no".
	out, err := execute("\nn\n", "add", folder)

	if err != nil || !strings.Contains(out, "Name of the new project [shop-backend]") || !strings.Contains(out, "Nothing was sent.") || len(fake.proposals) != 0 {
		t.Fatalf("declined: %q, %v, %d sent", out, err, len(fake.proposals))
	}

	// Another name, then the default "yes".
	out, err = execute("Shop backend\n\n", "add", folder)

	if err != nil || len(fake.proposals) != 1 {
		t.Fatalf("add: %q, %v", out, err)
	}

	proposal := fake.proposals[0]

	if proposal.Name != "Shop backend" || proposal.Project != "" || proposal.Source["tool"] != "profilement-cli" || proposal.Source["folder"] != "shop-backend" {
		t.Errorf("proposal %+v", proposal)
	}

	names := map[string]string{}

	for _, term := range proposal.Terms {
		names[term.Name] = term.Version
	}

	if names["Laravel"] != "13.0" || names["PHP"] != "8.3" {
		t.Errorf("terms %+v", proposal.Terms)
	}

	if !strings.Contains(out, "Check and apply it here") || !strings.Contains(out, "/account/project-imports/x") {
		t.Errorf("no review address: %q", out)
	}

	// --name and --yes ask nothing (the flags also after the folder).
	if _, err = execute("", "add", folder, "--name", "Quiet", "--yes"); err != nil || fake.proposals[1].Name != "Quiet" {
		t.Errorf("--yes: %v %+v", err, fake.proposals)
	}
}

func TestUpdateChoosesAProjectAndRemembersItForTheFolder(t *testing.T) {
	isolated(t)
	fake := newProfilement(t)
	signedInTo(t, fake)
	folder := project(t)

	// --yes needs a project the first time.
	if _, err := execute("", "update", folder, "--yes"); err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("--yes without a project: %v", err)
	}

	// A wrong choice is asked again; the second project is chosen.
	out, err := execute("7\n2\n\n", "update", folder)

	if err != nil || !strings.Contains(out, "Globex · Shop") || !strings.Contains(out, "Choose one number") || len(fake.proposals) != 1 {
		t.Fatalf("update: %q, %v", out, err)
	}

	if fake.proposals[0].Project != "shop" || fake.proposals[0].Name != "Shop" {
		t.Errorf("proposal %+v", fake.proposals[0])
	}

	// Remembered for the folder: offered first next time, enough for --yes.
	settings, _ := config.Load()

	if settings.Projects[folder] != "shop" {
		t.Errorf("remembered %+v", settings.Projects)
	}

	if out, err = execute("\n\n", "update", folder); err != nil || !strings.Contains(out, "Which project [2]") || fake.proposals[1].Project != "shop" {
		t.Errorf("remembered project offered: %q, %v", out, err)
	}

	if _, err = execute("", "update", folder, "--yes"); err != nil || fake.proposals[2].Project != "shop" {
		t.Errorf("--yes with the remembered project: %v", err)
	}

	// A project the account may not change.
	if _, err = execute("", "update", folder, "--project", "nobodys", "--yes"); err == nil || !strings.Contains(err.Error(), "nobodys") {
		t.Errorf("unknown project: %v", err)
	}
}

func TestTheChosenGitTopicsBecomeTasks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	isolated(t)
	fake := newProfilement(t)
	signedInTo(t, fake)
	folder := project(t)
	git := func(arguments ...string) {
		command := exec.Command("git", append([]string{"-C", folder}, arguments...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-10-01T10:00:00", "GIT_COMMITTER_DATE=2026-10-01T10:00:00")

		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", arguments, err, output)
		}
	}

	git("init", "-q", "-b", "main")
	git("config", "user.email", "anna@example.com")
	git("config", "user.name", "Anna")
	git("add", "composer.json")
	git("commit", "-q", "-m", "SHOP-12: rework checkout")
	git("commit", "-q", "--allow-empty", "-m", "update admin grids")

	// Two topics: the ticket (suggested, marked *) and a commit's subject; "all" chooses both.
	out, err := execute("Shop\nall\n\n", "add", folder)

	if err != nil || len(fake.proposals) != 1 {
		t.Fatalf("add: %q, %v", out, err)
	}

	if !strings.Contains(out, "*") || !strings.Contains(out, "SHOP-12: Rework checkout") || !strings.Contains(out, "(1 commit, 2026-10-01)") {
		t.Errorf("topics listed: %q", out)
	}

	tasks := fake.proposals[0].Tasks

	if len(tasks) != 2 || tasks[0].StartedOn != "2026-10-01" || tasks[0].Commits != 1 {
		t.Errorf("tasks %+v", tasks)
	}

	if fake.proposals[0].Source["branch"] != "main" {
		t.Errorf("source %+v", fake.proposals[0].Source)
	}

	// The default answer takes only the suggested topics; --no-git none.
	if _, err = execute("Shop\n\n\n", "add", folder); err != nil || len(fake.proposals[1].Tasks) != 1 || fake.proposals[1].Tasks[0].Title != "SHOP-12: Rework checkout" {
		t.Errorf("suggested only: %v %+v", err, fake.proposals[1].Tasks)
	}

	if _, err = execute("", "add", folder, "--no-git", "--yes"); err != nil || len(fake.proposals[2].Tasks) != 0 {
		t.Errorf("--no-git: %v %+v", err, fake.proposals[2].Tasks)
	}
}

func TestScanSendsNothingAndPrintsJson(t *testing.T) {
	isolated(t)
	fake := newProfilement(t)
	signedInTo(t, fake)
	folder := project(t)

	out, err := execute("", "scan", folder, "--json")

	if err != nil {
		t.Fatal(err)
	}

	var terms []api.Term

	if json.Unmarshal([]byte(out), &terms) != nil || len(terms) < 3 {
		t.Fatalf("json: %q", out)
	}

	if out, err = execute("", "scan", folder); err != nil || !strings.Contains(out, "Laravel 13.0") {
		t.Errorf("scan: %q, %v", out, err)
	}

	if len(fake.requests) != 0 {
		t.Errorf("scan talked to the server: %v", fake.requests)
	}
}

func TestHelpVersionAndMistakes(t *testing.T) {
	isolated(t)

	if out, err := execute("", "help"); err != nil || !strings.Contains(out, "profilement login") {
		t.Errorf("help: %q, %v", out, err)
	}

	if out, _ := execute("", "version"); !strings.Contains(out, "profilement dev") {
		t.Errorf("version: %q", out)
	}

	if _, err := execute("", "upload"); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("unknown command: %v", err)
	}

	if _, err := execute("", "add", "/no/such/folder"); err == nil || !strings.Contains(err.Error(), "is no folder") {
		t.Errorf("no folder: %v", err)
	}

	if _, err := execute("", "add", t.TempDir()); err == nil || !strings.Contains(err.Error(), "not signed in") {
		t.Errorf("not signed in: %v", err)
	}
}

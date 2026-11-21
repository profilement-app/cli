// Command profilement sends what a project uses — its tools, frameworks, libraries and the topics of its Git history
// — from the project's folder to Profilement, where its owner checks it before it is added to a project.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/profilement-app/cli/internal/api"
	"github.com/profilement-app/cli/internal/config"
	"github.com/profilement-app/cli/internal/gitlog"
	"github.com/profilement-app/cli/internal/prompt"
	"github.com/profilement-app/cli/internal/scan"
)

// version is set when a release is built (-ldflags "-X main.version=…").
var version = "dev"

// maxTopics is how many of the newest Git topics are offered.
const maxTopics = 100

const usage = `profilement — send what a project uses to Profilement

Usage:
  profilement login [--server URL] [--token-stdin]   connect this computer with your account (a token of the account page)
  profilement logout                                 forget the token
  profilement whoami                                 show the account the token belongs to
  profilement scan [FOLDER] [--json]                 show what would be sent (nothing is sent)
  profilement add [FOLDER] [--name NAME]             propose a new project
  profilement update [FOLDER] [--project CODE]       propose an update of one of your projects
  profilement version

Options of add and update:
  --no-git           send no Git topics
  --author EMAIL     whose commits become topics (default: git config user.email)
  --yes              ask nothing: the suggested topics, no questions (update needs --project once)

Nothing is added to a project before you check it on the review page whose address is printed.
The settings are kept in %s (or $PROFILEMENT_CONFIG); $PROFILEMENT_URL and $PROFILEMENT_TOKEN override them.
`

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(arguments []string, in io.Reader, out io.Writer) error {
	if len(arguments) == 0 || arguments[0] == "help" || arguments[0] == "--help" || arguments[0] == "-h" {
		path, _ := config.Path()
		fmt.Fprintf(out, usage, path)

		return nil
	}

	ask := prompt.New(in, out)
	command, rest := arguments[0], arguments[1:]

	switch command {
	case "version", "--version":
		fmt.Fprintln(out, "profilement", version)

		return nil
	case "login":
		return login(rest, in, ask, out)
	case "logout":
		return logout(out)
	case "whoami":
		return whoami(out)
	case "scan":
		return scanCommand(rest, out)
	case "add", "update":
		return send(command, rest, ask, out)
	}

	return fmt.Errorf("unknown command %q (see \"profilement help\")", command)
}

func client(settings *config.Config) *api.Client {
	return api.New(settings.Server, settings.Token, "profilement-cli/"+version)
}

func login(arguments []string, in io.Reader, ask *prompt.Prompter, out io.Writer) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	server := flags.String("server", "", "the Profilement server")
	fromStdin := flags.Bool("token-stdin", false, "read the token from the standard input")

	if err := flags.Parse(arguments); err != nil {
		return err
	}

	settings, err := config.Load()

	if err != nil {
		return err
	}

	address := *server

	if address == "" && !*fromStdin {
		if address, err = ask.Ask("Profilement server", settings.Server); err != nil {
			return err
		}
	}

	if address == "" {
		address = settings.Server
	}

	if settings.Server, err = config.CheckServer(address); err != nil {
		return err
	}

	var token string

	if *fromStdin {
		content, err := io.ReadAll(io.LimitReader(in, 4096))

		if err != nil {
			return err
		}

		token = strings.TrimSpace(string(content))
	} else {
		fmt.Fprintln(out, "Create a token on your account page in Profilement (card \"Access tokens\") and paste it here.")

		if token, err = ask.Ask("Token", ""); err != nil {
			return err
		}
	}

	if token == "" {
		return errors.New("no token given")
	}

	settings.Token = token
	me, err := client(settings).Me()

	if err != nil {
		return err
	}

	if err := settings.Save(); err != nil {
		return err
	}

	path, _ := config.Path()
	fmt.Fprintf(out, "Signed in as %s (%s). Kept in %s.\n", me.Name, me.Company.Name, path)

	return nil
}

func logout(out io.Writer) error {
	settings, err := config.Load()

	if err != nil {
		return err
	}

	settings.Token = ""

	if err := settings.Save(); err != nil {
		return err
	}

	fmt.Fprintln(out, "The token is forgotten on this computer. Delete it on your account page too, if you no longer use it.")

	return nil
}

func whoami(out io.Writer) error {
	settings, err := signedIn()

	if err != nil {
		return err
	}

	me, err := client(settings).Me()

	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%s (%s) on %s\n", me.Name, me.Company.Name, settings.Server)

	return nil
}

func signedIn() (*config.Config, error) {
	settings, err := config.Load()

	if err != nil {
		return nil, err
	}

	if settings.Token == "" {
		return nil, errors.New("not signed in: run \"profilement login\" first")
	}

	if settings.Server, err = config.CheckServer(settings.Server); err != nil {
		return nil, err
	}

	return settings, nil
}

// folderOf is the absolute folder of the arguments (the current one without).
func folderOf(flags *flag.FlagSet) (string, error) {
	folder := "."

	if flags.NArg() > 0 {
		folder = flags.Arg(0)
	}

	absolute, err := filepath.Abs(folder)

	if err != nil {
		return "", err
	}

	if info, err := os.Stat(absolute); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is no folder", absolute)
	}

	return absolute, nil
}

// parseInterspersed parses flags before and after the folder ("add . --name X" and "add --name X .").
func parseInterspersed(flags *flag.FlagSet, arguments []string) error {
	var positional []string

	for len(arguments) > 0 {
		if err := flags.Parse(arguments); err != nil {
			return err
		}

		arguments = flags.Args()

		if len(arguments) > 0 {
			positional = append(positional, arguments[0])
			arguments = arguments[1:]
		}
	}

	return flags.Parse(positional)
}

func scanCommand(arguments []string, out io.Writer) error {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	asJSON := flags.Bool("json", false, "print the terms as JSON")

	if err := parseInterspersed(flags, arguments); err != nil {
		return err
	}

	folder, err := folderOf(flags)

	if err != nil {
		return err
	}

	terms, err := scan.Folder(folder)

	if err != nil {
		return err
	}

	if *asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")

		return encoder.Encode(terms)
	}

	printTerms(out, terms)

	if gitlog.IsRepository(folder) {
		if commits, err := gitlog.Commits(folder, gitlog.UserEmail(folder)); err == nil {
			fmt.Fprintf(out, "\nGit: %d of your commits in %d topics (add and update offer them as tasks).\n", len(commits), len(gitlog.Topics(commits)))
		}
	}

	return nil
}

func printTerms(out io.Writer, terms []scan.Term) {
	var used, dev []string

	for _, term := range terms {
		text := term.Name

		if term.Version != "" {
			text += " " + term.Version
		}

		if term.Dev {
			dev = append(dev, text)
		} else {
			used = append(used, text)
		}
	}

	fmt.Fprintf(out, "Found %d things this project uses:\n  %s\n", len(used), strings.Join(used, ", "))

	if len(dev) > 0 {
		fmt.Fprintf(out, "and %d for its development only:\n  %s\n", len(dev), strings.Join(dev, ", "))
	}
}

func send(command string, arguments []string, ask *prompt.Prompter, out io.Writer) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	name := flags.String("name", "", "the new project's name (add)")
	projectCode := flags.String("project", "", "the code of the project to update (update)")
	noGit := flags.Bool("no-git", false, "send no Git topics")
	author := flags.String("author", "", "whose commits become topics")
	yes := flags.Bool("yes", false, "ask nothing")

	if err := parseInterspersed(flags, arguments); err != nil {
		return err
	}

	folder, err := folderOf(flags)

	if err != nil {
		return err
	}

	settings, err := signedIn()

	if err != nil {
		return err
	}

	profilement := client(settings)
	me, err := profilement.Me()

	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Signed in as %s (%s).\n\n", me.Name, me.Company.Name)
	proposal := api.Proposal{Name: filepath.Base(folder), Source: map[string]string{"tool": "profilement-cli", "version": version, "folder": filepath.Base(folder)}}

	if command == "update" {
		project, err := chooseProject(profilement, settings, folder, *projectCode, *yes, ask, out)

		if err != nil {
			return err
		}

		proposal.Project, proposal.Name = project.Code, project.Name
		settings.Projects[folder] = project.Code
	} else if *name != "" {
		proposal.Name = *name
	} else if !*yes {
		if proposal.Name, err = ask.Ask("Name of the new project", proposal.Name); err != nil {
			return err
		}
	}

	terms, err := scan.Folder(folder)

	if err != nil {
		return err
	}

	fmt.Fprintln(out)
	printTerms(out, terms)

	for _, term := range terms {
		proposal.Terms = append(proposal.Terms, api.Term(term))
	}

	if !*noGit && gitlog.IsRepository(folder) {
		proposal.Source["remote"], proposal.Source["branch"] = gitlog.Remote(folder), gitlog.Branch(folder)

		if proposal.Tasks, err = chooseTopics(folder, *author, *yes, ask, out); err != nil {
			return err
		}
	}

	if !*yes {
		confirmed, err := ask.Confirm(fmt.Sprintf("\nSend %d terms and %d topics to %s?", len(proposal.Terms), len(proposal.Tasks), settings.Server), true)

		if err != nil || !confirmed {
			fmt.Fprintln(out, "Nothing was sent.")

			return err
		}
	}

	sent, err := profilement.Send(proposal)

	if err != nil {
		return err
	}

	// Remember the project of the folder (an update's; a new project gets its code only when applied).
	if command == "update" {
		if err := settings.Save(); err != nil {
			fmt.Fprintln(out, "(The folder's project could not be remembered:", err.Error()+")")
		}
	}

	fmt.Fprintf(out, "\nSent. Check and apply it here — nothing is added before:\n  %s\n", sent.ReviewURL)

	return nil
}

// chooseProject is the project to update: --project, else the folder's remembered one (confirmed), else a choice.
func chooseProject(profilement *api.Client, settings *config.Config, folder, code string, yes bool, ask *prompt.Prompter, out io.Writer) (*api.Project, error) {
	projects, err := profilement.Projects()

	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return nil, errors.New("you have no project you may change: use \"profilement add\" for a new one")
	}

	find := func(code string) *api.Project {
		for index := range projects {
			if projects[index].Code == code {
				return &projects[index]
			}
		}

		return nil
	}

	if code != "" {
		if project := find(code); project != nil {
			return project, nil
		}

		return nil, fmt.Errorf("you have no project %q that you may change", code)
	}

	remembered := find(settings.Projects[folder])

	if yes {
		if remembered == nil {
			return nil, errors.New("--yes needs --project (no project is remembered for this folder)")
		}

		return remembered, nil
	}

	fallback := ""

	for index, project := range projects {
		line := fmt.Sprintf("%3d) %s · %s", index+1, project.Customer.Name, project.Name)

		if project.StartsOn != "" || project.EndsOn != "" {
			line += fmt.Sprintf("  (%s – %s)", orDash(project.StartsOn), orDash(project.EndsOn))
		}

		fmt.Fprintln(out, line)

		if remembered != nil && project.Code == remembered.Code {
			fallback = fmt.Sprint(index + 1)
		}
	}

	for {
		answer, err := ask.Ask("Which project", fallback)

		if err != nil {
			return nil, err
		}

		chosen, err := prompt.Selection(answer, len(projects), nil)

		if err == nil && len(chosen) == 1 {
			return &projects[chosen[0]], nil
		}

		fmt.Fprintln(out, "Choose one number of the list.")
	}
}

// chooseTopics groups the author's commits into topics and asks which become tasks.
func chooseTopics(folder, author string, yes bool, ask *prompt.Prompter, out io.Writer) ([]api.Task, error) {
	if author == "" {
		author = gitlog.UserEmail(folder)
	}

	commits, err := gitlog.Commits(folder, author)

	if err != nil {
		fmt.Fprintln(out, "\nGit topics left out:", err)

		return nil, nil
	}

	topics := gitlog.Topics(commits)

	if len(topics) > maxTopics {
		topics = topics[:maxTopics]
	}

	if len(topics) == 0 {
		fmt.Fprintf(out, "\nNo commits of %s in the Git history.\n", author)

		return nil, nil
	}

	var suggested []int
	fmt.Fprintf(out, "\nYour commits (%s) grouped into topics; only the topics you choose are sent, as tasks:\n", author)

	for index, topic := range topics {
		mark := " "

		if topic.Suggested {
			mark = "*"
			suggested = append(suggested, index)
		}

		commits := fmt.Sprintf("%d commits", len(topic.Commits))

		if len(topic.Commits) == 1 {
			commits = "1 commit"
		}

		period := topic.Start

		if topic.End != topic.Start {
			period += " – " + topic.End
		}

		fmt.Fprintf(out, "%3d)%s %s  (%s, %s)\n", index+1, mark, topic.Title, commits, period)
	}

	chosen := suggested

	if !yes {
		for {
			answer, err := ask.Ask("Topics as tasks — numbers like 1,3-5, \"all\", \"none\"; empty: the ones marked *", "")

			if err != nil {
				return nil, err
			}

			if chosen, err = prompt.Selection(answer, len(topics), suggested); err == nil {
				break
			}

			fmt.Fprintln(out, err)
		}
	}

	tasks := make([]api.Task, 0, len(chosen))

	for _, index := range chosen {
		topic := topics[index]
		tasks = append(tasks, api.Task{Title: truncate(topic.Title, 255), StartedOn: topic.Start, EndedOn: topic.End, Commits: len(topic.Commits)})
	}

	return tasks, nil
}

func orDash(text string) string {
	if text == "" {
		return "–"
	}

	return text
}

func truncate(text string, length int) string {
	runes := []rune(text)

	if len(runes) <= length {
		return text
	}

	return string(runes[:length-1]) + "…"
}

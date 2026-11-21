// Package api talks to Profilement's API (/api/v1) with a personal access token.
package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client is a client of one Profilement server.
type Client struct {
	Server    string
	Token     string
	UserAgent string
	http      *http.Client
}

// New is a client of the server with the token. $PROFILEMENT_INSECURE_TLS=1 skips the certificate check (only for a
// development server with a certificate of its own; never use it otherwise).
func New(server, token, userAgent string) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if os.Getenv("PROFILEMENT_INSECURE_TLS") == "1" {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // asked for explicitly
	}

	return &Client{Server: strings.TrimRight(server, "/"), Token: token, UserAgent: userAgent, http: &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
		// A redirect would carry the token elsewhere: never followed.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// Me is the account the token belongs to.
type Me struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Company struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"company"`
}

// Project is a project the account may update.
type Project struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Customer struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"customer"`
	StartsOn string `json:"starts_on"`
	EndsOn   string `json:"ends_on"`
}

// Term is a thing the project uses, as the API takes it.
type Term struct {
	Name    string `json:"name"`
	Package string `json:"package,omitempty"`
	Version string `json:"version,omitempty"`
	File    string `json:"file,omitempty"`
	Dev     bool   `json:"dev"`
}

// Task is a Git topic that becomes a task.
type Task struct {
	Title     string `json:"title"`
	StartedOn string `json:"started_on,omitempty"`
	EndedOn   string `json:"ended_on,omitempty"`
	Commits   int    `json:"commits"`
}

// Proposal is what is sent about a project.
type Proposal struct {
	Name    string            `json:"name"`
	Project string            `json:"project,omitempty"`
	Source  map[string]string `json:"source"`
	Terms   []Term            `json:"terms"`
	Tasks   []Task            `json:"tasks"`
}

// Sent is the answer to a proposal.
type Sent struct {
	Code      string `json:"code"`
	ReviewURL string `json:"review_url"`
	Terms     int    `json:"terms"`
	Tasks     int    `json:"tasks"`
}

// Me asks who the token belongs to.
func (client *Client) Me() (*Me, error) {
	var me Me

	return &me, client.do(http.MethodGet, "/api/v1/me", nil, &me)
}

// Projects are the projects the account may update, latest first.
func (client *Client) Projects() ([]Project, error) {
	var answer struct {
		Projects []Project `json:"projects"`
	}

	return answer.Projects, client.do(http.MethodGet, "/api/v1/projects", nil, &answer)
}

// Send sends a proposal; its review page is in the answer.
func (client *Client) Send(proposal Proposal) (*Sent, error) {
	var sent Sent

	return &sent, client.do(http.MethodPost, "/api/v1/project-imports", proposal, &sent)
}

func (client *Client) do(method, path string, body any, answer any) error {
	var reader io.Reader

	if body != nil {
		encoded, err := json.Marshal(body)

		if err != nil {
			return err
		}

		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequest(method, client.Server+path, reader)

	if err != nil {
		return err
	}

	request.Header.Set("Authorization", "Bearer "+client.Token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", client.UserAgent)

	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.http.Do(request)

	if err != nil {
		return fmt.Errorf("Profilement could not be reached: %w", err)
	}

	defer response.Body.Close()
	content, _ := io.ReadAll(io.LimitReader(response.Body, 10<<20))

	if response.StatusCode >= 300 {
		return failure(response.StatusCode, content)
	}

	return json.Unmarshal(content, answer)
}

// failure says what went wrong, with Profilement's message when it sent one.
func failure(status int, content []byte) error {
	var answer struct {
		Message string              `json:"message"`
		Errors  map[string][]string `json:"errors"`
	}

	_ = json.Unmarshal(content, &answer)

	switch status {
	case http.StatusUnauthorized:
		return fmt.Errorf("the token was refused (deleted or mistyped?): run \"profilement login\" with a new one")
	case http.StatusForbidden:
		return fmt.Errorf("the token may not do this (a token of the account page, of an account with work of its own, is needed)")
	case http.StatusTooManyRequests:
		return fmt.Errorf("too many requests: try again in a minute")
	}

	if len(answer.Errors) > 0 {
		var lines []string

		for field, messages := range answer.Errors {
			lines = append(lines, field+": "+strings.Join(messages, " "))
		}

		return fmt.Errorf("Profilement refused it: %s", strings.Join(lines, "; "))
	}

	if answer.Message != "" {
		return fmt.Errorf("Profilement answered %d: %s", status, answer.Message)
	}

	return fmt.Errorf("Profilement answered %d", status)
}

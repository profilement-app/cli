package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// recorded is a request the fake server got.
type recorded struct {
	method, path, authorization, accept, contentType, userAgent, body string
}

// fakeServer answers every request with the status and body of answer() and keeps what it got.
func fakeServer(t *testing.T, answer func(path string) (int, string)) (*httptest.Server, *[]recorded) {
	t.Helper()
	var requests []recorded
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, recorded{request.Method, request.URL.Path, request.Header.Get("Authorization"), request.Header.Get("Accept"), request.Header.Get("Content-Type"), request.Header.Get("User-Agent"), string(body)})
		status, content := answer(request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(content))
	}))
	t.Cleanup(server.Close)

	return server, &requests
}

func TestMeSendsTheTokenAsABearerAndReadsTheAccount(t *testing.T) {
	server, requests := fakeServer(t, func(string) (int, string) {
		return 200, `{"code": "anna", "name": "Anna Example", "company": {"code": "acme", "name": "Acme"}}`
	})

	me, err := New(server.URL+"/", "1|pfm_secret", "profilement-cli/test").Me()

	if err != nil || me.Name != "Anna Example" || me.Company.Name != "Acme" {
		t.Fatalf("me %+v, %v", me, err)
	}

	got := (*requests)[0]

	// The server's trailing slash is dropped; JSON is asked for; the tool names itself.
	if got.method != "GET" || got.path != "/api/v1/me" || got.authorization != "Bearer 1|pfm_secret" || got.accept != "application/json" || got.userAgent != "profilement-cli/test" {
		t.Errorf("request %+v", got)
	}
}

func TestProjectsAndSend(t *testing.T) {
	server, requests := fakeServer(t, func(path string) (int, string) {
		if path == "/api/v1/projects" {
			return 200, `{"projects": [{"code": "shop", "name": "Shop", "customer": {"code": "acme", "name": "Acme"}, "starts_on": "2026-01-01", "ends_on": null}]}`
		}

		return 201, `{"code": "shop_backend", "review_url": "https://example.com/account/project-imports/shop_backend", "terms": 1, "tasks": 1}`
	})
	client := New(server.URL, "1|pfm_secret", "test")

	projects, err := client.Projects()

	if err != nil || len(projects) != 1 || projects[0].Customer.Name != "Acme" || projects[0].StartsOn != "2026-01-01" || projects[0].EndsOn != "" {
		t.Fatalf("projects %+v, %v", projects, err)
	}

	sent, err := client.Send(Proposal{Name: "Shop backend", Project: "shop", Source: map[string]string{"tool": "profilement-cli"},
		Terms: []Term{{Name: "Laravel", Package: "laravel/framework", Version: "13.35.0", File: "composer.json"}},
		Tasks: []Task{{Title: "SHOP-12: Rework checkout", StartedOn: "2026-10-01", EndedOn: "2026-10-05", Commits: 3}}})

	if err != nil || sent.ReviewURL != "https://example.com/account/project-imports/shop_backend" {
		t.Fatalf("sent %+v, %v", sent, err)
	}

	got := (*requests)[1]
	var body map[string]any

	if got.method != "POST" || got.path != "/api/v1/project-imports" || got.contentType != "application/json" || json.Unmarshal([]byte(got.body), &body) != nil {
		t.Fatalf("request %+v", got)
	}

	// The fields as the API names them.
	for _, field := range []string{`"name":"Shop backend"`, `"project":"shop"`, `"package":"laravel/framework"`, `"dev":false`, `"started_on":"2026-10-01"`, `"commits":3`} {
		if !strings.Contains(got.body, field) {
			t.Errorf("body lacks %s: %s", field, got.body)
		}
	}
}

func TestARedirectIsNeverFollowedSoTheTokenGoesNowhereElse(t *testing.T) {
	elsewhere, reached := fakeServer(t, func(string) (int, string) { return 200, `{}` })
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, elsewhere.URL+"/steal", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	if _, err := New(server.URL, "1|pfm_secret", "test").Me(); err == nil {
		t.Error("a redirect was taken for an answer")
	}

	if len(*reached) != 0 {
		t.Errorf("the redirect was followed: %+v", *reached)
	}
}

func TestRefusalsSayWhatToDo(t *testing.T) {
	cases := []struct {
		status int
		body   string
		says   string
	}{
		{401, `{"message": "Unauthenticated."}`, "profilement login"},
		{403, `{"message": "Invalid ability provided."}`, "may not do this"},
		{429, ``, "try again in a minute"},
		{422, `{"message": "invalid", "errors": {"project": ["No project of yours that you may change has this code."]}}`, "project: No project of yours"},
		{500, `{"message": "Server Error"}`, "500: Server Error"},
		{502, `<html>Bad gateway</html>`, "answered 502"},
	}

	for _, test := range cases {
		server, _ := fakeServer(t, func(string) (int, string) { return test.status, test.body })
		_, err := New(server.URL, "1|pfm_secret", "test").Me()

		if err == nil || !strings.Contains(err.Error(), test.says) {
			t.Errorf("%d: %v, want it to say %q", test.status, err, test.says)
		}
	}
}

func TestAServerThatCannotBeReachedSaysSo(t *testing.T) {
	server, _ := fakeServer(t, func(string) (int, string) { return 200, `{}` })
	address := server.URL
	server.Close()

	if _, err := New(address, "1|pfm_secret", "test").Me(); err == nil || !strings.Contains(err.Error(), "could not be reached") {
		t.Errorf("closed server: %v", err)
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOnlyHttpsOrLocalHttpServersAreAccepted(t *testing.T) {
	cases := map[string]string{
		"https://www.profilement.com/":           "https://www.profilement.com",
		" https://user:pw@example.com/x/?a=1#b ": "https://example.com/x",
		"http://localhost:8000":                  "http://localhost:8000",
		"http://127.0.0.1":                       "http://127.0.0.1",
	}

	for given, want := range cases {
		if got, err := CheckServer(given); err != nil || got != want {
			t.Errorf("CheckServer(%q) = %q, %v; want %q", given, got, err, want)
		}
	}

	for _, refused := range []string{"http://www.profilement.com", "ftp://example.com", "www.profilement.com", ""} {
		if _, err := CheckServer(refused); err == nil {
			t.Errorf("CheckServer(%q) accepted", refused)
		}
	}
}

func TestTheConfigurationIsKeptForTheUserOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	t.Setenv("PROFILEMENT_CONFIG", path)
	t.Setenv("PROFILEMENT_URL", "")
	t.Setenv("PROFILEMENT_TOKEN", "")

	settings, err := Load()

	if err != nil || settings.Server != DefaultServer || settings.Token != "" {
		t.Fatalf("empty configuration: %+v %v", settings, err)
	}

	settings.Token = "1|pfm_secret"
	settings.Projects["/home/me/shop"] = "acme_shop"

	if err := settings.Save(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)

	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v %v", info.Mode().Perm(), err)
	}

	loaded, _ := Load()

	if loaded.Token != "1|pfm_secret" || loaded.Projects["/home/me/shop"] != "acme_shop" {
		t.Errorf("loaded %+v", loaded)
	}

	// The environment wins.
	t.Setenv("PROFILEMENT_TOKEN", "2|pfm_other")

	if loaded, _ = Load(); loaded.Token != "2|pfm_other" {
		t.Errorf("token from the environment: %q", loaded.Token)
	}
}

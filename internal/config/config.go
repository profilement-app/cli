// Package config keeps the CLI's settings: the Profilement server, the access token, and which Profilement project
// each folder was sent to. Stored in the user's configuration folder, readable by the user only.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DefaultServer is the Profilement the CLI talks to unless told otherwise.
const DefaultServer = "https://www.profilement.com"

// Config is what the CLI keeps between runs.
type Config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
	// Projects maps a folder's absolute path to the code of the Profilement project it was sent to.
	Projects map[string]string `json:"projects,omitempty"`
}

// Path is the configuration file: $PROFILEMENT_CONFIG, else <user config dir>/profilement/config.json.
func Path() (string, error) {
	if path := os.Getenv("PROFILEMENT_CONFIG"); path != "" {
		return path, nil
	}

	dir, err := os.UserConfigDir()

	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "profilement", "config.json"), nil
}

// Load reads the configuration (an empty one when there is none); $PROFILEMENT_URL and $PROFILEMENT_TOKEN override it.
func Load() (*Config, error) {
	config := &Config{Server: DefaultServer, Projects: map[string]string{}}
	path, err := Path()

	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)

	if err == nil {
		if err := json.Unmarshal(content, config); err != nil {
			return nil, fmt.Errorf("%s is no valid configuration: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if server := os.Getenv("PROFILEMENT_URL"); server != "" {
		config.Server = server
	}

	if token := os.Getenv("PROFILEMENT_TOKEN"); token != "" {
		config.Token = token
	}

	if config.Projects == nil {
		config.Projects = map[string]string{}
	}

	return config, nil
}

// Save writes the configuration, readable and writable by the user only.
func (config *Config) Save() error {
	path, err := Path()

	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	content, err := json.MarshalIndent(config, "", "  ")

	if err != nil {
		return err
	}

	// Written to a new file first, so a failed write never leaves half a configuration.
	temporary := path + ".tmp"

	if err := os.WriteFile(temporary, append(content, '\n'), 0o600); err != nil {
		return err
	}

	return os.Rename(temporary, path)
}

// CheckServer accepts only https addresses, or http on this computer (for development).
func CheckServer(server string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(server), "/"))

	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("%q is no address of a server", server)
	}

	host := parsed.Hostname()
	local := host == "localhost" || host == "127.0.0.1" || host == "::1"

	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && local) {
		return "", fmt.Errorf("%q: only https is allowed (the token would travel unencrypted)", server)
	}

	parsed.Path, parsed.RawQuery, parsed.Fragment, parsed.User = strings.TrimRight(parsed.Path, "/"), "", "", nil

	return parsed.String(), nil
}

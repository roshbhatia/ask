package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	shared "github.com/roshbhatia/go-utils/config"
	"go.yaml.in/yaml/v3"

	"github.com/roshbhatia/ask/internal/provider"
)

const (
	ProviderDefault = "provider.default"
	Version         = "ask.config/v1"
)

type Provider struct {
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
}

type File struct {
	Version  string   `json:"version" yaml:"version" jsonschema:"enum=ask.config/v1"`
	Provider Provider `json:"provider,omitempty" yaml:"provider,omitempty"`
}

type Setting struct {
	Key    string
	Help   string
	Values func() []string
	Clean  func(string) (string, error)
}

var settings = []Setting{{
	Key:    ProviderDefault,
	Help:   "the agent to run when no -p and no ASK_PROVIDER say otherwise",
	Values: provider.Names,
	Clean: func(value string) (string, error) {
		one, ok := provider.Lookup(value)
		if !ok {
			return "", fmt.Errorf("unknown provider %q, known: %s", value, strings.Join(provider.Names(), ", "))
		}
		return one.Name, nil
	},
}}

func Settings() []Setting { return slices.Clone(settings) }

func Keys() []string {
	keys := make([]string, 0, len(settings))
	for _, one := range settings {
		keys = append(keys, one.Key)
	}
	return keys
}

func find(key string) (Setting, error) {
	for _, one := range settings {
		if one.Key == key {
			return one, nil
		}
	}
	return Setting{}, fmt.Errorf("unknown setting %q, known: %s", key, strings.Join(Keys(), ", "))
}

func options() shared.Options { return shared.Options{Name: "ask", EnvPrefix: "ASK"} }

func Path() string {
	path, err := shared.Path(options())
	if err == nil {
		return path
	}
	return filepath.Join(".config", "ask", "config.yaml")
}

func legacyPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".config", "ask", "config.json")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "ask", "config.json")
}

func loadFile() (File, error) {
	defaults := File{Version: Version}
	if _, err := os.Stat(Path()); errors.Is(err, os.ErrNotExist) && os.Getenv("ASK_CONFIG") == "" {
		legacy, readErr := os.ReadFile(legacyPath())
		if readErr == nil && len(strings.TrimSpace(string(legacy))) > 0 {
			values := map[string]string{}
			if err := json.Unmarshal(legacy, &values); err != nil {
				return defaults, fmt.Errorf("%s will not parse: %w", legacyPath(), err)
			}
			defaults.Provider.Default = values[ProviderDefault]
		} else if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return defaults, readErr
		}
	}
	loaded, err := shared.Load(defaults, options())
	if err != nil {
		return defaults, err
	}
	if loaded.Version != Version {
		return defaults, fmt.Errorf("%s version must be %q", Path(), Version)
	}
	return loaded, nil
}

func Load() (map[string]string, error) {
	loaded, err := loadFile()
	if err != nil {
		return nil, err
	}
	return map[string]string{ProviderDefault: loaded.Provider.Default}, nil
}

func Get(key string) (string, error) {
	if _, err := find(key); err != nil {
		return "", err
	}
	held, err := Load()
	if err != nil {
		return "", err
	}
	return held[key], nil
}

func Set(pair string) (string, string, error) {
	key, value, ok := strings.Cut(pair, "=")
	if !ok {
		return "", "", fmt.Errorf("say it as KEY=VALUE, not %q", pair)
	}
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)

	setting, err := find(key)
	if err != nil {
		return "", "", err
	}
	if setting.Clean != nil && value != "" {
		if value, err = setting.Clean(value); err != nil {
			return "", "", err
		}
	}

	loaded, err := loadFile()
	if err != nil {
		return "", "", err
	}
	loaded.Provider.Default = value
	return key, value, save(loaded)
}

func save(value File) error {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer func() { _ = os.Remove(name) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func List() ([]string, error) {
	held, err := Load()
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(settings))
	for _, one := range settings {
		value := held[one.Key]
		if value == "" {
			value = "(unset)"
		}
		lines = append(lines, one.Key+"="+value)
	}
	sort.Strings(lines)
	return lines, nil
}

func Schema() ([]byte, error) { return shared.Schema[File]("Ask configuration") }

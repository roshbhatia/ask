package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Package struct {
	Name         string    `json:"name"`
	Core         string    `json:"core"`
	Nix          string    `json:"nix"`
	Brew         string    `json:"brew"`
	Binary       string    `json:"binary"`
	RuntimeNote  string    `json:"runtime_note"`
	ManifestPath string    `json:"manifest_path"`
	Dependencies *[]string `json:"dependencies,omitempty"`
	Summary      string    `json:"summary,omitempty"`
}

type Demo struct {
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	Recording string `json:"recording,omitempty"`
}

type Archive struct {
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	Binary       string    `json:"binary"`
	Description  string    `json:"description"`
	Archive      string    `json:"archive"`
	Share        []string  `json:"share"`
	PythonScript bool      `json:"python_script"`
	Dependencies *[]string `json:"dependencies,omitempty"`
}

func readJSON(path string, value interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func write(path string, data []byte, check bool) error {
	if check {
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, data) {
			return fmt.Errorf("stale generated file: %s", path)
		}
		return nil
	}
	return os.WriteFile(path, data, 0644)
}

func run() error {
	check := len(os.Args) == 2 && os.Args[1] == "--check"
	if len(os.Args) > 2 || len(os.Args) == 2 && !check {
		return fmt.Errorf("usage: go run ./hack/docgen [--check]")
	}
	paths, err := filepath.Glob("extras/*/package.json")
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no extra package metadata; run from the repository root")
	}
	index := struct {
		Version   int       `json:"version"`
		Providers []Package `json:"providers"`
		Packages  []Archive `json:"packages"`
	}{Version: 1}
	catalog := "| Extra | Capability | Recording |\n|---|---|---|\n"
	for _, path := range paths {
		dir := filepath.Dir(path)
		var pkg Package
		if err := readJSON(path, &pkg); err != nil {
			return err
		}
		if pkg.Name != filepath.Base(dir) {
			return fmt.Errorf("package name does not match directory: %s", path)
		}
		manifest, err := os.ReadFile(filepath.Join(dir, "provider.yaml"))
		if err != nil {
			return err
		}
		var capability struct {
			Description string `yaml:"description"`
		}
		if err := yaml.Unmarshal(manifest, &capability); err != nil {
			return err
		}
		if capability.Description == "" {
			return fmt.Errorf("missing provider description: %s", dir)
		}
		pkg.Summary = capability.Description
		index.Providers = append(index.Providers, pkg)
		index.Packages = append(index.Packages, Archive{
			Name: pkg.Brew, Kind: "provider", Binary: pkg.Binary,
			Description: capability.Description,
			Archive:     pkg.Core + "_provider_" + pkg.Name + "_%{version}_%{os}_%{arch}.tar.gz",
			Share:       []string{pkg.ManifestPath}, Dependencies: pkg.Dependencies,
		})
		var demo Demo
		if err := readJSON(filepath.Join(dir, "demo.json"), &demo); err != nil {
			return err
		}
		if demo.Status != "pending" && demo.Status != "recorded" {
			return fmt.Errorf("invalid demo status: %s", dir)
		}
		for _, name := range []string{"demo.sh", "demo.tape"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				return err
			}
		}
		text := "# " + pkg.Name + "\n\n" + capability.Description + ".\n\n"
		text += "## Install\n\n```sh\nbrew install roshbhatia/tap/" + pkg.Brew + "\nnix profile add '" + pkg.Nix + "'\n```\n\n"
		text += "Install the core utility separately, or select its all-provider bundle.\n\n" + pkg.RuntimeNote + "\n\n## Demo\n\n"
		if demo.Status == "recorded" {
			if demo.Recording == "" {
				return fmt.Errorf("missing recording: %s", dir)
			}
			if _, err := os.Stat(filepath.Join(dir, demo.Recording)); err != nil {
				return err
			}
			text += "![" + demo.Summary + "](" + demo.Recording + ")\n\n"
			catalog += fmt.Sprintf("| [%s](%s/README.md) | %s | [Demo](%s/README.md#demo) |\n", pkg.Name, pkg.Name, capability.Description, pkg.Name)
		} else {
			text += "Live recording pending. The previous canned-response recording was withdrawn.\n\n"
			catalog += fmt.Sprintf("| [%s](%s/README.md) | %s | Pending |\n", pkg.Name, pkg.Name, capability.Description)
		}
		text += demo.Summary + ". [Tape source](demo.tape).\n\n"
		text += "Run `nix develop -c bash extras/" + pkg.Name + "/demo.sh` with the real runtime installed and authenticated.\n"
		text += "Add `--record` to capture the interactive session. This invokes the real service; output and timing vary.\n"
		if err := write(filepath.Join(dir, "README.md"), []byte(text), check); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := write("package-index.json", append(data, '\n'), check); err != nil {
		return err
	}
	data, err = os.ReadFile("extras/README.md")
	if err != nil {
		return err
	}
	start, end := "<!-- BEGIN GENERATED CATALOG -->", "<!-- END GENERATED CATALOG -->"
	text := string(data)
	if strings.Count(text, start) != 1 || strings.Count(text, end) != 1 {
		return fmt.Errorf("invalid extras catalog markers")
	}
	before, rest, _ := strings.Cut(text, start)
	_, after, _ := strings.Cut(rest, end)
	return write("extras/README.md", []byte(before+start+"\n\n"+catalog+"\n"+end+after), check)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Package inventory loads the embedded machine-readable command inventory.
package inventory

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

//go:embed commands.json
var commandsJSON []byte

//go:embed sources.json
var sourcesJSON []byte

type Inventory struct {
	Commands []Command `json:"commands"`
	UIOnly   []UIOnly  `json:"ui_only"`
	Sources  []Source  `json:"sources"`
}

type Command struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	AliasOf    string   `json:"alias_of"`
	Sources    []string `json:"sources"`
	Build      string   `json:"build"`
	Capability string   `json:"capability"`
	Provenance string   `json:"provenance"`
	Tests      []string `json:"tests"`
}

type UIOnly struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

type Source struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Revision       string `json:"revision"`
	License        string `json:"license"`
	Status         string `json:"status"`
	Notes          string `json:"notes"`
	ImportRevision string `json:"import_revision,omitempty"`
	SourceMap      string `json:"source_map,omitempty"`
}

// Load decodes the embedded data and returns entries sorted by name.
func Load() (*Inventory, error) {
	var commands struct {
		Commands []Command `json:"commands"`
		UIOnly   []UIOnly  `json:"ui_only"`
	}
	var sources struct {
		Sources []Source `json:"sources"`
	}
	if err := decode(commandsJSON, &commands); err != nil {
		return nil, fmt.Errorf("commands.json: %w", err)
	}
	if err := decode(sourcesJSON, &sources); err != nil {
		return nil, fmt.Errorf("sources.json: %w", err)
	}
	sort.Slice(commands.Commands, func(i, j int) bool { return commands.Commands[i].Name < commands.Commands[j].Name })
	sort.Slice(commands.UIOnly, func(i, j int) bool { return commands.UIOnly[i].Name < commands.UIOnly[j].Name })
	sort.Slice(sources.Sources, func(i, j int) bool { return sources.Sources[i].Name < sources.Sources[j].Name })
	return &Inventory{Commands: commands.Commands, UIOnly: commands.UIOnly, Sources: sources.Sources}, nil
}

func decode(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

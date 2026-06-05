package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const manifestFile = "manifest.json"

// ReadManifest reads the manifest from the store directory. See design §6.1.
func ReadManifest(storePath string) (*Manifest, error) {
	p := filepath.Join(storePath, manifestFile)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{
				Version:     1,
				Collections: make(map[string]CollectionConfig),
			}, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Collections == nil {
		m.Collections = make(map[string]CollectionConfig)
	}
	return &m, nil
}

// WriteManifest writes the manifest to the store directory. See design §6.1.
func WriteManifest(storePath string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	p := filepath.Join(storePath, manifestFile)
	// Write atomically via temp file + rename.
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write manifest tmp: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("rename manifest: %w", err)
	}
	return nil
}

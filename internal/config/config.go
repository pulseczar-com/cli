// Package config reads and writes the CLI's saved sign-in.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultURL is the Czar the CLI talks to unless told otherwise.
const DefaultURL = "https://pulseczar.com"

// File is what ~/.config/pulseczar/config.json holds.
type File struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// DefaultPath returns ~/.config/pulseczar/config.json for the given home directory.
func DefaultPath(home string) string {
	return filepath.Join(home, ".config", "pulseczar", "config.json")
}

// Load reads the file. A missing file is an empty File, not an error.
func Load(path string) (File, error) {
	var file File

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return file, nil
	}
	if err != nil {
		return file, err
	}

	if err := json.Unmarshal(data, &file); err != nil {
		return file, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}

	return file, nil
}

// Save writes the file with mode 0600, because it holds a token.
func Save(path string, file File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return err
	}

	// WriteFile keeps the mode of a file that already exists.
	return os.Chmod(path, 0o600)
}

// Remove deletes the file. A missing file is not an error.
func Remove(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

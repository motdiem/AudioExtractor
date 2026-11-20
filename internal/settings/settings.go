// Package settings is a tiny per-user key/value store backed by one JSON file.
// Each feature owns a key and a Go type (the output format today, the window
// position in ticket #10), so adding a setting never touches this package.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store reads and writes a JSON object of named values. It is safe for
// concurrent use. A missing or corrupt file simply reads as empty: settings
// are a convenience and must never stop the app from starting.
type Store struct {
	path string
	mu   sync.Mutex
}

// New returns a Store for the file at path (created on first Set).
func New(path string) *Store { return &Store{path: path} }

// DefaultPath is <user config dir>/AudioExtractor/settings.json.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "AudioExtractor", "settings.json"), nil
}

// Get decodes the value stored under key into v and reports whether it was
// present and valid. v is left untouched otherwise.
func (s *Store) Get(key string, v any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.read()[key]
	return ok && json.Unmarshal(raw, v) == nil
}

// Set stores v under key, keeping every other key, and writes the file
// atomically (temp file, then rename).
func (s *Store) Set(key string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	all := s.read()
	all[key] = raw
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".settings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func (s *Store) read() map[string]json.RawMessage {
	all := map[string]json.RawMessage{}
	if data, err := os.ReadFile(s.path); err == nil {
		if json.Unmarshal(data, &all) != nil || all == nil {
			return map[string]json.RawMessage{}
		}
	}
	return all
}

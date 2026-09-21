package appsettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu     sync.RWMutex
	path   string
	values Values
}

func Open(path string) (*Store, error) {
	values := Values{TaiwanAlerts: TaiwanAlerts{CorporateEventsEnabled: true}}
	if path != "" {
		loaded, err := readSettings(path, values)
		if err != nil {
			return nil, err
		}
		values = loaded
	}
	return &Store{path: path, values: compatibleValues(values)}, nil
}

func (s *Store) Snapshot() Values {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneValues(s.values)
}

func (s *Store) Update(update func(*Values) error) (Values, error) {
	if update == nil {
		return Values{}, errors.New("settings update is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	next := cloneValues(s.values)
	if err := update(&next); err != nil {
		return Values{}, err
	}
	next = compatibleValues(next)
	next.UpdatedAt = time.Now().UTC()
	if err := writeSettings(s.path, next); err != nil {
		return Values{}, err
	}
	s.values = next
	return cloneValues(next), nil
}

func readSettings(path string, defaults Values) (Values, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return Values{}, fmt.Errorf("read settings: %w", err)
	}
	if len(data) == 0 {
		return Values{}, errors.New("decode settings: empty file")
	}
	values := defaults
	if err := json.Unmarshal(data, &values); err != nil {
		return Values{}, fmt.Errorf("decode settings: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return Values{}, fmt.Errorf("secure settings permissions: %w", err)
	}
	return values, nil
}

func writeSettings(path string, values Values) error {
	if path == "" {
		return nil
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	temp, err := os.CreateTemp(directory, ".mystocktracer-settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create settings temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	closeWithError := func(cause error) error {
		_ = temp.Close()
		return cause
	}
	if err := temp.Chmod(0o600); err != nil {
		return closeWithError(fmt.Errorf("secure settings temp file: %w", err))
	}
	if _, err := temp.Write(data); err != nil {
		return closeWithError(fmt.Errorf("write settings: %w", err))
	}
	if err := temp.Sync(); err != nil {
		return closeWithError(fmt.Errorf("sync settings: %w", err))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close settings: %w", err)
	}
	if err := replaceFile(tempPath, path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure settings file: %w", err)
	}
	return nil
}

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}

func cloneValues(values Values) Values {
	values.LLMProfiles = append([]LLMProfile(nil), values.LLMProfiles...)
	return values
}

package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RegisteredStack holds persistent metadata for a user-registered compose stack.
type RegisteredStack struct {
	Name        string `json:"name"`
	WorkingDir  string `json:"working_dir"`
	ComposePath string `json:"compose_path"`
}

// Store is a thread-safe, file-backed registry of compose stacks.
type Store struct {
	mu     sync.RWMutex
	path   string
	stacks map[string]RegisteredStack
}

// NewStore creates a Store backed by stacks.json in dataDir.
// If dataDir is empty, defaults to ~/.hola/.
func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("registry: user home dir: %w", err)
		}
		dataDir = filepath.Join(home, ".hola")
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("registry: create data dir: %w", err)
	}

	s := &Store{
		path:   filepath.Join(dataDir, "stacks.json"),
		stacks: make(map[string]RegisteredStack),
	}

	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Register adds or updates a stack in the registry and persists to disk.
func (s *Store) Register(name, workingDir, composePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stacks[name] = RegisteredStack{
		Name:        name,
		WorkingDir:  workingDir,
		ComposePath: composePath,
	}
	return s.save()
}

// Unregister removes a stack from the registry and persists to disk.
func (s *Store) Unregister(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.stacks, name)
	return s.save()
}

// Get returns a registered stack by name, or nil if not found.
func (s *Store) Get(name string) *RegisteredStack {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if rs, ok := s.stacks[name]; ok {
		return &rs
	}
	return nil
}

// All returns a copy of every registered stack.
func (s *Store) All() []RegisteredStack {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]RegisteredStack, 0, len(s.stacks))
	for _, rs := range s.stacks {
		out = append(out, rs)
	}
	return out
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // first run — no file yet
		}
		return fmt.Errorf("registry: read %s: %w", s.path, err)
	}

	var list []RegisteredStack
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("registry: parse %s: %w", s.path, err)
	}

	for _, rs := range list {
		s.stacks[rs.Name] = rs
	}
	return nil
}

func (s *Store) save() error {
	list := make([]RegisteredStack, 0, len(s.stacks))
	for _, rs := range s.stacks {
		list = append(list, rs)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshal: %w", err)
	}
	return os.WriteFile(s.path, data, 0o644)
}

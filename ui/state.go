package ui

import "sync"

// State holds the secret that was most recently read, shared across all tabs.
// It is the in-memory equivalent of the bash scripts' .secret-metadata file.
type State struct {
	mu      sync.RWMutex
	Vault   string
	Secret  string
	Version string
	Value   string
}

// Set stores a freshly-read secret atomically.
func (s *State) Set(vault, secret, version, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Vault = vault
	s.Secret = secret
	s.Version = version
	s.Value = value
}

// Get returns the current state values atomically.
func (s *State) Get() (vault, secret, version, value string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Vault, s.Secret, s.Version, s.Value
}

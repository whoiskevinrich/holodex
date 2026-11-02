// Package settings is the owner-settable runtime settings allowlist
// (docs/architecture/config-and-settings.md, "Promoting a config value to an owner-editable
// runtime setting"). A key is writable only when it is declared here with its default and
// validator, and every write path goes through Put, so a validator can't be skipped. The effective
// value is the stored row, else the key's default, held in memory so reads never touch the DB.
package settings

import (
	"context"
	"errors"
	"sync"
)

// ErrUnknownKey refuses a key that isn't on the allowlist.
var ErrUnknownKey = errors.New("not an owner-settable setting")

// InvalidError is a value its key's validator refused, as opposed to a storage failure.
type InvalidError struct{ Err error }

func (e *InvalidError) Error() string { return e.Err.Error() }
func (e *InvalidError) Unwrap() error { return e.Err }

// Key declares one owner-settable setting. Validate returns the value to store (it may normalise
// it) or an error that names what is wrong.
type Key struct {
	Name     string
	Default  string
	Validate func(value string) (string, error)
}

// Backing is the persistent store under the allowlist (repo.Repo).
type Backing interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	PutSetting(ctx context.Context, key, value string) error
}

// Store holds the effective value of every declared key.
type Store struct {
	db   Backing
	mu   sync.RWMutex
	keys map[string]Key
	vals map[string]string
}

// New declares keys over db. Load fills the stored values.
func New(db Backing, keys ...Key) *Store {
	s := &Store{db: db, keys: map[string]Key{}, vals: map[string]string{}}
	for _, k := range keys {
		s.keys[k.Name] = k
		s.vals[k.Name] = k.Default
	}
	return s
}

// Load reads every declared key's stored row. A stored value that no longer validates (a template
// naming a provider removed since, say) still loads, so the owner sees and fixes it rather than
// losing it to the default; every reader re-validates what it uses.
func (s *Store) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name := range s.keys {
		v, ok, err := s.db.GetSetting(ctx, name)
		if err != nil {
			return err
		}
		if ok {
			s.vals[name] = v
		}
	}
	return nil
}

// Get returns a declared key's effective value, or "" for an undeclared one.
func (s *Store) Get(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vals[name]
}

// All returns every declared key's effective value.
func (s *Store) All() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.vals))
	for k, v := range s.vals {
		out[k] = v
	}
	return out
}

// Put validates and stores a declared key, then refreshes its effective value.
func (s *Store) Put(ctx context.Context, name, value string) error {
	k, ok := s.keys[name]
	if !ok {
		return ErrUnknownKey
	}
	nv, err := k.Validate(value)
	if err != nil {
		return &InvalidError{Err: err}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.PutSetting(ctx, name, nv); err != nil {
		return err
	}
	s.vals[name] = nv
	return nil
}

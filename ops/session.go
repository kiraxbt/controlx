package ops

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// SessionConfig holds session timeout configuration.
type SessionConfig struct {
	Enabled    bool
	TimeoutMin int // minutes of inactivity before auto-lock
	mu         sync.Mutex
	lastActive time.Time
	locked     bool
}

// NewSession creates a new session config.
func NewSession(timeoutMin int) *SessionConfig {
	return &SessionConfig{
		Enabled:    timeoutMin > 0,
		TimeoutMin: timeoutMin,
		lastActive: time.Now(),
	}
}

// NoSession returns a disabled session config.
func NoSession() *SessionConfig {
	return &SessionConfig{Enabled: false}
}

// Touch resets the idle timer.
func (s *SessionConfig) Touch() {
	if !s.Enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActive = time.Now()
	s.locked = false
}

// IsLocked returns true if the session has timed out.
func (s *SessionConfig) IsLocked() bool {
	if !s.Enabled {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked {
		return true
	}
	if time.Since(s.lastActive) > time.Duration(s.TimeoutMin)*time.Minute {
		s.locked = true
		return true
	}
	return false
}

// Lock manually locks the session.
func (s *SessionConfig) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locked = true
}

// Unlock unlocks the session.
func (s *SessionConfig) Unlock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.locked = false
	s.lastActive = time.Now()
}

// IdleTime returns how long the session has been idle.
func (s *SessionConfig) IdleTime() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.lastActive)
}

// RemainingTime returns the time until auto-lock.
func (s *SessionConfig) RemainingTime() time.Duration {
	if !s.Enabled {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	timeout := time.Duration(s.TimeoutMin) * time.Minute
	remaining := timeout - time.Since(s.lastActive)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// HashLogEntry generates a SHA-256 hash for tamper detection in audit logs.
func HashLogEntry(prevHash, entry string) string {
	data := fmt.Sprintf("%s|%s", prevHash, entry)
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h[:16])
}

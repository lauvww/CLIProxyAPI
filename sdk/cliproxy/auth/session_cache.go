package auth

import (
	"sync"
	"time"
)

type sessionEntry struct {
	authID    string
	expiresAt time.Time
}

// SessionCache keeps session-to-auth bindings with TTL-based eviction.
type SessionCache struct {
	mu      sync.RWMutex
	entries map[string]sessionEntry
	ttl     time.Duration
	stopCh  chan struct{}
}

func NewSessionCache(ttl time.Duration) *SessionCache {
	if ttl <= 0 {
		ttl = time.Hour
	}
	cache := &SessionCache{
		entries: make(map[string]sessionEntry),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}
	go cache.cleanupLoop()
	return cache
}

func (c *SessionCache) Get(sessionID string) (string, bool) {
	if c == nil || sessionID == "" {
		return "", false
	}

	c.mu.RLock()
	entry, ok := c.entries[sessionID]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, sessionID)
		c.mu.Unlock()
		return "", false
	}
	return entry.authID, true
}

func (c *SessionCache) GetAndRefresh(sessionID string) (string, bool) {
	if c == nil || sessionID == "" {
		return "", false
	}

	now := time.Now()
	c.mu.Lock()
	entry, ok := c.entries[sessionID]
	if !ok {
		c.mu.Unlock()
		return "", false
	}
	if now.After(entry.expiresAt) {
		delete(c.entries, sessionID)
		c.mu.Unlock()
		return "", false
	}
	entry.expiresAt = now.Add(c.ttl)
	c.entries[sessionID] = entry
	c.mu.Unlock()
	return entry.authID, true
}

func (c *SessionCache) Set(sessionID, authID string) {
	if c == nil || sessionID == "" || authID == "" {
		return
	}

	c.mu.Lock()
	c.entries[sessionID] = sessionEntry{
		authID:    authID,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *SessionCache) Invalidate(sessionID string) {
	if c == nil || sessionID == "" {
		return
	}
	c.mu.Lock()
	delete(c.entries, sessionID)
	c.mu.Unlock()
}

func (c *SessionCache) InvalidateAuth(authID string) {
	if c == nil || authID == "" {
		return
	}
	c.mu.Lock()
	for sessionID, entry := range c.entries {
		if entry.authID == authID {
			delete(c.entries, sessionID)
		}
	}
	c.mu.Unlock()
}

func (c *SessionCache) Stop() {
	if c == nil {
		return
	}
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

func (c *SessionCache) cleanupLoop() {
	interval := c.ttl / 2
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

func (c *SessionCache) cleanup() {
	if c == nil {
		return
	}
	now := time.Now()
	c.mu.Lock()
	for sessionID, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, sessionID)
		}
	}
	c.mu.Unlock()
}

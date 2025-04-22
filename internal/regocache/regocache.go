// Package regocache implements in-memory caching of
// policy data structure. It also implements a function
// to purge the cache when external data changes have happened.
package regocache

import (
	"context"
	"sync"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
)

type Cache struct {
	mu    sync.RWMutex
	cache map[string]*storage.Policy
}

func New() *Cache {
	return &Cache{
		cache: map[string]*storage.Policy{},
	}
}

func (c *Cache) Set(key string, policy *storage.Policy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = policy
}

func (c *Cache) Get(key string) (policy *storage.Policy, found bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.cache[key]
	return v, ok
}

// Purge deletes all cache values.
func (c *Cache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = map[string]*storage.Policy{}
}

// PolicyDataChange triggers purge on the cache.
func (c *Cache) PolicyDataChange(_ context.Context, _, _, _, _ string) error {
	c.Purge()
	return nil
}

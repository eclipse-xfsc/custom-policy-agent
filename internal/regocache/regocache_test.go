package regocache_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/regocache"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/policy"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
)

func TestNew(t *testing.T) {
	cache := regocache.New()
	assert.Implements(t, (*policy.RegoCache)(nil), cache)
}

func TestCache_SetAndGet(t *testing.T) {
	p1 := storage.Policy{
		Filename:   "policy.rego",
		Name:       "example",
		Group:      "example",
		Version:    "1.0",
		Rego:       `package example.example _ = external.http.header("Authorization")`,
		Data:       `{"hello":"world"}`,
		Locked:     false,
		LastUpdate: time.Now(),
	}

	cache := regocache.New()
	cache.Set("key1", &p1)

	p2, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, p1, *p2)
}

func TestCache_Purge(t *testing.T) {
	p1 := storage.Policy{
		Filename:   "policy.rego",
		Name:       "example",
		Group:      "example",
		Version:    "1.0",
		Rego:       `package example.example _ = external.http.header("Authorization")`,
		Data:       `{"hello":"world"}`,
		Locked:     false,
		LastUpdate: time.Now(),
	}

	cache := regocache.New()
	cache.Set("key1", &p1)

	cache.Purge()
	q2, ok := cache.Get("key1")
	assert.False(t, ok)
	assert.Nil(t, q2)
}

func TestCache_PolicyDataChange(t *testing.T) {
	p1 := storage.Policy{
		Filename:   "policy.rego",
		Name:       "example",
		Group:      "example",
		Version:    "1.0",
		Rego:       `package example.example _ = external.http.header("Authorization")`,
		Data:       `{"hello":"world"}`,
		Locked:     false,
		LastUpdate: time.Now(),
	}

	cache := regocache.New()
	cache.Set("key1", &p1)

	err := cache.PolicyDataChange(context.Background(), "", "", "", "")
	assert.Nil(t, err)
	q2, ok := cache.Get("key1")
	assert.False(t, ok)
	assert.Nil(t, q2)
}

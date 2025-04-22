package memory_test

import (
	"context"
	"testing"
	"time"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage/memory"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage/memory/memoryfakes"
)

const validKey = "policies,example,foo,1.0"

func TestStorage_Policy(t *testing.T) {
	// policy is not found in memory storage
	keyConstructor := &memoryfakes.FakeKeyConstructor{ConstructKeyStub: func(s string, s2 string, s3 string, s4 string) string {
		return "non-existing policy key"
	}}

	storage := memory.New(keyConstructor, makePolicies(), zap.NewNop())

	p, err := storage.Policy(context.Background(), "repo", "group", "name", "version")
	assert.Error(t, err)
	assert.Nil(t, p)

	e, ok := err.(*errors.Error)
	assert.True(t, ok)
	assert.Equal(t, errors.NotFound, e.Kind)
	assert.Contains(t, e.Error(), "policy not found in memory storage")

	// policy is successfully found in memory storage
	keyConstructor = &memoryfakes.FakeKeyConstructor{ConstructKeyStub: func(s string, s2 string, s3 string, s4 string) string {
		return validKey
	}}

	storage = memory.New(keyConstructor, makePolicies(), zap.NewNop())

	p, err = storage.Policy(context.Background(), "repo", "group", "name", "version")
	assert.NoError(t, err)
	assert.Equal(t, "policies", p.Repository)
	assert.Equal(t, "example", p.Group)
	assert.Equal(t, "foo", p.Name)
	assert.Equal(t, "1.0", p.Version)
}

func TestStorage_SetPolicyLock(t *testing.T) {
	// policy is not found in memory storage
	keyConstructor := &memoryfakes.FakeKeyConstructor{ConstructKeyStub: func(s string, s2 string, s3 string, s4 string) string {
		return "non-existing policy key"
	}}

	storage := memory.New(keyConstructor, makePolicies(), zap.NewNop())

	err := storage.SetPolicyLock(context.Background(), "repo", "group", "name", "version", true)
	assert.Error(t, err)

	e, ok := err.(*errors.Error)
	assert.True(t, ok)
	assert.Equal(t, errors.NotFound, e.Kind)
	assert.Contains(t, e.Error(), "policy not found in memory storage")

	// policy is locked successfully
	keyConstructor = &memoryfakes.FakeKeyConstructor{ConstructKeyStub: func(s string, s2 string, s3 string, s4 string) string {
		return validKey
	}}

	policies := makePolicies()
	storage = memory.New(keyConstructor, policies, zap.NewNop())

	// lock the policy
	err = storage.SetPolicyLock(context.Background(), "repo", "group", "name", "version", true)
	assert.NoError(t, err)
	assert.Equal(t, true, policies[validKey].Locked)
}

func TestStorage_GetPolicies(t *testing.T) {
	policies := makePolicies()
	storage := memory.New(nil, policies, zap.NewNop())

	// get all policies
	res, err := storage.GetPolicies(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, len(policies), len(res))

	// get locked policies
	locked := true
	res, err = storage.GetPolicies(context.Background(), &locked, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, locked, res[0].Locked)

	// get unlocked policies
	locked = false
	res, err = storage.GetPolicies(context.Background(), &locked, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, locked, res[0].Locked)

	// get policies filtered by name
	name := "example"
	res, err = storage.GetPolicies(context.Background(), nil, &name)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
	assert.Contains(t, res[0].Name, name)
}

func TestStorage_GetRefreshPolicies(t *testing.T) {
	nextDataRefreshTime := time.Now().Add(-5 * time.Minute) // time must be in the past

	policies := makePolicies()
	policies[validKey].NextDataRefreshTime = nextDataRefreshTime

	storage := memory.New(nil, policies, zap.NewNop())

	res, err := storage.GetRefreshPolicies(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))

	// GetRefreshPolicies function adds storage.RefreshPostponePeriod Duration
	// to the stored policy's NextDataRefreshTime
	assert.Greater(t, policies[validKey].NextDataRefreshTime, nextDataRefreshTime)
}

func TestStorage_CommonStorage(t *testing.T) {
	storage := memory.New(nil, nil, zap.NewNop())

	t.Run("set data", func(t *testing.T) {
		err := storage.SetData(context.Background(), "exampleKey", map[string]interface{}{"some": "data"})
		assert.NoError(t, err)
	})
	t.Run("update data", func(t *testing.T) {
		err := storage.SetData(context.Background(), "exampleKey", map[string]interface{}{"some": "updated_data"})
		assert.NoError(t, err)
	})

	t.Run("get data", func(t *testing.T) {
		data, err := storage.GetData(context.Background(), "exampleKey")
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"some": "updated_data"}, data)
	})

	t.Run("get with not existing key", func(t *testing.T) {
		_, err := storage.GetData(context.Background(), "notExistingKey")
		assert.ErrorContains(t, err, "doesn't exist")
	})

	t.Run("delete data", func(t *testing.T) {
		err := storage.DeleteData(context.Background(), "exampleKey")
		assert.NoError(t, err)
	})

	t.Run("delete data with key error", func(t *testing.T) {
		err := storage.DeleteData(context.Background(), "exampleKey")
		assert.ErrorContains(t, err, "doesn't exist")
	})
}

func TestStorage_PolicySubscriber(t *testing.T) {
	s := memory.New(nil, nil, zap.NewNop())
	subscriber := &storage.Subscriber{
		Name:             "name",
		WebhookURL:       "webhook",
		PolicyRepository: "repo",
		PolicyName:       "policyname",
		PolicyGroup:      "policygroup",
		PolicyVersion:    "policyversion",
		CreatedAt:        time.Time{},
		UpdatedAt:        time.Time{},
	}
	t.Run("create policy subscriber", func(t *testing.T) {
		sub, err := s.CreateSubscriber(context.Background(), subscriber)
		assert.NoError(t, err)
		assert.Equal(t, subscriber, subscriber, sub)
	})

	t.Run("get subscriber", func(t *testing.T) {
		sub, err := s.Subscriber(context.Background(),
			subscriber.PolicyRepository,
			subscriber.PolicyGroup,
			subscriber.PolicyName,
			subscriber.PolicyVersion,
			subscriber.WebhookURL,
			subscriber.Name)
		assert.NoError(t, err)
		assert.Equal(t, subscriber, sub)
	})

	t.Run("get subscriber return error", func(t *testing.T) {
		sub, err := s.Subscriber(context.Background(),
			subscriber.PolicyRepository,
			subscriber.PolicyGroup,
			subscriber.PolicyName,
			subscriber.PolicyVersion,
			subscriber.WebhookURL,
			"not existing name")
		assert.ErrorContains(t, err, "subscriber not found in memory storage")
		assert.Nil(t, sub)
	})
}

// makePolicies makes a valid policies map
func makePolicies() map[string]*storage.Policy {
	return map[string]*storage.Policy{
		"policies,example,foo,1.0": {
			Repository: "policies",
			Name:       "foo",
			Group:      "example",
			Version:    "1.0",
			Locked:     false,
		},
		"policies,example,bar,1.1": {
			Repository: "policies",
			Name:       "bar",
			Group:      "example",
			Version:    "1.1",
			Locked:     true,
		},
		"policies,example,examplePolicy,1.1": {
			Repository: "policies",
			Name:       "examplePolicy",
			Group:      "example",
			Version:    "1.1",
			Locked:     false,
		},
	}
}

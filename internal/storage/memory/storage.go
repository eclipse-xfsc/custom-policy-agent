package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

//go:generate counterfeiter . KeyConstructor

type KeyConstructor interface {
	ConstructKey(repo, group, name, version string) string
}

type Storage struct {
	keyConstructor KeyConstructor
	subscribers    []storage.PolicySubscriber
	changes        chan storage.Policy

	mu       sync.RWMutex
	policies map[string]*storage.Policy

	muSubscribers     sync.RWMutex
	policySubscribers map[string]*storage.Subscriber

	muCommonStorage sync.RWMutex
	commonStorage   map[string]interface{}

	muAutoImport sync.RWMutex
	autoImport   map[string]*storage.PolicyAutoImport

	logger *zap.Logger
}

func New(c KeyConstructor, p map[string]*storage.Policy, l *zap.Logger) *Storage {
	ch := make(chan storage.Policy)

	return &Storage{
		keyConstructor:    c,
		changes:           ch,
		policies:          p,
		policySubscribers: map[string]*storage.Subscriber{},
		commonStorage:     map[string]interface{}{},
		logger:            l,
	}
}

func (s *Storage) Policy(_ context.Context, repository, group, name, version string) (*storage.Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.policies[s.keyConstructor.ConstructKey(repository, group, name, version)]
	if !ok {
		return nil, errors.New(errors.NotFound, "policy not found in memory storage")
	}

	res := *p // don't return the Policy by reference
	return &res, nil
}

func (s *Storage) SavePolicy(ctx context.Context, policy *storage.Policy) error {
	key := s.keyConstructor.ConstructKey(
		policy.Repository,
		policy.Group,
		policy.Name,
		policy.Version,
	)

	s.mu.Lock()
	s.policies[key] = policy
	s.mu.Unlock()

	// send the changed policy to subscribers
	go func(policy *storage.Policy) {
		select {
		case s.changes <- *policy:
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
		}
	}(policy)

	return nil
}

func (s *Storage) SetPolicyLock(ctx context.Context, repository, group, name, version string, lock bool) error {
	key := s.keyConstructor.ConstructKey(repository, group, name, version)

	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.policies[key]
	if !ok {
		return errors.New(errors.NotFound, "policy not found in memory storage")
	}

	p.Locked = lock

	// send the changed policy to subscribers
	go func(policy *storage.Policy) {
		select {
		case s.changes <- *policy:
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
		}
	}(p)

	return nil
}

func (s *Storage) GetPolicies(_ context.Context, locked *bool, policyName *string) ([]*storage.Policy, error) {
	var res []*storage.Policy

	s.mu.RLock()
	defer s.mu.RUnlock()

	var name string
	if policyName != nil {
		name = strings.ToLower(strings.TrimSpace(*policyName))
	}

	for _, p := range s.policies {
		if locked != nil && *locked != p.Locked {
			continue
		}

		if policyName != nil && !strings.Contains(strings.ToLower(p.Name), name) {
			continue
		}

		cpy := *p
		res = append(res, &cpy)
	}

	return res, nil
}

func (s *Storage) GetRefreshPolicies(_ context.Context) ([]*storage.Policy, error) {
	var res []*storage.Policy

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.policies {
		if p.NextDataRefreshTime.Before(time.Now()) && p.NextDataRefreshTime.After(time.Time{}) {
			cpy := *p
			res = append(res, &cpy)

			// postpone next refresh time for this policy
			s.policies[i].NextDataRefreshTime = time.Now().Add(storage.RefreshPostponePeriod)
		}
	}

	return res, nil
}

func (s *Storage) UpdateNextRefreshTime(_ context.Context, p *storage.Policy, nextDataRefreshTime time.Time) error {
	key := s.keyConstructor.ConstructKey(p.Repository, p.Group, p.Name, p.Version)

	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.policies[key]
	if !ok {
		return errors.New(errors.NotFound, "policy not found in memory storage")
	}

	s.policies[key].NextDataRefreshTime = nextDataRefreshTime

	return nil
}

func (s *Storage) AddPolicySubscribers(subscribers ...storage.PolicySubscriber) {
	s.subscribers = subscribers
}

func (s *Storage) ListenPolicyDataChanges(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p := <-s.changes:
			for _, subscriber := range s.subscribers {
				err := subscriber.PolicyDataChange(
					ctx,
					p.Repository,
					p.Name,
					p.Version,
					p.Group,
				)
				if err != nil {
					return err
				}
			}

			s.logger.Info("memory policy data changed")
		}
	}
}

func (s *Storage) GetData(_ context.Context, key string) (any, error) {
	s.muCommonStorage.Lock()
	defer s.muCommonStorage.Unlock()

	data, ok := s.commonStorage[key]
	if !ok {
		return nil, fmt.Errorf("key: %s doesn't exist", key)
	}

	return data, nil
}

func (s *Storage) SetData(_ context.Context, key string, data map[string]interface{}) error {
	s.muCommonStorage.Lock()
	defer s.muCommonStorage.Unlock()

	s.commonStorage[key] = data

	return nil
}

func (s *Storage) DeleteData(_ context.Context, key string) error {
	s.muCommonStorage.Lock()
	defer s.muCommonStorage.Unlock()

	if _, ok := s.commonStorage[key]; !ok {
		return fmt.Errorf("key: %s doesn't exist", key)
	}

	delete(s.commonStorage, key)

	return nil
}

func (s *Storage) Close(_ context.Context) {}

func (s *Storage) CreateSubscriber(_ context.Context, sub *storage.Subscriber) (*storage.Subscriber, error) {
	s.muSubscribers.RLock()
	defer s.muSubscribers.RUnlock()

	s.policySubscribers[sub.PolicyRepository+sub.PolicyGroup+sub.PolicyName+sub.PolicyVersion+sub.WebhookURL+sub.Name] = sub

	res := *sub // don't return the Subscriber by reference
	return &res, nil
}

func (s *Storage) Subscriber(_ context.Context, policyRepository, policyGroup, policyName, policyVersion, webhookURL, name string) (*storage.Subscriber, error) {
	s.muSubscribers.RLock()
	defer s.muSubscribers.RUnlock()
	subscriber, ok := s.policySubscribers[policyRepository+policyGroup+policyName+policyVersion+webhookURL+name]
	if !ok {
		return nil, errors.New(errors.NotFound, "subscriber not found in memory storage")
	}

	res := *subscriber // don't return the Subscriber by reference
	return &res, nil
}

func (s *Storage) SaveAutoImportConfig(_ context.Context, importConfig *storage.PolicyAutoImport) error {
	s.muAutoImport.Lock()
	s.autoImport[importConfig.PolicyURL] = importConfig
	s.muAutoImport.Unlock()

	return nil
}

func (s *Storage) ActiveImportConfigs(_ context.Context) ([]*storage.PolicyAutoImport, error) {
	s.muAutoImport.Lock()
	defer s.muAutoImport.Unlock()

	var active []*storage.PolicyAutoImport
	for key, cfg := range s.autoImport {
		if cfg.NextImport.After(time.Now()) {
			c := *cfg
			c.NextImport = c.NextImport.Add(c.Interval)
			s.autoImport[key] = &c
			active = append(active, &c)
		}
	}

	return active, nil
}

func (s *Storage) AutoImportConfigs(_ context.Context) ([]*storage.PolicyAutoImport, error) {
	s.muAutoImport.RLock()
	defer s.muAutoImport.RUnlock()

	var configs []*storage.PolicyAutoImport
	for _, cfg := range s.autoImport {
		c := *cfg
		configs = append(configs, &c)
	}

	return configs, nil
}

func (s *Storage) AutoImportConfig(_ context.Context, policyURL string) (*storage.PolicyAutoImport, error) {
	s.muAutoImport.RLock()
	defer s.muAutoImport.RUnlock()

	for _, cfg := range s.autoImport {
		if cfg.PolicyURL == policyURL {
			return cfg, nil
		}
	}

	return nil, errors.New(errors.NotFound)
}

func (s *Storage) DeleteAutoImportConfig(_ context.Context, policyURL string) error {
	s.muAutoImport.Lock()
	defer s.muAutoImport.Unlock()
	delete(s.autoImport, policyURL)

	return nil
}

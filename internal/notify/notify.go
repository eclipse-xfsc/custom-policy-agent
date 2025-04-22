package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

//go:generate counterfeiter . Events
//go:generate counterfeiter . Storage

type Events interface {
	Send(ctx context.Context, data any) error
}

type Storage interface {
	PolicySubscribers(ctx context.Context, policyRepository, policyName, policyGroup, policyVersion string) ([]*storage.Subscriber, error)
}

type Notifier struct {
	events  Events
	storage Storage
	client  *http.Client
	logger  *zap.Logger
}

type EventPolicyChange struct {
	Repository string `json:"repository"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Group      string `json:"group"`
}

// New creates a policy change notifier for interested subscribers.
// It can notify for policy changes both via MessageQueue or Web hooks.
func New(events Events, storage Storage, client *http.Client, logger *zap.Logger) *Notifier {
	return &Notifier{events: events, storage: storage, client: client, logger: logger}
}

// TODO unit testing PolicyDataChange

// PolicyDataChange is called when the policies source code or data are updated
// in storage. The function will notify subscribers of the given changes.
func (n *Notifier) PolicyDataChange(ctx context.Context, policyRepository, policyName, policyGroup, policyVersion string) error {
	logger := n.logger.With(zap.String("operation", "PolicyDataChange"))

	event := &EventPolicyChange{
		Repository: policyRepository,
		Name:       policyName,
		Version:    policyVersion,
		Group:      policyGroup,
	}

	go func() {
		err := n.notifySubscribers(ctx, event)
		if err != nil {
			logger.Error("error notifying subscribers", zap.Error(err))
		}
	}()

	return n.events.Send(ctx, event)
}

func (n *Notifier) notifySubscribers(ctx context.Context, event *EventPolicyChange) error {
	subscribers, err := n.storage.PolicySubscribers(ctx, event.Repository, event.Name, event.Group, event.Version)
	if err != nil {
		return err
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	for _, subscriber := range subscribers {
		if err := n.notifySubscriber(ctx, subscriber, eventJSON); err != nil {
			n.logger.Error("error notifying subscriber webhook",
				zap.Error(err),
				zap.String("subscriber", subscriber.Name),
				zap.String("webhookURL", subscriber.WebhookURL),
			)
		}
	}

	return nil
}

func (n *Notifier) notifySubscriber(ctx context.Context, subscriber *storage.Subscriber, eventJSON []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, subscriber.WebhookURL, bytes.NewBuffer(eventJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.New(errors.GetKind(res.StatusCode), getErrorBody(res))
	}

	return nil
}

func getErrorBody(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ""
	}
	return string(body)
}

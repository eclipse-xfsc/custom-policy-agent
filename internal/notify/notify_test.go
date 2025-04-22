package notify_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/notify"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/notify/notifyfakes"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
)

func TestNotify_PolicyDataChange(t *testing.T) {
	tests := []struct {
		name              string
		events            notify.Events
		storage           notify.Storage
		eventPolicyChange *notify.EventPolicyChange

		errText    string
		errLogText string
	}{
		{
			name:              "error when sending event",
			eventPolicyChange: &notify.EventPolicyChange{Repository: "exampleRepo", Name: "exampleName", Version: "exampleVersion", Group: "exampleGroup"},
			storage: &notifyfakes.FakeStorage{PolicySubscribersStub: func(ctx context.Context, s1, s2, s3, s4 string) ([]*storage.Subscriber, error) {
				return []*storage.Subscriber{}, nil
			}},
			events: &notifyfakes.FakeEvents{SendStub: func(ctx context.Context, a any) error {
				return fmt.Errorf("some error")
			}},

			errText: "some error",
		},

		{
			name:              "sending event is successful",
			eventPolicyChange: &notify.EventPolicyChange{Repository: "exampleRepo", Name: "exampleName", Version: "exampleVersion", Group: "exampleGroup"},
			storage: &notifyfakes.FakeStorage{PolicySubscribersStub: func(ctx context.Context, s1, s2, s3, s4 string) ([]*storage.Subscriber, error) {
				return []*storage.Subscriber{}, nil
			}},
			events: &notifyfakes.FakeEvents{SendStub: func(ctx context.Context, a any) error {
				return nil
			}},
		},

		{
			name:              "storage return error",
			eventPolicyChange: &notify.EventPolicyChange{Repository: "exampleRepo", Name: "exampleName", Version: "exampleVersion", Group: "exampleGroup"},
			storage: &notifyfakes.FakeStorage{PolicySubscribersStub: func(ctx context.Context, s1, s2, s3, s4 string) ([]*storage.Subscriber, error) {
				return []*storage.Subscriber{}, fmt.Errorf("some error")
			}},
			events: &notifyfakes.FakeEvents{SendStub: func(ctx context.Context, a any) error {
				return nil
			}},

			errLogText: "error notifying subscribers",
		},

		{
			name:              "wrong webhook url return error",
			eventPolicyChange: &notify.EventPolicyChange{Repository: "exampleRepo", Name: "exampleName", Version: "exampleVersion", Group: "exampleGroup"},
			storage: &notifyfakes.FakeStorage{PolicySubscribersStub: func(ctx context.Context, s1, s2, s3, s4 string) ([]*storage.Subscriber, error) {
				return []*storage.Subscriber{{WebhookURL: "wrong/url"}}, nil
			}},
			events: &notifyfakes.FakeEvents{SendStub: func(ctx context.Context, a any) error {
				return nil
			}},

			errLogText: "error notifying subscriber webhook",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core, logs := observer.New(zap.ErrorLevel)
			logger := zap.New(core)

			notifier := notify.New(test.events, test.storage, http.DefaultClient, logger)
			err := notifier.PolicyDataChange(context.Background(),
				test.eventPolicyChange.Repository,
				test.eventPolicyChange.Name,
				test.eventPolicyChange.Group,
				test.eventPolicyChange.Version)

			// we need to sleep a little, as notifier.PolicyDataChange(...)
			// spawns a new go routine which needs some time to start and execute
			time.Sleep(10 * time.Millisecond)

			if test.errLogText != "" {
				assert.Contains(t, logs.All()[0].Message, test.errLogText)
			}

			if test.errText != "" {
				assert.ErrorContains(t, err, test.errText)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

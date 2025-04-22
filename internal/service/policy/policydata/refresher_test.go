package policydata_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/policy/policydata"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/policy/policydata/policydatafakes"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func Test_Execute(t *testing.T) {
	tests := []struct {
		// test input
		name       string
		statusCode int
		policy     *storage.Policy
		storage    policydata.Storage
		// expected result
		logCnt   int
		firstLog string
	}{
		{
			name:   "invalid data configuration",
			policy: &storage.Policy{DataConfig: "<invalid data configuration>"},
			storage: &policydatafakes.FakeStorage{
				UpdateNextRefreshTimeStub: func(ctx context.Context, policy *storage.Policy, t time.Time) error {
					return nil
				},
			},
			logCnt:   1,
			firstLog: "error unmarshalling data configuration",
		},
		{
			name:   "data configuration is missing required fields",
			policy: &storage.Policy{DataConfig: `{"url": "https://example.com"}`},
			storage: &policydatafakes.FakeStorage{
				UpdateNextRefreshTimeStub: func(ctx context.Context, policy *storage.Policy, t time.Time) error {
					return nil
				},
			},
			logCnt:   1,
			firstLog: "required fields are missing in data configuration",
		},
		{
			name:   "error making an http request",
			policy: &storage.Policy{DataConfig: `{"url": "htt//example.com", "method": "GET", "period": "1h"}`},
			storage: &policydatafakes.FakeStorage{
				UpdateNextRefreshTimeStub: func(ctx context.Context, policy *storage.Policy, t time.Time) error {
					return nil
				},
			},
			logCnt:   1,
			firstLog: "error making a data refresh request",
		},
		{
			name:       "unexpected response code",
			statusCode: 500,
			policy:     &storage.Policy{DataConfig: `{"url": "https://example.com", "method": "GET", "period": "1h"}`},
			storage: &policydatafakes.FakeStorage{
				UpdateNextRefreshTimeStub: func(ctx context.Context, policy *storage.Policy, t time.Time) error {
					return nil
				},
			},
			logCnt:   1,
			firstLog: "unexpected response on data refresh request",
		},
		{
			name:   "error updating data after successful refresh request",
			policy: &storage.Policy{DataConfig: `{"url": "https://example.com", "method": "GET", "period": "1h"}`},
			storage: &policydatafakes.FakeStorage{
				UpdateNextRefreshTimeStub: func(ctx context.Context, policy *storage.Policy, t time.Time) error {
					return errors.New("storage error")
				},
			},
			logCnt:   1,
			firstLog: "error updating data after successful refresh request",
		},
		{
			name:   "data refresh is successfully executed",
			policy: &storage.Policy{DataConfig: `{"url": "https://example.com", "method": "GET", "period": "1h"}`},
			storage: &policydatafakes.FakeStorage{
				UpdateNextRefreshTimeStub: func(ctx context.Context, policy *storage.Policy, t time.Time) error {
					return nil
				},
			},
			logCnt: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observedZapCore, observedLogs := observer.New(zap.ErrorLevel)
			logger := zap.New(observedZapCore)
			httpClient := http.DefaultClient
			if test.statusCode != 0 {
				httpClient = NewTestClient(func(req *http.Request) *http.Response {
					return &http.Response{
						StatusCode: test.statusCode,
					}
				})
			}
			refresher := policydata.NewRefresher(test.storage, time.Duration(0), httpClient, logger)
			refresher.Execute(context.Background(), test.policy)

			assert.Equal(t, test.logCnt, observedLogs.Len())
			if observedLogs.Len() > 0 {
				firstLog := observedLogs.All()[0]
				assert.Equal(t, test.firstLog, firstLog.Message)
			}
		})
	}
}

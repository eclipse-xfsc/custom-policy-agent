package policydata

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

//go:generate counterfeiter . Storage

type Storage interface {
	GetRefreshPolicies(ctx context.Context) ([]*storage.Policy, error)
	UpdateNextRefreshTime(ctx context.Context, p *storage.Policy, nextDataRefreshTime time.Time) error
}

type Refresher struct {
	storage      Storage
	pollInterval time.Duration

	httpClient *http.Client
	logger     *zap.Logger
}

func NewRefresher(
	storage Storage,
	pollInterval time.Duration,
	httpClient *http.Client,
	logger *zap.Logger,
) *Refresher {
	return &Refresher{
		storage:      storage,
		pollInterval: pollInterval,
		httpClient:   httpClient,
		logger:       logger,
	}
}

func (e *Refresher) Start(ctx context.Context) error {
	defer e.logger.Info("policy data refresher stopped")

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-time.After(e.pollInterval):
			policies, err := e.storage.GetRefreshPolicies(ctx)
			if err != nil {
				if !errors.Is(errors.NotFound, err) {
					e.logger.Error("error getting policies for data refresh from storage", zap.Error(err))
				}
				continue
			}
			for _, policy := range policies {
				e.Execute(ctx, policy)
			}
		}
	}

	return ctx.Err()
}

func (e *Refresher) Execute(ctx context.Context, p *storage.Policy) {
	logger := e.logger.With(
		zap.String("policyRepository", p.Repository),
		zap.String("policyName", p.Name),
		zap.String("policyGroup", p.Group),
		zap.String("policyVersion", p.Version),
	)

	var config DataConfig
	if err := json.Unmarshal([]byte(p.DataConfig), &config); err != nil {
		// data configuration is corrupted, set next refresh time to Go's zero date
		_ = e.storage.UpdateNextRefreshTime(ctx, p, time.Time{})
		logger.Error("error unmarshalling data configuration", zap.Error(err))
		return
	}
	if config.URL == "" || config.Period == Duration(0) || config.Method == "" {
		// data configuration is missing required fields, set next refresh time to Go's zero date
		_ = e.storage.UpdateNextRefreshTime(ctx, p, time.Time{})
		logger.Error("required fields are missing in data configuration")
		return
	}

	req, err := e.createHTTPRequest(ctx, &config)
	if err != nil {
		// cannot create a request, set next refresh time to Go's zero date
		_ = e.storage.UpdateNextRefreshTime(ctx, p, time.Time{})
		logger.Error("error creating an http request", zap.Error(err))
		return
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		// making data configuration request failed, set next refresh time to current time added data config's period
		_ = e.storage.UpdateNextRefreshTime(ctx, p, time.Now().Add(time.Duration(config.Period)))
		logger.Error("error making a data refresh request", zap.Error(err))
		return
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		// unexpected response on data refresh request, set next refresh time to current time added data config's period
		_ = e.storage.UpdateNextRefreshTime(ctx, p, time.Now().Add(time.Duration(config.Period)))
		logger.Error("unexpected response on data refresh request", zap.Int("response code", resp.StatusCode))
		return
	}

	dataBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// error reading response from data refresh request, set next refresh time to current time added data config's period
		_ = e.storage.UpdateNextRefreshTime(ctx, p, time.Now().Add(time.Duration(config.Period)))
		logger.Error("error reading response from data refresh request", zap.Error(err))
		return
	}

	p.Data = string(dataBytes)
	if err = e.storage.UpdateNextRefreshTime(ctx, p, time.Now().Add(time.Duration(config.Period))); err != nil {
		logger.Error("error updating data after successful refresh request", zap.Error(err))
		return
	}
	logger.Debug("data refresh is successfully executed")
}

func (e *Refresher) createHTTPRequest(ctx context.Context, config *DataConfig) (*http.Request, error) {
	bodyBytes, err := json.Marshal(config.Body)
	if err != nil {
		return nil, errors.New("error marshaling data configuration body")
	}

	url, err := url.Parse(config.URL)
	if err != nil {
		return nil, errors.New("invalid data configuration url")
	}
	if url.Scheme == "" {
		url.Scheme = "https"
	}

	if config.Method == http.MethodPost {
		return http.NewRequestWithContext(ctx, config.Method, url.String(), bytes.NewReader(bodyBytes))
	}
	return http.NewRequestWithContext(ctx, config.Method, url.String(), nil)
}

package policy

import (
	"context"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

func (s *Service) StartAutoImporter(ctx context.Context, pollInterval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			s.logger.Error("auto importer process stopped", zap.Error(ctx.Err()))
			return
		case <-time.After(pollInterval):
			importConfigs, err := s.storage.ActiveImportConfigs(ctx)
			if err != nil {
				s.logger.Error("error getting auto import configurations", zap.Error(err))
				continue
			}

			if len(importConfigs) == 0 {
				continue
			}

			count, err := s.doImport(ctx, importConfigs)
			if err != nil {
				s.logger.Error("policy bundle automatic import", zap.Error(err))
				continue
			}

			s.logger.Debug("automatic policy import completed", zap.Int("importCount", count))
		}
	}
}

func (s *Service) doImport(ctx context.Context, importConfigs []*storage.PolicyAutoImport) (int, error) {
	var imported int
	for _, i := range importConfigs {
		bundleReader, err := s.fetchBundle(ctx, i.PolicyURL)
		if err != nil {
			s.logger.Error("error on fetch policy bundle", zap.Error(err))
			continue
		}

		_, err = s.ImportBundle(ctx, nil, bundleReader)
		if err != nil {
			s.logger.Error("failed to import policy bundle", zap.Error(err))
			continue
		}

		imported++
	}

	return imported, nil
}

func (s *Service) fetchBundle(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(errors.GetKind(resp.StatusCode), getErrorBody(resp))
	}

	return resp.Body, nil
}

func getErrorBody(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ""
	}
	return string(body)
}

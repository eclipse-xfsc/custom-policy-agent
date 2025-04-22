package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

// Client for the Cache service.
type Client struct {
	addr       string
	httpClient *http.Client
}

func New(addr string, opts ...Option) *Client {
	c := &Client{
		addr:       addr,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) Set(ctx context.Context, key, namespace, scope string, value []byte, ttl int) error {
	if c.addr == "" {
		return errors.New(errors.ServiceUnavailable, "trying to use cache service, but address is not set")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.addr+"/v1/cache", bytes.NewReader(value))
	if err != nil {
		return err
	}

	req.Header = http.Header{
		"x-cache-key":       []string{key},
		"x-cache-namespace": []string{namespace},
		"x-cache-scope":     []string{scope},
	}
	if ttl != 0 {
		req.Header.Add("x-cache-ttl", strconv.Itoa(ttl))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("unexpected response: %d %s", resp.StatusCode, resp.Status)
		return errors.New(errors.GetKind(resp.StatusCode), msg)
	}

	return nil
}

func (c *Client) Get(ctx context.Context, key, namespace, scope string) ([]byte, error) {
	if c.addr == "" {
		return nil, errors.New(errors.ServiceUnavailable, "trying to use cache service, but address is not set")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.addr+"/v1/cache", nil)
	req.Header = http.Header{
		"x-cache-key":       []string{key},
		"x-cache-namespace": []string{namespace},
		"x-cache-scope":     []string{scope},
	}
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.New(errors.NotFound)
		}
		msg := fmt.Sprintf("unexpected response: %d %s", resp.StatusCode, resp.Status)
		return nil, errors.New(errors.GetKind(resp.StatusCode), msg)
	}

	return io.ReadAll(resp.Body)
}

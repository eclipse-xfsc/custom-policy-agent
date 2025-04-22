package signer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

type signRequest struct {
	Key       string `json:"key"`
	Namespace string `json:"namespace"`
	Data      string `json:"data"`
}

type signResult struct {
	Signature string `json:"signature"`
}

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

func (c *Client) Sign(ctx context.Context, namespace, key string, data []byte) ([]byte, error) {
	if c.addr == "" {
		return nil, errors.New(errors.ServiceUnavailable, "signer address is not set")
	}

	r := &signRequest{
		Key:       key,
		Namespace: namespace,
		Data:      base64.StdEncoding.EncodeToString(data),
	}

	payload, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.addr+"/v1/sign", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(errors.GetKind(resp.StatusCode), getErrorBody(resp))
	}

	var result signResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(result.Signature)
}

func (c *Client) Key(ctx context.Context, namespace string, key string) (any, error) {
	if c.addr == "" {
		return nil, errors.New(errors.ServiceUnavailable, "signer address is not set")
	}

	keyPath := fmt.Sprintf("/v1/jwk/%s/%s", namespace, key)
	req, err := http.NewRequestWithContext(ctx, "GET", c.addr+keyPath, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(errors.GetKind(resp.StatusCode), getErrorBody(resp))
	}

	var pubkey map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pubkey); err != nil {
		return nil, err
	}

	return pubkey, nil
}

func getErrorBody(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ""
	}
	return string(body)
}

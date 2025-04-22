package ocm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	proofOutOfBandPath        = "/v1/out-of-band-proof"
	proofOutOfBandRequestPath = "/v1/send-out-of-band-presentation-request"
	proofPresentationPath     = "/v1/find-by-presentation-id"
)

// Client is the OCM service client
type Client struct {
	proofManagerAddr string
	httpClient       *http.Client
}

// New initializes an OCM service client given the OCM service address
func New(proofManagerAddr string, opts ...Option) *Client {
	c := &Client{
		proofManagerAddr: proofManagerAddr,
		httpClient:       http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// GetLoginProofInvitation calls the "invitation" endpoint on
// the "out-of-band" protocol in the OCM.
func (c *Client) GetLoginProofInvitation(ctx context.Context, credTypes []string) (*LoginProofInvitationResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.proofManagerAddr+proofOutOfBandPath, nil)
	if err != nil {
		return nil, err
	}

	v := url.Values{}
	v.Add("type", strings.Join(credTypes, ","))
	req.URL.RawQuery = v.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response code: %s", resp.Status)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response LoginProofInvitationResponse
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// SendOutOfBandRequest calls the "send out of band presentation request" endpoint on
// the "out-of-band" protocol in the OCM.
func (c *Client) SendOutOfBandRequest(ctx context.Context, r map[string]interface{}) (*LoginProofInvitationResponse, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.proofManagerAddr+proofOutOfBandRequestPath, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response code: %s", resp.Status)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response LoginProofInvitationResponse
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetLoginProofResult calls the "find-by-presentation-id" endpoint in the OCM.
func (c *Client) GetLoginProofResult(ctx context.Context, presentationID string) (*LoginProofResultResponse, error) {
	resBytes, err := c.findByPresentationID(ctx, presentationID)
	if err != nil {
		return nil, err
	}

	var response LoginProofResultResponse
	if err := json.Unmarshal(resBytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetRawLoginProofResult calls the "find-by-presentation-id" endpoint in the OCM and returns the raw result.
func (c *Client) GetRawLoginProofResult(ctx context.Context, presentationID string) (map[string]interface{}, error) {
	resBytes, err := c.findByPresentationID(ctx, presentationID)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(resBytes, &response); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *Client) findByPresentationID(ctx context.Context, presentationID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.proofManagerAddr+proofPresentationPath, nil)
	if err != nil {
		return nil, err
	}

	v := url.Values{}
	v.Add("proofRecordId", presentationID)
	req.URL.RawQuery = v.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response code: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

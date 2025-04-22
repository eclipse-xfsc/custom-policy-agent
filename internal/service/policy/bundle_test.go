package policy

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
)

// should not be modified, read only
var testPolicy = &storage.Policy{
	Filename:   "example/mypolicy/1.0/policy.rego",
	Repository: "myrepo",
	Name:       "mypolicy",
	Group:      "example",
	Version:    "1.0",
	Rego:       "package test",
	Data:       `{"hello":"static data"}`,
	DataConfig: `{"cfg":"static data config"}`,
	Locked:     true,
	LastUpdate: time.Date(2023, 11, 7, 1, 0, 0, 0, time.UTC),
}

var testMetadata = Metadata{
	Policy: struct {
		Name       string    `json:"name"`
		Group      string    `json:"group"`
		Version    string    `json:"version"`
		Repository string    `json:"repository"`
		Locked     bool      `json:"locked"`
		LastUpdate time.Time `json:"lastUpdate"`
	}{
		Name:       "mypolicy",
		Group:      "example",
		Version:    "1.0",
		Repository: "myrepo",
		Locked:     true,
		LastUpdate: time.Date(2023, 11, 7, 1, 0, 0, 0, time.UTC),
	},
	PublicKeyURL: "https://policyservice.com/policy/myrepo/example/mypolicy/1.0/key",
}

func TestPolicy_createPolicyBundle(t *testing.T) {
	svc := New(context.Background(), nil, nil, nil, nil, "https://policyservice.com", false, 10*time.Second, http.DefaultClient, zap.NewNop())
	bundle, err := svc.createPolicyBundle(testPolicy)
	assert.NoError(t, err)
	assert.NotNil(t, bundle)

	reader, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	// check metadata
	require.NotNil(t, reader.File[0])
	require.Equal(t, "metadata.json", reader.File[0].Name)
	metaFile, err := reader.File[0].Open()
	require.NoError(t, err)

	var meta Metadata
	err = json.NewDecoder(metaFile).Decode(&meta)
	require.NoError(t, err)
	assert.Equal(t, testMetadata, meta)

	// check policy source code
	assert.NotNil(t, reader.File[1])
	assert.Equal(t, "policy.rego", reader.File[1].Name)
	sourceFile, err := reader.File[1].Open()
	require.NoError(t, err)
	source, err := io.ReadAll(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, "package test", string(source))

	// check static data
	assert.NotNil(t, reader.File[2])
	assert.Equal(t, "data.json", reader.File[2].Name)
	dataFile, err := reader.File[2].Open()
	require.NoError(t, err)
	data, err := io.ReadAll(dataFile)
	require.NoError(t, err)
	assert.Equal(t, `{"hello":"static data"}`, string(data))

	// check static data configuration
	assert.NotNil(t, reader.File[3])
	assert.Equal(t, "data-config.json", reader.File[3].Name)
	dataConfigFile, err := reader.File[3].Open()
	require.NoError(t, err)
	dataConfig, err := io.ReadAll(dataConfigFile)
	require.NoError(t, err)
	assert.Equal(t, `{"cfg":"static data config"}`, string(dataConfig))
}

func TestPolicy_policyFromBundle(t *testing.T) {
	svc := New(context.Background(), nil, nil, nil, nil, "https://policyservice.com", false, 10*time.Second, http.DefaultClient, zap.NewNop())
	bundle, err := svc.createPolicyBundle(testPolicy)
	require.NoError(t, err)
	require.NotNil(t, bundle)

	policy, err := svc.policyFromBundle(bundle)
	require.NoError(t, err)
	require.NotNil(t, policy)
	assert.Equal(t, testPolicy, policy)
}

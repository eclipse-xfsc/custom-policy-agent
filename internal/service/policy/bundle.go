package policy

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

var errExportConfigNotFound = fmt.Errorf("policy export configuration is not defined")

type ZipFile struct {
	Name    string
	Content []byte
}

type Metadata struct {
	Policy struct {
		Name       string    `json:"name"`
		Group      string    `json:"group"`
		Version    string    `json:"version"`
		Repository string    `json:"repository"`
		Locked     bool      `json:"locked"`
		LastUpdate time.Time `json:"lastUpdate"`
	} `json:"policy"`
	PublicKeyURL string `json:"publicKeyURL"`
}

// exportConfig specifies which signer namespace and key to be used
// when exporting policy as signed bundle.
type exportConfig struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

func (s *Service) createPolicyBundle(policy *storage.Policy) ([]byte, error) {
	var files []ZipFile

	// prepare metadata
	metadata, err := s.createMetadata(policy)
	if err != nil {
		return nil, err
	}

	files = append(files, ZipFile{
		Name:    "metadata.json",
		Content: metadata,
	})

	// prepare source code
	files = append(files, ZipFile{
		Name:    "policy.rego",
		Content: []byte(policy.Rego),
	})

	// prepare static data file
	if strings.TrimSpace(policy.Data) != "" {
		files = append(files, ZipFile{
			Name:    "data.json",
			Content: []byte(policy.Data),
		})
	}

	// prepare static data configuration file
	if strings.TrimSpace(policy.DataConfig) != "" {
		files = append(files, ZipFile{
			Name:    "data-config.json",
			Content: []byte(policy.DataConfig),
		})
	}

	// prepare json schema config file
	if strings.TrimSpace(policy.OutputSchema) != "" {
		files = append(files, ZipFile{
			Name:    "output-schema.json",
			Content: []byte(policy.OutputSchema),
		})
	}

	// prepare json schema config file
	if strings.TrimSpace(policy.ExportConfig) != "" {
		files = append(files, ZipFile{
			Name:    "export-config.json",
			Content: []byte(policy.ExportConfig),
		})
	}

	return s.createZipArchive(files)
}

func (s *Service) createMetadata(policy *storage.Policy) ([]byte, error) {
	var meta Metadata
	meta.Policy.Name = policy.Name
	meta.Policy.Group = policy.Group
	meta.Policy.Version = policy.Version
	meta.Policy.Repository = policy.Repository
	meta.Policy.Locked = policy.Locked
	meta.Policy.LastUpdate = policy.LastUpdate
	meta.PublicKeyURL = s.policyPublicKeyURL(policy)
	return json.Marshal(meta)
}

func (s *Service) createZipArchive(files []ZipFile) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	for _, file := range files {
		f, err := zipWriter.Create(file.Name)
		if err != nil {
			return nil, err
		}

		_, err = f.Write(file.Content)
		if err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *Service) unzip(archive []byte) ([]ZipFile, error) {
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}

	var files []ZipFile
	for _, file := range r.File {
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		files = append(files, ZipFile{
			Name:    file.Name,
			Content: content,
		})
	}

	return files, nil
}

func (s *Service) policyPublicKeyURL(policy *storage.Policy) string {
	return fmt.Sprintf("%s/policy/%s/%s/%s/%s/key",
		s.externalHostname,
		policy.Repository,
		policy.Group,
		policy.Name,
		policy.Version,
	)
}

func (s *Service) policyFromBundle(bundle []byte) (*storage.Policy, error) {
	bundleFiles, err := s.unzip(bundle)
	if err != nil {
		return nil, errors.New("error unzipping bundle archive", err)
	}

	var policy storage.Policy
	for _, f := range bundleFiles {
		switch f.Name {
		case "metadata.json":
			var metadata Metadata
			if err := json.Unmarshal(f.Content, &metadata); err != nil {
				return nil, err
			}
			policy.Repository = metadata.Policy.Repository
			policy.Group = metadata.Policy.Group
			policy.Name = metadata.Policy.Name
			policy.Version = metadata.Policy.Version
			policy.Locked = metadata.Policy.Locked
			policy.LastUpdate = metadata.Policy.LastUpdate
			policy.Filename = policy.Group + "/" + policy.Name + "/" + policy.Version + "/" + "policy.rego"
		case "policy.rego":
			policy.Rego = string(f.Content)
		case "data.json":
			policy.Data = string(f.Content)
		case "data-config.json":
			policy.DataConfig = string(f.Content)
		case "output-schema.json":
			policy.OutputSchema = string(f.Content)
		case "export-config.json":
			policy.ExportConfig = string(f.Content)
		}
	}

	return &policy, nil
}

func policyExportConfig(p *storage.Policy) (*exportConfig, error) {
	if strings.TrimSpace(p.ExportConfig) == "" {
		return nil, errExportConfigNotFound
	}

	var cfg exportConfig
	if err := json.Unmarshal([]byte(p.ExportConfig), &cfg); err != nil {
		return nil, fmt.Errorf("cannot unmarshal policy export configuration: %v", err)
	}

	return &cfg, nil
}

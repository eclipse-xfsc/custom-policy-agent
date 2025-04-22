// Package clone provides functions for cloning a GIT repository and
// extract REGO policies out of the cloned repository.
package clone

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
)

const (
	pathSeperator        = string(os.PathSeparator)
	policyFilename       = "policy.rego"
	cloneFolder          = "temp"
	dataFilename         = "data.json"
	dataConfigFilename   = "data-config.json"
	jsonSchemaFilename   = "output-schema.json"
	exportConfigFilename = "export-config.json"
)

type Cloner struct {
}

func New() (*Cloner, error) {
	c := &Cloner{}
	if err := c.Cleanup(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Cloner) Cleanup() error {
	return os.RemoveAll(cloneFolder)
}

// Clone clones a Policy repository to cloneFolder and returns
// the repository name
func (c *Cloner) Clone(ctx context.Context, cloneURL, user, pass, branch string) (string, error) {
	opts := &git.CloneOptions{
		URL:   cloneURL,
		Depth: 1,
	}

	if user != "" && pass != "" {
		opts.Auth = &http.BasicAuth{
			Username: user,
			Password: pass,
		}
	}

	if branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		opts.SingleBranch = true
	}

	_, err := git.PlainCloneContext(ctx, cloneFolder, false, opts)

	return getRepoName(cloneURL), err
}

// IterateRepo iterates over the repoFolder and returns a map
// of Policy structs
func (c *Cloner) IterateRepo(repoFolder, repository string) (map[string]*storage.Policy, error) {
	if repoFolder == "" {
		repoFolder = cloneFolder
	} else {
		repoFolder = filepath.Join(cloneFolder, repoFolder)
	}

	policies := make(map[string]*storage.Policy)
	err := filepath.WalkDir(repoFolder, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == policyFilename {
			policy, err := createPolicy(p, repository)
			if err != nil {
				return err
			}
			policies[c.ConstructKey(policy.Repository, policy.Group, policy.Name, policy.Version)] = policy
		}
		return nil
	})

	return policies, err
}

// createPolicy instantiates a Policy struct out of a policy file on given path
func createPolicy(p, repository string) (*storage.Policy, error) {
	ex, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("error getting executable path: %v", err)
	}

	exPath := filepath.Dir(ex)
	// path to Rego policy must be {group}/{name}/{version}/policy.rego
	// strings.Split on the path give us an array containing at least group, name, version and filename
	ss := strings.Split(p, pathSeperator)
	if len(ss) < 4 {
		return nil, fmt.Errorf("failed to get policy filename, name, version and group out of policy path: %s", p)
	}

	version := ss[len(ss)-2] // second last element is the version
	name := ss[len(ss)-3]    // third last element is the policy name
	group := ss[len(ss)-4]   // fourth last element is the policy group
	bytes, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	regoSrc := string(bytes)

	// generate policy filename for DB from pattern {group}/{name}/{version}/policy.rego
	dbFilename := group + "/" + name + "/" + version + "/" + policyFilename

	// check if there is a data.json file in the same folder as the policy
	dataBytes, err := os.ReadFile(filepath.Join(exPath, strings.TrimSuffix(p, policyFilename)+dataFilename))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// check if there is a data-config.json file in the same folder as the policy
	configBytes, err := os.ReadFile(strings.TrimSuffix(p, policyFilename) + dataConfigFilename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// if both data.json and data-config.json files exist, log a warning message
	if len(dataBytes) > 0 && len(configBytes) > 0 {
		log.Printf("[WARNING] policy data will be overwritten by a data configuration execution for policy %q, group %q and version %q\n", name, group, version)
	}

	// check if there is an output-schema.json file in the same folder as the policy
	schemaBytes, err := os.ReadFile(strings.TrimSuffix(p, policyFilename) + jsonSchemaFilename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// check if there is policy export configuration in the same folder as the policy
	exportConfigBytes, err := os.ReadFile(strings.TrimSuffix(p, policyFilename) + exportConfigFilename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return &storage.Policy{
		Repository:   repository,
		Filename:     dbFilename,
		Name:         name,
		Group:        group,
		Version:      version,
		Rego:         regoSrc,
		Data:         string(dataBytes),
		DataConfig:   string(configBytes),
		OutputSchema: string(schemaBytes),
		ExportConfig: string(exportConfigBytes),
		Locked:       false,
	}, nil
}

func (c *Cloner) ConstructKey(repo, group, name, version string) string {
	return fmt.Sprintf("%s.%s.%s.%s", repo, group, name, version)
}

// getRepoName returns the repository name out of a clone url
//
// Example: clone url - `https://gitlab.example.com/policy.git`; repository name - `policy`
func getRepoName(url string) string {
	ss := strings.Split(strings.TrimSuffix(url, ".git"), "/")

	return ss[len(ss)-1]
}

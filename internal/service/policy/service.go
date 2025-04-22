package policy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.uber.org/zap"

	"github.com/eclipse-xfsc/custom-policy-agent/gen/policy"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/header"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	ptr "github.com/eclipse-xfsc/microservice-core-go/pkg/ptr"
)

//go:generate counterfeiter . Cache
//go:generate counterfeiter . Storage
//go:generate counterfeiter . RegoCache
//go:generate counterfeiter . Signer

const (
	BundleFilename          = "policy_bundle.zip"
	BundleSignatureFilename = "signature.raw"
)

type Cache interface {
	Set(ctx context.Context, key, namespace, scope string, value []byte, ttl int) error
}

type RegoCache interface {
	Set(key string, policy *storage.Policy)
	Get(key string) (policy *storage.Policy, found bool)
}

type Signer interface {
	Key(ctx context.Context, namespace string, key string) (any, error)
	Sign(ctx context.Context, namespace string, key string, data []byte) ([]byte, error)
}

type Service struct {
	storage        Storage
	policyCache    RegoCache
	cache          Cache
	signer         Signer
	httpClient     *http.Client
	validationLock bool
	logger         *zap.Logger

	// externalHostname specifies the hostname where the policy service can be
	// reached from the public internet. This setting is very important for
	// export/import of policy bundles, as the policy service must include the
	// full path to its verification public key in the bundle, so that verifiers
	// can use it for signature verification.
	externalHostname string
}

func New(
	ctx context.Context,
	storage Storage,
	policyCache RegoCache,
	cache Cache,
	signer Signer,
	hostname string,
	validationLock bool,
	importPollInterval time.Duration,
	httpClient *http.Client,
	logger *zap.Logger,
) *Service {
	svc := &Service{
		storage:          storage,
		policyCache:      policyCache,
		cache:            cache,
		signer:           signer,
		httpClient:       httpClient,
		validationLock:   validationLock,
		logger:           logger,
		externalHostname: hostname,
	}

	// start process to auto import policy bundles
	go svc.StartAutoImporter(ctx, importPollInterval)

	return svc
}

// Evaluate executes a policy with the given input.
//
// Note: The policy must follow strict conventions so that such generic
// evaluation function could work: package declaration inside the policy must
// be exactly the same as 'group.policy'. For example:
// Evaluating the URL: `.../policies/mygroup/example/1.0/evaluation` will
// return results correctly, only if the package declaration inside the policy is:
// `package mygroup.example`.
func (s *Service) Evaluate(ctx context.Context, req *policy.EvaluateRequest) (*policy.EvaluateResult, error) {
	var evaluationID string
	if req.EvaluationID != nil && *req.EvaluationID != "" {
		evaluationID = *req.EvaluationID
	} else {
		evaluationID = uuid.NewString()
	}

	logger := s.logger.With(
		zap.String("operation", "evaluate"),
		zap.String("repository", req.Repository),
		zap.String("group", req.Group),
		zap.String("name", req.PolicyName),
		zap.String("version", req.Version),
		zap.String("evaluationID", evaluationID),
	)

	headers, _ := header.FromContext(ctx)
	query, err := s.prepareQuery(ctx, req.Repository, req.Group, req.PolicyName, req.Version, headers)
	if err != nil {
		logger.Error("error getting prepared query", zap.Error(err))
		return nil, errors.New("error evaluating policy", err)
	}

	resultSet, err := query.Eval(ctx, rego.EvalInput(req.Input))
	if err != nil {
		logger.Error("error evaluating rego query", zap.Error(err))
		return nil, errors.New("error evaluating rego query", err)
	}

	if len(resultSet) == 0 {
		logger.Error("policy evaluation results are empty")
		return nil, errors.New("policy evaluation results are empty")
	}

	if len(resultSet[0].Expressions) == 0 {
		logger.Error("policy evaluation result expressions are empty")
		return nil, errors.New("policy evaluation result expressions are empty")
	}

	// If there is only a single result from the policy evaluation and it was assigned to an empty
	// variable, then we'll return a custom response containing only the value of the empty variable
	// without any mapping.
	result := resultSet[0].Expressions[0].Value
	if resultMap, ok := result.(map[string]interface{}); ok {
		if len(resultMap) == 1 {
			for k, v := range resultMap {
				if k == "$0" {
					result = v
				}
			}
		}
	}

	jsonValue, err := json.Marshal(result)
	if err != nil {
		logger.Error("error encoding result to json", zap.Error(err))
		return nil, errors.New("error encoding result to json")
	}

	var ttl int
	if req.TTL != nil {
		ttl = *req.TTL
	}

	err = s.cache.Set(ctx, evaluationID, "", "", jsonValue, ttl)
	if err != nil {
		// if the cache service is not available, don't stop but continue with returning the result
		if !errors.Is(errors.ServiceUnavailable, err) {
			logger.Error("error storing policy result in cache", zap.Error(err))
			return nil, errors.New("error storing policy result in cache")
		}
	}

	return &policy.EvaluateResult{
		Result: result,
		ETag:   evaluationID,
	}, nil
}

// Validate executes a policy with given input and then validates the output against
// a predefined JSON schema.
func (s *Service) Validate(ctx context.Context, req *policy.EvaluateRequest) (*policy.EvaluateResult, error) {
	logger := s.logger.With(
		zap.String("operation", "validate"),
		zap.String("repository", req.Repository),
		zap.String("group", req.Group),
		zap.String("name", req.PolicyName),
		zap.String("version", req.Version),
	)

	// retrieve policy
	pol, err := s.retrievePolicy(ctx, req.Repository, req.Group, req.PolicyName, req.Version)
	if err != nil {
		logger.Error("error retrieving policy", zap.Error(err))
		return nil, errors.New("error retrieving policy", err)
	}

	if pol.OutputSchema == "" {
		logger.Error("validation schema for policy output is not found")
		return nil, errors.New(errors.BadRequest, "validation schema for policy output is not found")
	}

	// evaluate the policy and get the result
	res, err := s.Evaluate(ctx, req)
	if err != nil {
		return nil, err
	}

	// compile the validation schema
	sch, err := jsonschema.CompileString("", pol.OutputSchema)
	if err != nil {
		logger.Error("error compiling output validation schema", zap.Error(err))
		return nil, errors.New("error compiling output validation schema")
	}

	// validate the policy output
	if err := sch.Validate(res.Result); err != nil {
		// lock the policy for execution if configured
		if s.validationLock {
			if err := s.lock(ctx, pol); err != nil {
				logger.Error("error locking policy after validation failure", zap.Error(err))
			}
		}

		logger.Error("policy output schema validation failed", zap.Error(err))
		return nil, errors.New(errors.Unknown, "policy output schema validation failed", err)
	}

	return res, nil
}

// Lock a policy so that it cannot be evaluated.
func (s *Service) Lock(ctx context.Context, req *policy.LockRequest) error {
	logger := s.logger.With(
		zap.String("operation", "lock"),
		zap.String("repository", req.Repository),
		zap.String("group", req.Group),
		zap.String("name", req.PolicyName),
		zap.String("version", req.Version),
	)

	pol, err := s.storage.Policy(ctx, req.Repository, req.Group, req.PolicyName, req.Version)
	if err != nil {
		logger.Error("error getting policy from storage", zap.Error(err))
		if errors.Is(errors.NotFound, err) {
			return err
		}
		return errors.New("error locking policy", err)
	}

	if err := s.lock(ctx, pol); err != nil {
		logger.Error("error locking policy", zap.Error(err))
		return err
	}

	logger.Debug("policy is locked")

	return nil
}

func (s *Service) lock(ctx context.Context, p *storage.Policy) error {
	if p.Locked {
		return errors.New(errors.Forbidden, "policy is already locked")
	}

	if err := s.storage.SetPolicyLock(ctx, p.Repository, p.Group, p.Name, p.Version, true); err != nil {
		return errors.New("error locking policy", err)
	}

	return nil
}

// Unlock a policy so it can be evaluated again.
func (s *Service) Unlock(ctx context.Context, req *policy.UnlockRequest) error {
	logger := s.logger.With(
		zap.String("operation", "unlock"),
		zap.String("repository", req.Repository),
		zap.String("group", req.Group),
		zap.String("name", req.PolicyName),
		zap.String("version", req.Version),
	)

	pol, err := s.storage.Policy(ctx, req.Repository, req.Group, req.PolicyName, req.Version)
	if err != nil {
		logger.Error("error getting policy from storage", zap.Error(err))
		if errors.Is(errors.NotFound, err) {
			return err
		}
		return errors.New("error unlocking policy", err)
	}

	if !pol.Locked {
		return errors.New(errors.Forbidden, "policy is unlocked")
	}

	if err := s.storage.SetPolicyLock(ctx, req.Repository, req.Group, req.PolicyName, req.Version, false); err != nil {
		logger.Error("error unlocking policy", zap.Error(err))
		return errors.New("error unlocking policy", err)
	}

	logger.Debug("policy is unlocked")

	return nil
}

func (s *Service) ExportBundle(ctx context.Context, req *policy.ExportBundleRequest) (*policy.ExportBundleResult, io.ReadCloser, error) {
	logger := s.logger.With(
		zap.String("operation", "exportBundle"),
		zap.String("repository", req.Repository),
		zap.String("group", req.Group),
		zap.String("name", req.PolicyName),
		zap.String("version", req.Version),
	)

	pol, err := s.storage.Policy(ctx, req.Repository, req.Group, req.PolicyName, req.Version)
	if err != nil {
		logger.Error("error getting policy from storage", zap.Error(err))
		return nil, nil, err
	}

	exportConfig, err := policyExportConfig(pol)
	if err != nil {
		logger.Error(err.Error())
		if err == errExportConfigNotFound {
			return nil, nil, errors.New(errors.Forbidden, err)
		}
		return nil, nil, err
	}

	// bundle is the complete policy bundle zip file
	bundle, err := s.createPolicyBundle(pol)
	if err != nil {
		logger.Error("error creating policy bundle", zap.Error(err))
		return nil, nil, err
	}

	// only the sha256 file digest will be signed, not the file itself
	bundleDigest := sha256.Sum256(bundle)

	// signer namespace and key are taken from policy export configuration
	signature, err := s.signer.Sign(ctx, exportConfig.Namespace, exportConfig.Key, bundleDigest[:])
	if err != nil {
		logger.Error("error signing policy bundle", zap.Error(err))
		return nil, nil, err
	}

	// the final ZIP file that will be exported to the client wraps the policy bundle
	// zip file and the jws detached payload signature file
	var files = []ZipFile{
		{
			Name:    BundleFilename,
			Content: bundle,
		},
		{
			Name:    BundleSignatureFilename,
			Content: signature,
		},
	}

	signedBundle, err := s.createZipArchive(files)
	if err != nil {
		logger.Error("error making final zip with signature", zap.Error(err))
		return nil, nil, err
	}

	filename := fmt.Sprintf("%s_%s_%s_%s.zip", pol.Repository, pol.Group, pol.Name, pol.Version)
	filename = strings.TrimSpace(filename)

	return &policy.ExportBundleResult{
		ContentType:        "application/zip",
		ContentLength:      len(signedBundle),
		ContentDisposition: fmt.Sprintf(`attachment; filename="%s"`, filename),
	}, io.NopCloser(bytes.NewReader(signedBundle)), nil
}

// PolicyPublicKey returns the public key in JWK format which must be used to
// verify a signed policy bundle.
func (s *Service) PolicyPublicKey(ctx context.Context, req *policy.PolicyPublicKeyRequest) (any, error) {
	logger := s.logger.With(
		zap.String("operation", "policyPublicKey"),
		zap.String("repository", req.Repository),
		zap.String("group", req.Group),
		zap.String("name", req.PolicyName),
		zap.String("version", req.Version),
	)

	pol, err := s.storage.Policy(ctx, req.Repository, req.Group, req.PolicyName, req.Version)
	if err != nil {
		logger.Error("error getting policy from storage", zap.Error(err))
		return nil, err
	}

	exportConfig, err := policyExportConfig(pol)
	if err != nil {
		logger.Error(err.Error())
		if err == errExportConfigNotFound {
			return nil, errors.New(errors.Forbidden, err)
		}
		return nil, err
	}

	// signer namespace and key are taken from policy export configuration
	key, err := s.signer.Key(ctx, exportConfig.Namespace, exportConfig.Key)
	if err != nil {
		logger.Error("error getting policy public key", zap.Error(err))
		return nil, err
	}

	return key, nil
}

// ImportBundle imports a signed policy bundle.
func (s *Service) ImportBundle(ctx context.Context, _ *policy.ImportBundlePayload, payload io.ReadCloser) (any, error) {
	logger := s.logger.With(zap.String("operation", "importBundle"))
	defer payload.Close() //nolint:errcheck

	archive, err := io.ReadAll(payload)
	if err != nil {
		logger.Error("error reading bundle payload", zap.Error(err))
		return nil, errors.New(errors.BadRequest, fmt.Errorf("error reading bundle payload: %v", err))
	}

	files, err := s.unzip(archive)
	if err != nil {
		logger.Error("failed to unzip bundle", zap.Error(err))
		return nil, errors.New(errors.BadRequest, fmt.Errorf("failed to unzip bundle: %v", err))
	}

	if len(files) != 2 {
		err := fmt.Errorf("expected to contain two files, but has: %d", len(files))
		logger.Error("invalid bundle", zap.Error(err))
		return nil, errors.New(errors.BadRequest, "invalid bundle", err)
	}

	if err := s.verifyBundle(ctx, files); err != nil {
		logger.Error("failed to verify bundle", zap.Error(err))
		return nil, errors.New(errors.Forbidden, "failed to verify bundle", err)
	}
	logger.Debug("bundle signature is valid")

	policy, err := s.policyFromBundle(files[0].Content)
	if err != nil {
		logger.Error("cannot make policy from bundle", zap.Error(err))
		return nil, errors.New("cannot make policy from bundle", err)
	}

	if err := s.storage.SavePolicy(ctx, policy); err != nil {
		logger.Error("error saving imported policy bundle", zap.Error(err))
		return nil, errors.New("error saving imported policy bundle", err)
	}

	return map[string]interface{}{
		"repository": policy.Repository,
		"group":      policy.Group,
		"name":       policy.Name,
		"version":    policy.Version,
		"locked":     policy.Locked,
		"lastUpdate": policy.LastUpdate,
	}, err
}

// SetPolicyAutoImport enables automatic import of policy
// bundle on a given time interval.
func (s *Service) SetPolicyAutoImport(ctx context.Context, req *policy.SetPolicyAutoImportRequest) (res any, err error) {
	logger := s.logger.With(
		zap.String("operation", "setPolicyAutoImport"),
		zap.String("policyURL", req.PolicyURL),
		zap.String("interval", req.Interval),
	)

	interval, err := time.ParseDuration(req.Interval)
	if err != nil {
		logger.Error("invalid interval definition", zap.Error(err))
		return nil, errors.New(errors.BadRequest, fmt.Sprintf("invalid interval definition: %v", err))
	}

	err = s.storage.SaveAutoImportConfig(ctx, &storage.PolicyAutoImport{
		PolicyURL:  req.PolicyURL,
		Interval:   interval,
		NextImport: time.Now().Add(interval),
	})
	if err != nil {
		logger.Error("error saving auto import configuration", zap.Error(err))
		return nil, errors.New("error saving auto import configuration", err)
	}

	return map[string]string{
		"policyURL": req.PolicyURL,
		"interval":  req.Interval,
	}, nil
}

// PolicyAutoImport returns all automatic import configurations.
func (s *Service) PolicyAutoImport(ctx context.Context) (res any, err error) {
	logger := s.logger.With(zap.String("operation", "policyAutoImport"))

	configs, err := s.storage.AutoImportConfigs(ctx)
	if err != nil {
		logger.Error("error getting auto import configurations", zap.Error(err))
		return nil, errors.New("error getting auto import configurations", err)
	}

	// return an empty json array instead of null
	if configs == nil {
		configs = []*storage.PolicyAutoImport{}
	}

	return map[string]interface{}{
		"autoimport": configs,
	}, nil
}

// DeletePolicyAutoImport removes automatic import configuration.
func (s *Service) DeletePolicyAutoImport(ctx context.Context, req *policy.DeletePolicyAutoImportRequest) (res any, err error) {
	logger := s.logger.With(
		zap.String("operation", "deletePolicyAutoImport"),
		zap.String("policyURL", req.PolicyURL),
	)

	config, err := s.storage.AutoImportConfig(ctx, req.PolicyURL)
	if err != nil {
		logger.Error("cannot get auto import configuration", zap.Error(err))
		return nil, errors.New("cannot get auto import configuration", err)
	}

	if err := s.storage.DeleteAutoImportConfig(ctx, req.PolicyURL); err != nil {
		logger.Error("failed to delete auto import configuration", zap.Error(err))
		return nil, errors.New("failed to delete auto import configuration", err)
	}

	return map[string]string{
		"policyURL": config.PolicyURL,
		"interval":  config.Interval.String(),
	}, nil
}

func (s *Service) ListPolicies(ctx context.Context, req *policy.PoliciesRequest) (*policy.PoliciesResult, error) {
	logger := s.logger.With(zap.String("operation", "listPolicies"))

	policies, err := s.storage.GetPolicies(ctx, req.Locked, req.PolicyName)
	if err != nil {
		logger.Error("error retrieving policies", zap.Error(err))
		return nil, errors.New("error retrieving policies", err)
	}

	policiesResult := make([]*policy.Policy, 0, len(policies))

	for _, p := range policies {
		policy := &policy.Policy{
			Repository: p.Repository,
			PolicyName: p.Name,
			Group:      p.Group,
			Version:    p.Version,
			Locked:     p.Locked,
			LastUpdate: p.LastUpdate.Unix(),
		}

		if req.Rego != nil && *req.Rego {
			policy.Rego = ptr.String(p.Rego)
		}

		if req.Data != nil && *req.Data {
			policy.Data = ptr.String(p.Data)
		}

		if req.DataConfig != nil && *req.DataConfig {
			policy.DataConfig = ptr.String(p.DataConfig)
		}

		policiesResult = append(policiesResult, policy)
	}

	return &policy.PoliciesResult{Policies: policiesResult}, nil
}

func (s *Service) SubscribeForPolicyChange(ctx context.Context, req *policy.SubscribeRequest) (any, error) {
	logger := s.logger.With(zap.String("operation", "subscribeForPolicyChange"))

	_, err := s.storage.Policy(ctx, req.Repository, req.Group, req.PolicyName, req.Version)
	if err != nil {
		return nil, err
	}

	sub, err := s.storage.Subscriber(ctx, req.Repository, req.Group, req.PolicyName, req.Version, req.WebhookURL, req.Subscriber)
	if err != nil && !errors.Is(errors.NotFound, err) {
		return nil, errors.New("error while retrieving subscriber", err)
	}

	if sub != nil {
		return nil, errors.New(errors.Exist, "subscriber already exist")
	}

	subscriber, err := s.storage.CreateSubscriber(ctx, &storage.Subscriber{
		Name:             req.Subscriber,
		WebhookURL:       req.WebhookURL,
		PolicyRepository: req.Repository,
		PolicyName:       req.PolicyName,
		PolicyGroup:      req.Group,
		PolicyVersion:    req.Version,
	})
	if err != nil {
		logger.Error("error storing policy change subscription", zap.Error(err))
		return nil, err
	}

	return subscriber, nil
}

// prepareQuery tries to get a prepared query from the regocache.
// If the policyCache entry is not found, it will try to prepare a new
// query and will set it into the policyCache for future use.
func (s *Service) prepareQuery(ctx context.Context, repository, group, policyName, version string, headers map[string]string) (*rego.PreparedEvalQuery, error) {
	// retrieve policy
	pol, err := s.retrievePolicy(ctx, repository, group, policyName, version)
	if err != nil {
		return nil, err
	}

	// if policy is locked, return an error
	if pol.Locked {
		return nil, errors.New(errors.Forbidden, "policy is locked")
	}

	// regoQuery must match both the package declaration inside the policy
	// and the group and policy name.
	regoQuery := fmt.Sprintf("data.%s.%s", group, policyName)

	// regoArgs contains all rego functions passed to evaluation runtime
	regoArgs, err := s.buildRegoArgs(pol.Filename, pol.Rego, regoQuery, pol.Data)
	if err != nil {
		return nil, errors.New("error building rego runtime functions", err)
	}

	// Append dynamically the external.http.header function on every request,
	// because it is populated with different headers each time.
	regoArgs = append(regoArgs, rego.Function1(regofunc.GetHeaderFunc(headers)))

	newQuery, err := rego.New(
		regoArgs...,
	).PrepareForEval(ctx)
	if err != nil {
		return nil, errors.New("error preparing rego query", err)
	}

	return &newQuery, nil
}

func (s *Service) buildRegoArgs(filename, regoPolicy, regoQuery, regoData string) (availableFuncs []func(*rego.Rego), err error) {
	availableFuncs = make([]func(*rego.Rego), 3)
	availableFuncs[0] = rego.Module(filename, regoPolicy)
	availableFuncs[1] = rego.Query(regoQuery)
	availableFuncs[2] = rego.StrictBuiltinErrors(true)
	extensionFuncs := regofunc.List()
	for i := range extensionFuncs {
		availableFuncs = append(availableFuncs, extensionFuncs[i])
	}

	// add static data to evaluation runtime
	if regoData != "" {
		var data map[string]interface{}
		err := json.Unmarshal([]byte(regoData), &data)
		if err != nil {
			return nil, err
		}

		store := inmem.NewFromObject(data)
		availableFuncs = append(availableFuncs, rego.Store(store))
	}

	return availableFuncs, nil
}

func (s *Service) retrievePolicy(ctx context.Context, repository, group, policyName, version string) (*storage.Policy, error) {
	// retrieve policy from cache
	key := s.queryCacheKey(repository, group, policyName, version)
	p, ok := s.policyCache.Get(key)
	if !ok {
		// retrieve policy from storage
		var err error
		p, err = s.storage.Policy(ctx, repository, group, policyName, version)
		if err != nil {
			if errors.Is(errors.NotFound, err) {
				return nil, err
			}
			return nil, errors.New("error getting policy from storage", err)
		}
		s.policyCache.Set(key, p)
	}

	return p, nil
}

func (s *Service) queryCacheKey(repository, group, policyName, version string) string {
	return fmt.Sprintf("%s,%s,%s,%s", repository, group, policyName, version)
}

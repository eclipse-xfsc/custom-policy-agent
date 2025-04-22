package config

import "time"

type Config struct {
	HTTP        httpConfig
	Mongo       mongoConfig
	Cache       cacheConfig
	Task        taskConfig
	Signer      signerConfig
	DIDResolver didResolverConfig
	Metrics     metricsConfig
	OCM         ocmConfig
	OAuth       oauthConfig
	Refresher   refresherConfig
	Auth        authConfig
	IPFilter    ipFilterConfig
	Nats        natsConfig
	Policy      policyConfig
	AutoImport  autoimportConfig

	// ExternalAddr specifies the external address where
	// the policy service could be reached, so that
	// policy bundle verifiers can fetch public keys
	// for verification.
	ExternalAddr string `envconfig:"EXTERNAL_HTTP_ADDR" default:"http://policy:8080"`

	LogLevel string `envconfig:"LOG_LEVEL" default:"INFO"`
}

// HTTP Server configuration
type httpConfig struct {
	Host         string        `envconfig:"HTTP_HOST"`
	Port         string        `envconfig:"HTTP_PORT" default:"8080"`
	IdleTimeout  time.Duration `envconfig:"HTTP_IDLE_TIMEOUT" default:"120s"`
	ReadTimeout  time.Duration `envconfig:"HTTP_READ_TIMEOUT" default:"10s"`
	WriteTimeout time.Duration `envconfig:"HTTP_WRITE_TIMEOUT" default:"10s"`
}

type cacheConfig struct {
	// Addr specifies the address of the cache service.
	Addr string `envconfig:"CACHE_ADDR"`
}

type taskConfig struct {
	// Addr specifies the address of the task service.
	Addr string `envconfig:"TASK_ADDR"`
}

type signerConfig struct {
	// Addr specifies the address of the signer service.
	Addr string `envconfig:"SIGNER_ADDR"`
}

type didResolverConfig struct {
	// Addr specifies the address of the DID resolver service.
	Addr string `envconfig:"DID_RESOLVER_ADDR"`
}

// MongoDB configuration
type mongoConfig struct {
	Addr          string `envconfig:"MONGO_ADDR"` // required if POLICY_REPOSITORY_CLONE_URL is not set
	User          string `envconfig:"MONGO_USER"`
	Pass          string `envconfig:"MONGO_PASS"`
	DB            string `envconfig:"MONGO_DBNAME" default:"policy"`
	Collection    string `envconfig:"MONGO_COLLECTION" default:"policies"`
	AuthMechanism string `envconfig:"MONGO_AUTH_MECHANISM" default:"SCRAM-SHA-1"`
}

// Policy repository configuration
type policyConfig struct {
	CloneURL string `envconfig:"POLICY_REPOSITORY_CLONE_URL"` // required if MONGO_ADDR is not set
	User     string `envconfig:"POLICY_REPOSITORY_USER"`
	Pass     string `envconfig:"POLICY_REPOSITORY_PASS"` // an Access Token is strongly recommended
	Branch   string `envconfig:"POLICY_REPOSITORY_BRANCH"`

	// Folder inside the policy repository containing
	// needed policies. If present, only policies inside this folder
	// are going to be fetched and used for evaluation.
	Folder string `envconfig:"POLICY_REPOSITORY_FOLDER"`

	// LockOnValidationFailure indicates whether a policy must be locked for execution
	// if the policy output fails the schema validation.
	LockOnValidationFailure bool `envconfig:"POLICY_LOCK_ON_VALIDATION_FAILURE" default:"false"`
}

type metricsConfig struct {
	// Addr specifies the address of the metrics endpoint.
	Addr string `envconfig:"METRICS_ADDR" default:":2112"`
}

type ocmConfig struct {
	// Addr specifies the address of the OCM server.
	Addr string `envconfig:"OCM_ADDR" required:"true"`
}

// OAuth client configuration
type oauthConfig struct {
	ClientID     string `envconfig:"OAUTH_CLIENT_ID"`
	ClientSecret string `envconfig:"OAUTH_CLIENT_SECRET"`
	TokenURL     string `envconfig:"OAUTH_TOKEN_URL"`
}

type refresherConfig struct {
	// PollInterval specifies the interval between two data refresher runs.
	PollInterval time.Duration `envconfig:"REFRESHER_POLL_INTERVAL" default:"10s"`
}

type authConfig struct {
	// Enabled specifies whether the authentication is enabled.
	Enabled         bool          `envconfig:"AUTH_ENABLED" default:"false"`
	JwkURL          string        `envconfig:"AUTH_JWK_URL"`
	RefreshInterval time.Duration `envconfig:"AUTH_REFRESH_INTERVAL" default:"1h"`
}

type ipFilterConfig struct {
	// Enabled specifies whether the IP filtering is enabled.
	Enabled bool `envconfig:"IP_FILTER_ENABLE" default:"false"`
	// AllowedIPs specifies a list of allowed IPs.
	AllowedIPs []string `envconfig:"IP_FILTER_ALLOWED_IPS"`
}

type natsConfig struct {
	// Addr specifies the address of the NATS server.
	Addr string `envconfig:"NATS_ADDR" required:"true"`
	// Subject specifies the subject to be used for NATS subscription.
	Subject string `envconfig:"NATS_SUBJECT" default:"policy_notifier"`
}

type autoimportConfig struct {
	// PollInterval specifies the interval between two policy bundle autoimport runs.
	PollInterval time.Duration `envconfig:"AUTO_IMPORT_POLL_INTERVAL" default:"10s"`
}

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jpillora/ipfilter"
	"github.com/kelseyhightower/envconfig"
	"github.com/open-policy-agent/opa/rego"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	goahttp "goa.design/goa/v3/http"
	goa "goa.design/goa/v3/pkg"
	"golang.ngrok.com/ngrok"
	ngrokconfig "golang.ngrok.com/ngrok/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/sync/errgroup"

	goahealth "github.com/eclipse-xfsc/custom-policy-agent/gen/health"
	goahealthsrv "github.com/eclipse-xfsc/custom-policy-agent/gen/http/health/server"
	goaopenapisrv "github.com/eclipse-xfsc/custom-policy-agent/gen/http/openapi/server"
	goapolicysrv "github.com/eclipse-xfsc/custom-policy-agent/gen/http/policy/server"
	"github.com/eclipse-xfsc/custom-policy-agent/gen/openapi"
	goapolicy "github.com/eclipse-xfsc/custom-policy-agent/gen/policy"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/clients/cache"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/clients/nats"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/clients/signer"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/clone"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/config"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/header"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/notify"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/regocache"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/service"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/health"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/policy"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/service/policy/policydata"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage/memory"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage/mongodb"
	auth "github.com/eclipse-xfsc/microservice-core-go/pkg/auth"
	graceful "github.com/eclipse-xfsc/microservice-core-go/pkg/graceful"
)

var Version = "0.0.0+development"

func main() {
	// load configuration from environment
	var cfg config.Config
	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatalf("cannot load configuration: %v", err)
	}

	// create logger
	logger, err := createLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalln(err)
	}
	defer logger.Sync() //nolint:errcheck

	logger.Info("policy service started", zap.String("version", Version), zap.String("goa", goa.Version()))

	httpClient := httpClient()

	oauthClient := httpClient
	if cfg.Auth.Enabled {
		// Create an HTTP Client which automatically issues and carries an OAuth2 token.
		// The token will auto-refresh when its expiration is near.
		oauthCtx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
		oauthClient = newOAuth2Client(oauthCtx, cfg.OAuth.ClientID, cfg.OAuth.ClientSecret, cfg.OAuth.TokenURL)
	}

	signer := signer.New(cfg.Signer.Addr, signer.WithHTTPClient(httpClient))

	// create cache client
	cache := cache.New(cfg.Cache.Addr, cache.WithHTTPClient(oauthClient))

	// create event client
	events, err := nats.New(cfg.Nats.Addr, cfg.Nats.Subject)
	if err != nil {
		logger.Fatal("failed to create events client", zap.Error(err))
	}
	defer events.Close(context.Background())

	// create policy change subscribers collection
	var subscribers []storage.PolicySubscriber

	// create rego policy cache
	regocache := regocache.New()
	subscribers = append(subscribers, regocache)

	storage, err := makeStorage(cfg, logger)
	if err != nil {
		logger.Fatal("error creating storage", zap.Error(err))
	}
	defer storage.Close(context.Background())

	// create policy changes notifier
	var notifier *notify.Notifier
	subscriberStorage, ok := storage.(notify.Storage)
	if ok {
		notifier = notify.New(events, subscriberStorage, httpClient, logger)
		subscribers = append(subscribers, notifier)
	} else {
		logger.Info("policy storage does not support policy change notifications")
	}

	// subscribe the cache for policy data changes
	storage.AddPolicySubscribers(subscribers...)

	// create policy data refresher
	var dataRefresher *policydata.Refresher
	dataStorage, ok := storage.(policydata.Storage)
	if ok {
		dataRefresher = policydata.NewRefresher(
			dataStorage,
			cfg.Refresher.PollInterval,
			httpClient,
			logger,
		)
	}

	// register rego extension functions
	{
		cacheFuncs := regofunc.NewCacheFuncs(cfg.Cache.Addr, oauthClient)
		didResolverFuncs := regofunc.NewDIDResolverFuncs(cfg.DIDResolver.Addr, httpClient)
		taskFuncs := regofunc.NewTaskFuncs(cfg.Task.Addr, oauthClient)
		ocmFuncs := regofunc.NewOcmFuncs(cfg.OCM.Addr, httpClient)
		signerFuncs := regofunc.NewSignerFuncs(cfg.Signer.Addr, oauthClient)
		didWebFuncs := regofunc.NewDIDWebFuncs()
		storageFuncs := regofunc.NewStorageFuncs(storage)
		regofunc.Register("cacheGet", rego.Function3(cacheFuncs.CacheGetFunc()))
		regofunc.Register("cacheSet", rego.Function4(cacheFuncs.CacheSetFunc()))
		regofunc.Register("didResolve", rego.Function1(didResolverFuncs.ResolveFunc()))
		regofunc.Register("taskCreate", rego.Function2(taskFuncs.CreateTaskFunc()))
		regofunc.Register("taskListCreate", rego.Function2(taskFuncs.CreateTaskListFunc()))
		regofunc.Register("verificationMethod", rego.Function3(signerFuncs.VerificationMethodFunc()))
		regofunc.Register("verificationMethods", rego.Function2(signerFuncs.VerificationMethodsFunc()))
		regofunc.Register("addVCProof", rego.Function3(signerFuncs.AddVCProofFunc()))
		regofunc.Register("addVPProof", rego.Function4(signerFuncs.AddVPProofFunc()))
		regofunc.Register("verifyProof", rego.Function1(signerFuncs.VerifyProofFunc()))
		regofunc.Register("ocmLoginProofInvitation", rego.Function2(ocmFuncs.GetLoginProofInvitation()))
		regofunc.Register("ocmSendPresentationRequest", rego.Function1(ocmFuncs.SendPresentationRequest()))
		regofunc.Register("ocmLoginProofResult", rego.Function1(ocmFuncs.GetLoginProofResult()))
		regofunc.Register("ocmRawProofResult", rego.Function1(ocmFuncs.GetRawProofResult()))
		regofunc.Register("didToURL", rego.Function1(didWebFuncs.DIDToURLFunc()))
		regofunc.Register("urlToDID", rego.Function1(didWebFuncs.URLToDIDFunc()))
		regofunc.Register("storageGet", rego.Function1(storageFuncs.GetData()))
		regofunc.Register("storageSet", rego.Function2(storageFuncs.SetData()))
		regofunc.Register("storageDelete", rego.Function1(storageFuncs.DeleteData()))
	}

	// create the errgroup running all background processes here
	// so that the context could be given to components which
	// themselves run long-running processes, which should be
	// cancelled when the context is cancelled.
	g, ctx := errgroup.WithContext(context.Background())

	// create services
	var (
		policySvc goapolicy.Service
		healthSvc goahealth.Service
	)
	{
		policySvc = policy.New(
			ctx,
			storage,
			regocache,
			cache,
			signer,
			cfg.ExternalAddr,
			cfg.Policy.LockOnValidationFailure,
			cfg.AutoImport.PollInterval,
			httpClient,
			logger,
		)
		healthSvc = health.New(Version)
	}

	// create endpoints
	var (
		policyEndpoints  *goapolicy.Endpoints
		healthEndpoints  *goahealth.Endpoints
		openapiEndpoints *openapi.Endpoints
	)
	{
		policyEndpoints = goapolicy.NewEndpoints(policySvc)
		healthEndpoints = goahealth.NewEndpoints(healthSvc)
		openapiEndpoints = openapi.NewEndpoints(nil)
	}

	// Provide the transport specific request decoder and response encoder.
	// The goa http package has built-in support for JSON, XML and gob.
	// Other encodings can be used by providing the corresponding functions,
	// see goa.design/implement/encoding.
	var (
		dec = goahttp.RequestDecoder
		enc = goahttp.ResponseEncoder
	)

	// Build the service HTTP request multiplexer and configure it to serve
	// HTTP requests to the service endpoints.
	mux := goahttp.NewMuxer()

	// Wrap the endpoints with the transport specific layers. The generated
	// server packages contains code generated from the design which maps
	// the service input and output data structures to HTTP requests and
	// responses.
	var (
		policyServer  *goapolicysrv.Server
		healthServer  *goahealthsrv.Server
		openapiServer *goaopenapisrv.Server
	)
	{
		policyServer = goapolicysrv.New(policyEndpoints, mux, dec, enc, nil, errFormatter)
		healthServer = goahealthsrv.New(healthEndpoints, mux, dec, enc, nil, errFormatter)
		openapiServer = goaopenapisrv.New(openapiEndpoints, mux, dec, enc, nil, errFormatter, nil, nil)
	}

	// Apply middlewares on the servers
	policyServer.Evaluate = header.Middleware()(policyServer.Evaluate)

	// Apply IP filter middleware if enabled
	if cfg.IPFilter.Enabled {
		m := ipfilter.New(ipfilter.Options{
			AllowedIPs:     cfg.IPFilter.AllowedIPs,
			BlockByDefault: true,
			Logger:         zap.NewStdLog(logger),
		})

		policyServer.Use(m.Wrap)
	}

	// Apply Authentication middleware if enabled
	if cfg.Auth.Enabled {
		m, err := auth.NewMiddleware(cfg.Auth.JwkURL, cfg.Auth.RefreshInterval, httpClient)
		if err != nil {
			logger.Fatal("failed to create authentication middleware", zap.Error(err))
		}
		policyServer.Use(m.Handler())
	}

	// Configure the mux.
	goapolicysrv.Mount(mux, policyServer)
	goahealthsrv.Mount(mux, healthServer)
	goaopenapisrv.Mount(mux, openapiServer)

	// expose metrics
	go exposeMetrics(cfg.Metrics.Addr, logger)

	var handler http.Handler = mux
	srv := &http.Server{
		Addr:              cfg.HTTP.Host + ":" + cfg.HTTP.Port,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTP.ReadTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
	}

	g.Go(func() error {
		// use ngrok to expose the service externally
		if useNgrok := os.Getenv("USE_NGROK"); useNgrok == "true" {
			return ngrokListenAndServe(ctx, srv, logger)
		}

		if err := graceful.Shutdown(ctx, srv, 20*time.Second); err != nil {
			logger.Error("server shutdown error", zap.Error(err))
			return err
		}
		return errors.New("server stopped successfully")
	})
	g.Go(func() error {
		if err := storage.ListenPolicyDataChanges(ctx); err != nil {
			logger.Error("mongo change streams listener stopped", zap.Error(err))
			return err
		}
		return nil
	})
	if dataRefresher != nil {
		g.Go(func() error {
			return dataRefresher.Start(ctx)
		})
	}

	if err := g.Wait(); err != nil {
		logger.Error("run group stopped", zap.Error(err))
	}

	logger.Info("bye bye")
}

func createLogger(logLevel string, opts ...zap.Option) (*zap.Logger, error) {
	var level = zapcore.InfoLevel
	if logLevel != "" {
		err := level.UnmarshalText([]byte(logLevel))
		if err != nil {
			return nil, err
		}
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	config.DisableStacktrace = true
	config.EncoderConfig.TimeKey = "ts"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	return config.Build(opts...)
}

func errFormatter(ctx context.Context, e error) goahttp.Statuser {
	return service.NewErrorResponse(ctx, e)
}

func httpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     60 * time.Second,
		},
		Timeout: 30 * time.Second,
	}
}

func newOAuth2Client(ctx context.Context, cID, cSecret, tokenURL string) *http.Client {
	oauthCfg := clientcredentials.Config{
		ClientID:     cID,
		ClientSecret: cSecret,
		TokenURL:     tokenURL,
	}

	return oauthCfg.Client(ctx)
}

func exposeMetrics(addr string, logger *zap.Logger) {
	promMux := http.NewServeMux()
	promMux.Handle("/metrics", promhttp.Handler())
	logger.Info(fmt.Sprintf("exposing prometheus metrics at %s/metrics", addr))
	if err := http.ListenAndServe(addr, promMux); err != nil { //nolint:gosec
		logger.Error("error exposing prometheus metrics", zap.Error(err))
	}
}

// ngrokListenAndServe starts the HTTP server through ngrok tunnel,
// so that it's automatically exposed to the internet with HTTPS scheme.
// This functionality is needed for resolving DIDs (through policy evaluation),
// because DID resolvers do not work with insecure HTTP.
// WARNING: must be used only for development!
func ngrokListenAndServe(ctx context.Context, srv *http.Server, logger *zap.Logger) error {
	// If you have static ngrok domain, you can set it in your environment and use it
	// to have a stable domain for testing. If you don't, the service will be exposed with
	// randomly generated domain everytime it's restarted.
	var tunnel ngrokconfig.Tunnel
	if os.Getenv("NGROK_STATIC_DOMAIN") != "" {
		tunnel = ngrokconfig.HTTPEndpoint(ngrokconfig.WithDomain(os.Getenv("NGROK_STATIC_DOMAIN")))
	} else {
		tunnel = ngrokconfig.HTTPEndpoint()
	}

	var connOpts []ngrok.ConnectOption
	if os.Getenv("NGROK_TOKEN") != "" {
		connOpts = append(connOpts, ngrok.WithAuthtoken(os.Getenv("NGROK_TOKEN")))
	}

	ln, err := ngrok.Listen(ctx, tunnel, connOpts...)
	if err != nil {
		return fmt.Errorf("error starting ngrok listener: %v", err)
	}

	logger.Info(fmt.Sprintf("starting http server using ngrok: %v", ln.URL()))

	return srv.Serve(ln)
}

func makeStorage(cfg config.Config, logger *zap.Logger) (policy.Storage, error) {
	if cfg.Mongo.Addr != "" { // create MongoDB storage
		// connect to mongo db
		db, err := mongo.Connect(
			context.Background(),
			options.Client().ApplyURI(cfg.Mongo.Addr).SetAuth(options.Credential{
				AuthMechanism: cfg.Mongo.AuthMechanism,
				Username:      cfg.Mongo.User,
				Password:      cfg.Mongo.Pass,
			}),
		)
		if err != nil {
			return nil, err
		}

		storage, err := mongodb.New(db, cfg.Mongo.DB, cfg.Mongo.Collection, logger)
		if err != nil {
			return nil, err
		}

		return storage, nil
	} else if cfg.Policy.CloneURL != "" { // create memory storage
		cloner, err := clone.New()
		if err != nil {
			return nil, err
		}
		defer cloner.Cleanup() //nolint:errcheck

		repo, err := cloner.Clone(context.Background(), cfg.Policy.CloneURL, cfg.Policy.User, cfg.Policy.Pass, cfg.Policy.Branch)
		if err != nil {
			return nil, err
		}

		policies, err := cloner.IterateRepo(cfg.Policy.Folder, repo)
		if err != nil {
			return nil, err
		}

		storage := memory.New(cloner, policies, logger)

		return storage, nil
	}

	return nil, errors.New("storage configuration is not provided")
}

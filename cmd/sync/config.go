package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config defines the options for syncing policies.
// Configuration can be loaded from environment or populated by
// the given command-line arguments. The `envconfig` tags described
// below are used only when parsing config values from Environment.
type Config struct {
	// KeepAlive makes the sync process behave like a service. It will make
	// the process run indefinitely and perform syncing on every SyncInterval.
	// If KeepAlive is false, the sync program will be executed only once
	// and the process will stop. This behaviour is used in the CI pipeline
	// when policy changes are merged in the policy repo.
	KeepAlive bool `envconfig:"KEEP_ALIVE" default:"false"`

	// SyncInterval defines how often sync will be performed when the program is
	// running as a service. This is the case when KeepAlive is true.
	SyncInterval time.Duration `envconfig:"SYNC_INTERVAL" default:"120s"`

	Repo repoConfig
	DB   mongoConfig
}

type repoConfig struct {
	URL    string `envconfig:"POLICY_REPO" required:"true"`
	User   string `envconfig:"POLICY_REPO_USER"`
	Pass   string `envconfig:"POLICY_REPO_PASS"`
	Branch string `envconfig:"POLICY_REPO_BRANCH"`
	Folder string `envconfig:"POLICY_REPO_FOLDER"`
}

type mongoConfig struct {
	Addr string `envconfig:"DB_ADDR" required:"true"`
	User string `envconfig:"DB_USER"`
	Pass string `envconfig:"DB_PASS"`
	Name string `envconfig:"DB_NAME" default:"policy"`
}

func loadConfig() (*Config, error) {
	cfg := Config{}

	// load from command-line flags if present
	if len(os.Args) > 1 {
		flag.StringVar(&cfg.Repo.URL, "repoURL", "", "Policy Git repo URL.")
		flag.StringVar(&cfg.Repo.User, "repoUser", "", "Git repo username. This flag is optional.")
		flag.StringVar(&cfg.Repo.Pass, "repoPass", "", "Git repo password. This flag is optional.")
		flag.StringVar(&cfg.Repo.Branch, "branch", "", "Git branch for explicit checkout. This flag is optional.")
		flag.StringVar(&cfg.Repo.Folder, "repoFolder", "", "Folder to search for Policies within Repo. This flag is optional.")
		flag.StringVar(&cfg.DB.Addr, "dbAddr", "", "Mongo DB connection string.")
		flag.StringVar(&cfg.DB.User, "dbUser", "", "Mongo DB username.")
		flag.StringVar(&cfg.DB.Pass, "dbPass", "", "Mongo DB password.")
		flag.StringVar(&cfg.DB.Name, "dbName", "policy", "Mongo DB name.")
		flag.BoolVar(&cfg.KeepAlive, "keepAlive", false, "If true, the sync process behaves like a service and is continuously executing sync on syncInterval period.")
		flag.DurationVar(&cfg.SyncInterval, "syncInterval", 120*time.Second, "Sync interval given as time duration string, e.g. 120s.")
		flag.Parse()
		if cfg.Repo.URL == "" || cfg.DB.Addr == "" {
			return nil, fmt.Errorf("required command-line flag values are missing")
		}
		// load from environment if no command-line flags are given
	} else if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

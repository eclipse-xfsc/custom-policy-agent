// Package main provides a script to clone a repository containing Rego policies
// and add them to a Mongo DB collection
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/clone"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
)

const (
	cloneFolder      = "temp"
	policyCollection = "policies"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalln("failed to setup policy sync: ", err)
	}

	log.Println("start updating policies...")

	mongoOpts := options.Client().ApplyURI(cfg.DB.Addr)
	if cfg.DB.User != "" && cfg.DB.Pass != "" {
		mongoOpts.SetAuth(options.Credential{
			Username: cfg.DB.User,
			Password: cfg.DB.Pass,
		})
	}

	// connect to mongo db
	db, err := mongo.Connect(context.Background(), mongoOpts)
	if err != nil {
		log.Fatalln("error connecting to database: ", err)
	}
	defer db.Disconnect(context.Background()) //nolint:errcheck

	for {
		if err := sync(cfg, db); err != nil {
			log.Println(err)
		}

		if cfg.KeepAlive {
			// TODO catch SIGTERM/INTERRUPT here instead of hanging the program

			time.Sleep(cfg.SyncInterval)
			continue
		}

		break // quit sync
	}
}

func sync(cfg *Config, db *mongo.Client) error {
	// delete policy repository local folder in case the script failed last time it was executed
	if err := os.RemoveAll(cloneFolder); err != nil {
		return fmt.Errorf("failed to remove clone folder: %v", err)
	}

	// clone policy repository
	cloner, err := clone.New()
	if err != nil {
		return err
	}

	log.Println("Cloning repository...")
	repo, err := cloner.Clone(context.Background(), cfg.Repo.URL, cfg.Repo.User, cfg.Repo.Pass, cfg.Repo.Branch)
	if err != nil {
		return fmt.Errorf("error cloning repo: %v", err)
	}

	log.Println("Repository is cloned successfully.")

	// get all policies from the repository and the given directory

	log.Println("Getting policies from the cloned repository...")

	policies, err := cloner.IterateRepo(cfg.Repo.Folder, repo)
	if err != nil {
		return fmt.Errorf("error iterating repo: %v", err)
	}

	log.Println("Policies are extracted successfully.")

	// insert or update policies in Mongo DB
	if err := upsertPolicies(context.Background(), db, policies, cfg.DB.Name, cloner); err != nil {
		return fmt.Errorf("error updating policies: %v", err)
	}

	// delete policy repository folder
	if err := os.RemoveAll(cloneFolder); err != nil {
		return fmt.Errorf("error deleting policy repo folder: %v", err)
	}

	log.Println("Policies are updated successfully.")

	return nil
}

// upsertPolicies compares policies from Git repository and MongoDB
// and then updates the modified policies and inserts new ones.
func upsertPolicies(ctx context.Context, db *mongo.Client, repoPolicies map[string]*storage.Policy, policyDatabase string, cloner *clone.Cloner) error {
	log.Println("Updating policies in Database...")
	collection := db.Database(policyDatabase).Collection(policyCollection)

	currPolicies, err := fetchCurrPolicies(ctx, collection, cloner)
	if err != nil {
		return err
	}

	forUpsert := compare(currPolicies, repoPolicies)
	if len(forUpsert) > 0 {
		return upsert(ctx, forUpsert, collection)
	}

	return nil
}

// fetchCurrPolicies fetches all policies currently stored in MongoDB
// and returns a map with keys constructed out of the "group", "name" and
// "version" fields of a Policy and value - a reference to the Policy
func fetchCurrPolicies(ctx context.Context, db *mongo.Collection, cloner *clone.Cloner) (map[string]*storage.Policy, error) {
	results, err := db.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer results.Close(ctx)

	currPolicies := make(map[string]*storage.Policy)
	for results.Next(ctx) {
		var p storage.Policy
		err := results.Decode(&p)
		if err != nil {
			return nil, err
		}
		currPolicies[cloner.ConstructKey(p.Repository, p.Group, p.Name, p.Version)] = &p
	}
	if results.Err() != nil {
		return nil, results.Err()
	}

	return currPolicies, nil
}

// compare analyzes policies from Git repository and policies from MongoDB
// and returns a slice containing new and modified policies
func compare(currPolicies map[string]*storage.Policy, repoPolicies map[string]*storage.Policy) []*storage.Policy {
	var forUpsert []*storage.Policy
	for k, rPolicy := range repoPolicies {
		// check if the policy from GIT (by key) exists in MongoDB
		if cPolicy, ok := currPolicies[k]; ok {
			// if GIT policy exists in MongoDB, check if it is modified
			if !equal(cPolicy, rPolicy) {
				// if GIT policy is modified, save the 'lock' state before updating in MongoDB
				rPolicy.Locked = cPolicy.Locked
				forUpsert = append(forUpsert, rPolicy)
			}
		} else { // policy from GIT does not exist in MongoDB
			forUpsert = append(forUpsert, rPolicy)
		}
	}

	return forUpsert
}

// upsert inserts or updates policies in MongoDB collection
//
// Decision whether to insert or update is taken based on the composition of
// Policy "repository", "group", "name" and "version" fields
func upsert(ctx context.Context, policies []*storage.Policy, db *mongo.Collection) error {
	var ops []mongo.WriteModel
	for _, policy := range policies {
		op := mongo.NewUpdateOneModel()
		op.SetFilter(bson.M{
			"repository": policy.Repository,
			"group":      policy.Group,
			"name":       policy.Name,
			"version":    policy.Version,
		})
		op.SetUpdate(bson.M{
			"$set": bson.M{
				"filename":            policy.Filename,
				"locked":              policy.Locked,
				"rego":                policy.Rego,
				"data":                policy.Data,
				"dataConfig":          policy.DataConfig,
				"outputSchema":        policy.OutputSchema,
				"exportConfig":        policy.ExportConfig,
				"lastUpdate":          time.Now(),
				"nextDataRefreshTime": nextDataRefreshTime(policy),
			},
		})
		op.SetUpsert(true)
		ops = append(ops, op)
	}

	_, err := db.BulkWrite(ctx, ops, &options.BulkWriteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func nextDataRefreshTime(p *storage.Policy) time.Time {
	if p.DataConfig != "" {
		return time.Now()
	}

	return time.Time{}
}

func equal(p1 *storage.Policy, p2 *storage.Policy) bool {
	if p1.Rego == p2.Rego &&
		p1.Data == p2.Data &&
		p1.DataConfig == p2.DataConfig &&
		p1.OutputSchema == p2.OutputSchema &&
		p1.ExportConfig == p2.ExportConfig &&
		p1.Repository == p2.Repository &&
		p1.Name == p2.Name &&
		p1.Version == p2.Version &&
		p1.Filename == p2.Filename &&
		p1.Group == p2.Group {
		return true
	}

	return false
}

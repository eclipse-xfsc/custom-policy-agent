package mongodb

import (
	"context"
	goerrors "errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	zap "go.uber.org/zap"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/storage"
	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

const (
	subscriberCollectionName = "subscribers"
	commonStorage            = "common_storage"
	autoImportCollection     = "policy_auto_import"
	lockedField              = "locked"
	policyNameField          = "name"
	dataField                = "data"
	nextDataRefreshTimeField = "nextDataRefreshTime"
)

type Storage struct {
	db            *mongo.Client
	policy        *mongo.Collection
	subscriber    *mongo.Collection
	commonStorage *mongo.Collection
	autoImport    *mongo.Collection
	subscribers   []storage.PolicySubscriber
	logger        *zap.Logger
}

func New(db *mongo.Client, dbname, collection string, logger *zap.Logger) (*Storage, error) {
	if err := db.Ping(context.Background(), nil); err != nil {
		return nil, err
	}

	database := db.Database(dbname)

	return &Storage{
		db:            db,
		policy:        database.Collection(collection),
		subscriber:    database.Collection(subscriberCollectionName),
		commonStorage: database.Collection(commonStorage),
		autoImport:    database.Collection(autoImportCollection),
		logger:        logger,
	}, nil
}

func (s *Storage) Policy(ctx context.Context, repository, group, name, version string) (*storage.Policy, error) {
	result := s.policy.FindOne(ctx, bson.M{
		"repository": repository,
		"group":      group,
		"name":       name,
		"version":    version,
	})

	if result.Err() != nil {
		if goerrors.Is(result.Err(), mongo.ErrNoDocuments) {
			return nil, errors.New(errors.NotFound, "policy not found")
		}
		return nil, result.Err()
	}

	var policy storage.Policy
	if err := result.Decode(&policy); err != nil {
		return nil, err
	}

	return &policy, nil
}

func (s *Storage) SavePolicy(ctx context.Context, policy *storage.Policy) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{
		"repository": policy.Repository,
		"group":      policy.Group,
		"name":       policy.Name,
		"version":    policy.Version,
	}
	update := bson.M{"$set": bson.M{
		"locked":              policy.Locked,
		"rego":                policy.Rego,
		"data":                policy.Data,
		"dataConfig":          policy.DataConfig,
		"outputSchema":        policy.OutputSchema,
		"lastUpdate":          time.Now(),
		"nextDataRefreshTime": time.Time{},
	}}

	_, err := s.policy.UpdateOne(ctx, filter, update, opts)

	return err
}

func (s *Storage) SetPolicyLock(ctx context.Context, repository, group, name, version string, lock bool) error {
	_, err := s.policy.UpdateOne(
		ctx,
		bson.M{
			"repository": repository,
			"group":      group,
			"name":       name,
			"version":    version,
		},
		bson.M{
			"$set": bson.M{
				"locked":     lock,
				"lastUpdate": time.Now(),
			},
		},
	)
	return err
}

type PolicyEvent struct {
	OperationType string         `bson:"operationType"`
	Policy        storage.Policy `bson:"fullDocument"`
}

func (s *Storage) ListenPolicyDataChanges(ctx context.Context) error {
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	stream, err := s.policy.Watch(ctx, mongo.Pipeline{}, opts)
	if err != nil {
		return errors.New("cannot subscribe for policy data changes", err)
	}
	defer stream.Close(ctx)

	for stream.Next(ctx) {
		var policyEvent PolicyEvent
		err := stream.Decode(&policyEvent)
		if err != nil {
			return err
		}

		policy := policyEvent.Policy

		for _, subscriber := range s.subscribers {
			err := subscriber.PolicyDataChange(ctx, policy.Repository, policy.Name, policy.Group, policy.Version)
			if err != nil {
				s.logger.Error("error notifying policy change subscribers", zap.Error(err))
			}
		}

		s.logger.Info("mongo policy data changed")
	}

	return stream.Err()
}

func (s *Storage) AddPolicySubscribers(subscribers ...storage.PolicySubscriber) {
	s.subscribers = subscribers
}

func (s *Storage) GetRefreshPolicies(ctx context.Context) ([]*storage.Policy, error) {
	// create a callback for the mongodb transaction
	callback := func(mCtx mongo.SessionContext) (interface{}, error) {
		filter := bson.M{nextDataRefreshTimeField: bson.M{
			"$gt":  time.Time{}, // greater than the Go's zero date
			"$lte": time.Now(),
		}}

		cursor, err := s.policy.Find(ctx, filter)
		if err != nil {
			return nil, err
		}

		var policies []*storage.Policy
		if err := cursor.All(ctx, &policies); err != nil {
			return nil, err
		}
		if len(policies) == 0 {
			return nil, errors.New(errors.NotFound, "policies for data refresh not found")
		}

		err = s.postponeRefresh(ctx, policies)
		if err != nil {
			return nil, err
		}

		return policies, nil
	}

	// execute transaction
	res, err := s.Transaction(ctx, callback)
	if err != nil {
		return nil, err
	}
	policies, _ := res.([]*storage.Policy)

	return policies, nil
}

// PostponeRefresh adds a refreshPostponePeriod Duration to each policy's
// nextDataRefreshTimeField in order to prevent concurrent data refresh
func (s *Storage) postponeRefresh(ctx context.Context, policies []*storage.Policy) error {
	var ids []primitive.ObjectID
	for _, p := range policies {
		ids = append(ids, p.MongoID)
	}

	filter := bson.M{"_id": bson.M{"$in": ids}}
	update := bson.M{"$set": bson.M{nextDataRefreshTimeField: time.Now().Add(storage.RefreshPostponePeriod)}}
	_, err := s.policy.UpdateMany(ctx, filter, update)

	return err
}

// UpdateNextRefreshTime updates policy's data and nextDataRefreshTimeField fields
func (s *Storage) UpdateNextRefreshTime(ctx context.Context, p *storage.Policy, nextDataRefreshTime time.Time) error {
	filter := bson.M{"_id": p.MongoID}
	update := bson.M{"$set": bson.M{
		nextDataRefreshTimeField: nextDataRefreshTime,
		dataField:                p.Data,
	}}
	_, err := s.policy.UpdateOne(ctx, filter, update)

	return err
}

func (s *Storage) Transaction(ctx context.Context, callback func(mCtx mongo.SessionContext) (interface{}, error)) (interface{}, error) {
	session, err := s.db.StartSession()
	if err != nil {
		return nil, errors.New("failed creating session", err)
	}
	defer session.EndSession(ctx)

	res, err := session.WithTransaction(ctx, callback)
	if err != nil {
		return nil, errors.New("failed executing transaction", err)
	}

	return res, nil
}

func (s *Storage) GetPolicies(ctx context.Context, locked *bool, policyName *string) ([]*storage.Policy, error) {
	filter := bson.M{}
	if locked != nil {
		filter[lockedField] = locked
	}

	if policyName != nil {
		filter[policyNameField] = primitive.Regex{Pattern: *policyName, Options: "i"}
	}

	cursor, err := s.policy.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var policies []*storage.Policy
	if err := cursor.All(ctx, &policies); err != nil {
		return nil, err
	}

	return policies, nil
}

func (s *Storage) Close(ctx context.Context) {
	s.db.Disconnect(ctx) //nolint:errcheck
}

func (s *Storage) CreateSubscriber(ctx context.Context, subscriber *storage.Subscriber) (*storage.Subscriber, error) {
	subscriber.CreatedAt = time.Now()
	subscriber.UpdatedAt = time.Now()
	subscriber.MongoID = primitive.NewObjectID()

	_, err := s.subscriber.InsertOne(ctx, subscriber)
	if err != nil {
		return nil, err
	}

	return subscriber, nil
}

func (s *Storage) PolicySubscribers(ctx context.Context, policyRepository, policyName, policyGroup, policyVersion string) ([]*storage.Subscriber, error) {
	cursor, err := s.subscriber.Find(ctx, bson.M{
		"policyrepository": policyRepository,
		"policyname":       policyName,
		"policygroup":      policyGroup,
		"policyversion":    policyVersion,
	})
	if err != nil {
		return nil, err
	}

	var subscribers []*storage.Subscriber
	if err := cursor.All(ctx, &subscribers); err != nil {
		return nil, err
	}

	return subscribers, nil
}

func (s *Storage) Subscriber(ctx context.Context, policyRepository, policyGroup, policyName, policyVersion, webhookURL, name string) (*storage.Subscriber, error) {
	result := s.subscriber.FindOne(ctx, bson.M{
		"webhookurl":       webhookURL,
		"name":             name,
		"policyrepository": policyRepository,
		"policygroup":      policyGroup,
		"policyname":       policyName,
		"policyversion":    policyVersion,
	})

	if result.Err() != nil {
		if goerrors.Is(result.Err(), mongo.ErrNoDocuments) {
			return nil, errors.New(errors.NotFound, "subscriber not found")
		}
		return nil, result.Err()
	}

	var subscriber storage.Subscriber
	if err := result.Decode(&subscriber); err != nil {
		return nil, err
	}

	return &subscriber, nil
}

func (s *Storage) SetData(ctx context.Context, key string, data map[string]interface{}) error {
	opts := options.Update().SetUpsert(true)
	query := bson.M{"key": key}
	update := bson.M{"$set": bson.M{"key": key, "data": data}}

	_, err := s.commonStorage.UpdateOne(ctx, query, update, opts)

	return err
}

func (s *Storage) GetData(ctx context.Context, key string) (any, error) {
	res := s.commonStorage.FindOne(ctx, bson.M{
		"key": key,
	})
	if res.Err() != nil {
		return nil, res.Err()
	}

	commonStorage := storage.CommonStorage{}
	if err := res.Decode(&commonStorage); err != nil {
		return nil, err
	}

	return commonStorage.Data, nil
}

func (s *Storage) DeleteData(ctx context.Context, key string) error {
	res, err := s.commonStorage.DeleteOne(ctx, bson.M{
		"key": key,
	})

	if res.DeletedCount < 1 {
		return fmt.Errorf("this key doesn't exist")
	}

	return err
}

func (s *Storage) SaveAutoImportConfig(ctx context.Context, importConfig *storage.PolicyAutoImport) error {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{
		"policyURL": importConfig.PolicyURL,
	}
	update := bson.M{"$set": bson.M{
		"interval":   importConfig.Interval,
		"nextImport": importConfig.NextImport,
	}}

	_, err := s.autoImport.UpdateOne(ctx, filter, update, opts)

	return err
}

func (s *Storage) ActiveImportConfigs(ctx context.Context) ([]*storage.PolicyAutoImport, error) {
	// create a callback for the mongodb transaction
	callback := func(mCtx mongo.SessionContext) (interface{}, error) {
		filter := bson.M{"nextImport": bson.M{
			"$lte": time.Now(),
		}}

		cursor, err := s.autoImport.Find(ctx, filter)
		if err != nil {
			return nil, err
		}

		var configs []*storage.PolicyAutoImport
		if err := cursor.All(ctx, &configs); err != nil {
			return nil, err
		}

		if len(configs) == 0 {
			return nil, nil
		}

		for _, i := range configs {
			if err := s.updateAutoImportNextInterval(ctx, i); err != nil {
				s.logger.Error("failed to update next import interval", zap.Error(err))
			}
		}

		return configs, nil
	}

	// execute transaction
	res, err := s.Transaction(ctx, callback)
	if err != nil {
		return nil, err
	}

	if res == nil {
		return nil, nil
	}

	imports, ok := res.([]*storage.PolicyAutoImport)
	if !ok {
		return nil, fmt.Errorf("failed to cast auto import result: %T", res)
	}

	return imports, nil
}

// updateAutoImportNextInterval sets the next update time by adding the interval
// to the current time.
func (s *Storage) updateAutoImportNextInterval(ctx context.Context, importConfig *storage.PolicyAutoImport) error {
	filter := bson.M{"_id": importConfig.MongoID}
	update := bson.M{"$set": bson.M{"nextImport": time.Now().Add(importConfig.Interval)}}
	_, err := s.autoImport.UpdateOne(ctx, filter, update)
	return err
}

func (s *Storage) AutoImportConfigs(ctx context.Context) ([]*storage.PolicyAutoImport, error) {
	cursor, err := s.autoImport.Find(ctx, bson.D{}, nil)
	if err != nil {
		return nil, err
	}

	var configs []*storage.PolicyAutoImport
	if err := cursor.All(ctx, &configs); err != nil {
		return nil, err
	}

	return configs, nil
}

func (s *Storage) AutoImportConfig(ctx context.Context, policyURL string) (*storage.PolicyAutoImport, error) {
	filter := bson.M{"policyURL": policyURL}

	result := s.autoImport.FindOne(ctx, filter)
	if result.Err() != nil {
		if goerrors.Is(result.Err(), mongo.ErrNoDocuments) {
			return nil, errors.New(errors.NotFound, result.Err())
		}
		return nil, result.Err()
	}

	var cfg storage.PolicyAutoImport
	if err := result.Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (s *Storage) DeleteAutoImportConfig(ctx context.Context, policyURL string) error {
	filter := bson.M{"policyURL": policyURL}
	result, err := s.autoImport.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return errors.New(errors.NotFound)
	}

	return nil
}

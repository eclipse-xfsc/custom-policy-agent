package storage

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const RefreshPostponePeriod = 5 * time.Minute

type PolicySubscriber interface {
	PolicyDataChange(ctx context.Context, repo, group, name, version string) error
}

type Policy struct {
	MongoID             primitive.ObjectID `bson:"_id"`
	Filename            string
	Repository          string
	Name                string
	Group               string
	Version             string
	Rego                string
	Data                string
	DataConfig          string
	OutputSchema        string
	ExportConfig        string
	Locked              bool
	LastUpdate          time.Time
	NextDataRefreshTime time.Time
}

type Subscriber struct {
	MongoID          primitive.ObjectID `bson:"_id"`
	Name             string
	WebhookURL       string
	PolicyRepository string
	PolicyName       string
	PolicyGroup      string
	PolicyVersion    string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CommonStorage struct {
	Key  string
	Data map[string]interface{}
}

type PolicyAutoImport struct {
	MongoID    primitive.ObjectID `bson:"_id"`
	PolicyURL  string
	Interval   time.Duration
	NextImport time.Time
}

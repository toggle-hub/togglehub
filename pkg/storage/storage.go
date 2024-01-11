package storage

import (
	"context"
	"os"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStorage struct {
	db   *mongo.Database
	init bool
}

var lock = &sync.Mutex{}
var store *MongoStorage

func newMongoStorage(ctx context.Context) (*MongoStorage, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("DATABASE_URL")))
	if err != nil {
		return nil, err
	}

	db := client.Database(os.Getenv("DATABASE"))
	return &MongoStorage{
		db: db,
	}, nil
}

func (ms *MongoStorage) DB() *mongo.Database {
	return ms.db
}

func (ms *MongoStorage) Init() error {
	if ms.init {
		return nil
	}

	ms.init = true
	indexes := []struct {
		collection string
		field      string
		opts       mongo.IndexModel
	}{
		{
			collection: "user",
			field:      "email",
			opts: mongo.IndexModel{
				Keys:    bson.D{{Key: "email", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
		},
	}

	for _, index := range indexes {
		_, err := ms.db.Collection(index.collection).Indexes().CreateOne(context.Background(), index.opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func GetInstance() (*MongoStorage, error) {
	if store != nil {
		return store, nil
	}

	lock.Lock()
	defer lock.Unlock()

	newStore, err := newMongoStorage(context.Background())
	if err != nil {
		return nil, err
	}

	store = newStore
	return store, nil
}

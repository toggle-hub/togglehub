package storage

import "go.mongodb.org/mongo-driver/bson/primitive"

type Timestamps struct {
	CreatedAt primitive.Timestamp `json:"created_at" bson:"created_at"`
	UpadtedAt primitive.Timestamp `json:"updated_at" bson:"updated_at"`
	DeletedAt primitive.Timestamp `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

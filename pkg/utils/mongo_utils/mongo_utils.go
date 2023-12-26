package mongoutils

import "go.mongodb.org/mongo-driver/bson/primitive"

type MongoTimeStamps struct {
	CreatedAt primitive.Timestamp `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpadtedAt primitive.Timestamp `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.Timestamp `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

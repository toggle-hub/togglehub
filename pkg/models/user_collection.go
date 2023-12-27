package models

import (
	"time"

	apiutils "github.com/Roll-Play/togglelabs/pkg/utils/api_utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const UserCollectionName = "user"

type UserRecord struct {
	ID        primitive.ObjectID  `json:"_id,omitempty" bson:"_id,omitempty"`
	Email     string              `json:"email" bson:"email"`
	Password  string              `json:"password" bson:"password"`
	FirstName string              `json:"first_name" bson:"first_name"`
	LastName  string              `json:"last_name" bson:"last_name"`
	CreatedAt primitive.Timestamp `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpadtedAt primitive.Timestamp `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.Timestamp `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

func NewUserRecord(email, password, firstName, lastName string) (*UserRecord, error) {
	ep, err := apiutils.EncryptPassword(password)
	if err != nil {
		return nil, err
	}

	return &UserRecord{
		Email:     email,
		Password:  ep,
		FirstName: firstName,
		LastName:  lastName,
		CreatedAt: primitive.Timestamp{T: uint32(time.Now().Unix())},
		UpadtedAt: primitive.Timestamp{T: uint32(time.Now().Unix())},
	}, nil
}

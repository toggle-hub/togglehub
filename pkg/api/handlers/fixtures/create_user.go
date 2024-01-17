package fixtures

import (
	"context"
	"fmt"

	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	"go.mongodb.org/mongo-driver/mongo"
)

var userCounter = 0

func CreateUser(email, firstName, lastName, password string, db *mongo.Database) *usermodel.UserRecord {
	userCounter++
	model := usermodel.New(db)
	if email == "" {
		email = fmt.Sprintf("johndoe%d@gmail.com", userCounter)
	}

	if firstName == "" {
		firstName = "john"
	}

	if lastName == "" {
		lastName = "doe"
	}

	if password == "" {
		password = "big_secret_password"
	}
	record, err := usermodel.NewUserRecord(email, password, firstName, lastName)
	if err != nil {
		panic(err)
	}

	_, err = model.InsertOne(context.Background(), record)
	if err != nil {
		panic(err)
	}

	return record
}

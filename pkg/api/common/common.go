package common

import "go.mongodb.org/mongo-driver/bson/primitive"

type AuthResponse struct {
	ID        primitive.ObjectID `json:"_id,omitempty"`
	Email     string             `json:"email" `
	FirstName string             `json:"first_name,omitempty" `
	LastName  string             `json:"last_name,omitempty" `
}

type Tuple[T comparable, U comparable] struct {
	First  T
	Second U
}

func NewTuple[T comparable, U comparable](first T, second U) Tuple[T, U] {
	return Tuple[T, U]{
		First:  first,
		Second: second,
	}
}

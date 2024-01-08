package fixtures

import (
	"context"
	"fmt"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	"github.com/Roll-Play/togglelabs/pkg/models"
	"go.mongodb.org/mongo-driver/mongo"
)

var organizationCounter = 0

var EmptyMemberTupleList = []common.Tuple[*models.UserRecord, models.PermissionLevelEnum]{}

func CreateOrganization(
	name string,
	users []common.Tuple[*models.UserRecord, models.PermissionLevelEnum],
	db *mongo.Database,
) *models.OrganizationRecord {
	organizationCounter++
	members := make([]models.OrganizationMember, 0)
	if len(users) < 1 {
		user := CreateUser("", "", "", "", db)
		members = append(members, models.OrganizationMember{
			User:            *user,
			PermissionLevel: models.Admin,
		})
	}

	for _, tuple := range users {
		members = append(members, models.OrganizationMember{
			User:            *tuple.First,
			PermissionLevel: tuple.Second,
		})
	}

	if name == "" {
		name = fmt.Sprintf("the company %d", organizationCounter)
	}

	model := models.NewOrganizationModel(db)

	record := models.NewOrganizationRecord(name, members)

	_, err := model.InsertOne(context.Background(), record)
	if err != nil {
		panic(err)
	}

	return record
}

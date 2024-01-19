package fixtures

import (
	"context"
	"fmt"

	"github.com/Roll-Play/togglelabs/pkg/api/common"
	organizationmodel "github.com/Roll-Play/togglelabs/pkg/models/organization"
	usermodel "github.com/Roll-Play/togglelabs/pkg/models/user"
	"go.mongodb.org/mongo-driver/mongo"
)

var organizationCounter = 0

var EmptyMemberTupleList = []common.Tuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum]{}

func CreateOrganization(
	name string,
	users []common.Tuple[*usermodel.UserRecord, organizationmodel.PermissionLevelEnum],
	db *mongo.Database,
) *organizationmodel.OrganizationRecord {
	organizationCounter++
	members := make([]organizationmodel.OrganizationMember, 0, len(users))
	if len(users) < 1 {
		user := CreateUser("", "", "", "", db)
		members = append(members, organizationmodel.OrganizationMember{
			User:            *user,
			PermissionLevel: organizationmodel.Admin,
		})
	}

	for _, tuple := range users {
		members = append(members, organizationmodel.OrganizationMember{
			User:            *tuple.First,
			PermissionLevel: tuple.Second,
		})
	}

	if name == "" {
		name = fmt.Sprintf("the company %d", organizationCounter)
	}

	model := organizationmodel.New(db)

	record := organizationmodel.NewOrganizationRecord(name, members)

	_, err := model.InsertOne(context.Background(), record)
	if err != nil {
		panic(err)
	}

	return record
}

package fixtures

import (
	"github.com/aws/aws-sdk-go/service/sqs"
)

type MockSqs struct {
	_ *sqs.SQS
	_ *string
}

func (s MockSqs) CreateSession() error {
	return nil
}

func (s MockSqs) SendMessage(
	_ int64,
	_ map[string]*sqs.MessageAttributeValue,
	_ string,
) error {
	return nil
}

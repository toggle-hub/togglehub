package sqs_helper

import (
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

var sqsHelper *SqsHelper
var lock = &sync.Mutex{}

type SqsHelper struct {
	SqsClient *sqs.SQS
	QueueUrl  *string
}

func newSqsHelper() (*SqsHelper, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := sqs.New(sess)
	queueName := os.Getenv("SQS_QUEUE_NAME")
	result, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		return nil, err
	}

	return &SqsHelper{
		SqsClient: svc,
		QueueUrl:  result.QueueUrl,
	}, nil

}

func GetInstance() (*SqsHelper, error) {
	if sqsHelper != nil {
		return sqsHelper, nil
	}

	lock.Lock()
	defer lock.Unlock()
	newSqsHelper, err := newSqsHelper()
	if err != nil {
		return nil, err
	}

	sqsHelper = newSqsHelper
	return sqsHelper, nil
}

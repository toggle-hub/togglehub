package sqs_helper

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type Sqs struct {
	sqsClient *sqs.SQS
	queueUrl  *string
}

func NewSqs() (*Sqs, error) {
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

	return &Sqs{
		sqsClient: svc,
		queueUrl:  result.QueueUrl,
	}, nil

}

func (sh Sqs) SendMessage(
	delay int64,
	messageAttributes map[string]*sqs.MessageAttributeValue,
	messageBody string,
) error {
	_, err := sh.sqsClient.SendMessage(&sqs.SendMessageInput{
		DelaySeconds:      aws.Int64(delay),
		MessageAttributes: messageAttributes,
		MessageBody:       aws.String(messageBody),
		QueueUrl:          sh.queueUrl,
	})
	return err
}

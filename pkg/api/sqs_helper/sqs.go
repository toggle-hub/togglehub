package sqs_helper

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type BaseSqs interface {
	CreateSession() error
	SendMessage(
		delay int64,
		messageAttributes map[string]*sqs.MessageAttributeValue,
		messageBody string,
	) error
}

type Sqs struct {
	sqsClient *sqs.SQS
	queueUrl  *string
}

func (s *Sqs) CreateSession() error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := sqs.New(sess)
	queueName := os.Getenv("SQS_QUEUE_NAME")
	result, err := svc.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &queueName,
	})
	if err != nil {
		return err
	}

	s.sqsClient = svc
	s.queueUrl = result.QueueUrl

	return nil
}

func (s *Sqs) SendMessage(
	delay int64,
	messageAttributes map[string]*sqs.MessageAttributeValue,
	messageBody string,
) error {
	_, err := s.sqsClient.SendMessage(&sqs.SendMessageInput{
		DelaySeconds:      aws.Int64(delay),
		MessageAttributes: messageAttributes,
		MessageBody:       aws.String(messageBody),
		QueueUrl:          s.queueUrl,
	})
	return err
}

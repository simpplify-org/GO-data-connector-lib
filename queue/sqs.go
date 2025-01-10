package queue

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
)

type ToSqs struct {
	AwsAccessKey string
	AwsSecretKey string
	AwsRegion    string
	QueueUrl     string
}

func NewToSqs(AwsAccessKey, AwsSecretKey, AwsRegion, QueueUrl string) *ToSqs {
	return &ToSqs{
		AwsAccessKey: AwsAccessKey,
		AwsSecretKey: AwsSecretKey,
		AwsRegion:    AwsRegion,
		QueueUrl:     QueueUrl,
	}
}

func (q *ToSqs) SendMessage(message []byte, messageGroupId string) (*sqs.SendMessageOutput, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(q.AwsRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(q.AwsAccessKey, q.AwsSecretKey, "")),
	)
	if err != nil {
		return &sqs.SendMessageOutput{}, err
	}

	sqsClient := sqs.NewFromConfig(cfg)
	strUUid := uuid.New()
	input := &sqs.SendMessageInput{
		MessageBody:            aws.String(string(message)),
		QueueUrl:               aws.String(q.QueueUrl),
		MessageGroupId:         aws.String(messageGroupId),
		MessageDeduplicationId: aws.String(strUUid.String()),
	}

	result, err := sqsClient.SendMessage(context.TODO(), input)
	if err != nil {
		return &sqs.SendMessageOutput{}, err
	}

	return result, err
}

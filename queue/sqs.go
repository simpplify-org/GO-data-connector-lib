package queue

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"time"
)

type ToSqs struct {
	AwsAccessKey string
	AwsSecretKey string
	AwsRegion    string
	QueueUrl     string
}
type ConsumerConfig struct {
	MaxNumberOfMessages int32         // padrao 10 segundos
	WaitTimeSeconds     int32         // padrao 10 segundos
	VisibilityTimeout   int32         // padrao 30 segundos
	PollInterval        time.Duration // padrao 5 segundos
	BufferSize          int           // padrao 20 mensagens
}

func NewToSqs(AwsAccessKey, AwsSecretKey, AwsRegion, QueueUrl string) *ToSqs {
	return &ToSqs{
		AwsAccessKey: AwsAccessKey,
		AwsSecretKey: AwsSecretKey,
		AwsRegion:    AwsRegion,
		QueueUrl:     QueueUrl,
	}
}

func (q *ToSqs) getClient() (*sqs.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(q.AwsRegion),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(q.AwsAccessKey, q.AwsSecretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	return sqs.NewFromConfig(cfg), nil
}

func (q *ToSqs) SendMessage(message []byte, messageGroupId string) (*sqs.SendMessageOutput, error) {
	client, err := q.getClient()
	if err != nil {
		return nil, err
	}
	strUUid := uuid.New()
	input := &sqs.SendMessageInput{
		MessageBody:            aws.String(string(message)),
		QueueUrl:               aws.String(q.QueueUrl),
		MessageGroupId:         aws.String(messageGroupId),
		MessageDeduplicationId: aws.String(strUUid.String()),
	}

	result, err := client.SendMessage(context.TODO(), input)
	if err != nil {
		return &sqs.SendMessageOutput{}, err
	}

	return result, err
}

func (q *ToSqs) Consume(ctx context.Context, cfg ConsumerConfig) (<-chan types.Message, error) {
	client, err := q.getClient()
	if err != nil {
		return nil, err
	}

	if cfg.MaxNumberOfMessages <= 0 {
		cfg.MaxNumberOfMessages = 10
	}
	if cfg.WaitTimeSeconds <= 0 {
		cfg.WaitTimeSeconds = 10
	}
	if cfg.VisibilityTimeout <= 0 {
		cfg.VisibilityTimeout = 30
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 20
	}

	msgCh := make(chan types.Message, cfg.BufferSize)

	go func() {
		defer close(msgCh)

		for {
			select {
			case <-ctx.Done():
				fmt.Println("Consumer finalizado...")
				return
			default:
				resp, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
					QueueUrl:            aws.String(q.QueueUrl),
					MaxNumberOfMessages: cfg.MaxNumberOfMessages,
					WaitTimeSeconds:     cfg.WaitTimeSeconds,
					VisibilityTimeout:   cfg.VisibilityTimeout,
				})
				if err != nil {
					fmt.Println("Erro ao receber mensagem:", err)
					time.Sleep(cfg.PollInterval)
					continue
				}

				if len(resp.Messages) == 0 {
					continue
				}

				for _, m := range resp.Messages {
					select {
					case msgCh <- m:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return msgCh, nil
}

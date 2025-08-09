package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	cfg               aws.Config
	ctx               context.Context
	s3Client          *s3.Client
	snsClient         *sns.Client
	dynamoDbClient    *dynamodb.Client
	bucketName        string
	dynamoDbTableName string
	snsTopicArn       string
	imageExtensions   = map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}
)

type SNSMessage struct {
	JobId      string   `json:"jobId"`
	ObjectKeys []string `json:"objectKeys"`
	EventId    string   `json:"eventId"`
	TaskNumber int      `json:"taskNumber"`
}

func init() {
	var err error
	ctx = context.TODO()
	cfg, err = config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, not loading default config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)
	dynamoDbClient = dynamodb.NewFromConfig(cfg)
	snsClient = sns.NewFromConfig(cfg)
	bucketName = os.Getenv("BUCKET_NAME")
	dynamoDbTableName = os.Getenv("DYNAMODB_TABLE_NAME")
	snsTopicArn = os.Getenv("SNS_TOPIC_CREATE_JOB_ARN")
}

func handleRequest(ctx context.Context, event map[string]interface{}) (map[string]interface{}, error) {

	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		Prefix:  aws.String("images/"),
		MaxKeys: aws.Int32(10),
	})

	objectKeysPages, err := getObjectKeysPages(paginator)
	if err != nil {
		log.Printf("Error retrieving object keys: %v", err)
		return map[string]interface{}{
			"statusCode": 500,
			"body":       "Internal Server Error",
		}, nil
	}
	log.Printf("Retrieved object keys pages: %d pages", len(objectKeysPages))
	childJobCount := len(objectKeysPages)
	jobId := uuid.New().String()
	eventId := uuid.New().String()

	err = writeDynamoDbArchiveJob(&eventId, &jobId, childJobCount)
	if err != nil {
		return nil, err
	}

	for i, page := range objectKeysPages {
		log.Printf("Page %d: %d keys", i+1, len(page))
		objectKeys := make([]string, len(page))
		for j, key := range page {
			objectKeys[j] = *key
		}

		err := sendSNSMessage(&jobId, objectKeys, &eventId, i+1)
		if err != nil {
			log.Printf("Error sending SNS message for page %d: %v", i+1, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"statusCode": 200,
		"body":       "Hello, World!",
	}, nil
}

func getObjectKeysPages(paginator *s3.ListObjectsV2Paginator) ([][]*string, error) {
	var objectKeysPages [][]*string
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		var objectKeys []*string
		for _, object := range output.Contents {
			itemExtension := filepath.Ext(*object.Key)
			if !imageExtensions[itemExtension] {
				continue
			}
			objectKeys = append(objectKeys, object.Key)
		}
		objectKeysPages = append(objectKeysPages, objectKeys)
	}
	return objectKeysPages, nil
}

func sendSNSMessage(jobId *string, objectKeys []string, eventId *string, taskNumber int) error {
	snsMessage := SNSMessage{
		JobId:      *jobId,
		ObjectKeys: objectKeys,
		EventId:    *eventId,
		TaskNumber: taskNumber,
	}
	messageBody := serializeSNSMessage(&snsMessage)
	snsInput := &sns.PublishInput{
		Message:  aws.String(messageBody),
		TopicArn: aws.String(snsTopicArn),
	}
	_, err := snsClient.Publish(ctx, snsInput)
	if err != nil {
		log.Printf("Error publishing SNS message: %v", err)
		return err
	}

	return nil
}

func writeDynamoDbArchiveJob(eventId *string, jobId *string, childJobCount int) error {
	input := &dynamodb.PutItemInput{
		TableName: aws.String(dynamoDbTableName),
		Item: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{
				Value: "EVENT#" + *eventId,
			},
			"SK": &types.AttributeValueMemberS{
				Value: "ARCHIVE_JOB#" + *jobId,
			},
			"CHILD_JOB_COUNT": &types.AttributeValueMemberN{
				Value: fmt.Sprintf("%d", childJobCount),
			},
			"TOTAL_CHILD_JOB_COUNT": &types.AttributeValueMemberN{
				Value: fmt.Sprintf("%d", childJobCount),
			},
			"TTL": &types.AttributeValueMemberN{
				Value: fmt.Sprintf("%d", time.Now().Add(15*time.Minute).Unix()),
			},
		},
	}

	_, err := dynamoDbClient.PutItem(ctx, input)
	if err != nil {
		log.Printf("Error writing to DynamoDB: %v", err)
		return err
	}
	return nil

}

func serializeSNSMessage(message *SNSMessage) string {
	response, err := json.Marshal(message)
	if err != nil {
		log.Fatalf("Error serializing response: %v", err)
	}
	return string(response)
}

func main() {
	lambda.Start(handleRequest)
}

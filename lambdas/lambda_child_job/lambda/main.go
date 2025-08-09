package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
)

var (
	cfg               aws.Config
	ctx               context.Context
	s3Client          *s3.Client
	dynamoDbClient    *dynamodb.Client
	snsClient         *sns.Client
	uploader          *manager.Uploader
	bucketName        string
	dynamoDbTableName string
	snsTopicArn       string
)

type SNSMessage struct {
	JobId      string   `json:"jobId"`
	ObjectKeys []string `json:"objectKeys"`
	EventId    string   `json:"eventId"`
	TaskNumber int      `json:"taskNumber"`
}

type SNSZipArchiveRequest struct {
	EventId            string `json:"eventId"`
	JobId              string `json:"jobId"`
	TotalArchivesCount int    `json:"totalArchivesCount"`
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
	snsTopicArn = os.Getenv("SNS_TOPIC_CHILD_JOB_FINISHED_ARN")
	uploader = manager.NewUploader(s3Client)
}

func handleRequest(ctx context.Context, event events.SNSEvent) {
	log.Printf("Handle request %v", event)
	for _, record := range event.Records {
		message := record.SNS.Message
		request := getRequestBody(&message)

		log.Printf("Processing message: %+v", request)

		err := streamZipToS3(bucketName, fmt.Sprintf("archives/%s/archive_%d.zip", request.EventId, request.TaskNumber), request.ObjectKeys)

		if err != nil {
			log.Printf("Error streaming zip to S3: %v", err)
			return
		}

		updatedItem, err := decrementChildJobCount(&request.EventId, &request.JobId)
		if err != nil {
			return
		}
		childJobCount, err := strconv.Atoi(updatedItem.Attributes["CHILD_JOB_COUNT"].(*types.AttributeValueMemberN).Value)
		if err != nil {
			log.Printf("Error converting CHILD_JOB_COUNT to int: %v", err)
			return
		}
		log.Printf("Decremented child job count for event %s, job %s: %+v", request.EventId, request.JobId, childJobCount)

		if childJobCount != 0 {
			continue
		}

		log.Printf("All child jobs finished for event %s, job %s", request.EventId, request.JobId)

		totalChildJobCount, err := strconv.Atoi(updatedItem.Attributes["TOTAL_CHILD_JOB_COUNT"].(*types.AttributeValueMemberN).Value)
		if err != nil {
			log.Printf("Error converting TOTAL_CHILD_JOB_COUNT to int: %v", err)
			return
		}

		err = publishSNSZipArchiveRequest(&request.EventId, &request.JobId, totalChildJobCount)
		if err != nil {
			return
		}

	}
}

func getRequestBody(message *string) SNSMessage {
	var request SNSMessage
	if err := json.Unmarshal([]byte(*message), &request); err != nil {
		log.Printf("Error unmarshalling message: %v", err)
		return SNSMessage{}
	}
	log.Printf("Request: %+v", request)
	return request
}

func streamZipToS3(bucketName, archiveKey string, fileKeys []string) error {
	pipeReader, pipeWriter := io.Pipe()
	zipWriter := zip.NewWriter(pipeWriter)

	var waitGroup sync.WaitGroup
	var uploadErr error

	// Start S3 upload in background
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		putInput := s3.PutObjectInput{
			Bucket:      aws.String(bucketName),
			Key:         aws.String(archiveKey),
			Body:        pipeReader,
			ContentType: aws.String("application/zip"),
		}
		_, err := uploader.Upload(ctx, &putInput)
		if err != nil {
			uploadErr = fmt.Errorf("upload failed: %w", err)
			pipeReader.CloseWithError(err)
		}
	}()

	// Write ZIP contents
	for _, s3Key := range fileKeys {

		err := writeZipFile(&s3Key, pipeWriter, zipWriter)

		if err != nil {
			log.Printf("Error writing zip file %s: %v", s3Key, err)
			pipeWriter.CloseWithError(err)
		}
	}

	// Finalize ZIP
	zipWriter.Close()
	pipeWriter.Close()

	waitGroup.Wait()

	log.Printf("Finished writing zip file %s", archiveKey)

	// Wait for upload to finish
	return uploadErr
}

func writeZipFile(s3Key *string, pipeWriter *io.PipeWriter, zipWriter *zip.Writer) error {
	s3Input := s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    s3Key,
	}
	object, err := s3Client.GetObject(ctx, &s3Input)

	if err != nil {
		pipeWriter.CloseWithError(err)
		return fmt.Errorf("get object %s: %w", *s3Key, err)
	}

	header := &zip.FileHeader{
		Name:   *s3Key,
		Method: zip.Store,
	}

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		pipeWriter.CloseWithError(err)
		return fmt.Errorf("create zip header: %w", err)
	}

	_, err = io.Copy(writer, object.Body)

	if err != nil {
		pipeWriter.CloseWithError(err)
		return fmt.Errorf("writing zip file in crtitical section: %w", err)
	}

	object.Body.Close()

	if err != nil {
		pipeWriter.CloseWithError(err)
		return fmt.Errorf("copy file %s to zip: %w", *s3Key, err)
	}

	return nil
}

func decrementChildJobCount(eventId *string, jobId *string) (*dynamodb.UpdateItemOutput, error) {
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(dynamoDbTableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{
				Value: fmt.Sprintf("EVENT#%s", *eventId),
			},
			"SK": &types.AttributeValueMemberS{
				Value: fmt.Sprintf("ARCHIVE_JOB#%s", *jobId),
			},
		},
		UpdateExpression: aws.String("ADD CHILD_JOB_COUNT :decrement"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":decrement": &types.AttributeValueMemberN{
				Value: "-1",
			},
		},
		ReturnValues: types.ReturnValueAllNew,
	}
	update, err := dynamoDbClient.UpdateItem(ctx, updateInput)
	if err != nil {
		log.Printf("Error decrementing child job count: %v", err)
		return nil, err
	}

	return update, nil
}

func publishSNSMessage(jobId *string, objectKeys []string, eventId *string, taskNumber int) error {
	snsMessage := SNSMessage{
		JobId:      *jobId,
		ObjectKeys: objectKeys,
		EventId:    *eventId,
		TaskNumber: taskNumber,
	}
	messageBody, err := json.Marshal(snsMessage)
	if err != nil {
		log.Printf("Error marshalling SNS message: %v", err)
		return err
	}

	snsInput := &sns.PublishInput{
		Message:  aws.String(string(messageBody)),
		TopicArn: aws.String(snsTopicArn),
	}

	_, err = snsClient.Publish(ctx, snsInput)
	if err != nil {
		log.Printf("Error publishing SNS message: %v", err)
		return err
	}

	return nil
}

func publishSNSZipArchiveRequest(eventId *string, jobId *string, totalArchivesCount int) error {
	snsRequest := SNSZipArchiveRequest{
		EventId:            *eventId,
		JobId:              *jobId,
		TotalArchivesCount: totalArchivesCount,
	}

	snsMessage, err := json.Marshal(snsRequest)
	if err != nil {
		log.Printf("Error marshalling SNS message: %v", err)
		return err
	}

	snsInput := &sns.PublishInput{
		Message:  aws.String(string(snsMessage)),
		TopicArn: aws.String(snsTopicArn),
	}

	_, err = snsClient.Publish(ctx, snsInput)
	if err != nil {
		log.Printf("Error publishing SNS message: %v", err)
		return err
	}

	return nil
}

func main() {
	lambda.Start(handleRequest)
}

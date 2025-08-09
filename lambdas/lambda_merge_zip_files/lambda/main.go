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
	"io"
	"log"
	"os"
	"sync"
)

type SNSZipArchiveRequest struct {
	EventId            string `json:"eventId"`
	JobId              string `json:"jobId"`
	TotalArchivesCount int    `json:"totalArchivesCount"`
}

var (
	cfg               aws.Config
	ctx               context.Context
	s3Client          *s3.Client
	dynamoDbClient    *dynamodb.Client
	uploader          *manager.Uploader
	bucketName        string
	dynamoDbTableName string
)

func init() {
	var err error
	ctx = context.TODO()
	cfg, err = config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, not loading default config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)
	dynamoDbClient = dynamodb.NewFromConfig(cfg)
	uploader = manager.NewUploader(s3Client)
	bucketName = os.Getenv("BUCKET_NAME")
	dynamoDbTableName = os.Getenv("DYNAMODB_TABLE_NAME")
}

func handleRequest(ctx context.Context, event events.SNSEvent) {
	for _, record := range event.Records {
		snsMessage := record.SNS.Message
		request := getRequestBody(&snsMessage)

		pipeReader, pipeWriter := io.Pipe()
		zipWriter := zip.NewWriter(pipeWriter)

		var waitGroup sync.WaitGroup
		var uploadErr error

		finalArchiveKey := fmt.Sprintf("archives/%s/final_archive.zip", request.EventId)

		waitGroup.Add(1)
		go createWritePipe(&waitGroup, pipeReader, finalArchiveKey, &uploadErr)

		for i := 1; i <= request.TotalArchivesCount; i++ {
			archiveKey := fmt.Sprintf("archives/%s/archive_%d.zip", request.EventId, i)
			err := writeArchiveZipFile(&archiveKey, pipeWriter, zipWriter)
			if err != nil {
				log.Printf("Error writing archive zip file: %v", err)
				return
			}

			err = deleteS3Object(&archiveKey)
			if err != nil {
				log.Printf("Error deleting S3 object %s: %v", archiveKey, err)
				continue
			}
		}

		zipWriter.Close()
		pipeWriter.Close()

		waitGroup.Wait()

		log.Printf("Finished writing zip file %s", finalArchiveKey)

		err := deleteDynamoDbItem(&request.EventId, &request.JobId)
		if err != nil {
			return
		}
	}
}

func getRequestBody(message *string) SNSZipArchiveRequest {
	var request SNSZipArchiveRequest
	if err := json.Unmarshal([]byte(*message), &request); err != nil {
		log.Printf("Error unmarshalling message: %v", err)
		return SNSZipArchiveRequest{}
	}
	log.Printf("Request: %+v", request)
	return request
}

func createWritePipe(waitGroup *sync.WaitGroup, pipeReader *io.PipeReader, finalArchiveKey string, uploadErr *error) {
	defer waitGroup.Done()
	putInput := s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(finalArchiveKey),
		Body:        pipeReader,
		ContentType: aws.String("application/zip"),
	}
	_, err := uploader.Upload(ctx, &putInput)
	if err != nil {
		pipeReader.CloseWithError(err)
		*uploadErr = err
		return
	}

}

func writeArchiveZipFile(s3Key *string, pipeWriter *io.PipeWriter, zipWriter *zip.Writer) error {
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

func deleteS3Object(s3Key *string) error {
	deleteInput := s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    s3Key,
	}
	_, err := s3Client.DeleteObject(ctx, &deleteInput)
	if err != nil {
		return fmt.Errorf("delete object %s: %w", *s3Key, err)
	}
	return nil
}

func deleteDynamoDbItem(eventId *string, jobId *string) error {
	deleteInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(dynamoDbTableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{
				Value: fmt.Sprintf("EVENT#%s", *eventId),
			},
			"SK": &types.AttributeValueMemberS{
				Value: fmt.Sprintf("ARCHIVE_JOB#%s", *jobId),
			},
		},
	}
	_, err := dynamoDbClient.DeleteItem(ctx, deleteInput)
	if err != nil {
		return fmt.Errorf("delete item from DynamoDB: %w", err)
	}
	return nil
}

func main() {
	lambda.Start(handleRequest)
}

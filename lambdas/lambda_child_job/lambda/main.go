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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"log"
	"os"
	"sync"
)

var (
	cfg        aws.Config
	ctx        context.Context
	s3Client   *s3.Client
	uploader   *manager.Uploader
	bucketName string
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
	bucketName = os.Getenv("BUCKET_NAME")
	uploader = manager.NewUploader(s3Client)
}

func handleRequest(ctx context.Context, event events.SNSEvent) {
	log.Printf("Handle request %v", event)
	for _, record := range event.Records {
		message := record.SNS.Message
		request := getRequestBody(&message)

		log.Printf("Processing message: %+v", request)

		err := streamZipToS3(bucketName, fmt.Sprintf("archives/%s/archive_%d.zip", request.JobId, request.TaskNumber), request.ObjectKeys)
		if err != nil {
			log.Printf("Error streaming zip to S3: %v", err)
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
		log.Printf("Adding %s", s3Key)

		// Stream file from S3
		s3Input := s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(s3Key),
		}
		object, err := s3Client.GetObject(ctx, &s3Input)

		if err != nil {
			pipeWriter.CloseWithError(err)
			return fmt.Errorf("get object %s: %w", s3Key, err)
		}

		header := &zip.FileHeader{
			Name:   s3Key,
			Method: zip.Deflate,
		}
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			pipeWriter.CloseWithError(err)
			return fmt.Errorf("create zip header: %w", err)
		}

		_, err = io.Copy(writer, object.Body)
		object.Body.Close()
		if err != nil {
			pipeWriter.CloseWithError(err)
			return fmt.Errorf("copy file %s to zip: %w", s3Key, err)
		}
	}

	// Finalize ZIP
	zipWriter.Close()
	pipeWriter.Close()

	// Wait for upload to finish
	waitGroup.Wait()
	return uploadErr
}

func main() {
	lambda.Start(handleRequest)
}

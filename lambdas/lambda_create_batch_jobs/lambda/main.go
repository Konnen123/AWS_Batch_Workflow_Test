package main

import (
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

func handleRequest(ctx context.Context, event map[string]interface{}) (map[string]interface{}, error) {
	// This is a placeholder for the actual implementation.
	// You can replace this with your logic to handle the request.
	log.Printf("Received event: %v", event)
	return map[string]interface{}{
		"statusCode": 200,
		"body":       "Hello, World!",
	}, nil
}

func main() {
	lambda.Start(handleRequest)
}

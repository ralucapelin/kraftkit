package ami

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
)

type Message struct {
	Image string `json:"image"`
	OS    string `json:"os"`
	Arch  string `json:"arch"`
}

func CreateQueues() []string {
	// Load AWS SDK configuration from environment variables, shared config, or AWS config file
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return nil
	}

	client := sts.NewFromConfig(cfg)

	// Get caller identity to retrieve AWS account ID
	identityOutput, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Println("Error getting caller identity:", err)
		return nil
	}

	// Print AWS account ID
	fmt.Println("AWS Account ID:", *identityOutput.Account)

	// Create an SQS client
	sqsClient := sqs.NewFromConfig(cfg)

	// Create two SQS queues
	queueNames := []string{"Orders", "Results"}
	var queueURLS []string
	for _, queueName := range queueNames {
		createQueueOutput, err := sqsClient.CreateQueue(context.TODO(), &sqs.CreateQueueInput{
			QueueName: aws.String(queueName),
		})
		if err != nil {
			fmt.Printf("Error creating queue %s: %v\n", queueName, err)
			return nil
		} else {
			// Print the ARN of the created queue
			parts := strings.Split(*createQueueOutput.QueueUrl, "/")
			queueName := parts[len(parts)-1]
			fmt.Printf("arn:aws:sqs:%s:%s:%s", cfg.Region, *identityOutput.Account, queueName)
			fmt.Println()
			queueURLS = append(queueURLS, *createQueueOutput.QueueUrl)
		}
	}
	return queueURLS
}

func DeleteQueues(queueURLs []string) {
	// Load AWS SDK configuration from environment variables, shared config, or AWS config file
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return
	}

	// Create an SQS client
	client := sqs.NewFromConfig(cfg)

	// Delete the SQS queues
	for _, queueARN := range queueURLs {
		_, err := client.DeleteQueue(context.TODO(), &sqs.DeleteQueueInput{
			QueueUrl: aws.String(queueARN),
		})
		if err != nil {
			fmt.Printf("Error deleting queue %s: %v\n", queueARN, err)
		} else {
			fmt.Printf("Deleted queue %s\n", queueARN)
		}
	}
}

func SendBuildOrder(name string, os string, arch string) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Errorf("unable to load SDK config, %v", err)
	}

	// Create an SQS client
	client := sqs.NewFromConfig(cfg)
	message := Message{
		Image: "index.unikraft.io/" + name,
		OS:    os,
		Arch:  arch,
	}
	messageBody, err := json.Marshal(message)
	if err != nil {
		log.Fatalf("unable to marshal message, %v", err)
	}

	// Define the input for the SendMessage API call
	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String("https://sqs." + GetAccountRegion() + ".amazonaws.com/" + GetAccountID() + "/Orders"),
		MessageBody: aws.String(string(messageBody)),
	}

	// Send the message
	result, err := client.SendMessage(context.TODO(), input)
	if err != nil {
		log.Fatalf("unable to send message, %v", err)
	}
	fmt.Printf("Message ID: %s\n", *result.MessageId)
}

func ReceiveResult() {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Errorf("unable to load SDK config, %v", err)
	}

	// Create an SQS client
	client := sqs.NewFromConfig(cfg)

	// Define the input for the ReceiveMessage API call
	input := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String("https://sqs." + GetAccountRegion() + ".amazonaws.com/" + GetAccountID() + "/Results"),
		WaitTimeSeconds:     20,
		MaxNumberOfMessages: 10, // Adjust the number of messages to receive at a time as needed
	}

	// Receive messages
	result, err := client.ReceiveMessage(context.TODO(), input)
	if err != nil {
		log.Fatalf("unable to receive messages, %v", err)
	}

	// Check if messages are received
	if len(result.Messages) == 0 {
		fmt.Println("No messages received")
		return
	}

	// Print received messages
	for _, message := range result.Messages {
		fmt.Printf("Message ID: %s\n", *message.MessageId)
		fmt.Printf("Message Body: %s\n", *message.Body)
		// Handle the message as needed (e.g., delete it after processing)
	}
}

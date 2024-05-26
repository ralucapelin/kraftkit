package ami

import (
	"context"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
)

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

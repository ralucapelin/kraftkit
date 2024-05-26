package ami

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go/aws"
)

// Create an S3 bucket
func createS3Bucket(bucketName string) error {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(cfg)
	bucket, err := s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
		CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraintEuCentral1,
		},
	})
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println("Created bucket. Waiting")
	// Wait until the bucket is created
	for {
		_, err := s3Client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err == nil {
			break
		}

		fmt.Printf("Waiting for bucket %s to be created...\n", bucketName)
		time.Sleep(5 * time.Second)
	}

	fmt.Printf("Bucket %s created successfully.\n", bucketName)
	fmt.Println(bucket)
	return nil
}

// Download the exported AMI from S3
func downloadFromS3(bucketName, key, localFilePath string) error {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(cfg)
	output, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer output.Body.Close()

	// Create the file
	localFile, err := os.Create(localFilePath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	// Write the file
	_, err = io.Copy(localFile, output.Body)
	if err != nil {
		return err
	}

	return nil
}

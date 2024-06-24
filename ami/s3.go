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

func bucketExists(bucketName string) (bool, error) {
	// Load the shared AWS configuration
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return false, err
	}
	s3Client := s3.NewFromConfig(cfg)

	// Attempt to get the bucket's metadata
	_, err = s3Client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	// If an error occurred, check if it's a NotFound error
	if err != nil {
		if err.Error() == "NotFound" {
			return false, nil
		}
		return false, err
	}

	// If no error, the bucket exists
	return true, nil
}

// Create an S3 bucket
func createS3Bucket(bucketName string) error {

	bucketExists, _ := bucketExists(bucketName)
	if bucketExists == true {
		return nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(cfg)
	bucket, err := s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
		CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(GetAccountRegion()),
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
		time.Sleep(3 * time.Second)
	}

	fmt.Printf("Bucket %s created successfully.\n", bucketName)
	fmt.Println(bucket)
	return nil
}

func deleteS3Bucket(bucketName string) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	s3Client := s3.NewFromConfig(cfg)
	err = deleteAllBucketObjects(s3Client, bucketName)
	if err != nil {
		fmt.Errorf("unable to delete objects in bucket %q, %v", bucketName, err)
	}

	// Delete the bucket
	_, err = s3Client.DeleteBucket(context.TODO(), &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		fmt.Errorf("unable to delete bucket %q, %v", bucketName, err)
	}

	fmt.Printf("Bucket %q successfully deleted\n", bucketName)
}

func deleteAllBucketObjects(svc *s3.Client, bucketName string) error {
	listObjectsInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	}

	for {
		listObjectsOutput, err := svc.ListObjectsV2(context.TODO(), listObjectsInput)
		if err != nil {
			return fmt.Errorf("unable to list objects in bucket %q, %v", bucketName, err)
		}

		// Delete objects
		if len(listObjectsOutput.Contents) == 0 {
			break
		}

		var deleteObjectsInput s3.DeleteObjectsInput
		deleteObjectsInput.Bucket = aws.String(bucketName)
		for _, object := range listObjectsOutput.Contents {
			deleteObjectsInput.Delete.Objects = append(deleteObjectsInput.Delete.Objects, s3types.ObjectIdentifier{
				Key: aws.String(*object.Key),
			})
		}

		_, err = svc.DeleteObjects(context.TODO(), &deleteObjectsInput)
		if err != nil {
			return fmt.Errorf("unable to delete objects in bucket %q, %v", bucketName, err)
		}

		if !*listObjectsOutput.IsTruncated {
			break
		}

		listObjectsInput.ContinuationToken = listObjectsOutput.ContinuationToken
	}

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

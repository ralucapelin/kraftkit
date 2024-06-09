package ami

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
)

// MakeInstance creates an EC2 instance with the specified user data script
func MakeInstance(name, value *string, instanceProfileName string) (*ec2.RunInstancesOutput, error) {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	// Create an EC2 service client
	svc := ec2.NewFromConfig(cfg)

	arnPrefix := "arn:aws:sqs:" + GetAccountRegion() + ":" + GetAccountID() + ":"
	userDataScript := `#!/bin/bash
yum update -y
yum install -y wget tar
GO_VERSION=1.22.2
wget https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz
tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
sudo bash -c 'echo "export PATH=$PATH:/usr/local/go/bin" >> /etc/profile'
sudo bash -c 'echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc'
source /etc/profile
source ~/.bashrc
sudo bash -c 'echo ` + *name + ` > /home/ec2-user/image-name'
sudo bash -c 'curl -o /home/ec2-user/amibuilderd  https://raw.githubusercontent.com/ralucapelin/kraftkit/staging/amibuilder/amibuilderd'
sudo bash -c 'chmod +x /home/ec2-user/amibuilderd'
sleep 10
sudo bash -c './home/ec2-user/amibuilderd -results-queue "` + arnPrefix + `Results" -orders-queue "` + arnPrefix + `Orders"'`
	fmt.Println(`./home/ec2-user/amibuilderd -results-queue "` + arnPrefix + `Results" -orders-queue "` + arnPrefix + `Orders"`)
	instanceProfile := CreateInstanceProfile(instanceProfileName)

	time.Sleep(5 * time.Second)

	encodedUserData := base64.StdEncoding.EncodeToString([]byte(userDataScript))
	runInstancesInput := &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-0f673487d7e5f89ca"),
		InstanceType: types.InstanceTypeT3Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		UserData:     aws.String(encodedUserData),
		KeyName:      aws.String("ssh-pair-central"),
		IamInstanceProfile: &types.IamInstanceProfileSpecification{
			Name: aws.String(instanceProfile),
		},
	}

	result, err := svc.RunInstances(context.TODO(), runInstancesInput)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	instanceID := result.Instances[0].InstanceId

	createTagsInput := &ec2.CreateTagsInput{
		Resources: []string{*instanceID},
		Tags: []types.Tag{
			{
				Key:   name,
				Value: value,
			},
		},
	}

	_, err = svc.CreateTags(context.TODO(), createTagsInput)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// TerminateInstance terminates an EC2 instance with the given instance ID
func TerminateInstance(instanceID string) {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Errorf("failed to load configuration, %v", err)
	}

	// Create an EC2 service client
	svc := ec2.NewFromConfig(cfg)

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}

	// Terminate the instance
	result, err := svc.TerminateInstances(context.TODO(), input)
	if err != nil {
		fmt.Errorf("failed to terminate instance, %v", err)
	}

	// Print the result
	fmt.Println("Termination initiated for instance:", instanceID)
	for _, instance := range result.TerminatingInstances {
		fmt.Printf("Instance ID: %s, Previous state: %s, Current state: %s\n",
			instance.InstanceId,
			instance.PreviousState.Name,
			instance.CurrentState.Name)
	}
}

func DeregisterImageByID(amiID string) []*string {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Errorf("failed to load configuration, %v", err)
	}
	// Create an EC2 service client
	svc := ec2.NewFromConfig(cfg)

	describeInput := &ec2.DescribeImagesInput{
		ImageIds: []string{amiID},
	}

	describeResult, err := svc.DescribeImages(context.TODO(), describeInput)
	if err != nil {
		fmt.Errorf("Could not describe image")
	}

	if len(describeResult.Images) == 0 {
		fmt.Errorf("AMI ID %s not found", amiID)
	}

	var snapshotIDs []*string
	for _, blockDevice := range describeResult.Images[0].BlockDeviceMappings {
		if blockDevice.Ebs != nil && blockDevice.Ebs.SnapshotId != nil {
			snapshotIDs = append(snapshotIDs, blockDevice.Ebs.SnapshotId)
		}
	}

	// Deregister the AMI
	input := &ec2.DeregisterImageInput{
		ImageId: aws.String(amiID),
	}

	result, err := svc.DeregisterImage(context.TODO(), input)
	if err != nil {
		fmt.Errorf("failed to deregister image, %v", err)
	}

	fmt.Printf("Successfully deregistered AMI: %s\n", amiID)
	fmt.Println(result)
	return snapshotIDs
}

// Delete the given snapshots
func DeleteSnapshots(snapshotIDs []*string) error {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil
	}

	// Create an EC2 service client
	svc := ec2.NewFromConfig(cfg)
	for _, snapshotID := range snapshotIDs {
		fmt.Println("id: " + *snapshotID)
		deleteInput := &ec2.DeleteSnapshotInput{
			SnapshotId: snapshotID,
		}

		_, err := svc.DeleteSnapshot(context.TODO(), deleteInput)
		if err != nil {
			return fmt.Errorf("failed to delete snapshot %s: %v", *snapshotID, err)
		}

		fmt.Printf("Successfully deleted snapshot with ID: %s\n", *snapshotID)
	}

	return nil
}

func DeregisterImageByName(amiName string) []*string {
	return DeregisterImageByID(*GetAMIIDByName(amiName))
}

func GetAMIIDByName(amiName string) *string {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil
	}

	// Create an EC2 service client
	svc := ec2.NewFromConfig(cfg)

	// Describe Images input
	describeInput := &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{amiName},
			},
		},
	}

	// Call the DescribeImages operation
	describeResult, err := svc.DescribeImages(context.TODO(), describeInput)
	if err != nil {
		fmt.Printf("failed to describe images, %v", err)
		return nil
	}

	if len(describeResult.Images) == 0 {
		fmt.Printf("no images found with name: %s", amiName)
		return nil
	}

	// Assume we want to deregister the first AMI found with the specified name
	return describeResult.Images[0].ImageId

}

func isValidAMI(amiNameOrID string) bool {
	// Check if the AMI ID is valid
	isValidID, err := isValidAmiID(amiNameOrID)
	if err != nil {
		fmt.Errorf("failed to validate AMI ID, %v", err)
	}

	// Check if the AMI name is valid
	isValidName, err := isValidAmiName(amiNameOrID)
	if err != nil {
		fmt.Errorf("failed to validate AMI name, %v", err)
	}

	return isValidID || isValidName
}

// Check if the given AMI ID is valid for the current account
func isValidAmiID(amiID string) (bool, error) {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return false, err
	}

	// Create an EC2 service client
	ec2Client := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeImagesInput{
		ImageIds: []string{amiID},
	}

	result, err := ec2Client.DescribeImages(context.TODO(), input)
	if err != nil {
		return false, err
	}

	return len(result.Images) > 0, nil
}

// Check if the given AMI name is valid for the current account
func isValidAmiName(amiName string) (bool, error) {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return false, err
	}

	// Create an EC2 service client
	svc := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{amiName},
			},
		},
	}

	result, err := svc.DescribeImages(context.TODO(), input)
	if err != nil {
		return false, err
	}

	return len(result.Images) > 0, nil
}

// Export the AMI to S3
func exportImageToS3(amiID, bucketName string) string {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return ""
	}

	// Create an EC2 service client
	svc := ec2.NewFromConfig(cfg)
	exportTaskInput := &ec2.ExportImageInput{
		ImageId: &amiID,
		S3ExportLocation: &types.ExportTaskS3LocationRequest{
			S3Bucket: &bucketName,
		},
		DiskImageFormat: types.DiskImageFormatRaw,
	}

	fmt.Println("HEREEE")
	exportTaskOutput, err := svc.ExportImage(context.TODO(), exportTaskInput)
	fmt.Println(exportTaskOutput)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	fmt.Println("exported image")
	fmt.Println(*exportTaskOutput.ExportImageTaskId)
	fmt.Println(*exportTaskOutput.S3ExportLocation)
	return *exportTaskOutput.ExportImageTaskId
}

func checkExportTaskStatus(exportTaskID string) {
	// Load the default configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("HERE 1")
	// Create an EC2 service client
	svc := ec2.NewFromConfig(cfg)
	output, err := svc.DescribeExportImageTasks(context.TODO(), &ec2.DescribeExportImageTasksInput{
		ExportImageTaskIds: []string{exportTaskID},
	})
	fmt.Println("HERE 2")
	if err != nil {
		fmt.Errorf("failed to describe export image tasks, %v", err)
	}

	fmt.Println("HERE 2")
	for _, task := range output.ExportImageTasks {
		fmt.Printf("Export Task ID: %s\n", *task.ExportImageTaskId)
		fmt.Printf("Status: %s\n", *task.Status)
		fmt.Println("HERE 4")
		if task.StatusMessage != nil {
			fmt.Printf("Status Message: %s\n", *task.StatusMessage)
		}
	}
}

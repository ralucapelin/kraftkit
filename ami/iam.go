package ami

import (
	"context"
	"fmt"
	"log"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws"
)

func GetAccountID() string {
	// Load AWS SDK configuration from environment variables, shared config, or AWS config file
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return ""
	}
	client := sts.NewFromConfig(cfg)

	// Get caller identity to retrieve AWS account ID
	identityOutput, err := client.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Println("Error getting caller identity:", err)
		return ""
	}

	// Print AWS account ID
	fmt.Println("AWS Account ID:", *identityOutput.Account)
	return *identityOutput.Account
}

func AddPolicies() {
	// Load AWS SDK configuration from environment variables, shared config, or AWS config file
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return
	}

	// Create an IAM client
	client := iam.NewFromConfig(cfg)

	user, err := client.GetUser(context.Background(), &iam.GetUserInput{})

	// Print the user name
	fmt.Printf("IAM Username: %s\n", *user.User.UserName)

	STSclient := sts.NewFromConfig(cfg)

	// Get caller identity to retrieve AWS account ID
	identityOutput, err := STSclient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Println("Error getting caller identity:", err)
		return
	}

	// IAM user name and policy document
	userName := *user.User.UserName
	policyName := "kraftkit-package-manager"
	policyDocument := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Sid": "EC2Resources",
				"Effect": "Allow",
				"Action": [
					"ec2:CreateTags",
					"ec2:DescribeVolumes",
					"ec2:CreateVolume",
					"ec2:DeleteVolume",
					"ec2:AttachVolume",
					"ec2:DetachVolume",
					"ec2:DescribeSnapshots",
					"ec2:CreateSnapshot",
					"ec2:DescribeImages",
					"ec2:RegisterImage",
					"ec2:DeregisterImage",
					"ec2:DeleteSnapshot",
					"ec2:RunInstances",
					"ec2:TerminateInstances",
					"ssm:GetParameters",
					"iam:PassRole",
					"iam:CreateRole",
					"iam:CreateInstanceProfile",
					"iam:AddRoleToInstanceProfile"
				],
				"Resource": "*"
			},
			{
				"Sid": "SQSResourcesReceive",
				"Effect": "Allow",
				"Action": [
					"sqs:GetQueueUrl",
					"sqs:ReceiveMessage",
					"sqs:DeleteMessage",
					"sqs:SendMessage"
				],
				"Resource": "arn:aws:sqs:` + cfg.Region + `:` + *identityOutput.Account + `:Orders"
			},
			{
				"Sid": "SQSResourcesSend",
				"Effect": "Allow",
				"Action": [
					"sqs:GetQueueUrl",
					"sqs:SendMessage",
					"sqs:ReceiveMessage"
				],
				"Resource": "arn:aws:sqs:` + cfg.Region + `:` + *identityOutput.Account + `:Results"
			},
			{
                "Effect": "Allow",
                "Action": [
					"sqs:DeleteQueue",
					"sqs:CreateQueue"
				],
                "Resource": "*"
            }
		]
	}`

	// Add policy to the IAM user
	_, err = client.PutUserPolicy(context.TODO(), &iam.PutUserPolicyInput{
		UserName:       aws.String(userName),
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	})
	if err != nil {
		fmt.Println("Error adding policy to IAM user:", err)
		return
	}

	fmt.Println("Policy added to IAM user successfully.")
	fmt.Println("Waiting for policies to propagate...")
	time.Sleep(10 * time.Second)
}

func CreateRole(roleName string) string {
	// Load AWS SDK configuration from environment variables, shared config, or AWS config file
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return ""
	}

	// Create an IAM client
	iamSvc := iam.NewFromConfig(cfg)
	assumeRolePolicy := `{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Service": "ec2.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
            }
        ]
    }`

	createRoleOutput, err := iamSvc.CreateRole(context.TODO(), &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicy),
	})
	if err != nil {
		log.Fatalf("unable to create IAM role, %v", err)
	}
	fmt.Printf("Created IAM role: %s\n", *createRoleOutput.Role.RoleName)
	return *createRoleOutput.Role.RoleName
}

func AttachPolicyToRole(roleName string, policyName string, policyDocument string) {
	// Load AWS SDK configuration from environment variables, shared config, or AWS config file
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return
	}

	// Create an IAM client
	iamSvc := iam.NewFromConfig(cfg)

	if err != nil {
		log.Fatalf("unable to marshal policy document, %v", err)
	}

	// Attach the inline policy to the role using PutRolePolicy
	_, err = iamSvc.PutRolePolicy(context.TODO(), &iam.PutRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	})
	if err != nil {
		log.Fatalf("unable to attach inline policy to role, %v", err)
	}
	fmt.Printf("Attached inline policy to IAM role: %s\n", policyName)

}

func AttachAMIBuilderPolicyToEC2Role() string {
	var roleName string = CreateRole("amibuilder-role")
	policyDocument := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Sid": "EC2Resources",
				"Effect": "Allow",
				"Action": [
					"ec2:CreateTags",
					"ec2:DescribeVolumes",
					"ec2:CreateVolume",
					"ec2:DeleteVolume",
					"ec2:AttachVolume",
					"ec2:DetachVolume",
					"ec2:DescribeSnapshots",
					"ec2:CreateSnapshot",
					"ec2:DescribeImages",
					"ec2:RegisterImage"
				],
				"Resource": "*"
			},
			{
				"Sid": "SQSResourcesReceive",
				"Effect": "Allow",
				"Action": [
					"sqs:GetQueueUrl",
					"sqs:ReceiveMessage",
					"sqs:DeleteMessage",
					"sqs:SendMessage"
				],
				"Resource": "arn:aws:sqs:eu-central-1:` + GetAccountID() + `:Orders"
			},
			{
				"Sid": "SQSResourcesSend",
				"Effect": "Allow",
				"Action": [
					"sqs:GetQueueUrl",
					"sqs:ReceiveMessage",
					"sqs:DeleteMessage",
					"sqs:SendMessage"
				],
				"Resource": "arn:aws:sqs:eu-central-1:` + GetAccountID() + `:Results"
			}
		]
	}`
	AttachPolicyToRole(roleName, "amibuilder-policy", policyDocument)
	return roleName
}
func CreateInstanceProfile(instanceProfileName string) string {
	// Load AWS SDK configuration from environment variables, shared config, or AWS config file
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return ""
	}

	// Create an IAM client
	iamSvc := iam.NewFromConfig(cfg)
	// Create an instance profile
	_, err = iamSvc.CreateInstanceProfile(context.TODO(), &iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
	})
	if err != nil {
		log.Fatalf("unable to create instance profile, %v", err)
	}
	fmt.Printf("Created instance profile: %s\n", instanceProfileName)

	// Add the role to the instance profile
	_, err = iamSvc.AddRoleToInstanceProfile(context.TODO(), &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
		RoleName:            aws.String(AttachAMIBuilderPolicyToEC2Role()),
	})
	if err != nil {
		log.Fatalf("unable to add role to instance profile, %v", err)
	}
	fmt.Printf("Added role to instance profile: %s\n", instanceProfileName)
	return instanceProfileName
}

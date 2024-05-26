package ami

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	policyName     = "vmimportPolicy"
	policyDocument = `{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "s3:ListBucket",
                    "s3:GetBucketLocation",
                    "s3:GetObject",
                    "s3:PutObject",
					"s3:GetBucketAcl"
                ],
                "Resource": [
                    "arn:aws:s3:::YOUR_BUCKET_NAME",
                    "arn:aws:s3:::YOUR_BUCKET_NAME/*"
                ]
            },
            {
                "Effect": "Allow",
                "Action": [
                    "ec2:ModifySnapshotAttribute",
                    "ec2:CopySnapshot",
                    "ec2:Describe*",
                    "ec2:ImportSnapshot",
					"ec2:RegisterImage",
                    "ec2:ExportImage"
                ],
                "Resource": "*"
            },
            {
                "Effect": "Allow",
                "Action": "iam:PassRole",
                "Resource": "arn:aws:iam::YOUR_ACCOUNT_ID:role/vmimport"
            }
        ]
    }`
	trustPolicyDocument = `{
		"Version": "2012-10-17",
		"Statement": [
		   {
			  "Effect": "Allow",
			  "Principal": { "Service": "vmie.amazonaws.com" },
			  "Action": "sts:AssumeRole",
			  "Condition": {
				 "StringEquals":{
					"sts:Externalid": "vmimport"
				 }
			  }
		   }
		]
	 }`
)

// Create an IAM service client
func createIAMClient() (*iam.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	return iam.NewFromConfig(cfg), nil
}

// Create the vmimport role
func createVMImportRole(iamClient *iam.Client, policyDoc string) error {
	roleName := "vmimport"
	// Create the role
	_, err := iamClient.CreateRole(context.TODO(), &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(trustPolicyDocument),
	})
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}
	fmt.Printf("Role %s created successfully.\n", roleName)

	// Attach the policy
	_, err = iamClient.PutRolePolicy(context.TODO(), &iam.PutRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDoc),
	})
	if err != nil {
		return fmt.Errorf("failed to attach policy to role: %w", err)
	}
	fmt.Printf("Policy %s attached to role %s successfully.\n", policyName, roleName)

	return nil
}

// Replace placeholders in the policy document
func replacePlaceholders(policy, bucketName, accountID string) string {
	policy = replace(policy, "YOUR_BUCKET_NAME", bucketName)
	policy = replace(policy, "YOUR_ACCOUNT_ID", accountID)
	return policy
}

// Helper function to replace substrings
func replace(input, old, new string) string {
	return strings.ReplaceAll(input, old, new)
}

func CreateVMImportRole(bucketName string) {
	iamClient, err := createIAMClient()
	if err != nil {
		log.Fatalf("failed to create IAM client, %v", err)
	}

	// Replace placeholders in the policy document
	finalPolicyDocument := replacePlaceholders(policyDocument, bucketName, GetAccountID())

	// Create the vmimport role
	err = createVMImportRole(iamClient, finalPolicyDocument)
	fmt.Println(finalPolicyDocument)
	if err != nil {
		log.Fatalf("failed to create vmimport role, %v", err)
	}

	fmt.Println("vmimport role created and configured successfully.")
}

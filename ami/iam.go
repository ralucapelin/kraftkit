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

func GetAccountRegion() string {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return ""
	}
	return cfg.Region
}

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
					"ec2:ExportImage",
					"ec2:DeregisterImage",
					"ec2:DeleteSnapshot",
					"ec2:RunInstances",
					"ec2:TerminateInstances",
					"ssm:GetParameters",
					"iam:PassRole",
					"iam:CreateRole",
					"iam:CreateInstanceProfile",
					"iam:AddRoleToInstanceProfile",
					"iam:DeleteInstanceProfile",
					"iam:DeleteRole",
					"iam:DeleteRolePolicy",
					"iam:RemoveRoleFromInstanceProfile"
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

	maxRetries := 10
	retryInterval := 5 * time.Second

	for i := 0; i < maxRetries; i++ {
		time.Sleep(retryInterval)

		// Check if the policy is attached
		listPolicyInput := &iam.ListUserPoliciesInput{
			UserName: aws.String(userName),
		}

		policies, err := client.ListUserPolicies(context.TODO(), listPolicyInput)
		if err != nil {
			log.Printf("failed to list user policies, %v", err)
			continue
		}

		for _, p := range policies.PolicyNames {
			if p == policyName {
				fmt.Println("User policy is now attached")
				return
			}
		}

		fmt.Println("Waiting for user policy to be attached...")
	}

	log.Fatalf("user policy attachment timed out")
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

func AttachAMIBuilderPolicyToEC2Role(name string) string {
	var roleName string = CreateRole(name)
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
		RoleName:            aws.String(AttachAMIBuilderPolicyToEC2Role(instanceProfileName)),
	})
	if err != nil {
		log.Fatalf("unable to add role to instance profile, %v", err)
	}
	fmt.Printf("Added role to instance profile: %s\n", instanceProfileName)
	maxRetries := 10
	retryInterval := 5 * time.Second
	for i := 0; i < maxRetries; i++ {
		time.Sleep(retryInterval)
		// Check if the instance profile exists
		_, err := iamSvc.GetInstanceProfile(context.TODO(), &iam.GetInstanceProfileInput{
			InstanceProfileName: aws.String(instanceProfileName),
		})
		if err == nil {
			return instanceProfileName
		}
		// If the instance profile does not exist, wait for the retry interval before retrying
		fmt.Println("Waiting for instance profile to be available...")
	}

	fmt.Println("Instance profile is now available")
	return instanceProfileName
}

func DeleteInstanceProfileAndRole(name string) {
	// Load AWS SDK configuration from environment variables, shared config, or AWS config file
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading AWS SDK configuration:", err)
		return
	}

	// Create an IAM client
	iamSvc := iam.NewFromConfig(cfg)

	// Detach each role from the instance profile

	_, err = iamSvc.RemoveRoleFromInstanceProfile(context.TODO(), &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: aws.String(name),
		RoleName:            aws.String(name),
	})
	if err != nil {
		fmt.Println("Error detaching role from instance profile:", err)
		return
	}
	fmt.Println("Role detached from instance profile")

	// List attached policies
	policiesOutput, err := iamSvc.ListRolePolicies(context.TODO(), &iam.ListRolePoliciesInput{
		RoleName: aws.String(name),
	})
	if err != nil {
		fmt.Println("Error listing attached role policies:", err)
		return
	}
	fmt.Printf("poliies: %s\n", policiesOutput.PolicyNames)

	// Detach each policy from the IAM role
	for _, policy := range policiesOutput.PolicyNames {
		_, err := iamSvc.DeleteRolePolicy(context.TODO(), &iam.DeleteRolePolicyInput{
			RoleName:   aws.String(name),
			PolicyName: aws.String(policy),
		})
		if err != nil {
			fmt.Println("Error detaching policy from IAM role:", err)
			return
		}
		fmt.Println("Policy detached from IAM role:", policy)
	}

	maxRetries := 10
	retryInterval := 1500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		time.Sleep(retryInterval)

		// Check if the policy is deleted
		listPolicyInput := &iam.ListRolePoliciesInput{
			RoleName: aws.String(name),
		}

		policies, err := iamSvc.ListRolePolicies(context.TODO(), listPolicyInput)
		if err != nil {
			log.Printf("failed to list role policies, %v", err)
			continue
		}

		policyDeleted := true
		for _, p := range policies.PolicyNames {
			contains := func(arr []string, str string) bool {
				for _, s := range arr {
					if s == str {
						return true
					}
				}
				return false
			}(policiesOutput.PolicyNames, p)
			if contains {
				policyDeleted = false
				break
			}
		}

		if policyDeleted {
			fmt.Println("Role policy is now deleted")
			_, err = iamSvc.DeleteRole(context.TODO(), &iam.DeleteRoleInput{
				RoleName: aws.String(name),
			})
			if err != nil {
				fmt.Println("Error deleting IAM role:", err)
				return
			}

			fmt.Println("IAM role deleted successfully")
			// Delete the instance profile
			_, err = iamSvc.DeleteInstanceProfile(context.TODO(), &iam.DeleteInstanceProfileInput{
				InstanceProfileName: aws.String(name),
			})
			if err != nil {
				fmt.Println("Error deleting instance profile:", err)
				return
			}
			fmt.Println("Successfully deleted instance profile")
			return
		}

		fmt.Println("Waiting for role policy to be deleted...")
	}

}

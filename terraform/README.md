# Terraform Infrastructure

This directory contains the Terraform configuration for the Parity Server infrastructure.

## Directory Structure

````
terraform/
├── modules/                    # Reusable modules
│   ├── s3/                    # S3 bucket configuration
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   └── outputs.tf
│   └── iam/                   # IAM user and policy configuration
│       ├── main.tf
│       ├── variables.tf
│       └── outputs.tf
├── environments/              # Environment-specific configurations
│   └── dev/                  # Development environment
│       ├── main.tf
│       ├── variables.tf
│       └── outputs.tf
└── README.md                 # This file

## Usage

### Development Environment

1. Navigate to the environment directory:
   ```bash
   cd environments/dev
````

2. Initialize Terraform:

   ```bash
   terraform init
   ```

3. Plan the changes:

   ```bash
   terraform plan
   ```

4. Apply the changes:
   ```bash
   terraform apply
   ```

## Modules

### S3 Module

Creates and configures an S3 bucket for storing Docker images with:

- Versioning enabled
- Server-side encryption
- Public access blocked

### IAM Module

Creates an IAM user with:

- Access key and secret
- Policy for S3 bucket access

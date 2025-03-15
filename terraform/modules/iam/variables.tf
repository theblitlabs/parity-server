variable "environment" {
  description = "Environment (dev, staging, prod)"
  type        = string
}

variable "project_name" {
  description = "Project name"
  type        = string
}

variable "s3_bucket_arn" {
  description = "ARN of the S3 bucket to grant access to"
  type        = string
}
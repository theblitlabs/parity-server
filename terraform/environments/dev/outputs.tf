output "bucket_name" {
  description = "The name of the S3 bucket"
  value       = module.s3.bucket_name
}

output "aws_access_key_id" {
  description = "The access key ID for the IAM user"
  value       = module.iam.aws_access_key_id
  sensitive   = true
}

output "aws_secret_access_key" {
  description = "The secret access key for the IAM user"
  value       = module.iam.aws_secret_access_key
  sensitive   = true
}
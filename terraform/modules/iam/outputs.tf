output "aws_access_key_id" {
  description = "The access key ID for the IAM user"
  value       = aws_iam_access_key.app_user.id
  sensitive   = true
}

output "aws_secret_access_key" {
  description = "The secret access key for the IAM user"
  value       = aws_iam_access_key.app_user.secret
  sensitive   = true
}
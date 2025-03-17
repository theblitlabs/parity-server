resource "aws_s3_bucket" "docker_images" {
  bucket = "${var.environment}-${var.project_name}-docker-images"
}

resource "aws_s3_bucket_versioning" "docker_images" {
  bucket = aws_s3_bucket.docker_images.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "docker_images" {
  bucket = aws_s3_bucket.docker_images.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "docker_images" {
  bucket = aws_s3_bucket.docker_images.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
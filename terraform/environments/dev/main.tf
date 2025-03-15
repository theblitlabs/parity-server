terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

module "s3" {
  source = "../../modules/s3"

  environment  = var.environment
  project_name = var.project_name
}

module "iam" {
  source = "../../modules/iam"

  environment   = var.environment
  project_name  = var.project_name
  s3_bucket_arn = module.s3.bucket_arn
}
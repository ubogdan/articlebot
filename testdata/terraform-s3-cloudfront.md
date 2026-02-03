---
title: "Using Terraform to host a Secure Static Website with AWS S3 and Cloudfront"
date: 2021-06-25
lastmod: 2026-02-02
description: "By the time you finish reading this article, you will know how to get your static websites up and running securely on AWS using Terraform."
categories:
  - Terraform
  - DevOps
tags:
  - terraform
  - aws
---

By the time you finish reading this article, you will know how to get your static websites up and running securely on AWS using Terraform.

## config.tf

Terraform needs plugins called providers to interact with remote systems.

```terraform
provider "aws" {
  region = "us-east-1"
}

terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
```

## s3.tf

In this file, we are going to set up the S3 bucket.

```terraform
resource "aws_s3_bucket" "website" {
  bucket = var.bucket_name
}
```

## terraform.tfvars

The tfvars file is used to specify variable values.

```terraform
domain_name = "example.com"
bucket_name = "example.com"
```

There are a lot of things to consider when setting this up.

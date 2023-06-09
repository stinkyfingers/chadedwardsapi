# vars
variable "region" {
  type    = string
  default = "us-west-1"
}

variable "profile" {
  type    = string
  default = "" #TODO - change during local tf apply
}

# provider
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.0"
    }
  }
}

provider "aws" {
  profile = var.profile
  region  = var.region
}

# import
data "terraform_remote_state" "stinkyfingers" {
  backend = "s3"
  config = {
    bucket  = "remotebackend"
    key     = "stinkyfingers/terraform.tfstate"
    region  = "us-west-1"
    profile = var.profile
  }
}

# Lambda
resource "aws_lambda_permission" "server" {
  statement_id  = "AllowExecutionFromApplicationLoadBalancer"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.server.arn
  principal     = "elasticloadbalancing.amazonaws.com"
  source_arn    = aws_lb_target_group.target.arn
}

resource "aws_lambda_permission" "server_live" {
  statement_id  = "AllowExecutionFromApplicationLoadBalancer"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_alias.server_live.arn
  principal     = "elasticloadbalancing.amazonaws.com"
  source_arn    = aws_lb_target_group.target.arn
}

resource "aws_lambda_alias" "server_live" {
  name             = "live"
  description      = "set a live alias"
  function_name    = aws_lambda_function.server.arn
  function_version = aws_lambda_function.server.version
}

resource "aws_lambda_function" "server" {
  filename         = "../lambda.zip"
  function_name    = "server"
  role             = aws_iam_role.lambda_role.arn
  handler          = "lambda-lambda"
  runtime          = "go1.x"
  source_code_hash = filebase64sha256("../lambda.zip")
  environment {
    variables = {
      STORAGE = "s3"
      AUTH    = "GCP"
    }
  }
}

# IAM
resource "aws_iam_role" "lambda_role" {
  name               = "chadedwardsapi-lambda-role"
  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "cloudwatch-attach" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_policy" "s3-policy" {
  name        = "chadedwardsapi-lambda-s3-policy"
  description = "Grants lambda access to s3"
  policy      = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:*"
      ],
      "Resource": "arn:aws:s3:::*"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "s3-policy-attach" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.s3-policy.arn
}

# ALB
resource "aws_lb_target_group" "target" {
  name        = "chadedwardsapi"
  target_type = "lambda"
}

resource "aws_lb_target_group_attachment" "server" {
  target_group_arn = aws_lb_target_group.target.arn
  target_id        = aws_lambda_alias.server_live.arn
  depends_on       = [aws_lambda_permission.server_live]
}

resource "aws_lb_listener_rule" "server" {
  listener_arn = data.terraform_remote_state.stinkyfingers.outputs.stinkyfingers_https_listener
  priority     = 2
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.target.arn
  }
  condition {
    path_pattern {
      values = ["/chadedwardsapi/*"]
    }
  }
  depends_on = [aws_lb_target_group.target]
}

# db
resource "aws_s3_bucket" "chadedwardsapi" {
  bucket = "chadedwardsapi"
}

resource "aws_s3_bucket_policy" "chadedwardsapi" {
  bucket = "chadedwardsapi"
  policy = data.aws_iam_policy_document.allow_lambda.json
}

data "aws_iam_policy_document" "allow_lambda" {
  statement {
    principals {
      type        = "AWS"
      identifiers = [aws_iam_role.lambda_role.arn]
    }
    actions = [
      "s3:*"
    ]
    resources = [
      "arn:aws:s3:::chadedwardsapi",
      "arn:aws:s3:::chadedwardsapi/*"
    ]
  }
}

# backend
terraform {
  backend "s3" {
    bucket = "remotebackend"
    key    = "chadedwardsapi/terraform.tfstate"
    region = "us-west-1"
    #    profile = "jds"
  }
}

data "terraform_remote_state" "chadedwardsapi" {
  backend = "s3"
  config = {
    bucket  = "remotebackend"
    key     = "chadedwardsapi/terraform.tfstate"
    region  = "us-west-1"
    profile = var.profile
  }
}

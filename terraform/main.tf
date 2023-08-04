# vars
variable "region" {
  type    = string
  default = "us-west-1"
}

variable "profile" {
  type    = string
  default = "" #TODO - change during local tf apply
}

variable "twilio_user" {
  type    = string
  default = "twilio_user"
}

variable "twilio_pass" {
  type    = string
  default = "twilio_pass"
}

variable "twilio_source" {
  type    = string
  default = "twilio_source"
}

variable "twilio_destination" {
  type    = string
  default = "twilio_destination"
}

variable "nexmo_key" {
  type    = string
  default = "/chadedwardsapi/nexmokey"
}

variable "nexmo_secret" {
  type    = string
  default = "/chadedwardsapi/nexmosecret"
}

variable "nexmo_source" {
  type    = string
  default = "/chadedwardsapi/nexmosource"
}

variable "nexmo_destination" {
  type    = string
  default = "/chadedwardsapi/nexmodestination"
}

variable "gmail_email" {
  type    = string
  default = "/chadedwardsapi/gmailemail"
}

variable "gmail_password" {
  type    = string
  default = "/chadedwardsapi/gmailpassword"
}

variable "gmail_destination" {
  type    = string
  default = "/chadedwardsapi/gmaildestination"
}

variable "jwt_key" {
    type    = string
    default = "/chadedwardsapi/jwtkey"
}

variable "positionstack_key" {
    type    = string
    default = "/chadedwardsapi/positionstack_key"
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
  function_name    = "chadedwardsapi"
  role             = aws_iam_role.lambda_role.arn
  handler          = "lambda-lambda"
  runtime          = "go1.x"
  source_code_hash = filebase64sha256("../lambda.zip")
  timeout          = 15
  environment {
    variables = {
      TWILIO_USER        = data.aws_ssm_parameter.twilio_user.value
      TWILIO_PASS        = data.aws_ssm_parameter.twilio_pass.value
      TWILIO_SOURCE      = data.aws_ssm_parameter.twilio_source.value
      TWILIO_DESTINATION = data.aws_ssm_parameter.twilio_destination.value
      NEXMO_KEY          = data.aws_ssm_parameter.nexmo_key.value
      NEXMO_SECRET       = data.aws_ssm_parameter.nexmo_secret.value
      NEXMO_SOURCE       = data.aws_ssm_parameter.nexmo_source.value
      NEXMO_DESTINATION  = data.aws_ssm_parameter.nexmo_destination.value
      GMAIL_EMAIL        = data.aws_ssm_parameter.gmail_email.value
      GMAIL_PASSWORD     = data.aws_ssm_parameter.gmail_password.value
      GMAIL_DESTINATION  = data.aws_ssm_parameter.gmail_destination.value
      JWT_KEY            = data.aws_ssm_parameter.jwt_key.value
      POSITIONSTACK_KEY  = data.aws_ssm_parameter.positionstack_key.value
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

resource "aws_iam_role_policy_attachment" "ssm-policy-attach" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.ssm-policy.arn
}

resource "aws_iam_policy" "ssm-policy" {
  name        = "chadedwardsapi-lambda-ssm-policy"
  description = "Grants lambda access to ssm"
  policy      = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ssm:GetParameter"
      ],
      "Resource": "arn:aws:ssm:::*"
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
  priority     = 22
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

resource "aws_s3_bucket_policy" "chadedwardsapi_s3" {
  bucket = "chadedwardsapi"
  policy = data.aws_iam_policy_document.allow_lambda_s3.json
}

data "aws_iam_policy_document" "allow_lambda_s3" {
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

data "aws_ssm_parameter" "twilio_user" {
  name            = var.twilio_user
  with_decryption = true
}

data "aws_ssm_parameter" "twilio_pass" {
  name            = var.twilio_pass
  with_decryption = true
}

data "aws_ssm_parameter" "twilio_source" {
  name            = var.twilio_source
  with_decryption = false
}

data "aws_ssm_parameter" "twilio_destination" {
  name            = var.twilio_destination
  with_decryption = false
}

data "aws_ssm_parameter" "nexmo_key" {
  name            = var.nexmo_key
  with_decryption = true
}

data "aws_ssm_parameter" "nexmo_secret" {
  name            = var.nexmo_secret
  with_decryption = true
}

data "aws_ssm_parameter" "nexmo_source" {
  name            = var.nexmo_source
  with_decryption = false
}

data "aws_ssm_parameter" "nexmo_destination" {
  name            = var.nexmo_destination
  with_decryption = false
}

data "aws_ssm_parameter" "gmail_email" {
  name            = var.gmail_email
  with_decryption = false
}

data "aws_ssm_parameter" "gmail_password" {
  name            = var.gmail_password
  with_decryption = true
}

data "aws_ssm_parameter" "gmail_destination" {
  name            = var.gmail_destination
  with_decryption = false
}

data "aws_ssm_parameter" "jwt_key" {
  name            = var.jwt_key
  with_decryption = true
}

data "aws_ssm_parameter" "positionstack_key" {
  name            = var.positionstack_key
  with_decryption = false
}

# backend
terraform {
  backend "s3" {
    bucket = "remotebackend"
    key    = "chadedwardsapi/terraform.tfstate"
    region = "us-west-1"
    #        profile = "jds"
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

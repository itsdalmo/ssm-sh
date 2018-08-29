terraform {
  required_version = "0.11.8"
}

provider "aws" {
  version = "1.33.0"
  region  = "eu-west-1"
}

# Use the default VPC and subnets
data "aws_vpc" "main" {
  default = true
}

data "aws_subnet_ids" "main" {
  vpc_id = "${data.aws_vpc.main.id}"
}

# Use the latest Amazon Linux 2 AMI
data "aws_ami" "linux2" {
  owners      = ["amazon"]
  most_recent = true

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }

  filter {
    name   = "name"
    values = ["amzn2-ami*gp2"]
  }
}

module "ssm-example" {
  source = "modules/ssm-example"

  name_prefix = "ssm-sh-example"
  vpc_id      = "${data.aws_vpc.main.id}"
  subnet_ids  = ["${data.aws_subnet_ids.main.ids}"]

  instance_ami   = "${data.aws_ami.linux2.id}"
  instance_count = "1"
  instance_type  = "t3.micro"

  tags = {
    environment = "dev"
    terraform   = "True"
  }
}

output "output_bucket" {
  value = "${module.ssm-example.output_bucket}"
}

output "output_log_group" {
  value = "${module.ssm-example.output_log_group}"
}

data "aws_iam_policy_document" "ssm-cwl" {
  statement = {
    effect = "Allow"
    sid    = "AllowAccessCloudWatchLogStream"

    actions = [
      "logs:DescribeLogStream",
      "logs:GetLogEvents",
      "logs:DescribeLogGroup",
    ]

    resources = [
      "*", #can replace with arn loggroups
    ]
  }
}

resource "aws_iam_policy" "ssm-cwl-policy" {
  name        = "ssm-cwl-policy"
  description = "policy to access cloudwatch logstreams"
  policy      = "${data.aws_iam_policy_document.ssm-cwl.json}"
}

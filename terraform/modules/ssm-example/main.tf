# -------------------------------------------------------------------------------
# Resources
# -------------------------------------------------------------------------------
data "aws_region" "current" {}

data "aws_caller_identity" "current" {}

resource "aws_cloudwatch_log_group" "output" {
  name = "${var.name_prefix}-output-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket" "output" {
  bucket = "${var.name_prefix}-output-${data.aws_caller_identity.current.account_id}"
  acl    = "private"

  tags = "${merge(var.tags, map("Name", "${var.name_prefix}-output-${data.aws_caller_identity.current.account_id}"))}"
}

resource "aws_cloudwatch_log_group" "main" {
  name = "${var.name_prefix}-ssm-agent"
}

data "template_file" "main" {
  template = "${file("${path.module}/cloud-config.yml")}"

  vars {
    region         = "${data.aws_region.current.name}"
    stack_name     = "${var.name_prefix}-asg"
    log_group_name = "${aws_cloudwatch_log_group.main.name}"
  }
}

module "asg" {
  source  = "telia-oss/asg/aws"
  version = "0.2.0"

  name_prefix       = "${var.name_prefix}"
  user_data         = "${data.template_file.main.rendered}"
  vpc_id            = "${var.vpc_id}"
  subnet_ids        = "${var.subnet_ids}"
  await_signal      = "true"
  pause_time        = "PT5M"
  health_check_type = "EC2"
  instance_policy   = "${data.aws_iam_policy_document.permissions.json}"
  min_size          = "${var.instance_count}"
  instance_type     = "${var.instance_type}"
  instance_ami      = "${var.instance_ami}"
  instance_key      = ""
  tags              = "${var.tags}"
}

data "aws_iam_policy_document" "permissions" {
  statement {
    effect = "Allow"

    actions = [
      "ssm:DescribeAssociation",
      "ssm:GetDeployablePatchSnapshotForInstance",
      "ssm:GetDocument",
      "ssm:GetManifest",
      "ssm:GetParameters",
      "ssm:ListAssociations",
      "ssm:ListInstanceAssociations",
      "ssm:PutInventory",
      "ssm:PutComplianceItems",
      "ssm:PutConfigurePackageResult",
      "ssm:UpdateAssociationStatus",
      "ssm:UpdateInstanceAssociationStatus",
      "ssm:UpdateInstanceInformation",
    ]

    resources = ["*"]
  }

  statement {
    effect = "Allow"

    actions = [
      "ec2messages:AcknowledgeMessage",
      "ec2messages:DeleteMessage",
      "ec2messages:FailMessage",
      "ec2messages:GetEndpoint",
      "ec2messages:GetMessages",
      "ec2messages:SendReply",
    ]

    resources = ["*"]
  }

  statement {
    effect = "Allow"

    actions = [
      "cloudwatch:PutMetricData",
    ]

    resources = ["*"]
  }

  statement {
    effect = "Allow"

    actions = [
      "ec2:DescribeInstanceStatus",
    ]

    resources = ["*"]
  }

  statement {
    effect = "Allow"

    resources = [
      "${aws_cloudwatch_log_group.main.arn}",
      "${aws_cloudwatch_log_group.output.arn}",
    ]

    actions = [
      "logs:CreateLogStream",
      "logs:CreateLogGroup",
      "logs:PutLogEvents",
    ]
  }

  statement {
    effect = "Allow"

    actions = [
      "s3:PutObject",
      "s3:GetObject",
      "s3:AbortMultipartUpload",
      "s3:ListMultipartUploadParts",
      "s3:ListBucket",
      "s3:ListBucketMultipartUploads",
    ]

    resources = [
      "${aws_s3_bucket.output.arn}/*",
      "${aws_s3_bucket.output.arn}",
    ]
  }
}

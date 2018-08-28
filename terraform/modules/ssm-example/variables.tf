# ------------------------------------------------------------------------------
# Variables
# ------------------------------------------------------------------------------
variable "name_prefix" {
  description = "Prefix used for resource names."
}

variable "vpc_id" {
  description = "ID of the VPC for the subnets."
}

variable "subnet_ids" {
  description = "IDs of subnets where the instances will be provisioned."
  type        = "list"
}

variable "output_bucket" {
  description = "Name of the bucket to which instance should be allowed to write command outputs."
}

variable "output_log_group" {
  description = "Name of the CloudWatch log group to which instance should be allowed to write command outputs."
}

variable "instance_count" {
  description = "Desired (and minimum) number of instances."
  default     = "1"
}

variable "instance_ami" {
  description = "ID of an Amazon Linux 2 AMI. (Comes with SSM agent installed)"
  default     = "ami-db51c2a2"
}

variable "instance_type" {
  description = "Type of instance to provision."
  default     = "t2.micro"
}

variable "tags" {
  description = "A map of tags (key-value pairs) passed to resources."
  type        = "map"
  default     = {}
}

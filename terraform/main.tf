terraform {
  required_version = "0.11.3"
}

provider "aws" {
  version = "1.9.0"
  region  = "eu-west-1"
}

module "ssm-example" {
  source = "./module"

  prefix = "ssm-sh-example"
  vpc_id = "<vpc-id>"

  subnet_ids = [
    "<subnet-1>",
    "<subnet-2>",
    "<subnet-3>",
  ]

  ssm_output_bucket = "<some-bucket-name>"

  tags = {
    environment = "dev"
    terraform   = "True"
  }
}

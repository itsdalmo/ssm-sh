## SSM deployment

Quick way of bootstrapping one or more instances managed by SSM,
using terraform, in order to test `ssm-sh`. It takes care of
the following:

- Creating an autoscaling group running Amazon linux 2 (includes SSM agent).
- An instance profile with the correct privileges for the SSM agent.
- A log group where each instance will send their SSM agent logs.

### Usage

Have terraform installed, configure [main.tf](./main.tf) and run the following:

```bash
terraform init
terraform apply
```

If everything deploys successfully, you should see your instances listed when
running: `ssm-sh list`.

This terraform will create a managed policy `ssm-cwl-policy` to access cloudwatch logs, 
please attach `ssm-cwl-policy` to your user or roles, if you are sending output to cloudwatch logs 
but don't have the the right permission yet.

Terraform state will be stored locally unless you add a remote backend to `main.tf`,
and when you are done testing you can tear everything down with `terraform destroy`.

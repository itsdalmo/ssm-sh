package command

import (
	"fmt"

	"github.com/itsdalmo/ssm-sh/manager"
)

type SSMOptions struct {
	ExtendOutput bool   `short:"x" long:"extend-output" description:"Extend truncated command outputs by fetching S3 objects containing full ones"`
	S3Bucket     string `short:"b" long:"s3-bucket" description:"S3 bucket in which S3 objects containing full command outputs are stored. Required when --extend-output is provided." default:""`
	S3KeyPrefix  string `short:"k" long:"s3-key-prefix" description:"Key prefix of S3 objects containing full command outputs." default:""`
}

func (o SSMOptions) Validate() error {
	if o.ExtendOutput && o.S3Bucket == "" {
		return fmt.Errorf("--s3-bucket must be a non-empty string when --extend-output is provided")
	}
	return nil
}

func (o SSMOptions) Parse() (*manager.Opts, error) {
	err := o.Validate()
	if err != nil {
		return nil, err
	}
	return &manager.Opts{
		ExtendOutput: o.ExtendOutput,
		S3Bucket:     o.S3Bucket,
		S3KeyPrefix:  o.S3KeyPrefix,
	}, nil
}

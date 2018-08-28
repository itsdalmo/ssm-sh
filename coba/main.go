package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

func main() {
	sess, err := newSession()
	if err != nil {
		os.Exit(1)
	}
	cwlClient := cloudwatchlogs.New(sess)

	stream, err := cwlClient.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String("coba"),
		LogStreamNamePrefix: aws.String("132"),
	})
	fmt.Println(len(stream.LogStreams))
}

func newSession() (*session.Session, error) {
	opts := session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}
	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

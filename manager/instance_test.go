package manager_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestInstance(t *testing.T) {
	ssmInput := &ssm.InstanceInformation{
		InstanceId:       aws.String("i-00000000000000001"),
		PlatformName:     aws.String("Amazon Linux"),
		PlatformVersion:  aws.String("1.0"),
		IPAddress:        aws.String("10.0.0.1"),
		PingStatus:       aws.String("Online"),
		LastPingDateTime: aws.Time(time.Date(2018, time.January, 27, 13, 32, 0, 0, time.UTC)),
	}

	ec2Input := &ec2.Instance{
		ImageId:  aws.String("ami-db000001"),
		Platform: aws.String("Linux"),
		State:    &ec2.InstanceState{Name: aws.String("running")},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String("instance 1"),
			},
		},
	}

	output := &manager.Instance{
		InstanceID:       "i-00000000000000001",
		Name:             "instance 1",
		State:            "running",
		ImageID:          "ami-db000001",
		Platform:         "Linux",
		PlatformName:     "Amazon Linux",
		PlatformVersion:  "1.0",
		IPAddress:        "10.0.0.1",
		PingStatus:       "Online",
		LastPingDateTime: time.Date(2018, time.January, 27, 13, 32, 0, 0, time.UTC),
	}

	t.Run("NewInstance works", func(t *testing.T) {
		expected := output
		actual := manager.NewInstance(ssmInput, ec2Input)
		assert.Equal(t, expected, actual)
	})

	t.Run("Instance Id works", func(t *testing.T) {
		expected := "i-00000000000000001"
		actual := output.ID()
		assert.Equal(t, expected, actual)
	})

	t.Run("Instance TabString works", func(t *testing.T) {
		expected := "i-00000000000000001\t|\tinstance 1\t|\trunning\t|\tami-db000001\t|\tLinux\t|\tAmazon Linux\t|\t1.0\t|\t10.0.0.1\t|\tOnline\t|\t2018-01-27 13:32"
		actual := output.TabString()
		assert.Equal(t, expected, actual)
	})
}

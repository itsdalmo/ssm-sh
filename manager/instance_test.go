package manager_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestInstance(t *testing.T) {
	input := &ssm.InstanceInformation{
		InstanceId:       aws.String("i-00000000000000001"),
		PlatformName:     aws.String("Amazon Linux"),
		PlatformVersion:  aws.String("1.0"),
		IPAddress:        aws.String("10.0.0.1"),
		LastPingDateTime: aws.Time(time.Date(2018, time.January, 27, 0, 0, 0, 0, time.UTC)),
	}

	output := &manager.Instance{
		InstanceID:       "i-00000000000000001",
		PlatformName:     "Amazon Linux",
		PlatformVersion:  "1.0",
		IPAddress:        "10.0.0.1",
		LastPingDateTime: time.Date(2018, time.January, 27, 0, 0, 0, 0, time.UTC),
	}

	t.Run("NewInstance works", func(t *testing.T) {
		expected := output
		actual := manager.NewInstance(input)
		assert.Equal(t, expected, actual)
	})

	t.Run("Instance Id works", func(t *testing.T) {
		expected := "i-00000000000000001"
		actual := output.ID()
		assert.Equal(t, expected, actual)
	})

	t.Run("Instance TabString works", func(t *testing.T) {
		expected := "i-00000000000000001\t|\tAmazon Linux\t|\t1.0\t|\t10.0.0.1\t|\t2018-01-27"
		actual := output.TabString()
		assert.Equal(t, expected, actual)
	})
}

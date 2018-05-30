package command_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/itsdalmo/ssm-sh/command"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/stretchr/testify/assert"
)

func TestPrintInstances(t *testing.T) {
	input := []*manager.Instance{
		{
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
		},
		{
			InstanceID:       "i-00000000000000002",
			Name:             "instance 2",
			State:            "running",
			ImageID:          "ami-db000002",
			Platform:         "Linux",
			PlatformName:     "Amazon Linux 2",
			PlatformVersion:  "2.0",
			IPAddress:        "10.0.0.100",
			PingStatus:       "Online",
			LastPingDateTime: time.Date(2018, time.January, 30, 13, 32, 0, 0, time.UTC),
		},
	}

	t.Run("Print works", func(t *testing.T) {
		expected := strings.TrimSpace(`
Instance ID         | Name       | State   | Image ID     | Platform | Platform Description | Version | IP         | Status | Last pinged
i-00000000000000001 | instance 1 | running | ami-db000001 | Linux    | Amazon Linux         | 1.0     | 10.0.0.1   | Online | 2018-01-27 13:32
i-00000000000000002 | instance 2 | running | ami-db000002 | Linux    | Amazon Linux 2       | 2.0     | 10.0.0.100 | Online | 2018-01-30 13:32
`)

		b := new(bytes.Buffer)
		err := command.PrintInstances(b, input)
		actual := strings.TrimSpace(b.String())
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})
}

func TestPrintCommandOutput(t *testing.T) {
	input := []*manager.CommandOutput{
		{
			InstanceID: "i-00000000000000001",
			Status:     "Success",
			Output:     "Standard output",
			Error:      nil,
		},
		{
			InstanceID: "i-00000000000000001",
			Status:     "Success",
			Output:     "Extended standard output",
			OutputUrl:  "https://s3-ap-northeast-1.amazonaws.com/mybucket/foobar/c0896747-af2b-4359-bc34-0f951ce02007/i-00000000000000001/awsrunShellScript/0.awsrunShellScript/stdout",
			Error:      nil,
		},
		{
			InstanceID: "i-00000000000000002",
			Status:     "Failed",
			Output:     "Standard error",
			Error:      nil,
		},
		{
			InstanceID: "i-00000000000000003",
			Status:     "Error",
			Output:     "",
			Error:      errors.New("error"),
		},
	}

	t.Run("Print works", func(t *testing.T) {
		expected := strings.TrimSpace(`
i-00000000000000001 - Success:
Standard output


i-00000000000000001 - Success:
Extended standard output

(Output URL: https://s3-ap-northeast-1.amazonaws.com/mybucket/foobar/c0896747-af2b-4359-bc34-0f951ce02007/i-00000000000000001/awsrunShellScript/0.awsrunShellScript/stdout)

i-00000000000000002 - Failed:
Standard error


i-00000000000000003 - Error:
error
`)

		b := new(bytes.Buffer)
		for _, instance := range input {
			err := command.PrintCommandOutput(b, instance)
			assert.Nil(t, err)
		}
		actual := strings.TrimSpace(b.String())
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})
}

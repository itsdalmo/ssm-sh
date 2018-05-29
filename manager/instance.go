package manager

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"strings"
	"time"
)

// NewInstance creates a new Instance from ssm.InstanceInformation.
func NewInstance(ssmInstance *ssm.InstanceInformation, ec2Instance *ec2.Instance) *Instance {
	var name string
	for _, tag := range ec2Instance.Tags {
		if aws.StringValue(tag.Key) == "Name" {
			name = aws.StringValue(tag.Value)
		}
	}
	return &Instance{
		InstanceID:       aws.StringValue(ssmInstance.InstanceId),
		Name:             name,
		State:            aws.StringValue(ec2Instance.State.Name),
		ImageID:          aws.StringValue(ec2Instance.ImageId),
		Platform:         aws.StringValue(ec2Instance.Platform),
		PlatformName:     aws.StringValue(ssmInstance.PlatformName),
		PlatformVersion:  aws.StringValue(ssmInstance.PlatformVersion),
		IPAddress:        aws.StringValue(ssmInstance.IPAddress),
		PingStatus:       aws.StringValue(ssmInstance.PingStatus),
		LastPingDateTime: aws.TimeValue(ssmInstance.LastPingDateTime),
	}
}

// Instance describes relevant information about an instance-id
// as collected from SSM and EC2 endpoints. And does not user pointers
// for all values.
type Instance struct {
	InstanceID       string    `json:"instanceId"`
	Name             string    `json:"name"`
	State            string    `json:"state"`
	ImageID          string    `json:"imageId"`
	Platform         string    `json:"platform"`
	PlatformName     string    `json:"platformName"`
	PlatformVersion  string    `json:"platformVersion"`
	IPAddress        string    `json:"ipAddress"`
	PingStatus       string    `json:"pingStatus"`
	LastPingDateTime time.Time `json:"lastPingDateTime"`
}

// ID returns the InstanceID of an Instance.
func (i *Instance) ID() string {
	return i.InstanceID
}

// TabString returns all field values separated by "\t|\t" for
// an instance. Use with tabwriter to output a table of instances.
func (i *Instance) TabString() string {
	var del = "|"
	var tab = "\t"

	fields := []string{
		i.InstanceID,
		i.Name,
		i.State,
		i.ImageID,
		i.Platform,
		i.PlatformName,
		i.PlatformVersion,
		i.IPAddress,
		i.PingStatus,
		i.LastPingDateTime.Format("2006-01-02 15:04"),
	}
	return strings.Join(fields, tab+del+tab)
}

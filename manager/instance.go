package manager

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"strings"
	"time"
)

// NewInstance creates a new Instance from ssm.InstanceInformation.
func NewInstance(in *ssm.InstanceInformation) *Instance {
	return &Instance{
		InstanceID:       aws.StringValue(in.InstanceId),
		PlatformName:     aws.StringValue(in.PlatformName),
		PlatformVersion:  aws.StringValue(in.PlatformVersion),
		IPAddress:        aws.StringValue(in.IPAddress),
		PingStatus:       aws.StringValue(in.PingStatus),
		LastPingDateTime: aws.TimeValue(in.LastPingDateTime),
	}
}

// Instance is a replacement for ssm.InstanceInformation which
// does not use pointers for all values.
type Instance struct {
	InstanceID       string
	PlatformName     string
	PlatformVersion  string
	IPAddress        string
	PingStatus       string
	LastPingDateTime time.Time
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
		i.PlatformName,
		i.PlatformVersion,
		i.IPAddress,
		i.PingStatus,
		i.LastPingDateTime.Format("2006-01-02 15:04"),
	}
	return strings.Join(fields, tab+del+tab)
}

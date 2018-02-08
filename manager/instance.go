package manager

import (
	"github.com/aws/aws-sdk-go/service/ssm"
	"strings"
	"time"
)

func NewInstance(in *ssm.InstanceInformation) *Instance {
	return &Instance{
		InstanceId:       *in.InstanceId,
		PlatformName:     *in.PlatformName,
		PlatformVersion:  *in.PlatformVersion,
		IPAddress:        *in.IPAddress,
		LastPingDateTime: *in.LastPingDateTime,
	}
}

type Instance struct {
	InstanceId       string
	PlatformName     string
	PlatformVersion  string
	IPAddress        string
	LastPingDateTime time.Time
}

func (self *Instance) Id() string {
	return self.InstanceId
}

func (self *Instance) TabString() string {
	var del = "|"
	var tab = "\t"

	fields := []string{
		self.InstanceId,
		self.PlatformName,
		self.PlatformVersion,
		self.IPAddress,
		self.LastPingDateTime.Format("2006-01-02"),
	}
	return strings.Join(fields, tab+del+tab)
}

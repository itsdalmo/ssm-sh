## SSM shell

Little experiment to mimic SSH by using SSM agent to send commands to
remote instances and fetching the output.

### Usage

```bash
$ dep ensure
$ go install

$ ssm-sh --help

Usage:
  ssm-sh [OPTIONS]

Application Options:
  -p, --profile=   AWS Profile to use. (If you are not using Vaulted).
  -r, --region=    Region to target. (default: eu-west-1)
  -f, --frequency= Polling frequency (millseconds to wait between requests). (default: 500)
  -t, --timeout=   Seconds to wait for command result before timing out. (default: 30)

Help Options:
  -h, --help       Show this help message
  ```


### Example

```bash
$ vaulted -n lab-admin -- ssm-sh

[INFO] 2018/01/27 16:12:22 AWS credentials loaded
Instance ID          | Platform             | Version | IP             | Last pinged
i-02162678c46646813  | Amazon Linux         | 2.0     | 172.32.18.168  | 2018-01-27

$ Chose target (instance id): i-02162678c46646813
[INFO] 2018/01/27 16:12:57 Targeting: 'i-02162678c46646813'

$ ps aux | grep agent
root       316  0.0  0.0   9152   920 ?        S    15:13   0:00 grep agent
root      3261  0.0  1.8 234052 18608 ?        Ssl  12:56   0:02 /usr/bin/amazon-ssm-agent

$ echo $HOSTNAME
ip-172-32-18-168.eu-west-1.compute.internal
```

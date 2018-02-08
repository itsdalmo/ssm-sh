## SSM shell

[![Build Status](https://travis-ci.org/itsdalmo/ssm-sh.svg?branch=master)](https://travis-ci.org/itsdalmo/ssm-sh)

Little experiment to mimic SSH by using SSM agent to send commands to
remote instances and fetching the output.

## Install

Have Go installed:

```bash
$ which go
/usr/local/bin/go

$ echo $GOPATH
/Users/dalmo/go

$ echo $PATH
# Make sure $GOPATH/bin is in your PATH.
```

Get the repository:

```bash
go get -u github.com/itsdalmo/ssm-sh
```

If everything was successful, you should have a shiny new binary:

```bash
which ssm-sh
# Should point to $GOPATH/bin/ssm-sh
```

### Usage

```bash
$ ssm-sh --help

Usage:
  ssm-sh [OPTIONS] <list | run | shell>

AWS Options:
  -p, --profile= AWS Profile to use. (If you are not using Vaulted).
  -r, --region=  Region to target. (default: eu-west-1)

Help Options:
  -h, --help     Show this help message

Available commands:
  list   List managed instances. (aliases: ls)
  run    Run a command on the targeted instances.
  shell  Start an interactive shell. (aliases: sh)
```

### List usage

```bash
$ ssm-sh list --help

...
[list command options]
      -l, --limit= Limit the number of instances printed (default: 50)
```

## Run/shell usage

```bash
$ ssm-sh run --help

...
[run command options]
      -t, --target=      One or more instance ids to target
          --target-file= Path to a file containing a list of targets.
      -i, --timeout=     Seconds to wait for command result before timing out. (default: 30)
```

### Example

```bash
$ vaulted -n lab-admin -- ssm-sh list
Instance ID         | Platform     | Version | IP            | Last pinged
i-03762678c45546813 | Amazon Linux | 2.0     | 172.53.17.163 | 2018-02-06
i-0d04464ff18b5db7d | Amazon Linux | 2.0     | 172.53.20.172 | 2018-02-06

$ vaulted -n lab-admin -- ssm-sh shell -t i-03762678c45546813 -t i-0d04464ff18b5db7d
Initialized with targets: [i-03762678c45546813 i-0d04464ff18b5db7d]
Type 'exit' to exit. Use ctrl-c to abort running commands.

$ ps aux | grep agent
i-03762678c45546813 - Success:
root      3261  0.0  1.9 243560 19668 ?        Ssl  Jan27   4:29 /usr/bin/amazon-ssm-agent
root      9058  0.0  0.0   9152   936 ?        S    15:02   0:00 grep agent

i-0d04464ff18b5db7d - Success:
root      3245  0.0  1.9 317292 19876 ?        Ssl  Feb05   0:27 /usr/bin/amazon-ssm-agent
root      4893  0.0  0.0   9152   924 ?        S    15:02   0:00 grep agent

$ echo $HOSTNAME
i-03762678c45546813 - Success:
ip-172-53-17-163.eu-west-1.compute.internal

i-0d04464ff18b5db7d - Success:
ip-172-53-20-172.eu-west-1.compute.internal
```

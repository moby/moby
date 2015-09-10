<!--[metadata]>
+++
title = "Amazon CloudWatch Logs logging driver"
description = "Describes how to use the Amazon CloudWatch Logs logging driver."
keywords = ["AWS, Amazon, CloudWatch, logging, driver"]
[menu.main]
parent = "smn_logging"
+++
<![end-metadata]-->

# Amazon CloudWatch Logs logging driver

The `awslogs` logging driver sends container logs to
[Amazon CloudWatch Logs](https://aws.amazon.com/cloudwatch/details/#log-monitoring).
Log entries can be retrieved through the [AWS Management
Console](https://console.aws.amazon.com/cloudwatch/home#logs:) or the [AWS SDKs
and Command Line Tools](http://docs.aws.amazon.com/cli/latest/reference/logs/index.html).

## Usage

You can configure the default logging driver by passing the `--log-driver`
option to the Docker daemon:

    docker --log-driver=awslogs

You can set the logging driver for a specific container by using the
`--log-driver` option to `docker run`:

    docker run --log-driver=awslogs ...

## Amazon CloudWatch Logs options

You can use the `--log-opt NAME=VALUE` flag to specify Amazon CloudWatch Logs logging driver options.

### awslogs-region

You must specify a region for the `awslogs` logging driver. You can specify the
region with either the `awslogs-region` log option or `AWS_REGION` environment
variable:

    docker run --log-driver=awslogs --log-opt awslogs-region=us-east-1 ...

### awslogs-group

You must specify a
[log group](http://docs.aws.amazon.com/AmazonCloudWatch/latest/DeveloperGuide/WhatIsCloudWatchLogs.html)
for the `awslogs` logging driver.  You can specify the log group with the
`awslogs-group` log option:

    docker run --log-driver=awslogs --log-opt awslogs-region=us-east-1 --log-opt awslogs-group=myLogGroup ...

### awslogs-stream

To configure which
[log stream](http://docs.aws.amazon.com/AmazonCloudWatch/latest/DeveloperGuide/WhatIsCloudWatchLogs.html)
should be used, you can specify the `awslogs-stream` log option.  If not
specified, the container ID is used as the log stream.

> **Note:**
> Log streams within a given log group should only be used by one container
> at a time.  Using the same log stream for multiple containers concurrently
> can cause reduced logging performance.

## Credentials

You must provide AWS credentials to the Docker daemon to use the `awslogs`
logging driver. You can provide these credentials with the `AWS_ACCESS_KEY_ID`,
`AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` environment variables, the
default AWS shared credentials file (`~/.aws/credentials` of the root user), or
(if you are running the Docker daemon on an Amazon EC2 instance) the Amazon EC2
instance profile.

Credentials must have a policy applied that allows the `logs:CreateLogStream`
and `logs:PutLogEvents` actions, as shown in the following example.

    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Action": [
            "logs:CreateLogStream",
            "logs:PutLogEvents"
          ],
          "Effect": "Allow",
          "Resource": "*"
        }
      ]
    }



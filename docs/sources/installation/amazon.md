page_title: Installation on Amazon EC2
page_description: Installation instructions for Docker on Amazon EC2.
page_keywords: amazon ec2, virtualization, cloud, docker, documentation, installation

# Amazon EC2

There are several ways to install Docker on AWS EC2. You can use Amazon Linux, which includes the Docker packages in its Software Repository, or opt for any of the other supported Linux images, for example a [*Standard Ubuntu Installation*](#standard-ubuntu-installation).

**You'll need an** [AWS account](http://aws.amazon.com/) **first, of
course.**

## Amazon QuickStart with Amazon Linux AMI 2014.09.1

The latest Amazon Linux AMI, 2014.09.1, is Docker ready. Docker packages can be installed from Amazon's provided Software
Repository.

1. **Choose an image:**
   - Launch the [Create Instance
     Wizard](https://console.aws.amazon.com/ec2/v2/home?#LaunchInstanceWizard:)
     menu on your AWS Console.
   - In the Quick Start menu, select the Amazon provided AMI for Amazon Linux 2014.09.1
   - For testing you can use the default (possibly free)
     `t2.micro` instance (more info on
     [pricing](http://aws.amazon.com/ec2/pricing/)).
   - Click the `Next: Configure Instance Details`
      button at the bottom right.
2. After a few more standard choices where defaults are probably ok,
   your Amazon Linux instance should be running!
3. SSH to your instance to install Docker :
   `ssh -i <path to your private key> ec2-user@<your public IP address>`
4. Once connected to the instance, type
    `sudo yum install -y docker ; sudo service docker start`
 to install and start Docker

**If this is your first AWS instance, you may need to set up your Security Group to allow SSH.** By default all incoming ports to your new instance will be blocked by the AWS Security Group, so you might just get timeouts when you try to connect.

Once you`ve got Docker installed, you're ready to try it out – head on
over to the [User Guide](/userguide).

## Standard Ubuntu Installation

If you want a more hands-on installation, then you can follow the
[*Ubuntu*](/installation/ubuntulinux) instructions installing Docker
on any EC2 instance running Ubuntu. Just follow Step 1 from the Amazon
QuickStart above to pick an image (or use one of your
own) and skip the step with the *User Data*. Then continue with the
[*Ubuntu*](/installation/ubuntulinux) instructions.

Continue with the [User Guide](/userguide/).

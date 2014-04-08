page_title: Installation on Amazon EC2
page_description: Please note this project is currently under heavy development. It should not be used in production. 
page_keywords: amazon ec2, virtualization, cloud, docker, documentation, installation

# Amazon EC2

Note

Docker is still under heavy development! We don’t recommend using it in
production yet, but we’re getting closer with each release. Please see
our blog post, ["Getting to Docker
1.0"](http://blog.docker.io/2013/08/getting-to-docker-1-0/)

There are several ways to install Docker on AWS EC2:

-   [*Amazon QuickStart (Release Candidate - March
    2014)*](#amazonquickstart-new) or
-   [*Amazon QuickStart*](#amazonquickstart) or
-   [*Standard Ubuntu Installation*](#amazonstandard)

**You’ll need an** [AWS account](http://aws.amazon.com/) **first, of
course.**

## Amazon QuickStart

1.  **Choose an image:**
    -   Launch the [Create Instance
        Wizard](https://console.aws.amazon.com/ec2/v2/home?#LaunchInstanceWizard:)
        menu on your AWS Console.
    -   Click the `Select` button for a 64Bit Ubuntu
        image. For example: Ubuntu Server 12.04.3 LTS
    -   For testing you can use the default (possibly free)
        `t1.micro` instance (more info on
        [pricing](http://aws.amazon.com/en/ec2/pricing/)).
    -   Click the `Next: Configure Instance Details`
        button at the bottom right.

2.  **Tell CloudInit to install Docker:**
    -   When you’re on the "Configure Instance Details" step, expand the
        "Advanced Details" section.
    -   Under "User data", select "As text".
    -   Enter `#include https://get.docker.io` into
        the instance *User Data*.
        [CloudInit](https://help.ubuntu.com/community/CloudInit) is part
        of the Ubuntu image you chose; it will bootstrap Docker by
        running the shell script located at this URL.

3.  After a few more standard choices where defaults are probably ok,
    your AWS Ubuntu instance with Docker should be running!

**If this is your first AWS instance, you may need to set up your
Security Group to allow SSH.** By default all incoming ports to your new
instance will be blocked by the AWS Security Group, so you might just
get timeouts when you try to connect.

Installing with `get.docker.io` (as above) will
create a service named `lxc-docker`. It will also
set up a [*docker group*](../binaries/#dockergroup) and you may want to
add the *ubuntu* user to it so that you don’t have to use
`sudo` for every Docker command.

Once you’ve got Docker installed, you’re ready to try it out – head on
over to the [*First steps with Docker*](../../use/basics/) or
[*Examples*](../../examples/) section.

## Amazon QuickStart (Release Candidate - March 2014)

Amazon just published new Docker-ready AMIs (2014.03 Release Candidate).
Docker packages can now be installed from Amazon’s provided Software
Repository.

1.  **Choose an image:**
    -   Launch the [Create Instance
        Wizard](https://console.aws.amazon.com/ec2/v2/home?#LaunchInstanceWizard:)
        menu on your AWS Console.
    -   Click the `Community AMI` menu option on the
        left side
    -   Search for ‘2014.03’ and select one of the Amazon provided AMI,
        for example `amzn-ami-pv-2014.03.rc-0.x86_64-ebs`
        .literal}
    -   For testing you can use the default (possibly free)
        `t1.micro` instance (more info on
        [pricing](http://aws.amazon.com/en/ec2/pricing/)).
    -   Click the `Next: Configure Instance Details`
        button at the bottom right.

2.  After a few more standard choices where defaults are probably ok,
    your Amazon Linux instance should be running!
3.  SSH to your instance to install Docker :
    `ssh -i <path to your private key> ec2-user@<your public IP address>`

4.  Once connected to the instance, type
    `sudo yum install -y docker ; sudo service docker start`
 to install and start Docker

## Standard Ubuntu Installation

If you want a more hands-on installation, then you can follow the
[*Ubuntu*](../ubuntulinux/#ubuntu-linux) instructions installing Docker
on any EC2 instance running Ubuntu. Just follow Step 1 from [*Amazon
QuickStart*](#amazonquickstart) to pick an image (or use one of your
own) and skip the step with the *User Data*. Then continue with the
[*Ubuntu*](../ubuntulinux/#ubuntu-linux) instructions.

Continue with the [*Hello
World*](../../examples/hello_world/#hello-world) example.

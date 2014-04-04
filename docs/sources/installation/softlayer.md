page_title: Installation on IBM SoftLayer 
page_description: Please note this project is currently under heavy development. It should not be used in production. 
page_keywords: IBM SoftLayer, virtualization, cloud, docker, documentation, installation

# IBM SoftLayer

Note

Docker is still under heavy development! We don’t recommend using it in
production yet, but we’re getting closer with each release. Please see
our blog post, ["Getting to Docker
1.0"](http://blog.docker.io/2013/08/getting-to-docker-1-0/)

## IBM SoftLayer QuickStart

1.  Create an [IBM SoftLayer
    account](https://www.softlayer.com/cloudlayer/).
2.  Log in to the [SoftLayer
    Console](https://control.softlayer.com/devices/).
3.  Go to [Order Hourly Computing Instance
    Wizard](https://manage.softlayer.com/Sales/orderHourlyComputingInstance)
    on your SoftLayer Console.
4.  Create a new *CloudLayer Computing Instance* (CCI) using the default
    values for all the fields and choose:

-   *First Available* as `Datacenter` and
-   *Ubuntu Linux 12.04 LTS Precise Pangolin - Minimal Install (64 bit)*
    as `Operating System`.

5.  Click the *Continue Your Order* button at the bottom right and
    select *Go to checkout*.
6.  Insert the required *User Metadata* and place the order.
7.  Then continue with the [*Ubuntu*](../ubuntulinux/#ubuntu-linux)
    instructions.

Continue with the [*Hello
World*](../../examples/hello_world/#hello-world) example.

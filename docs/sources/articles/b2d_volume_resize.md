page_title: Resizing a Boot2Docker Volume	
page_description: Resizing a Boot2Docker Volume in VirtualBox with GParted
page_keywords: boot2docker, volume, virtualbox

# Getting “no space left on device” Errors with Boot2Docker?

If you're using Boot2Docker with a large number of images, or the images you're working 
with are very large, you might run into trouble if the Boot2Docker VM's volume runs out of 
space. The solution is to increase the volume size by first cloning it, then resizing it 
using a disk partitioning tool. We'll use [GParted](http://gparted.sourceforge.net/download.php/index.php) 
since it's a free ISO and works well with VirtualBox.

## 1. Stop Boot2Docker’s VM

    $ boot2docker stop 

Boot2Docker ships with a VMDK image, which can’t be resized by VirtualBox’s native tools. We will instead 
create a VDI volume and clone the VMDK volume to it.

## 2. Clone the VMDK image to a VDI image

Using the command line VirtualBox tools, clone the VMDK image to a VDI image:

    $ vboxmanage clonehd /full/path/to/boot2docker-hd.vmdk /full/path/to/<newVDIimage>.vdi -—format VDI -—variant Standard

## 3. Resize the new clone volume

Choose a size that will be appropriate for your needs. If you’re spinning up a lot of containers, 
or your containers are particularly large, larger will be better:

    $ vboxmanage modifyhd /full/path/to/<newVDIimage>.vdi —-resize <size in MB>

## 4. Download a disk partitioning tool ISO 

To resize the volume, you'll need a disk partitioning tool like [GParted](http://gparted.sourceforge.net/download.php/). 
Once you've downloaded the tool, add the ISO to the Boot2Docker VM’s IDE bus. You might need to 
create the bus before you can add the ISO.

<img src="/articles/b2d_volume_images/add_new_controller.png"></br>
<img src="/articles/b2d_volume_images/add_cd.png">

## 5. Add the new VDI image 

to the Boot2Docker image in VirtualBox.

<img src="/articles/b2d_volume_images/add_volume.png">

## 6. Verify the boot order

In the **System** settings for the Boot2Docker VM, make sure that **CD/DVD** is the at the top of the **Boot Order** list.

<img src="/articles/b2d_volume_images/boot_order.png">

## 7. Boot to the disk partitioning ISO

Manually start the Boot2Docker VM, and the disk partitioning ISO should start up. 
Using GParted, choose the **GParted Live (default settings)** option. Choose the 
default keyboard, language, and XWindows settings, and the GParted tool will start 
up and display the new VDI volume you created. Right click on the VDI and choose 
**Resize/Move**. Drag the slider representing the volume to its maximum size, click 
**Resize/Move**, and then **Apply**. Quit GParted and shut down the VM. Remove 
the GParted ISO from the IDE controller for the Boot2Docker VM in VirtualBox.

## 8. Start the Boot2Docker VM 

Either directly in VirtualBox or using the command line (`boot2docker start`), start the Boot2Docker 
VM to make sure the volume changes took effect.

You’re done!


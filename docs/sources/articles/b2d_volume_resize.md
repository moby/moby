page_title: Resizing a Boot2Docker Volume	
page_description: Resizing a Boot2Docker Volume in VirtualBox with GParted
page_keywords: boot2docker, volume, virtualbox

# Getting “no space left on device” errors with Boot2Docker?

If you're using Boot2Docker with a large number of images, or the images you're
working with are very large, your pulls might start failing with "no space left 
on device" errors when the Boot2Docker volume fills up. The solution is to 
increase the volume size by first cloning it, then resizing it using a disk 
partitioning tool. 

We recommend [GParted](http://gparted.sourceforge.net/download.php/index.php).
The tool comes as a bootable ISO, is a free download, and works well with 
VirtualBox.

## 1. Stop Boot2Docker

Issue the command to stop the Boot2Docker VM on the command line:

    $ boot2docker stop

## 2. Clone the VMDK image to a VDI image

Boot2Docker ships with a VMDK image, which can’t be resized by VirtualBox’s 
native tools. We will instead create a VDI volume and clone the VMDK volume to 
it. 

Using the command line VirtualBox tools, clone the VMDK image to a VDI image:

    $ vboxmanage clonehd /full/path/to/boot2docker-hd.vmdk /full/path/to/<newVDIimage>.vdi --format VDI --variant Standard

## 3. Resize the VDI volume

Choose a size that will be appropriate for your needs. If you’re spinning up a 
lot of containers, or your containers are particularly large, larger will be 
better:

    $ vboxmanage modifyhd /full/path/to/<newVDIimage>.vdi --resize <size in MB>

## 4. Download a disk partitioning tool ISO 

To resize the volume, we'll use [GParted](http://gparted.sourceforge.net/download.php/). 
Once you've downloaded the tool, add the ISO to the Boot2Docker VM IDE bus. 
You might need to create the bus before you can add the ISO. 

> **Note:** 
> It's important that you choose a partitioning tool that is available as an ISO so 
> that the Boot2Docker VM can be booted with it.

<table>
	<tr>
		<td><img src="/articles/b2d_volume_images/add_new_controller.png"><br><br></td>
	</tr>
	<tr>
		<td><img src="/articles/b2d_volume_images/add_cd.png"></td>
	</tr>
</table>

## 5. Add the new VDI image 

In the settings for the Boot2Docker image in VirtualBox, remove the VMDK image 
from the SATA contoller and add the VDI image.

<img src="/articles/b2d_volume_images/add_volume.png">

## 6. Verify the boot order

In the **System** settings for the Boot2Docker VM, make sure that **CD/DVD** is 
at the top of the **Boot Order** list.

<img src="/articles/b2d_volume_images/boot_order.png">

## 7. Boot to the disk partitioning ISO

Manually start the Boot2Docker VM in VirtualBox, and the disk partitioning ISO 
should start up. Using GParted, choose the **GParted Live (default settings)** 
option. Choose the default keyboard, language, and XWindows settings, and the 
GParted tool will start up and display the VDI volume you created. Right click 
on the VDI and choose **Resize/Move**. 

<img src="/articles/b2d_volume_images/gparted.png">

Drag the slider representing the volume to the maximum available size, click 
**Resize/Move**, and then **Apply**. 

<img src="/articles/b2d_volume_images/gparted2.png">

Quit GParted and shut down the VM. Remove the GParted ISO from the IDE controller 
for the Boot2Docker VM in VirtualBox.

## 8. Start the Boot2Docker VM 

Fire up the Boot2Docker VM manually in VirtualBox. The VM should log in 
automatically, but if it doesn't, the credentials are `docker/tcuser`. Using 
the `df -h` command, verify that your changes took effect.

<img src="/articles/b2d_volume_images/verify.png">

You’re done!


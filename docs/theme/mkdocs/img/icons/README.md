### About the images 

Generally the icons are created in .svg, because it is a nicer format. Then we can easily convert them to .png as required.

Using imagemagick; mogrify:

mogrify -background none -format png *.svg

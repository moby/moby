package containerd

// LogImageEvent generates an event related to an image with only the
// default attributes.
func (i *ImageService) LogImageEvent(imageID, refName, action string) {
	panic("not implemented")
}

// LogImageEventWithAttributes generates an event related to an image with
// specific given attributes.
func (i *ImageService) LogImageEventWithAttributes(imageID, refName, action string, attributes map[string]string) {
	panic("not implemented")
}

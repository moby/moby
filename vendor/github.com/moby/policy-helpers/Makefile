.PHONY: all
all:

.PHONY: vendor
vendor:
	$(eval $@_TMP_OUT := $(shell mktemp -d -t policy-helpers-output.XXXXXXXXXX))
	docker buildx bake --set "*.output=type=local,dest=$($@_TMP_OUT)" vendor
	rm -rf ./vendor
	cp -R "$($@_TMP_OUT)"/* ./
	rm -rf "$($@_TMP_OUT)"/*

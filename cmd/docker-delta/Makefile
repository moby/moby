.PHONY: build install clean

PLUGIN_NAME = docker-delta
BUILD_DIR = ../../bundles/binary-daemon

build:
	@echo "Building Docker CLI plugin..."
	go build -o $(BUILD_DIR)/$(PLUGIN_NAME) docker-delta-plugin.go

install: build
	@echo "Installing Docker CLI plugin..."
	@mkdir -p ~/.docker/cli-plugins
	@cp $(BUILD_DIR)/$(PLUGIN_NAME) ~/.docker/cli-plugins/$(PLUGIN_NAME)
	@chmod +x ~/.docker/cli-plugins/$(PLUGIN_NAME)
	@echo "Plugin installed! Try: docker delta --help"

uninstall:
	@echo "Uninstalling Docker CLI plugin..."
	@rm -f ~/.docker/cli-plugins/$(PLUGIN_NAME)
	@echo "Plugin uninstalled"

clean:
	@rm -f $(BUILD_DIR)/$(PLUGIN_NAME)

test-plugin:
	@echo "Testing plugin installation..."
	@docker delta --help || echo "Plugin not working, check installation"

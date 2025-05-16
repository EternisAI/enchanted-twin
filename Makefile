APP_DIR = app
BACKEND_DIR = backend/golang
RESOURCES_DIR = $(APP_DIR)/resources
PODMAN_VERSION = 5.5.0

BUILD_TARGETS = mac-silicon linux

BINARY_mac-silicon = enchanted-twin-darwin-arm64
BINARY_windows = enchanted-twin-windows-amd64.exe
BINARY_linux = enchanted-twin-linux-amd64

TARGET_mac-silicon = enchanted-twin
TARGET_windows = enchanted-twin.exe
TARGET_linux = enchanted-twin

BUILD_CMD_mac-silicon = build:mac
BUILD_CMD_windows = build:win
BUILD_CMD_linux = build:linux

PODMAN_URL_mac-silicon = https://github.com/containers/podman/releases/download/v$(PODMAN_VERSION)/podman-installer-macos-arm64.pkg
PODMAN_URL_mac-intel = https://github.com/containers/podman/releases/download/v$(PODMAN_VERSION)/podman-installer-macos-amd64.pkg
PODMAN_URL_linux = https://github.com/containers/podman/releases/download/v$(PODMAN_VERSION)/podman-v$(PODMAN_VERSION)-linux-amd64.rpm
PODMAN_URL_windows = https://github.com/containers/podman/releases/download/v$(PODMAN_VERSION)/podman-installer-windows-amd64.exe

PODMAN_INSTALLER_mac-silicon = podman-installer-macos-arm64.pkg
PODMAN_INSTALLER_mac-intel = podman-installer-macos-amd64.pkg
PODMAN_INSTALLER_linux = podman-installer-linux-amd64.rpm
PODMAN_INSTALLER_windows = podman-installer-windows-amd64.exe

define build_recipe
build-$(1):
	rm -f $(RESOURCES_DIR)/$(TARGET_$(1))
	cd $(APP_DIR)/ && pnpm install
	cd $(BACKEND_DIR) && make release
	mkdir -p $(RESOURCES_DIR)
	cp $(BACKEND_DIR)/bin/$(BINARY_$(1)) $(RESOURCES_DIR)/$(TARGET_$(1))
	$(if $(findstring windows,$(1)),,chmod +x $(RESOURCES_DIR)/$(TARGET_$(1)))
	$(MAKE) download-podman-$(1)
	cd $(APP_DIR) && pnpm $(BUILD_CMD_$(1))
endef

$(foreach target,$(BUILD_TARGETS),$(eval $(call build_recipe,$(target))))

build-all: $(addprefix build-,$(BUILD_TARGETS))

.PHONY: $(addprefix build-,$(BUILD_TARGETS)) build-all $(addprefix download-podman-,$(BUILD_TARGETS))


download-podman-mac-silicon:
	mkdir -p $(RESOURCES_DIR)
	curl -L $(PODMAN_URL_mac-silicon) -o $(RESOURCES_DIR)/$(PODMAN_INSTALLER_mac-silicon)

download-podman-mac-intel:
	mkdir -p $(RESOURCES_DIR)
	curl -L $(PODMAN_URL_mac-intel) -o $(RESOURCES_DIR)/$(PODMAN_INSTALLER_mac-intel)

download-podman-linux:
	mkdir -p $(RESOURCES_DIR)
	curl -L $(PODMAN_URL_linux) -o $(RESOURCES_DIR)/$(PODMAN_INSTALLER_linux)

download-podman-windows:
	mkdir -p $(RESOURCES_DIR)
	curl -L $(PODMAN_URL_windows) -o $(RESOURCES_DIR)/$(PODMAN_INSTALLER_windows)


download-podman-all: download-podman-mac-silicon download-podman-mac-intel download-podman-linux download-podman-windows
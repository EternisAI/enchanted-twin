APP_DIR = app
BACKEND_DIR = backend/golang
RESOURCES_DIR = $(APP_DIR)/resources

BUILD_TARGETS = mac-silicon mac-intel windows linux

BINARY_mac-silicon = enchanted-twin-darwin-arm64
BINARY_mac-intel = enchanted-twin-darwin-amd64
BINARY_windows = enchanted-twin-windows-amd64.exe
BINARY_linux = enchanted-twin-linux-amd64

TARGET_mac-silicon = enchanted-twin
TARGET_mac-intel = enchanted-twin
TARGET_windows = enchanted-twin.exe
TARGET_linux = enchanted-twin

BUILD_CMD_mac-silicon = build:mac
BUILD_CMD_mac-intel = build:mac
BUILD_CMD_windows = build:win
BUILD_CMD_linux = build:linux

# Common build recipe
define build_recipe
build-$(1):
	rm -f $(RESOURCES_DIR)/$(TARGET_$(1))
	cd $(APP_DIR)/ && pnpm install
	cd $(BACKEND_DIR) && make release
	mkdir -p $(RESOURCES_DIR)
	cp $(BACKEND_DIR)/bin/$(BINARY_$(1)) $(RESOURCES_DIR)/$(TARGET_$(1))
	$(if $(findstring windows,$(1)),,chmod +x $(RESOURCES_DIR)/$(TARGET_$(1)))
	cd $(APP_DIR) && pnpm $(BUILD_CMD_$(1))
endef

$(foreach target,$(BUILD_TARGETS),$(eval $(call build_recipe,$(target))))

build-all: $(addprefix build-,$(BUILD_TARGETS))

.PHONY: $(addprefix build-,$(BUILD_TARGETS)) build-all
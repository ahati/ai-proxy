.PHONY: build install

BINARY_NAME=ai-proxy
INSTALL_DIR=/home/hati/.local/bin
BUILD_DIR=.
CONFIG_DIR=$(HOME)/.config/ai-proxy
CONFIG_FILE=$(CONFIG_DIR)/config.json

build:
	go build -o $(BINARY_NAME) .

install: $(CONFIG_DIR)
	install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	install -m 644 test-config.json $(CONFIG_FILE)

$(CONFIG_DIR):
	mkdir -p $(CONFIG_DIR)

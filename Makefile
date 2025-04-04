PROJECT_NAME := sichek
GO := go
INSTALL_DIR := /usr/sbin

all:
	mkdir -p build/bin/
	GOOS=linux GOARCH=amd64 $(GO) build -o build/bin/$(PROJECT_NAME) cmd/main.go

debug:
	mkdir -p build/bin/
	GOOS=linux GOARCH=amd64 $(GO) build -gcflags "all=-N -l" -o build/bin/$(PROJECT_NAME) cmd/main.go

clean:
	rm -f build/bin/*

install: all
	# Install the binary to the specified directory
	cp build/bin/$(PROJECT_NAME) $(INSTALL_DIR)/$(PROJECT_NAME)
	@echo "Installed $(PROJECT_NAME) to $(INSTALL_DIR)/$(PROJECT_NAME)"
# Variables
BINARY_NAME=momirbox
PI_USER=momir
PI_ADDR=raspberrypi.local
PI_PATH=/home/pi/momirbox
GO_FILES=$(shell find . -name "*.go")

.PHONY: build upload deploy clean

# Build for 32-bit ARM (Raspberry Pi Zero 2W running 32-bit OS)
build: $(GO_FILES)
	GOOS=linux GOARCH=arm GOARM=7 go build -o bin/$(BINARY_NAME) ./main.go

# Upload binary and assets to the Pi
upload: build
	ssh $(PI_USER)@$(PI_ADDR) "mkdir -p $(PI_PATH)/assets $(PI_PATH)/icons"
	scp bin/$(BINARY_NAME) $(PI_USER)@$(PI_ADDR):$(PI_PATH)/
	scp -r assets/* $(PI_USER)@$(PI_ADDR):$(PI_PATH)/assets/
	scp -r icons/* $(PI_USER)@$(PI_ADDR):$(PI_PATH)/icons/

# Build, upload, and restart the systemd service
deploy: upload
	ssh $(PI_USER)@$(PI_ADDR) "sudo systemctl restart $(BINARY_NAME).service"

clean:
	rm -rf bin/
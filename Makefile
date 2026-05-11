include .env
export

BINARY_NAME=$(APP_NAME)
SYNC_NAME=momir-sync
GO_FILES=$(shell find . -name "*.go")

.PHONY: build-pi build-sync deploy clean

build-pi: $(GO_FILES)
	GOOS=linux GOARCH=arm64 go build -tags pi -o bin/$(BINARY_NAME) ./cmd/momirbox

build-sync: $(GO_FILES)
	go build -o bin/$(SYNC_NAME) ./cmd/momir-sync

deploy: build-pi
	ssh -A $(PI_USER)@$(PI_HOST) "mkdir -p $(PI_DEST)/assets $(PI_DEST)/icons"
	scp bin/$(BINARY_NAME) $(PI_USER)@$(PI_HOST):$(PI_DEST)/
	rsync -avz --delete assets/ $(PI_USER)@$(PI_HOST):$(PI_DEST)/assets/
	rsync -avz --delete icons/ $(PI_USER)@$(PI_HOST):$(PI_DEST)/icons/

clean:
	rm -rf bin/
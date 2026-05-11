#!/bin/bash

cd "$(dirname "$0")"

if [ -f ../.env ]; then
    export $(cat ../.env | xargs)
else
    echo ".env file not found!"
    exit 1
fi

cleanup() {
    echo -e "\n--- Stopping $APP_NAME on Raspberry Pi ---"
    ssh $PI_USER@$PI_HOST "pkill $APP_NAME" > /dev/null 2>&1
    exit 0
}

trap cleanup SIGINT SIGTERM

echo "--- Building for Raspberry Pi ---"
GOOS=linux GOARCH=arm64 go build -tags pi -o ../bin/$APP_NAME ../cmd/momirbox

echo "--- Deploying to Pi ---"
ssh -A $PI_USER@$PI_HOST "mkdir -p $PI_DEST"
scp ../bin/$APP_NAME $PI_USER@$PI_HOST:$PI_DEST/

# Sync assets but skip the heavy data directory (handled by sync_images.sh)
rsync -avz ../assets/ $PI_USER@$PI_HOST:$PI_DEST/assets/

echo "--- Launching App ---"
ssh -t $PI_USER@$PI_HOST "pkill $APP_NAME || true; cd $PI_DEST && ./$APP_NAME"
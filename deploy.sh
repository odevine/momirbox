#!/bin/bash

# Configuration
APP_NAME="momirbox"
PI_USER="momir"
PI_HOST="YOUR_RASPBERRY_PI_IP"
PI_DEST="/home/momir/momirbox"

cleanup() {
    echo -e "\n--- Stopping $APP_NAME on Raspberry Pi ---"
    ssh $PI_USER@$PI_HOST "pkill $APP_NAME" > /dev/null 2>&1
    echo "App terminated. Exiting."
    exit 0
}

trap cleanup SIGINT SIGTERM

echo "--- Building for Raspberry Pi (ARMv7) ---"
GOOS=linux GOARCH=arm GOARM=7 go build -tags pi -o $APP_NAME .

echo "--- Syncing Assets and Binary ---"
ssh $PI_USER@$PI_HOST "mkdir -p $PI_DEST"
scp $APP_NAME $PI_USER@$PI_HOST:$PI_DEST/
rsync -avz assets/ $PI_USER@$PI_HOST:$PI_DEST/assets/

echo "--- Done! ---"

echo "--- Launching App (Press Ctrl+C here to stop it) ---"
ssh -t $PI_USER@$PI_HOST "pkill $APP_NAME || true; cd $PI_DEST && ./$APP_NAME"
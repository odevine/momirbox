#!/bin/bash

cd "$(dirname "$0")"

if [ -f ../.env ]; then
    export $(cat ../.env | xargs)
else
    echo ".env file not found at root!"
    exit 1
fi

LOCAL_DATA_DIR="../data/images/"

echo "--- Starting Binary Asset Sync ---"
echo "Target: $PI_USER@$PI_HOST:$PI_DEST/data/images/"

ssh $PI_USER@$PI_HOST "mkdir -p $PI_DEST/data/images"

# Sync only .bin files and directories, deleting removed items on the destination
rsync -avzP --delete-during \
    --include="*/" \
    --include="*.bin" \
    --exclude="*" \
    "$LOCAL_DATA_DIR" $PI_USER@$PI_HOST:"$PI_DEST/data/images/"

echo "--- Sync Complete! ---"
#!/bin/bash

cd "$(dirname "$0")"

if [ -f ../.env ]; then
    export $(cat ../.env | xargs)
else
    echo ".env file not found!"
    exit 1
fi

# Relative path to local data directory
LOCAL_DATA_DIR="../data/"

echo "--- Starting Fast Image Sync ---"
echo "Local Source:  $LOCAL_DATA_DIR"
echo "Remote Dest:   $PI_USER@$PI_HOST:$PI_DEST/data"

# Ensure the destination directory exists
ssh $PI_USER@$PI_HOST "mkdir -p $PI_DEST/data"

# Sync command:
# -a: Archive mode (preserves permissions/times)
# -v: Verbose (shows files as they transfer)
# -z: Compress (speeds up transfer over Wi-Fi)
# -P: Shows progress bar for large transfers
rsync -avzP --delete-during --bwlimit=1000 "$LOCAL_DATA_DIR" $PI_USER@$PI_HOST:"$PI_DEST/data"

echo "--- Sync Complete! ---"
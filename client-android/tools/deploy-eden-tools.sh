#!/bin/bash
# Deploy hactool and icon extraction script to an Android device via adb.
# Usage: ./deploy-eden-tools.sh
#        ./deploy-eden-tools.sh -s <serial>   (for specific device)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ADB_ARGS=""
if [ "$1" = "-s" ] && [ -n "$2" ]; then
    ADB_ARGS="-s $2"
fi

echo "Pushing hactool..."
adb $ADB_ARGS push "$SCRIPT_DIR/hactool-aarch64" /data/local/tmp/hactool
adb $ADB_ARGS shell chmod 755 /data/local/tmp/hactool

echo "Pushing extract_eden_icons.sh..."
adb $ADB_ARGS push "$SCRIPT_DIR/extract_eden_icons.sh" /data/local/tmp/extract_eden_icons.sh
adb $ADB_ARGS shell chmod 755 /data/local/tmp/extract_eden_icons.sh

echo "Running icon extraction..."
adb $ADB_ARGS shell su -c /data/local/tmp/extract_eden_icons.sh

echo "Done."

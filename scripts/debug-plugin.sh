#!/bin/bash
if [ "$1" == "-h" ]; then
  echo "Usage: ${BASH_SOURCE[0]} [plugin name] [debug port]"
  exit
fi
DEBUG_PORT="${2:-2345}"
PLUGIN_NAME="${1:-plugin}"
PLUGIN_PID=`pgrep ${PLUGIN_NAME}`
dlv attach ${PLUGIN_PID} --headless --listen=:${DEBUG_PORT} --api-version 2 --log
pkill dlv

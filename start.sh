#!/bin/bash

if [ "$DEBUG" = "true" ]; then
    echo "Starting in debug mode..."
    dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./tmp/main
else
    echo "Starting with hot reload..."
    air -c .air.toml
fi
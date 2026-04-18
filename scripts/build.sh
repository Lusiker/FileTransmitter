#!/bin/bash

# Build script for FileTransmitter

set -e

VERSION=${VERSION:-"dev"}
BUILD_DIR="bin"
WEB_DIR="web"

echo "Building FileTransmitter..."

# Build frontend
echo "Building frontend..."
cd $WEB_DIR
npm install
npm run build
cd ..

# Build backend
echo "Building backend..."
mkdir -p $BUILD_DIR

# Linux amd64
echo "Building for Linux (amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $BUILD_DIR/filetransmitter_linux_amd64 ./cmd/server

# Linux arm64
echo "Building for Linux (arm64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $BUILD_DIR/filetransmitter_linux_arm64 ./cmd/server

# Copy frontend dist to bin
cp -r $WEB_DIR/dist $BUILD_DIR/web

echo "Build complete!"
echo "Binaries in $BUILD_DIR:"
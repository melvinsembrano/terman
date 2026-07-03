#!/usr/bin/env bash

# build binaries for different platforms

# build linux binary
echo "Building linux binaries..."
GOOS=linux GOARCH=amd64 go build -o terman-linux main.go
GOOS=linux GOARCH=arm64 go build -o terman-linux-arm main.go
echo "Linux binary built successfully."

#build windows binary
echo "Building windows binary..."
GOOS=windows GOARCH=amd64 go build -o terman.exe main.go
echo "Windows binary built successfully."

# build mac binaries
echo "Building mac binary..."
GOOS=darwin GOARCH=amd64 go build -o terman-mac main.go
GOOS=darwin GOARCH=arm64 go build -o terman-mac-arm main.go
echo "Mac binary built successfully."


echo "All binaries built successfully."

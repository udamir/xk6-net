#!/bin/bash

# Build script for xk6-net extension

set -e

echo "Building k6 with xk6-net extension..."

# Check if xk6 is installed
if ! command -v xk6 &> /dev/null; then
    echo "xk6 not found. Installing..."
    go install go.k6.io/xk6/cmd/xk6@latest
fi

# Build k6 with xk6-net extension
xk6 build --with github.com/udamir/xk6-net=.

echo "Build complete! k6 binary created with xk6-net extension."
echo "Usage: ./k6 run your-script.js"

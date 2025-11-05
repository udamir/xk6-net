#!/bin/bash

# Simple build script for xk6-net extension with verification

set -euo pipefail

echo "Installing xk6 (ensures binary is available)..."
go install go.k6.io/xk6/cmd/xk6@latest

# Resolve xk6 binary (prefer GOBIN, then GOPATH/bin)
X="$(go env GOBIN 2>/dev/null)/xk6"
if [ ! -x "$X" ]; then
  X="$(go env GOPATH 2>/dev/null)/bin/xk6"
fi
if [ ! -x "$X" ]; then
  echo "xk6 binary not found after install. Add \"$(go env GOPATH)/bin\" to PATH or set GOBIN."
  exit 1
fi

echo "Building k6 with local xk6-net module..."
"$X" build --with github.com/udamir/xk6-net=.

echo "Placing k6 binary into ./bin/k6 ..."
mkdir -p ./bin
if [ -f ./k6 ]; then
  mv -f ./k6 ./bin/k6
elif [ -f ./k6.exe ]; then
  mv -f ./k6.exe ./bin/k6.exe
fi

K6_BIN="./bin/k6"
[ -x "$K6_BIN" ] || K6_BIN="./k6"
[ -x "$K6_BIN" ] || { echo "k6 binary not found after build"; exit 1; }

echo "Verifying xk6-net is loadable..."
"$K6_BIN" run - <<'EOF'
import net from 'k6/x/net';
export default function () {
  const s = new net.Socket();
  console.log('xk6-net loaded:', typeof s.connect === 'function');
}
EOF

echo "Build and verification complete. Use: $K6_BIN run <script.js>"

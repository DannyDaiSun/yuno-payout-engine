#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
echo "==> Running unit tests"
go test -race -count=1 ./... -v
echo "==> All unit tests passed."

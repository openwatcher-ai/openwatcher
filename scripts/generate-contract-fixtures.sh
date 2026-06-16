#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"
UPDATE_OPENWATCHER_CONTRACT_FIXTURES=1 go test ./internal/server ./desktop-app -run 'Test.*ContractFixture' -count=1

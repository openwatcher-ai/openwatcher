#!/usr/bin/env bash
set -euo pipefail

"$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/prepare-desktop-bundled-deps.sh" windows-amd64

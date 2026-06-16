#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${OPENWATCHER_PUBLIC_RELEASE_DIR:-$ROOT_DIR/dist/public-release}"
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release-common.sh"

ensure_release_dir

FILES=()
while IFS= read -r path; do
  FILES+=("$path")
done < <(find "$RELEASE_DIR" -maxdepth 1 -type f ! -name 'checksums.txt' | sort)

[[ ${#FILES[@]} -gt 0 ]] || die "发布目录没有可生成校验和的文件：$RELEASE_DIR"

CHECKSUMS_PATH="$RELEASE_DIR/checksums.txt"
: >"$CHECKSUMS_PATH"

for path in "${FILES[@]}"; do
  rel_path="${path#$RELEASE_DIR/}"
  sha="$(sha256_file "$path")"
  printf '%s  %s\n' "$sha" "$rel_path" >>"$CHECKSUMS_PATH"
done

note "已生成：$CHECKSUMS_PATH"

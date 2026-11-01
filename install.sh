#!/usr/bin/env bash
set -euo pipefail

case "$(uname -s)/$(uname -m)" in
  Linux/x86_64) target=linux_amd64 ;;
  Linux/aarch64|Linux/arm64) target=linux_arm64 ;;
  Linux/armv7l) target=linux_armv7 ;;
  Linux/i?86) target=linux_386 ;;
  Darwin/x86_64) target=darwin_amd64 ;;
  Darwin/arm64) target=darwin_arm64 ;;
  *)
    echo "Unsupported platform: $(uname -s)/$(uname -m)" >&2
    exit 1
    ;;
esac

for command in curl jq tar awk; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "Required command not found: $command" >&2
    exit 1
  fi
done

if command -v sha256sum >/dev/null 2>&1; then
  hash_command=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  hash_command=shasum
else
  echo "Required command not found: sha256sum or shasum" >&2
  exit 1
fi

dogu_home=${DOGU_HOME:-"$HOME/.dogu"}
install_dir=${DOGU_INSTALL_DIR:-"$dogu_home/bin"}
mkdir -p "$install_dir"

release_json=$(curl -fsSL https://api.github.com/repos/rahulsom/dogu/releases/latest)
asset_name=$(jq -r --arg suffix "_${target}.tar.gz" \
  '[.assets[] | select(.name | endswith($suffix))][0].name // empty' <<<"$release_json")
asset_url=$(jq -r --arg name "$asset_name" \
  '.assets[] | select(.name == $name) | .browser_download_url' <<<"$release_json")
checksums_url=$(jq -r \
  '.assets[] | select(.name == "checksums.txt") | .browser_download_url' <<<"$release_json")
if [[ -z "$asset_name" || -z "$asset_url" ]]; then
  echo "No release asset found for $target" >&2
  exit 1
fi
if [[ -z "$checksums_url" ]]; then
  echo "Release does not include checksums.txt" >&2
  exit 1
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT
archive="$tmp_dir/$asset_name"
curl -fsSL "$asset_url" -o "$archive"
curl -fsSL "$checksums_url" -o "$tmp_dir/checksums.txt"

expected_hash=$(awk -v filename="$asset_name" '$2 == filename { print $1; exit }' "$tmp_dir/checksums.txt")
if [[ ! "$expected_hash" =~ ^[[:xdigit:]]{64}$ ]]; then
  echo "No valid checksum found for $asset_name" >&2
  exit 1
fi
if [[ "$hash_command" == sha256sum ]]; then
  actual_hash=$(sha256sum "$archive" | awk '{ print $1 }')
else
  actual_hash=$(shasum -a 256 "$archive" | awk '{ print $1 }')
fi
if [[ "$(printf '%s' "$actual_hash" | tr '[:upper:]' '[:lower:]')" != \
      "$(printf '%s' "$expected_hash" | tr '[:upper:]' '[:lower:]')" ]]; then
  echo "SHA-256 checksum verification failed for $asset_name" >&2
  exit 1
fi

tar -xzf "$archive" -C "$tmp_dir" dogu
install -m 755 "$tmp_dir/dogu" "$install_dir/dogu"
"$install_dir/dogu" --help

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) printf '\nAdd dogu to PATH for this shell with:\n  export PATH="%s:$PATH"\n' "$install_dir" ;;
esac

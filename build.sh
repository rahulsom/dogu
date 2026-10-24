#!/usr/bin/env bash
set -euo pipefail

OUTPUT_DIR="build/bin"
mkdir -p "$OUTPUT_DIR"

# Target combinations: "GOOS GOARCH [GOARM]"
TARGETS=(
  "linux amd64"
  "linux arm64"
  "linux 386"
  "linux arm 7"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
  "windows 386"
)

echo "Building all variants into $OUTPUT_DIR..."

for target in "${TARGETS[@]}"; do
  read -r os arch arm <<< "$target"

  ext=""
  if [ "$os" = "windows" ]; then
    ext=".exe"
  fi

  arm_suffix=""
  if [ -n "$arm" ]; then
    arm_suffix="v${arm}"
  fi

  binary_name="dogu-${os}-${arch}${arm_suffix}${ext}"
  output_path="${OUTPUT_DIR}/${binary_name}"

  echo "Building $binary_name (GOOS=$os GOARCH=$arch ${arm:+GOARM=$arm})..."

  env CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" ${arm:+GOARM="$arm"} \
    go build -ldflags="-s -w" -o "$output_path" ./cmd/dogu
done

echo "Done building all variants!"

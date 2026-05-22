#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
JNI_DIR="$PROJECT_DIR/android-overlay/app/src/main/jniLibs"

MPV_LIB_VERSION="0.1.12"
AAR_URL="https://repo1.maven.org/maven2/io/github/abdallahmehiz/mpv-android-lib/${MPV_LIB_VERSION}/mpv-android-lib-${MPV_LIB_VERSION}.aar"
AAR_TMP="$(mktemp -d)/mpv-android-lib.aar"

echo "setup-mpv-libs: downloading mpv-android-lib ${MPV_LIB_VERSION} AAR..."
curl -fSL -o "$AAR_TMP" "$AAR_URL"

echo "setup-mpv-libs: extracting native libraries..."
mkdir -p "$JNI_DIR"

for abi in arm64-v8a; do
    echo "  extracting $abi..."
    mkdir -p "$JNI_DIR/$abi"
    unzip -o -j "$AAR_TMP" "jni/$abi/libmpv.so" "jni/$abi/libplayer.so" -d "$JNI_DIR/$abi" 2>/dev/null || true
    if [ -f "$JNI_DIR/$abi/libmpv.so" ]; then
        echo "  ✓ $abi: libmpv.so + libplayer.so"
    else
        echo "  ✗ $abi: not found in AAR (skipping)"
        rmdir "$JNI_DIR/$abi" 2>/dev/null || true
    fi
done

rm -f "$AAR_TMP"
echo "setup-mpv-libs: done. Libraries saved to $JNI_DIR"

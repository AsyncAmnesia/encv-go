#!/bin/bash
set -euo pipefail

FFMPEG_VERSION="7.1.1"
X264_VERSION="stable"
NDK_VERSION="26.1.10909125"
API_LEVEL=24
ABI="arm64-v8a"
ARCH="aarch64"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/.ffmpeg-build"
OUTPUT_DIR="${SCRIPT_DIR}/android/app/src/main/jniLibs/${ABI}"

NDK_PATH="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Android/Sdk}}/ndk/${NDK_VERSION}"
if [ ! -d "$NDK_PATH" ]; then
    echo "❌ NDK not found at $NDK_PATH"
    echo "Install with: sdkmanager \"ndk;${NDK_VERSION}\""
    exit 1
fi

TOOLCHAIN="${NDK_PATH}/toolchains/llvm/prebuilt/linux-x86_64"
CC="${TOOLCHAIN}/bin/${ARCH}-linux-android${API_LEVEL}-clang"
AR="${TOOLCHAIN}/bin/llvm-ar"
NM="${TOOLCHAIN}/bin/llvm-nm"
RANLIB="${TOOLCHAIN}/bin/llvm-ranlib"
STRIP="${TOOLCHAIN}/bin/llvm-strip"

mkdir -p "$BUILD_DIR" "$OUTPUT_DIR"

echo "=== Checking for cached ffmpeg output ==="
if [ -f "${OUTPUT_DIR}/libffmpeg.so" ] && [ -f "${OUTPUT_DIR}/libffprobe.so" ]; then
    echo "✅ ffmpeg output already exists, checking symbols..."
    ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" | grep -q "ffmpeg_run" && \
    ${NM} -D "${OUTPUT_DIR}/libffprobe.so" | grep -q "ffprobe_run" && {
        echo "✅ All ffmpeg libraries cached and valid, skipping build"
        echo "Output: $OUTPUT_DIR"
        ls -lh "$OUTPUT_DIR"
        exit 0
    }
    echo "⚠️  Cached libraries missing expected symbols, rebuilding..."
fi

cd "$BUILD_DIR"

echo "=== Building ffmpeg ${FFMPEG_VERSION} for Android ${ABI} ==="

if [ ! -d "ffmpeg-${FFMPEG_VERSION}" ]; then
    echo "Downloading ffmpeg ${FFMPEG_VERSION}..."
    curl -sL "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz" -o ffmpeg.tar.xz
    tar xf ffmpeg.tar.xz
    rm ffmpeg.tar.xz
fi

X264_INSTALL="${BUILD_DIR}/x264-install"
if [ ! -f "${X264_INSTALL}/lib/libx264.a" ]; then
    if [ ! -d "x264" ]; then
        echo "Downloading x264..."
        git clone --depth 1 --branch ${X264_VERSION} https://code.videolan.org/videolan/x264.git
    fi

    echo "=== Building x264 ==="
    cd "${BUILD_DIR}/x264"
    CC="$CC" AR="$AR" RANLIB="$RANLIB" STRIP="$STRIP" \
    ./configure \
        --host=${ARCH}-linux-android \
        --prefix="${X264_INSTALL}" \
        --enable-static \
        --disable-cli \
        --disable-opencl \
        --cross-prefix="${TOOLCHAIN}/bin/llvm-" \
        --extra-cflags="-fPIC -DANDROID" \
        --extra-ldflags="-lm"

    make -j$(nproc)
    make install
else
    echo "✅ x264 already built, skipping"
fi

echo "=== Patching ffmpeg source ==="
cd "${BUILD_DIR}/ffmpeg-${FFMPEG_VERSION}"

sed -i 's/^int main(/int ffmpeg_run(/' fftools/ffmpeg.c
sed -i 's/^int main(void)/int ffmpeg_run(void)/' fftools/ffmpeg.c

sed -i 's/^int main(/int ffprobe_run(/' fftools/ffprobe.c
sed -i 's/^int main(void)/int ffprobe_run(void)/' fftools/ffprobe.c

cat >> fftools/ffmpeg.c << 'PATCH'

void ffmpeg_reset(void) {
}
PATCH

cat >> fftools/ffprobe.c << 'PATCH'

void ffprobe_reset(void) {
}
PATCH

echo "=== Setting up pkg-config for cross-compilation ==="
if ! command -v pkg-config &>/dev/null; then
    echo "pkg-config not found, installing..."
    apt-get update -qq && apt-get install -y -qq pkg-config
fi

echo "Fixing x264.pc for Android (remove -lpthread -ldl)..."
sed -i 's/-lpthread//g; s/-ldl//g' "${X264_INSTALL}/lib/pkgconfig/x264.pc" 2>/dev/null || true

cat > "${BUILD_DIR}/pkg-config-wrapper" << PCEOF
#!/bin/bash
export PKG_CONFIG_PATH="${X264_INSTALL}/lib/pkgconfig"
export PKG_CONFIG_LIBDIR="${X264_INSTALL}/lib/pkgconfig"
export PKG_CONFIG_SYSROOT_DIR="${TOOLCHAIN}/sysroot"
exec pkg-config "\$@"
PCEOF
chmod +x "${BUILD_DIR}/pkg-config-wrapper"

echo "Verifying x264 via wrapper:"
"${BUILD_DIR}/pkg-config-wrapper" --cflags --libs x264 || echo "⚠️  x264 not found via wrapper"

echo "=== Configuring ffmpeg ==="
./configure \
    --prefix="${BUILD_DIR}/ffmpeg-install" \
    --enable-cross-compile \
    --cross-prefix="${TOOLCHAIN}/bin/llvm-" \
    --cc="$CC" \
    --ar="$AR" \
    --nm="$NM" \
    --ranlib="$RANLIB" \
    --strip="$STRIP" \
    --arch=${ARCH} \
    --target-os=android \
    --sysroot="${TOOLCHAIN}/sysroot" \
    --enable-shared \
    --disable-static \
    --disable-programs \
    --disable-doc \
    --disable-htmlpages \
    --disable-manpages \
    --disable-podpages \
    --disable-txtpages \
    --disable-everything \
    --enable-decoder=h264,hevc,aac,mp3,opus,vorbis,flac,pcm_s16le,pcm_s24le,pcm_s32le,aac_latm \
    --enable-encoder=h264,aac,pcm_s16le,pcm_s24le,pcm_s32le \
    --enable-muxer=mp4,matroska,flac,mp3,adts,null \
    --enable-demuxer=mov,matroska,aac,mp3,flac,ogg,wav \
    --enable-parser=h264,hevc,aac,aac_latm,mpegaudio,opus,vorbis \
    --enable-protocol=file,pipe \
    --enable-filter=aresample \
    --enable-libx264 \
    --enable-gpl \
    --pkg-config="${BUILD_DIR}/pkg-config-wrapper" \
    --extra-cflags="-fPIC -DANDROID -I${X264_INSTALL}/include" \
    --extra-ldflags="-L${X264_INSTALL}/lib -lm" \
    --extra-libs="-lm" || {
    echo "=== ffmpeg configure FAILED ==="
    echo "=== Last 80 lines of config.log ==="
    tail -80 ffbuild/config.log 2>/dev/null || echo "(no config.log found)"
    exit 1
}

echo "=== Building ffmpeg ==="
make -j$(nproc)
make install

echo "=== Copying shared libraries ==="
for lib in libavcodec libavformat libavutil libswresample libswscale libavfilter libavdevice libpostproc; do
    src="${BUILD_DIR}/ffmpeg-install/lib/${lib}.so"
    if [ -f "$src" ]; then
        cp "$src" "$OUTPUT_DIR/"
        echo "✅ Copied ${lib}.so"
    else
        echo "⚠️  ${lib}.so not found (may be disabled)"
    fi
done

echo "=== Building fftools shared libraries ==="
FFMPEG_SRC="${BUILD_DIR}/ffmpeg-${FFMPEG_VERSION}"
FFMPEG_INSTALL="${BUILD_DIR}/ffmpeg-install"
FTOOLS_BUILD="${BUILD_DIR}/ftools-build"
mkdir -p "$FTOOLS_BUILD"

CFLAGS="-fPIC -DANDROID -I${FFMPEG_INSTALL}/include -I${FFMPEG_SRC}"
LDFLAGS="-L${FFMPEG_INSTALL}/lib -L${X264_INSTALL}/lib"

FFMPEG_FFTOOLS="fftools/ffmpeg.c fftools/ffmpeg_dec.c fftools/ffmpeg_demux.c fftools/ffmpeg_enc.c fftools/ffmpeg_filter.c fftools/ffmpeg_hw.c fftools/ffmpeg_mux.c fftools/ffmpeg_opt.c fftools/cmdutils.c fftools/opt_common.c fftools/sync_queue.c fftools/thread_queue.c"

FFPROBE_FFTOOLS="fftools/ffprobe.c fftools/cmdutils.c fftools/opt_common.c"

echo "Compiling ffmpeg fftools..."
FFMPEG_OBJS=""
for src in $FFMPEG_FFTOOLS; do
    if [ -f "${FFMPEG_SRC}/${src}" ]; then
        objname=$(basename "${src}" .c)
        obj="${FTOOLS_BUILD}/ffmpeg_${objname}.o"
        $CC $CFLAGS -c -o "$obj" "${FFMPEG_SRC}/${src}" || {
            echo "⚠️  Failed to compile ${src}, skipping"
            continue
        }
        FFMPEG_OBJS="$FFMPEG_OBJS $obj"
    else
        echo "⚠️  ${src} not found, skipping"
    fi
done

echo "Linking libffmpeg.so..."
$CC $CFLAGS -shared -o "${FTOOLS_BUILD}/libffmpeg.so" \
    $FFMPEG_OBJS \
    -lavcodec -lavformat -lavutil -lswresample -lswscale -lavfilter -lavdevice -lpostproc \
    -lx264 -lm -llog \
    -Wl,-rpath,\$ORIGIN \
    $LDFLAGS

echo "Compiling ffprobe fftools..."
FFPROBE_OBJS=""
for src in $FFPROBE_FFTOOLS; do
    if [ -f "${FFMPEG_SRC}/${src}" ]; then
        objname=$(basename "${src}" .c)
        obj="${FTOOLS_BUILD}/ffprobe_${objname}.o"
        $CC $CFLAGS -c -o "$obj" "${FFMPEG_SRC}/${src}" || {
            echo "⚠️  Failed to compile ${src}, skipping"
            continue
        }
        FFPROBE_OBJS="$FFPROBE_OBJS $obj"
    else
        echo "⚠️  ${src} not found, skipping"
    fi
done

echo "Linking libffprobe.so..."
$CC $CFLAGS -shared -o "${FTOOLS_BUILD}/libffprobe.so" \
    $FFPROBE_OBJS \
    -lavcodec -lavformat -lavutil -lswresample -lswscale -lavfilter -lavdevice -lpostproc \
    -lx264 -lm -llog \
    -Wl,-rpath,\$ORIGIN \
    $LDFLAGS

cp "${FTOOLS_BUILD}/libffmpeg.so" "$OUTPUT_DIR/"
cp "${FTOOLS_BUILD}/libffprobe.so" "$OUTPUT_DIR/"

echo "✅ Copied libffmpeg.so"
echo "✅ Copied libffprobe.so"

echo "=== Verifying exported symbols ==="
for lib in libffmpeg.so libffprobe.so; do
    echo "--- ${lib} symbols ---"
    ${NM} -D "${OUTPUT_DIR}/${lib}" | grep -E "ffmpeg_run|ffprobe_run|ffmpeg_reset|ffprobe_reset" || echo "⚠️  No expected symbols found"
done

echo "=== Build complete ==="
echo "Output: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"

TOTAL_SIZE=$(du -sh "$OUTPUT_DIR" | cut -f1)
echo "Total size: $TOTAL_SIZE"

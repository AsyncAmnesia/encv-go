#!/bin/bash
set -euo pipefail

FFMPEG_VERSION="8.0"
X264_VERSION="stable"
NDK_VERSION="26.1.10909125"
API_LEVEL=24
ABI="arm64-v8a"
ARCH="aarch64"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BUILD_DIR="${SCRIPT_DIR}/.ffmpeg-build"
OUTPUT_DIR="${PROJECT_DIR}/android/app/src/main/jniLibs/${ABI}"
LOG_DIR="${BUILD_DIR}/logs"

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

mkdir -p "$BUILD_DIR" "$OUTPUT_DIR" "$LOG_DIR"

echo "=== Checking for cached ffmpeg output ==="
if [ -f "${OUTPUT_DIR}/libffmpeg.so" ] && [ -f "${OUTPUT_DIR}/libffprobe.so" ]; then
    HAS_FFMPEG_RUN=$(${NM} -D "${OUTPUT_DIR}/libffmpeg.so" 2>/dev/null | grep -q "ffmpeg_run" && echo "yes" || echo "")
    HAS_FF_GRAPH_CSS=$(${NM} -D "${OUTPUT_DIR}/libffmpeg.so" 2>/dev/null | grep -q "ff_graph_css_data" && echo "yes" || "")
    HAS_FFPROBE_RUN=$(${NM} -D "${OUTPUT_DIR}/libffprobe.so" 2>/dev/null | grep -q "ffprobe_run" && echo "yes" || "")

    if [ "$HAS_FFMPEG_RUN" = "yes" ] && [ "$HAS_FF_GRAPH_CSS" = "yes" ] && [ "$HAS_FFPROBE_RUN" = "yes" ]; then
        echo "✅ All ffmpeg libraries cached and valid, skipping build"
        echo "Output: $OUTPUT_DIR"
        ls -lh "$OUTPUT_DIR"
        exit 0
    else
        echo "⚠️  Cached libraries missing expected symbols (ffmpeg_run=$HAS_FFMPEG_RUN ff_graph_css_data=$HAS_FF_GRAPH_CSS ffprobe_run=$HAS_FFPROBE_RUN), rebuilding..."
        rm -f "${OUTPUT_DIR}/libffmpeg.so" "${OUTPUT_DIR}/libffprobe.so"
    fi
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
        --enable-pic \
        --disable-cli \
        --disable-opencl \
        --cross-prefix="${TOOLCHAIN}/bin/llvm-" \
        --extra-cflags="-fPIC -DANDROID" \
        --extra-ldflags="-lm" \
        > "${LOG_DIR}/x264-configure.log" 2>&1
    echo "x264 configure done (log: ${LOG_DIR}/x264-configure.log)"

    make -j$(nproc) > "${LOG_DIR}/x264-make.log" 2>&1
    make install > "${LOG_DIR}/x264-install.log" 2>&1
    echo "✅ x264 built and installed"
else
    echo "✅ x264 already built, skipping"
fi

echo "=== Patching ffmpeg source ==="
cd "${BUILD_DIR}/ffmpeg-${FFMPEG_VERSION}"

sed -i 's/^int main(/int ffmpeg_run(/' fftools/ffmpeg.c
sed -i 's/^int main(void)/int ffmpeg_run(void)/' fftools/ffmpeg.c
sed -i 's/^int wmain(/int ffmpeg_run(/' fftools/ffmpeg.c

sed -i 's/^int main(/int ffprobe_run(/' fftools/ffprobe.c
sed -i 's/^int main(void)/int ffprobe_run(void)/' fftools/ffprobe.c

if ! grep -q "void ffmpeg_reset" fftools/ffmpeg.c; then
    cat >> fftools/ffmpeg.c << 'PATCH'

void ffmpeg_reset(void) {
}
PATCH
fi

if ! grep -q "void ffprobe_reset" fftools/ffprobe.c; then
    cat >> fftools/ffprobe.c << 'PATCH'

void ffprobe_reset(void) {
}
PATCH
fi

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
export PKG_CONFIG_ALLOW_SYSTEM_CFLAGS=1
export PKG_CONFIG_ALLOW_SYSTEM_LIBS=1
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
    --enable-static \
    --disable-asm \
    --disable-programs \
    --disable-doc \
    --disable-htmlpages \
    --disable-manpages \
    --disable-podpages \
    --disable-txtpages \
    --disable-everything \
    --enable-decoder=h264,hevc,aac,mp3,opus,vorbis,flac,pcm_s16le,pcm_s24le,pcm_s32le,aac_latm \
    --enable-encoder=aac,pcm_s16le,pcm_s24le,pcm_s32le,libx264 \
    --enable-muxer=mp4,matroska,flac,mp3,adts,null \
    --enable-demuxer=mov,matroska,aac,mp3,flac,ogg,wav \
    --enable-parser=h264,hevc,aac,aac_latm,mpegaudio,opus,vorbis \
    --enable-protocol=file,pipe \
    --enable-filter=aresample \
    --enable-small \
    --enable-libx264 \
    --enable-gpl \
    --pkg-config="${BUILD_DIR}/pkg-config-wrapper" \
    --extra-cflags="-fPIC -ffunction-sections -fdata-sections -DANDROID -I${X264_INSTALL}/include" \
    --extra-ldflags="-L${X264_INSTALL}/lib -lm" \
    --extra-libs="-lm" || {
    echo "=== ffmpeg configure FAILED ==="
    echo "=== Last 80 lines of config.log ==="
    tail -80 ffbuild/config.log 2>/dev/null || echo "(no config.log found)"
    exit 1
}

echo "=== Building ffmpeg ==="
make -j$(nproc) > "${LOG_DIR}/ffmpeg-make.log" 2>&1
echo "ffmpeg make done (log: ${LOG_DIR}/ffmpeg-make.log)"

make install > "${LOG_DIR}/ffmpeg-install.log" 2>&1
echo "✅ ffmpeg built and installed"

echo "=== FFmpeg shared libs (.so) are statically linked into libffmpeg.so/libffprobe.so ==="

echo "=== Building fftools shared libraries ==="
FFMPEG_SRC="${BUILD_DIR}/ffmpeg-${FFMPEG_VERSION}"
FFMPEG_INSTALL="${BUILD_DIR}/ffmpeg-install"
FTOOLS_BUILD="${BUILD_DIR}/ftools-build"
mkdir -p "$FTOOLS_BUILD"

CFLAGS="-std=c11 -fPIC -ffunction-sections -fdata-sections -DANDROID -D_POSIX_C_SOURCE=200809L \
  -DHAVE_SYS_RESOURCE_H=1 -DHAVE_UNISTD_H=1 -DHAVE_SYS_SELECT_H=1 \
  -include time.h \
  -I${FFMPEG_INSTALL}/include \
  -I${FFMPEG_SRC} \
  -I${FFMPEG_SRC}/compat/stdbit \
  -I${FFMPEG_SRC}/fftools \
  -I${FFMPEG_SRC}/fftools/textformat \
  -I${FFMPEG_SRC}/fftools/graph \
  -I${FFMPEG_SRC}/fftools/resources \
  -I${X264_INSTALL}/include"
LDFLAGS="-L${FFMPEG_INSTALL}/lib -L${X264_INSTALL}/lib"

STATIC_LIBS=""
for lib in libavformat libavcodec libavutil libswresample libswscale libavfilter libavdevice; do
    if [ -f "${FFMPEG_INSTALL}/lib/${lib}.a" ]; then
        STATIC_LIBS="$STATIC_LIBS ${FFMPEG_INSTALL}/lib/${lib}.a"
    fi
done

FFMPEG_CORE_FFTOOLS="fftools/ffmpeg.c fftools/ffmpeg_dec.c fftools/ffmpeg_demux.c fftools/ffmpeg_enc.c fftools/ffmpeg_filter.c fftools/ffmpeg_hw.c fftools/ffmpeg_mux.c fftools/ffmpeg_mux_init.c fftools/ffmpeg_opt.c fftools/ffmpeg_sched.c fftools/cmdutils.c fftools/opt_common.c fftools/sync_queue.c fftools/thread_queue.c"

FFMPEG_FFTOOLS=""
for f in $FFMPEG_CORE_FFTOOLS; do
    [ -f "${FFMPEG_SRC}/${f}" ] && FFMPEG_FFTOOLS="$FFMPEG_FFTOOLS $f"
done

FFMPEG_GRAPH_FFTOOLS=""
if [ -d "${FFMPEG_SRC}/fftools/graph" ]; then
    for f in ${FFMPEG_SRC}/fftools/graph/*.c; do
        [ -f "$f" ] && FFMPEG_GRAPH_FFTOOLS="$FFMPEG_GRAPH_FFTOOLS fftools/graph/$(basename $f)"
        FFMPEG_FFTOOLS="$FFMPEG_FFTOOLS fftools/graph/$(basename $f)"
    done
fi

FFMPEG_OPTIONAL_DIRS="fftools/textformat fftools/resources"
for dir in $FFMPEG_OPTIONAL_DIRS; do
    if [ -d "${FFMPEG_SRC}/${dir}" ]; then
        for f in ${FFMPEG_SRC}/${dir}/*.c; do
            [ -f "$f" ] && FFMPEG_FFTOOLS="$FFMPEG_FFTOOLS ${dir}/$(basename $f)"
        done
    fi
done

FFPROBE_FFTOOLS=""
for f in fftools/ffprobe.c fftools/cmdutils.c fftools/opt_common.c; do
    [ -f "${FFMPEG_SRC}/${f}" ] && FFPROBE_FFTOOLS="$FFPROBE_FFTOOLS $f"
done
if [ -d "${FFMPEG_SRC}/fftools/textformat" ]; then
    for f in ${FFMPEG_SRC}/fftools/textformat/*.c; do
        [ -f "$f" ] && FFPROBE_FFTOOLS="$FFPROBE_FFTOOLS fftools/textformat/$(basename $f)"
    done
fi

echo "Compiling ffmpeg fftools..."
FFMPEG_OBJS=""
for src in $FFMPEG_FFTOOLS; do
    if [ ! -f "${FFMPEG_SRC}/${src}" ]; then
        continue
    fi
    objname=$(basename "${src}" .c)
    obj="${FTOOLS_BUILD}/ffmpeg_${objname}.o"
    is_core=false
    for core in $FFMPEG_CORE_FFTOOLS; do
        [ "$src" = "$core" ] && is_core=true && break
    done
    is_graph=false
    for graph in $FFMPEG_GRAPH_FFTOOLS; do
        [ "$src" = "$graph" ] && is_graph=true && break
    done
    if $CC $CFLAGS -c -o "$obj" "${FFMPEG_SRC}/${src}" > "${LOG_DIR}/ffmpeg_${objname}.log" 2>&1; then
        FFMPEG_OBJS="$FFMPEG_OBJS $obj"
    else
        if $is_core || $is_graph; then
            echo "❌ Failed to compile required file ${src} (see ${LOG_DIR}/ffmpeg_${objname}.log)"
            cat "${LOG_DIR}/ffmpeg_${objname}.log" | tail -10
            exit 1
        else
            echo "⚠️  Failed to compile optional ${src}, skipping (see ${LOG_DIR}/ffmpeg_${objname}.log)"
        fi
    fi
done

echo "Linking libffmpeg.so..."
$CC $CFLAGS -shared -o "${FTOOLS_BUILD}/libffmpeg.so" \
    $FFMPEG_OBJS \
    $STATIC_LIBS \
    ${X264_INSTALL}/lib/libx264.a \
    -lm -lz -llog \
    -Wl,--gc-sections \
    -Wl,--allow-multiple-definition \
    $LDFLAGS > "${LOG_DIR}/link_ffmpeg.log" 2>&1 || {
    echo "❌ Failed to link libffmpeg.so (see ${LOG_DIR}/link_ffmpeg.log)"
    cat "${LOG_DIR}/link_ffmpeg.log" | tail -10
    exit 1
}

echo "Compiling ffprobe fftools..."
FFPROBE_OBJS=""
for src in $FFPROBE_FFTOOLS; do
    if [ ! -f "${FFMPEG_SRC}/${src}" ]; then
        continue
    fi
    objname=$(basename "${src}" .c)
    obj="${FTOOLS_BUILD}/ffprobe_${objname}.o"
    if $CC $CFLAGS -c -o "$obj" "${FFMPEG_SRC}/${src}" > "${LOG_DIR}/ffprobe_${objname}.log" 2>&1; then
        FFPROBE_OBJS="$FFPROBE_OBJS $obj"
    else
        echo "❌ Failed to compile ${src} (see ${LOG_DIR}/ffprobe_${objname}.log)"
        cat "${LOG_DIR}/ffprobe_${objname}.log" | tail -10
        exit 1
    fi
done

echo "Linking libffprobe.so..."
$CC $CFLAGS -shared -o "${FTOOLS_BUILD}/libffprobe.so" \
    $FFPROBE_OBJS \
    $STATIC_LIBS \
    ${X264_INSTALL}/lib/libx264.a \
    -lm -lz -llog \
    -Wl,--gc-sections \
    -Wl,--allow-multiple-definition \
    $LDFLAGS > "${LOG_DIR}/link_ffprobe.log" 2>&1 || {
    echo "❌ Failed to link libffprobe.so (see ${LOG_DIR}/link_ffprobe.log)"
    cat "${LOG_DIR}/link_ffprobe.log" | tail -10
    exit 1
}

cp "${FTOOLS_BUILD}/libffmpeg.so" "$OUTPUT_DIR/"
cp "${FTOOLS_BUILD}/libffprobe.so" "$OUTPUT_DIR/"

echo "=== Stripping debug symbols ==="
$STRIP --strip-all "${OUTPUT_DIR}/libffmpeg.so"
$STRIP --strip-all "${OUTPUT_DIR}/libffprobe.so"

echo "✅ Copied and stripped libffmpeg.so"
echo "✅ Copied and stripped libffprobe.so"

echo "=== Verifying exported symbols ==="
for lib in libffmpeg.so libffprobe.so; do
    echo "--- ${lib} symbols ---"
    ${NM} -D "${OUTPUT_DIR}/${lib}" | grep -E "ffmpeg_run|ffprobe_run|ffmpeg_reset|ffprobe_reset" || echo "⚠️  No expected symbols found"
done

echo "=== Verifying ff_graph_css_data in libffmpeg.so ==="
if ! ${NM} -D "${OUTPUT_DIR}/libffmpeg.so" 2>/dev/null | grep -q "ff_graph_css_data"; then
    echo "❌ ff_graph_css_data not found in libffmpeg.so — graph module compilation may have failed"
    exit 1
fi
echo "✅ ff_graph_css_data present"

echo "=== Generating build-info.json ==="
ENABLED_DECODERS="h264,hevc,aac,mp3,opus,vorbis,flac,pcm_s16le,pcm_s24le,pcm_s32le,aac_latm"
ENABLED_ENCODERS="aac,pcm_s16le,pcm_s24le,pcm_s32le,libx264"
ENABLED_MUXERS="mp4,matroska,flac,mp3,adts,null"
ENABLED_DEMUXERS="mov,matroska,aac,mp3,flac,ogg,wav"
ENABLED_PARSERS="h264,hevc,aac,aac_latm,mpegaudio,opus,vorbis"
ENABLED_PROTOCOLS="file,pipe"
ENABLED_FILTERS="aresample"

STATIC_LIBS_LIST=""
for lib in libavformat libavcodec libavutil libswresample libswscale libavfilter libavdevice; do
    if [ -f "${FFMPEG_INSTALL}/lib/${lib}.a" ]; then
        STATIC_LIBS_LIST="${STATIC_LIBS_LIST}\"${lib}\","
    fi
done
STATIC_LIBS_LIST="${STATIC_LIBS_LIST%,}"

X264_CONFIGURE_OPTS="--enable-static --enable-pic --disable-cli --disable-opencl"

cat > "${OUTPUT_DIR}/build-info.json" << BIEOF
{
  "ffmpeg_version": "${FFMPEG_VERSION}",
  "ffmpeg_codename": "Huffman",
  "x264_version": "${X264_VERSION}",
  "x264_configure_opts": "${X264_CONFIGURE_OPTS}",
  "ndk_version": "${NDK_VERSION}",
  "api_level": ${API_LEVEL},
  "abi": "${ABI}",
  "build_date": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "enabled_decoders": [$(echo "$ENABLED_DECODERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_encoders": [$(echo "$ENABLED_ENCODERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_muxers": [$(echo "$ENABLED_MUXERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_demuxers": [$(echo "$ENABLED_DEMUXERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_parsers": [$(echo "$ENABLED_PARSERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_protocols": [$(echo "$ENABLED_PROTOCOLS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_filters": [$(echo "$ENABLED_FILTERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "static_libs": [${STATIC_LIBS_LIST}],
  "linking": "static-into-so",
  "cflags": "-std=c11 -fPIC -DANDROID -D_POSIX_C_SOURCE=200809L -include time.h",
  "ffmpeg_license": "GPL v2+",
  "x264_license": "GPL v2"
}
BIEOF

echo "✅ Generated build-info.json"

ASSETS_DIR="${PROJECT_DIR}/android/app/src/main/assets"
mkdir -p "$ASSETS_DIR"
cp "${OUTPUT_DIR}/build-info.json" "${ASSETS_DIR}/"
echo "✅ Copied build-info.json to Android assets"

echo "=== Build complete ==="
echo "Output: $OUTPUT_DIR"
ls -lh "$OUTPUT_DIR"

TOTAL_SIZE=$(du -sh "$OUTPUT_DIR" | cut -f1)
echo "Total size: $TOTAL_SIZE"

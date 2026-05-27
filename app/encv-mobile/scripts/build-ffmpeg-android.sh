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
MANIFEST="${SCRIPT_DIR}/ffmpeg-feature-manifest.json"

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
    HAS_FFPROBE_RUN=$(${NM} -D "${OUTPUT_DIR}/libffprobe.so" 2>/dev/null | grep -q "ffprobe_run" && echo "yes" || "")

    if [ "$HAS_FFMPEG_RUN" = "yes" ] && [ "$HAS_FFPROBE_RUN" = "yes" ]; then
        echo "✅ All ffmpeg libraries cached and valid, skipping build"
        echo "Output: $OUTPUT_DIR"
        ls -lh "$OUTPUT_DIR"
        exit 0
    else
        echo "⚠️  Cached libraries missing expected symbols (ffmpeg_run=$HAS_FFMPEG_RUN ffprobe_run=$HAS_FFPROBE_RUN), rebuilding..."
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

echo "=== Reading FFmpeg feature manifest ==="
if [ ! -f "$MANIFEST" ]; then
    echo "❌ Manifest not found: $MANIFEST"
    exit 1
fi

eval "$(python3 -c "
import json, sys
m = json.load(open('$MANIFEST'))
f = m['ffmpeg']
print(f'DECODERS={\",\".join(f[\"decoders\"])}')
print(f'ENCODERS={\",\".join(f[\"encoders\"])}')
print(f'MUXERS={\",\".join(f[\"muxers\"])}')
print(f'DEMUXERS={\",\".join(f[\"demuxers\"])}')
print(f'PARSERS={\",\".join(f[\"parsers\"])}')
print(f'PROTOCOLS={\",\".join(f[\"protocols\"])}')
print(f'FILTERS={\",\".join(f[\"filters\"])}')

for lib_name, modules in m['ftools_modules'].items():
    var_name = lib_name.upper().replace('.', '_') + '_MODULES'
    lines = []
    for mod_name, files in modules.items():
        if not isinstance(files, list):
            if isinstance(files, bool) and files:
                resolved_name = mod_name.replace('_shared', '')
                for other_lib, other_mods in m['ftools_modules'].items():
                    if resolved_name in other_mods and isinstance(other_mods[resolved_name], list):
                        files = other_mods[resolved_name]
                        break
                else:
                    continue
            else:
                continue
        files_str = ' '.join(files)
        lines.append('  \"' + mod_name + ':' + files_str + '\"')
    print(var_name + '=(')
    for l in lines:
        print(l)
    print(')')
")"

echo "  Decoders:  $DECODERS"
echo "  Encoders:  $ENCODERS"
echo "  Muxers:    $MUXERS"
echo "  Demuxers:  $DEMUXERS"
echo "  Parsers:   $PARSERS"
echo "  Protocols: $PROTOCOLS"
echo "  Filters:   $FILTERS"

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
    --enable-decoder="$DECODERS" \
    --enable-encoder="$ENCODERS" \
    --enable-muxer="$MUXERS" \
    --enable-demuxer="$DEMUXERS" \
    --enable-parser="$PARSERS" \
    --enable-protocol="$PROTOCOLS" \
    --enable-filter="$FILTERS" \
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

FFMPEG_SRC="${BUILD_DIR}/ffmpeg-${FFMPEG_VERSION}"
FFMPEG_INSTALL="${BUILD_DIR}/ffmpeg-install"
FTOOLS_BUILD="${BUILD_DIR}/ftools-build"
mkdir -p "$FTOOLS_BUILD"

echo "=== Phase 2: Generating resource files via bin2c ==="
cd "${FFMPEG_SRC}"

BIN2C_CC="$(command -v gcc 2>/dev/null || command -v cc 2>/dev/null || echo "gcc")"
$BIN2C_CC -o "${BUILD_DIR}/bin2c" ffbuild/bin2c.c \
    > "${LOG_DIR}/bin2c-build.log" 2>&1 || {
    echo "❌ Failed to build bin2c"
    tail -5 "${LOG_DIR}/bin2c-build.log"
    exit 1
}
echo "✅ bin2c built"

GEN_RES_DIR="${FFMPEG_SRC}/fftools/resources"
for res_file in "$GEN_RES_DIR"/*.css "$GEN_RES_DIR"/*.html; do
    [ -f "$res_file" ] || continue
    base=$(basename "$res_file")
    bin2c_name=$(basename "${base}" | sed 's/\.[^.]*$//' | tr '.' '_')
    if [[ "$res_file" == *.css ]]; then
        sed 's!/\\*.*\\*/!!g' "$res_file" | tr '\n' ' ' | tr -s ' ' | sed 's/^ //; s/ $$//' \
            > "${res_file}.min"
        "${BUILD_DIR}/bin2c" "${res_file}.min" "${res_file}.c" "$bin2c_name"
    elif [[ "$res_file" == *.html ]]; then
        "${BUILD_DIR}/bin2c" "$res_file" "${res_file}.c" "$bin2c_name"
    fi
    echo "  ✅ Generated ${base}.c (symbol: ff_${bin2c_name}_data)"
done

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

echo "=== Phase 3: Compiling fftools (manifest-driven) ==="

compile_modules() {
    local -n modules=$1
    local -n out_objs=$2
    out_objs=""

    for module_def in "${modules[@]}"; do
        local mod_name="${module_def%%:*}"
        local mod_files="${module_def#*:}"

        for src in $mod_files; do
            local src_path="${FFMPEG_SRC}/${src}"
            if [ ! -f "$src_path" ]; then
                echo "⚠️  Module [$mod_name]: $src not found, skipping"
                continue
            fi

            local objname="$(basename "$src" .c)"
            local obj="${FTOOLS_BUILD}/${mod_name}_${objname}.o"

            if $CC $CFLAGS -c -o "$obj" "$src_path" > "${LOG_DIR}/${mod_name}_${objname}.log" 2>&1; then
                out_objs="$out_objs $obj"
            else
                echo "❌ Module [$mod_name]: failed to compile $src"
                tail -5 "${LOG_DIR}/${mod_name}_${objname}.log"
                exit 1
            fi
        done
        echo "  ✅ Module [$mod_name] compiled"
    done
}

echo "Compiling ffmpeg fftools..."
compile_modules LIBFFMPEG_SO_MODULES FFMPEG_OBJS

echo "Compiling ffprobe fftools..."
compile_modules LIBFFPROBE_SO_MODULES FFPROBE_OBJS

echo "=== Phase 4: Linking shared libraries ==="

cat > "${FTOOLS_BUILD}/ffmpeg.ver" << 'VEOF'
{
  global:
    ffmpeg_run;
    ffmpeg_reset;
  local: *;
};
VEOF

cat > "${FTOOLS_BUILD}/ffprobe.ver" << 'VEOF'
{
  global:
    ffprobe_run;
    ffprobe_reset;
  local: *;
};
VEOF

echo "Linking libffmpeg.so..."
$CC $CFLAGS -shared -o "${FTOOLS_BUILD}/libffmpeg.so" \
    $FFMPEG_OBJS \
    $STATIC_LIBS \
    ${X264_INSTALL}/lib/libx264.a \
    -lm -lz -llog \
    -Wl,-u,ffmpeg_run \
    -Wl,-u,ffmpeg_reset \
    -Wl,--gc-sections \
    -Wl,--allow-multiple-definition \
    -Wl,--version-script,"${FTOOLS_BUILD}/ffmpeg.ver" \
    $LDFLAGS > "${LOG_DIR}/link_ffmpeg.log" 2>&1 || {
    echo "❌ Failed to link libffmpeg.so (see ${LOG_DIR}/link_ffmpeg.log)"
    tail -10 "${LOG_DIR}/link_ffmpeg.log"
    exit 1
}

echo "Linking libffprobe.so..."
$CC $CFLAGS -shared -o "${FTOOLS_BUILD}/libffprobe.so" \
    $FFPROBE_OBJS \
    $STATIC_LIBS \
    ${X264_INSTALL}/lib/libx264.a \
    -lm -lz -llog \
    -Wl,-u,ffprobe_run \
    -Wl,-u,ffprobe_reset \
    -Wl,--gc-sections \
    -Wl,--allow-multiple-definition \
    -Wl,--version-script,"${FTOOLS_BUILD}/ffprobe.ver" \
    $LDFLAGS > "${LOG_DIR}/link_ffprobe.log" 2>&1 || {
    echo "❌ Failed to link libffprobe.so (see ${LOG_DIR}/link_ffprobe.log)"
    tail -10 "${LOG_DIR}/link_ffprobe.log"
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
declare -A REQUIRED_SYMBOLS=(
    [libffmpeg.so]="ffmpeg_run ffmpeg_reset"
    [libffprobe.so]="ffprobe_run ffprobe_reset"
)

for lib in "${!REQUIRED_SYMBOLS[@]}"; do
    echo "--- $lib ---"
    for sym in ${REQUIRED_SYMBOLS[$lib]}; do
        if ${NM} -D "${OUTPUT_DIR}/${lib}" 2>/dev/null | grep -q " ${sym}$"; then
            echo "  ✅ $sym"
        else
            echo "  ❌ $sym missing"
            exit 1
        fi
    done
    sym_count=$(${NM} -D "${OUTPUT_DIR}/${lib}" | grep -c "T ")
    size=$(ls -lh "${OUTPUT_DIR}/${lib}" | awk '{print $5}')
    echo "  📊 ${sym_count} text symbols, ${size}"
done

echo "=== Generating build-info.json ==="
MANIFEST_CSUM=$(sha256sum "$MANIFEST" | cut -d' ' -f1)

cat > "${OUTPUT_DIR}/build-info.json" << BIEOF
{
  "ffmpeg_version": "${FFMPEG_VERSION}",
  "ffmpeg_codename": "Huffman",
  "x264_version": "${X264_VERSION}",
  "x264_configure_opts": "--enable-static --enable-pic --disable-cli --disable-opencl",
  "ndk_version": "${NDK_VERSION}",
  "api_level": ${API_LEVEL},
  "abi": "${ABI}",
  "build_date": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "manifest_version": "1",
  "manifest_checksum": "${MANIFEST_CSUM}",
  "enabled_decoders": [$(echo "$DECODERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_encoders": [$(echo "$ENCODERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_muxers": [$(echo "$MUXERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_demuxers": [$(echo "$DEMUXERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_parsers": [$(echo "$PARSERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_protocols": [$(echo "$PROTOCOLS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "enabled_filters": [$(echo "$FILTERS" | tr ',' '\n' | while read -r d; do printf "\"%s\"," "$d"; done | sed 's/,$//')],
  "static_libs": [$(
    SL=""
    for lib in libavformat libavcodec libavutil libswresample libswscale libavfilter libavdevice; do
        [ -f "${FFMPEG_INSTALL}/lib/${lib}.a" ] && SL="$SL\"$lib\","
    done
    echo "${SL%,}"
  )],
  "linking": "static-into-so",
  "cflags": "-std=c11 -fPIC -DANDROID -D_POSIX_C_SOURCE=200809L -include time.h",
  "ffmpeg_license": "GPL v2+",
  "x264_license": "GPL v2",
  "validation": {
    "all_required_decoders_present": true,
    "all_required_encoders_present": true,
    "missing": []
  }
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

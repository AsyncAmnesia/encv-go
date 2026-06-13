#!/usr/bin/env bash
# common/paths.sh - 跨平台路径检测
#
# 零硬编码原则：所有路径都从环境变量 / git / 系统探测 拿。
# 适用环境：沙盒（/workspace）+ CI runner（/home/runner/.../）+ 用户本机
#
# 用法：source "$(dirname "${BASH_SOURCE[0]}")/common/paths.sh"
# 暴露变量：
#   ROOT_DIR         - git repo 根
#   PROJECT_DIR      - encv-mobile/ 根
#   BUILDER_DIR      - ffmpeg-builder/ 根
#   MANIFEST_FILE    - ffmpeg-feature-manifest.json 路径
#   BUILD_ROOT       - 跨平台 build 根（按 target/host 区分）
#   OUTPUT_DIR       - 当前 target 的最终产物目录
#   LOG_DIR          - 当前 target 的 build 日志目录
#   TARGET           - host | android | ios
#   HOST_OS / HOST_ARCH - 当前主机平台

set -euo pipefail

# === 自动 source 基础库（log_info/die/run_logged/require_file 到处都要用） ===
# 路径：此文件在 common/paths.sh → 兄弟文件 logging.sh / exec.sh
# 用 BASH_SOURCE 拿绝对路径，避免被 caller 切到其他目录
_THIS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=logging.sh
source "$_THIS_DIR/logging.sh"
# shellcheck source=exec.sh
source "$_THIS_DIR/exec.sh"
unset _THIS_DIR

# === 根路径检测（git rev-parse 跨平台可靠） ===
# 不写死 monorepo 子目录结构（不假设 "app/encv-mobile"）
# 返回 git 仓库根；monorepo 子工程位置由 detect_mobile_dir 探测
detect_root() {
    if command -v git >/dev/null 2>&1; then
        local git_root
        git_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
        if [ -n "$git_root" ]; then
            echo "$git_root"
            return 0
        fi
    fi
    # fallback: 脚本在 <root>/<子工程>/scripts/ffmpeg-builder/common/paths.sh
    # 从 ${BASH_SOURCE[0]} 向上 5 级 = git 仓库根
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local candidate
    candidate="$(cd "$script_dir/../../../.." && pwd 2>/dev/null || true)"
    if [ -n "$candidate" ] && [ -d "$candidate/.git" -o -d "$candidate" ]; then
        echo "$candidate"
        return 0
    fi
    die "Cannot detect ROOT_DIR (no git toplevel, no fallback)"
}

# === monorepo 子工程（encv-mobile/）位置探测 ===
# 完全不写死 "app/encv-mobile" — 通过 env / file marker 探测
#
# 优先级：
#   1) $MONOREPO_MOBILE_DIR env (caller 显式告知)
#   2) find capacitor.config.{ts,js,json} (Capacitor mobile 工程最强标识)
#   3) find AndroidManifest.xml at android/app/src/main/ (AGP 标识)
#   4) find package.json with @capacitor/* dep (兜底)
#
# 返回 mobile 工程的绝对路径（父目录 = "encv-mobile"）
detect_mobile_dir() {
    # 1) env 优先
    if [ -n "${MONOREPO_MOBILE_DIR:-}" ] && [ -d "${MONOREPO_MOBILE_DIR}" ]; then
        (cd "$MONOREPO_MOBILE_DIR" && pwd)
        return 0
    fi

    local root="${ROOT_DIR:-}"
    if [ -z "$root" ]; then
        root="$(detect_root)"
    fi

    # 2) Capacitor 配置（mobile 工程最强标识）
    local found
    found="$(find "$root" -maxdepth 4 \
        \( -name "capacitor.config.ts" -o -name "capacitor.config.js" -o -name "capacitor.config.json" \) \
        -not -path "*/node_modules/*" -not -path "*/build/*" -not -path "*/dist/*" \
        2>/dev/null | head -1)"
    if [ -n "$found" ]; then
        dirname "$found"
        return 0
    fi

    # 3) AndroidManifest.xml at android/app/src/main/ (AGP 标识)
    found="$(find "$root" -maxdepth 6 -path "*/android/app/src/main/AndroidManifest.xml" \
        -not -path "*/build/*" -not -path "*/node_modules/*" \
        2>/dev/null | head -1)"
    if [ -n "$found" ]; then
        # /path/to/mobile/android/app/src/main/AndroidManifest.xml → /path/to/mobile
        dirname "$found" | sed 's|/android/app/src/main$||'
        return 0
    fi

    # 4) package.json 含 @capacitor/* 依赖（兜底）
    found="$(grep -lE '"@capacitor/' "$root"/*/package.json 2>/dev/null | head -1)"
    if [ -n "$found" ]; then
        dirname "$found"
        return 0
    fi

    die "Cannot detect MONOREPO_MOBILE_DIR. Set env var or ensure capacitor.config.* / AndroidManifest.xml exists"
}

# === Android NDK / SDK 路径检测（跨平台） ===
# 优先级：
#   1) $ANDROID_NDK_HOME (Android Studio 风格)
#   2) $ANDROID_HOME/ndk/<version> (sdkmanager 默认)
#   3) $ANDROID_SDK_ROOT/ndk/<version> (旧 sdkmanager)
#   4) find 命令搜 ~/.local / /opt / /usr/local（无硬编码根）
detect_ndk() {
    if [ -n "${ANDROID_NDK_HOME:-}" ] && [ -d "$ANDROID_NDK_HOME" ]; then
        echo "$ANDROID_NDK_HOME"
        return 0
    fi
    local android_home="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
    if [ -n "$android_home" ] && [ -d "$android_home/ndk" ]; then
        # 选最新版本（按字典序）
        local latest
        latest="$(ls -1 "$android_home/ndk" 2>/dev/null | sort -V | tail -1)"
        if [ -n "$latest" ] && [ -d "$android_home/ndk/$latest" ]; then
            echo "$android_home/ndk/$latest"
            return 0
        fi
    fi
    # 兜底：搜几个常见位置（不写死具体路径）
    for base in "$HOME" "/opt" "/usr/local"; do
        if [ -d "$base" ]; then
            local found
            found="$(find "$base" -maxdepth 5 -type d -name "ndk*" 2>/dev/null | grep -E "/ndk-[0-9]" | head -1 || true)"
            if [ -n "$found" ]; then
                echo "$found"
                return 0
            fi
        fi
    done
    return 1
}

detect_android_sdk() {
    if [ -n "${ANDROID_HOME:-}" ] && [ -d "$ANDROID_HOME" ]; then
        echo "$ANDROID_HOME"
        return 0
    fi
    if [ -n "${ANDROID_SDK_ROOT:-}" ] && [ -d "$ANDROID_SDK_ROOT" ]; then
        echo "$ANDROID_SDK_ROOT"
        return 0
    fi
    # 兜底：找 build-tools 目录定位 SDK
    for base in "$HOME" "/opt" "/usr/local"; do
        if [ -d "$base" ]; then
            local found
            found="$(find "$base" -maxdepth 5 -type d -name "build-tools" 2>/dev/null | head -1 || true)"
            if [ -n "$found" ]; then
                dirname "$found"
                return 0
            fi
        fi
    done
    return 1
}

# === 主机平台检测 ===
detect_host_os() {
    case "$(uname -s 2>/dev/null || echo unknown)" in
        Linux*)   echo linux  ;;
        Darwin*)  echo darwin ;;
        CYGWIN*|MINGW*|MSYS*) echo windows ;;
        *)        echo unknown ;;
    esac
}

detect_host_arch() {
    case "$(uname -m 2>/dev/null || echo unknown)" in
        x86_64|amd64)   echo x86_64  ;;
        aarch64|arm64)  echo aarch64 ;;
        i386|i686)      echo x86     ;;
        armv7l)         echo armv7   ;;
        *)              echo unknown ;;
    esac
}

# === 派生路径（不写死） ===
# 强制要求 caller 先 export TARGET=host|android|ios
compute_paths() {
    if [ -z "${TARGET:-}" ]; then
        echo "❌ TARGET not set (host|android|ios)" >&2
        return 1
    fi
    ROOT_DIR="$(detect_root)"
    PROJECT_DIR="$(detect_mobile_dir)"  # 探测 monorepo 子工程，零硬编码
    BUILDER_DIR="${PROJECT_DIR}/scripts/ffmpeg-builder"
    MANIFEST_FILE="${BUILDER_DIR}/ffmpeg-feature-manifest.json"
    if [ ! -f "$MANIFEST_FILE" ]; then
        MANIFEST_FILE="${PROJECT_DIR}/scripts/ffmpeg-feature-manifest.json"
    fi
    if [ ! -f "$MANIFEST_FILE" ]; then
        echo "❌ manifest not found: $MANIFEST_FILE" >&2
        return 1
    fi

    HOST_OS="$(detect_host_os)"
    HOST_ARCH="$(detect_host_arch)"

    # build 根：<project>/build/ffmpeg/<target>-<arch>/
    # 跨 target 隔离，跨 host 共享（CI cache 友好）
    BUILD_ROOT="${PROJECT_DIR}/build/ffmpeg/${TARGET}-${HOST_ARCH}"
    OUTPUT_DIR="${BUILD_ROOT}/out"
    LOG_DIR="${BUILD_ROOT}/logs"
    BUILD_DIR="${BUILD_ROOT}/src"  # 源码解压目录

    # Android jniLibs 路径（runtime 由 applicationInfo.nativeLibraryDir 取）
    # Android 标准布局: <mobile>/android/app/src/main/jniLibs
    JNI_LIBS_BASE="$(detect_jni_libs_base)"

    mkdir -p "$OUTPUT_DIR" "$LOG_DIR" "$BUILD_DIR"
}

# === Android jniLibs base 路径探测 ===
# 优先 env (CI workflow 显式传)，否则用 AGP 标准布局 + find AndroidManifest.xml 兜底
detect_jni_libs_base() {
    if [ -n "${ANDROID_JNI_LIBS_BASE:-}" ] && [ -d "${ANDROID_JNI_LIBS_BASE}" ]; then
        echo "$ANDROID_JNI_LIBS_BASE"
        return 0
    fi

    # AGP 标准布局：<mobile>/android/app/src/main/jniLibs
    local candidate="${PROJECT_DIR}/android/app/src/main/jniLibs"
    if [ -d "$candidate" ] || mkdir -p "$candidate" 2>/dev/null; then
        echo "$candidate"
        return 0
    fi

    # fallback: find AndroidManifest.xml 推断 jniLibs 位置
    local manifest_path
    manifest_path="$(find "${PROJECT_DIR}/android" -maxdepth 6 -name "AndroidManifest.xml" -path "*/app/src/main/*" -not -path "*/build/*" 2>/dev/null | head -1)"
    if [ -n "$manifest_path" ]; then
        # /path/mobile/android/app/src/main/AndroidManifest.xml → /path/mobile/android/app/src/main/jniLibs
        dirname "$manifest_path" | sed 's|/src/main$||' | xargs -I{} echo "{}/jniLibs"
        return 0
    fi

    die "Cannot detect JNI_LIBS_BASE (set ANDROID_JNI_LIBS_BASE or ensure AGP layout exists)"
}

# === 初始化入口（被 Makefile / 单 .sh 都调用） ===
ffmpeg_builder_init() {
    compute_paths
    export ROOT_DIR PROJECT_DIR BUILDER_DIR MANIFEST_FILE
    export HOST_OS HOST_ARCH
    export BUILD_ROOT OUTPUT_DIR LOG_DIR BUILD_DIR JNI_LIBS_BASE
}

# 兼容直接 source 模式
if [ "${FFMPEG_BUILDER_LIB_ONLY:-}" != "1" ]; then
    if [ -z "${TARGET:-}" ]; then
        : # 允许仅 source 不立即初始化（Makefile 会先 export TARGET）
    fi
fi

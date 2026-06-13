#!/usr/bin/env bash
# worker.sh - 编译 libffmpeg-worker.so (Go c-shared 子进程包装器)
#
# 为什么需要 worker：
#   真机 cgo ffmpeg_run 阻塞 OS thread，父进程 ctx cancel 无法 unblock。
#   解决：父进程 os.Exec 启子进程跑 ffmpeg_run，父进程 SIGKILL 子进程 → 父进程 unblock。
#   子进程 = libffmpeg-worker.so（Go c-shared 模式，main 是 JSON RPC 入口）。
#
# 用法（被 Makefile 调用）：
#   source worker.sh
#   build_worker
#
# 前置：build_fftools 完成（libffmpeg.so 存在）
# 输出：$OUTPUT_LIB_DIR/libffmpeg-worker.so
#
# 设计：
#   - Go module root 100% 走 git rev-parse，不写死 /workspace
#   - cgo cross-compile flags 全部走 TOOLCHAIN_BIN，零硬编码
#   - 仅在 android target 时构建（host 不需要 worker；真机才需要）

set -euo pipefail

build_worker() {
    log_section "build libffmpeg-worker.so (target=$TARGET)"

    # host target 不需要 worker（开发态）
    if [ "${TARGET:-}" = "host" ]; then
        log_warn "host target: skipping worker build (worker is Android-specific)"
        return 0
    fi

    require_cmd go "Install Go: https://go.dev/doc/install"
    require_file "${OUTPUT_LIB_DIR}/libffmpeg.so" "build fftools first"

    # === Go module root 探测（零硬编码） ===
    local gomod_root
    gomod_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
    if [ -z "$gomod_root" ] || [ ! -d "${gomod_root}/cmd/ffmpeg-worker" ]; then
        # fallback: 从 PROJECT_DIR 父目录（脚本跟 Go module 同一仓库时成立）
        gomod_root="$(cd "${PROJECT_DIR}/.." && pwd)"
    fi
    if [ ! -d "${gomod_root}/cmd/ffmpeg-worker" ]; then
        die "Go module root has no cmd/ffmpeg-worker/: $gomod_root"
    fi
    log_info "Go module root: $gomod_root"

    local worker_out="${BUILD_ROOT}/fftools-build/libffmpeg-worker.so"
    mkdir -p "$(dirname "$worker_out")"

    # === cgo env ===
    # 关键：必须显式传 --sysroot 给 clang，否则 ld.lld 用默认 sysroot 找不到 bionic libpthread.so
    # （Go 1.25 cmd/link 调 external linker 时已知不自动加 --sysroot）
    # SYSROOT 来自 target_android_setup（L66），必须 env 传到 worker
    # 派生：${TOOLCHAIN_BIN%/bin}/sysroot  ← %/bin 同时去掉 trailing /bin，路径无 //
    local SYSROOT="${SYSROOT:-${TOOLCHAIN_BIN%/bin}/sysroot}"
    if [ ! -d "$SYSROOT/usr/lib" ]; then
        die "NDK sysroot missing: $SYSROOT (target_android_setup 没设 SYSROOT?)"
    fi
    
    # =================================================================
    # FIX: Go 1.25 cmd/link HARDCODES -lpthread for Android targets
    # Android bionic has NO separate libpthread.so (pthread is in libc.so)
    # Solution: create a linker script stub that redirects to libc.so
    # =================================================================
    local android_arch_dir
    case "${TARGET_ARCH:-}" in
        aarch64) android_arch_dir="aarch64-linux-android" ;;  # arm64-v8a
        x86_64)  android_arch_dir="x86_64-linux-android" ;;
        armv7)   android_arch_dir="arm-linux-androideabi" ;;
        x86)     android_arch_dir="i686-linux-android" ;;
        *)       die "unknown TARGET_ARCH: ${TARGET_ARCH:-}" ;;
    esac

    local pthread_stub_dir="${SYSROOT}/usr/lib/${android_arch_dir}"
    local pthread_stub="${pthread_stub_dir}/libpthread.so"

    mkdir -p "$pthread_stub_dir"

    # Create a linker script that tells ld: libpthread.so = libc.so
    echo 'INPUT(-lc)' > "$pthread_stub"
    log_info "Go 1.25 FIX: created libpthread.so stub → $pthread_stub"
    log_info "  Stub content: INPUT(-lc) (redirects pthread to libc.so)"

    # Also create in parent dir as fallback
    echo 'INPUT(-lc)' > "${SYSROOT}/usr/lib/libpthread.so" 2>/dev/null || true
    # =================================================================
    # END FIX
    # =================================================================

    local cgo_cflags="--sysroot=${SYSROOT} -fPIC -DANDROID -I${SYSROOT}/usr/include"
    local cgo_ldflags="--sysroot=${SYSROOT} -llog -ldl -lm"
    # 运行时 dlopen libffmpeg.so，rpath 帮助非 dlopen 时的 fallback
    [ -d "${FFMPEG_INSTALL_DIR}/lib" ] && cgo_ldflags+=" -Wl,-rpath,${FFMPEG_INSTALL_DIR}/lib"

    local goos="${GOOS:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
    local goarch
    case "${TARGET_ARCH:-}" in
        aarch64) goarch="arm64" ;;
        x86_64)  goarch="amd64" ;;
        armv7)   goarch="arm"   ;;
        x86)     goarch="386"   ;;
        *)       die "unknown TARGET_ARCH for go cross-compile: ${TARGET_ARCH:-}" ;;
    esac

    # === go build -C（关键：用 -C 进 gomod_root，不写死路径） ===
    # Go 1.25+ 关键变化：-ldflags 不再透传 -Wl,xxx 给系统 linker（flag provided but not defined）
    # 必须用 -extldflags 透传 external linker flags
    # 旧 Go（≤1.24）: -ldflags='-s -w -Wl,-soname,libffmpeg-worker.so'
    # 新 Go（≥1.25）: -ldflags='-s -w -extldflags=-Wl,-soname,libffmpeg-worker.so'
    # 二者都设 SONAME=c-shared 包名外的自定义名（dlopen("libffmpeg-worker.so") 需要 SONAME 匹配）
    #
    # --sysroot 已在 CGO_CFLAGS / CGO_LDFLAGS 处理（LD 看 sysroot 才能找 bionic libpthread.so）。
    # 这里 -extldflags 只设 -soname。
    log_cmd "go build -C $gomod_root -buildmode=c-shared ./cmd/ffmpeg-worker/ → $worker_out"
    if ! GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=1 \
        CC="$CC" \
        CXX="${TOOLCHAIN_BIN}/clang++" \
        CGO_CFLAGS="$cgo_cflags" \
        CGO_LDFLAGS="$cgo_ldflags" \
        PKG_CONFIG_PATH="${DEPS_INSTALL_DIR}/lib/pkgconfig" \
        go build -C "$gomod_root" -buildmode=c-shared \
        -ldflags='-s -w -extldflags=-Wl,-soname,libffmpeg-worker.so' \
        -o "$worker_out" \
        ./cmd/ffmpeg-worker/ \
        > "${LOG_DIR}/ffmpeg-worker-build.log" 2>&1; then
        log_error "worker build failed (see ${LOG_DIR}/ffmpeg-worker-build.log)"
        tail -30 "${LOG_DIR}/ffmpeg-worker-build.log" >&2 || true
        die "worker build failed"
    fi
    log_ok "worker built: $worker_out"

    # === copy + strip ===
    cp "$worker_out" "${OUTPUT_LIB_DIR}/"
    $STRIP --strip-all "${OUTPUT_LIB_DIR}/libffmpeg-worker.so" 2>/dev/null || true

    # === verify main symbol ===
    if ! ${NM} -D "${OUTPUT_LIB_DIR}/libffmpeg-worker.so" 2>/dev/null | grep -q " main$"; then
        log_error "libffmpeg-worker.so missing 'main' symbol"
        ${NM} -D "${OUTPUT_LIB_DIR}/libffmpeg-worker.so" | grep -E " T " | head -5 >&2 || true
        die "worker verification failed"
    fi
    log_ok "libffmpeg-worker.so verified: $(ls -lh "${OUTPUT_LIB_DIR}/libffmpeg-worker.so" | awk '{print $5}')"
}

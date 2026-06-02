#!/usr/bin/env bash
# =============================================================================
# build-openlist-aar.sh
# -----------------------------------------------------------------------------
# Build the OpenList Android AAR (gomobile bind product) from the Hi-Sillot
# OpenList fork and drop it under --output/openlist.aar for ComboLite to
# consume via `app/encv-mobile/plugin-openlist/libs/openlist.aar`.
#
# Usage:
#   scripts/build-openlist-aar.sh \
#     --output  /workspace/app/encv-mobile/plugin-openlist/libs \
#     --fork    https://github.com/Hi-Sillot/OpenList \
#     --branch  main \
#     --ndk     "$ANDROID_HOME/ndk/26.3.11579264" \
#     --encv-go-root /workspace
#
# Required environment:
#   - Go 1.25.x     (matches Hi-Sillot fork go.mod)
#   - NDK r25c+     (r26b / 26.3.11579264 recommended)
#   - Java 17       (Temurin / OpenJDK)
#   - cmake, git, tar, curl, sha256sum
#
# Output:
#   <output>/openlist.aar            (gomobile bind product)
#   <output>/openlist.aar.sha256    (sha256 checksum, sidecar)
# =============================================================================
# TODO: keep NDK version in sync with .github/workflows/build-mpv-lib.yml.
# TODO: Hi-Sillot fork must already contain `openlistlib/` (see
#       .trae/specs/integrate-openlist-as-combolite-plugin/spec.md §一).
# =============================================================================

set -euo pipefail

FORK_DEFAULT="https://github.com/Hi-Sillot/OpenList"
BRANCH_DEFAULT="main"
NDK_DEFAULT="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}/ndk/26.3.11579264"
ENCV_GO_ROOT_DEFAULT="/workspace"

FORK="${FORK_DEFAULT}"
BRANCH="${BRANCH_DEFAULT}"
NDK="${NDK_DEFAULT}"
ENCV_GO_ROOT="${ENCV_GO_ROOT_DEFAULT}"
OUTPUT=""

usage() {
    cat <<EOF
Usage: $(basename "$0") --output <aar-dir> [options]

Options:
  --output         <dir>   Output directory for openlist.aar (required)
  --fork           <url>   Hi-Sillot fork URL       (default: ${FORK_DEFAULT})
  --branch         <name>  Git branch / tag          (default: ${BRANCH_DEFAULT})
  --ndk            <path>  Android NDK install path  (default: ${NDK_DEFAULT})
  --encv-go-root   <dir>   Local encv-go checkout    (default: ${ENCV_GO_ROOT_DEFAULT})
  -h, --help               Show this help
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --output)        OUTPUT="${2:-}"; shift 2 ;;
        --fork)          FORK="${2:-}"; shift 2 ;;
        --branch)        BRANCH="${2:-}"; shift 2 ;;
        --ndk)           NDK="${2:-}"; shift 2 ;;
        --encv-go-root)  ENCV_GO_ROOT="${2:-}"; shift 2 ;;
        -h|--help)       usage; exit 0 ;;
        *) echo "ERROR: unknown option: $1" >&2; usage; exit 2 ;;
    esac
done

if [[ -z "${OUTPUT}" ]]; then
    echo "ERROR: --output is required" >&2
    usage
    exit 2
fi

if [[ ! -d "${ENCV_GO_ROOT}" ]]; then
    echo "ERROR: encv-go root not found: ${ENCV_GO_ROOT}" >&2
    exit 2
fi

# Normalize encv-go root to an absolute, trailing-slash-free path for sed.
ENCV_GO_ROOT="$(cd "${ENCV_GO_ROOT}" && pwd)"
ENCV_GO_ROOT="${ENCV_GO_ROOT%/}"

# Normalize output to absolute path.
mkdir -p "${OUTPUT}"
OUTPUT="$(cd "${OUTPUT}" && pwd)"

log() { printf '\033[1;36m[openlist-aar]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[openlist-aar]\033[0m %s\n' "$*" >&2; exit 1; }

log "== Environment check =="
command -v go       >/dev/null 2>&1 || die "go not found in PATH"
command -v java     >/dev/null 2>&1 || die "java not found in PATH"
command -v git      >/dev/null 2>&1 || die "git not found in PATH"
command -v curl     >/dev/null 2>&1 || die "curl not found in PATH"
command -v tar      >/dev/null 2>&1 || die "tar not found in PATH"
command -v cmake    >/dev/null 2>&1 || die "cmake not found in PATH (NDK toolchain needs it)"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum not found in PATH"

# Resolve NDK absolute path; allow the user to pass either the NDK dir itself
# or its parent (e.g. /opt/android-sdk/ndk/26.3.11579264).
if [[ ! -d "${NDK}" ]]; then
    if [[ -d "${ANDROID_HOME:-}/ndk/26.3.11579264" ]]; then
        NDK="${ANDROID_HOME}/ndk/26.3.11579264"
    elif [[ -d "${ANDROID_HOME:-}/ndk/25.2.9519653" ]]; then
        NDK="${ANDROID_HOME}/ndk/25.2.9519653"
    else
        die "NDK not found at: ${NDK}"
    fi
fi
NDK="$(cd "${NDK}" && pwd)"
[[ -x "${NDK}/ndk-build" ]] || die "ndk-build not executable under ${NDK}"

log "== Toolchain =="
log "  go         : $(go version)"
log "  java       : $(java -version 2>&1 | head -n 1)"
log "  NDK        : ${NDK}"
log "  encv-go    : ${ENCV_GO_ROOT}"
log "  fork       : ${FORK}@${BRANCH}"
log "  output dir : ${OUTPUT}"

WORK_DIR="${TMPDIR:-/tmp}/openlist-aar-build"
SRC_DIR="${WORK_DIR}/openlist"

log "== Workspace =="
log "  ${WORK_DIR}"
rm -rf "${SRC_DIR}"
mkdir -p "${WORK_DIR}"

log "== Clone Hi-Sillot fork (--depth 1) =="
git clone --depth 1 --branch "${BRANCH}" "${FORK}" "${SRC_DIR}"

GOMOD="${SRC_DIR}/go.mod"
[[ -f "${GOMOD}" ]] || die "go.mod not found in ${SRC_DIR}"

# -----------------------------------------------------------------------------
# Hi-Sillot ships the encv-go replace as `../../../` which is meaningless once
# the fork is cloned into a temp dir. Rewrite it to the user-supplied local
# encv-go root so gomobile bind can resolve `github.com/Soltus/encv-go`.
# -----------------------------------------------------------------------------
log "== Patch go.mod replace directive =="
if grep -qE '^replace[[:space:]]+github\.com/Soltus/encv-go[[:space:]]+=>' "${GOMOD}"; then
    sed -i.bak -E "s|^replace[[:space:]]+github\\.com/Soltus/encv-go[[:space:]]+=>[[:space:]]+[^[:space:]]+|replace github.com/Soltus/encv-go => ${ENCV_GO_ROOT}|" "${GOMOD}"
    grep -E '^replace[[:space:]]+github\.com/Soltus/encv-go' "${GOMOD}" || die "go.mod replace patch failed"
else
    log "  (no encv-go replace line found, appending one)"
    printf '\nreplace github.com/Soltus/encv-go => %s\n' "${ENCV_GO_ROOT}" >> "${GOMOD}"
fi
rm -f "${GOMOD}.bak"

# -----------------------------------------------------------------------------
# Prepare OpenList-Frontend dist (Web assets) — required because OpenList
# embeds `public/dist` into the running binary at startup. The frontend is
# downloaded as a release tarball, not built from source.
# -----------------------------------------------------------------------------
log "== Fetch OpenList-Frontend dist =="
DIST_DIR="${SRC_DIR}/public/dist"
mkdir -p "${DIST_DIR}"

FE_API="https://api.github.com/repos/OpenListTeam/OpenList-Frontend/releases/latest"
RELEASE_INFO="$(curl -fsSL --max-time 15 -H 'Accept: application/vnd.github.v3+json' "${FE_API}" || true)"
if [[ -z "${RELEASE_INFO}" ]]; then
    die "failed to query OpenList-Frontend releases/latest"
fi

if command -v jq >/dev/null 2>&1; then
    DL_URL="$(printf '%s' "${RELEASE_INFO}" \
        | jq -r '.assets[] | select(.browser_download_url | test("openlist-frontend-dist.*\\.tar\\.gz$")) | select(.browser_download_url | test("openlist-frontend-dist-lite") | not) | .browser_download_url' \
        | head -n 1)"
else
    DL_URL="$(printf '%s' "${RELEASE_INFO}" \
        | grep -oE '"browser_download_url"[[:space:]]*:[[:space:]]*"[^"]*openlist-frontend-dist[^"]*\.tar\.gz"' \
        | grep -v 'lite' | head -n 1 \
        | sed -E 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
fi
[[ -n "${DL_URL}" && "${DL_URL}" != "null" ]] || die "could not resolve frontend tarball URL"
log "  frontend: ${DL_URL}"

TMP_TAR="${WORK_DIR}/openlist-frontend-dist.tar.gz"
curl -fsSL --max-time 60 -o "${TMP_TAR}" "${DL_URL}"
tar -xzf "${TMP_TAR}" -C "${DIST_DIR}" --strip-components=1
rm -f "${TMP_TAR}"
[[ -f "${DIST_DIR}/index.html" ]] || die "frontend dist extraction failed (no index.html)"

# -----------------------------------------------------------------------------
# NDK env + gomobile toolchain.
# -----------------------------------------------------------------------------
log "== Set up NDK env =="
export ANDROID_HOME="${ANDROID_HOME:-$(dirname "$(dirname "${NDK}")")}"
export ANDROID_NDK_HOME="${NDK}"
log "  ANDROID_HOME=${ANDROID_HOME}"
log "  ANDROID_NDK_HOME=${ANDROID_NDK_HOME}"

log "== Install / update gomobile =="
GOPATH_BIN="$(go env GOPATH)/bin"
mkdir -p "${GOPATH_BIN}"
export PATH="${GOPATH_BIN}:${PATH}"
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init -ndk "${NDK}"

# -----------------------------------------------------------------------------
# Resolve the gobindable package directory. Hi-Sillot fork is expected to
# provide `openlistlib/` (added per the spec). Fall back to `cmd/openlist`
# if it is missing — this keeps the script useful for testing against the
# upstream tree where the gobind entrypoint does not yet exist.
# -----------------------------------------------------------------------------
cd "${SRC_DIR}"
BIND_PKG=""
if [[ -d "openlistlib" ]] && ls openlistlib/*.go >/dev/null 2>&1; then
    BIND_PKG="./openlistlib"
elif [[ -d "cmd/openlistlib" ]] && ls cmd/openlistlib/*.go >/dev/null 2>&1; then
    BIND_PKG="./cmd/openlistlib"
else
    die "Hi-Sillot fork is missing openlistlib/ (see spec §一) and no fallback exists"
fi
log "== gomobile bind (bind pkg: ${BIND_PKG}) =="

LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.Version=${BRANCH}'"
LDFLAGS="${LDFLAGS} -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.WebVersion=rolling'"
LDFLAGS="${LDFLAGS} -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.BuiltAt=$(date +'%F %T %z')'"
LDFLAGS="${LDFLAGS} -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.GitAuthor=The OpenList Projects Contributors <noreply@openlist.team>'"
LDFLAGS="${LDFLAGS} -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.GitCommit=$(git -C "${SRC_DIR}" rev-parse --short HEAD)'"

cd "${SRC_DIR}"
gomobile bind \
    -ldflags "${LDFLAGS}" \
    -v \
    -androidapi 19 \
    -target="android/arm64" \
    -o "${OUTPUT}/openlist.aar" \
    "${BIND_PKG}"

[[ -s "${OUTPUT}/openlist.aar" ]] || die "openlist.aar was not produced"

log "== Checksum =="
( cd "${OUTPUT}" && sha256sum openlist.aar > openlist.aar.sha256 )
cat "${OUTPUT}/openlist.aar.sha256"

log "== Done =="
log "  AAR  : ${OUTPUT}/openlist.aar"
log "  SIZE : $(du -h "${OUTPUT}/openlist.aar" | cut -f1)"

#!/usr/bin/env bash
# =============================================================================
# build-openlist-aar.sh
# -----------------------------------------------------------------------------
# Build the OpenList Android AAR (gomobile bind product) from the Hi-Sillot
# OpenList fork and drop it under --output/openlist.aar for ComboLite to
# consume via `app/encv-mobile/plugin-openlist/libs/openlist.aar`.
#
# Required environment:
#   - Go 1.25.x     (matches Hi-Sillot fork go.mod)
#   - NDK r25c+     (r26b / 26.3.11579264 recommended)
#   - Java 17       (Temurin / OpenJDK)
#   - cmake, git, tar, curl, sha256sum, jq (jq only required when fork ships
#     public/dist/i18n-overlay/)
#
# Configuration precedence (highest first):
#   1. CLI flags                                (--fork / --branch / ...)
#   2. scripts/openlist-fork.env.local          (gitignored personal override)
#   3. scripts/openlist-fork.env                (tracked default)
#   4. hard-coded fallback                      (only when above are absent)
# =============================================================================
# TODO: keep NDK version in sync with .github/workflows/build-mpv-lib.yml.
# TODO: Hi-Sillot fork must already contain `openlistlib/` (see
#       .trae/specs/integrate-openlist-as-combolite-plugin/spec.md §一).
# TODO: when adding new ENCV setting items in the fork, bump the version
#       recorded in Hi-Sillot/OpenList/frontend-pinned.txt.
# =============================================================================

set -euo pipefail

source "$(dirname "$0")/openlist-fork.env" 2>/dev/null || true
if [[ -f "$(dirname "$0")/openlist-fork.env.local" ]]; then
    # shellcheck disable=SC1091
    source "$(dirname "$0")/openlist-fork.env.local"
fi

NDK_DEFAULT="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}/ndk/26.3.11579264"
ENCV_GO_ROOT_DEFAULT="/workspace"

FORK="${OPENLIST_FORK_URL:-https://github.com/Hi-Sillot/OpenList}"
BRANCH="${OPENLIST_FORK_BRANCH:-dev}"
NDK="${NDK_DEFAULT}"
ENCV_GO_ROOT="${ENCV_GO_ROOT_DEFAULT}"
OUTPUT=""
FRONTEND_VERSION_CLI=""
LOCAL_FRONTEND_DIST=""

usage() {
    cat <<EOF
Usage: $(basename "$0") --output <aar-dir> [options]

Options:
  --output                 <dir>    Output directory for openlist.aar (required)
  --fork                   <url>    Hi-Sillot fork URL       (default: ${FORK})
  --branch                 <name>   Git branch / tag          (default: ${BRANCH})
  --ndk                    <path>   Android NDK install path  (default: ${NDK_DEFAULT})
  --encv-go-root           <dir>    Local encv-go checkout    (default: ${ENCV_GO_ROOT_DEFAULT})
  --frontend-version       <vX.Y.Z> Pin OpenList-Frontend version (overrides env and frontend-pinned.txt)
  --local-frontend-dist    <dir>    Skip download, copy local frontend dist directly into public/dist/
  -h, --help                           Show this help

Defaults are loaded from scripts/openlist-fork.env:
  OPENLIST_FORK_URL / OPENLIST_FORK_BRANCH / OPENLIST_FRONTEND_VERSION
CLI flags always win over env values.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --output)                OUTPUT="${2:-}"; shift 2 ;;
        --fork)                  FORK="${2:-}"; shift 2 ;;
        --branch)                BRANCH="${2:-}"; shift 2 ;;
        --ndk)                   NDK="${2:-}"; shift 2 ;;
        --encv-go-root)          ENCV_GO_ROOT="${2:-}"; shift 2 ;;
        --frontend-version)      FRONTEND_VERSION_CLI="${2:-}"; shift 2 ;;
        --local-frontend-dist)   LOCAL_FRONTEND_DIST="${2:-}"; shift 2 ;;
        -h|--help)               usage; exit 0 ;;
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

ENCV_GO_ROOT="$(cd "${ENCV_GO_ROOT}" && pwd)"
ENCV_GO_ROOT="${ENCV_GO_ROOT%/}"

mkdir -p "${OUTPUT}"
OUTPUT="$(cd "${OUTPUT}" && pwd)"

log() { printf '\033[1;36m[openlist-aar]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[openlist-aar]\033[0m %s\n' "$*" >&2; exit 1; }

log "== fork env =="
log "  OPENLIST_FORK_BRANCH=${OPENLIST_FORK_BRANCH:-<unset>}"
log "  OPENLIST_FRONTEND_VERSION=${OPENLIST_FRONTEND_VERSION:-<unset>}"

log "== Environment check =="
command -v go       >/dev/null 2>&1 || die "go not found in PATH"
command -v java     >/dev/null 2>&1 || die "java not found in PATH"
command -v git      >/dev/null 2>&1 || die "git not found in PATH"
command -v curl     >/dev/null 2>&1 || die "curl not found in PATH"
command -v tar      >/dev/null 2>&1 || die "tar not found in PATH"
command -v cmake    >/dev/null 2>&1 || die "cmake not found in PATH (NDK toolchain needs it)"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum not found in PATH"
command -v jq       >/dev/null 2>&1 || log "  (jq not found, will skip i18n overlay merge if fork ships it)"

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
log "  frontend-version CLI : ${FRONTEND_VERSION_CLI:-<none>}"
log "  local-frontend-dist  : ${LOCAL_FRONTEND_DIST:-<none>}"

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

log "== Patch go.mod replace directive =="
if grep -qE '^replace[[:space:]]+github\.com/Soltus/encv-go[[:space:]]+=>' "${GOMOD}"; then
    sed -i.bak -E "s|^replace[[:space:]]+github\\.com/Soltus/encv-go[[:space:]]+=>[[:space:]]+[^[:space:]]+|replace github.com/Soltus/encv-go => ${ENCV_GO_ROOT}|" "${GOMOD}"
    grep -E '^replace[[:space:]]+github\.com/Soltus/encv-go' "${GOMOD}" || die "go.mod replace patch failed"
else
    log "  (no encv-go replace line found, appending one)"
    printf '\nreplace github.com/Soltus/encv-go => %s\n' "${ENCV_GO_ROOT}" >> "${GOMOD}"
fi
rm -f "${GOMOD}.bak"

log "== Resolve frontend version =="
DIST_DIR="${SRC_DIR}/public/dist"
mkdir -p "${DIST_DIR}"

FRONTEND_VERSION=""

if [[ -n "${LOCAL_FRONTEND_DIST}" ]]; then
    [[ -d "${LOCAL_FRONTEND_DIST}" ]] || die "--local-frontend-dist path not found: ${LOCAL_FRONTEND_DIST}"
    log "  source: local dist at ${LOCAL_FRONTEND_DIST}"
    rm -rf "${DIST_DIR}"
    mkdir -p "${DIST_DIR}"
    cp -a "${LOCAL_FRONTEND_DIST}/." "${DIST_DIR}/"
    [[ -f "${DIST_DIR}/index.html" ]] || die "local frontend dist missing index.html after copy"
    FRONTEND_VERSION="${FRONTEND_VERSION_CLI:-${OPENLIST_FRONTEND_VERSION:-local}}"
    log "  version: ${FRONTEND_VERSION} (label, not a real upstream tag)"
else
    if [[ -f "${SRC_DIR}/frontend-pinned.txt" ]]; then
        PINNED="$(cat "${SRC_DIR}/frontend-pinned.txt" 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?' | head -n 1 || true)"
        if [[ -n "${PINNED}" ]]; then
            FRONTEND_VERSION="${PINNED}"
            log "  source: fork frontend-pinned.txt"
        fi
    fi
    if [[ -z "${FRONTEND_VERSION}" && -n "${FRONTEND_VERSION_CLI}" ]]; then
        FRONTEND_VERSION="${FRONTEND_VERSION_CLI}"
        log "  source: --frontend-version CLI"
    fi
    if [[ -z "${FRONTEND_VERSION}" && -n "${OPENLIST_FRONTEND_VERSION:-}" ]]; then
        FRONTEND_VERSION="${OPENLIST_FRONTEND_VERSION}"
        log "  source: OPENLIST_FRONTEND_VERSION env"
    fi
    if [[ -z "${FRONTEND_VERSION}" ]]; then
        FRONTEND_VERSION="latest"
        echo "[WARN] no frontend pin, using latest" >&2
        log "  source: fallback (releases/latest) — pin via frontend-pinned.txt to silence this warning"
    fi
    log "  version: ${FRONTEND_VERSION}"

    if [[ "${FRONTEND_VERSION}" == "latest" ]]; then
        FE_API="https://api.github.com/repos/OpenListTeam/OpenList-Frontend/releases/latest"
    else
        FE_API="https://api.github.com/repos/OpenListTeam/OpenList-Frontend/releases/tags/${FRONTEND_VERSION}"
    fi

    RELEASE_INFO="$(curl -fsSL --max-time 15 -H 'Accept: application/vnd.github.v3+json' "${FE_API}" || true)"
    if [[ -z "${RELEASE_INFO}" ]]; then
        die "OpenList-Frontend ${FRONTEND_VERSION} not found (or API rate-limited)"
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
    [[ -n "${DL_URL}" && "${DL_URL}" != "null" ]] || die "could not resolve frontend tarball URL for ${FRONTEND_VERSION}"
    log "  frontend: ${DL_URL}"

    TMP_TAR="${WORK_DIR}/openlist-frontend-dist.tar.gz"
    curl -fsSL --max-time 60 -o "${TMP_TAR}" "${DL_URL}"
    tar -xzf "${TMP_TAR}" -C "${DIST_DIR}" --strip-components=1
    rm -f "${TMP_TAR}"
    [[ -f "${DIST_DIR}/index.html" ]] || die "frontend dist extraction failed (no index.html)"
fi

log "== Apply i18n overlay =="
OVERLAY_DIR="${SRC_DIR}/public/dist/i18n-overlay"
if [[ -d "${OVERLAY_DIR}" ]]; then
    if ! command -v jq >/dev/null 2>&1; then
        die "i18n-overlay/ exists in fork but jq is not installed"
    fi
    ASSETS_DIR="${DIST_DIR}/assets"
    if [[ -d "${ASSETS_DIR}" ]]; then
        find "${OVERLAY_DIR}" -type f -name 'translation.json' | while read -r overlay_file; do
            rel="${overlay_file#${OVERLAY_DIR}/}"
            lang="${rel%%/*}"
            target="${ASSETS_DIR}/${lang}.json"
            if [[ -f "${target}" ]]; then
                tmp="$(mktemp)"
                if jq -s '.[0] * .[1]' "${target}" "${overlay_file}" > "${tmp}"; then
                    mv "${tmp}" "${target}"
                    log "  merged ${lang}: $(basename "${overlay_file}")"
                else
                    rm -f "${tmp}"
                    die "jq merge failed for ${lang}"
                fi
            else
                log "  skipped ${lang}: ${target} not present in frontend dist"
            fi
        done
    else
        log "  (no ${ASSETS_DIR} in frontend dist, skipping overlay merge)"
    fi
else
    log "  (no i18n-overlay/ in fork, nothing to merge)"
fi

log "== Write public/dist/VERSION =="
echo "${FRONTEND_VERSION}-encv" > "${DIST_DIR}/VERSION"
log "  ${DIST_DIR}/VERSION = $(cat "${DIST_DIR}/VERSION")"

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
LDFLAGS="${LDFLAGS} -X 'github.com/OpenListTeam/OpenList/v4/internal/conf.WebVersion=${FRONTEND_VERSION}'"
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

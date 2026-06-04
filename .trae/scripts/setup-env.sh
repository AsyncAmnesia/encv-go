#!/usr/bin/env bash
# /workspace/.trae/scripts/setup-env.sh
# 沙箱环境一键准备：Hi-Sillot OpenList 双 fork + Kotlin 编译器
#
# Why:
#   - 沙箱新会话 /tmp 与 /workspace/app/openlist/{Hi-Sillot-OpenList,Hi-Sillot-OpenList-Frontend} 都为空
#   - 手动跑 git clone × 2 + bash setup-kotlinc.sh 易漏（漏切 dev / 漏 GITHUB_TOKEN / 漏 jar）
#   - 统一入口便于 AI 会话开头一把梭，省 3~5 分钟排查
#
# What (4 步):
#   1. 克隆 Hi-Sillot/OpenList (dev)      → app/openlist/Hi-Sillot-OpenList
#   2. 克隆 Hi-Sillot/OpenList-Frontend (main) → app/openlist/Hi-Sillot-OpenList-Frontend
#   3. 调 setup-kotlinc.sh 拉 4 个 jar + 写 /usr/local/bin/kotlinc-<ver>
#   4. 状态报告 + 下一步建议
#
# 退出码:
#   0 = 全部就绪
#   1 = 前置缺 (git/java 不在 PATH, app/openlist 不存在)
#   2 = clone 失败 (沙箱到 GitHub 不通 / repo 不存在 / 网络超时)
#   3 = kotlinc 准备失败
#   4 = 分支切换失败
#
# 幂等:
#   - 仓库目录已存在 .git → 跳过 clone，仅校验分支；不匹配自动 checkout
#   - 仓库目录存在但非 .git → 报错退出，避免覆盖用户文件
#
# 设计参考:
#   - setup-kotlinc.sh 的 6 步骤结构 + step() 函数
#   - app/openlist/README.md §10 的 GITHUB_TOKEN URL 注入
set -euo pipefail

# ---- 路径定位 ----
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." 2>/dev/null && pwd)"
OPENLIST_DIR="${REPO_ROOT}/app/openlist"
KOTLIN_SETUP="${SCRIPT_DIR}/setup-kotlinc.sh"

# ---- 工具函数 ----
step() { echo ""; echo "==> $*"; }
fail() { echo "❌ $*" >&2; exit "${2:-1}"; }

# ---- 凭证注入 ----
# 沙箱 GITHUB_TOKEN 已 export，但 git 不会自动用，需 URL 注入
# 参考 app/openlist/README.md §10.2 方案 1
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  GH_AUTH_URL="https://x-access-token:${GITHUB_TOKEN}@github.com"
  echo "    GITHUB_TOKEN 已注入 (前 4 字符: ${GITHUB_TOKEN:0:4}****)"
else
  GH_AUTH_URL="https://github.com"
  echo "    GITHUB_TOKEN 未设置，公开 clone（无 push 权限，写操作需先 export）"
fi

# ---- 克隆/更新仓库 ----
# 幂等：.git 存在 → 跳过 clone + 校验分支；不存在 → clone + 失败回滚
ensure_clone() {
  local dest="$1"
  local repo="$2"   # 例: Hi-Sillot/OpenList
  local branch="$3" # 例: dev

  if [[ -d "${dest}" && ! -d "${dest}/.git" ]]; then
    fail "目标已存在但非 git repo: ${dest}（手动 rm -rf 后重试）" 2
  fi

  if [[ -d "${dest}/.git" ]]; then
    echo "    [skip] ${dest} 已存在"
    local cur_branch
    cur_branch=$(git -C "${dest}" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "(unknown)")
    echo "    [info] 当前分支: ${cur_branch}（期望: ${branch}）"
    if [[ "${cur_branch}" != "${branch}" ]]; then
      echo "    [fix] 切换到 ${branch}..."
      if ! git -C "${dest}" checkout "${branch}" 2>&1 | tail -3; then
        fail "分支切换失败: ${dest} → ${branch}" 4
      fi
    fi
  else
    echo "    [clone] ${GH_AUTH_URL}/${repo}.git  --branch ${branch}  →  ${dest}"
    if ! git clone --branch "${branch}" --single-branch --depth 50 \
        "${GH_AUTH_URL}/${repo}.git" "${dest}" 2>&1 | tail -8; then
      rm -rf "${dest}"
      fail "clone 失败: ${repo}（沙箱到 GitHub 不通 / 分支 ${branch} 不存在）" 2
    fi
  fi
}

# ---- Step 0: 前置检查 ----
step "0/4 前置检查（git / java / app/openlist）"
command -v git  >/dev/null 2>&1 || fail "git 不在 PATH" 1
command -v java >/dev/null 2>&1 || fail "java 不在 PATH（沙箱应通过 mise 装好 OpenJDK 25）" 1
[[ -d "${OPENLIST_DIR}" ]] || fail "目标目录不存在: ${OPENLIST_DIR}" 1
echo "    git:    $(command -v git) ($(git --version))"
echo "    java:   $(command -v java) ($(java -version 2>&1 | head -1))"
echo "    target: ${OPENLIST_DIR}"

# ---- Step 1: 克隆主 fork (dev) ----
step "1/4 克隆 Hi-Sillot/OpenList (dev 分支) → app/openlist/Hi-Sillot-OpenList"
ensure_clone "${OPENLIST_DIR}/Hi-Sillot-OpenList" "Hi-Sillot/OpenList" "dev"
echo "    HEAD: $(git -C "${OPENLIST_DIR}/Hi-Sillot-OpenList" rev-parse --short HEAD 2>/dev/null)"

# ---- Step 2: 克隆前端 fork (main) ----
step "2/4 克隆 Hi-Sillot/OpenList-Frontend (main 分支) → app/openlist/Hi-Sillot-OpenList-Frontend"
ensure_clone "${OPENLIST_DIR}/Hi-Sillot-OpenList-Frontend" "Hi-Sillot/OpenList-Frontend" "main"
echo "    HEAD: $(git -C "${OPENLIST_DIR}/Hi-Sillot-OpenList-Frontend" rev-parse --short HEAD 2>/dev/null)"

# ---- Step 3: Kotlin 编译器准备 ----
step "3/4 调用 setup-kotlinc.sh 准备 Kotlin 编译器（Maven Central 4 个 jar）"
[[ -f "${KOTLIN_SETUP}" ]] || fail "找不到 setup-kotlinc.sh: ${KOTLIN_SETUP}" 1
if ! bash "${KOTLIN_SETUP}"; then
  fail "setup-kotlinc.sh 失败（exit $?，可能沙箱到 Maven Central 不通）" 3
fi
# 从 libs.versions.toml 解析版本号以便报告
KOTLIN_VERSION="$(grep -E '^kotlin\s*=' "${REPO_ROOT}/app/encv-mobile/android/gradle/libs.versions.toml" 2>/dev/null \
    | head -1 | sed -E 's/.*"([^"]+)".*/\1/' || echo '?')"

# ---- Step 4: 状态报告 ----
step "4/4 ✅ 沙箱环境就绪"
cat <<EOF

========================================
✅ ENCV 沙箱开发环境已就绪

  Fork 1 (build script 依赖，dev 分支):
    ${OPENLIST_DIR}/Hi-Sillot-OpenList
    HEAD: $(git -C "${OPENLIST_DIR}/Hi-Sillot-OpenList" rev-parse --short HEAD 2>/dev/null)

  Fork 2 (前端 i18n 同步源，main 分支):
    ${OPENLIST_DIR}/Hi-Sillot-OpenList-Frontend
    HEAD: $(git -C "${OPENLIST_DIR}/Hi-Sillot-OpenList-Frontend" rev-parse --short HEAD 2>/dev/null)

  Kotlin 编译器:
    /usr/local/bin/kotlinc-${KOTLIN_VERSION}
    KOTLIN_HOME: /tmp/kotlin-home

  下一步推荐:
    # 1) 启动 ENCV 预览（Vite + encv 后端，detached 模式不阻塞 terminal）
    ENCV_DETACH=1 bash /workspace/app/encv-mobile/scripts/start-preview.sh

    # 2) Kotlin 语法自检
    /usr/local/bin/kotlinc-${KOTLIN_VERSION} -version

    # 3) 跑 OpenList dev 模式（端口 5244，浏览器直访）
    bash /workspace/app/encv-mobile/scripts/dev-openlist.sh --data /tmp/preview-openlist-data

  清理（重做时）:
    rm -rf ${OPENLIST_DIR}/Hi-Sillot-OpenList ${OPENLIST_DIR}/Hi-Sillot-OpenList-Frontend
    bash ${KOTLIN_SETUP}  # 内部检查/重建 jar
========================================
EOF
exit 0

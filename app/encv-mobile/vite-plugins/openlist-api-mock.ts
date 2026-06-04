import type { Plugin } from "vite"
import type { Connect } from "vite"

/**
 * openlist-api-mock：在 encv-mobile vite 上拦截 OpenList 公共 API 端点。
 *
 * 为什么需要 mock：
 *   OpenList 公共 API（/api/public/settings、/api/public/archive_extensions 等）
 *   在 App.tsx 启动时由 Solid 应用调用，失败会让 UI 永远停在 FullScreenLoading。
 *   沙箱预览模式下不强制要求起 OpenList backend（构建 5+ 分钟、需 sqlite + 配置），
 *   用一个轻量 mock 让 UI 跑起来。
 *
 * 数据源（按优先级）：
 *   1. 真 OpenList backend（Hi-Sillot-OpenList 二进制） — 通过 OPENLIST_API_UPSTREAM 配置
 *   2. 本 mock（默认）
 *
 * 切换方式：
 *   - 想要真后端：在 start-preview.sh 加 Step 6 起 `openlist server`（:5244），
 *     并 export OPENLIST_API_UPSTREAM=http://127.0.0.1:5244
 *   - 想要 mock：不设 OPENLIST_API_UPSTREAM
 *
 * 通用 mock 规则：
 *   - GET /api/public/settings → 200 { code:200, message:"success", data: {} }
 *   - GET /api/public/archive_extensions → 200 { code:200, message:"success", data: [] }
 *   - 其它 /api/*（非 encv-go 处理的）→ 200 { code:200, message:"mock", data: null }
 *     （让请求不挂起、不污染控制台）
 */
export function openlistApiMock(opts: { upstream?: string } = {}): Plugin {
  const upstream = opts.upstream || process.env.OPENLIST_API_UPSTREAM || ""

  const handler: Connect.NextHandleFunction = async (req, res, next) => {
    const url = req.url || ""
    const pathOnly = url.split("?")[0]

    // 只 mock 公共 OpenList API；encv-go 的 /api/* 继续走原本的 proxy
    // 兼容：Solid app 在 api=undefined 时会把请求发到 /undefined/api/public/*
    const isPublic = pathOnly.startsWith("/api/public/")
    const isBrokenPublic = pathOnly.startsWith("/undefined/api/public/")
    if (!isPublic && !isBrokenPublic) {
      // 其它 /api/ 在 vite config 里已有 proxy，不处理
      return next()
    }
    // 标准化 url（去掉前缀的 /undefined），让 mock 逻辑统一处理
    const normalizedUrl = isBrokenPublic ? pathOnly.replace(/^\/undefined/, "") + (url.includes("?") ? url.slice(url.indexOf("?")) : "") : url
    // 调试日志：第一次重启后观察一下
    if (process.env.OPENLIST_API_MOCK_DEBUG === "1") {
      console.log(`[openlist-api-mock] ${req.method} ${url} → mock (normalized=${normalizedUrl})`)
    }

    // 有 upstream 时转发（用 globalThis.fetch，避免拉额外依赖）
    if (upstream) {
      try {
        const target = upstream.replace(/\/$/, "") + url
        const upstreamRes = await fetch(target, {
          method: req.method,
          headers: req.headers as any,
          body: ["GET", "HEAD"].includes(req.method || "GET")
            ? undefined
            : (await new Promise<string>((resolve) => {
                let data = ""
                req.on("data", (chunk) => (data += chunk))
                req.on("end", () => resolve(data))
              })),
        })
        res.statusCode = upstreamRes.status
        upstreamRes.headers.forEach((v, k) => res.setHeader(k, v))
        const buf = Buffer.from(await upstreamRes.arrayBuffer())
        res.end(buf)
        return
      } catch (e) {
        console.warn(`[openlist-api-mock] upstream ${upstream}${normalizedUrl} failed, falling back to mock:`, e)
        // 降级到本地 mock
      }
    }

    // 本地 mock
    res.setHeader("content-type", "application/json; charset=utf-8")
    res.statusCode = 200

    let data: any = null
    if (normalizedUrl === "/api/public/settings" || normalizedUrl === "/api/public/settings/") {
      data = {
        // 默认 OpenList 设置
        version: "dev",
        site_title: "OpenList",
        login_bg: "",
        login_logo: "",
        favicon: "https://res.oplist.org/logo/logo.svg",
        main_color: "",
        icon_color: "",
        default_page: "home",
        school_titles: [],
        home_readme: "",
        home_icon: "",
        home_pinned: [],
        search_enabled: true,
        // ENCV 插件相关（不设置时 OpenList 不展示）
        // encv_enabled: false,
      }
    } else if (normalizedUrl === "/api/public/archive_extensions" || normalizedUrl === "/api/public/archive_extensions/") {
      data = []
    }

    res.end(JSON.stringify({ code: 200, message: "success", data }))
  }

  return {
    name: "openlist-api-mock",
    configureServer(server) {
      // 注册在 /api/public 前缀，优先于 proxy
      server.middlewares.use(handler)
    },
  }
}

export default openlistApiMock

package com.encvgo.plugin.openlist

import android.content.Context
import android.util.Log
import androidx.compose.runtime.Composable
import com.combo.core.api.IPluginEntryClass
import com.combo.core.model.PluginContext
import org.koin.core.module.Module

/**
 * ComboLite 插件入口（合规修复版）
 *
 * 合规修复（参考 MpvPluginEntry 模式）：
 *   - pluginModule = emptyList()（不注册 Koin module）
 *   - onLoad() 初始化运行时（OpenListBridge + Service）
 *   - onUnload() 清理运行时
 *   - Content() 渲染嵌入式 WebView 承载 OpenList UI（详见 OpenListEmbedWebView）
 *
 * UI 架构变更（spec openlist-extension-rewrite-capacitor-ui）：
 *   - 删除原本的 Compose Material3 UI（StatusCard/ControlCard/ConfigCard/InfoGrid）
 *   - 所有用户交互通过嵌入式 WebView + JSInterface（@encvgo/components + plugin-openlist/web）
 *   - 与主 app 共享 Vue 组件（pnpm workspace）
 *
 * 状态共享（与 UI 无关）：
 *   - 主 app 通过 ContentProvider 读 OpenListStatusProvider
 *   - 插件 Content() 内 WebView 通过 OpenListPluginJSInterface 调 OpenListBridge
 */
class OpenListPluginEntry : IPluginEntryClass {

    private val tag = "OpenList-PluginEntry"

    /**
     * 仿 MpvPluginEntry: 不注册任何 Koin module
     * （Bridge 初始化在 onLoad 中完成，Service 由 OpenListPluginJSInterface 按需触发）
     */
    override val pluginModule: List<Module> = emptyList()

    /**
     * ComboLite 框架加载插件时调用。
     *
     * 修复：原来几乎为空，运行时启动委托给 OpenListService。
     * 现在：初始化 OpenListBridge + OpenListConfig（与 MpvPluginEntry 的 Content() 内
     * 初始化模式一致，只是提到 onLoad 阶段）。
     */
    override fun onLoad(context: PluginContext) {
        Log.e(tag, "[OpenList] onLoad() | thread=${Thread.currentThread().name}")
        try {
            // Phase 14 修复：PluginContext 是 data class(application: Application, pluginInfo)
            // 字段名是 application，不是 applicationContext（Android 习惯的 applicationContext
            // 在这里用 .application.context 也可，但 PluginContext 自身直接暴露 application）
            val appCtx: Context = context.application
            // 加载持久化配置
            val cfg = OpenListConfig.load(appCtx)
            cfg.applyToBridge(OpenListBridge)
            // 初始化 OpenListBridge（gomobile bind 初始化 + 资源提取）
            OpenListBridge.init(appCtx)
            Log.e(tag, "[OpenList] onLoad() OK | port=${cfg.port} dataDir=${cfg.dataDir}")
        } catch (t: Throwable) {
            Log.e(tag, "[OpenList] onLoad() FAILED", t)
        }
    }

    /**
     * ComboLite 框架卸载插件时调用。
     *
     * 修复：原来只 log，运行时继续在后台。
     * 现在：shutdown Bridge + Service（彻底清理）。
     */
    override fun onUnload() {
        Log.e(tag, "[OpenList] onUnload()")
        try {
            // 停止前台服务（如有运行）
            OpenListService.stopIfRunning()
            // shutdown gomobile runtime
            OpenListBridge.shutdown(5_000L)
            Log.e(tag, "[OpenList] onUnload() OK")
        } catch (t: Throwable) {
            Log.e(tag, "[OpenList] onUnload() FAILED", t)
        }
    }

    /**
     * 渲染插件主页面。
     *
     * 修复：原来是 400 行 Compose Material3 UI（StatusCard/ControlCard/ConfigCard/InfoGrid）。
     * 现在：嵌入式 Android WebView + JSInterface（详见 OpenListEmbedWebView），
     *      加载 plugin-openlist/web 产出的 Vite bundle。
     *
     * 占位 Box：作为 fallback，当宿主不支持 AndroidView 时显示最小提示。
     * 实际渲染：宿主调用 OpenListEmbedWebView 作为 Content() 的实现。
     */
    @Composable
    override fun Content() {
        OpenListEmbedWebView(
            containerId = "openlist-plugin-embed",
            initialUrl = "https://localhost/openlist/"
        )
    }
}

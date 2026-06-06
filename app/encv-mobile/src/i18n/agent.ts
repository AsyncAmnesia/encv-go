export default {
  'zh-CN': {
    'agent.title': 'AI 助手',
    'agent.fabLabel': '打开 AI 助手',
    'agent.model': '模型',
    'agent.temperature': '温度',
    'agent.placeholder': '输入消息，回车发送…',
    'agent.thinking': '正在思考',
    'agent.thinkingMeta': '正在推理…',
    'agent.thought': '已思考',
    'agent.reasoning': '推理',
    'agent.webSearch': '搜索',
    'agent.queries': '个查询',
    'agent.query': '个查询',
    'agent.running': '正在运行',
    'agent.completed': '已完成',
    'agent.failed': '失败',
    'agent.cancelled': '已取消',
    'agent.editing': '正在编辑',
    'agent.collapse': '收起',
    'agent.expand': '展开',
    'agent.copy': '复制',
    'agent.copied': '已复制',
    'agent.copyFailed': '复制失败',
    'agent.send': '发送',
    'agent.stop': '停止',
    'agent.resume': '恢复',
    'agent.newSession': '新会话',
    'agent.confirmReset': '确定清空当前会话并重新开始？',
    'agent.confirmNewSession': '开启新会话？当前会话会保留在历史中。',
    'agent.history': '会话历史',
    'agent.noHistory': '暂无历史会话',
    'agent.deleteSession': '删除会话',
    'agent.confirmDeleteSession': '确定永久删除该会话？此操作不可恢复。',
    'agent.messages': '条消息',
    'agent.errorTitle': '请求失败',
    'agent.retry': '重试',
    'agent.loadingModels': '加载模型列表',
    'agent.modelsError': '模型加载失败',
    'agent.noApiKeyHint': '未配置 API Key，请先填写上方 API Key 后保存',
    'agent.modelFallbackPlaceholder': '手动输入模型名称',
    'agent.noModelsAvailable': '无可用模型，可手动输入',
    'agent.apiKeyPlaceholder': 'sk-...',
    'agent.inputHint': 'Shift/⌘/Ctrl + Enter 发送，Enter 换行',
    'agent.emptyHint': '向 AI 助手提问、生成文件、或调用工具',

    // ── Task 12: 附件（Composer `+` 按钮） ────────────────
    'agent.attach': '添加附件',
    'agent.removeAttachment': '移除附件',
    'agent.attachmentCount': '已附加 {n} 个文件',
    'agent.imageAttachment': '图片附件',
    'agent.fileAttachment': '文件附件',

    // ── API Key 加密状态（状态反馈 UI） ─────────────────
    'agent.apiKeyStatusEmpty': '未配置',
    'agent.apiKeyStatusPlaintext': '明文',
    'agent.apiKeyStatusEncrypted': '已加密',
    'agent.apiKeyStatusDecrypting': '解密中…',
    'agent.apiKeyStatusEncrypting': '加密中…',
    'agent.apiKeyStatusDecryptFailed': '解密失败',
    'agent.apiKeyStatusDecryptFailedEmpty': '存储的 API Key 无法解密（所有格式尝试均失败）。加密密钥可能已轮换，或存储值已损坏。请重新输入 API Key 后保存。',
    'agent.apiKeyPlaceholderBroken': '已存储加密值但无法解密，请重新输入',
    'agent.apiKeyPlaceholderKeep': '已配置（输入新值将覆盖）',
    'agent.apiKeyMaskHint': '此处原本有加密的 API Key，但当前无法解密显示。请重新输入并保存以覆盖损坏的密文。',
    'agent.modelErrorNoApiKey': '未配置 API Key：模型列表需要有效的 OpenAI API Key 才能拉取。请在上方填写 API Key 后保存。',
    'agent.modelErrorDecryptFailed': 'API Key 已存储但无法解密：加密密钥可能已轮换或存储值已损坏。请在上方重新输入 API Key 后保存以覆盖。',
    'agent.modelErrorFixApiKey': '↑ 跳转到 API Key 设置',
    'agent.apiKeyStatusEncryptFailed': '加密失败',
    'agent.apiKeyStatusTestFailed': '连通性测试失败',
    'agent.apiKeyStatusRoundtripOk': '加解密往返一致',
    'agent.apiKeyStatusRoundtripMismatch': '往返解密结果不一致',
    'agent.apiKeyActionRoundtrip': '测试加解密',
    'agent.apiKeyBackendLabel': 'Agent API',
    'agent.apiKeyBackendDev': 'dev 网关',
    'agent.apiKeyBackendNative': '本地后端',
    'agent.apiKeyBackendUser': '用户配置',
    'agent.apiKeyBackendFallback': '兜底配置',
    'agent.apiKeyViewLogs': '查看日志',

    // ── AI 设置页错误态（后端离线 / 配置加载失败） ─────
    // 之前 v-if 三态全 false 时页面一片空白，用户完全看不到发生了什么。
    // 错误态必须给"是什么错 + 怎么修"两条信息，而不是只丢一个 spinner 卡死。
    'agent.backendOffline': '后端服务未连接',
    'agent.backendOfflineHint': '请确认 encv-go 服务已启动，或检查网络连接。',
    'agent.configLoadFailed': '加载 AI 配置失败',

    // ── API Key 加密失败时中止保存的提示 ─────
    // 关键：必须在 /api/encrypt-key 失败时立即中止 saveConfig，
    // 否则明文 API Key 会被写入磁盘，破坏加密存储设计。
    'agent.apiKeyEncryptFailedSaveAborted': 'API Key 加密失败，已中止保存（避免明文写入磁盘）',

    'modals.approve': '批准',
    'modals.approveForSession': '本轮批准',
    'modals.decline': '拒绝',
    'modals.cancel': '拒绝并停止',

    'agent.ops.commands': '已运行 {n} 条命令，{ms}ms',
    'agent.ops.files': '已编辑 {n} 个文件',
    'agent.ops.mixed': '已执行 {n} 个操作（{cmd} 命令 + {file} 文件变更）',
    'agent.ops.toolOutputs': '已执行 {n} 个工具',
    'agent.ops.commandsSummary': '{n} 条命令',
    'agent.ops.filesSummary': '{n} 个文件',
    'agent.ops.expandAll': '展开全部',
    'agent.ops.collapseAll': '收起全部',
    'agent.ops.showMore': '显示更多 ({n})',
    'agent.tool.command': '运行命令',
    'agent.tool.fileChange': '编辑文件',
    'agent.tool.readOnly': '读取信息',
    'agent.tool.webSearch': '联网搜索',
    'agent.tool.unknown': '调用工具',

    // ── Plan / Todo 块 ─────────────────────────────────
    'agent.plan': '计划',
    'agent.planEmpty': '（暂无计划）',
    'agent.planStatusPending': '待办',
    'agent.planStatusInProgress': '进行中',
    'agent.planStatusCompleted': '已完成',
    'agent.streaming': '加载中',

    // ── 活跃态细分文案（active 集合下区分显示用） ─────
    'agent.statusRunning': '正在运行',
    'agent.statusEditing': '正在编辑',
    'agent.statusThinking': '正在思考',

    // ── Task 7: 上下文自动压缩分隔线 ─────────────────
    // 后端在 messages token 数越过 80% 窗口时调用 LLM summary 压缩老消息，
    // 推送 EventCompaction 事件，前端渲染为不可展开的水平分隔线。
    'agent.contextCompaction': '上下文已自动压缩',

    // ── Task 10: Slash 命令菜单（"/" 触发） ─────────────────
    // 触发条件：textarea 内容以 "/" 开头时弹出，分组"功能" + "技能"。
    // 功能项固定 3 条（attach / plan-mode / permission-mode），
    // 技能项从后端 /api/skills 动态拉取。
    'agent.slashMenuTitle': 'Slash 命令',
    'agent.slashMenuFeatures': '功能',
    'agent.slashMenuSkills': '技能',
    'agent.slashMenuNoMatches': '无匹配项',
    'agent.slashMenuHint': '↑↓ 选择 · Enter 应用 · Esc 关闭',

    // ── Task 22: Agent Task Message（subagent 子任务列表） ─────────
    // 后端 SubagentDispatch 事件触发：AI 把复杂任务拆给多个 subagent
    // 并行 / 串行处理，前端把子任务列表渲染为可折叠块。折叠阈值与
    // codex-web MessageBlocks.tsx:68-69 对齐（7 行 / 520 字符）。
    'agent.agentTask': '子任务',
    'agent.subTaskProgress': '{done}/{total}',
    'agent.agentTaskEmpty': '（暂无子任务）',

    // ── Task 26: LAN Access（局域网访问地址面板） ─────────────────
    // 后端 /api/network/lan-access 枚举当前可被同 WiFi 设备访问的
    // URL，前端在 AgentChat 顶部折叠面板展示。面板用途：用户把
    // http://192.168.x.x:5245/ 输入手机/平板浏览器也能用 AI 助手。
    'agent.lanAccess': '网络访问地址',
    'agent.lanAccessHelp': '同 WiFi 下的设备可用此地址访问',
    'agent.lanAccessEmpty': '未发现可用的网络接口',
    'agent.lanAccessRefresh': '刷新',
    'agent.lanAccessCopy': '复制',
    'agent.lanAccessCopied': '已复制 {url}',
    'agent.lanAccessCopyFailed': '复制失败',
    'agent.lanAccessInterface': '接口：{name}',

    // ── Task 25: Sync Doctor（脱敏诊断按钮） ─────────────
    // 后端 /api/sync/doctor 返回的 DoctorReport 报告由用户在
    // Settings 面板中点击「运行 sync 诊断」拉取；展示原文 JSON
    // 给用户用于 bug 报告（报告中所有 API key/token/password
    // 已被后端 Redact，无需在前端再次脱敏）。
    'agent.syncDoctor': '运行 sync 诊断',
    'agent.syncDoctorRunning': '正在生成诊断报告…',
    'agent.syncDoctorResult': '诊断结果',
    'agent.syncDoctorCopy': '复制 JSON',
    'agent.syncDoctorCopied': '诊断 JSON 已复制',
    'agent.syncDoctorCopyFailed': '复制失败',
    'agent.syncDoctorFailed': '诊断失败：{msg}',
    'agent.syncDoctorEmpty': '未发现问题',
  },
  en: {
    'agent.title': 'AI Assistant',
    'agent.fabLabel': 'Open AI assistant',
    'agent.model': 'Model',
    'agent.temperature': 'Temp',
    'agent.placeholder': 'Type a message, press Enter to send…',
    'agent.thinking': 'Thinking',
    'agent.thinkingMeta': 'Thinking…',
    'agent.thought': 'Thought',
    'agent.reasoning': 'Reasoning',
    'agent.webSearch': 'Search',
    'agent.queries': 'queries',
    'agent.query': 'query',
    'agent.running': 'Running',
    'agent.completed': 'Completed',
    'agent.failed': 'Failed',
    'agent.cancelled': 'Cancelled',
    'agent.editing': 'Editing',
    'agent.collapse': 'Collapse',
    'agent.expand': 'Expand',
    'agent.copy': 'Copy',
    'agent.copied': 'Copied',
    'agent.copyFailed': 'Copy failed',
    'agent.send': 'Send',
    'agent.stop': 'Stop',
    'agent.resume': 'Resume',
    'agent.newSession': 'New session',
    'agent.confirmReset': 'Clear current session and start over?',
    'agent.confirmNewSession': 'Start a new session? The current one will be saved to history.',
    'agent.history': 'History',
    'agent.noHistory': 'No history yet',
    'agent.deleteSession': 'Delete session',
    'agent.confirmDeleteSession': 'Permanently delete this session? This cannot be undone.',
    'agent.messages': 'messages',
    'agent.errorTitle': 'Request failed',
    'agent.retry': 'Retry',
    'agent.loadingModels': 'Loading models',
    'agent.modelsError': 'Failed to load',
    'agent.noApiKeyHint': 'API Key not configured. Please enter it above and save.',
    'agent.modelFallbackPlaceholder': 'Enter model name manually',
    'agent.noModelsAvailable': 'No models available, type manually',
    'agent.apiKeyPlaceholder': 'sk-...',
    'agent.inputHint': 'Shift/⌘/Ctrl + Enter to send, Enter for newline',
    'agent.emptyHint': 'Ask the assistant, generate files, or invoke tools',

    // ── Task 12: attachments (Composer `+` button) ─────────────
    'agent.attach': 'Attach',
    'agent.removeAttachment': 'Remove attachment',
    'agent.attachmentCount': '{n} attachment(s) attached',
    'agent.imageAttachment': 'Image attachment',
    'agent.fileAttachment': 'File attachment',

    // ── API Key encryption status (status feedback UI) ───────────
    'agent.apiKeyStatusEmpty': 'Not set',
    'agent.apiKeyStatusPlaintext': 'Plaintext',
    'agent.apiKeyStatusEncrypted': 'Encrypted',
    'agent.apiKeyStatusDecrypting': 'Decrypting…',
    'agent.apiKeyStatusEncrypting': 'Encrypting…',
    'agent.apiKeyStatusDecryptFailed': 'Decrypt failed',
    'agent.apiKeyStatusDecryptFailedEmpty': 'Stored API key cannot be decrypted (all formats exhausted). The encryption key may have rotated or the stored value is corrupted. Please re-enter the API key and save.',
    'agent.apiKeyPlaceholderBroken': 'Stored encrypted value cannot be decrypted. Re-enter to overwrite.',
    'agent.apiKeyPlaceholderKeep': 'Already configured (typing a new value will overwrite).',
    'agent.apiKeyMaskHint': 'A previously-encrypted API Key is stored here but cannot be decrypted. Re-enter and save to overwrite the corrupted value.',
    'agent.modelErrorNoApiKey': 'API Key not configured: the model list requires a valid OpenAI API Key. Fill in the API Key above and save.',
    'agent.modelErrorDecryptFailed': 'Stored API Key cannot be decrypted: the encryption key may have rotated or the stored value is corrupted. Re-enter the API Key above and save to overwrite.',
    'agent.modelErrorFixApiKey': '↑ Jump to API Key setting',
    'agent.apiKeyStatusEncryptFailed': 'Encrypt failed',
    'agent.apiKeyStatusTestFailed': 'Connectivity test failed',
    'agent.apiKeyStatusRoundtripOk': 'Round-trip OK',
    'agent.apiKeyStatusRoundtripMismatch': 'Round-trip mismatch',
    'agent.apiKeyActionRoundtrip': 'Test encrypt/decrypt',
    'agent.apiKeyBackendLabel': 'Agent API',
    'agent.apiKeyBackendDev': 'dev gateway',
    'agent.apiKeyBackendNative': 'local backend',
    'agent.apiKeyBackendUser': 'user config',
    'agent.apiKeyBackendFallback': 'fallback config',
    'agent.apiKeyViewLogs': 'View logs',

    // ── AI settings error state (backend offline / config load failed) ─────
    // Previously when all 3 v-if conditions were false, the page was completely
    // blank. The error state must say "what went wrong + how to fix" — not
    // just an eternal spinner.
    'agent.backendOffline': 'Backend service not connected',
    'agent.backendOfflineHint': 'Please confirm encv-go is running, or check the network connection.',
    'agent.configLoadFailed': 'Failed to load AI configuration',

    // API Key encryption failure → abort save (avoid writing plaintext to disk)
    'agent.apiKeyEncryptFailedSaveAborted': 'API Key encryption failed, save aborted (to avoid writing plaintext to disk)',

    'modals.approve': 'Approve',
    'modals.approveForSession': 'Approve for session',
    'modals.decline': 'Decline',
    'modals.cancel': 'Decline & stop',

    'agent.ops.commands': 'Ran {n} command(s) in {ms}ms',
    'agent.ops.files': 'Edited {n} file(s)',
    'agent.ops.mixed': 'Performed {n} operation(s) ({cmd} commands + {file} file changes)',
    'agent.ops.toolOutputs': 'Ran {n} tool(s)',
    'agent.ops.commandsSummary': '{n} command(s)',
    'agent.ops.filesSummary': '{n} file(s)',
    'agent.ops.expandAll': 'Expand all',
    'agent.ops.collapseAll': 'Collapse all',
    'agent.ops.showMore': 'Show more ({n})',
    'agent.tool.command': 'Run command',
    'agent.tool.fileChange': 'Edit file',
    'agent.tool.readOnly': 'Read info',
    'agent.tool.webSearch': 'Web search',
    'agent.tool.unknown': 'Invoke tool',

    // ── Plan / Todo block ─────────────────────────────
    'agent.plan': 'Plan',
    'agent.planEmpty': '(no plan yet)',
    'agent.planStatusPending': 'Pending',
    'agent.planStatusInProgress': 'In progress',
    'agent.planStatusCompleted': 'Done',
    'agent.streaming': 'Loading',

    // ── Active-state sub-labels (used to differentiate within the active set) ─────
    'agent.statusRunning': 'Running',
    'agent.statusEditing': 'Editing',
    'agent.statusThinking': 'Thinking',

    // ── Task 7: context auto-compression divider ─────────────────
    // Backend triggers auto-compaction when the running messages
    // exceed 80% of the model context window. The front-end
    // renders a non-expandable horizontal divider at the position
    // the compacted messages used to occupy.
    'agent.contextCompaction': 'Context auto-compressed',

    // ── Task 10: Slash command menu ("/" trigger) ─────────────────
    // Trigger: textarea content starts with "/". Two groups: "Features"
    // (static, 3 items: attach / plan-mode / permission-mode) and
    // "Skills" (dynamic, fetched from backend /api/skills on mount).
    'agent.slashMenuTitle': 'Slash commands',
    'agent.slashMenuFeatures': 'Features',
    'agent.slashMenuSkills': 'Skills',
    'agent.slashMenuNoMatches': 'No matches',
    'agent.slashMenuHint': '↑↓ navigate · Enter apply · Esc close',

    // ── Task 22: Agent Task Message (subagent sub-task list) ─────────
    // Backend SubagentDispatch event: AI splits complex tasks across
    // multiple subagents (parallel/serial) and the front-end renders
    // the sub-task list as a foldable block. Collapse thresholds
    // align with codex-web MessageBlocks.tsx:68-69 (7 lines / 520 chars).
    'agent.agentTask': 'Sub-tasks',
    'agent.subTaskProgress': '{done}/{total}',
    'agent.agentTaskEmpty': '(no sub-tasks)',

    // ── Task 26: LAN Access (LAN access URL panel) ─────────────────
    // Backend /api/network/lan-access enumerates URLs reachable from
    // a peer on the same WiFi. The Settings panel surfaces them so
    // the user can type http://192.168.x.x:5245/ into a phone/tablet
    // browser on the same network and reach the AI assistant.
    'agent.lanAccess': 'LAN access',
    'agent.lanAccessHelp': 'Devices on the same WiFi can use these addresses',
    'agent.lanAccessEmpty': 'No usable network interface found',
    'agent.lanAccessRefresh': 'Refresh',
    'agent.lanAccessCopy': 'Copy',
    'agent.lanAccessCopied': 'Copied {url}',
    'agent.lanAccessCopyFailed': 'Copy failed',
    'agent.lanAccessInterface': 'Interface: {name}',

    // ── Task 25: Sync Doctor (redacted diagnostic) ─────────────
    // Triggered from the Settings panel; the report is shown
    // raw to the user for bug-reporting purposes (all secrets
    // have already been Redacted server-side).
    'agent.syncDoctor': 'Run sync doctor',
    'agent.syncDoctorRunning': 'Generating diagnostic report…',
    'agent.syncDoctorResult': 'Diagnostic result',
    'agent.syncDoctorCopy': 'Copy JSON',
    'agent.syncDoctorCopied': 'Diagnostic JSON copied',
    'agent.syncDoctorCopyFailed': 'Copy failed',
    'agent.syncDoctorFailed': 'Doctor failed: {msg}',
    'agent.syncDoctorEmpty': 'No issues detected',
  },
}

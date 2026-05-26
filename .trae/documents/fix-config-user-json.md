# 修复 config.user.json + 项目规则更新

## 问题根因

之前启动 dev 时为对齐 Vite proxy 端口（`:2025`），直接修改了 `config.user.json` 的端口字段（1999→2025），并大幅简化了配置内容，导致丢失了大量必要字段。

## 对比分析

| 字段 | GitHub main (原始) | 当前 (被篡改) | 状态 |
|------|-------------------|--------------|------|
| `$schema` | ✅ 有 | ❌ 缺失 | 需恢复 |
| `password` | 示例值 | `"test123"` | 需恢复示例值 |
| `output_path` | `"./output"` | ❌ 缺失 | 需恢复 |
| `server.port` | `2025` | `2025` | ✅ 一致 |
| `server.dir` | `"/"` | `"/"` | ✅ 一致 |
| **`admin.port`** | **`1808`** | ❌ 缺失 | **⚠️ 过时！AdminServer 已无 Port 字段** |
| `admin.password` | `"123456"` | ❌ 包含 | 需恢复 |
| `proxy.sites` | 3 个 (pc/vivo/pc_dev) | 1 个测试站 | 需恢复原始 |
| `log.level` | `"info"` | `"debug"` | 开发用 debug 可接受 |
| `log.file` | `"encv.log"` | ❌ 缺失 | 需恢复 |
| **`webdav.port`** | **`1234`** | ❌ 缺失 | **⚠️ 过时！WebdavServer 已无 Port 字段** |
| `webdav.root` | `"webdav"` | ❌ 缺失 | 需恢复 |
| `webdav.dir` | `"./"` | ❌ 缺失 | 需恢复 |
| `webdav.username/password` | 有 | ❌ 缺失 | 需恢复 |
| `plugin_settings` | 完整 6 插件 | ❌ 缺失 | 需恢复 |

### 过时配置识别

通过代码审查确认以下字段在当前类型定义中**已不存在**：
- ~~`admin.port`~~ → [`AdminServer`](internal/v2/types/types.go#L170) 只有 `Password` 字段
- ~~`webdav.port`~~ → [`WebdavServer`](internal/v2/types/types.go#L150) 无 `Port` 字段（已合并到主 HttpServer）
- ~~`plugin_settings.video.plugin_cache_dir: "D:/TEMP/encv-cache"~~ → Windows 绝对路径，跨平台不兼容

---

## 执行步骤

### Step 1: 恢复 config.user.json 为完整模板

以 GitHub main 版本为基础，做以下调整：

1. **保留 `$schema`**
2. **password 恢复为原始示例值**（含中文说明）
3. **删除过时字段**: `admin.port`(1808)、`webdav.port`(1234)
4. **删除平台特定路径**: `plugin_cache_dir: "D:/TEMP/encv-cache"` （改为空字符串或注释说明）
5. **保留所有其他字段**：output_path、admin.password、proxy.sites（完整 3 站点）、log.file、webdav（root/dir/username/password）、plugin_settings（全部 6 个插件）

最终文件应包含所有有效配置节点，作为**完整的用户配置模板**。

```json
{
  "$schema": "https://raw.githubusercontent.com/Soltus/encv-go/main/config.schema.json",
  "password": "my-encv_key，可以使用中文和标点符号✔",
  "output_path": "./output",
  "server": {
    "port": 2025,
    "dir": "/"
  },
  "admin": {
    "password": "123456"
  },
  "proxy": {
    "sites": {
      "pc": { "host": "http://localhost:5244", "description": "电脑上的openlist" },
      "vivo": { "host": "http://192.168.31.19:5244", "description": "手机上的openlist" },
      "pc_dev": { "host": "http://localhost:5234", "description": "电脑上定制版的openlist" }
    }
  },
  "log": {
    "level": "info",
    "file": "encv.log"
  },
  "webdav": {
    "root": "/webdav/",
    "dir": "./output",
    "username": "admin",
    "password": "123456"
  },
  "plugin_settings": {
    "video": {
      "ext": ".sccgv",
      "chunk_size_mb": 0,
      "light_main_chunk_enabled": true,
      "verify_after_pack": true,
      "track_extensions": ".ass,.srt,.dm.ass,.vtt",
      "skip_merge_for_split_mkv": false
    },
    "image": { "ext": ".sccgi" },
    "audio": { "ext": ".sccga" },
    "text": { "ext": ".sccgt" },
    "wps": { "ext": ".sccgwps" },
    "pdf": { "ext": ".sccgpdf" }
  }
}
```

### Step 2: 还原 DefaultConfig 端口回 1999

[`config.go:83`](internal/config/config.go#L83) 之前被改为 `2025`，应还原为 `1999`（默认端口和 config.user.json 的开发端口解耦）：

- `DefaultConfig()` 中 `Port: 1999` — 这是**默认值**，当没有 config 文件时使用
- `config.user.json` 中 `port: 2025` — 这是**开发者模板值**，有 config 文件时使用
- 两者独立，互不影响

### Step 3: 移动端适配策略

**不修改 config.user.json**（它是桌面端/通用模板）。移动端已有独立的 [`config.mobile.json`](app/encv-mobile/assets/config.mobile.json) 作为 Android assets 默认配置。

移动端配置加载链路：
```
APK assets/config.mobile.json  (默认模板, 只读)
        ↓ EncvGoService.ensureBuildInfoExists() 复制
filesDir/config.mobile.json   (运行时可写, Go 后端读取 HOME 环境变量)
```

当前 `config.mobile.json` 已包含移动端特有路径（`/storage/emulated/0`），结构合理，无需修改。

### Step 4: Mock 处理确认

| Mock 缺失项 | 处理方式 | 责任方 |
|-------------|----------|--------|
| `build-info.json` 不在 assets 中 | 构建脚本负责生成并打包 | CI/CD 构建流程（非代码层面） |
| `Preview()` 无 Android stub | ✅ 已创建 [`decrypt_preview_mobile.go`](internal/service/decrypt_preview_mobile.go) | 本次会话已完成 |
| `IdentifyWithMKVMerge()` 死代码 | ✅ 已标记 DEAD CODE 注释 | 本次会话已完成 |

### Step 5: 添加项目规则

在 [`.trae/rules/project_rules.md`](.trae/rules/project_rules.md) 末尾追加：

```markdown
## 配置模板保护（重要！）

- **严禁擅自修改 `config.user.json`**：该文件是用户配置模板，任何端口/路径/密码等值的修改必须通过用户明确指令执行
- 如需临时改变开发端口等参数，应使用环境变量 `ENCV_CONFIG_PATH` 指向临时配置文件，或命令行 `--config` 标志
- `config.mobile.json` 同理：是 Android 端的默认配置模板，不得擅自修改其结构或关键字段
- 违反此规则导致的配置模板破坏将被视为严重错误
```

---

## 验证

1. `config.user.json` 与 GitHub main 对比：保留所有有效字段，移除过时字段
2. `go run ./cmd/encv/ start` 使用恢复后的 config.user.json 能正常启动在 `:2025`
3. 前端 Vite proxy (`:2025`) → 后端 (`:2025`) 联调正常
4. `go test ./internal/...` 无回归

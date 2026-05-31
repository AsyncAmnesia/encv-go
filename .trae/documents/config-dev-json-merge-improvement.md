# 配置系统改进：`config.dev.json` + 合并覆盖机制

## 一、现状分析

### 当前问题
1. **无团队共享的 dev 配置模板**：开发者各自维护完整的 `config.user.json`，新成员接入成本高
2. **配置文件是全量副本**：`config.user.json` 包含所有字段（~76 行），任何变更需要同步两处
3. **无分层合并机制**：`FindConfigPath()` 只查找单一 `config.user.json`，不支持 base + override 模式

### 当前配置加载流程
```
FindConfigPath() → 只查找 config.user.json → Load() → json.Unmarshal 覆盖默认值
```

### 关键文件
| 文件 | 作用 |
|------|------|
| [config.go](internal/config/config.go) | 配置结构体定义、`Load()`、`FindConfigPath()`、`DefaultConfig()` |
| [config.user.json](config.user.json) | 当前开发者个人全量配置（含密码等敏感信息） |
| [.gitignore](.gitignore) | 未显式忽略 `config.dev.json`（但 `config.user.json` 也未忽略，存在风险） |

---

## 二、改进目标

1. **新增 `config.dev.json`**：提交到 Git，作为团队共享的开发基准配置（仅包含与默认值不同的开发环境必要字段）
2. **实现两层合并加载**：`config.base.json`(可选) / `config.dev.json` → `config.user.json`(个人覆盖)
3. **最小化 dev 配置体积**：dev 文件只保留差异化字段（预计 ~20 行 vs 当前 76 行）

---

## 三、详细实现步骤

### Step 1: 创建 `config.dev.json`（团队共享）

**文件路径**: `/workspace/config.dev.json`
**加入 Git**: 不加入 `.gitignore`（这是核心需求）
**内容原则**: 仅包含开发环境必要的**差异字段**，其余继承 `DefaultConfig()`

```json
{
  "$schema": "https://raw.githubusercontent.com/Soltus/encv-go/main/config.schema.json",
  "password": "dev-password",
  "log": {
    "level": "debug"
  },
  "server": {
    "port": 2025,
    "dir": "./"
  },
  "admin": {
    "password": "123456"
  },
  "proxy": {
    "sites": {
      "pc": { "host": "http://localhost:5244", "description": "本地 OpenList" }
    }
  },
  "plugin_settings": {
    "video": { "ext": ".sccgv" },
    "audio": { "ext": ".sccga" },
    "image": { "ext": ".sccgi" },
    "text": { "ext": ".sccgt" },
    "wps": { "ext": ".sccgwps" },
    "pdf": { "ext": ".sccgpdf" }
  }
}
```

**设计决策**：
- `output_path` 使用默认值 `"./encrypted"`，不覆盖
- `webdav` 使用默认空值（开发时可能不需要）
- `mobile` 段不在此文件中（纯后端开发不需要）
- `recover`、`strict_deprecated_version` 等使用 Go 默认零值

### Step 2: 更新 `.gitignore` — 保护敏感配置

**修改文件**: `/workspace/.gitignore`

在末尾添加：
```gitignore
# 用户个人配置（含真实密码/路径），不提交
config.user.json
```

**同时确认**：`config.dev.json` **不出现**在 `.gitignore` 中（满足需求 #1）

### Step 3: 实现配置合并加载逻辑

**修改文件**: `/workspace/internal/config/config.go`

#### 3.1 新增 `mergeConfig()` 函数

```go
// mergeConfig 将 overlay 中的非零值字段合并到 base 上
// 遵循 JSON merge patch 语义（RFC 7386 简化版）：
//   - nil/zero 值不覆盖（保留 base 的值）
//   - object 类型递归合并
//   - 数组/标量类型直接替换
func mergeConfig(base, overlay *Config) *Config {
    if overlay == nil {
        return base
    }

    overlayData, err := json.Marshal(overlay)
    if err != nil {
        return base
    }

    baseData, err := json.Marshal(base)
    if err != nil {
        return base
    }

    merged := make(map[string]interface{})
    if err := json.Unmarshal(baseData, &merged); err != nil {
        return base
    }

    var overlayMap map[string]interface{}
    if err := json.Unmarshal(overlayData, &overlayMap); err != nil {
        return base
    }

    deepMerge(merged, overlayMap)

    resultData, err := json.Marshal(merged)
    if err != nil {
        return base
    }

    var result Config
    if err := json.Unmarshal(resultData, &result); err != nil {
        return base
    }

    // 保持 Provider 引用（json:"-" 字段不会被序列化）
    result.Provider = base.Provider

    return &result
}

// deepMerge 递归合并两个 map，overlay 中的非 nil 值覆盖 base
func deepMerge(base, overlay map[string]interface{}) {
    for key, ov := range overlay {
        if ov == nil {
            continue // nil 值不覆盖（JSON merge patch 语义）
        }

        bv, exists := base[key]
        if !exists {
            base[key] = ov
            continue
        }

        bvMap, bIsMap := bv.(map[string]interface{})
        ovMap, oIsMap := ov.(map[string]interface{})

        if bIsMap && oIsMap {
            deepMerge(bvMap, ovMap) // 递归合并嵌套对象
        } else {
            base[key] = ov // 标量/数组直接替换
        }
    }
}
```

#### 3.2 新增 `loadWithMerge()` 函数

```go
// loadWithMerge 按优先级加载并合并多层配置文件
// 加载顺序（低→高优先级）：
//   1. DefaultConfig()                    — Go 代码硬编码默认值
//   2. config.dev.json（如果存在）         — 团队共享的开发基准
//   3. config.user.json（如果存在）       — 个人覆盖（最高优先级）
//
// 返回最终合并后的配置
func loadWithMerge(devPath, userPath string) (*Config, error) {
    cfg := DefaultConfig()

    // Layer 1: 合并 config.dev.json
    if devData, err := os.ReadFile(devPath); err == nil {
        var devCfg Config
        if err := json.Unmarshal(devData, &devCfg); err == nil {
            cfg = mergeConfig(cfg, &devCfg)
            slog.Info("Merged config.dev.json", "path", devPath)
        }
    }

    // Layer 2: 合并 config.user.json（个人覆盖）
    if userData, err := os.ReadFile(userPath); err == nil {
        var userCfg Config
        if err := json.Unmarshal(userData, &userCfg); err == nil {
            cfg = mergeConfig(cfg, &userCfg)
            slog.Info("Merged config.user.json (personal override)", "path", userPath)
        }
    }

    // 后处理（保持现有逻辑不变）
    if cfg.Server.Dir == "/" {
        if wd, err := os.Getwd(); err == nil {
            cfg.Server.Dir = wd
        }
    }

    if os.Getenv("ENCV_MOBILE") == "1" && cfg.Mobile != nil {
        ApplyMobileOverrides(cfg)
    }

    slog.Info("Configuration loaded with merge strategy",
        "dev_config", fileExists(devPath),
        "user_config", fileExists(userPath),
        "log_level", cfg.Log.Level)

    return cfg, nil
}

func fileExists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}
```

#### 3.3 修改 `FindConfigPath()` — 返回多个候选路径

```go
// ConfigCandidates 表示找到的配置文件候选列表
type ConfigCandidates struct {
    DevPath  string // config.dev.json 路径（可能为空）
    UserPath string // config.user.json 路径（可能为空）
}

// FindConfigPaths 查找所有可用的配置文件（dev + user）
// 替代原 FindConfigPath 的单文件查找逻辑
func FindConfigPaths(flagPath string) (*ConfigCandidates, error) {
    candidates := &ConfigCandidates{}

    // 确定搜索目录（从 flag/env/cwd/exe 中选）
    searchDirs := resolveSearchDirs(flagPath)

    for _, dir := range searchDirs {
        // 同时查找 dev 和 user 配置
        devPath := filepath.Join(dir, "config.dev.json")
        userPath := filepath.Join(dir, "config.user.json")

        if candidates.UserPath == "" && fileExists(userPath) {
            candidates.UserPath = userPath
        }
        if candidates.DevPath == "" && fileExists(devPath) {
            candidates.DevPath = devPath
        }

        // 如果都找到了，提前退出
        if candidates.DevPath != "" && candidates.UserPath != "" {
            break
        }
    }

    // 至少要找到一个配置源
    if candidates.DevPath == "" && candidates.UserPath == "" {
        return nil, fmt.Errorf("no config file found (tried config.dev.json and config.user.json)")
    }

    return candidates, nil
}

// resolveSearchDirs 解析配置文件搜索目录列表（复用原有逻辑）
func resolveSearchDirs(flagPath string) []string {
    var dirs []string

    // 1. 命令行标志
    if flagPath != "" {
        dirs = append(dirs, filepath.Dir(flagPath))
        return dirs
    }

    // 2. 环境变量
    if envPath := os.Getenv("ENCV_CONFIG_PATH"); envPath != "" {
        dirs = append(dirs, filepath.Dir(envPath))
        return dirs
    }

    // 3. 当前工作目录
    if wd, err := os.Getwd(); err == nil {
        dirs = append(dirs, wd)
    }

    // 4. 可执行文件目录
    if exePath, err := os.Executable(); err == nil {
        dirs = append(dirs, filepath.Dir(exePath))
    }

    return dirs
}
```

#### 3.4 修改 `Load()` 函数入口

保持向后兼容，`Load(configPath)` 仍然支持单文件模式（用于生产环境）：

```go
// Load 从指定的文件路径加载配置（向后兼容的单文件模式）
// 如果传入空字符串，自动走新的多文件合并流程
func Load(configPath string) (*Config, error) {
    // 向后兼容：显式指定了路径 → 单文件模式（生产环境）
    if configPath != "" {
        return loadSingleFile(configPath)
    }

    // 新流程：自动查找并合并多层配置
    candidates, err := FindConfigPaths("")
    if err != nil {
        slog.Warn("No config files found, using defaults")
        cfg := DefaultConfig()
        if os.Getenv("ENCV_MOBILE") == "1" && cfg.Mobile != nil {
            ApplyMobileOverrides(cfg)
        }
        return cfg, nil
    }

    return loadWithMerge(candidates.DevPath, candidates.UserPath)
}

// loadSingleFile 原有的单文件加载逻辑（生产环境使用）
func loadSingleFile(configPath string) (*Config, error) {
    cfg := DefaultConfig()

    data, err := os.ReadFile(configPath)
    if err != nil {
        if os.IsNotExist(err) {
            return cfg, nil
        }
        return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
    }

    if err := json.Unmarshal(data, cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config file '%s': %w", configPath, err)
    }

    if cfg.Server.Dir == "/" {
        cfg.Server.Dir, _ = os.Getwd()
    }

    if os.Getenv("ENCV_MOBILE") == "1" && cfg.Mobile != nil {
        ApplyMobileOverrides(cfg)
    }

    return cfg, nil
}
```

### Step 4: 更新调用方

**需检查的调用点**（通过 grep 确认）：

```bash
grep -r "config\.Load\|config\.FindConfigPath\|FindConfigPath" --include="*.go"
```

预期修改点：
- `cmd/encv/start.go` 或类似入口文件中的 `config.Load("")` 调用
- 无需改动调用签名（`Load("")` 自动走新流程）

### Step 5: 更新移动端 assets 中的配置

**文件**: `/workspace/app/encv-mobile/android/app/src/main/assets/config.user.json`

当前此文件是压缩格式的完整配置。考虑是否也需要支持 dev 合并：
- 移动端场景特殊（assets 打包），暂维持现状
- 或将 `config.dev.json` 也打入 assets 作为 baseline

**建议**：移动端暂不变，后续单独评估。

---

## 四、合并策略语义表

| overlay 值 | base 值 | 结果 | 说明 |
|-----------|---------|------|------|
| `{ "port": 3000 }` | `{ "port": 1999 }` | `3000` | 标量替换 |
| `{ "level": "debug" }` | `{ "level": "info" }` | `"debug"` | 字符串替换 |
| `{ "sites": { "pc": {...} } }` | `{ "sites": {} }` | `{ "sites": { "pc": {...} } }` | 对象递归合并 |
| `{}` (空对象) | `{ "dir": "/tmp" }` | `{ "dir": "/tmp" }` | 空对象不删除已有 key |
| `null` | `{ "port": 1999 }` | `{ "port": 1999 }` | null 不覆盖（RFC 7386） |

---

## 五、文件变更清单

| 操作 | 文件 | 说明 |
|------|------|------|
| **新建** | `config.dev.json` | 团队共享 dev 配置（~20 行，仅差异字段） |
| **修改** | `.gitignore` | 添加 `config.user.json` 忽略规则 |
| **修改** | `internal/config/config.go` | 新增 `mergeConfig()`、`deepMerge()`、`loadWithMerge()`、`FindConfigPaths()`；重构 `Load()` 入口 |

---

## 六、验证清单

- [ ] `config.dev.json` 可被 `git status` 显示为 untracked/new（未被忽略）
- [ ] `config.user.json` 被 `.gitignore` 忽略
- [ ] 无 `config.dev.json` 时行为与原来完全一致（向后兼容）
- [ ] 只有 `config.dev.json` 无 `config.user.json` 时正确加载 dev 配置
- [ ] 两者都存在时 `config.user.json` 的同名字段覆盖 dev
- [ ] `config.dev.json` 中省略的字段使用 `DefaultConfig()` 默认值
- [ ] 生产环境显式指定 `Load("/etc/encv/config.json")` 走单文件模式不受影响
- [ ] `ApplyMobileOverrides()` 在合并之后仍正确执行
- [ ] `Provider` 字段（`json:"-"`）在合并过程中不丢失
- [ ] `go build ./...` 编译通过
- [ ] `go test ./internal/config/...` 测试通过（如有）

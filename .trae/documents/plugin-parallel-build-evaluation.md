# 扩展插件与主应用并行构建评估

## 一、当前流水线依赖分析

### 1.1 步骤依赖图（硬约束）

```
阶段 A: 环境准备 (共享基础, 所有步骤串行)
  checkout → setup(node/go/java/android/NDK) → cache restore → install platform&tools
                                    ↓ 约 60s
阶段 B: 独立构建物 (彼此无依赖, 但当前串行执行)
  ├── frontend npm build         (~30s)
  ├── lynx bundle build           (~20s)
  ├── [mpv lib download]          (~10s) ← 从 Release 下载, 很快
  ├── ffmpeg build                (缓存命中~5s / 冷启动~300s+)
  └── Go binary build             (~30s)
                                    ↓ 全部完成才能进入下一阶段
阶段 C: 主应用准备 (依赖 B 的所有输出)
  keystore → cap copy(debug) → copy Go binary → verify libs
                                    ↓ 约 20s
阶段 D: 插件构建 (依赖 B 中对应插件的 native libs)
  └── [download libs] → ndk-build libplayer.so → gradle compile → aar2apk convert
        (~10s)        (~30s)            (~30s)           (~20s)
                                    ↓ 插件 APK 必须在此刻就绪
阶段 E: 最终打包 (硬依赖: 插件 APK ∈ host assets)
  └── build host APK (cap build 165s / assembleDebug 60s) → verify → upload
```

### 1.2 关键硬约束

**插件 APK 必须在主应用构建之前存在于 `app/src/main/assets/plugins/` 目录中。**

原因：
- `npx cap copy`（debug）或 `npx cap build`（release）会将 `app/src/main/assets/` 同步到 Android 项目
- Gradle `assembleDebug`/`assembleRelease` 打包时将 assets 编入 APK
- ComboLite 运行时从 host APK 的 assets 中读取插件 APK 进行安装

这意味着：**无论怎么并行，插件构建必须在主应用构建开始前完成。**

---

## 二、并行化方案评估

### 方案 A: 多 Job 并行（plugins job ⊥ host job）

```
Job 1: prepare-host              Job 2: build-plugins (matrix)
  checkout                         checkout (重复!)
  setup env (~45s)                 setup env (~45s) ← 重复开销!
  frontend build                   download each plugin's native libs
  ffmpeg build                     ndk-build each plugin
  go build                         gradle compile each plugin
  keystore                         aar2apk convert each plugin
  cap copy                          upload plugin APKs as artifacts
  copy Go binary                       ↓
                                     Job 3: assemble (needs: [job1, job2])
                                       download plugin artifacts
                                       copy to app/src/main/assets/plugins/
                                       build host APK
                                       verify + upload
```

| 维度 | 评估 |
|------|------|
| **插件数=1 时耗时** | **增加** ~45s（job2 的环境准备开销 > 节省的 0s 并行收益） |
| **插件数=2 时耗时** | 持平或略增（两个轻量插件的总时间 < 2×环境开销） |
| **插件数=3+ 时耗时** | 开始有收益，节省 ≈ max(插件时间) × (N-1) - 2×环境开销 |
| **额外成本** | 每个 job 重复 checkout + SDK/NDK 安装 + 缓存恢复；artifact 上传/下载传输时间 |
| **复杂度** | 高：`needs` 依赖、artifact 命名、错误传播、条件触发 |
| **CI 配额消耗** | 每个 job 消耗 1 个 runner 并发槽位（免费版限制 20 并行） |
| **调试体验** | 差：跨 job 排查需要跳转多个日志页面 |

### 方案 B: 单 Job 内 Matrix（不可行）

GitHub Actions 的 `strategy.matrix` 只能用于 **job 级别**，不能在单个 job 的 steps 内并行。❌ 不适用。

### 方案 C: 可复用 Workflow（workflow_call）

创建 `.github/workflows/build-plugin.yml` 作为可复用 workflow：

```yaml
# 主 workflow 中调用
jobs:
  build-plugins:
    strategy:
      matrix:
        plugin: [mpv-player, future-plugin-a, future-plugin-b]
    uses: ./.github/workflows/build-plugin.yml
    with:
      plugin: ${{ matrix.plugin }}
      build_type: ${{ inputs.version && 'release' || 'debug' }}

  build-host:
    needs: [build-plugins]
    # ... 构建主应用, 下载插件 artifacts ...
```

本质上是方案 A 的语法糖。同样存在环境重复开销问题。

### 方案 D: 保持单 Job 串行 + 结构优化（推荐 ✅）

不拆分 job，但优化内部结构使未来迁移成本低：

- 将每个插件的构建逻辑封装为**独立的可组合步骤块**
- 使用统一的变量约定（`PLUGIN_NAME`, `PLUGIN_MODULE`, `PLUGIN_NATIVE_LIBS_URL`）
- 当插件数量增长到 3+ 时，可以低成本迁移到多 job

---

## 三、量化对比（基于实际 CI 数据）

### 实测耗时参考值

| 步骤 | 耗时 (缓存命中) | 耗时 (冷启动) |
|------|----------------|---------------|
| 环境准备 (checkout + setup + caches) | ~60s | ~120s |
| Frontend npm build | ~30s | ~30s |
| Lynx bundle build | ~20s | ~20s |
| MPV lib download (from Release) | ~10s | ~10s |
| FFmpeg build | ~5s | ~300s+ |
| Go binary build | ~30s | ~30s |
| Keystore + cap copy + prep | ~20s | ~20s |
| **MPV 插件完整流程** (dl + ndk-build + compile + aar2apk) | **~90s** | **~90s** |
| **Host APK 构建** (release cap build) | **165s** | **165s** |
| Host APK 构建 (debug assembleDebug) | ~60s | ~60s |
| Verify + upload | ~20s | ~20s |

### 各方案总耗时估算

| 场景 | 当前串行 | 方案 A (多 Job) | 方案 D (优化串行) |
|------|---------|-----------------|------------------|
| **1 插件 + Debug** | ~350s | ~400s (+50s 开销) | ~350s |
| **1 插件 + Release** | ~450s | ~500s (+50s 开销) | ~450s |
| **2 插件 + Debug** | ~440s | ~420s (-20s) | ~440s |
| **3 插件 + Debug** | ~530s | ~440s (-90s) | ~530s |
| **skip_plugin + Debug** | ~260s | ~260s | ~260s |

### 结论

| 插件数量 | 是否值得并行？ | 说明 |
|----------|---------------|------|
| 1 个 (当前) | ❌ 不值得 | 多 job 开销 > 收益 |
| 2 个 | ⚠️ 边际 | 节省约 20s，但增加大量复杂度 |
| 3 个 | ✅ 开始值得 | 节省约 90s，可覆盖开销 |
| 4+ 个 | ✅ 明显值得 | 线性节省 |

---

## 四、推荐的渐进式策略

### 阶段一（当前）：保持单 Job + 结构规范化

不改架构，只做结构优化：
1. 将插件构建步骤参数化（用变量代替硬编码的 `plugin-mpv-player`）
2. 统一插件构建为"下载 native libs → ndk-build → gradle compile → aar2apk"四步模式
3. 未来新增插件只需复制这四步并改变量名

### 阶段二（当插件 ≥ 3 个时）：迁移到多 Job

当确实有 3+ 个插件时：
1. 创建 `build-all-plugins` job（matrix 策略）
2. 创建 `build-host-app` job（`needs: build-all-plugins`）
3. 利用已有的 `build-mpv-lib.yml` 模式（native libs 从独立 Release 获取）避免每个 job 都装 NDK

### 阶段三（可选）：Plugin Registry

如果插件数量继续增长（5+），考虑：
- 插件注册表文件（如 `plugins.json` 或 `plugins.gradle`）
- CI 自动发现并 matrix 构建
- 与 ComboLite 的 `aar2apk { modules {} }` 配置对齐

---

## 五、最终建议

**当前不需要并行化。** 理由：

1. 只有 1 个插件，并行反而更慢（环境重复开销 ~45s）
2. 真正的瓶颈是 **cap build release (165s)**，不是插件构建 (~90s)
3. `skip_plugin` 已解决"只构建主应用"的场景需求
4. 多 job 架构的维护成本（跨 job artifact 传递、错误排查、条件逻辑）目前得不偿失

**应做的**：将现有插件步骤参数化为可复制模板，降低未来迁移成本。

# MpvControls.kt CI 修复 + 本地构建（Sillot-KMP 镜像方案）

## 一、Sillot-KMP 镜像配置

来源：[Hi-Sillot/Sillot-KMP](https://github.com/Hi-Sillot/Sillot-KMP) `settings.gradle.kts`（master 分支）

### 1.1 pluginManagement.repositories 镜像列表
```kotlin
maven { url = uri("https://jitpack.io") }
maven { url = uri("https://maven.aliyun.com/repository/releases") }
maven { url = uri("https://maven.aliyun.com/repository/google") }     // ← Google Maven 镜像
maven { url = uri("https://maven.aliyun.com/repository/central") }    // ← Central 镜像
maven { url = uri("https://maven.aliyun.com/repository/gradle-plugin") } // ← Gradle Plugin 镜像
maven { url = uri("https://maven.aliyun.com/repository/public") }
google()
gradlePluginPortal()
mavenCentral()
mavenLocal()
maven {
    url = uri("https://mirrors.tencent.com/nexus/repository/maven-tencent/")
}
maven {
    url = uri("https://mirrors.tencent.com/repository/maven-tencent/")
}
```

### 1.2 dependencyResolutionManagement.repositories 镜像列表
```kotlin
maven { url = uri("https://maven.aliyun.com/repository/google") }
// 非 CI 环境额外使用腾讯镜像
if (System.getenv("CI") == null) {
    maven { url = uri("https://mirrors.tencent.com/nexus/repository/maven-public/") }
}
maven { url = uri("https://mirrors.tencent.com/repository/maven-tencent/") }
maven { url = uri("https://maven.cnb.cool/tencent-tds/shiply-public/-/packages/") }
maven { url = uri("https://maven.aliyun.com/repository/public") }
google()
mavenCentral()
gradlePluginPortal()
mavenLocal()
```

## 二、JDK 环境

| 项目 | 结果 |
|------|------|
| Gradle 8.14.3 + JDK 25 | ❌ 不兼容 |
| Gradle 8.14.3 + JDK 21 | ✅ 已安装 (`mise install java@temurin-21`) |

构建命令：
```bash
export JAVA_HOME=/root/.local/share/mise/installs/java/21.0.2
cd /workspace/app/encv-mobile/android
$JAVA_HOME/bin/java -version && gradle :plugin-mpv-player:compileReleaseKotlin --no-daemon
```

## 三、CI 错误修复（3 个错误，5 处改动）

### 权威依据（三方交叉验证）
1. Android 官方文档 developer.android.com/develop/ui/compose/state — `by` 委托需 `getValue` + `setValue`
2. MpvPlayerScreen.kt（本项目 CI 通过）— L53 `var volume by remember { mutableStateOf(1f) }` 有 `setValue` import
3. CI 日志 L176-178 — 精确错误位置和消息

### 修改文件：[MpvControls.kt](app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvControls.kt)

| # | 行号 | 操作 |
|---|------|------|
| 1 | L32 | import `...icons.outlined.Subtitles` → `...icons.Icons.Outlined.Subtitles` |
| 2 | L33 | import `...icons.outlined.MusicNote` → `...icons.Icons.Outlined.MusicNote` |
| 3 | L44 后 | **新增** `import androidx.compose.runtime.setValue` |
| 4 | L266 | 引用 `Icons.outlined.Subtitles` → `Icons.Outlined.Subtitles` |
| 5 | L273 | 引用 `Icons.outlined.MusicNote` → `Icons.Outlined.MusicNote` |

## 四、执行顺序

1. **配置镜像**：修改 [settings.gradle.kts](app/encv-mobile/android/settings.gradle.kts)，在 `pluginManagement` 和 `dependencyResolutionManagement` 的 repositories 中插入阿里云+腾讯镜像
2. **运行本地编译**：JDK 21 + Gradle 编译 `plugin-mpv-player`
3. **修复 MpvControls.kt**：上述 5 处改动
4. **再次编译验证**
5. **清理日志文件**

## 五、清理

完成后删除：
- `/workspace/job_logs_extracted/`
- `/workspace/job_logs.zip`
- `/tmp/androidx/`

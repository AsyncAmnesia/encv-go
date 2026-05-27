# 修复 MpvControls.kt import 路径错误

## 问题

CI 构建失败于 `:plugin-mpv-player:compileReleaseKotlin`：

```
e: MpvControls.kt:32: Syntax error: Expecting a top level declaration.
e: MpvControls.kt:33: Syntax error: imports are only allowed in the beginning of file.
```

**根因**：L32-L33 的 import 路径使用了大写 `Outlined`，但 Compose Material Icons 包名是小写 `outlined`。

| 错误（当前） | 正确 |
|-------------|------|
| `androidx.compose.material.icons.Outlined.Subtitles` | `androidx.compose.material.icons.outlined.Subtitles` |
| `androidx.compose.material.icons.Outlined.Audiotrack` | `androidx.compose.material.icons.outlined.Audiotrack` |

同时 L262、L269 的引用 `Icons.Outlined.Subtitles` / `Icons.Outlined.Audiotrack` 也需改为 `Icons.outlined.Subtitles` / `Icons.outlined.Audiotrack`。

## 修复

**文件**: `app/encv-mobile/plugin-mpv-player/src/main/java/com/encvgo/plugin/mpv/MpvControls.kt`

1. L32: `Icons.Outlined` → `Icons.outlined`
2. L33: `Icons.Outlined` → `Icons.outlined`
3. L262: `Icons.Outlined` → `Icons.outlined`
4. L269: `Icons.Outlined` → `Icons.outlined`

## 验证

修复后无需本地验证（纯字符串替换），CI 应能通过 Kotlin 编译。

## 清理

- 删除 `job_logs_extracted/`
- 删除 `job_logs.zip`

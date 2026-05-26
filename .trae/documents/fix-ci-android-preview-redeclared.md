# 修复 CI Android 构建失败：Preview 函数重复声明

## 问题根因分析

### 错误信息
```
Error: internal/service/decrypt_preview_mobile.go:10:6: Preview redeclared in this block
Error:  internal/service/decrypt_preview.go:40:6: other declaration of Preview
```

### 为什么本地验证通过但 CI 失败

| 环境 | 编译命令 | GOOS | 包含的文件 | 结果 |
|------|----------|------|------------|------|
| **本地沙箱** | `go build ./internal/...` | `linux` (默认) | `decrypt_preview.go` ✅<br>`decrypt_preview_mobile.go` ❌ (被 android tag 排除) | ✅ 通过 |
| **CI Android 构建** | `go build ./cmd/encv` | `android` (显式设置) | `decrypt_preview.go` ✅ (无约束，被包含)<br>`decrypt_preview_mobile.go` ✅ (android tag 匹配) | ❌ **重复声明** |

**关键点**：CI 的 [android.yml 第 131-143 行](file:///workspace/.github/workflows/android.yml#L131-L143) 设置了：
```yaml
env:
  GOOS: android
  GOARCH: arm64
```

当 `GOOS=android` 时：
- `decrypt_preview_mobile.go`（有 `//go:build android`）→ **被包含**
- `decrypt_preview.go`（**无 build tag 约束**）→ **也被包含**

两个文件在同一 package 中都声明了 `func Preview()` → 编译错误。

## 修复方案

### 需要修改的文件
- `/workspace/internal/service/decrypt_preview.go`

### 具体修改

在文件顶部添加 build tag 排除 android 平台：

```go
//go:build !android

// internal/service/decrypt_preview.go
// 预览功能：处理通过HTTP流和mpv播放器来预览加密文件的逻辑。

package service

// ... 其余代码不变
```

### 修改说明

| 文件 | 修改前 | 修改后 |
|------|--------|--------|
| `decrypt_preview.go` | 无 build tag（所有平台编译） | `//go:build !android`（仅非 android 平台编译） |
| `decrypt_preview_mobile.go` | `//go:build android`（仅 android 平台编译） | 不变 |

修改后的效果：

| 平台 | decrypt_preview.go | decrypt_preview_mobile.go |
|------|---------------------|---------------------------|
| Linux/Windows/macOS | ✅ 包含 | ❌ 排除 |
| Android (GOOS=android) | ❌ 排除 | ✅ 包含 |

两个文件互斥，不再冲突。

## 验证步骤

1. **模拟 Android 交叉编译验证**：
   ```bash
   CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build ./internal/service/
   ```

2. **桌面端编译验证**：
   ```bash
   go build ./internal/service/
   ```

3. **确认无其他类似问题**：
   检查项目中是否还有其他缺少平台约束的 stub 文件对。

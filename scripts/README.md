# scripts/

辅助构建 / 运维脚本。所有脚本应可在仓库根目录执行（路径均相对仓库根）。

## build-openlist-aar.sh / build-openlist-aar.ps1

从 [Hi-Sillot/OpenList](https://github.com/Hi-Sillot/OpenList) fork 编译出
`openlist.aar`（gomobile bind 产物），供
[plugin-openlist](../app/encv-mobile/plugin-openlist/) ComboLite 插件
`libs/openlist.aar` 使用。

- Linux / macOS: `scripts/build-openlist-aar.sh`
- Windows (CI): `scripts/build-openlist-aar.ps1`

### 环境要求

| 工具 | 版本 | 备注 |
|------|------|------|
| Go | 1.25.x | 与 Hi-Sillot fork `go.mod` 一致 |
| Android NDK | r25c+ | 推荐 r26b / 26.3.11579264 |
| Java | 17 | Temurin / OpenJDK |
| cmake | 系统包 | NDK 工具链依赖 |
| git / curl / tar / sha256sum | 系统包 | 拉取源码与 frontend dist |

### 快速开始

```bash
# 默认参数（fork=Hi-Sillot/OpenList、branch=main、ndk=$ANDROID_HOME/ndk/26.3.11579264、encv-go-root=/workspace）
scripts/build-openlist-aar.sh \
    --output /workspace/app/encv-mobile/plugin-openlist/libs
```

或 Windows：

```powershell
pwsh -File scripts/build-openlist-aar.ps1 `
    -Output  C:\workspace\app\encv-mobile\plugin-openlist\libs `
    -EncvGoRoot C:\workspace
```

### 入参

| 参数 | 必填 | 默认 | 说明 |
|------|------|------|------|
| `--output` / `-Output` | 是 | — | `openlist.aar` 输出目录（目录，脚本会生成 `openlist.aar` + `openlist.aar.sha256`） |
| `--fork` / `-Fork` | 否 | `https://github.com/Hi-Sillot/OpenList` | fork 仓库 URL |
| `--branch` / `-Branch` | 否 | `main` | 分支或 tag |
| `--ndk` / `-Ndk` | 否 | `$ANDROID_HOME/ndk/26.3.11579264` | NDK 安装路径 |
| `--encv-go-root` / `-EncvGoRoot` | 否 | `/workspace`（Linux）/ `C:\workspace`（Windows） | 本地 encv-go 仓库路径（用于修复 `replace github.com/Soltus/encv-go => ../../../`） |

### 工作流程

1. 解析入参，校验 go / java / git / curl / tar / cmake / ndk-build
2. `$TMPDIR/openlist-aar-build/openlist/` 准备临时工作区，删除旧副本
3. `git clone --depth 1 --branch $BRANCH $FORK`
4. `sed` 修复 `go.mod` 中 `replace github.com/Soltus/encv-go => ../../../` 指向 `--encv-go-root`
5. 拉取 OpenList-Frontend 最新 release tar 解压到 `openlist/public/dist/`
6. 设置 `ANDROID_NDK_HOME`，`go install gomobile/gobind@latest` + `gomobile init -ndk $NDK`
7. `gomobile bind -ldflags "..." -androidapi 19 -target="android/arm64" -o $OUTPUT/openlist.aar ./openlistlib`
8. 生成 `openlist.aar.sha256`

### 故障排查

| 症状 | 根因 | 修复 |
|------|------|------|
| `Hi-Sillot fork is missing openlistlib/` | fork 还没补 `openlistlib/{server,settings,common,event}.go` 入口 | 参见 spec §一 |
| `replace github.com/Soltus/encv-go => ../../../` 解析失败 | sed 未生效 | 检查 `--encv-go-root` 是否为绝对路径 |
| `frontend dist extraction failed` | GitHub API 限流 | 重跑脚本（脚本本身有缓存逻辑之外的简单重试） |
| `gomobile init` 失败 | NDK 路径错 | `--ndk` 传 NDK 安装根目录（含 `ndk-build`） |

### 相关文档

- Spec: [integrate-openlist-as-combolite-plugin](../.trae/specs/integrate-openlist-as-combolite-plugin/spec.md)
- Reference fork: [Hi-Sillot/OpenList](https://github.com/Hi-Sillot/OpenList)
- 参考实现脚本（K-Sillot 仓库，仅参考）：
  - `init_openlist.sh` / `init_web.sh` / `init_gomobile.sh` / `gobind.sh`

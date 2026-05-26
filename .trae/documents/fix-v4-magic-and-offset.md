# V4 容器识别失败 + Magic Header 错误 — 根因修正

## Why

上次修复**不完整**，V4 容器仍然无法被识别。同时 Magic Header 使用 "ENVC" 而非项目名 "ENCV"，属于根本性错误。

---

## 🔴 Bug 1 (CRITICAL): Close() 中 V4 ManifestOffset 偏移计算错误

### 位置
[single_file_container_writer.go:251-256](file:///workspace/internal/v2/writer/single_file_container_writer.go#L251-L256)

### 问题

```go
// Close() L251-252 — 对所有版本统一计算
manifestBlockSize := block.GetBlockHeader_v2_Size() + int64(len(w.manifestBytes))  // ❌ V4 不该加 block header!
manifestBlockStart := fileInfo.Size() - manifestBlockSize                      // ❌ 比实际位置早 12 字节

// L254-257 V4 分支
if w.headerVersion == 4 {
    w.v4Header.ManifestOffset = uint64(manifestBlockStart)  // ← 写入了错误偏移！
}
```

**V4 manifest 的实际情况**:
- `writeManifestV4()` 直接写混淆数据到文件（**无 Block header 包裹**）
- 文件布局: `[Header 2048B] [Segments...] [Manifest(混淆) 12B offset] [Footer 12B]`
- 实际 manifest 起始位置 = `fileSize - len(manifestBytes)`
- 但代码计算出的位置 = `fileSize - (12 + len(manifestBytes))` → **多了 12 字节的 block header**

**后果链**:
1. Header 中写入错误的 `ManifestOffset`（偏小 12）
2. Reader 从偏小的位置开始读 12 字节**前导垃圾**（实际上是最后一个 Segment 的尾部）
3. `DeobfuscateManifest(垃圾数据)` → XOR 解出乱码
4. `DeserializeManifest_v4(乱码)` → JSON 解析失败
5. Fallback `tryReadManifestAsLegacyV4` → 从错误位置读 BlockHeader → 也失败
6. `openV4()` 返回 error → `DetectContainer` 失败 → `isEncrypted=false`
7. 移动端显示"加密"而非"解密"

### 修复

```go
func (w *SingleFileContainerWriter) Close() error {
    // ...
    var manifestBlockStart int64
    
    if w.headerVersion == 4 {
        // V4: manifest 无 Block header 包裹，直接计算
        manifestBlockStart = fileInfo.Size() - int64(len(w.manifestBytes))
    } else {
        // V2/V3: manifest 有 Block header 包裹
        manifestBlockSize := block.GetBlockHeader_v2_Size() + int64(len(w.manifestBytes))
        manifestBlockStart = fileInfo.Size() - manifestBlockSize
    }
    
    if w.headerVersion == 4 {
        w.v4Header.ManifestOffset = uint64(manifestBlockStart)
        // ...
    }
    // ...
}
```

---

## 🔴 Bug 2 (CRITICAL): Magic Header "ENVC" 应为 "ENCV"

### 位置
[types/container.go:49-50](file:///workspace/internal/v2/types/container.go#L49-L50)

```go
MagicHeader_v2 = [4]byte{'E', 'N', 'V', 'C'}  // ❌ C/V 颠倒
MagicFooter_v2 = [4]byte{'E', 'N', 'V', 'C'}  // ❌ 同上
```

### 问题

| | 正确 | 当前 |
|---|---|---|
| 项目名 | **ENCV** (Encrypted Container) | encv-go |
| Magic bytes | **ENCV** | **ENVC** ❌ |
| 字母顺序 | E-N-C-V | E-N-V-C |

项目名为 `encv-go`，Magic 应为 `ENCV`，当前写成 `ENVC` 属于笔误/历史遗留错误。

### 影响范围

修改 Magic 会影响**所有已存在的容器文件**（V2/V3/V4）：
- 所有存量容器使用旧的 "ENVC" magic
- 改为 "ENCV" 后，旧容器无法被 DetectHeaderVersion 识别
- **必须兼容两种 magic**

### 修复方案

#### Step 1: 定义新 Magic 常量
```go
// types/container.go
var (
    // 新的正确 Magic（用于新写入的容器）
    MagicHeader = [4]byte{'E', 'N', 'C', 'V'}  // ENCV
    MagicFooter = [4]byte{'E', 'N', 'C', 'V'}

    // 旧 Magic 兼容（用于读取已有容器）
    LegacyMagicHeader_v2 = [4]byte{'E', 'N', 'V', 'C'}
    LegacyMagicFooter_v2 = [4]byte{'E', 'N', 'V', 'C'}
)
```

#### Step 2: DetectHeaderVersion 同时接受新旧 Magic
```go
// header_v3.go
func DetectHeaderVersion(data []byte) int {
    if len(data) < 4 { return 0 }
    magic := string(data[:4])
    if magic == string(MagicHeader[:]) || magic == string(LegacyMagicHeader_v2[:]) {
        // ... version detection unchanged
    }
    return 0
}
```

#### Step 3: ReadHeaderV4 / ReadHeaderV3 / ReadFooterV4 同时接受新旧 Magic
```go
// header_v4.go ReadHeaderV4:
if header.Magic != MagicHeader && header.Magic != LegacyMagicHeader_v2 {
    return nil, ErrInvalidMagic_v2
}

// header_v4.go ReadFooterV4:
if footer.Magic != MagicFooter && footer.Magic != LegacyMagicFooter_v2 {
    return nil, ErrInvalidMagic_v2
}
```

#### Step 4: WriteHeaderV4 / WriteFooterV4 / CreateHeaderV4 使用新 Magic
```go
// 所有写入路径使用新的正确 Magic
header.Magic = MagicHeader       // 不是 MagicHeader_v2
footer.Magic = MagicFooter        // 不是 MagicFooter_v2
```

#### Step 5: IsEncvContainerFromBytes 兼容旧 Magic
```go
// detector/detector.go
return bytes.Equal(footer.Magic[:], MagicFooter[:]) || 
       bytes.Equal(footer.Magic[:], LegacyMagicFooter_v2[:]), nil
```

#### Step 6: V2/V3 写入也使用新 Magic（WriteHeaderV3, envelope footer 等）

---

## Tasks

### Task 1: 修复 Close() V4 ManifestOffset 偏移计算
- [ ] 1.1 将 `manifestBlockSize` / `manifestBlockStart` 计算按版本分流
- [ ] 1.2 V4 不加 `GetBlockHeader_v2_Size()`，V2/V3 保持不变
- [ ] 1.3 验证: 用 SingleFileContainerWriterV4 写入后 openV4 能成功读取

### Task 2: 修正 Magic Header ENVC → ENCV + 兼容旧格式
- [ ] 2.1 在 types/container.go 定义新 Magic 常量 + 保留旧常量为 Legacy
- [ ] 2.2 修改 DetectHeaderVersion 接受新旧 Magic
- [ ] 2.3 修改 ReadHeaderV4/ReadFooterV4 接受新旧 Magic
- [ ] 2.4 修改 WriteHeaderV4/WriteFooterV4/CreateHeaderV4 使用新 Magic
- [ ] 2.5 修改 WriteHeaderV3/CreateHeaderV3 使用新 Magic
- [ ] 2.6 修改 IsEncvContainerFromBytes 兼容旧 Footer Magic
- [ ] 2.7 修改 V2/V3 footer 读写接受新旧 Magic

### Task 3: 补全端到端验证测试
- [ ] 3.1 TestV4_SingleFileWriter_Roundtrip — 通过 SinglePhysicalPacker→SingleFileContainerWriterV4 写入 → Open 成功
- [ ] 3.2 TestV4_DetectContainer_AfterFix — detector.DetectContainer 对插件路径产生的 V4 容器返回 non-nil
- [ ] 3.3 TestMagic_LegacyCompatibility — 用旧 "ENVC" magic 构造容器 → 仍能被检测
- [ ] 3.4 TestMagic_NewFormat — 用新 "ENCV" magic 构造容器 → 能被检测

### Task 4: 全量迭代验证
- [ ] 4.1 `go test ./internal/... -count=1` 零失败
- [ ] 4.2 确认 V4 E2E 测试真正走插件加密路径（不是 WriteV4Container 直接路径）

## Task Dependencies
- Task 1 (ManifestOffset) → Task 3 (E2E 测试依赖正确写入)
- Task 2 (Magic) → Task 3 (E2E 测试需要正确的 magic)
- Task 1 和 Task 2 可并行
- Task 4 最后执行

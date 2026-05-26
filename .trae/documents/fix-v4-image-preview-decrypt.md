# 修复图片（及所有插件）V4 容器预览/解密失败

## 问题诊断

### 现象
新加密的图片容器（V4 格式）预览和解密都无法正确显示图片。
文本 V4 容器之前被认为"可用"，但实际也受同样问题影响（可能因测试不充分未暴露）。

### 根因：V4 存在两种磁盘格式，读取代码假设了错误的格式

#### 格式 A：纯 V4（`WriteV4Container` 直接写入路径）
```
[SegmentHeader(18)][Nonce(N)][EncryptedData(D)]
```
Manifest 中 `Nonce` 非空，`Size = 18+N+D`

#### 格式 B：混合 V4（所有插件实际使用的路径）
```
[BlockHeader_v2(12)][EncryptedData(D)]
```
通过 `SingleFileContainerWriterV4.WriteFragment()` 写入，Manifest 中 `Nonce=""`，`Size=D`（纯数据长度）

**所有插件（ImagePlugin、TextPlugin 等）都走格式 B！**

### 三级 Bug 链

#### Bug 1: `AdaptV4ToV2` 偏移量计算错误（致命）

**文件:** [`handle/adapter.go:9-45`](internal/v2/container/handle/adapter.go#L9-L45)

当前代码假设格式 A（SegmentHeader + Nonce）：
```go
nonce, _ := base64.StdEncoding.DecodeString(seg.Nonce)       // "" → nil, len=0
encDataSize := seg.Size - SegmentHeaderSize(18) - len(nonce)  // D - 18 - 0 = D-18 ❌ 负数或错误值
PhysicalOffset: seg.Offset + 18 + 0                           // 指向加密数据中间 ❌
```

对于格式 B 的实际磁盘布局：
```
偏移 seg.Offset:   [BlockHeader(12)] ← seg.Offset 指向这里
偏移 seg.Offset+12: [EncryptedData(D)] ← 我们需要读这里
```

正确计算应为：
```go
// Nonce 为空 → 混合格式，使用 BlockHeader 而非 SegmentHeader
encDataSize := seg.Size                                    // Size 已经是纯数据长度
PhysicalOffset: seg.Offset + GetBlockHeader_v2_Size()     // 跳过 12 字节 BlockHeader
```

#### Bug 2: `fileContainerReader` 对 V4 重复加 BlockHeader 偏移

**文件:** [`reader/file_container_reader.go:121-126`](internal/v2/reader/file_container_reader.go#L121-L126)

```go
headerSize := block.GetBlockHeader_v2_Size()  // 固定 12 字节
for _, frag := range r.manifest.Fragments {
    r.physicalOffsets[frag.ID] = frag.PhysicalOffset + uint64(headerSize)  // V4 再加 12 ❌
}
```

V2/V3 的 `PhysicalOffset` 指向 BlockHeader 起始位置，需要 +12 到达数据区。
但 V4 经过 `AdaptV4ToV2` 后（Bug 1 修复后），`PhysicalOffset` 已指向数据区，不应再加。

#### Bug 3: `verifyFragmentAt` 对 V4 校验失败

**文件:** [`reader/file_container_reader.go:191, 454-469`](internal/v2/reader/file_container_reader.go#L191)

两重问题：
1. 偏移量错误导致读到垃圾数据（Bug 1+2 的后果）
2. 即使偏移正确，`AdaptV4ToV2` 设置 `DataCRC32: 0`，但磁盘上 BlockHeader 的 CRC ≠ 0 → 校验必失败

### 影响范围

**所有经过 `StandardPostEncrypt → SinglePhysicalPacker.Pack(V4)` 路径创建的 V4 容器全部无法解密/预览：**
- 图片（ImagePlugin `.sccgi`）— 用户报告
- 文本（TextPlugin `.sccgv`）— 同样受影响
- 音频、视频、PDF、WPS — 全部受影响

**不受影响的路径：**
- 直接使用 `WriteV4Container` 创建的容器（无插件使用此路径）
- V2/V3 容器（完全不同的代码路径）

---

## 修复计划

### Task 1: 修复 `AdaptV4ToV2` 支持混合 V4 格式

**修改文件:** [`internal/v2/container/handle/adapter.go`](internal/v2/container/handle/adapter.go)

**修改内容:**

```go
func AdaptV4ToV2(v4 *types.Manifest_v4, header *types.EnvelopeHeaderV4) *types.Manifest {
    fragments := make([]types.Fragment, len(v4.Segments))
    for i, seg := range v4.Segments {
        nonce, _ := base64.StdEncoding.DecodeString(seg.Nonce)

        var encDataSize uint64
        var physicalOffset uint64

        if len(nonce) == 0 {
            // 混合格式：WriteFragment 使用 block.WriteBlock 写入
            // 磁盘布局: [BlockHeader_v2(12)][EncryptedData]
            // seg.Offset = BlockHeader 起始位置, seg.Size = 纯数据长度
            encDataSize = seg.Size
            physicalOffset = seg.Offset + uint64(block.GetBlockHeader_v2_Size())
        } else {
            // 纯 V4 格式：WriteV4Container 直接写入
            // 磁盘布局: [SegmentHeader(18)][Nonce(N)][EncryptedData]
            encDataSize = seg.Size - uint64(types.SegmentHeaderSize) - uint64(len(nonce))
            physicalOffset = seg.Offset + uint64(types.SegmentHeaderSize) + uint64(len(nonce))
        }

        fragments[i] = types.Fragment{
            ID:             seg.ID,
            Type:           types.FragmentType_SeekableStream,
            Length:         encDataSize,
            PhysicalOffset: physicalOffset,
            DataCRC32:      0,
        }
    }
    // ... kind 映射不变 ...
}
```

需要导入 `block` 包以获取 `GetBlockHeader_v2_Size()`。

### Task 2: 修复 `fileContainerReader` 对 V4 不重复加偏移

**修改文件:** [`internal/v2/reader/file_container_reader.go`](internal/v2/reader/file_container_reader.go)

**修改 `NewEncryptedContainerReaderFromFile` L121-126:**

```go
// 原来（V2/V3 逻辑）：
headerSize := block.GetBlockHeader_v2_Size()
for _, frag := range r.manifest.Fragments {
    if frag.PhysicalPath == "" {
        r.physicalOffsets[frag.ID] = frag.PhysicalOffset + uint64(headerSize)
    }
}

// 修改为版本感知：
var blockHeaderSize int64
if r.headerVersion == 4 {
    // V4: AdaptV4ToV2 已将 PhysicalOffset 计算到数据区起始，无需再跳过 BlockHeader
    blockHeaderSize = 0
} else {
    // V2/V3: PhysicalOffset 指向 BlockHeader 起始，需跳过
    blockHeaderSize = block.GetBlockHeader_v2_Size()
}
for _, frag := range r.manifest.Fragments {
    if frag.PhysicalPath == "" {
        r.physicalOffsets[frag.ID] = frag.PhysicalOffset + uint64(blockHeaderSize)
    }
}
```

### Task 3: 修复 `GetFragmentReader` 对 V4 跳过 BlockHeader 校验

**修改文件:** [`internal/v2/reader/file_container_reader.go`](internal/v2/reader/file_container_reader.go)

**修改 `GetFragmentReader` L177-206，Case A（主文件）分支:**

```go
// Case A: 主文件
if frag.PhysicalPath == "" {
    payloadOffset, ok := r.physicalOffsets[frag.ID]
    if !ok || payloadOffset == 0 {
        return nil, fmt.Errorf("fragment '%s' offset missing or zero", fragID)
    }

    mainFile, useInit, err := r.acquireMainFile()
    if err != nil {
        return nil, err
    }

    // V4 容器的 Fragment 数据没有 BlockHeader 包裹，跳过校验
    if r.headerVersion != 4 {
        if err := r.verifyFragmentAt(mainFile, int64(payloadOffset)-int64(block.GetBlockHeader_v2_Size()), frag); err != nil {
            if !useInit {
                globalFileHandlePool.Put(mainFile)
            }
            return nil, err
        }
    }

    section := io.NewSectionReader(mainFile, int64(payloadOffset), int64(frag.Length))
    // ... 后续不变
}
```

### Task 4: 补全 V4 解密/预览端到端测试

**新建/修改文件:** `internal/v2/reader/reader_v4_e2e_test.go`

**测试用例：**

| 测试名 | 描述 |
|--------|------|
| `TestV4Roundtrip_PluginPath_Image` | 用 ImagePlugin 路径创建 V4 图片容器 → DecryptReaderFactory → 解密 → 验证输出与原始一致 |
| `TestV4Roundtrip_PluginPath_Text` | 同上，Text 类型 |
| `TestV4PreviewChain_WebDAV` | 创建 V4 容器 → WebDAV lazyAdapter.Read → 验证解密数据 |
| `TestV4AdaptV4ToV2_HybridFormat_Offset` | 验证混合格式下 AdaptV4ToV2 的 PhysicalOffset 正确指向数据区 |
| `TestV4AdaptV4ToV2_PureFormat_Offset` | 验证纯格式下偏移量也正确 |

**关键：必须使用 `createV4ViaPluginPath`（来自 `single_file_writer_v4_e2e_test.go`）或类似方法创建容器，确保走的是真实的插件加密路径（WriteKVI→WriteFragment→WriteManifest→Close），而非 `WriteV4Container` 直接路径。**

### Task 5: 补全图片插件 V4 专用测试

**新建文件:** `internal/v2/plugins/image/plugin_v4_test.go`

**测试用例：**

| 测试名 | 描述 |
|--------|------|
| `TestImagePlugin_V4_CanDecrypt` | 加密图片 V4 → CanDecrypt 返回 true |
| `TestImagePlugin_V4_Decrypt_Roundtrip` | 加密图片 V4 → Decrypt → 输出文件是合法图片 |
| `TestImagePlugin_V4_DetectIndexKind` | detector.DetectIndexKind 对图片 V4 返回 "image" |

### Task 6: 全量验证

```bash
go test ./internal/... -count=1 2>&1
go build ./internal/... ./cmd/encv/...
```

确保零 FAIL/PANIC。

---

## 数据流完整对照

### 加密写入（混合 V4 格式）

```
ImagePlugin.PostEncryptProcessor
  → packer.StandardPostEncrypt
    → SinglePhysicalPacker.Pack(V4)
      → NewSingleFileContainerWriterV4(path, header)
      → writeAndClose:
          loop fragments:
            → tempWriter.WriteFragment(&frag, chunkData)
              → frag.PhysicalOffset = 当前文件位置 (如 2048)
              → block.WriteBlock(file, BlockTypeData_v2, chunkData)
                → 写入 [BlockHeader(12)][chunkData]  ← 实际磁盘格式！
          → tempWriter.WriteManifest(manifestObj)
            → writeManifestV4:
              → segments[] = {{Offset: 2048, Size: D, Nonce: ""}}
              → ObfuscateManifest → 写入
          → tempWriter.Close()
            → Header.ManifestOffset = 当前位置
```

### 解密读取（修复前 — 断裂）

```
ImagePlugin.Decrypt / WebDAV Preview
  → containerManager.GetReadablePath
  → reader.NewDecryptReaderFactory
    → NewEncryptedContainerReaderFromFile
      → containerhandle.Open(src) → openV4()
        → DeobfuscateManifest → DeserializeManifest_v4 → AdaptV4ToV2  ❌ Bug1: 偏移 +18 错误
      → physicalOffsets[fragID] = frag.PhysicalOffset + 12  ❌ Bug2: 又加了 12
    → NewDecryptReader → SequentialSeekableDecryptReader
      → setupFragmentAtIndex → GetFragmentReader(fragID)
        → verifyFragmentAt(file, offset-12, frag)  ❌ Bug3: 读到垃圾数据，CRC 不匹配
        → SectionReader(file, 错误offset, 错误length)  ❌ 读到错误数据
      → AES-CTR 解密垃圾数据 → 输出乱码/空文件
```

### 解密读取（修复后 — 通畅）

```
ImagePlugin.Decrypt / WebDAV Preview
  → containerManager.GetReadablePath
  → reader.NewDecryptReaderFactory
    → NewEncryptedContainerReaderFromFile
      → containerhandle.Open(src) → openV4()
        → DeobfuscateManifest → DeserializeManifest_v4 → AdaptV4ToV2  ✅ 检测 Nonce="" 用混合格式
          → PhysicalOffset = seg.Offset + 12 (跳过 BlockHeader)
          → Length = seg.Size (纯数据长度)
      → headerVersion==4 → blockHeaderSize = 0  ✅ 不重复加
      → physicalOffsets[fragID] = frag.PhysicalOffset + 0  ✅ 直接用
    → NewDecryptReader → SequentialSeekableDecryptReader
      → setupFragmentAtIndex → GetFragmentReader(fragID)
        → headerVersion!=4? No → 跳过 verifyFragmentAt  ✅
        → SectionReader(file, 正确offset, 正确length)  ✅
      → AES-CTR 解密 → 输出原始明文 ✅
```

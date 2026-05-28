# Checklist

## Phase 1: FilePickerModal 新建文件夹修复

- [ ] 模板重构：new-folder-input 使用 overlay 定位，不与 ion-list 互斥
- [ ] 点击 + 按钮正确显示输入框（文件列表保持可见）
- [ ] createDirectory API 调用路径和参数正确
- [ ] 创建成功后 navigateTo 触发 loadFiles 刷新
- [ ] 取消操作正确恢复 UI 状态
- [ ] 错误场景有明确用户反馈（alert/toast）

## Phase 2: v4 stsz box missing 修复

- [ ] VerifyOptions 新增 SkipStructCheck 字段
- [ ] QuickStructCheck 支持 SkipStructCheck 跳过模式
- [ ] verifyContainer() 对重编码源传入 SkipStructCheck:true
- [ ] 重编码 MP4 不再报 stsz box missing
- [ ] 非重编码模式下结构检查仍正常执行

## Phase 3: v3 临时目录创建修复

- [ ] ensureOutputDir() 辅助函数已创建
- [ ] 所有 CreateTemp 调用点均已添加 MkdirAll 防御
- [ ] outputDir 不存在时能自动创建并成功创建临时文件
- [ ] 测试覆盖 outputDir 不存在场景

## Phase 4: v4 容器信息乱码修复

- [ ] container_id 显示为有效字符串（非乱码/非空）
- [ ] manifest JSON 可正确解析和格式化显示
- [ ] version 数字正确显示
- [ ] 前端对异常数据有容错处理

## Phase 5: Mock 测试完善

### FilePickerModal 测试
- [ ] + 按钮点击 → 输入框显示测试通过
- [ ] 确认创建 → API → 刷新流程测试通过
- [ ] 取消操作测试通过
- [ ] 空名称拦截测试通过
- [ ] API 失败错误提示测试通过

### 加密流程 E2E 测试
- [ ] v3 不重编码加密完整流程测试通过
- [ ] v4 重编码加密 + SkipSizeCheck + SkipStructCheck 测试通过
- [ ] outputDir 自动创建测试通过

### 容器信息 API 测试
- [ ] v3 容器 info 返回结构验证通过
- [ ] v4 容器 info container_id + manifest 编码验证通过

# 前端设计规范

## 状态展示规范（重要！）

### 任务卡片/选项项状态徽章

**正确做法**：在 `ion-item` 末尾使用 `ion-badge` 显示状态

```vue
<!-- ✅ 正确：ion-badge 在 ion-item 末尾显示 -->
<ion-item>
  <ion-icon :icon="filmOutline" slot="start"></ion-icon>
  <ion-select>
    <ion-select-option value="mpv-plugin">MPV 播放器</ion-select-option>
  </ion-select>
  <ion-badge slot="end" color="warning">未安装</ion-badge>
</ion-item>
```

**错误做法**：在 `ion-select-option` 内部使用 `<span>` 标签

```vue
<!-- ❌ 错误：ion-select-option 内部 span 在 action-sheet 模式下不显示 -->
<ion-select-option value="mpv-plugin">
  MPV 播放器
  <span style="color: red">(未安装)</span>  <!-- 不显示！ -->
</ion-select-option>
```

### 原因分析

- `ion-select` 的 `interface="action-sheet"` 模式下，`ion-select-option` 内部的非文本元素（如 `<span>`）**不渲染**
- Action Sheet 是原生弹出层，只显示 `ion-select-option` 的纯文本内容
- 状态徽章必须在 `ion-item` 层级显示，才能在选项列表中可见

### 状态徽章颜色规范

| 状态类型 | 颜色 | 示例 |
|---------|------|------|
| 警告（未安装/已禁用/未加载） | `warning` | 未安装、已禁用、未加载 |
| 错误（加载失败/查询失败） | `danger` | 加载失败、查询失败 |
| 成功（已就绪） | `success` | ✓ |
| 信息（可选） | `primary` | 可用 |

### 禁用选项配合状态徽章

```vue
<ion-item>
  <ion-select>
    <!-- 选项禁用 + 状态徽章 -->
    <ion-select-option value="mpv-plugin" :disabled="status !== 'ready'">MPV 播放器</ion-select-option>
  </ion-select>
  <!-- 状态徽章：仅在非 ready 状态显示 -->
  <ion-badge v-if="status !== 'ready'" slot="end" color="warning">未安装</ion-badge>
  <!-- 成功徽章：仅在 ready 状态显示 -->
  <ion-badge v-if="status === 'ready'" slot="end" color="success">✓</ion-badge>
</ion-item>
```

---

## Ionic 组件使用规范

### ion-select-option 内容限制

- **只允许纯文本**：`<ion-select-option>文本内容</ion-select-option>`
- **禁止嵌套元素**：`<span>`、`<div>`、`<ion-badge>` 等在 action-sheet 模式下不渲染
- **状态信息通过 ion-item 层级展示**

### ion-badge 位置规范

- **slot="end"**：放在 ion-item 末尾，与主内容对齐
- **条件渲染**：使用 `v-if` 控制显示时机
- **颜色语义**：warning/danger/success 对应警告/错误/成功状态

---

## 铁律关联

1. **严禁 Toast 提示**：状态信息必须通过持久性 UI 元素（如 ion-badge）显示
2. **严禁自动 fallback**：选项禁用 + 状态徽章让用户明确知道不可用，主动选择其他方案
3. **饱和调试**：前端状态展示必须可见、持久、语义清晰
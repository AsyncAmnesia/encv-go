# 主题色暗黑模式适配计划

## 问题根因分析

### 核心冲突

`useTheme.ts` 的 `applyColor()` 将用户选择的颜色**原样**设置为 CSS 变量，未考虑当前是否为暗黑模式。

**冲突链路**：

```
variables.css 定义了两套 primary 色：
├── :root (亮色)    → --ion-color-primary: #4f8cff   (原始蓝)
└── body.dark (暗色) → --ion-color-primary: #6a9eff   (更亮的蓝 ✅ 正确)

useTheme.ts::applyColor() 行为：
├── 用户选 Purple #8b5cf6
├── root.style.setProperty('--ion-color-primary', '#8b5cf6')  ← inline style!
└── inline style 优先级 > body.dark 选择器
    → 暗黑模式下仍然使用 #8b5cf6（未做亮度提升）
    → 在深色背景上对比度不足 / 视觉不协调
```

### 具体表现

| 场景 | 当前行为 | 预期行为 |
|------|---------|---------|
| 亮色 + 自定义色 | ✅ 正常显示 | — |
| 暗色 + 自定义色 | ⚠️ 原色直接用，偏暗 | 应自动提亮/增强饱和度 |
| 暗色切换亮色 | ⚠️ 保持暗色调整后的值 | 应恢复原始色 |
| 亮色切换暗色 | ⚠️ 无变化 | 应自动应用暗色调整 |

---

## 修改方案

### 文件清单

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `app/encv-mobile/src/composables/useTheme.ts` | **核心逻辑修改** | 增加暗黑模式感知的颜色调整算法 |
| `app/encv-mobile/src/theme/variables.css` | 无需改动 | 保持现有暗色默认值作为 fallback |
| `app/encv-mobile/src/App.vue` | 无需改动 | toggle 样式已正确引用 CSS 变量 |

---

### Step 1: 修改 `useTheme.ts` — 增加暗黑模式颜色调整

#### 1.1 新增颜色调整函数

```typescript
function adjustForDarkMode(hex: string): string {
  // 将颜色向亮度和饱和度方向微调，使其在深色背景上更醒目
  // 算法：在 HSL 空间中提升亮度 (+8%~15%) + 轻微提升饱和度
}
```

**调整策略**：
- 亮度提升：原始亮度 < 50% 时 +12%，≥ 50% 时 +8%
- 饱和度微调：+5% 避免暗色下显得灰暗
- 保护上限：亮度不超过 75%，避免过曝

#### 1.2 修改 `applyColor()` 签名和行为

```typescript
// Before:
function applyColor(color: string) { ... }

// After:
function applyColor(color: string, forceDark?: boolean) {
  const dark = forceDark ?? isDark.value
  const actualColor = dark ? adjustForDarkMode(color) : color
  // 用 actualColor 设置所有 --ion-color-primary-* 变量
}
```

**关键点**：
- 同时计算 adjusted 和 original 两套衍生变量（shade/tint/contrast）
- 基于 `actualColor`（非原始 color）计算 contrast，确保文字可读性

#### 1.3 修改 `toggleDark()` — 切换时重新应用颜色

```typescript
function toggleDark() {
  const newDark = !isDark.value
  applyTheme(newDark)
  localStorage.setItem(THEME_KEY, newDark ? 'dark' : 'light')
  applyColor(currentColor.value)  // ← 新增：用新模式重新应用当前色
}
```

#### 1.4 新增状态追踪

```typescript
const baseColor = ref('#4f8cff')  // 用户选择的原始色（不受暗黑模式影响）
// currentColor 保持现有语义，但改为存储「实际应用的值」（用于 UI 显示）
```

或者更简洁的方案：`currentColor` 始终存储用户原始选择，显示用时根据 `isDark` 动态计算。

#### 1.5 导出 API 变更

```typescript
export function useTheme() {
  return {
    isDark,
    currentColor,        // 用户原始选择（用于 Settings 显示）
    appliedColor,        // 实际生效值（computed: isDark ? adjusted : raw）
    initTheme,
    toggleDark,
    setThemeColor,       // 内部调用 applyColor(current, isDark)
    THEME_PRESETS,
  }
}
```

---

### Step 2: 验证场景清单

| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 亮色模式选 Purple `#8b5cf6` | 按钮/FAB/toggle 显示 `#8b5cf6` |
| 2 | 切换到暗色模式 | 同元素自动变为更亮的紫色 (~亮度+12%) |
| 3 | 暗色模式切回亮色 | 恢复为原始 `#8b5cf6` |
| 4 | 暗色模式选 Orange `#f97316` | 显示提亮后的橙色 |
| 5 | 暗色模式使用取色器选极暗色 `#1a1a1e` | 自动提亮到可见范围 |
| 6 | 刷新页面后恢复 | 从 localStorage 读取后正确应用对应模式的颜色 |
| 7 | ion-toggle ON 状态手柄 | 暗色下保持白色（已有 ::part 规则） |
| 8 | ion-toggle ON 状态轨道 | 使用调整后的 primary 色 |

---

## 实现细节

### adjustForDarkMode() 伪代码

```typescript
function adjustForDarkMode(hex: string): string {
  const clean = hex.replace('#', '')
  let r = parseInt(clean.substring(0, 2), 16)
  let g = parseInt(clean.substring(2, 4), 16)
  let b = parseInt(clean.substring(4, 6), 16)

  // 转 HSL
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const l = (max + min) / 2 / 255

  // 亮度提升：暗色需要更醒目
  const liftPercent = l < 0.5 ? 12 : 8
  r = Math.min(255, Math.round(r + (255 - r) * liftPercent / 100))
  g = Math.min(255, Math.round(g + (255 - g) * liftPercent / 100))
  b = Math.min(255, Math.round(b + (255 - b) * liftPercent / 100))

  return `#${r.toString(16).padStart(2,'0')}${g.toString(16).padStart(2,'0')}${b.toString(16).padStart(2,'0')}`
}
```

### 不破坏的功能

- localStorage 持久化键名不变 (`encv-theme-color`)
- `THEME_PRESETS` 数组不变（存储的是用户视角的原始值）
- Settings.vue 的 color-dot `.active` 判断不变（比较原始值）
- `getContrastColor()` 基于 actualColor 计算（自动适配）

---

## 风险与注意事项

1. **性能**：`adjustForDarkMode` 是纯计算函数，无 DOM 操作，无性能风险
2. **向后兼容**：已存储的 localStorage 值是原始色，升级后首次加载会正确应用调整
3. **极端颜色**：用户选纯黑 `#000000` → 提亮后仍偏暗但可用；可选加最低亮度保底

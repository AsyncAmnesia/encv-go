# 清理死代码 + 控制台开关说明

## 一、控制台（vConsole）开关说明

设置页面的 **"vConsole 调试工具"** 开关控制的是 **[vConsole](https://github.com/nicedoc/vconsole)** —— 一个移动端前端调试面板。

### 工作原理

```
App.vue 启动时 → autoInitVConsole()
  → 检查 localStorage 'encv_vconsole_enabled' === 'true'
  → 是：动态 import('vconsole') 并实例化
  → 否：不加载

Settings.vue 开关切换 → toggleVConsole(checked)
  → 开：创建 VConsole 实例（屏幕右下角出现绿色调试按钮）
  → 关：销毁 VConsole 实例
```

### 作用

- 在手机上打开一个**类似 Chrome DevTools 的浮动面板**
- 可以查看 `console.log/warn/error`、网络请求、DOM/Storage 等
- 对**开发调试有用**，正式发布建议关闭（vconsole.min.js 有 ~78KB gzip）
- 状态持久化在 `localStorage`，不会随页面刷新丢失

### 结论

这个功能本身没问题，是标准的前端移动端调试工具。如果觉得对普通用户太技术化，可以考虑：
- 隐藏到"连续点击版本号 5 次"的彩蛋入口里
- 或者保持现状，反正默认关闭不影响体验

---

## 二、清理死文件 WebDAV.vue

### 问题

`src/views/WebDAV.vue` 是 Remote.vue 的旧版副本，路由已迁移到 Remote.vue，WebDAV.vue 完全未被引用。

### 执行步骤

1. **删除 `app/encv-mobile/src/views/WebDAV.vue`**

2. **确认无残留引用** — 搜索全项目确保没有 import 或动态引用 WebDAV.vue

3. **验证构建** — `vue-tsc --noEmit && vite build`

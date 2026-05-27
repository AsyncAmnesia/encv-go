# 修复 Files.vue PlayMode 类型定义

## 问题

`src/views/Files.vue` L273 的 `PlayMode` 类型定义仍为旧值 `'mpv'`，但 `getPlayMode()` 和 `playMedia()` 已使用 `'mpv-plugin'`，导致 vue-tsc 编译报 4 个 TS2322/TS2678 错误。

## 修复

**文件**: `/workspace/app/encv-mobile/src/views/Files.vue`

**L273** 将：
```typescript
type PlayMode = 'artplayer' | 'mpv' | 'external'
```
改为：
```typescript
type PlayMode = 'artplayer' | 'mpv-plugin' | 'external'
```

## 验证

```bash
cd app/encv-mobile && npx vue-tsc --noEmit && npm run build
```

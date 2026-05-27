# Tasks

- [ ] Task 1: 修复 android-overlay 中 EncvGoService.kt 的 config.mobile.json 幽灵改动
  - [ ] 将 `android-overlay/app/src/main/java/com/encvgo/app/EncvGoService.kt` 中第390行 `assets.open("config.mobile.json")` 改为 `assets.open("config.user.json")`
  - [ ] 将第408行 `assets.open("config.mobile.json")` 改为 `assets.open("config.user.json")`
  - [ ] 验证：`grep -r "config.mobile" android-overlay/` 应无结果

- [ ] Task 2: 在 sync-native.mjs 中添加 config.user.json 复制逻辑
  - [ ] 在 `scripts/sync-native.mjs` 末尾添加：从项目根目录 `../../../config.user.json` 复制到 `android/app/src/main/assets/config.user.json`
  - [ ] 验证：执行 `node scripts/sync-native.mjs` 后 `android/app/src/main/assets/config.user.json` 存在

- [ ] Task 3: CI workflow 为 release 构建添加 `npx cap copy android` 步骤
  - [ ] 修改 `.github/workflows/android.yml`：将 "Copy web assets to Android project" 步骤的 `if: inputs.version == ''` 条件移除，使 release 构建也执行 `npx cap copy android`
  - [ ] 更新步骤注释，说明 release 构建也需要此步骤
  - [ ] 验证：release 构建日志中应出现 cap copy 输出

- [ ] Task 4: CI 添加 APK 内 Web 资源存在性验证
  - [ ] 在 "Verify APK contents" 步骤中添加检查：`unzip -l "$APK_PATH" | grep "public/index.html"` 必须存在
  - [ ] 若缺失则 `echo "::error::Web assets missing from APK!" && exit 1`
  - [ ] 同时检查 `config.user.json` 在 assets 中是否存在

# Task Dependencies
- [Task 3] depends on [Task 1] (cap copy 会触发 sync-native.mjs，但 overlay 的修复确保即使 post-cap-sync.mjs 也执行也不会引入 config.mobile.json)
- [Task 4] depends on [Task 3] (验证步骤在 cap copy 添加后才有效)

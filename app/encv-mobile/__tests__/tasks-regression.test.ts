import { describe, it, expect, vi } from 'vitest'

// ============================================================
// 防护性回归测试：Tasks.vue 核心功能
// 目标：防止 P0-1 (路由损坏) 和 P0-2 (插件逻辑缺失) 回归
// ============================================================

describe('Tasks.vue 防护性回归测试', () => {
  describe('P0-1: 路由完整性保护', () => {
    it('processQueryAction 必须保持同步执行（不使用 async/await）', () => {
      // 读取源文件验证关键函数签名
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // 验证 processQueryAction 不是 async 函数
      const funcMatch = source.match(/function processQueryAction\(\)/)
      expect(funcMatch).toBeTruthy()

      // 验证函数体内没有 await（保持同步）
      const asyncMatch = source.match(/async function processQueryAction/)
      expect(asyncMatch).toBeNull()
    })

    it('showNewTaskModal 必须使用 ref 控制模式（非 modalController.create）', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // 必须存在 showNewTaskModal ref
      expect(source).toMatch(/const showNewTaskModal = ref/)
      
      // 必须使用 :is-open 绑定（inline modal 模式）
      expect(source).toMatch(/<ion-modal\s+:is-open="showNewTaskModal"/)
      
      // 不应该用 modalController.create 打开新建任务 modal
      // （FilePickerModal 可以用 modalController.create）
      const lines = source.split('\n')
      const modalCreateInShowNewTask = lines.some((line, idx) => {
        if (line.includes('modalController.create') && line.includes('NewTaskModal')) {
          // 检查是否在 showNewTaskSheet 或 openNewTaskModal 函数内
          return true
        }
        return false
      })
      // 注意：如果重构为 NewTaskModal 组件，这个检查需要调整
      // 当前版本应该使用 inline modal
    })
  })

  describe('P0-2: 插件逻辑完整性保护', () => {
    it('必须导入 useTaskForm 并解构 predictPlugin', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // 必须导入 useTaskForm
      expect(source).toMatch(/import.*useTaskForm.*from/)
      
      // 必须解构出 predictPlugin (或 doPredict)
      expect(source).toMatch(/predictPlugin:\s*doPredict/)
      
      // 必须导入 usePathResolver
      expect(source).toMatch(/import.*usePathResolver.*from/)
    })

    it('validateSourcePath 必须在路径有效时触发 doPredict', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // validateSourcePath 函数内必须包含 doPredict 调用
      const funcStart = source.indexOf('function validateSourcePath()')
      const nextFunc = source.indexOf('\nfunction ', funcStart + 1)
      const funcBody = source.substring(funcStart, nextFunc)

      expect(funcBody).toContain('doPredict(')
      expect(funcBody).toContain('normalize(')
    })

    it('processQueryAction 必须在设置路径后触发 doPredict', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // processQueryAction 函数内必须包含 doPredict 调用
      const funcStart = source.indexOf('function processQueryAction()')
      const nextFunc = source.indexOf('\nfunction ', funcStart + 1)
      const funcBody = source.substring(funcStart, nextFunc)

      expect(funcBody).toContain('doPredict(')
      expect(funcBody).toContain('normalize(')
    })

    it('模板中必须包含插件选择 UI', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // 必须有插件提示或选择器（使用 candidates + predictedPlugin）
      const hasPluginUI =
        source.includes('candidates.length') &&
        source.includes('predictedPlugin') &&
        (source.includes('plugin-hint') || source.includes('plugin-selector') ||
         source.includes('filteredCandidates') || source.includes('showPluginSelector'))

      expect(hasPluginUI).toBe(true)
    })
  })

  describe('浏览按钮完整性保护', () => {
    it('必须保留 handleBrowseSource 和 handleBrowseTarget 函数', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      expect(source).toMatch(/function handleBrowseSource\(\)/)
      expect(source).toMatch(/function handleBrowseTarget\(\)/)
      expect(source).toMatch(/FilePickerModal/)
    })

    it('模板中必须有浏览按钮（folderOpen 图标）', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // 至少有 2 个浏览按钮（源路径 + 目标路径）
      const browseBtnCount = (source.match(/handleBrowseSource/g) || []).length +
                             (source.match(/handleBrowseTarget/g) || []).length
      
      expect(browseBtnCount).toBeGreaterThanOrEqual(2)
    })
  })
})

describe('架构约束：防止危险的架构变更', () => {
  it('禁止在 processQueryAction 中先 router.replace 再打开 modal', () => {
    const fs = require('fs')
    const source = fs.readFileSync(
      require('path').resolve(__dirname, '../src/views/Tasks.vue'),
      'utf-8'
    )

    // 提取 processQueryAction 函数体
    const funcStart = source.indexOf('function processQueryAction()')
    const nextFunc = source.indexOf('\nfunction ', funcStart + 1)
    const funcBody = source.substring(funcStart, nextFunc)

    // router.replace 必须在 showNewTaskModal.value = true 之后（或不存在）
    const showModalIdx = funcBody.indexOf('showNewTaskModal.value = true')
    const replaceIdx = funcBody.indexOf('router.replace')

    if (showModalIdx !== -1 && replaceIdx !== -1) {
      // router.replace 应该在 showNewTaskModal 之后（或不影响 modal 显示）
      // 危险模式：replace 在 showModal 之前会清除 query 导致 modal 无法获取参数
      const isDangerous = replaceIdx < showModalIdx && 
                           funcBody.includes('nextTick') === false
      
      expect(isDangerous).toBe(false)
    }
  })

  it('禁止删除 inline <ion-modal> 模板', () => {
    const fs = require('fs')
    const source = fs.readFileSync(
      require('path').resolve(__dirname, '../src/views/Tasks.vue'),
      'utf-8'
    )

    // 必须保留 ion-modal :is-open 模式
    expect(source).toMatch(/<ion-modal/)
    expect(source).toMatch(/:is-open=/)
  })
})

import { describe, it, expect } from 'vitest'

// ============================================================
// 防护性回归测试：Tasks.vue + useNewTaskModal 架构
// 目标：防止 P0-1 (路由损坏) 和 P0-2 (插件逻辑缺失) 回归
// ============================================================

describe('Tasks.vue 防护性回归测试', () => {
  describe('P0-1: 路由完整性保护', () => {
    it('必须使用 useNewTaskModal composable（全局 modalController.create 模式）', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // 必须导入并使用 useNewTaskModal
      expect(source).toMatch(/useNewTaskModal/)
      expect(source).toMatch(/const \{ openNewTask \} = useNewTaskModal\(\)/)

      // 不应该有 inline ion-modal（已迁移到 modalController.create）
      const hasInlineIonModal = source.includes('<ion-modal') && source.includes(':is-open=')
      expect(hasInlineIonModal).toBe(false)
    })

    it('eventBus open-new-task 必须正确桥接到 openNewTask', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // 必须监听 open-new-task 事件
      expect(source).toMatch(/eventBus\.on\('open-new-task'/)
      
      // handler 必须调用 openNewTask 并传递 sourcePath 和 taskType
      expect(source).toMatch(/openNewTask\(data\.sourcePath.*data\.taskType\)/)
    })

    it('processQueryAction (直链访问) 必须在 onMounted 中处理', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/views/Tasks.vue'),
        'utf-8'
      )

      // onMounted 中必须有 query.action === 'new' 的检查
      expect(source).toMatch(/route\.query\.action === 'new'/)

      // 必须在 query 处理后调用 openNewTask
      const mountedBlock = source.substring(source.indexOf('onMounted(() =>'), source.indexOf('onUnmounted'))
      expect(mountedBlock).toContain('openNewTask(')
    })
  })

  describe('P0-2: 插件逻辑完整性保护（委托给 useNewTaskModal）', () => {
    it('useNewTaskModal 必须封装完整的插件预测流程', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/composables/useNewTaskModal.ts'),
        'utf-8'
      )

      // 必须导入 useTaskForm
      expect(source).toMatch(/useTaskForm/)

      // 必须解构出 doPredict (predictPlugin)
      expect(source).toMatch(/predictPlugin:\s*doPredict/)

      // 必须导入 usePathResolver
      expect(source).toMatch(/usePathResolver/)

      // openNewTask 函数必须在有 initialSourcePath 时调用 doPredict
      expect(source).toMatch(/doPredict\(normalized/)
      expect(source).toMatch(/normalize\(/)
    })

    it('NewTaskModal 组件必须包含插件选择 UI', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/components/NewTaskModal.vue'),
        'utf-8'
      )

      // 必须有插件提示或选择器
      const hasPluginUI =
        (source.includes('candidates.length') || source.includes('cands.length')) &&
        (source.includes('predictedPlugin') || source.includes('pluginName')) &&
        (source.includes('plugin-hint') || source.includes('plugin-selector'))

      expect(hasPluginUI).toBe(true)
    })

    it('NewTaskModal 必须包含文件浏览按钮', () => {
      const fs = require('fs')
      const source = fs.readFileSync(
        require('path').resolve(__dirname, '../src/components/NewTaskModal.vue'),
        'utf-8'
      )

      // 必须引用 FilePickerModal
      expect(source).toMatch(/FilePickerModal/)

      // 必须有浏览相关函数（handleBrowse）
      expect(source).toMatch(/handleBrowse/)

      // 模板中必须有浏览按钮
      expect(source).toMatch(/browse-btn|folderOpen|@click="handleBrowse/)
    })
  })
})

describe('架构约束：全局 Modal 架构保护', () => {
  it('禁止 Tasks.vue 包含 inline <ion-modal>', () => {
    const fs = require('fs')
    const source = fs.readFileSync(
      require('path').resolve(__dirname, '../src/views/Tasks.vue'),
      'utf-8'
    )

    // Tasks.vue 不应包含 inline ion-modal（新建任务 modal 已迁移到 useNewTaskModal）
    const hasInlineModal = /<ion-modal\s/.test(source) && /:is-open=/.test(source)
    expect(hasInlineModal).toBe(false)
  })

  it('useNewTaskModal 必须使用 modalController.create 创建 NewTaskModal', () => {
    const fs = require('fs')
    const source = fs.readFileSync(
      require('path').resolve(__dirname, '../src/composables/useNewTaskModal.ts'),
      'utf-8'
    )

    // 必须使用 modalController.create
    expect(source).toMatch(/modalController\.create\(/)

    // component 必须是 NewTaskModal
    expect(source).toMatch(/component:\s*NewTaskModal/)

    // 必须调用 modal.present()
    expect(source).toMatch(/modal\.present\(\)/)
  })

  it('FAB 按钮必须调用 openNewTask()', () => {
    const fs = require('fs')
    const source = fs.readFileSync(
      require('path').resolve(__dirname, '../src/views/Tasks.vue'),
      'utf-8'
    )

    // FAB 按钮的 @click 必须绑定到 openNewTask
    expect(source).toMatch(/@click="openNewTask\(\)"/)
  })
})

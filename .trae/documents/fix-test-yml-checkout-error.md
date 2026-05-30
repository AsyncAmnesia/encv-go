# 修复 test.yml actions/checkout@v4 报错计划

## 问题诊断

### 错误现象
```
actions/checkout@v4 → git fetch 失败
fetch spec: +refs/heads/4/merge*:refs/remotes/origin/4/merge*
exit code 1 (重试 3 次均失败)
```

### 根因
1. **触发场景**：工作流由 `pull_request` 事件触发（PR #4）
2. **问题表达式**：`ref: ${{ inputs.branch || github.ref_name }}`
3. **失败机制**：
   - `actions/checkout@v4` 在 `pull_request` 事件中，当 `ref` 参数被显式设置时，会基于该值构造 fetch spec
   - 如果解析出的 ref 值与 PR merge ref 格式冲突，或 checkout 无法正确映射到实际存在的 Git 引用，就会构造出类似 `4/merge*` 的异常 fetch spec
   - shallow clone（默认 `fetch-depth: 1`）加剧了此问题——merge ref 需要额外的 fetch 操作
4. **影响范围**：所有 3 个 job（layer1 / layer2 / layer3）的 checkout 步骤都使用相同配置，全部会失败

## 修复方案

### 方案选择：事件感知的智能 ref 选择

为每个 job 的 checkout 步骤添加事件类型判断，针对不同触发源使用最可靠的 ref：

| 触发事件 | 推荐取值 | 原因 |
|---------|---------|------|
| `push` | `github.sha` 或 `github.ref_name` | 稳定可用 |
| `pull_request` | `github.sha` | 始终指向 merge commit，无需额外 fetch |
| `workflow_dispatch` | `inputs.branch \|\| 'main'` | 用户指定分支 |
| `schedule` | `'main'` | 定时任务用主分支 |

### 具体修改

#### 修改点 1：layer1-quick-tests job 的 checkout 步骤（L44-L48）

```yaml
# 修改前
- name: Checkout repository
  uses: actions/checkout@v4
  with:
    ref: ${{ inputs.branch || github.ref_name }}

# 修改后
- name: Checkout repository
  uses: actions/checkout@v4
  with:
    ref: ${{ github.event_name == 'pull_request' && github.sha || github.event_name == 'workflow_dispatch' && inputs.branch || github.event_name == 'schedule' && 'main' || github.sha }}
    fetch-depth: 1
```

#### 修改点 2：layer2-full-regression job 的 checkout 步骤（L127-L131）

同上模式修改。

#### 修改点 3：layer3-e2e-integration job 的 checkout 步骤（L196-L200）

同上模式修改。

## 实施步骤

1. [ ] 修改 `layer1-quick-tests` job 的 checkout 配置（L44-L48）
2. [ ] 修改 `layer2-full-regression` job 的 checkout 配置（L127-L131）
3. [ ] 修改 `layer3-e2e-integration` job 的 checkout 配置（L196-L200）
4. [ ] 验证 YAML 语法正确性
5. [ ] 提交修复并推送到远程触发 CI 验证

## 为什么这个修复有效

- **`github.sha` 在所有事件中都可靠**：它是一个绝对存在的 commit SHA，不需要额外的 ref 解析
- **避免 PR merge ref fetch 问题**：直接用 SHA checkout 绕过了 checkout 内部对 merge ref 的猜测逻辑
- **保持 `fetch-depth: 1`**：shallow clone 对速度有利，SHA checkout 不需要历史
- **向后兼容**：`workflow_dispatch` 的 `inputs.branch` 仍然生效

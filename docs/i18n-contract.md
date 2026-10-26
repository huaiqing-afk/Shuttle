# 中英双语（i18n）架构契约（需求 4）

状态：Type C Phase 2 契约，待确认后冻结。依据：AGENTS.md、docs/architecture.md、LESSONS.md。

## 1. 范围与边界

| 维度 | 纳入范围 | 排除范围（Out of Scope） | 归属/原因 |
|---|---|---|---|
| **前端文案** | 全部 13 个视图文件中的静态文案、状态标签、对话框、提示信息 | 动态终端输出流（ConPTY/xterm 输出）、第三方 Agent 生成内容 | 终端输出为原生进程字符流，由底层 CLI 自身语言决定 |
| **后端 Go 侧** | 保持稳定，v1 不改动 Go 错误与日志 | Go 内部错误信息的全量本地化 | 错误信息在桥接层透传，前端由统一错误捕获或原样展示，避免过度抽象 |
| **运行时依赖** | 零外部依赖，手写类型安全轻量字典（~100 行） | 引入 `react-i18next`、`formatjs` 等重型框架 | 避免打包膨胀与破坏现有单向状态流动 |
| **测试锚定** | 存量验收脚本硬编码中文标签需保持 100% 兼容 | 强行将存量测试重构为双语多分支断言 | 存量脚本（UIA / geometry）固定在 `zh-CN` 基准下运行 |

## 2. 状态机与持久化

### 2.1 语言枚举与解析
- 语言选项：`auto`（跟随系统）、`zh-CN`（简体中文）、`en-US`（English）。
- 本地存储键：`shuttle-language`（存入 `localStorage`）。
- 解析规则（`resolveLanguage`）：
  - 显式 `zh-CN` → `zh-CN`
  - 显式 `en-US` → `en-US`
  - `auto` 或空/非法值 → 读取 `navigator.language`；以 `zh` 开头返回 `zh-CN`，以 `en` 开头返回 `en-US`；其他默认回退 `zh-CN`。
- 响应式同步：`useI18n()` 提供全局上下文，切换语言时整树即时重渲染。

## 3. 字典结构与契约签名

### 3.1 字典键命名规范
键采用层级扁平化点分隔命名：`<模块>.<子项>.<属性>`，如：
- `app.tagline`: "接驳" / "Shuttle"
- `app.versionBadge`: "多终端验证版" / "Multi-terminal Preview"
- `app.btn.inventory`: "安装归属" / "Inventory"
- `app.btn.config`: "配置与还原" / "Configuration"
- `terminal.palette`: "终端配色" / "Terminal Theme"
- `terminal.empty.title`: "接上任何一个会话" / "Resume Any Session"

### 3.2 契约接口签名 (TypeScript)

```typescript
export type Language = 'zh-CN' | 'en-US'
export type LanguageChoice = 'auto' | Language

export interface I18nContextValue {
  language: Language
  choice: LanguageChoice
  setChoice: (choice: LanguageChoice) => void
  t: (key: string, params?: Record<string, string | number>) => string
}
```

### 3.3 参数插值规范
仅支持简单模板变量语法 `{param}`，如：
- `t('dialog.exitConfirm.desc', { count: 3 })`
- 中文模板：`"有 {count} 个终端仍在运行。退出将终止全部终端进程，未保存的工作将丢失。是否退出并终止？"`
- 英文模板：`"{count} terminal(s) still running. Exiting will terminate all processes and unsaved work will be lost. Exit anyway?"`

## 4. UI 布局与 12px 几何保护

- 切换器位置：置于 Header 区域的 ThemeSeg（主题切换）左侧或右侧。
- 形式：紧凑型三态或双态分段器/切换按钮（图标+文字或简写 `ZH / EN`）。
- 几何硬线：
  - 与相邻兄弟（ThemeSeg、PanelLeft 等）严格保持 `gap ≥ 12px`。
  - 在最窄测试视口 720×480 及 640×400 下，Header 内部不能发生控件重叠、截断或横向滚动。
  - 门禁在 `geometry-check.cjs` 中追加 Header 控件列表与间距断言。

## 5. 存量测试脚本兼容保障

- `geometry-check.cjs`：在打开页面前或执行探针前，默认或显式注入 `localStorage.setItem('shuttle-language', 'zh-CN')`，保证查找中文文本（`安装归属`、`自动安装`等）零破坏。
- 新增 `i18n.test.ts`：纯单元测试，断言：
  1. `zh` 与 `en` 字典 key 集合 100% 对齐（`Set(Object.keys(zh)) == Set(Object.keys(en))`）。
  2. 插值逻辑正确替换 `{key}`。
  3. 非法值安全回退。
- 新增双语切换专项几何与 UIA 脚本：验证切换为 `en-US` 后英文排版正常、无横向溢出。

## 6. 任务拆解（每卡 ≤ 300 行）

- **Card 1：i18n 核心模块与字典定义**（`frontend/src/lib/i18n/` + `frontend/tests/i18n.test.ts`）。实现上下文、字典表、插值、对称性单元测试。
- **Card 2：Header 语言切换组件与全局挂载**（`frontend/src/app/LangSeg.tsx` + `App.tsx` + `geometry-check.cjs` 锚定）。通过 13 组几何门禁。
- **Card 3：全站 13 个业务组件文案双语替换**（App、Header、Footer、Terminal、Sessions、Config、Inventory）。整体验收并保留双语截图。

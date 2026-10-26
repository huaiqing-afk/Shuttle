# 初始化与技术验证任务

用户已授权工程初始化与技术验证。以下是验证批次，不代表首版功能完成。

- [x] T01：建立 AGENTS.md、LESSONS.md、任务清单；保留架构文档。
- [x] T02：Go 1.27 安装（winget）；Wails v2.17.0 稳定版 + WebView2 已确认。
- [x] T03：Go 最小终端后端：检测、ConPTY 输入输出、尺寸、退出清理（app.go、internal/terminal）。
- [x] T04：React/shadcn 验证界面（frontend/，含 6 个前端测试）。
- [x] T05：隔离终端集成测试与全部门禁：Go 6 测试 + 前端 6 测试 + 构建，全绿。
- [x] T06：桌面 exe 打包成功（11.9 MB）；双 Agent 启动已做版本冒烟，交互验收项见 work/verification.md 待人工执行。
- [x] T07：README、验证记录已落盘。

首版待实现：跨工作区发现/恢复、配置安全合并/还原、安装归属、安全卸载、安装包。不得将验证按钮视作完整 Agent 适配。

## 会话发现与恢复（T08 批次）

- [x] T08a：Claude 适配器——projects 扫描（跳过 agent-*.jsonl）、摘要标题优先、running 会话并入（pid 存活校验）。
- [x] T08b：Codex 适配器——rollout 扫描、session_meta/turn_context 解析、注入消息过滤。
- [x] T08c：ResumeSession（原工作目录 + 唯一终端 ID）与 ListSessions 桥接；Go 8 测试 + 前端 7 测试全绿。
- [x] T08d：真实恢复验收（自动化 GUI 测试完成，2026-10-24）：P1–P6 全通过，见 work/verification.md 与 work/test-artifacts/p1–p6 截图。
- [x] T08e：多终端并存——后端 sessions map（上限 8）按 ID 查表路由；前端多标签（TerminalTabs/TerminalView），会话重复点击聚焦已开标签；顺带修复 L01 写路由缺陷。Go 8 测试 + 前端 7 测试 + 双构建全绿。
- [x] T09b：UI 定稿（v4 验收通过：夜航琥珀 chrome + Tokyo Night 终端、浅色主题、Agent 筛选胶囊）。正式版已同步：styles.css 令牌、theme.ts 三态主题、Tabs/筛选/会话徽标、statusbar、快捷键。

## 配置安全合并/还原（T10 批次，契约见 docs/config-contract.md）

- [x] T10a：core/configuration 纯用例——Plan（幂等/冲突行/校验拒绝）、Execute（stale_plan 前置重查、备份先于写入、blocked 拒绝）、O/A/C 三方还原决策与编排。10 个测试全绿（gofmt/vet/test/build 全过）。契约修订：BackupStore 增加 oAbsent 标志。
- [x] T10b：adapters/configuration WT JSONC 字节级手术编辑 + fixture 测试。8 个测试全绿：目标 span 外逐字节保留、注释/尾逗号保留、二次应用幂等、同名不同 GUID 冲突、受管 Profile 读/删、缺失标量、缺失文件创建。契约修订：Editor 增加 Check(desired)（冲突判定需期望值），值一律为原始 JSON 字面量。文件按 400 行熔断拆为 wt.go(163)/wt_locate.go(267)/wt_edit.go(150)/scanner.go(204)。
- [x] T10c：PowerShell Profile 管理块编辑器（ps.go）+ 5 个隔离测试：Plan/幂等/移除、Starship 与 oh-my-posh 冲突拒写、UTF-8 BOM/CRLF 与块外字节保留、异常 marker/编码拒写、缺失文件创建。核心用实际 PSReadLine 历史预测块替换 present 占位；会话入口命令仍待定义，不宣称已实现。scripts/check.ps1 全过（2026-10-25）。
- [x] T10d-1：持久化操作记录与中断恢复核心（2026-10-25）：记录 schema/PlanID/状态/步骤与 O/A/缺失标志；备份 pending 先于写入、写后复读、逐步落盘；重开扫描标 interrupted；按现状生成独立还原计划，冲突保留当前值；Windows 文件锁隔离操作与恢复扫描。4 个新增测试（含写前/写后中断子用例），scripts/check.ps1 全过。全部使用临时目录，未接真实配置。
- [x] T10d-2：桌面入口注入存储与启动扫描、服务端单次计划与桥接、配置面板（WT 主题 / PS 历史预测，预览 → 应用 → 操作记录 → 还原）。隔离桥接集成测试覆盖双目标应用还原和计划重放拒绝；真实 exe UIA 验收通过，见 work/test-artifacts/configuration-desktop.png 与 configuration-desktop.ps1。配置写入期间阻止窗口退出；SHUTTLE_CONFIG_TEST_ROOT 同时隔离三类路径。其余配置字段的表单、故障注入与多尺寸/DPI 验收尚未完成，不能据此认定整个 T10 首版范围已交付。
- [x] T10e：修复终端关闭偶发卡死（L04）——Close 自持进程句柄，ClosePseudoConsole 后限时 1.5s，超时 TerminateProcess 再有界等待。新增 TestCloseIsBoundedAndTerminatesChild；60 次压力复现由偶发失败转为 200 次全通过；check.ps1 连续 5 次全绿；实机启动终端 → Ctrl+Shift+W 关闭 → 退出应用无残留。
- [x] T10f（收尾）：配置字段表单补齐、写入/落盘失败注入、多尺寸与 DPI 几何验收（2026-10-25）。
  - 表单：新增 `ConfigFields.tsx`（基础字段 / 受管 Profile / 快捷键行 / 额外标量），`ConfigurationPanel.tsx` 改为复用，保留可见校验提示（额外标量格式、快捷键 JSON）。快捷键行间距由 8px 修正为 12px（几何安全线）。
  - 失败注入（core，5 个新增测试）：写入失败（后续目标不写、已写保留、状态 partially_failed、失败行留下可还原备份）、备份落盘失败（该目标不写、其余继续）、步骤记录失败（立即中止，避免写出无法还原的配置）、写后验证不一致（"写后验证失败"上抛）、收尾落盘失败（不得报告成功）。
  - 失败注入（adapters，2 个新增测试）：`failingStore` 注入真实文件存储的单目标落盘失败，重开记录仍为 partially_failed 且步骤错误持久化；记录根目录不可写时整笔操作中止且配置文件逐字节未动。
  - 几何：`work/test-artifacts/geometry-check.cjs` 无头 Chromium + CDP 在 1080×720 / 1440×900 / 1920×1080 × 100%·125%·150% 共 9 组测量（shell 控件、确认弹窗前后的配置面板、对话框内每个 flex 容器的兄弟间距、裁剪与重叠），全部通过，最小兄弟间距 12px，报告见 `geometry-report.json` 与 21 张截图。
  - 真实窗口：`geometry-small.ps1` 用 UIA 驱动真实 exe，按 DIP 复现 1080×720 屏幕在 100%/125%/150% 下的可用视口（1080×720 / 864×576 / 720×480 DIP），三者 horizontalOverflow=0；最窄 720×480 下配置面板同样无横向溢出；截图 `window-1080x720-*.png`、`window-720x480-config.png`。
  - 修复实机缺陷：`main.go` 的窗口下限原为 960×600 DIP，会把窗口钳制得比 1080×720 屏幕在 125%/150% 缩放下还大；改为 720×480 后三档请求均按原值生效（`geo-minsize.ps1`）。
  - 门禁接入（2026-10-25）：`scripts/check.ps1` 在前端构建后调用 `work/test-artifacts/geometry-check.cjs`，13 组几何（含 4 组极端窄视口）任一失败即退出码 1；同时新增截图非空白校验（PNG 采样色数 <2 判失败）。负向验证：把快捷键行 `gap-3` 改回 `gap-2`，门禁退出码 1 并报 `sibling gap < 12`；恢复后退出码 0。

**T10 批次收口结论（2026-10-25）**：表单、写入与落盘失败注入、多尺寸与 DPI 几何验收全部完成，几何检查已进门禁并有负向验证。仍未实现、不得计入已交付：Claude 状态栏预设（仅接口占位）、Shuttle 会话入口命令（未定义）。

## 安装归属与卸载（T11 批次，契约见 docs/inventory-contract.md）

状态：Type C Phase 4/5，契约 D1–D8 已冻结。T11a–T11e 已交付。

- [x] T11a：`core/inventory` 归属三档判定与只读卸载计划（2026-10-25）。D1–D3 已冻结（契约 §9）：台账落盘 `%LOCALAPPDATA%/Shuttle/install-ledger.json`、数据源仅 npm + winget、`path` 源单列且恒为 external。7 个测试全绿：三档判定、相似名不继承归属、台账损坏为 unknown 而非 external（与台账缺失=external 区分）、来源/路径不一致为 unknown、未支持来源、计划冲突阻断且无关项保留、空选择拒绝。
- [x] T11b：`adapters/inventory` 只读采集（2026-10-25）。**契约修订**：不再解析 `winget list` 文本表——实测其名称列与 ID 列会退化成一个空格（`Microsoft Visual C++ ... - 14.50.35719 Microsoft.VCRedist.2015+.x64`），且表头为本地化文字；改用 `npm list -g --json` 与 `winget export --include-versions` 两个结构化输出。覆盖缺口实测：export 只含 winget 源 65 个包，少于 list 的 79 个（缺 `Microsoft.Outlook`、`VCRedist` 等 ARP/MSIX 来源），已作为 `CoverageNote` 常量要求界面上报。7 个测试全绿：真实样本解析、空/垃圾输入、外部源过滤、逐源降级、只读校验（记录全部命令且禁止 install/uninstall/upgrade/remove）、超时。超时测试暴露真实缺陷：runner 阻塞时仅检查 ctx 不够，改为 goroutine + select 主动放弃等待。
- [x] T11c：桌面桥接与只读归属面板（2026-10-25）。`inventory.go`（package main）把 `Collect` + `Classify` + `PlanUninstall` 接成 `inventoryService`，只读、不执行任何安装/卸载；`App.DetectInventory` 30s 超时、两源各自降级、两源全失败才报错；`App.PlanInventoryRemoval` 同样只返回计划。前端 `InventoryPanel` 列出包名/版本/来源/归属/原因，`external` 与 `unknown` 勾选框禁用并标「不可卸载」，顶部固定显示 `CoverageNote`，数据源不可用时红字显示该源错误而不是显示"0 个包"。验收证据：隔离台账的桥接测试（`inventory_test.go` 4 个）、13 组几何含面板（`geometry-*-inventory.png`）、真实 exe 面板截图（`inventory-720x480.png` / `inventory-1080x720.png`，UIA 882 个元素横向溢出 0）。**契约修订**：台账损坏一律 `unknown`（不是草案写的 `external`），草案 §6/§7/§9 已同步。
- [x] T11d-1：台账 schema 2 读写、v1 兼容、四态→归属映射（2026-10-25，契约 §10.2/§10.3，测试 9、10）。`core.Record` 扩为契约字段（`State`/`OperationID`/`EvidenceBefore`/`EvidenceAfter`/`InstallerExit`/`InstallerError`/`InstalledAt`，版本只作展示）；`Classify` 先比名称+来源+根，再按 `State` 收口：`installed`（及 v1 无状态行）→ managed，`unverified`/`intent`/未知状态 → unknown，`failed` → external。适配器按 `schemaVersion` 分派：1 读入时补 `installed`，2 直读，其它版本判损坏→全量 unknown；写入恒为 2。新增 4 个测试（core 2 个、adapter 2 个），两个包各 9 与 12 个测试全绿，`scripts/check.ps1` 退出码 0。
- [x] T11d-2：写入协议 `intent` 先行、原子替换、写入失败即操作失败、调用方持 operations 锁（2026-10-25，契约 §10.5，测试 11、13、14）。`FileLedger.Put` 按名称 upsert（D4：一台机器一份当前状态，不存历史），复刻 `Save` 的临时文件 + Sync + Rename；`Save` 保留给播种与文档形状断言。**Put 先读后写**：台账读不出来（损坏/未知版本）时直接报错，绝不拿一条新记录覆盖掉整份旧台账——否则一次写失败会静默销毁所有历史归属。所有写错误带「台账」前缀，调用方可原样展示。新增 4 个测试：intent 崩溃后仍 unknown 且不可卸载、目录不可用时写失败且不留临时文件、未知版本台账不被覆盖（字节级比对）、20 个 goroutine 并发 Put 后每条记录都在且同名只留一条。`-count=5` 重复运行稳定，`scripts/check.ps1` 退出码 0。
- [x] T11d-3：`PlanInstall` 只读计划（含完整命令与手动说明）与 `VerifyInstalled` 复检（2026-10-25，契约 §10.6/§10.10 D8，测试 15、17、18）。`internal/core/inventory/install.go`：`PlanInstall(wanted, detected)` 只读，每步带 `Command` 与 `Instruction`（npm `npm install -g <name>` + npmjs 页；winget `winget install --id <id> --exact …` + 官方文档页）；已 `managed`/`external` 一律 `skip`（不重复安装），`unknown` 的 `skip` 并阻断，未检测到的才 `install`，不支持来源进 `Conflicts` 而不是整份计划报错。匹配键是名称+来源，同名跨源不互相干扰。`VerifyInstalled` 按 D6 三元组复检：名称+来源+根一致 → `installed` 且版本取自复检结果，否则 `unverified` 并写明证据；`intent` 的 `OperationID`/`EvidenceBefore` 原样保留。`Step` 增加 `Command`/`Instruction`，前端 `InventoryStep` 同步放宽（`Item` 对未检测项为空、新增 `Package?`）。5 个新测试全绿；前端类型检查与生产构建通过；`scripts/check.ps1` 退出码 0。
- [x] T11d-4：`ExecutePlan` 安装侧、`stale_plan`、启动核对（2026-10-25，契约 §10.5/§10.6/§10.7，测试 12、16）。`internal/core/inventory/execute.go`：`ExecuteInstall` 固定顺序 intent → 安装器 → 复检 → 终态，每步落盘后才进行下一步；执行走 `Args` 参数数组而非 shell 字符串（作用域包名不会被重新解释），`Command` 只用于展示。`stale_plan`：执行前用 `PlanSource` 重新生成计划，签名不一致即拒绝且零副作用。安装器失败但复检到包 → `unverified`（不是 `installed` 也不是 `failed`）；复检本身失败 → `unverified` 并写明原因；终态写台账失败 → 操作失败且持久台账停在 `intent`。`ReconcileInterrupted` 启动时只按当前检测收口 `intent`，从不重试安装。`Ledger` 接口增加 `Put`/`List`，`FileLedger.List` 实现。8 个测试全绿；`scripts/check.ps1` 退出码 0。

- [x] T11e：桌面安装闭环（2026-10-25，已交付）。`App.PlanInstall/ExecuteInstall` 接单次预览；默认手动指引，自动模式展示命令后确认；`install_runner.go` 直接 node/npm CLI 或 winget argv，不拼 shell。操作历史先落盘，台账 intent 先于安装器，持有 operations 文件锁，安装期间阻止退出。无 intent 不运行启动采集；中断 intent 统一 unverified，不因包存在就认领。失败数据源阻止自动安装。显式桥接 DTO 修复匿名嵌入/Wails 保留字问题。隔离桥接/runner/core 测试与 `scripts/check.ps1` 已通过（退出码 0），前端 7 测试通过；13 组几何含真实展开预览，最小间距 12px。实机预览验收通过（`work/verification.md` T11e 桌面预览行）：UIA 驱动 `work/test-artifacts/install-desktop.ps1` 退出码 0，四组件预览行断言通过，720×480 与 1080×720 无横向溢出并截图（`preview-install-720x480.png`/`preview-install-1080x720.png`），隔离台账未被预览创建。`确认并执行安装` 全程未点击；未执行真实安装、卸载或付费模型调用。

契约已冻结（2026-10-25）：D4 每包一条当前状态、D5 `intent` 先行、D6 复检三元组一致且版本不参与归属、D7 仅 npm + winget、D8 手动优先且自动安装须先展示完整命令。四张卡单卡 ≤300 行，按 T11d-1 → 4 顺序推进。

## 新需求迭代（2026-10-25 需求分析后逐项开工）

来源：用户四项新需求（标签批量操作 / 终端配色预设 / 侧栏折叠 / 中英双语），经 luban-craft 判定：需求3 Type B、需求1 Type B、需求2 Type B、需求4 Type C（需先冻结 i18n 契约）。

- [x] 需求3 侧栏折叠（2026-10-25）。Ctrl Shift B + 头部 PanelLeft 按钮，localStorage 持久化（与主题同路径）；折叠为整体隐藏，13 组几何含折叠态断言全 PASS。
- [x] 需求1 标签右键批量操作（2026-10-25）。右键标签弹菜单：关闭/关闭其他/关闭右侧/全部关闭（无操作项不显示）；App.closeMany 统一单关与批量：运行中标签逐个确认、任一取消即整批中止、焦点落最近存活标签；复用新建菜单手写样式。真机验证含取消中止路径，证据见 verification.md。
- [x] 需求2 终端配色预设（2026-10-26）。8 套深浅色官方方案（Tokyo Night / Day, Catppuccin Mocha / Latte, Dracula, Nord, Solarized Dark / Light）+ 跟随应用主题；头部 PalettePicker（12px 间距，9 项选项）；localStorage 持久化，显式选择锁定不随应用主题翻转；13 组几何 PASS；真机 UIA 切换生效。
- [x] 需求4 中英双语（Type C，契约见 docs/i18n-contract.md，2026-10-27 交付）：
  - [x] Card 1: i18n 核心引擎、字典表与对称性测试（frontend/src/lib/i18n/ + frontend/tests/i18n.test.ts，15/15 全绿，178 keys 零差异）
  - [x] Card 2: Header 语言切换器 LangSeg、App 挂载与测试脚本锚定（geometry-check.cjs 保证 13 组几何全绿，最小间距 12px）
  - [x] Card 3: 全站 13 个组件静态文案双语映射与真机验收（真机 UIA 切换验证通过，双语截图 verify-language-zh.png / verify-language-en.png / verify-language-zh-restored.png 留盘）
- [x] 外部 Windows Terminal 联动（2026-10-27，Type B）。侧栏各工作区标题提供快捷外部打开按钮（ExternalLink 图标，中英 title/aria-label，支持 stopPropagation 不触发折叠）；Go 桥接 OpenInWindowsTerminal 校验合法目录并通过 LookPath 启动 wt.exe -d <workspace>（异步非阻塞且不接管外部进程）；Go 单元测试覆盖路径校验与参数、frontend 15 单元测试全绿；check.ps1 13 组几何全绿；真机 UIA 截图 verify-wt-sessions.png 留盘。

## 安全卸载执行与台账回滚闭环（T11 终阶段，Type C，契约见 docs/uninstall-contract.md）
- [x] Card 1: 卸载计划命令行生成与台账删除能力（PlanUninstall 产出 Command/Args，FileLedger.Delete，核心+适配器测试）
- [x] Card 2: 卸载执行核心用例与过期复核（ExecuteUninstall、三元复检判定、stale_plan、core/inventory 测试）
- [x] Card 3: 桌面桥接、前端卸载预览/执行面板与隔离实测（installs.go、InventoryPanel.tsx、UIA 驱动实测验收 verify-removal.ps1 / verify-removal-preview.png）

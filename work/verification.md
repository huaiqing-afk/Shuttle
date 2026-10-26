# 终端技术验证记录

环境：Windows 11；Claude Code 2.1.310；Codex 0.164.0；Node 24.16；WebView2 已安装。真实进程证据以本表为准，禁止复制为宣传结论。

## 已自动验证

| 项目 | 结果 | 证据 |
|---|---|---|
| Claude shim 经 pwsh -CommandWithArgs 启动 | 通过 | 输出版本 2.1.310 |
| Codex shim 经同路径启动 | 通过 | 输出版本 codex-cli 0.164.0 |
| ConPTY Unicode 往返/尺寸/退出 | 通过 | go test：中文输入往返、非法尺寸拒绝、活动进程关闭、重复 Close 幂等 |
| Agent shim 启动测试（--version） | 通过 | app_test.go：claude 2.1.310、codex 0.164.0 |
| Go 门禁（fmt/vet/test/build） | 通过 | 6 tests, 2 packages |
| 前端门禁（test/typecheck/build） | 通过 | 6 tests；dist 构建 286ms |
| Wails 桌面打包 | 通过 | build/bin/Shuttle.exe 11.9 MB（wails build v2.17.0，Go WebView2Loader） |

## 待人工验证（需启动桌面程序）

- [ ] 三按钮真实交互终端：中文输入法输入、粘贴、颜色、Ctrl+C。
- [ ] 窗口拉伸时 fit+Resize 无错行；字体缩放。
- [ ] 退出确认对话框；取消保留；确认后进程消失（任务管理器无残留）。
- [ ] Claude/Codex 完整交互登录态（现有账号）；Claude Code 斜杠菜单、箭头键。

## 已自动验证（T08 会话发现批次）

| 项目 | 结果 | 证据 |
|---|---|---|
| Claude 解析（跳过 meta/agent 转录、摘要标题优先、pid 存活过滤） | 通过 | agents_test.go fixture |
| Codex 解析（session_meta/模型/注入消息过滤） | 通过 | agents_test.go fixture |
| Go 门禁 8 测试 + 前端门禁 7 测试 | 通过 | go test / node --test |

## T08 待人工验收

- [ ] 会话侧栏：本机真实会话出现在工作区分组下，标题/模型/时间正确。
- [ ] 点击 Claude 历史会话：在原工作目录接续（`claude --resume`）；Codex 同理（`codex resume <uuid>`）。
- [ ] 有终端运行时点击会话：出现确认；确认后旧终端终止、新会话启动。
- [ ] 开着 Claude 窗口时侧栏显示运行中圆点与 busy/idle 状态。

## 已自动验证（T08e 多终端 + UI 同步批次）

| 项目 | 结果 | 证据 |
|---|---|---|
| 后端多终端：sessions map 按 ID 查表，上限 8 | 通过 | app.go；launch 注册 / watcher 注销 |
| 事件按终端 ID 复用路由到对应标签 | 通过 | session.test.ts 双会话复用用例 |
| resume 不再终止其他终端 | 通过 | session.test.ts 断言 closed 为空 |
| 会话测试 mock 修正为多订阅者扇出（对齐真实 Wails 事件语义） | 通过 | session.test.ts setup() |
| 浅/深主题令牌 + 终端主题跟随 | 通过 | styles.css / theme.ts / TerminalPanel termThemes |
| Go 门禁 8 测试 + 前端门禁 7 测试 + 双构建 | 通过 | gofmt/vet/test/build、tsc、vite、wails build（Shuttle.exe 11.9 MB） |
| L01 写路由缺陷修复（"validation" 哨兵 → 查表） | 通过 | docs/LESSONS.md L01；app.go Write/Resize/Close |
| L02 退出确认修复：原生对话框返回值不可靠，改为前端自绘确认弹窗 + ExitApp 绑定 | 通过（用户实机确认） | app.go beforeClose/ExitApp；App.tsx 退出弹窗 |
| L02 第二层：shutdown 并行关闭 + 2 秒上限，防退出被 ConPTY 排空阻塞 | 通过（随退出路径一并确认） | app.go shutdown |
| L04 终端关闭有界化：自持进程句柄 + 1.5s 宽限 + TerminateProcess | 通过 | session_windows.go Close；TestCloseIsBoundedAndTerminatesChild；200 次关闭压力；实机 Ctrl+Shift+W 后子进程退出、退出应用无残留 |

## 已自动验证（T10f 配置收尾批次，2026-10-25）

| 项目 | 结果 | 证据 |
|---|---|---|
| 配置表单字段（主题/字体/默认 GUID/受管 Profile/快捷键/额外标量/PS 历史预测）与可见校验 | 通过 | frontend typecheck + 7 测试 + 生产构建；ConfigFields.tsx |
| 写入失败注入（后续不写、已写保留、partially_failed、可还原备份） | 通过 | core operations_failure_test.go TestWriteFailureKeepsEarlierWritesAndRecords |
| 备份落盘失败 / 步骤记录失败 / 写后验证不一致 / 收尾失败 | 通过 | 同文件其余 4 个测试 |
| 真实文件存储单目标落盘失败、记录根不可写时不写配置 | 通过 | adapters operations_failure_test.go（临时目录，未触真实配置） |
| 几何 9 组：1080×720 / 1440×900 / 1920×1080 × 100%·125%·150% | 通过 | geometry-check.cjs → geometry-report.json；无横向滚动、无重叠、最小兄弟间距 12px |
| 几何 4 组极端窄视口：864×576 / 720×480 / 720×448 / 640×400 | 通过 | 同上，共 13 组全通过 |
| 真实 exe 窗口 1080×720 @100%/125%/150%（864×576、720×480 DIP） | 通过 | geometry-small.ps1 → uia-small-report.json；horizontalOverflow=0；window-1080x720-{100,125,150}.png |
| 真实 exe 最窄窗口 720×480 下的配置面板 | 通过 | 同上 720x480-config；window-720x480-config.png |
| scripts/check.ps1 全绿 | 通过 | gofmt/vet/test/build + typecheck + vite build + 13 组几何，退出码 0 |
| 几何检查已进门禁（负向验证） | 通过 | 临时把快捷键行 `gap-3` 改回 `gap-2` 后 check.ps1 退出码 1，报 `sibling gap < 12: gap 8`；恢复后退出码 0 |
| 截图非空白校验 | 通过 | geometry-check.cjs 解析 PNG 并统计采样颜色数，<2 色判失败；本轮各组 198–550 色 |

### 实机窗口下限修复（本次发现并修复）

原先 `main.go` 的 `MinWidth: 960, MinHeight: 600`（DIP）会把窗口钳制到大于屏幕：1080×720 屏幕在 125% 缩放下只有 864×576 DIP，在 150% 下只有 720×480 DIP。实测证据：请求 864×576 / 720×480 均被钳成 960×600。已改为 720×480 并按新二进制复测，三档请求均按原值生效（`work/test-artifacts/geo-minsize.ps1` 输出的 ok 三行）。

## T10f 未覆盖与已知限制

- 本机两块显示器均为 200% 缩放（物理 3072×1920 / 3840×2160），无法在真实窗口中呈现 100% / 125% / 150% 的**系统缩放**；真实 exe 的证据是用 DIP 尺寸等价复现（1080×720@125% = 864×576 DIP 窗口），CSS 像素与缩放后的物理像素换算已验证一致。125%·150% 的 DOM 级间距证据来自无头 Chromium 的 CDP device metrics。
- 真实 exe 的 CDP 被 Wails 清空 `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` 阻断；AppCompat `HIGHDPIAWARE` 图层也无效（WebView2 自行声明感知级别，实测 `GetDpiForWindow` 仍为 192，CSS 尺寸不变）。故实机只能做 UIA 矩形与 `PrintWindow` 截图，无法在实机内遍历 DOM 测间距。
- 无头侧使用 bridge 桩（`window.go.main.App`）驱动面板；桩只提供文档声明的返回结构，不伪造终端能力，截图与真实 exe 的差异仅在会话列表与终端区域。
- Claude 状态栏预设仍为接口占位；Shuttle 会话入口命令仍未定义。

## 已自动验证（T11 安装归属批次，2026-10-25）

| 项目 | 结果 | 证据 |
|---|---|---|
| 归属三档判定 + 只读卸载计划 | 通过 | core/inventory 7 测试（含台账缺失=external / 损坏=unknown 的区分、相似名不继承、来源或路径不一致=unknown） |
| 只读采集：npm JSON + winget export | 通过 | adapters/inventory 7 测试；真实样本来自 fixtures/inventory；断言全程未执行 install/uninstall/upgrade/remove |
| 采集超时与 runner 阻塞 | 通过 | 阻塞 runner + 50ms ctx，采集在 0.05s 内返回并同时满足 `errors.Is(ErrSourceUnavailable)` 与 `context.DeadlineExceeded` |
| 台账文件适配器 | 通过 | ledger_test.go 3 测试：缺失→external、损坏→unknown（带原因）、保存/重载/嵌套目录/拒绝未知 schema |
| 桌面桥接（隔离目录） | 通过 | inventory_test.go 4 测试：临时台账下 claude-code=managed、codex=external，计划动作 uninstall/skip；失败数据源带错误、`CoverageNote` 到达 UI |
| 几何 13 组含归属面板 | 通过 | geometry-check.cjs 新增 install 面板测量，13/13 PASS，最小兄弟间距 12px、无横向滚动、截图 213–558 色 |
| 真实 exe 归属面板 | 通过 | inventory-desktop.ps1：实机检测到 npm 11 个包 + winget 65 个包、76 行「不可卸载」；720×480 与 1080×720 窗口 UIA 882 元素横向溢出 0；inventory-720x480.png、inventory-1080x720.png |
| scripts/check.ps1 全绿 | 通过 | gofmt/vet/test/build（7 包）+ typecheck + vite build + 13 组几何，真实退出码 0 |
| T11d-1 台账 schema 2 与 v1 兼容 | 通过 | ledger_test.go：v1 读入补 `installed` 并判 managed、schema 0/3/99 判损坏→unknown、保存恒写 2 且字段往返不丢 |
| T11d-1 四态→归属映射 | 通过 | inventory_test.go TestRecordStateMapsToOwnership（installed/空→managed，unverified/intent/bogus→unknown，failed→external）与 TestNonInstalledRecordsAreNotUninstallable（非 installed 一律 skip，unknown 阻断计划） |
| T11d-2 `Put` 级写入协议 | 通过 | ledger_write_test.go 4 测试：intent 崩溃后仍 unknown 且计划 skip；不可用目录写失败、无残留临时文件；未知版本台账字节级未被覆盖；20 goroutine 并发 Put 全部保留且同名只留一条（D4）。`-count=5` 稳定通过 |
| T11d-2 竞态检测 | 未执行 | 本机 `go test -race` 需 cgo，`CGO_ENABLED=0` 环境下不可用；并发正确性由 `-count=5` 重复运行覆盖，未做数据竞争检测 |
| T11d-3 `PlanInstall` 只读计划 | 通过 | install_test.go 4 测试：每步带完整命令与官方说明（npm/winget 分别断言前缀与文档域名）、已装（managed/external）一律 skip、unknown 才阻断、未检测到才 install、同名跨源互不干扰、不支持来源进 Conflicts |
| T11d-3 `VerifyInstalled` 复检 | 通过 | 同名同源同根→installed（大小写与尾斜杠不同也算一致）；检测不到/换源/换根→unverified 且写明证据；版本取自复检而非计划；intent 的 OperationID/EvidenceBefore 原样保留 |
| T11d-3 前端类型同步 | 通过 | types.ts 的 `InventoryStep` 增加可选 `Package`/`Command`/`Instruction`；tsc --noEmit 与生产构建通过 |
| T11d-4 安装执行顺序与终态 | 通过 | execute_test.go 8 测试：intent 先落盘（断言 `EvidenceBefore=absent`、`OperationID`、复检得到的版本与根）、安装器以 argv 数组调用、失败且不存在→failed、失败但存在→unverified、成功但未检测到→partially_failed、复检失败→unverified、台账写失败→操作失败且持久层停在 intent |
| T11d-4 `stale_plan` 与启动核对 | 通过 | 计划签名变化时拒绝执行且**零副作用**（安装器未调用、台账未写）；`ReconcileInterrupted` 只收口 intent（存在或不存在都为 unverified，缺少安装器成功证据），已定终态记录不被改写，不会把包升格为 managed。T11e 修正此前“存在→installed”的错误实现与断言 |
| T11e 安装桌面桥接 | 通过 | PlanInstall/ExecuteInstall/启动核对已接入；隔离台账与安装器测试验证一次性预览、stale 拒绝、操作记录、跨实例锁与退出检查点；真实安装另行验收 |

| T11e Windows 命令执行 | 通过 | `install_runner_test.go` 验证 npm shim→node CLI argv、缺失路径、输出64KiB上限、失败与超时；真实只读 npm `--version` 为11.13.0，未执行安装 |
| T11e 几何与门禁 | 通过 | 最终 `scripts/check.ps1` 退出码0；Go fmt/vet/test/build、前端typecheck/build、13组几何通过；`geometry-report.json` 13项均记录安装预览、至少2条命令、最小兄弟间距12px；前端测试另跑7/7通过 |
| T11e 桌面预览 | 通过 | 预览专用 UIA 驱动 `work/test-artifacts/install-desktop.ps1` 退出码0（2026-10-25）：真实启动 Shuttle.exe，隔离 SHUTTLE_CONFIG_TEST_ROOT，打开安装归属→检测完成→自动安装→勾选4组件→生成安装预览；按「包ID：动作」节点序列断言四行（Microsoft.PowerShell/Git.Git/@anthropic-ai/claude-code/@openai/codex 均为跳过·已存在），720×480 与 1080×720 DIP（scale=2）下 horizontalOverflow=0 且无可横向滚动容器，截图 `preview-install-720x480.png`(1440×960) 与 `preview-install-1080x720.png`(2160×1440) 落盘并经人工核看；断言安装台账未创建；`确认并执行安装` 全程未点击（ClickName 硬编码拒绝），未发生真实安装/卸载。环境修正记录：搜狗输入法在 Shuttle 进程内创建顶层窗口需按标题匹配主窗口；WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--force-renderer-accessibility 防止 a11y 树为空；点击前不做 MoveWindow（Chromium 命中测试异步刷新导致静默丢点击）；复选框漏点由「重新生成预览+缺失重勾」自校正；侧栏 truncate 省略号与 WebView2 D3D 中间层旧矩形为断言误报已排除 |

## 已自动验证（新需求迭代批次，2026-10-25）

| 项 | 结论 | 证据 |
|---|---|---|
| 侧栏折叠 | 通过 | Ctrl Shift B 与头部按钮切换，localStorage 持久化；13 组几何各增加折叠态断言（aside 隐藏、无横向溢出、无重叠）全 PASS；实机截图 `verify-sidebar-collapsed.png`/`verify-sidebar-expanded.png` |
| 标签右键批量关闭 | 通过 | 真机 UIA 驱动（脚本已清理，日志留存于本行描述）：3 标签右键菜单四项（关闭/关闭其他/关闭右侧/全部关闭）齐全并截图 `verify-tabs-menu.png`；Escape 关闭菜单；关闭其他→确认对话框×2 均接受→余 1 标签；关闭右侧→接受×2→余 1；运行中标签「关闭其他」点取消→整批中止、2 标签保留。门禁 `scripts/check.ps1` 退出码 0（13 组几何 PASS） |
| 终端配色预设 | 通过 | 8 套深浅色官方方案（Tokyo Night / Day, Catppuccin Mocha / Latte, Dracula, Nord, Solarized Dark / Light）+ 跟随应用主题；头部 PalettePicker（12px 间距，9 项选项）；localStorage 持久化，显式选择锁定不随应用主题翻转；13 组几何 PASS；真机 UIA 切换生效并截图 erify-palette.png / erify-palette-dracula.png |
| 中英双语 (i18n) | 通过 | 零外部依赖手写轻量 i18n 引擎；中英字典 178 key 双向 100% 对称；前端单测 15/15 全绿（`frontend/tests/i18n.test.ts`）；Header 增加 `LangSeg` 切换器（12px 间距，`#app-language`）；存量脚本通过 `localStorage['shuttle-language'] = 'zh-CN'` 保证 100% 兼容；13 组几何测试全绿（最小间距 12px，极端 640×400 视口无溢出）；全站 13 个业务组件文案替换完成；真机 UIA `verify-language.ps1` 驱动 `build/bin/Shuttle.exe` 动态切换验证通过，双语截图 `verify-language-zh.png` / `verify-language-en.png` / `verify-language-zh-restored.png` 留盘 |
| 外部 Windows Terminal 联动 | 通过 | 侧栏工作区标题集成 `ExternalLink` 快捷外跳按钮，支持中英双语无障碍提示（`sessions.openInWT`），`stopPropagation` 防止误折叠；Go 桥接 `OpenInWindowsTerminal` 严格校验目录与 `wt.exe` 存在性并异步启动；Go 单元测试（空目录/不存在目录/目录校验通过）与前端 15 单元测试全绿；门禁 `scripts/check.ps1` 13 组几何测试全绿；真机 UIA 展开侧栏截图 `verify-wt-sessions.png` 留盘 |

### 新需求批次待人工验收

- [ ] 折叠状态跨重启恢复（localStorage 同路径，未机器验证）。
- [ ] 折叠后 Ctrl Shift K 会话搜索入口随侧栏隐藏，属于全隐藏形态的固有取舍，待用户确认是否可接受。

### T11 待人工验收

- [ ] 在真机上装一次由 Shuttle 记录的包后，面板应显示 `Shuttle 安装` 并允许勾选（当前台账无 `managed` 记录，实机全部为 `external`，managed 分支仅有测试覆盖）。
- [ ] winget 不可用的机器上，winget 源应红字显示失败原因而不是「0 个包」。

## T11 未覆盖与已知限制

- 安装执行已接桌面，卸载仍只预览；真实安装/权限提升/真实卸载尚未验收。安装命令超时为 15 分钟，只保证终止直属进程，不保证递归终止派生安装器。
- 自动安装要求 npm 与 winget 两源检测均成功；单源失败仍可查看库存，但不能自动安装。WebView2 本批仅提供官方手动指引。
- 归属判定以「名称 + 来源 + 安装根」三者比对；同机多版本共存、包改名后的重新归属未处理。
- winget 覆盖缺口（export 只含 winget 源 65 个，本机 list 为 79 个）以 `CoverageNote` 常显，不做 ARP/MSIX 直读。

## T08e 待人工验收（启动 build/bin/Shuttle.exe）


- [x] 多标签并行：Ctrl T 开多个终端同时运行，输出互不串扰。→ P3b 自动化验证（3 个 PowerShell 标签并行，坐标点击 + 截图）。
- [x] 标签状态点颜色区分 Claude/Codex/PowerShell；启动中琥珀加载条。→ P3b/p5 截图确认。
- [x] 左栏点历史会话开新标签；重复点击聚焦已开标签并显示对勾。→ P5 自动化验证（点击"讨论一个需求"行，claude --resume 在原工作目录 D:\harry\youchui\goplay 接续，行上出现对勾）。
- [x] 主题三态：跟随系统/深色/浅色，浅色下终端切 Tokyo Night Day。→ P4 自动化验证（UIA 点击"深色"，p4-dark.png）。
- [x] 快捷键：全局快捷键为 Ctrl+Shift+T（新终端）/ Ctrl+Shift+W（关闭）/ Ctrl+Shift+K（搜索）/ Ctrl+Tab 循环切换——焦点在终端内时普通 Ctrl+字母必须归终端（自动化发现 Ctrl+T 变 ^T 进入 pwsh 的缺陷后修正）；Alt+数字与截图工具全局热键冲突，已移除。
- [x] 退出 Shuttle：多终端运行时确认框列出数量，确认后全部终止（无残留）。→ P6 自动化验证（WM_CLOSE → 应用内弹窗出现；UIA 点击"退出并终止"→ 进程 2 秒内退出；孤儿进程扫描仅剩 Codex 自身常驻 daemon，非 Shuttle 残留）。
- [x] 真实输入回归：中文输入、exit（对应 L01 写路由修复）。→ P2 自动化验证（剪贴板粘贴中文+命令，pwsh 正确执行；IME 直接 SendKeys 会受中文输入法干扰，测试改用粘贴）。

## 已知限制（T08e 自动化测试遗留）

- 标签运行中关闭确认仍用 window.confirm（P6 未覆盖单标签关闭路径）。
- Codex 历史会话恢复未自动化实测（仅 Claude）；Codex 解析有 fixture 测试覆盖。

## 已知限制（T08）

- Codex 无运行状态数据源，一律显示灰点（不伪造 busy/idle）。
- Claude 大转录全量扫描，历史库达到数百 MB 后需换尾部采样（代码已留 shortcut 注释）。
- resume 从磁盘重扫定位工作区，会话量大时点击有可感知延迟。

- npm .cmd shim 经 pwsh 包装：依赖用户 PATH；原生 exe 渠道后续再验证。
- 标签关闭确认与全局退出确认目前用系统对话框（window.confirm / Wails MessageDialog），设计稿中的自绘弹窗未做。
- 浏览器/WebView 中 Ctrl W 的 preventDefault 不保证生效，原生快捷键冲突需实机确认。
- wails dev 热重载未验证；Wails v2.17.0 + WebView2 组合待实机跑。

## 安全卸载执行闭环验收（T11 终阶段，2026-10-27）

- [x] **Card 1 卸载计划与台账删除能力**：
  - `PlanUninstall` 产出完整 `Command`（npm uninstall -g / winget uninstall --id）与参数数组 `Args`，无 shell 拼接。
  - `FileLedger` 实现 `Delete(name string) error`，临时文件原子替换，幂等安全。
  - 适配器与核心单元测试覆盖删除能力。
- [x] **Card 2 卸载执行核心用例与过期复核**：
  - `ExecuteUninstall` 严格闭环：`intent` 优先落盘 -> 卸载器执行 -> 复检比对三元组 -> 成功则物理删除台账行 -> 失败或不彻底保留并记录 `unverified`。
  - `planSignature` 重构覆盖全动作类型；`assertFresh` 拦截过期与状态漂移（`stale_plan`）。
  - `internal/core/inventory` 37/37 单元测试全绿。
- [x] **Card 3 桌面桥接、前端面板与隔离实测**：
  - 桥接层 `App.PlanInventoryRemoval` 签发带 TTL 的单次 Token，`App.ExecuteInventoryRemoval` 持 operations 文件锁执行并在 `operations/` 留下完整审计。
  - 修复 `ExecuteInventoryRemoval` 中复检 snapshot 动态委托，确保安装器执行后检测结果实时生效，`installs_test.go` 8 个测试全绿。
  - 严格保持 400 行熔断规则：`installs.go` 为 389 行（<=400 行）。
  - 前端 `InventoryPanel.tsx`（153 行）实现卸载预览卡片、命令展示与「确认并执行卸载」按钮；12px 几何安全线合规。
  - `zh.ts` 与 `en.ts` 补充卸载执行文案，保持 100% 对称性。
  - `scripts/check.ps1` 门禁全绿（Go fmt/vet/test 124 组/build，前端 typecheck/build，13 组视口与 DPI 几何安全线全 PASS）。
  - UIA 实机自动化验收：`work/test-artifacts/verify-removal.ps1` 驱动 `build/bin/Shuttle.exe`，在临时隔离环境验证受管包勾选、生成卸载预览与命令展示，截图 `work/test-artifacts/verify-removal-preview.png` 留盘。未自动执行真实卸载，保护宿主环境。

# 安装归属与卸载契约（T11 批次）

状态：Type C Phase 2 契约 v1，D1–D3 已冻结（§9），T11a–T11c 已交付。执行卸载仍不在本批次范围。T11d 为第二阶段，契约已冻结（D4–D8，见 §10）。
依据：docs/architecture.md §1 已确认范围、§5「卸载安全顺序」、§10 验收清单中三条与归属相关的条目。

## 1. 本批次解决的问题

Shuttle 要区分"这台机器上已有什么"与"Shuttle 自己装了什么"，否则无法安全卸载。本批次只做**只读归属检测**与**计划生成**，不执行任何卸载、不写入安装台账的真实位置。

## 2. 已实测的数据源（2026-10-25，本机）

| 来源 | 命令 | 实测结果 | 延迟 |
|---|---|---|---|
| npm 全局（采用） | `npm list -g --depth=0 --json` | 11 个全局包，含 `@anthropic-ai/claude-code@2.1.310`、`@openai/codex@0.164.0`；版本独立成字段 | <1s |
| npm 根目录 | `npm root -g` | `C:\Users\admin\AppData\Roaming\npm\node_modules` | <1s |
| winget（采用） | `winget export -o <path> --include-versions` | JSON，含 `PackageIdentifier` 与 `Version`；**仅覆盖 winget 源 65 个包** | ~7s |
| winget 文本（已弃用） | `winget list` | 固定宽度表，79 行 winget 源；长名称行的名称与 ID 列会退化成一个空格 | ~6.7s |
| 可执行文件 | `where.exe <name>` | `claude`、`codex` 均落在 `AppData\Roaming\npm` | <1s |

本机没有 scoop、choco。两者不在首版数据源内。

**解析方式修订（实测后）**：原计划解析 `winget list` 文本表，实测有两处硬伤。其一，长名称行的名称列与 ID 列之间会退化成一个空格（如 `Microsoft Visual C++ v14 Redistributable (x64) - 14.50.35719 Microsoft.VCRedist.2015+.x64`），按空格切分必然错位；其二，表头是本地化文字（本机 `名称`/`ID`/`版本`/`可用`/`源`），不能作为定位依据。改用 `winget export` 的结构化 JSON，顺带拿到版本号。

**覆盖缺口（必须上报）**：`winget export` 只导出能关联到 winget 源的包，实测 65 个，`winget list` 为 79 个；缺少的包括 `Microsoft.Outlook`、`Microsoft.VCRedist.2015+.x64/.x86` 等 ARP/MSIX 来源项。因此**不得**把"导出结果里没有"表述为"这台机器没装"；该限制以 `CoverageNote` 常量随结果一并展示。

## 3. 归属判定规则（已冻结）

三档结论，不允许"疑似"：

| 结论 | 判定条件 | 是否可进卸载清单 |
|---|---|---|
| `managed` | 本机安装台账中有记录，且记录的安装方式与当前实际一致 | 是 |
| `external` | 有检测到，但台账无记录 | **否** |
| `unknown` | 检测到多个来源指向不同安装，或来源不在已支持列表 | **否** |

关键取向：**默认不归 Shuttle 所有**。未检测到台账记录一律 `external`，不因"看起来像 Shuttle 装的"而推定归属。

## 4. 核心接口草案

```go
package inventory // core/inventory

type Source string // "npm" | "winget" | "path"

type Package struct {
    Name    string // 规范化名，如 "@anthropic-ai/claude-code"
    Version string
    Source  Source
    Root    string // npm 全局根、winget ID 或可执行文件目录
}

type Ownership string // "managed" | "external" | "unknown"

type Item struct {
    Package
    Ownership Ownership
    Reason    string // 判定依据，面向用户展示，禁止留空
}

type Ledger interface { // 安装台账；本批次只定义，用内存实现
    // 第三个返回值区分"没有记录"与"台账读不出来"：前者判 external，后者判 unknown
    Lookup(name string) (record Record, ok bool, err error)
}

func Detect(ctx context.Context) ([]Package, error)        // 只读
func Classify(pkgs []Package, ledger Ledger) []Item        // 纯函数
func PlanUninstall(items []Item, choice Choice) (Plan, error) // 只读；冲突项阻止
```

不做的部分（本批次明确排除）：真正执行卸载、下载/安装、版本升级、依赖图解析、注册表 ARP 直读、scoop/choco。

## 5. 安全顺序（承接 architecture §5）

1. 生成还原 + 依赖检查计划，用户选择范围。
2. 先修复将引用待卸载组件的默认 Profile / 启动命令 / 受管配置。
3. 冲突未解决则阻止卸载该组件，不阻塞无关项。
4. 只处理 `managed` 且被选中的项；不级联卸载共享依赖。
5. 重新检测保留的 shell 与配置引用，记录结果。

本批次实现第 1、3、4 步的只读部分；第 2、5 步依赖配置写入，与 T10 已交付能力对接，但仍不执行真实卸载。

## 6. 故障与降级

- 数据源命令缺失（无 npm / 无 winget）：该源返回空并记录 `Reason`，不阻止其他源；整体不得报"一切正常"。
- 命令超时：`winget` 给 30s 上限，超时即该源标记失败并展示，不重试超过一次。
- 输出无法解析：该源整体判失败，不猜测半截结果。
- 台账损坏：所有项降级为 `unknown`（不是 `external`），并在 `Reason` 中写明台账不可用。缺失 ≠ 损坏：从未写过台账说明 Shuttle 没装过东西，缺失按"无记录"判 `external`；文件存在却读不出，说明归属证据丢了，此时既不能声称 Shuttle 装的，也不能声称用户自己装的，只能要求人工核对。

## 7. 测试矩阵

| # | 用例 | 层 |
|---|---|---|
| 1 | npm JSON 解析（正常、空、含 scoped 包、垃圾输入） | adapter fixture |
| 2 | winget export 解析（真实样本、非 winget 源过滤、垃圾输入、版本保留） | adapter fixture |
| 3 | 归属三档判定矩阵 | core 纯函数 |
| 4 | 未检测到台账记录一律 external（不得因名称相似推定） | core 纯函数 |
| 5 | 台账缺失（全量 external）与台账损坏（全量 unknown）分开断言 | core |
| 6 | 命令缺失与超时的降级行为 | adapter（注入失败 runCommand） |
| 7 | 计划生成：冲突项阻止、无关项保留 | core |
| 8 | 检测全过程只读（同一输入跑两次，磁盘无变化） | adapter 集成 |

## 8. 任务卡

| 卡 | 内容 | 验收 |
|---|---|---|
| T11a | `core/inventory`：Package / Item / Ownership、Classify 纯函数、PlanUninstall 只读计划 | 测试 3、4、5、7 通过 |
| T11b | `adapters/inventory`：npm 与 winget 只读采集 + 解析 + 降级 | 测试 1、2、6、8 通过 |
| T11c | 桌面桥接与只读面板（归属列表 + 原因 + 不可卸载标记） | 隔离目录集成测试 + 实机截图 |
| T11d | 安装台账落盘格式与"应用安装"入口（第二阶段） | 契约已冻结（§10，D4–D8）；验收为测试 9–18 |

## 9. 分叉决策（已冻结 2026-10-25）

- **D1 台账位置 = 落盘** `%LOCALAPPDATA%/Shuttle/install-ledger.json`，与 `operations/` 同级。理由：归属结论必须可审计、可撤销；"每次从已装包反推"无法区分"Shuttle 装的"与"用户自己装的"，正是本批次要解决的歧义。台账损坏或缺失时不得推定归属：缺失照样判 `external`（无记录即非 Shuttle 所装），损坏一律降级为 `unknown` 并显式提示，不猜。
- **D2 数据源范围 = 仅 npm + winget**。本机无 scoop/choco，无法取得真实输出样本；按文档猜写解析器属于用假数据冒充真实能力，违反 AGENTS.md。后续版本在装有二者的环境采样后再加，不改本批次的 `Source` 取值集合语义（新增值即新增契约条目）。
- **D3 `path` 源 = 单列，但一律 `external`**。`where.exe` 只能证明"PATH 上能找到"，不能证明来源与安装归属；单列的价值是让用户看到"这个东西确实存在"，因此它永远不进卸载清单，`Reason` 固定写明这一点。

## 10. 第二阶段：安装台账落盘与"应用安装"入口（T11d，已冻结 2026-10-25）

状态：契约已冻结（D4–D8 见 §10.10），进入 Phase 3 拆解。上一节 D1–D3 已冻结，本节不得与之冲突。执行卸载仍不在本阶段范围。

### 10.1 范围边界

交付：台账的真实落盘格式与读写、安装意图/结果的记录、`PlanInstall` 与 `ExecutePlan` 的安装侧只读计划 + 受控执行，以及启动时的未完成操作核对。

明确不做（各自需单独契约）：真正执行卸载、版本升级、依赖图解析、注册表 ARP 直读、凭据处理、Shuttle 自身安装/卸载、失败自动重试。

### 10.2 台账格式（schema 2）

沿用 D1 的位置 `%LOCALAPPDATA%/Shuttle/install-ledger.json`，外层结构不变（`schemaVersion` + `records`），单条记录扩为：

| 字段 | 类型 | 必填 | 用途 |
|---|---|---|---|
| `Name` | string | 是 | 规范化包名，与检测结果同名 |
| `Source` | `"npm"` \| `"winget"` | 是 | 安装渠道 |
| `Root` | string | 是 | npm 全局根 / winget 源标识；判定时与实际检测值比对 |
| `Version` | string | 是 | 安装后实测版本，不是计划版本 |
| `State` | `"intent"` \| `"installed"` \| `"unverified"` \| `"failed"` | 是 | 见 §10.3 |
| `OperationID` | string | 是 | 关联 `operations/` 下的操作记录，可追溯 |
| `EvidenceBefore` | string | 是 | 安装前该包的检测快照（名称/版本/来源/根）；未检测到记 `absent` |
| `EvidenceAfter` | string | 否 | 安装后检测快照；`intent` 状态时为空 |
| `InstallerExit` | int | 否 | 安装器退出码 |
| `InstallerError` | string | 否 | 安装器失败原因（保留原始首行，不吞错误） |
| `InstalledAt` | RFC3339 | 是 | 写入 `installed` 的时间 |

`schemaVersion` 为 1 的旧文件继续按 v1 读取，无 `State` 的记录视为 `installed`。事实依据：v1 的 `Save` 至今没有调用方（本批次无安装能力，见 T11b/T11c），本机 `%LOCALAPPDATA%/Shuttle/` 下也只有 `operations/`，因此不存在真实的 v1 记录文件——该兼容分支只覆盖手工写入或测试样本这类情形。读到既非 1 也非 2 的版本按 §6 的"台账损坏"处理，全量降级 `unknown`。写入时总是写 2。

### 10.3 记录状态与归属的映射

| `State` | 判为 | 能否进卸载清单 | 理由 |
|---|---|---|---|
| `installed` | `managed` | 是 | 安装器成功且安装后复检通过 |
| `unverified` | `unknown` | 否 | 安装成功证据不完整（复检失败、安装器失败但包存在或中断核对） |
| `intent` | `unknown` | 否 | 只写了意图，没有成功证据（常见于安装中崩溃/断电） |
| `failed` | `external` | 否 | 安装器失败且复检确认该包不存在 |

**取向与 D1 一致**：只有"安装器成功 + 安装后复检到该包 + 来源与根一致"三条同时成立才叫 `managed`。任何一条缺失即降级，且降级只会降成 `unknown` 或 `external`，永不升格。

### 10.4 归属判定的匹配键

维持 T11a 已实现的规则：按 `Name` 查记录，再用 `Source` 与 `Root` 比对。跨来源同名（npm 的 `x` 与 winget 的 `x`）不会互相冒领——记录只匹配一个来源，另一个走 §3 判 `unknown`。本阶段不引入新的复合主键，避免改变已交付语义。

### 10.5 写入协议

1. 复用已交付的存储写法：`FileLedger.Save` 已是"临时文件 + Sync + 原子替换"，`Put` 复用同一实现；不新造存储框架。**`Put` 先读后写**：台账读不出来时直接报错，不得用一条新记录覆盖整份旧台账。跨进程串行化复用 `FileOperationStore.Lock`（`operations/.operation.lock`）——台账写入只发生在安装操作内，而该操作本来就持有这把锁，因此**不新增第二把锁文件**，避免两把锁的获取顺序问题。检测路径只读，不需要锁。
2. 每次状态迁移都是一次完整重写（台账规模是几十条，不做增量）。
3. 写台账失败必须作为操作失败上报并保留操作记录；不得出现"安装成功但台账没写成"却显示成功的情况——此时该项保持降级，界面写明台账写入失败。
4. 顺序固定：写 `intent` → 执行安装器 → 复检 → 写 `installed`/`unverified`/`failed`。`intent` 先落盘是为了让崩溃有据可查，而不是让"没写成"变成"没发生过"。

### 10.6 接口草案

```go
package inventory // core/inventory

type RecordState string // "intent" | "installed" | "unverified" | "failed"

type Ledger interface {
    Lookup(name string) (Record, bool, error)
    // 整条写入是该状态的一次迁移。实现负责原子替换，不负责取锁：
    // 锁由调用方在安装操作开始时获取并持有（§10.5 第 1 条）。
    Put(record Record) error
}

// Step 增加字段（卸载计划留空）：
//   Package     已检测到时装包本身；未检测到的待装包，其名称/来源在这里
//   Command     将执行的完整命令，界面必须原样展示（D8）
//   Args        实际执行的 argv；不序列化给前端，执行不得经过 shell
//   Instruction 官方手动安装说明，无自动安装路径时使用
func PlanInstall(wanted []Package, detected []Item) (Plan, error) // 只读：装什么、跑什么命令
func VerifyInstalled(intent Record, detected []Package) Record     // 复检：三元组一致才 installed

// 执行侧：顺序固定为 intent → 安装器 → 复检 → 终态（§10.5 第 4 条）
type Installer interface { Run(argv []string) (exitCode int, message string, err error) }
type Snapshot func() ([]Package, error)
type PlanSource func() (Plan, error)   // 用于 stale_plan 复核

func ExecuteInstall(plan Plan, operationID string, ledger Ledger, install Installer,
    snapshot Snapshot, current PlanSource) (Execution, error)
func ReconcileInterrupted(ledger Ledger, snapshot Snapshot) ([]Record, error) // 启动核对，不重试
```

`PlanInstall` 的判定：已 `managed` / `external` 的一律 `skip`（不重复安装），`unknown` 的 `skip` 并阻断计划，未检测到的才 `install`。来源不支持时进入 `Conflicts` 而不是返回错误——一条不受支持的选择不该让整份计划消失。

`ExecutePlan` 的形态沿用 architecture §4：`stale_plan` 表示计划生成后环境已变，必须先重查；`OperationID` 关联 §10.2 的 `OperationID` 字段。取消语义仍是"请求取消，不保证安装器立刻终止"。

### 10.7 故障与降级

- 安装器失败：记 `failed` 并保留退出码与输出尾部；复检若发现包其实存在，改记 `unverified`（不猜）。安装器成功但复检失败：记 `unverified`，界面显示证据不足。
- 装完但终态台账写入失败：操作标记失败，历史保留在 `operations/` 中；已持久化的 `intent` 保持 `unknown`，不得认领为 `managed`，界面明确写出失败原因。
- 启动时发现 `intent` 未收尾：不做自动重试，重新检测后统一写 `unverified`；包存在也不能补出缺失的安装器成功证据。无 intent 时不运行采集器。
- 安装期间用户退出：按 architecture §2 的安全检查点处理，已开始的安装器不中途强杀，退出前等待其结束或标注未完成。

### 10.8 测试矩阵

| # | 用例 | 层 |
|---|---|---|
| 9 | schema 1 台账可读且视作 `installed`；schema 3 判损坏→`unknown` | adapter fixture |
| 10 | 四种 `State` 分别判为 managed/unknown/unknown/external | core 纯函数 |
| 11 | 写 `intent` 后崩溃（模拟不写第二次），重启检测仍为 `unknown` | adapter 集成 |
| 12 | 安装器失败 → `failed`；失败但复检到包 → `unverified` | core + adapter |
| 13 | 台账写入失败时操作判失败，配置与台账均未被部分写入 | adapter 故障注入 |
| 14 | 并发写：同一进程两个写者交替 `Put`，最终台账可解析且不丢记录；跨进程串行化由 operations 锁覆盖，测试断言写入发生在持锁区间内 | adapter 集成 |
| 15 | `PlanInstall` 全程只读（磁盘无变化、无安装器调用） | adapter 集成 |
| 16 | `stale_plan`：计划生成后包状态变化，`ExecutePlan` 返回过期而非照做 | core |
| 17 | `PlanInstall` 每个步骤都带完整命令文本与手动说明，界面可直接展示（D8） | core |
| 18 | `VerifyInstalled`：同名同源同根 → `installed`；检测不到 → `unverified`；台账不可读由执行侧报错，不进入安装器 | core + bridge |

### 10.9 任务卡

| 卡 | 内容 | 验收 |
|---|---|---|
| T11d-1 | 台账 schema 2：读写、v1 兼容、四态映射 | 测试 9、10 |
| T11d-2 | 写入协议：`intent` 先行、原子替换、写入失败即操作失败、调用方持 operations 锁 | 测试 11、13、14 |
| T11d-3 | `PlanInstall` 只读计划（含命令与手动说明）与 `VerifyInstalled` 复检 | 测试 15、17、18 |
| T11d-4 | `ExecutePlan` 安装侧与 `stale_plan`、启动核对 | 测试 12、16 |

单卡 ≤ 300 行；每卡完成时门禁退出码 0，且不得为通过而放宽既有断言。

### 10.10 分叉决策（已冻结 2026-10-25）

- **D4 记录粒度 = 每个包一条当前状态**，不保历史轨迹；历史由 `operations/` 承担。理由：台账只回答"现在归谁"，撤销与审计已有操作记录，两处重复保存必然分叉。
- **D5 `intent` 先行 = 是**。顺序固定为写 `intent` → 执行安装器 → 复检 → 写终态。理由：崩溃后可解释；不先行则"安装成功但没写台账"与"用户自己装的"无法区分，正是本批次要消灭的歧义。代价是每次安装多一次写盘。
- **D6 复检标准 = 名称 + 来源 + 根三者一致**，版本只用于展示、不参与归属判定。理由：用户可能手动升级到更高版本，此时仍应归 Shuttle 所装。
- **D7 安装渠道 = 仅 npm 与 winget**；权限提升只在安装步骤内请求且用户可见；不引入自研下载器。理由：与 D2 同源，未采样的渠道不写代码。
- **D8 手动 vs 自动 = 两者都提供，默认手动**。每个组件给出官方安装说明与可自动安装的按钮；自动安装必须在执行前展示将运行的完整命令。`PlanInstall` 与 `ExecutePlan` 因此都必须携带该命令文本。



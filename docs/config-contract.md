# 配置安全合并/还原契约（T10 批次）

状态：Type C Phase 2 契约草案 v1，待用户拍板后冻结。冻结前不写业务代码。
依据：docs/architecture.md §1 配置管理范围、§3 数据契约、§5 配置应用与还原、§7 状态机。

## 1. 本批次管理目标

| 文件 | 管理字段 | 身份规则（冻结） |
|---|---|---|
| WT `settings.json`（JSONC） | 主题、字体 face、默认 Profile、Shuttle 管理的 Profile、用户选择的快捷键与终端参数 | Profile 按 GUID；主题按名称；快捷键按 (keys, command) 二元组；标量按唯一路径（如 `profiles.defaults.fontFace`、`defaultProfile`） |
| PowerShell `$PROFILE` | 会话入口管理块（唯一标识块，整块替换） | 管理块 marker，见 §4.2 |
| Claude 状态栏 | 可选预设 | 本批次只定义接口与占位，不实现写入 |

排除：Starship（保持现状）、Codex 配置、凭据类字段、接管整个文件、通用递归 JSON merge。

## 2. 包与依赖方向

```
core/configuration      Plan/Apply/Restore 用例；Editor、BackupStore 接口定义在此
adapters/configuration  WT JSONC 手术式编辑、Profile 管理块编辑、备份与文件锁实现
app.go / main.go        组装适配器并注入核心；不出现 import core→adapters
```

- 操作记录与备份落盘 `%LOCALAPPDATA%/Shuttle/operations/<OperationID>/`（record.json、备份文件）；只属本机，不入 UserProfile。
- 写入一律"临时文件 + 替换"；同目标写操作互斥（文件锁）。

## 3. 核心数据契约（Go 签名草案）

```go
package configuration // core/configuration

// 用户选择（前端提交，schemaVersion 迁移）
type ConfigChoice struct {
    SchemaVersion int
    WT            WTChoice   // Theme, FontFace, DefaultProfileGUID, ManagedProfile, Keys []KeyChoice, Params
    PS            PSCheck    // SessionEntry bool
    Statusline    string     // 本批次恒为空；非空返回 unsupported
}

// 差异项：一个受管目标
type FieldDiff struct {
    Target     string // 稳定标识，如 "wt:profile:GUID"、"wt:scalar:defaultProfile"、"ps:managed-block"
    Action     string // add | modify | remove | none
    Before     string // O：应用前原值；缺失记 "" + WasAbsent
    WasAbsent  bool
    After      string // A：Shuttle 将写入值
    Conflict   string // 空=无冲突；否则为人类可读冲突原因
}

type ChangePlan struct {
    PlanID     string
    Choice     ConfigChoice
    Diffs      []FieldDiff
    Blocked    bool   // 存在未解决冲突时 true，ExecutePlan 拒绝执行
}

type Editor interface { // adapters 实现；核心只面向接口
    ReadCurrent(target string) (value string, err error)   // 读现状（C 的来源）；缺失返回 ErrNotFound
    Check(target string, desired string) error              // 只读冲突判定（如同名不同 GUID）；nil=无冲突
    Apply(target string, value string) (before string, changed bool, err error) // 手术式写入
}

值约定：WT Editor 使用**原始 JSON 字面量**（`"dark"`、`{"guid":...}`）；PS Editor 使用完整管理块文本（换行规范化为 LF）。空串表示"删除受管内容"。

type BackupStore interface {
    Save(opID string, target string, o string, a string, oAbsent bool) error
    Load(opID string, target string) (o string, a string, oAbsent bool, err error)
    Targets(opID string) []string // 还原顺序列出已备份目标
}

func Plan(choice ConfigChoice, ed Editor) (ChangePlan, error)              // 只读
func Execute(plan ChangePlan, ed Editor, bs BackupStore) (OperationResult, error) // 重查前置状态，stale_plan
func Restore(opID string, ed Editor, bs BackupStore) (OperationResult, error)     // O/A/C 决策，见 §5
func ListOperations() ([]OperationSummary, error)
```

错误复用 architecture §8 错误集（validation_failed / conflict / stale_plan / …），不吞底层错误。

## 4. 合并策略（实现前冻结）

### 4.1 WT JSONC：字节级手术替换
- 解析仅用于定位目标 span 与身份匹配；写回只替换目标 span 字节，**注释、尾逗号、无关键值逐字节保留**。
- 禁止反序列化→改内存→重新序列化整文件（该路线丢注释，禁止）。
- 数组身份匹配失败时：`add` 允许追加；`modify` 不猜，报 conflict。
- 同名 Profile 已存在但 GUID 不同 → conflict；GUID 相同 → 正常 modify。

### 4.2 PowerShell Profile：管理块
- Marker 常量（冻结）：

```
# >>> shuttle managed (do not edit) >>>
# <<< shuttle managed (do not edit) <<<
```

- 应用 = 整块替换（已有块）或文末追加（无块）；重复应用幂等。
- T10c 的默认块启用 PSReadLine `PredictionSource History`；完整文本定义于 `internal/core/configuration/profile.go`，Plan 展示实际内容，不使用 `present` 占位符。Shuttle 会话入口命令尚未定义，本卡不生成入口函数。
- 块外字节、UTF-8 BOM 与 LF/CRLF 保留；非 UTF-8、重复/残缺/逆序 marker 拒绝写入。追加所需分隔换行属于块外内容，移除时保留。
- 冲突检测：块外出现同类会话入口（匹配 `oh-my-posh init|starship init` 行）→ FieldDiff.Conflict 非空，Blocked=true，不注入。注释行不算初始化；检测采用保守行匹配，不承诺解析全部 PowerShell 动态表达式。

### 4.3 幂等
同一 Choice 连续应用两次：第二次 Plan 的 Diffs 全部 Action=none。验收清单第 3 条。

## 5. O/A/C 三方还原（决策函数落为代码 + 测试矩阵）

对每个受管目标：保存 O（应用前）、A（写入值）；还原时读 C。

| 现状 | 动作 |
|---|---|
| C == A | 还原 O；O 缺失 → 删除受管内容 |
| C == O | 已还原，跳过 |
| C ≠ O 且 C ≠ A | 冲突，默认保留当前值，标 conflict 待用户单独选择 |
| 文件丢失/结构不可识别 | 人工处理项，不整文件强写 |

整体文件级还原为额外选项，执行前必须再备份当前文件。

## 6. 中断恢复

每步执行**前**先持久化该步骤 pending 记录，执行后回写结果。启动时扫描存在 pending/running 记录 → 标 interrupted，提供"核对现状 → 继续或还原"入口，不盲目重放。

## 7. 测试契约（全部 t.TempDir()，禁触真实 WT/Profile）

fixtures：`tests/fixtures/config/wt-*.jsonc`（含注释/尾逗号/同名不同 GUID Profile/冲突快捷键各一）、`ps-*.ps1`（空文件/含 starship 行/已有管理块各一）。

必测矩阵（每条二元可判定）：

1. WT 修改 defaultProfile 后：目标 span 变更，文件其余字节不变（比对剔除 span 后的哈希）。
2. JSONC 注释与尾逗号在写回后保留。
3. 同 GUID Profile 二次应用 diff 为空。
4. 同名不同 GUID → conflict，Blocked=true，ExecutePlan 拒绝。
5. PS Profile 注入管理块后二次应用 diff 为空。
6. starship 行存在 → conflict，不注入。
7. C==A 还原后内容等于 O。
8. C==O 还原跳过，无写入。
9. C 被用户手改 → conflict 保留当前值。
10. 每步之间模拟中断（pending 记录存在）→ 重启标记 interrupted。

## 8. 任务卡（单卡 ≤ 300 行）

- T10a：core/configuration 纯用例（Plan/Restore O/A/C 决策、幂等判定，接口 + 内存 Editor 测试）。
- T10b：adapters/configuration WT JSONC 手术编辑 + fixture 测试（§7.1–4）。
- T10c：adapters/configuration PS 管理块 + fixture 测试（§7.5–6）。
- T10d：操作记录存储 + ExecutePlan 编排 + 中断扫描 + 桥接（PlanConfiguration/ExecutePlan/Restore/ListOperations）+ 前端配置页（预览差异 → 应用 → 还原入口）。

## 9. 拍板分叉（2026-10-24 已按推荐项冻结）

- **D1 首批范围**：WT + PowerShell Profile 全做，Claude 状态栏只留接口（本批次不写）。
- **D2 WT 写回**：字节级手术替换（§4.1）。
- **D3 操作记录位置**：`%LOCALAPPDATA%/Shuttle/operations/`；与安装台账同根不同目录。

契约修订记录：T10a 实现时发现 O 的"缺失"与"空值"不可区分（空串是合法值），BackupStore 增加 oAbsent 标志位——否则 O 缺失的受管块在还原时会被误判为"恢复为空值"而不是"删除"。

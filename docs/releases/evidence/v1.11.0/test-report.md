# v1.11.0 有限 CLI/工作流验收报告

用户于 2026-10-14 选择按有限范围发布 v1.11.0。本报告使用 schema 3，通过范围为 cli-fixes；
完整设计 E2E 延期至 follow-up。有限软件通过不代表电路或制造通过。
产品源码绑定 dev 提交 `699d140`；现场执行时间 2026-10-13T17:53:18Z–17:54:32Z（UTC；M1 绑定、F1、F2、L1、N1、R1），
E1 于 18:02:02Z–18:02:07Z 在同一候选、同一连接会话上补跑（含保存重载与退出码落盘）；其余离线验证在同一候选上于其前后完成。

## 已完成的软件验证

- 候选 Go 全仓测试：5,536 项通过、0 失败，15 个包；`go vet` 仅有一条早于 v1.10.0 的既有提示。
- 连接器测试：1,159 项通过、0 失败；typecheck 通过。新增导入空操作正反例 2 项。
- 发布脚本测试：122 项通过；Skill 规则自检通过；agent-check 通过；skill-check：205 个 tracked 文件及安装后链接通过。
- 现场 CLI/daemon 版本串 `v1.10.0-32-g699d140-dirty`（dirty 仅为未提交的连接器版本号）；dev 构建的
  health versionGate 为 skipped（不做硬判定），候选绑定依据是版本串与本地包哈希。
  连接器开发包 dist/index.js SHA256 `e64369604dc5f155664d30a16580ce1ae472116289fffe23fd028c7f6977fafd`，
  经 IndexedDB 热重载装入（现场 1.10.7）。该包构建于候选提交前约 1.5 分钟，`extension/src`
  自候选提交起无改动，未在提交后重新构建比对哈希。
- 五平台交叉构建与资产核对由 release-build 执行，不代表其他平台运行时验证。

## 现场回读

EDA Web 4.1.71，标签页可见。F1：P1 显式保存（saved:true）并 doc reload 后，严格门禁全部 PASS
（layout-lint 0 overlap、clusters 0 重叠/0 过近、check 0 error、bridge-check 0 短路/0 孤立、
原理图 DRC native PASS）。P1 有 4 个已登记模块分组，严格档下未出现 peripheral-not-direct 或
peripheral-direct-unverified（两者都会阻塞），据此判定外围直连规则通过；该规则通过时不打印单独记录。

F2：PCB1 保存后连续两次 doc reload，两份 dump 语义 SHA256 均为 `226113afd926c4ee…`，10 个器件；
`pcb check` ERROR=0、WARN=7，dangling/clearance/antennaKeepout/silkOffBoard/zoneClaimsStale 均为 0。
7 条 WARN 为已披露的设计取舍：平行间距 4 条（U0RXD/U0TXD 3 条、EN/LED_CTRL 1 条）、
10µF 体电容距 3V3 脚 3.1mm（布局 r4 经用户确认）、3V3 以 12–16mil 走线而非铺铜、GND 过孔短引线 12mil；
另有 1 条 INFO fiducial-missing。

E1：已同步状态下默认 import-changes 退出码 0（落盘），回包 confirm=no-dialog、noChanges=true、verified=true，
未回填器件库属性；随后 pcb save（saved:true，退出 0）、doc reload（退出 0）后原生 PCB DRC `passed:true`
（退出 0），导入前后语义哈希一致。CLI 在该空操作成功时仍向 stderr 输出一条"apply was NOT confirmed"告警，
与成功结果矛盾，列为已知未修。

L1：离线重算得到与已执行队列逐步骤一致的 89 步保留实例重建队列；现场 10 个器件实例 ID 与
重建前快照一致，逐脚网络一致。N1：旧快照队列 `--yes` 回放在第 2 步 verify-source-before-reset
以 source-drift 拒绝，journal 只有第 1 步只读校验 ok 与第 2 步 fail；页面完整快照按 `jq -S .result`
规范化后 SHA256 前后均为 `4244847022220cf3…`。R1：页面列表仅 P1，PCB 10 个设计器件，P1 分组注册表 4 个模块。

| ID | 状态 | 证据与限制 |
|---|---|---|
| M1 | pass | 候选 699d140：Go 5536/0、连接器 1159/0、发布脚本 122、agent/skill-check 通过；现场 CLI/daemon 版本串与候选一致，连接器源码自候选起未改。 |
| F1 | pass | f1-save saved:true、f1-reload 成功后 f1-gate 严格门禁五项 PASS（含原理图 DRC native PASS）；4 个登记模块下严格档无外围直连阻塞项。 |
| F2 | pass | f2-dump-a/b 两次重载后语义哈希同为 226113afd926c4ee…；f2-pcb-check ERROR=0，clearance/antenna/silk/zone-stale 为 0。 |
| E1 | pass | e1-import.exit=0、noChanges/verified=true；e1-save saved:true 与 e1-reload 后 e1-pcb-drc passed:true；e1-dump 与 f2-dump-b 语义哈希一致。 |
| L1 | pass | l1-apply 与已执行队列 89 步逐项相同；l1-after 与重建前快照的 10 个实例 primitiveId/uniqueId 与逐脚网络一致。 |
| N1 | pass | n1-apply 在 verify-source-before-reset 以 source-drift 拒绝，journal 无写入步；n1-before/after 按 jq -S 规范化哈希一致。 |
| R1 | pass | r1-pages 仅 P1；r1-pcb-list 10 个设计器件；r1-groups 只含 MCU/LED/PWR/PROG 四个模块。 |
| E2E | in-progress | 需求到两层 PCB 成品、整板布线与 DRC passed 已在开发中跑通，但跨多个候选（期间修码与热重载），未在冻结候选上完整执行；延期 follow-up。 |

## 失败与未完成项

- 完整 E2E 未在冻结候选上执行，B00–B10 未重跑；演示过程中的问题与修复记录在本地忽略目录。
- 已知未修：import-changes 已核验空操作成功时仍输出矛盾的 stderr 告警（`cmd_pcb.go` 只认 confirm=applied）；原理图 compose 在 MCU 区留下 5 处无接点交叉（INFO，可读性）；页面边界规则尚不检查自由文字；
  decap-too-far 文案写"≤2mm"而判据为 100mil；`silk-align --side` 提示方向不可靠；`pcb auto-place`
  不避让天线禁铜区；`pcb route solve` 无"避开器件本体"选项。
- Skill 包 205 个文件，仍超过 SkillHub 200 文件上限，该平台同步预计失败；连接器市场需人工提交。
- 本轮未使用外部自动布线引擎；布线由离线单层求解器逐对求解、独立复验后以类型化写入，少量 3V3/GND
  过孔与短线为手工坐标写入并由原生 DRC 证明。

原始记录保留在本地忽略目录 artifacts/release-v1.11.0/ 与 artifacts/demo-20261013/；
公开材料不包含用户路径、工程 UUID 或私有设计数据。

## 独立复核

独立 Agent 于 2026-10-14 两轮只读复核，未访问现场 EDA，也未运行 easyeda 命令，结论 pass。复算：F2 两次重载及
E1 补跑后的三份 PCB dump 语义哈希均为 226113afd926c4ee…，去掉采集元数据后内容一致；E1 补跑依次为导入（no-dialog、
noChanges、verified，10→10 个器件）、保存 saved:true、重载、原生 DRC passed:true，各步退出码均为 0。F1 严格门禁五项
PASS；f2-pcb-check ERROR=0、WARN=7、INFO=1，构成与报告一致。N1 的 journal 只有一个只读 ok 步和 verify-source-before-reset
的 fail 步，按 jq -S .result 前后哈希同为 4244847022220cf3…。L1 与 N1 的重建队列和已执行的 89 步队列逐字节相同；
L1 现场 10 个器件的 primitiveId、uniqueId 及逐脚网络与重建前快照一致；R1 为仅 P1、10 个 PCB 器件、4 个分组。
已核对候选 699d140 之后无产品源码改动，连接器包哈希、需求文件哈希与报告一致；定向 Go 测试 267 项与连接器
PCB import 测试 8 项通过，全仓测试计数未重跑。已知限制如实披露：dev 构建 versionGate 为 skipped；导入空操作成功时
仍输出矛盾的 stderr 告警；外围直连规则通过属推断而非单独记录；E2E 保持 in-progress，不构成完整设计或制造通过。

## 发布准备

具体版本与有限范围已获用户批准。正式发布仍由现有 release-check/release-build/publish-release
路径生成并核对版本、资产及远端 SHA256；正式非草稿 Release 完成前不宣称已发布。

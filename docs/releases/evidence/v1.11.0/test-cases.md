# v1.11.0 有限 CLI/工作流用例

以下 ID 只定义本次用户批准的有限范围；不沿用完整电路验收中的同名判据。
全部用例在冻结候选 `699d140` 上重新现场执行；现场写入只针对 ceshi 专用工程。

| ID | 范围 | 输入、步骤与判据 |
|---|---|---|
| M1 | 软件与候选绑定 | 候选上运行 Go 全仓、连接器测试与 typecheck、发布脚本测试、agent-check、skill-check；现场 CLI/daemon 版本串指向候选提交，连接器源码自候选起无改动。全部通过、0 失败。 |
| F1 | 原理图门禁 | 已重建的 P1 显式保存、doc reload 后跑 `sch gate --strict`：layout-lint/clusters/check/bridge-check/DRC 全 PASS；P1 已登记模块分组时，新增外围直连规则在严格档下不报 peripheral-not-direct / peripheral-direct-unverified（两者在严格档会阻塞）。 |
| F2 | PCB 检查与采集 | 已布线 PCB1 保存后两次 doc reload，各自 dump；semanticSha256 必须一致（浮点归一化修复）；`pcb check` 0 ERROR，antennaKeepout/silkOffBoard/zoneClaimsStale/clearance 均为 0。 |
| E1 | 导入与 DRC | 原理图与 PCB 已同步时默认 `pcb import-changes` 退出 0（无弹窗空操作核验为 noChanges），不回填器件库属性；随后显式保存（saved:true）、doc reload，原生 DRC passed（无 Netlist Error），导入前后 PCB 语义哈希不变。各步退出码落盘。 |
| L1 | 原理图整页重建 | 同一 layout-input 在候选上离线重算 lib-layout/compose：生成的保留实例队列与已执行队列逐步骤一致（可复现）；现场 10 个器件 primitiveId/uniqueId 与重建前快照一致，逐脚网络不变。 |
| N1 | 过期队列拒绝 | 把基于旧快照生成的 89 步重建队列 `--yes` 回放到已变化的现场页：必须在首个写入前于 verify-source-before-reset 拒绝；journal 只有校验步，页面完整快照（`jq -S .result` 规范化后的 SHA256）前后一致。 |
| R1 | 清理与状态 | 本轮临时页（含曾经假删除的幽灵页）与其导入的临时器件已清除：页面列表仅 P1，PCB 恰为 10 个设计器件，P1 分组注册表只含 4 个现行模块。 |
| E2E | 延期 | ESP32-S3 需求到原理图/两层 PCB 成品、整板布线与 DRC 已在开发过程中跑通，但期间多次修改源码与热重载连接器，未在单一冻结候选上完整执行，记 in-progress，留待 follow-up。 |

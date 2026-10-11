# v1.10.0 有限 CLI/工作流验收报告

用户于 2026-10-11 批准 v1.10.0 直接发布并复用刚完成的验证。本报告使用 schema 3，
通过范围为 cli-fixes；完整设计 E2E 延期至 follow-up。有限软件通过不代表电路或制造通过。
产品源码绑定 `46dc97bfcbc97809915d9c414d4c9891a075db2c`，最近执行版本为 v1.10.0-dev.1；
正式候选只同步版本与发布材料，不夹带源工作区其他开发改动。

## 已完成的软件验证

- 隔离候选 Go 全仓测试：4,938 项通过，17 个包。
- 同一候选连接器测试：939 项通过；typecheck 与 build 通过。
- agent-check：13 项通过；skill-check：205 个 tracked 文件及安装后链接通过。
- 发布脚本测试：122 项通过。
- v1.10.0-dev.1 local-build：五平台 CLI、连接器、Skill、两种安装脚本及 checksums 成功生成。
- local-dev 资产 smoke：9 项资产 SHA256、205 个 Skill 文件与 15 项原生离线命令通过。
- 其他平台只有交叉编译，未进行其运行时验证。上述记录复用最近原始执行输出，不在发布时重跑。

## 现场回读

最近定向回归使用同版 dev.1 CLI/daemon/指定连接器与 Web 4.1.71，旧窗口不参与证据。
三次真实保存、关闭、打开及 fresh readiness 均通过，saved:true、请求身份和阶段时间完整。
四份 dump/sidecar 的身份、字节数与 SHA256 匹配，空板库存与 unavailable/unknown 未被默认值覆盖；
原空板的 outline 仍为 unknown，不能称完整板框读取可用。

临时电阻 fresh 丝印实测 lineWidth=6：位号可见、Device/Footprint 值隐藏、keyVisible=false，
均有 getter 来源；自由 STRING、getter 缺失和 keyVisible=true 没有现场覆盖。
stage 共七份文件（六份 JSON 加一份 PNG）哈希与身份匹配；PNG 编码有效却为单色空白，
frameColors=1、frameContent=0、blank=true，命令正确失败。第二轮、成功官方图及成功同字节新截图
均未现场执行；同字节新采集的正确区分只由自动测试覆盖。

fresh fixture dump 离线生成 actual SVG，源文件 SHA256
`dcdbe464dc411b919a19b93bb7c1005569a44ace6c5088c192d5a8492886893c` 未变，
SVG SHA256 为 `7773f43c4926fd31455ff503227d9740efafaa4388675d3e346f2715c698496a`。
临时一个器件和一个板框精确清理后，保存重载恢复原空板，语义 SHA256 为
`fa3ca20a13bf0959f4209db18e84af60310313ee2c71fc87e4284d8511222620`。
这只是定向 fixture 回归，不能代表真实完整电路。

| ID | 状态 | 证据与限制 |
|---|---|---|
| M1 | pass | 同一源码候选最近 Go 4938/连接器939/发布脚本122 及入口、资产检查通过；正式候选代码未改变，仅同步版本与发布材料。 |
| F1 | pass | 05-doc-reload、13-fixture-reload、20-cleanup-reload 三次原始回执均 saved:true、closed:true，fresh 身份与阶段时间完整；负例属于自动覆盖。 |
| F2 | pass | board-before/after/fixture/restored 四份 dump 的 persisted bytes、SHA256、身份及 sidecar 对账一致；空库存保持空，outline unknown 保留。 |
| E1 | pass | actual SVG sourceSha256 精确绑定 fresh fixture JSON，渲染未改源；正反面、非零原点、旋转焊盘、通孔及文字由合成自动回归验证。 |
| L1 | pass | 15-silk-fresh 及 dump 实测三个属性 lineWidth=6、位号可见、Device/Footprint 隐藏、keyVisible=false，读取来源完整；缺 getter 等未现场覆盖。 |
| N1 | pass | stage 六份 JSON+PNG 哈希/身份匹配，现场 blank 图片按预期拒绝；缺图/坏图/哈希/身份漂移/显式重复及新采集同字节通过自动反例，成功官方图仍未验证。 |
| R1 | pass | 18-component-delete、19-outline-clear、20-cleanup-reload、21-cleanup-dump 精确清理本轮对象，恢复语义哈希及原库存/availability。 |
| E2E | not-run | 完整基础批次与 ESP32-S3 需求到成品、整板布线和最终 DRC 尚未完成，延期 follow-up；有限用例及发布不能补签完整设计。 |

## 失败与未完成项

- 基础工程创建 B03 为 fail：两种 project.create 输入均返回 EDA_CALL_FAILED / Project creation returned no UUID，真实原因未证实。首次失败后完整 find 读取 68 项且 complete:true，现有 ceshi 类型化打开成功；这些成功不替代新建回执。
- B00 仅 partial：精确同版与部分读取已核对；B09 仅 partial：PCB 定向持久化/采集通过，但有效官方截图及完整原理图持久化未覆盖。其他完整基础项不补签 pass。
- 官方截图失败与第二轮 not-run 保留；现场两轮完整 PCB 预览未成立。
- PCB-only 临时 fixture 无对应原理图，stage DRC 报 Netlist Error；没有 DRC pass 或网络连通通过声明。
- 收尾时类型化返回原工程成功，打开 PCB 文档超时；新窗口未上报活跃文档，已停止现场写入，旧窗口状态不能补签导航成功。
- 实验性 Beta 同源探针、Common Library、原理图保留及库身份能力只按其已有有限记录披露，任意整图替换和所有宿主/库变体未验收。

原始记录保留在本地忽略目录 artifacts/release-1-10/ 与本次会话执行记录；公开材料不包含
用户路径、工程 UUID、私有库资产或定位器设计。图册生成和硬件验收分别记录。

## 独立复核

独立 Agent 对最近原始记录、候选差异、有限用例、失败/缺测状态及脱敏边界进行了只读复核，
没有重跑测试或访问现场 EDA。直接确认本次会话中 13:27–13:29 UTC 的软件测试与资产 smoke
原始完成输出；复算四份 dump/sidecar、七份 stage 文件及 SVG 的哈希和身份，确认源码隔离。
截图 PNG 实为 1×1 像素、88 bytes 的单色图，拒绝正确；outline unknown、Netlist Error、
工程创建失败与导航超时均保留。结论为本次有限 CLI/工作流范围通过，完整 E2E 仍 not-run。
显式重复与同字节新采集由自动用例覆盖，不声明按截图年龄或时间窗完成现场验证。

## 发布准备

具体版本与有限范围已获用户批准。正式发布仍由现有 release-check/release-build/publish-release
路径生成并核对版本、资产及远端 SHA256；正式非草稿 Release 完成前不宣称已发布。

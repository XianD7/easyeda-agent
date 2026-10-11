# 外部工程导入与迁移

本页描述 Altium Designer 工程迁移的当前能力边界。它不是 `easyeda` 的导入命令。


## 文档重载与只读就绪核查

`easyeda doc reload <uuid> --project <project> --json` 显式保存、确认关页后只重开一次。
重开使用 30 秒连接器预算（另留响应宽限），成功后在 10 秒内核对活动工程／文档／类型／tab
以及连续稳定的完整组件清单。保存须返回布尔 `saved:true`；关闭、重开或对象读取失败均停止，
不自动重发 open，不用 GUI 刷新兜底。

JSON 回执保留 `saved/reloaded/documentType/activeRestored`，并记录目标、阶段请求 ID、CLI
观察起止时间和 readiness。失败也输出已发生阶段，进程仍非零退出；`saved:true` 而重开失败
是 partial，不等于重载通过。重载非活动文档后，恢复旧活动页必须新读确认；恢复失败单独
报告，已完成的重载事实不丢失。主机观察时间与响应 `createdAt` 分别保留。

超时后可执行纯只读核查：

```bash
easyeda doc ready <expected-document-uuid> --project <project> --timeout 10s --json
```

它不保存、关闭、打开或切页；指定目标必须已活动，省略 UUID 则检查当前页。每次组件读取前后
核对同一上下文，按 `primitiveId` 排序完整组件记录计算指纹；非空清单须连续两次相同，空页
须三次。相同数量而内容不同不算稳定，缺清单、缺身份、重复 ID、上下文漂移或超时均拒绝。
显式 `partial`／`incomplete`、非空 `captureErrors`、不可用或错型的采集状态同样拒绝；
PCB 未请求的焊盘／包络可保留 `not-requested`，该命令不扩大为几何验收。
`--timeout` 为选定窗口后的总读取预算，范围 1 秒至 1 分钟；不会等待队列阻塞自动重试。

`ready:true` 只证明当前组件清单稳定可读，**不证明先前超时 handler 已结束，也不替代 reload
成功、完整网络／导线／几何回读或持久化验收**。保留原失败及保存、关闭、关闭后目标离开活动态
的证据；另作完整新回读并与已保存目标对账。恢复后的新验收周期只执行一次显式 save/reload，
新周期仍失败就停止，不循环重开；不得把原失败改写成成功。

## Altium Designer 工程：当前 `unsupported`

当前没有经过验证的 typed action 或 CLI 能把 `.SchDoc` / `.PcbDoc` 程序化导入
EasyEDA Pro。Agent 不得用交互界面、CUA、`debug.exec_js`、分块 base64 上传或私有消息总线
兜底。需要导入时停止现场写入，将能力标为 `unsupported`，先实现并验证 typed import。

如果目标工程已经由用户独立提供，easyeda-agent 只从现有工程开始读取与核验：

1. `easyeda health --project <project>` 与 `easyeda doc ls --project <project>`：确认连接器、
   目标工程，以及导入所得的原理图/PCB 文档。
2. 对每张原理图读取 `easyeda sch connectivity --page <name-or-uuid> --project <project>`，
   核对器件身份、全部物理引脚、网络、NC、分层页面与位号；再逐页运行适用的
   `layout-lint`、`sch check`、`bridge-check` 和 SDK DRC，分别保存结果。
3. 对 PCB 核对 Board 绑定、器件与焊盘网络、板框、层叠、机械层/禁布区、关键网和
   丝印；运行 `easyeda pcb layout-lint`、`easyeda pcb check --strict` 与
   `easyeda pcb drc --strict`。AD 机械层到 EasyEDA
   层的映射必须在原生画布和数据回读中确认，不能只看工程已出现在目录里。
4. 记录导入前后的数量与关键不变量。缺页、空读、板框/机械层变化、网络不一致或检查
   未运行时，均不能报告迁移成功；修复完成后显式保存。

## 为什么不能调用名义上的导入 API

官方 beta 方法 `eda.sys_FileManager.importProjectByProjectFile` 的签名列出了
`'Altium Designer'` / `'Protel'` 类型，但在 issue #203 的 EasyEDA Pro 桌面端
3.2.149（Windows x64，本地工作区）实测中，对 `.epro2`、`.eprj2` 和真实 `.SchDoc`
都会快速 resolve 为 `undefined`，不抛错，也没有创建文档或改变工程。

因此：

- 方法存在、Promise resolve 或返回 `undefined` 均不算成功；不得继续后续自动化。
- `getAllProjectsUuid()` 在本地工作区可能返回 `[]`，不能单独作为导入副作用判据。
- `eda.sys_FormatConversion.convertAltiumDesignerLibrariesToEasyEDA*` 只转换
  `.SchLib` / `.PcbLib` 库文件，不转换 `.SchDoc` / `.PcbDoc` 工程文档。
- 当前不封装 `import.ad` typed action。将来只有在受支持宿主上证明真实副作用，并能在
  返回值缺失、超时和部分导入时明确失败，才可进入 CLI/action 实现。

未来接口封装至少要在写入前记录目标工程指纹，并在返回后核对工程/文档身份、文档清单、
原理图连接数据及 PCB 板框/机械层；不得把 `undefined`、无变化或单一 UUID 清单当成功。

来源与完整探测矩阵见 [GitHub issue #203](https://github.com/zhoushoujianwork/easyeda-agent/issues/203)。


## 工程级打开与原生导出

`project open --uuid` 是旧的文档打开别名，只能用于当前工程内页面，不是跨工程打开。
跨工程时先保存所有未保存文档，再显式调用：

```bash
easyeda project open --window <window-id> --project-uuid <project-uuid> --page-uuid <page-uuid> --allow-discard-unsaved
easyeda project export --window <window-id> --project-uuid <project-uuid> --out ./deliverable.epro2
```

打开命令使用官方 `dmt_Project.openProject`；该 API 可能丢弃未保存数据，因此标志是明确确认，
不是自动保存。命令核对打开后的真实工程身份；超时或失败先读回，不盲目重复。
导出使用官方 `sys_FileManager.getProjectFile`，要求目标工程已激活，前后检查 UUID，
拒绝覆盖文件，校验原生 ZIP 完整性并输出字节数/SHA-256。输出 `restoreVerified=false`：
ZIP 校验和导出成功不代表重新导入验证通过。请在导出前显式保存所有文档。
底层调用正式 typed action `project.open` / `project.export`，需要注册这两个 action 的新版 daemon 及包含 handler 的新版连接器。旧 daemon 或连接器拒绝 unknown action / UNKNOWN_ACTION 时停止并升级；禁止回退到 debug.exec_js。
直接调用 `project.open` 同样必须传 `allowDiscardUnsaved:true`；`project.export` 不接受页面路由。
限制：归档最大 16 MiB，解压验证上限 128 MiB，超限明确失败。

MCP 使用 `easyeda_project_transfer`，`operation` 为 `open` 或 `export`，均需 `window`、
`projectUuid`；打开另需 `allowDiscardUnsaved=true`，导出另需新 `out` 路径。不传文档路由。
此工具需要含上述 CLI 命令的匹配构建；不要仅替换 MCP 而仍使用旧 CLI。

工程身份可能先于文档树就绪。需直接进入原理图时，打开命令同时传 `--page-uuid`（MCP `pageUuid`），等待目标页面出现在树中后只打开一次，并核对工程和页面身份。省略此参数只保证工程身份，不保证页面已加载。

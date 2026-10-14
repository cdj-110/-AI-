# Go 网关整体检查报告

检查日期：2026-09-09  
范围：本地 `apps/gateway-go` 后端及 `webui` 前端，涵盖权限、配置、协议采集、历史、告警、边缘计算和系统维护相关实现。不是对云平台 API、移动端、所有现场设备的完整审计。当前工作树包含尚未部署的修改，本报告不等同于 EG100 当前二进制的全部行为。

本轮未修改业务代码、未清空数据、未更改设备配置、未部署。仅新增本报告与隔离诊断用例，并执行测试和构建。

## 结论

现有主要模块已有实现，不能再简单视为“缺少入口”。优先问题是：权限边界不一致、部分协议数据解释错误、历史数据完整性，以及前端操作与权限/保存行为不统一。建议先修正确性，再补齐功能和视觉统一。

| 优先级 | 问题 | 证据强度 |
| --- | --- | --- |
| P0 | 通用配置保存可越过用户管理权限，改写角色权限 | 隔离请求已复现、修改已落盘 |
| P1 | DL/T 645 标准点位字典有功电能 DI 对应错位 | 源码、厂商协议表核对、字典断言 |
| P1 | DL/T 645 带符号 BCD 越界写入被接受，日期时间解析不完整 | 隔离测试已复现 |
| P1 | 历史异步写入合并中间样本，并可能丢失任务关联 | 隔离队列测试已复现 |
| P1 | 单独授予边缘计算管理权限，页面保存仍被通用配置接口拒绝 | 前后端调用链确认 |
| P1 | 配置应用失败后的回滚未检查结果，却提示恢复成功 | 源码确认，磁盘故障注入待做 |
| P2 | 告警/历史只读用户仍看到管理动作 | 模板与后端权限要求不一致 |
| P2 | 离开页面未保存保护不统一，历史任务草稿可能丢失 | 页面生命周期与路由代码确认 |
| P2 | 历史及实时趋势横轴等距排列，不能体现真实采样间隔 | 图表坐标代码确认 |
| P2 | 超过 5 万点的大项目模式没有保持各协议功能一致 | 大项目分支与点位表模板确认 |
| P2 | 大配置请求内存上限、前端构建与回归检查需要收紧 | 源码及构建/测试输出 |

P0 表示应优先关闭的权限风险；P1 为正确性/可靠性问题；P2 为功能一致性、容量保护和使用体验完善项。

## 1. 通用配置接口可越权改写安全配置（P0）

[internal/web/server.go:316](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/web/server.go:316>) 的 PUT 只要求 `config.manage`，随后解析并保存整个配置对象。用户/角色专用接口 [internal/web/user_management.go:80](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/web/user_management.go:80>) 则要求 `users.manage`，两条路径边界不一致。

隔离测试创建仅有 `config.manage` 的测试会话，提交包含非空 Users 的配置，把测试角色权限改为 `*`：

- 接口返回 HTTP 200。
- 临时配置文件重新读取后，该角色权限确实为 `[*]`。
- 没有向 EG100、219 或其他运行网关发出该请求。

这是已登录低权限账号的越权路径，不是未登录直接入侵。掩码保留函数仅在 Users 为空时保留整段 Security，不能作为权限检查。应同时审核精简配置保存、工程导入等整包配置入口，而不是只隐藏前端按钮。

建议：按模块服务端白名单更新字段；Security 只经用户管理入口修改；云连接、网络及维护等敏感字段执行各自权限校验。增加权限矩阵测试，覆盖“无权限字段夹带在整包配置中”的情况。

## 2. DL/T 645 标准点位 DI 有错位（P1）

[internal/collector/dlt645_dictionary.go:23](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/collector/dlt645_dictionary.go:23>) 当前将正向有功总电能配置为 `00000000`，反向有功总电能配置为 `00010000`。厂商公布的 2007 规约表对应关系如下：

| 数据项 | 应对应 DI | 当前代码 |
| --- | --- | --- |
| 组合有功总电能 | 00000000 | 被标作正向 |
| 正向有功总电能 | 00010000 | 00000000 |
| 反向有功总电能 | 00020000 | 00010000 |

核对来源：[安科瑞 AMC 国网中文电力仪表使用说明书，2007 规约地址表](https://test123.acrel.cn/download/common/upload/2025/379%20AMC%E5%9B%BD%E7%BD%91%E4%B8%AD%E6%96%87%E7%94%B5%E5%8A%9B%E4%BB%AA%E8%A1%A8%E8%AF%B4%E6%98%8E%E4%B9%A6V1.2-%E4%B8%AD%E6%96%87-2025.02.14%28V2.3%29.pdf)。本轮网页搜索返回了该厂商表格正文，直接 PDF 打开失败；未做完整标准全文审查。

影响：通信正常、质量 GOOD，也可能显示了另一个数据项，不能把 GOOD 当成字典正确的证明。费率项、无功项及 1997 映射需要成组核对，不能只改两个常量。

建议：分版本维护 DI、长度和小数位；字典测试独立于编码器；已有配置迁移先显示差异，不静默改动用户手工 DI。

## 3. DL/T 645 编解码边界缺陷（P1）

位置：[internal/collector/dlt645.go:553](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/collector/dlt645.go:553>)、[internal/collector/dlt645.go:621](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/collector/dlt645.go:621>)。

已复现：

- 3 字节带符号 BCD 编码数值 900001，没有拒绝越界，生成 `01 00 90`，随后解码成 **-100001**。最高位用于符号，但编码边界没有预留符号位。
- `datetime` 与 `time` 共用三字节格式化逻辑，六字节输入仅返回 `26:09:09`，日期丢失。
- 非法日期字节 `FA 99 26` 未报错，返回 `2026-99-160`。

建议：严格校验符号范围、有限数值、BCD 每个半字节与日期有效性；按声明的数据布局解析完整日期时间。写入边界用例必须覆盖正负数、最大值和越界，不能只有正常值样例。

## 4. 历史“变化保存”与最新值合并语义冲突（P1）

[internal/history/writer.go:9](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/history/writer.go:9>) 使用 500ms 刷新周期；待写队列以设备+点位为唯一键，新样本直接覆盖旧样本。[internal/app/app.go:613](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/app/app.go:613>) 是先按各任务策略筛选，再放入该队列。

隔离测试连续入队同一点的两条样本，分别关联 task-a、task-b：

- 待写数量只有 1。
- 只保留第二个值和 task-b。
- Dropped 仍为 0。

这是为性能做的最新值合并，不等同于保存每次变化。正常 1 秒轮询未必触发；高频变化、多个任务不同保存节奏、数据库暂时写不动时，影响更明显。短时脉冲可能消失；任务已经判为“本次应保存”，对应记录却未进入该任务。

建议：区分“周期快照”和“变化事件”；变化事件使用有界多样本队列，保留时间及任务归属；拥塞时明确计数、提示或暂停，不能无感覆盖。需新增真实 SQLite 落盘及故障恢复集成用例。

## 5. 细分角色权限还未打通（P1/P2）

边缘计算页面保存调用 [webui/src/components/EdgeComputeView.vue:277](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/EdgeComputeView.vue:277>) → [webui/src/stores/config.ts:73](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/stores/config.ts:73>) → PUT /api/config。页面允许具有 `edge.manage` 的账号编辑，但后端保存要求 `config.manage`，单独授予边缘计算权限不能完成保存。建议新增模块级保存接口，不能通过扩大该角色全局权限来解决。

同时：

- [webui/src/components/AlarmView.vue:65](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/AlarmView.vue:65>) 的新增、保存、删除、确认及开关未依据管理权限统一隐藏/禁用。
- [webui/src/components/StorageView.vue:197](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/StorageView.vue:197>) 及模板没有统一的 history.manage 控制。
- 后端告警确认、规则修改和历史修改接口有管理权限检查，因此只读用户会遇到“能点击，提交后 forbidden”，而不是这些接口本身都可越权。

本项是源代码功能审查，不是本轮浏览器截图实测。应使用管理员、运维、只读、单模块自定义角色分别走通页面与接口。

## 6. 配置回滚结果可能被误报（P1）

[internal/web/server.go:411](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/web/server.go:411>) 在应用新配置失败后，忽略恢复配置文件及重新应用旧配置的错误，统一返回“previous configuration was restored”。

磁盘写满、只读或旧配置再次应用失败时，该提示不可信，可能出现磁盘配置与运行配置不一致。本轮确认了忽略错误的代码路径，未对设备执行断电/磁盘故障。

建议复用 [internal/web/project.go:158](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/web/project.go:158>) 已有检查回滚错误的事务逻辑，明确区分“新配置失败但已恢复”与“恢复也失败”，并提供审计与安全状态提示。

## 7. 未保存提醒、草稿生命周期不统一（P2）

[webui/src/App.vue:168](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/App.vue:168>) 的跨模块离开保护只处理 cloudDirty。历史页面任务编辑保存在本地 historyForm，离开后组件卸载，回来重新请求服务器数据（[webui/src/components/StorageView.vue:122](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/StorageView.vue:122>)、[webui/src/components/StorageView.vue:239](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/StorageView.vue:239>)），未应用的新增/修改/删除会丢失。

边缘计算、设备配置还共用配置 store：应防止一个页面保存时带入另一个页面尚未确认的草稿。

建议统一“保存并离开 / 放弃 / 继续编辑”；每模块持有独立草稿与版本号；不依赖用户记住先点保存。

## 8. 趋势图横轴不是真实时间尺度（P2）

[webui/src/components/StorageView.vue:81](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/StorageView.vue:81>) 与 [webui/src/components/PointValueDetailDialog.vue:60](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/PointValueDetailDialog.vue:60>) 的 X 坐标使用样本序号，而不是时间差。

变化保存的相邻样本可能间隔 1 秒、1 分钟或更久，但图上距离相同；断线空窗也可能被连续折线跨过。悬浮时间戳正确，并不能修正图形上的时间关系。

建议按时间戳计算横轴，BAD/断线期间按质量断开线段。历史曲线当前是“当前页样本”，应另提供查询时间范围趋势接口，按像素量降采样，保留峰谷与质量状态。

## 9. 大项目模式没有完整覆盖所有协议（P2）

[internal/web/large_config.go:19](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/web/large_config.go:19>) 以项目总点数超过 **50000** 开启精简/分段模式；[webui/src/components/SmartGatewayView.vue:509](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/SmartGatewayView.vue:509>) 对设备切换到独立大表组件。

[webui/src/components/LargeDevicePointWorkspace.vue:33](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/webui/src/components/LargeDevicePointWorkspace.vue:33>) 主要专门处理 OPC DA、Modbus、IEC104；非 OPC DA 的多项字段是禁用输入框，DL/T 645 的 DI/标准点位工具、其他协议专用操作没有保持与普通点位页一致。

告警候选点列表直接遍历 store.devices[].points；大模式配置恰好省略这些数组，因此也需要服务端分页点位选择，不能继续依赖全量数组。

此项针对大规模测试/高配网关，不说明 EG100 应承载 5 万点。建议共享协议列定义和功能适配，避免两套页面长期分叉，并测试“大项目中的少量其他协议设备”。

## 10. 容量保护、测试及构建仍有欠缺（P2）

- 全配置接口允许 128MiB 请求，并一次性 ReadAll 再解码（[internal/web/server.go:370](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/internal/web/server.go:370>)）。对低内存 EG100 是明显容量风险，实际峰值可能高于原始配置体积。本轮未做 OOM 压测；应根据设备档位限制上传、预检规模并避免多份整包副本。
- 多个 TS 源文件旁存在同名 JS，部分导入不带扩展名。之前列宽相关已显式指定 .ts，但总体仍容易发生“改了 TS，构建引用旧 JS”。需要整理源文件/构建产物边界；这里不把所有重复文件都判为当前功能错误。
- 前端主 JS 638.81kB，构建产生 >500kB 分块提示。可将维护、AI、历史等低频模块按需加载；这只是优化项，不是导致崩溃的证据。
- 一个既有告警回归脚本因中文类型标签修改后仍匹配旧模板文本而失败。实际自动判断文案仍存在，是测试断言过时，不代表告警已失效。

## 尚未完善的功能

1. **DL/T 645 版本兼容验收**：字典仅 DI 分版本，长度/格式/小数位仍共用；1997 真表、后续帧、写凭据位置、异常帧、重连和负数边界需补齐。当前有模拟串口与基础帧测试，不等于原计划全部验收通过。
2. **告警条件覆盖**：现有自动推断仅基于配置 dataType；auto 被当作数值，未依据实际返回类型修正。数组没有元素/聚合阈值入口；stale 类型后端有概念，但前端自动选择不会生成该类型。数值越限与通信质量/数据陈旧应可分别配置，避免强行用一种条件代替另一种。
3. **数据库本体加密**：已实现备份加密与恢复，但运行中的 SQLite 仍通过普通 sqlite3 打开。不能把“加密备份”称为“历史数据库已加密”。是否做本体加密、设备密钥绑定、恢复密钥托管，要按敏感场景决定。
4. **诊断包采集链路证据**：现有包包含状态、健康、脱敏配置、系统信息、审计和文件清单，值得保留；但未打包最近采集异常明细、协议会话重连事件和脱敏报文摘要。现场超时问题仍难从单个包完成排查。
5. **发布与稳定性验收**：需要带 OPC DA / IEC61850 本地库的正式构建矩阵、ARM 实机回归、长时间内存/CPU观测、串口断开/设备重启、磁盘满/只读、外置存储拔出测试，以及可核对版本和回滚包的发布记录。

## 本轮测试结果

- Linux 隔离容器、CGO_ENABLED=1：`go test -race ./...` **通过**，包括 SQLite 相关包。
- 此命令未开启 `opcda`、`iec61850_mms` 构建标签，**未覆盖这两个本地库实现分支及 ARM 真机采集**。
- Windows 初次测试因 CGO=0 使用 SQLite stub 失败，是环境问题；已用上述 Linux 测试补足，不记为数据库产品缺陷。
- 前端 `npm run build`：类型检查与生产构建通过，有分块大小警告。
- 本轮执行 9 个非浏览器前端检查：写入错误、OPC DA 模式、命名、表格布局、类型标签、拖拽命中、实时值详情、串口协议共 8 个通过；告警通知脚本在过时模板正则断言处失败。
- 新增 6 个隔离边界诊断用例均暴露问题：权限、字典、BCD、datetime、非法日期、历史队列。它们是额外诊断，未加入正常测试目录，因此不矛盾于现有全量测试通过。
- 用例保存在 [docs/audit-20260909/overlay.windows.json](<C:/Users/18507/Documents/物联网平台/weikong-iot-platform/apps/gateway-go/docs/audit-20260909/overlay.windows.json>) 所指的 .go.txt 中，通过 Go overlay 映射运行；不接触运行网关，也不修改生产代码。
- 浏览器连接多次超时/无法附着，本轮没有取得新页面截图。页面审查技能要求基于本轮截图，因此全站布局、移动端、弹窗遮挡、实际点击行为的视觉审查未完成；旧截图不作为本轮实测证据。

## 建议实施顺序与验收

第一批：关闭配置越权，修正 DL/T 645 字典/编解码，解决历史样本完整性。
验收：隔离失败用例转为正式通过用例；低权限整包提交拒绝敏感变更；正确 DI 黄金样本；写入越界拒绝；多任务变化实际落盘且无静默丢弃。

第二批：模块级权限保存、可信回滚、只读按钮、统一草稿与离开提醒、真实时间轴。
验收：每类角色页面和接口一致；失败回滚分别展示真实结果；跨页草稿行为可预测；趋势体现真实空窗。

第三批：大模式协议一致性、诊断包增强、发布矩阵与 24–72 小时稳定性测试。数据库本体加密单独评估，不建议在协议正确性尚未完成时混入同一发布批次。


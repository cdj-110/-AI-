# 云平台轻盈替代版 QA — 2026-09-12

- final result: passed
- Target: 用户明确不接受深色科技卡片，要求换一种。按图片实现技能在既有拓扑中局部替换为大云图标与白色圆角节点，不保留上一版英文和底栏。
- Evidence: `webui/artifacts/topology-cloud-light/before.png`、`after.png`（803×796、同串口筛选与全部展开，在同输入对照），`mobile.png`（390×844）。
- Visual pass: 76px 现有 Tabler Cloud 图标以浅蓝填充和蓝色细轮廓展示，无图标方框；白底节点 28px 圆角、270px 宽，手机 260px。标题 18px、辅助与状态 10px，原中文内容及云状态计算保持。移除深色背景、英文和分栏，节点与下方网关有明确视觉区分；中心连线连续。
- Verification: 页面及窄屏无重叠，390px 文档宽 390px、云节点宽 260px；搜索与清空正常，控制台无错误。类型检查及生产构建通过，保留既有分包大小提示。未改连接逻辑、未进行实机连接状态切换测试。
- Preview retained: http://127.0.0.1:5192/scripts/fixtures/status-topology.html?topology （示例数据）。未部署 EG100；浏览器视口已恢复。

---

# 云平台科技感精修 QA — 2026-09-12

- final result: passed
- User target: 在已选云—网关—端口拓扑中强化云平台科技感。本次使用图片实现技能进行局部迭代，未改变任何协议、连接判定或后台配置。
- Visual evidence: `webui/artifacts/topology-cloud-tech/before.png`、`after.png`（803×796、串口筛选全部展开、同输入对照），`mobile.png`（390×844）。当前浅蓝卡片为基线，用户明确要求改变此节点视觉风格。
- Findings resolved: 浅蓝卡片缺少技术层次。改为深蓝青底色、细青色边框和轻光晕；图标使用现有 Tabler CloudComputing，内容增加辅助英文标签，底部将既有“远程管理 / 数据服务”分区。没有虚构状态指标、背景动画或手绘资产。
- Fidelity: 正文沿用现有字体，标题 17px/600、辅助标签 9px、状态与底栏 10px；白/浅青文本与深底对比清晰。桌面 300px、窄屏 280px，与网关同宽；14px 圆角、图标区 50px，连线仍居中连续。新增底栏带来的高度变化属于本次指定节点的视觉调整。
- QA: 同屏前后截图及窄屏细节无重叠、文字完整。390px 文档 scrollWidth=390，云卡宽 280px。搜索、恢复列表正常，控制台无错误。仅正常云连接进行了界面验证，其他状态保留原计算分支并使用对应深色底上的高对比文本。
- Verification: 类型检查、生产构建及拓扑逻辑回归通过，只有既有大分包提示。视口已恢复，示例预览保留；未部署 EG100。

---

# 云平台卡片精修 QA — 2026-09-12

- final result: passed
- Scope: 延续已选拓扑设计，仅美化顶部云平台卡片。网关、端口、设备、折叠和云状态计算逻辑不变。
- Visual truth / evidence: `webui/artifacts/topology-cloud/before.png` 与 `after.png`（803×796，同一串口筛选、全部展开，同输入对照）；`mobile.png`（390×844）检查局部细节与适配。示例数据预览 http://127.0.0.1:5192/scripts/fixtures/status-topology.html?topology 。
- [P2 resolved] 原卡片图标、说明、状态分散且与主网关比例不协调。改为浅蓝底、44px 白色图标区、16px 标题、11px 下方说明，右上角 10px 状态标签；与深蓝网关桌面同宽 300px，窄屏同宽 280px。使用现有 Tabler Cloud 图标，不新增栅格资产或绘制硬件图形。
- Fidelity pass: 14px 圆角、轻阴影和现有蓝灰字体体系一致；正常/失败/未知/停用各有状态文字和底色，状态计算未修改。内容完整可读，云卡片较原版更矮，上下连线仍居中连续。移动文档宽 390px，无整页横向溢出。
- Verification: 搜索“三相”定位设备、清空恢复串口列表；浏览器控制台无错误；类型检查与最终前端构建通过，仅既有分包警告。仅正常连接状态进行了浏览器实测，其他状态沿用既有计算与类名分支，未进行实机连接切换。
- Deployment: 未部署 EG100，未改任何实机配置；浏览器视口重置，预览保留。

---

# 拓扑折叠控件精修 QA — 2026-09-12

- final result: passed
- Scope: 用户要求美化折叠按钮。以当前已实现页面为基线，仅修改端口折叠和顶部批量折叠控件；数据与折叠逻辑不变。
- Source / implementation: `webui/artifacts/topology-fold/before.png`、`after.png`（803×796，同一串口筛选、全部展开、同输入对照）；`collapsed.png` 检查关闭状态；`mobile.png` 为 390×844 检查。
- [P2 resolved] 旧控件继承全局按钮边框及高度，看起来高窄且突兀。改成无边框 32×32 点击区域，箭头展开向下、折叠向右；悬浮与折叠时用浅蓝圆形底色；键盘焦点显示轮廓。保留动态 title/aria-label/aria-expanded，减少动画偏好时停用旋转过渡。
- [P2 resolved] 顶部两枚厚边框按钮占用过多视觉空间。合并成浅灰工具组，文字与 Tabler 双箭头配对，单按钮高 30px，组宽 176px，悬浮白底。
- Fidelity: 沿用现有字体、蓝灰色、圆角；不改变网关、设备、状态颜色与文案。图标均为现有库矢量，无图片生成或自绘素材。对照中端口卡片未改变高度，正文及连线无重叠。
- Verification: 鼠标折叠单串口后设备数 2→1、aria-expanded=false；Enter 展开恢复 2；全部折叠为 0、全部展开恢复 2。实测按钮 32×32、border=0。手机文档宽 390px，无整页横向溢出，拓扑内部保留横向滚动；控制台无错误。类型检查、生产构建及拓扑逻辑回归通过，保留原分包大小警告。
- Local preview: http://127.0.0.1:5192/scripts/fixtures/status-topology.html?topology 。示例数据、未部署 EG100；视口重置，预览保留。

---

# 边缘网关主节点精修 QA — 2026-09-12

- final result: passed
- Scope / visual truth: 当前紧凑拓扑为修改基线，仅调整中心网关节点。用户要求美化网关，其余云—网关—端口—设备结构保持。使用现有 Tabler CPU 图标替换 Wi-Fi 路由器图标，没有新增图片、接口或运行状态含义。
- Preview: http://127.0.0.1:5192/scripts/fixtures/status-topology.html?topology （示例数据，未部署）。
- Evidence: `webui/artifacts/topology-gateway/before.png` 与 `desktop.png`（均 1600×1000、全部资源展开、同一输入对照）；`mobile.png`（390px 宽）。一次局部截图及一次完整截图捕获空白，已改用同一浏览器的原生截图验证；空白 `detail.png` 不作为 QA 证据。
- [P2 resolved] 网关主次层级不清：改为深蓝主卡片，白色“边缘网关”标题，编号降为辅助文本，底部三栏接入统计。桌面宽 300px、高约 113px；移动宽 280px。新增高度约 27px 属于本轮主节点强调，未扩大其他设备卡片。
- Typography: 沿用页面字体，标题 17px/600（移动 16px），编号 11px，统计文字 10px/数字 12px；编号可换行，不截断。前景白/浅蓝与深蓝底对比清晰。
- Layout / tokens: 身份区 15px/18px 内边距，44px 芯片图标，独立统计底栏及细分隔线，14px 圆角和轻阴影；中心连线连续，既有端口卡片和 56px 设备卡片不变。
- Asset / content: 图标库矢量清晰，未绘制假硬件。编号和串口/网口/设备计数仍取真实配置；筛选设备时统计仍表示全网关规模，不添加无数据依据的“在线”状态。
- Verification: 手机文档 scrollWidth=390、gateway=280，无页面横向溢出；搜索“三相”正常，清空恢复 5 设备；控制台无错误。拓扑逻辑回归通过、类型检查和前端生产构建通过（既有分包大小警告）。视口已重置，预览已保留。本轮无 EG100 写入或部署。

---

# 紧凑设备卡片与拓扑精修 QA — 2026-09-12

- Final result: passed（本地前端，未部署 EG100）。
- Reference: 延续用户云—边—端示意图，按本轮要求缩小设备卡片、仅用指示灯呈现状态。沿用图片实现技能和现有 Tabler 图标，不改变拓扑数据、设备健康判断及采集/转发层级。
- Preview: http://127.0.0.1:5192/scripts/fixtures/status-topology.html?topology ，使用示例数据。
- Evidence: `webui/artifacts/topology-compact/before.png`、`desktop.png`（1600×1000），`mobile.png`（390×844）；原参考图与最终桌面截图在同一输入中对照。前后桌面均为全部资源、全部展开。
- [P2 resolved] 设备卡片原 126px 高且重复展示状态、地址、ID、点数，改为 56px 高、最大 270px 宽的单行卡片，只显示图标、名称、状态灯；名称过长省略，完整名称/状态/协议/地址/点数通过 title 和 aria-label 保留。
- [P2 resolved] 状态色重复染色卡片边框和图标。改为中性色卡片与图标，仅状态灯着色；顶部图例对应。未知灯为空心、停用灰色、转发蓝色（不代表在线）。
- [P2 resolved] 端口区松散。端口改为统一 70px 高卡片；云、网关、端口、设备中心线连续；采集/转发分组分隔符避让连线，缩短上行和分叉间距。
- Visual pass: 桌面全部 5 个设备与空闲端口在一屏内可见，字体、边距、圆角、图标大小一致，无重叠；移动端搜索单设备无文档横向溢出。390px 全部资源时文档 scrollWidth=390，只有拓扑画布横向滚动（346px 可视 / 980px 内容）。
- Interaction pass: 单端口折叠隐藏设备，再次展开恢复；全部折叠 0 设备、展开恢复；搜索“三相”定位 RS485-2；网络筛选包含网口1及空网口2；采集点击输出 s2/c2/meter，转发点击输出 n1/f1/forward。无浏览器控制台错误。
- Verification: `node --experimental-strip-types scripts/verify-topology.mjs` 通过；`npm run build` 类型检查及生产构建通过，仅保留既有大分包提示。
- No external writes: 本轮未修改 EG100 配置、未重启服务或部署。测试浏览器视口已恢复，预览保留。

---

# 云—边—端拓扑结构 QA — 2026-09-12

- Source visual truth: `C:/Users/18507/AppData/Local/Temp/codex-clipboard-3d31e229-5a76-4481-8efa-9d44639b83c1.png`，540×405。
- Implementation: `http://127.0.0.1:5192/scripts/fixtures/status-topology.html?topology`，仅示例数据，不是实机运行截图。
- Evidence: `webui/artifacts/topology-tree/desktop.png`、`serial-focus.png`、`mobile.png`、`forward-focus.png`。
- Browser: Codex 内置浏览器。桌面 CSS 1600×1000；局部检查在默认 670px 宽浏览器；窄屏 CSS 390×844。桌面图片与源图在同一输入中展示，随后源图与清晰的串口分支截图再次同输入对照。参考图是概念结构，未拉伸为整页，也未把其虚构传感器当作实际设备。
- Scope: 用户要求参考结构并美化，而非逐像素复制。保留顶部云平台、居中网关、向下水平分叉；增加实际串口/网口中间层、通道分组、状态及点位数量。设备图形统一采用项目现有 Tabler 图标，不生成设备照片或自绘图标。

## Findings / comparison history

- [P2, resolved] 单一搜索结果占满画布导致设备卡片过宽；端口及设备区域限制到 360px，并居中。已重新检查串口和单设备状态。
- [P2, resolved] 端口标签与设备卡片之间连接线不连续；补充端口到设备的竖向总线，缩短端口区高度，通道文字使用画布背景避让线。`serial-focus.png` 为修复后对照。
- [P2, resolved] 窄屏“重新加载”按钮换行；设置不收缩和不换行。页面宽度测试 390px，文档 scrollWidth 390px，拓扑画布本身可横向滚动（346px 可视 / 1060px 内容）。不把整页控件推出屏幕。
- 首次桌面捕获出现浏览器缩放模糊，未用其判断字体。后续同尺寸清晰截图覆盖 desktop.png，并增加原生宽度串口细节截图。

## Required fidelity surfaces

- Fonts / typography: 延用项目中文字体。层级为标题20px、网关17px、端口14px、设备12px、辅助10–11px。长地址可换行，网关编号不被截断。
- Spacing / layout: 云和网关居中，端口等宽分布，分叉线位于各端口中心；通道按采集/转发分开。多设备垂直延伸，允许页面纵向滚动；窄屏仅画布横向滚动。保留空闲端口。
- Colors / tokens: 蓝色云及串口、淡紫网口，绿色正常、黄色部分异常、红色异常、灰色未知；状态均有文字，不依赖颜色。
- Image quality / assets: 图标同一库、统一线宽，清晰无失真；参考图设备插画按用户要求作为结构参考，不照搬不适用的虚构设备。连接线是布局关系，不是替代图片的插画。
- Copy / content: 明确配置拓扑不是自动物理探测，转发配置不代表远端在线。云状态复用已授权的主页面状态，缺少状态时显示未知，不额外调用 status.view 接口破坏 config.view 权限。

## Interaction verification

- 全部折叠后 0 个设备卡片，全部展开恢复；串口筛选只显示 RS485-1/2；搜索唯一电表成功。
- 点击电表返回正确的资源/通道/设备选择；转发设备同样定位到网口1/转发通道。
- 正常、部分异常、异常及转发状态在示例中可见；空网口有明确说明。
- 内置浏览器 console error 为空。拓扑结构/空闲与未绑定资源/状态/权限静态回归测试通过；正式前端构建通过。
- 本次没有部署新前端到 EG100。实机磁盘清理独立完成并核对运行状态。

final result: passed

---

# 分级声音紧凑布局 QA — 2026-09-12

- Source visual truth: `C:/Users/18507/AppData/Local/Temp/codex-clipboard-4805ca07-2c61-4754-8a96-2554759967e4.png`（563×271 像素，四限配置局部）。
- Implementation: `http://127.0.0.1:5187/?page=alarms`，声音设置；`webui/artifacts/alarm-audio/sound-desktop.png`（1440×900）、`sound-focus.png`（1105×320）、`sound-mobile.png`（390×844）。
- Browser: Codex 内置浏览器。桌面 CSS 1440×900、手机 CSS 390×844，截图按 CSS 像素对照；源图是局部而非整页，不拉伸至整页。此任务借用逐行配置模式，不把声音设置内容替换为四限阈值。
- State: 提示／警告使用工业分级蜂鸣，严重已上传 2 秒测试 WAV；各级开启、持续循环，说明折叠。原四限编辑区域仍保留单行启用／阈值／等级。
- Full-view comparison: 源图与桌面渲染图同次输入对照；桌面三等级行完整显示，原三张大卡片移除。手机由多列变为带字段标签的两列，不压缩输入框；允许纵向滚动，无整页横向溢出。
- Focused comparison: `sound-focus.png` 对照源图的行式边框、34px 输入框、启用复选框、列对齐及底部说明。音频上传占额外一条辅助行，内容不同故行高较四限多约 30px，这是有意保留的文件操作空间。

## Findings / required fidelity surfaces

- Fonts and typography：复用现有中文 UI 字体，正文 12px、表头及辅助文字 11px；音频长文件名省略并可悬浮查看，主标签无截断。
- Spacing and layout rhythm：统一浅边框圆角容器，三等级逐行对齐；当前桌面所有操作可见，手机下标签可读且控件不重叠。
- Colors and tokens：沿用蓝色选择／操作、浅灰边框和三级自定义色；仍显示文字等级，不仅靠颜色区分。
- Image quality and assets：参考区域无照片或插画；使用真实原生表单元素，无新增装饰图片或自绘图标。
- Copy and content：保留音色、循环、音量、颜色与试听，明确本地存储、2 MB／30 秒限制和错误回退。详细解释默认折叠。
- No actionable P0/P1/P2 findings remain. 原四限片段本身已符合所示紧凑布局，无需改变判定或重新设计。

## Comparison history and checks

- 旧结构为三张纵向设置卡片，改为单容器三行，已通过桌面及窄屏截图检查。
- 浏览器底层局部截屏曾输出空白，不作为通过依据；最终使用内置浏览器完整截图，并从已确认的桌面截图裁出焦点区域。
- 音频上传、保存、刷新保留、试听、错误输入、完整循环、不叠加、静音停止及异步取消均有测试。应用页面未见运行异常；本地测试脚本开发期语法错误已修复并重新运行通过。
- 未进行 EG100 部署或真实告警确认／恢复操作。

final result: passed

---

# 登录安全紧凑布局设计 QA（既有记录保留）

- Source visual truth: `C:\Users\18507\AppData\Local\Temp\codex-clipboard-7092a5ba-ef62-44cd-b2f4-ed49b838a216.png`
- Implementation capture: Codex in-app browser tab 1（本地网关真实页面，响应式窄屏视口）
- State: 用户管理 / 登录安全；TOTP 未启用；HTTPS 当前使用 HTTP；HTTPS 详情展开

## Findings

- No actionable P0/P1/P2 findings remain.
- TOTP 与 HTTPS 均收敛为约 72 px 的摘要行，默认只显示状态、名称、用途和主要操作。
- 次要说明、账号、验证方式、访问地址、证书有效期及指纹放入“详情”区域，默认折叠。
- 同一时间只展开一个详情区域，减少页面纵向占用；详情按钮保留 `aria-expanded` 状态。
- 窄屏下摘要区、详情按钮和主操作自动换行，长地址与证书指纹可断行，不会重叠或撑破卡片。

## Interaction verification

- 实测依次展开 MFA 与 HTTPS 详情时，前一个详情会自动收起。
- 关闭状态下两张卡片保持紧凑；展开状态下信息层级、边界和长文本换行正常。
- Vue TypeScript 检查及 Vite 生产构建通过。

final result: passed

---

# 告警规则启用状态与提示音设计 QA

- Source visual truth: `C:\Users\18507\AppData\Local\Temp\codex-clipboard-0830b8bc-5202-4fc8-8669-f1eec762e10c.png`
- Implementation screenshot: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-alarm-rule-switch-implementation.png`
- Full comparison: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-alarm-rule-switch-comparison.png`
- Focused comparison: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-alarm-rule-switch-focus.png`
- State: 告警管理 / 告警规则；两条规则，一条启用、一条停用；1 条 CRITICAL 活动告警
- Browser viewport/capture: 1312 × 912 CSS px, device density 1
- Source pixels: 1920 × 911
- Implementation pixels: 1312 × 912
- Normalization: full-view comparison将源截图等比缩至 1312 × 623，实施截图截取相同 1312 × 623 区域；focused comparison对规则表区域分别裁切并统一到 1019 × 220 后上下拼接。源截图与本地浏览器宽度不同，因此不对侧栏宽度和横向密度做像素级结论。

## Findings

- No actionable P0/P1/P2 findings remain.
- 启用列已从首列移动到最后一列，表格阅读顺序符合“规则信息 → 操作 → 启用状态”。
- 原生方形复选框已替换为现有网关风格的 42 × 22 px 滑动开关；开启为绿色，关闭为灰色，并保留键盘焦点样式和 `role="switch"` 语义。
- 停用规则整行使用浅灰背景、灰色文字和灰化等级标签；开关、编辑和删除仍可操作，便于重新启用或维护规则。
- 提示音属于行为状态，不在静态截图中表达。实现按一次轮询内最高告警等级播放：WARNING 双音、CRITICAL 三音；首次加载的已有告警不响，轮询到同一 EventID 不重复响。

## Interaction verification

- 点击已停用规则的滑动开关后，规则恢复启用并取消整行灰化；再次关闭后灰化恢复。
- 关闭状态下的编辑、删除和重新启用控件仍可访问，不会被整行状态误判为不可用。
- 告警规则表头顺序、两个开关的已启用/已停用可访问名称均已通过浏览器 DOM 检查。
- 页面控制台仅有 Vite 连接与热更新 debug 日志；无 error 或 warning。
- `npm run test:alarm-notifications` 通过；`npm run build` 通过。

## Comparison history

- Initial browser inspection found disabled-row `aria-disabled` caused辅助技术把编辑、删除和重新启用控件一并标记为不可用（P1）。
- Removed row-level `aria-disabled`; retained the visual disabled class only. Re-ran the switch interaction: disabled → enabled → disabled succeeded, and all controls remain accessible.
- Final full-view and focused comparisons show no remaining P0/P1/P2 visual or interaction issues.

## Follow-up polish

- P3: If field testing shows the sound is too prominent in a noisy control room, add an optional local volume/mute preference without changing the server-side alarm state.

final result: passed

---

# AI 助手精密蓝图视觉设计 QA

- Source visual truth: `C:\Users\18507\.codex\generated_images\01a03251-3f05-7d71-ac1f-14606c8ada63\exec-d9827ea6-57e2-4c8b-a4f3-c25753e872d0.png`
- Implementation screenshot: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-ai-blueprint-implementation-final.png`
- Full comparison: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-ai-blueprint-comparison-final.png`
- State: AI 助手悬浮弹窗 / 智能对话 / 未启用；空对话与五个快捷问题可见
- Browser viewport: 670 × 796 CSS px 与 1280 × 800 CSS px；device density 1
- Source pixels: 1150 × 1367；implementation pixels: 670 × 796
- Normalization: 源视觉稿高质量缩放到 670 × 796，与相同 CSS 视口的浏览器实施截图并排比较；两者宽高比一致，无裁切和设备框差异。

## Findings

- No actionable P0/P1/P2 findings remain.
- Fonts and typography: 沿用网关现有中文系统字体，标题、英文眉题、Agent 说明和等宽 `PROMPT` 微文案形成明确层级；正文换行与源视觉一致，未出现截断或重叠。
- Spacing and layout rhythm: 页头、等宽双页签、工具栏、快捷问题和底部输入区按源视觉重排；670 px 窄屏下五个快捷问题保持单行，弹窗仍可滚动但不再显示重复滚动条。
- Colors and visual tokens: 使用现有蓝色主色、低饱和金色状态色和冷白面板；蓝图纹理、细蓝边界及轻阴影增强科技感，同时保持工业现场可读性。
- Image quality and asset fidelity: 独立生成并接入 `webui/public/assets/ai-blueprint-surface.png`，用于低对比度网格与数据线路背景；资源清晰、无文字、无拉伸。所有操作图标来自 `@tabler/icons-vue`，没有用字符或手绘图形代替。
- Copy and content: 保留原有 AI 助手、DeepSeek V4、PDF 生成配置、快捷问题、Agent 安全说明和输入提示，不新增虚构能力。
- 发送按钮在当前“未启用”真实状态下保持灰色禁用，而源视觉稿显示蓝色可用；这是数据状态约束，不是视觉缺失。

## Interaction verification

- “智能对话”与“PDF 生成配置”页签双向切换成功，PDF 目标选择与上传区域正常显示。
- 关闭按钮可关闭弹窗，右下角悬浮入口可再次打开，组件状态保留。
- 670 × 796 响应式视口下页头、五个快捷问题和输入区完整可见；弹窗外背景正确虚化。
- 1280 × 800 桌面视口下输入框与发送按钮固定保持在弹窗可视区域内，不需要滚动才能继续对话。
- 浏览器控制台无 error。
- `npm run test:ai-blueprint-theme`、悬浮弹窗验证脚本和 Vite 生产构建通过。

## Comparison history

- First comparison found the fifth quick prompt wrapping to a second row and the inner vertical scrollbar remaining visible (P2).
- Replaced the narrow-screen prompt group with a five-column max-content grid, tightened icon/text spacing, and visually hid the inner scrollbar while preserving scroll behavior. Added the source-matching status dot from the shared icon library.
- Desktop responsive inspection then found the persistent composer falling below the initial modal viewport at 1280 × 800 (P2). Converted the modal body to a constrained two-row grid and moved overflow into the workspace/message region; the composer and send action now remain visible.
- Final same-viewport comparison shows the information hierarchy, page proportions, interaction affordances and light blueprint styling aligned with the selected visual target; no P0/P1/P2 differences remain.

## Focused comparison

- A separate crop was not required: at 670 × 796 the full-view comparison keeps header text, status, tabs, all quick prompts and composer labels legible at 1:1 density.

## Follow-up polish

- P3: 现场启用模型后可再截取一次“发送可用”状态，确认蓝色主按钮与真实对话消息状态的视觉平衡。

final result: passed

---

# 实时值详情按协议展示与标题防重叠 QA

- Source visual truth: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-live-value-neutral.png`
- Implementation screenshot: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-protocol-detail-modbus.png`
- Full-view comparison: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-protocol-detail-comparison.png`
- State: 智能网关 / Modbus RTU 点位 / 实时值详情；窄屏响应式弹窗
- Browser viewport: 670 × 796 CSS px；device density 1
- Source pixels: 670 × 764；implementation pixels: 670 × 796；同宽并排比较，无密度缩放

## Findings

- No actionable P0/P1/P2 findings remain.
- 标题区拆分为协议标签、名称和独立标识符行；标识符允许断行，标题区使用内容高度参与滚动布局。
- 修复前标题网格行被压缩为 56 px，标识符底部 120.9 px 与运行信息顶部 79 px 重叠；修复后标题区高度 126.9 px，标识符底部 120.9 px，运行信息顶部 149.9 px，`separated = true`。
- 通用运行信息仅保留当前值、质量、实际类型、源时间、采集时间和状态/诊断。
- 协议详情独立成区；浏览器实测 Modbus RTU 显示功能码、寄存器地址、数量、字节序和字序，不再显示 OPC DA Item ID。
- 字体、间距、颜色和卡片边界沿用现有网关设计系统；没有新增图片、图标或品牌资产。

## Protocol verification

- 自动验证覆盖 Modbus、OPC DA、OPC UA、Siemens S7、IEC104 和 IEC61850 的字段集合。
- OPC DA 使用 Item ID / Access Path / 访问权限 / 请求类型 / 采集方式。
- OPC UA 使用 Node ID / 采集方式 / 请求数据类型。
- S7 使用存储区域 / DB 号 / 字节地址 / 位索引 / 字节序。
- IEC104 使用 IOA / 点类型 / 公共地址；IEC61850 使用对象引用 / FC；GOOSE 使用对象引用 / 数据集索引及可用的数据集引用。

## Comparison history

- First implementation capture found the header grid track shrinking while the identifier overflowed into the common status cards (P1).
- Changed the dialog to content-sized grid rows and restored intrinsic header minimum height.
- Post-fix capture and browser measurements confirm the name, identifier and status cards are separated with no overlap.

## Verification

- Protocol detail verification script passed.
- Vue TypeScript check passed.
- Vite production build passed.

final result: passed

---

# 实时值详情中性配色与类型标签 QA

- Source visual truth: `C:\Users\18507\AppData\Local\Temp\codex-clipboard-6dd4df91-70e1-4e19-91b3-5ba0501f78bb.png`
- Implementation screenshot: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-live-value-neutral.png`
- State: 智能网关 / 点位配置 / 实时值详情；单数值点位；GOOD 质量
- Browser capture: 670 × 764 px responsive viewport

## Findings

- No actionable P0/P1/P2 findings remain.
- 仅当前值保留低饱和蓝色，质量保留 GOOD/UNCERTAIN/BAD/OFFLINE 语义色；采集方式、实际类型和时间信息统一为白底中性灰，减少颜色干扰。
- 实际类型从整张紫色卡片改为灰色胶囊标签，实测显示 `float64`，标签使用等宽字体并保持紧凑。
- 移动端实时值补充可点击与键盘访问入口，窄屏下也能打开同一详情弹窗。

## Interaction verification

- 浏览器实测类型卡背景为白色，类型标签为 `inline-flex`、浅灰底和灰色边框；采集方式与采集时间卡均为白底中性样式。
- 当前值与质量语义层仍然明确，趋势图、完整值和关闭操作未受影响。
- 实时值详情验证脚本与 Vue TypeScript 检查通过。

final result: passed

---

# 实时值详情颜色层级设计 QA

- Source visual truth: `C:\Users\18507\AppData\Local\Temp\codex-clipboard-6dd4df91-70e1-4e19-91b3-5ba0501f78bb.png`
- Implementation screenshot: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-live-value-hierarchy.png`
- Focused comparison: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-live-value-hierarchy-comparison.png`
- State: 智能网关 / 点位配置 / 实时值详情；单数值点位；GOOD 质量；趋势图右侧样本悬浮
- Browser viewport/capture: 1280 × 720 CSS px, device density 1
- Source pixels: 1920 × 911；source dialog crop: 980 × 758
- Implementation pixels: 1280 × 720；implementation dialog crop: 980 × 676
- Normalization: 两个弹窗内容区域均裁切为 980 px 宽并上下拼接。源图和实现的采样数量、数值及浏览器高度不同，因此比较信息层级、色彩、边界、排版和交互状态，不比较曲线形状或弹窗可见高度。

## Findings

- No actionable P0/P1/P2 findings remain.
- 当前值使用低饱和蓝色底、蓝色侧边强调和更大字号，成为首要阅读目标。
- GOOD 质量使用绿色语义层；UNCERTAIN、BAD 和 OFFLINE 已配置对应黄色、红色语义层，保持现场状态辨识一致。
- 实际类型使用低饱和紫色；采集方式与时间卡保持白底，仅用细色条区分，避免六张卡片同时争夺注意力。
- 趋势区和完整值区分别使用蓝色、深灰顶部边界，并统一白色面板和轻阴影；弹窗底色后退为浅灰蓝，层次清晰但不花哨。
- 字体沿用当前网关设计系统；标题、指标、辅助文字的字号和字重层级明确。没有新增或替代任何图片、图标和品牌资产。
- 文案内容保持不变，颜色调整没有改变点位数据、单位、时间戳或操作含义。

## Interaction verification

- 趋势图悬浮仍显示点位、当前值、单位和毫秒时间戳，右侧边缘提示框正常向左避让。
- 关闭按钮 hover 状态提高边界和文字对比度。
- 浏览器控制台无 error 或 warning。
- 实时值详情验证脚本、Vue TypeScript 检查及 Vite 生产构建通过。

## Comparison history

- First enhanced-hierarchy capture found the header metric code overlapping the first status card (P2).
- Added a minimum header height so the point identifier remains inside the header and the status grid begins below it.
- Post-fix browser measurement: metric code bottom 125 px, first card top 129 px, `separated = true`; revised screenshot shows no overlap.

## Follow-up polish

- P3: 现场若更偏好保守配色，可将实际类型的紫色饱和度再降低一级，不影响当前信息层级。

final result: passed

---

# 历史数据变化趋势图设计 QA

- Source visual truth: `C:\Users\18507\AppData\Local\Temp\codex-clipboard-25b58f27-4a54-441c-bc19-13f2a9c35554.png`
- Implementation screenshot: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-history-trend.png`
- Focused comparison: `C:\Users\18507\Documents\物联网平台\weikong-iot-platform\apps\gateway-go\design-qa-history-trend-comparison.png`
- State: 历史数据 / 查询与趋势；噪声仪 `noise-meter-1`；点位 `NOISE_DB`；100 条当前页记录；悬浮在一个历史采样点上
- Browser viewport/capture: 1280 × 720 CSS px, device density 1
- Source pixels: 1592 × 350
- Implementation pixels: 1280 × 720；focused implementation crop: 931 × 351
- Normalization: 源图等比缩放到 931 px 宽；实施截图裁切相同趋势图区域并上下拼接。源图与实现数据窗口不同，因此比较布局、层级、颜色和交互状态，不比较曲线形状。

## Findings

- No actionable P0/P1/P2 findings remain.
- 保留原页面的紧凑信息密度，在蓝色折线下增加轻量面积填充、柔和投影、虚线网格和白心数据点，趋势层次更清晰且没有引入额外卡片。
- 图表标题区增加单位标签；Y 轴同步标注单位，避免用户把数值与量纲分开理解。
- 悬浮状态包含横纵参考线、聚焦光晕和结构化提示框，提示框显示点位名称、标识符、当前值、单位及完整毫秒时间戳。
- 右侧边缘悬浮时提示框自动向左翻转，实测完全包含在图表区域内。

## Interaction verification

- 悬浮内容实测为：`噪声值 / NOISE_DB / 43.08 dB / 2026/09/02 12:11:20.325`。
- 图表右侧边缘提示框使用 `flipped` 状态，边界检测结果为 contained = true。
- 刷新后的独立浏览器标签页无 error 或 warning。
- 29 项历史数据前端验证通过；Vue TypeScript 检查通过；Go `internal/web` 测试通过；Vite 生产构建通过。

## Comparison history

- First hover inspection exposed the previous time formatter only rendering fractional digits in the tooltip.
- Added explicit year/month/day/hour/minute/second fields and retained `fractionalSecondDigits: 3`.
- Re-ran hover and edge-state checks; final tooltip displays the full timestamp and remains within bounds.

final result: passed

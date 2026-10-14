import fs from "node:fs/promises";
import path from "node:path";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const outputDir = path.resolve("outputs");
const outputPath = path.join(outputDir, "物联网平台_网关绑定进度看板_20260613.xlsx");

const tasks = [
  ["REQ-001", "功能说明和任务拆分", "P0 需求梳理", "产品/研发", "2026-06-13", "已完成原型", "P0", "需求", "无", "公司开发能按文档拆任务"],
  ["BE-001", "完善 FactoryGateway 出厂网关数据模型", "M1 原型闭环", "后端", "2026-06-14", "已完成原型", "P0", "后端", "无", "可保存 HardwareId、SN、密钥哈希、绑定状态、上线时间"],
  ["BE-002", "登记出厂网关并生成 SN、DeviceSecret、BindCode", "M1 原型闭环", "后端", "2026-06-14", "已完成原型", "P0", "后端", "BE-001", "重复 HardwareId 返回错误，成功后返回激活信息"],
  ["BE-003", "客户通过 SN + BindCode 绑定网关", "M1 原型闭环", "后端/前端", "2026-06-15", "已完成原型", "P0", "后端", "BE-001", "绑定后创建客户租户下的网关设备"],
  ["BE-004", "重置未绑定网关的绑定码", "M2 产品化", "后端", "2026-06-16", "已完成原型", "P1", "后端", "BE-001", "只允许未绑定设备重置，重置后旧绑定码失效"],
  ["BE-005", "禁用和启用出厂网关", "M2 产品化", "后端", "2026-06-18", "待开发", "P1", "后端", "BE-001", "禁用后 MQTT 鉴权拒绝，启用后恢复"],
  ["BE-006", "超级管理员绑定时显式选择租户", "M2 产品化", "后端/前端", "2026-06-18", "待开发", "P1", "后端", "BE-003", "避免默认绑定到错误租户"],
  ["BE-007", "出厂网关操作日志", "M2 产品化", "后端", "2026-06-19", "开发中", "P1", "系统日志", "操作日志模块", "记录登记、绑定、重置、禁用、启用"],
  ["BE-008", "DeviceSecret 轮换", "M3 量产运维", "后端", "2026-06-25", "待开发", "P2", "安全", "BE-001", "旧密钥失效，新激活文件生效"],
  ["MQTT-001", "MQTT 鉴权支持 factory:{HardwareId}", "M1 原型闭环", "后端", "2026-06-14", "已完成原型", "P0", "MQTT", "BE-001", "工厂通道可使用 DeviceSecret 连接"],
  ["MQTT-002", "MQTT ACL 限制出厂未绑定主题", "M1 原型闭环", "后端", "2026-06-14", "已完成原型", "P0", "MQTT", "MQTT-001", "未绑定阶段只能发布工厂心跳"],
  ["MQTT-003", "绑定后正式设备主题授权", "M2 产品化", "后端", "2026-06-19", "开发中", "P0", "MQTT", "BE-003", "绑定后允许发布正式网关和子设备主题"],
  ["ING-001", "订阅工厂心跳主题", "M1 原型闭环", "数据接入", "2026-06-14", "已完成原型", "P0", "数据接入", "MQTT-001", "收到 weikong/factory/+/heartbeat"],
  ["ING-002", "更新出厂网关首次上线和最近上线时间", "M1 原型闭环", "数据接入", "2026-06-14", "已完成原型", "P0", "数据接入", "ING-001", "正确更新 firstSeenAt 和 lastSeenAt"],
  ["ING-003", "已绑定网关同步正式设备在线状态", "M1 原型闭环", "数据接入", "2026-06-15", "已完成原型", "P0", "数据接入", "BE-003", "绑定后工厂心跳能带动设备在线"],
  ["ING-004", "遥测数据租户隔离", "M2 产品化", "数据接入", "2026-06-20", "开发中", "P0", "数据接入", "BE-003", "遥测只能写入绑定租户设备"],
  ["ING-005", "离线判定统一", "M2 产品化", "数据接入/前端", "2026-06-20", "开发中", "P1", "在线状态", "ING-002", "设备列表、详情、弹窗状态一致"],
  ["GW-001", "读取并展示 HardwareId", "M1 原型闭环", "网关端", "2026-06-14", "已完成原型", "P0", "Go 网关", "目标硬件", "本地页面能显示 HardwareId 和来源"],
  ["GW-002", "上传或粘贴 activation.local.json", "M1 原型闭环", "网关端", "2026-06-14", "已完成原型", "P0", "Go 网关", "GW-001", "保存到本地并校验 JSON 格式"],
  ["GW-003", "保存激活文件后自动应用 MQTT 连接", "M1 原型闭环", "网关端", "2026-06-15", "开发中", "P0", "Go 网关", "GW-002", "无需重启即可连接云平台"],
  ["GW-004", "激活通道和手动 MQTT 通道独立开关", "M2 产品化", "网关端", "2026-06-18", "已完成原型", "P1", "Go 网关", "GW-003", "两个通道可任意启停，页面显示清晰"],
  ["GW-005", "MQTT 断线重连和错误原因展示", "M2 产品化", "网关端", "2026-06-20", "开发中", "P1", "Go 网关", "GW-003", "网络恢复后自动上线，页面显示错误原因"],
  ["GW-006", "本地缓存和断点续传", "M3 量产运维", "网关端", "2026-06-28", "待开发", "P2", "Go 网关", "采集模块", "断网时缓存，恢复后补发"],
  ["GW-007", "Linux ARM 部署包", "M2 产品化", "网关端/运维", "2026-06-22", "待开发", "P1", "部署", "构建流程", "目标 ARM 网关可启动并连接云平台"],
  ["GW-008", "工厂批量烧录工具", "M3 量产运维", "网关端/工厂", "2026-06-30", "待开发", "P2", "工厂工具", "GW-002", "可批量写入激活文件并验证上线"],
  ["FE-001", "出厂网关列表页面", "M1 原型闭环", "前端", "2026-06-14", "已完成原型", "P0", "前端", "BE-001", "可查看 HardwareId、SN、状态和绑定设备"],
  ["FE-002", "登记出厂网关表单", "M1 原型闭环", "前端", "2026-06-14", "已完成原型", "P0", "前端", "BE-002", "登记后展示本次生成的密钥和绑定码"],
  ["FE-003", "激活文件复制和下载", "M1 原型闭环", "前端", "2026-06-14", "已完成原型", "P0", "前端", "BE-002", "工厂能获取 activation 文件"],
  ["FE-004", "客户绑定网关表单", "M1 原型闭环", "前端", "2026-06-15", "已完成原型", "P0", "前端", "BE-003", "输入 SN + BindCode 后绑定成功"],
  ["FE-005", "绑定成功后跳转设备详情", "M2 产品化", "前端", "2026-06-18", "待开发", "P1", "前端", "FE-004", "用户能直接进入网关详情"],
  ["FE-006", "出厂网关状态筛选", "M2 产品化", "前端", "2026-06-18", "待开发", "P1", "前端", "FE-001", "可按未上线、在线未绑定、已绑定筛选"],
  ["QA-001", "登记、上线、绑定、遥测端到端测试", "M1 原型闭环", "测试/研发", "2026-06-21", "待开发", "P0", "测试", "M1 全部任务", "完整链路可演示"],
  ["QA-002", "错误绑定码、重复绑定、禁用网关测试", "M2 产品化", "测试/研发", "2026-06-23", "待开发", "P1", "测试", "BE-005", "异常场景提示明确且不产生脏数据"],
  ["QA-003", "Linux ARM 运行测试", "M2 产品化", "测试/网关端", "2026-06-24", "待开发", "P1", "测试", "GW-007", "目标硬件可采集并连接云平台"],
];

const workbook = Workbook.create();
const story = workbook.worksheets.add("Story_list");
const progress = workbook.worksheets.add("进度看板");
const summary = workbook.worksheets.add("汇总");

story.showGridLines = false;
progress.showGridLines = false;
summary.showGridLines = false;

story.getRange("A1:E1").values = [["ID", "标题", "迭代", "处理人", "预计结束"]];
story.getRangeByIndexes(1, 0, tasks.length, 5).values = tasks.map((task) => [
  task[0],
  `[${task[5]}][${task[7]}] ${task[1]}`,
  task[2],
  task[3],
  task[4],
]);

progress.getRange("A1:J1").values = [["ID", "任务", "迭代", "负责人", "预计结束", "状态", "优先级", "模块", "依赖", "验收标准"]];
progress.getRangeByIndexes(1, 0, tasks.length, 10).values = tasks;

summary.getRange("A1:D1").values = [["状态", "数量", "说明", "建议动作"]];
const statusRows = [
  ["已完成原型", tasks.filter((task) => task[5] === "已完成原型").length, "当前项目已有雏形，可演示但需要产品化确认", "代码审查、补测试、确认交互"],
  ["开发中", tasks.filter((task) => task[5] === "开发中").length, "已有部分实现，还需要继续补齐", "优先处理 M1/M2 阻塞项"],
  ["待开发", tasks.filter((task) => task[5] === "待开发").length, "还未开始实现", "纳入排期并明确负责人"],
  ["待确认", tasks.filter((task) => task[5] === "待确认").length, "需要产品、硬件或实施侧确认", "开评审会确认范围"],
];
summary.getRange("A2:D5").values = statusRows;

summary.getRange("F1:H1").values = [["优先级", "数量", "说明"]];
summary.getRange("F2:H4").values = [
  ["P0", tasks.filter((task) => task[6] === "P0").length, "必须优先完成，影响主流程闭环"],
  ["P1", tasks.filter((task) => task[6] === "P1").length, "产品化必需，影响可用性和稳定性"],
  ["P2", tasks.filter((task) => task[6] === "P2").length, "量产和运维增强项"],
];

summary.getRange("A8:E8").values = [["里程碑", "目标", "必须完成", "建议时间", "验收"]];
summary.getRange("A9:E11").values = [
  ["M1 原型闭环", "演示登记、烧录、上线、绑定、上报完整链路", "P0 任务", "2026-06-21", "完整流程可演示"],
  ["M2 产品化", "支持内部试点客户", "P0 + P1 任务", "2026-06-25", "权限、安全、状态、部署可用"],
  ["M3 量产运维", "支持工厂批量生产和售后排障", "P2 任务", "2026-06-30", "批量烧录、密钥轮换、断点续传"],
];

for (const sheet of [story, progress, summary]) {
  const used = sheet.getUsedRange();
  used.format = {
    font: { name: "Microsoft YaHei", size: 10, color: "#0F172A" },
    borders: { preset: "all", style: "thin", color: "#D9E2EC" },
    alignment: { vertical: "center" },
    wrapText: true,
  };
}

story.getRange("A1:E1").format = {
  fill: "#1F4E78",
  font: { bold: true, color: "#FFFFFF", name: "Microsoft YaHei", size: 10 },
  alignment: { horizontal: "center", vertical: "center" },
};
progress.getRange("A1:J1").format = {
  fill: "#1F4E78",
  font: { bold: true, color: "#FFFFFF", name: "Microsoft YaHei", size: 10 },
  alignment: { horizontal: "center", vertical: "center" },
};
summary.getRange("A1:D1").format = {
  fill: "#1F4E78",
  font: { bold: true, color: "#FFFFFF", name: "Microsoft YaHei", size: 10 },
  alignment: { horizontal: "center", vertical: "center" },
};
summary.getRange("F1:H1").format = {
  fill: "#1F4E78",
  font: { bold: true, color: "#FFFFFF", name: "Microsoft YaHei", size: 10 },
  alignment: { horizontal: "center", vertical: "center" },
};
summary.getRange("A8:E8").format = {
  fill: "#1F4E78",
  font: { bold: true, color: "#FFFFFF", name: "Microsoft YaHei", size: 10 },
  alignment: { horizontal: "center", vertical: "center" },
};

story.freezePanes.freezeRows(1);
progress.freezePanes.freezeRows(1);
summary.freezePanes.freezeRows(1);

story.getRange("A:A").format.columnWidthPx = 92;
story.getRange("B:B").format.columnWidthPx = 470;
story.getRange("C:C").format.columnWidthPx = 130;
story.getRange("D:D").format.columnWidthPx = 110;
story.getRange("E:E").format.columnWidthPx = 110;

progress.getRange("A:A").format.columnWidthPx = 92;
progress.getRange("B:B").format.columnWidthPx = 280;
progress.getRange("C:C").format.columnWidthPx = 120;
progress.getRange("D:D").format.columnWidthPx = 110;
progress.getRange("E:E").format.columnWidthPx = 110;
progress.getRange("F:F").format.columnWidthPx = 100;
progress.getRange("G:G").format.columnWidthPx = 70;
progress.getRange("H:H").format.columnWidthPx = 100;
progress.getRange("I:I").format.columnWidthPx = 130;
progress.getRange("J:J").format.columnWidthPx = 360;

summary.getRange("A:A").format.columnWidthPx = 120;
summary.getRange("B:B").format.columnWidthPx = 70;
summary.getRange("C:C").format.columnWidthPx = 300;
summary.getRange("D:D").format.columnWidthPx = 240;
summary.getRange("F:F").format.columnWidthPx = 90;
summary.getRange("G:G").format.columnWidthPx = 70;
summary.getRange("H:H").format.columnWidthPx = 320;

story.getRangeByIndexes(0, 0, tasks.length + 1, 5).format.rowHeightPx = 28;
progress.getRangeByIndexes(0, 0, tasks.length + 1, 10).format.rowHeightPx = 30;
summary.getRange("A1:H11").format.rowHeightPx = 30;

progress.getRange(`F2:F${tasks.length + 1}`).dataValidation = {
  rule: { type: "list", values: ["已完成原型", "开发中", "待开发", "待确认"] },
};
progress.getRange(`G2:G${tasks.length + 1}`).dataValidation = {
  rule: { type: "list", values: ["P0", "P1", "P2"] },
};

const statusRange = progress.getRange(`F2:F${tasks.length + 1}`);
statusRange.conditionalFormats.add("containsText", { text: "已完成原型", format: { fill: "#DCFCE7", font: { color: "#166534" } } });
statusRange.conditionalFormats.add("containsText", { text: "开发中", format: { fill: "#FEF9C3", font: { color: "#854D0E" } } });
statusRange.conditionalFormats.add("containsText", { text: "待开发", format: { fill: "#F1F5F9", font: { color: "#475569" } } });
statusRange.conditionalFormats.add("containsText", { text: "待确认", format: { fill: "#DBEAFE", font: { color: "#1D4ED8" } } });

const priorityRange = progress.getRange(`G2:G${tasks.length + 1}`);
priorityRange.conditionalFormats.add("containsText", { text: "P0", format: { fill: "#FEE2E2", font: { color: "#B91C1C", bold: true } } });
priorityRange.conditionalFormats.add("containsText", { text: "P1", format: { fill: "#FFEDD5", font: { color: "#C2410C", bold: true } } });
priorityRange.conditionalFormats.add("containsText", { text: "P2", format: { fill: "#E0F2FE", font: { color: "#0369A1", bold: true } } });

story.tables.add(`A1:E${tasks.length + 1}`, true, "StoryList");
progress.tables.add(`A1:J${tasks.length + 1}`, true, "ProgressBoard");
summary.tables.add("A1:D5", true, "StatusSummary");
summary.tables.add("F1:H4", true, "PrioritySummary");

const chart = summary.charts.add("bar", summary.getRange("A1:B5"));
chart.title = "任务状态分布";
chart.hasLegend = false;
chart.xAxis = { axisType: "textAxis" };
chart.setPosition("J1", "Q16");

await fs.mkdir(outputDir, { recursive: true });
const preview = await workbook.render({ sheetName: "进度看板", range: "A1:J20", scale: 1, format: "png" });
await fs.writeFile(path.join(outputDir, "物联网平台_网关绑定进度看板_预览.png"), new Uint8Array(await preview.arrayBuffer()));

const exported = await SpreadsheetFile.exportXlsx(workbook);
await exported.save(outputPath);

console.log(outputPath);

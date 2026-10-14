# 鼠标锚点缩放抖动修复

候选变更：fix: 修复编辑器鼠标锚点缩放抖动。工作树变更，未创建正式提交，未部署；保留既有修改。

原因：EditRange 的 go-transition 与外层 edit-content 的 go-transition 均使用 transition: all 0.4s。缩放锚点补偿按目标 scale 计算，而浏览器仍在插值 transform、宽度及高度，导致逐帧位置偏移。缩放过程中每次销毁重建标尺并间隔 10ms，也增加重复渲染与闪烁。

行为变化：画布仅保留背景颜色过渡，外层仅保留阴影过渡；滚轮事件按 requestAnimationFrame 合并，每帧最多提交一次比例；锚点补偿仍在 Vue nextTick 后、浏览器绘制前完成；标尺完整重绘延后到本轮缩放停止 120ms 后，保留坐标变化响应；禁用浏览器 scroll anchoring 和 smooth scrolling 干扰。组件缩放快捷键规则不变。

验证环境：Windows Chrome 独立无头浏览器，1600×1000 视口，空白模拟项目，/api 响应全由测试拦截。没有真实凭据或业务写入。

- `node tools/test-canvas-zoom.cjs`：通过，锚点数学、滚动边界补偿、缩放范围、方向及 Vue SFC 编译。
- `npm run build`：构建通过；该 Windows 脚本不能证明独立类型检查通过。
- `node tools/test-canvas-zoom-browser.cjs <bundled-playwright-path>`：临时本机 3029 预览通过，逐帧连续事件测试普通位置最大偏移约 0.782px、滚动边界约 0.792px。
- 同一浏览器测试对比当前 8080 部署版本：复现抖动，普通位置最大偏移约 585.955px，测试按预期失败。

临时预览服务已停止。未测：实际用户复杂项目、触控板手感、低性能设备帧率。模拟空画布结果不等于复杂项目性能验收。

回滚：仅回退 EditRange/index.vue、ContentEdit/index.vue 中的过渡设置与 EditRule/index.vue 的滚轮合帧、标尺重绘和滚动设置；保留此前会话恢复、数据绑定和组件编辑改动。

# 移动对齐隐藏编辑器选框

候选：当前 backup-20260624 工作树修改，未创建正式提交，未部署。

新增 isMoving 编辑器临时状态，仅组件位置拖动触发，鼠标松开时在合并最后移动后清除。EditShapeBox 依据该状态隐藏选中/悬停框、背景遮罩和缩放锚点，保留组件自身外观及上层红色辅助线。松手恢复选框；尺寸调整、线段端点编辑不触发此隐藏。分组与多选共享该状态。

验证（Windows，临时本地3029预览，独立Chrome，模拟项目/接口，不写真实业务数据）：

- npm run build-cli 通过；不包含类型检查，仍有既有打包警告。
- node tools/test-component-resize.cjs 与 node tools/test-figure-line.cjs 通过。
- node tools/test-component-resize-browser.cjs <bundled-playwright-path> 通过：实际位置拖动时选框与锚点隐藏、辅助线仍存在，松手后恢复；尺寸调整期间选框保留；缩放与撤销重做回归，无页面错误。

未测：多层组合、多选和触摸操作逐项浏览器验收；真实项目发布。未自动部署。

回滚：只撤销本批 isMoving 状态、移动处理器标记与 EditShapeBox 隐藏样式，保留此前辅助线层级、组件缩放与线段优化。风险：用户可能将组件自身边框与编辑器选框混淆；本改动不更改任何已配置的组件边框。

# 云组态独立图标

采用内置图像生成工具生成独立蓝底白色画布/连接节点图标，加入少量青色节点；不再复用平台云形/环绕轨道造型。透明版本边缘质量不理想，迭代为不透明高对比方形底。最终生成图已目视检查，源输出原样复制至 apps/goview-designer/public/scada-editor-icon-v1.png，平台素材不修改。入口 favicon 与 touch icon 指向新文件，编辑器/预览共用；原平台图标保留便于回退。

最终提示词（内置 image_gen，无CLI/API调用）：One finished app favicon logo for an industrial visual configuration / SCADA canvas editor. Opaque azure blue square, centered bold white rounded canvas outline and connected square nodes, cyan accent, clean geometric edges, readable at 16 pixels. No cloud/orbit/planet/text/grunge/glow/mockup.

工作树候选，未提交/未部署。源PNG及HTML检查通过；真实浏览器16px清晰度、favicon缓存和部署制品未验证，不宣称已部署。回退 index.html 到 tenant-platform-favicon.png 即恢复上一版，不改平台品牌或业务功能。

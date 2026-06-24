<template>
    <div style="background: #fff;height: 100%;padding: 20px;">
        <div class="top">
            <div>SQL编辑器</div>
            <div>
                <el-button type="primary" :icon="Fold" @click="format">格式化</el-button>
                <el-button type="primary" :icon="Delete" @click="empty">清空</el-button>
                <el-button type="primary" :icon="CaretRight">执行</el-button>
            </div>
        </div>
        <div class="cont">
            <MonacoEditorComponent @on-change="onCodeChange" ref="monacoEditorRef" />
        </div>
        <div>
            <el-tabs v-model="activeName" class="demo-tabs" @tab-click="handleClick">
                <el-tab-pane label="结果" name="first">
                    <el-table :data="tableData" style="width: 100%" border>
                        <el-table-column prop="id" label="ID" />
                        <el-table-column prop="name" label="USERNAME" />
                        <el-table-column prop="EMAIL" label="EMAIL" />
                        <el-table-column prop="CREATE_TIME" label="CREATE_TIME" />
                    </el-table>
                </el-tab-pane>
            </el-tabs>
        </div>
    </div>
</template>
<script lang="ts" setup>
import { Delete, CaretRight, Fold } from '@element-plus/icons-vue'
import type { TabsPaneContext } from 'element-plus'
import MonacoEditorComponent from './monacoEditor.vue'
let emit = defineEmits(["clear"]);
let activeName = ref('first') // tab默认选中
let tableData = ref([{ id: '1' }]) // 表格数据
let text = ref('') // 编辑器数据
let monacoEditorRef = ref();
/**
 * 编辑器的内容
 */
let onCodeChange = (e: string) => {
    text.value = e
}
let handleClick = (tab: TabsPaneContext, event: Event) => {
    console.log(tab, event)
}
/**
 * 清空
 */
let empty = () => {
    monacoEditorRef.value.clears(); // 调用子组件的 clears 方法
}
// 脚本中的方法
let format = () => {
    if (monacoEditorRef.value) {
        monacoEditorRef.value.formatDocument();
    }
};
</script>
<style lang="scss" scoped>
.top {
    display: flex;
    justify-content: space-between;
    padding: 0 0 10px 0;
}

.cont {
    width: 100%;
    height: 200px;
}

:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>
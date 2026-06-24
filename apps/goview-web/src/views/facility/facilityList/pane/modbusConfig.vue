<template>
    <div class="empty-container">
        <!-- <div style="display: flex;align-items: center;justify-content: space-between;">
            <el-button type="primary" icon="Edit">更新设备模型</el-button>
        </div>
        <el-table :data="tableData" style="padding: 1rem 0;">
            <el-table-column prop="" label="功能名称" align="center" />
            <el-table-column prop="" label="从机地址" align="center" />
            <el-table-column prop="" label="寄存器类型" align="center" />
            <el-table-column prop="" label="寄存器地址" align="center" />
            <el-table-column prop="" label="数据类型" align="center" />
            <el-table-column prop="" label="寄存器数量" align="center" />
            <el-table-column prop="" label="字节序" align="center" />
        </el-table> -->
        <img src="@/assets/蒙版组 15.png" class="dev-image" />
    </div>
</template>

<script lang="ts" setup>
let tableData = ref([]);//modbus配置表格
</script>
<style scoped lang="scss">
.empty-container {
    width: 100%;
    height: 500px;
    display: flex;
    align-items: center;
    justify-content: center;
    box-sizing: border-box;
}

.dev-image {
    max-width: 300px;
    width: 100%;
    height: auto;
}
</style>
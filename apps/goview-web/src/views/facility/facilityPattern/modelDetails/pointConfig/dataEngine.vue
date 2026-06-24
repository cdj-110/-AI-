<template>
    <el-table :data="tableData" style="width: 100%">
        <el-table-column prop="property" label="属性" align="center">
            <template #default="scope">
                <p v-if="scope.row.type === 0">{{ scope.row.property }}</p>
                <el-select v-if="scope.row.type === 1" v-model="scope.row.property" placeholder="请选择属性">
                    <el-option label="DI1" value="DI1" />
                    <el-option label="DI2" value="DI2" />
                </el-select>
            </template>
        </el-table-column>
        <el-table-column prop="slave" label="从机地址" align="center" />
        <el-table-column prop="register" align="center" width="200">
            <template #header>
                <div style="display: flex;align-items: center;">
                    <p>寄存器地址</p>
                    (<el-switch v-model="isHexTrue" style="--el-switch-on-color: #8FAFF4; --el-switch-off-color: #D4D4D4" active-text="" inactive-text="HEX显示" />)
                </div>
            </template>
            <template #default="scope">
                <p v-if="scope.row.type === 0">{{ scope.row.register }}</p>
                <el-input-number v-if="scope.row.type === 1" v-model="scope.row.register" controls-position="right" />
            </template>
        </el-table-column>
        <el-table-column prop="listType" label="数据类型" align="center">
            <template #default="scope">
                <p v-if="scope.row.type === 0">{{ scope.row.listType }}</p>
                <el-select v-if="scope.row.type === 1" v-model="scope.row.listType" placeholder="请选择数据类型">
                    <el-option label="16位整数" value="16位整数" />
                    <el-option label="16位无符号整数" value="16位无符号整数" />
                    <el-option label="32位无符号整数" value="32位无符号整数" />
                    <el-option label="32位浮点数" value="32位浮点数" />
                </el-select>
            </template>
        </el-table-column>
        <el-table-column prop="registerCount" label="寄存器数量" align="center">
            <template #default="scope">
                <p v-if="scope.row.type === 0">{{ scope.row.registerCount }}</p>
                <el-input-number v-if="scope.row.type === 1" v-model="scope.row.registerCount" controls-position="right" />
            </template>
        </el-table-column>
        <el-table-column prop="little" label="字节序" align="center">
            <template #default="scope">
                <p v-if="scope.row.type === 0">{{ scope.row.little }}</p>
                <el-select v-if="scope.row.type === 1" v-model="scope.row.little" placeholder="请选择字节序">
                    <el-option label="AB" value="AB" />
                </el-select>
            </template>
        </el-table-column>
        <el-table-column prop="readWriteType" label="读写类型" align="center">
            <template #default="scope">
                <el-radio-group v-model="scope.row.readWriteType">
                    <el-radio-button :value="scope.row.readWriteType" v-if="scope.row.type === 0">{{ scope.row.readWriteType }}</el-radio-button>
                    <el-radio-button value="读写" v-if="scope.row.type === 1">读写</el-radio-button>
                    <el-radio-button value="只读" v-if="scope.row.type === 1">只读</el-radio-button>
                </el-radio-group>
            </template>
        </el-table-column>
        <el-table-column fixed="right" label="操作" align="center">
            <template #default="scope">
                <el-button link type="primary" v-if="scope.row.type === 0" @click="scope.row.type = 1;">修改</el-button>
                <el-button link type="danger" v-if="scope.row.type === 0">删除</el-button>
                <el-button link type="primary" v-if="scope.row.type === 1" @click="scope.row.type = 0;saveChange(scope.row)">保存</el-button>
                <el-button link type="info" v-if="scope.row.type === 1" @click="scope.row.type = 0;">取消</el-button>
            </template>
        </el-table-column>
    </el-table>
</template>

<script lang="ts" setup>
let props = withDefaults(defineProps<{
    config: {
        type: string
    }
}>(),{
    config: () => ({
        type: ""
    })
})
/**
 * 数据寄存器表格内容
 * @inner(type) 表格状态(0:表格为不编辑状态，1:表格为编辑状态)
 */
let tableData = ref([
    { type: 0, property: "DI1", slave: "子网关地址", register: 12, listType: "16位整数", little: "AB", registerCount: 22, readWriteType: "只读" }
]);
let isHexTrue = ref(false);//是否以HEX显示寄存器地址

/**
 * IO寄存器、数据寄存器表格修改
 */
let saveChange = (e: any) => {
    console.log("单条修改1111",e)
}

/**
 * 由父组件触发，批量修改IO寄存器、数据寄存器表格
 */
let IOEngineRef = (e: any) => {
    tableData.value.forEach(item => {
        e === true ? item.type = 1 : item.type = 0;
    })
    if(e === false) {
        console.log("批量保存",tableData.value)
    }
}

defineExpose({ IOEngineRef });
</script>

<style lang="scss" scoped>
:deep(.el-switch__label *){
    font-size: 13px;
}
:deep(.el-switch__core){
    height: 13px;
    min-width: 25px;
}
:deep(.el-switch__core .el-switch__action){
    width: 7px;
    height: 7px;
}
:deep(.el-switch.is-checked .el-switch__core .el-switch__action){
    left: calc(100% - 7px);
}
:deep(.el-switch__label.is-active){
    color: #D9D9D9;
}
:deep(.el-switch__label){
    color: #4D639B; 
}
:deep(.el-select__placeholder.is-transparent),
:deep(.el-input__inner) {
    font-size: 0.8rem;
}
</style>
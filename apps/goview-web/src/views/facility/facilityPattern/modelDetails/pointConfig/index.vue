<template>
    <!-- <div class="pointConfig" style=" overflow: auto;height: 50vh;padding: 0 0.5rem 0 0;">
        <div style="padding: 0 1.2rem;">
            <h2>Modbus寄存器设置</h2>
            <h3 style="padding: 0.5rem 0 0 0;">IO寄存器</h3>
            <div style="display: flex;align-items: center;justify-content: space-between;">
                <p style="font-size: 0.9rem;">IO寄存器包括线圈、离散寄存器，可以和开关量(Boolean)属性绑定</p>
                <p>
                    <el-button type="primary" icon="Plus" v-if="EngineType1 === false"
                        @click="configList = { isTrue: true, title: '添加IO寄存器', type: 0, list: {} }">添加寄存器</el-button>
                    <el-button type="primary" icon="Edit" v-if="EngineType1 === false"
                        @click="quantity">批量修改</el-button>
                    <el-button type="info" icon="Plus" v-if="EngineType1 === true"
                        @click="EngineType1 = false;">取消</el-button>
                    <el-button type="primary" icon="Edit" v-if="EngineType1 === true" @click="quantity">保存</el-button>
                </p>
            </div>
            IO寄存器
            <IOEngine ref="IOEngineRef" :config="{ type: 'IO寄存器' }"></IOEngine>
            <h3 style="padding: 0.5rem 0 0 0;">数据寄存器</h3>
            <div style="display: flex;align-items: center;justify-content: space-between;">
                <p style="font-size: 0.9rem;">数据寄存器包括输入寄存器、保持寄存器，可以和数值型(Number)属性绑定</p>
                <p>
                    <el-button type="primary" icon="Plus" v-if="EngineType2 === false"
                        @click="configList = { isTrue: true, title: '添加数据寄存器', type: 1, list: {} }">添加寄存器</el-button>
                    <el-button type="primary" icon="Edit" v-if="EngineType2 === false"
                        @click="quantityList">批量修改</el-button>
                    <el-button type="info" icon="Plus" v-if="EngineType2 === true"
                        @click="EngineType2 = false;">取消</el-button>
                    <el-button type="primary" icon="Edit" v-if="EngineType2 === true"
                        @click="quantityList">保存</el-button>
                </p>
            </div>
            数据寄存器
            <dataEngine ref="IOEngineListRef" :config="{ type: '数据寄存器' }"></dataEngine>
        </div>
        <el-dialog v-model="configList.isTrue" :title="configList.title" width="30%" :close-on-click-modal="false"
            :close-on-press-escape="false" draggable>
            <el-form :inline="true" :model="formInline">
                <el-form-item label="选择属性" style="width: 100%;">
                    <el-select v-model="formInline.property" multiple placeholder="请选择">
                        <el-option v-for="item in propertyList" :key="item.value" :label="item.label"
                            :value="item.value" />
                    </el-select>
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button type="info" @click="configList.isTrue = false;">取消</el-button>
                <el-button type="primary" @click="saveTypeList">添加</el-button>
            </template>
        </el-dialog>
    </div> -->
     <div class="empty-container">
        <img src="@/assets/蒙版组 15.png" alt="功能暂未开发" class="dev-image" />
    </div>
</template>

<script lang="ts" setup>
import IOEngine from "./IOEngine.vue";
import dataEngine from './dataEngine.vue'
const props = defineProps({
    id: {
        type: String,
        required: true
    }
});
let IOEngineRef: any = ref("");//IO寄存器Ref
let EngineType1 = ref(false);//IO寄存器是否编辑
let IOEngineListRef: any = ref("");//数据寄存器Ref
let EngineType2 = ref(false);//数据寄存器是否编辑
let configList = ref({ isTrue: false, title: "", type: 0, list: {} });//添加寄存器的弹窗 type === 0:添加IO寄存器 type === 1:添加数据寄存器
let formInline = ref({ property: [] });//添加寄存器表单
let propertyList = ref([{ value: "属性1", label: "属性1" }, { value: "属性2", label: "属性2" }, { value: "属性3", label: "属性3" }]);//选择属性的列表


/**
 * IO寄存器批量修改
 */
let quantity = () => {
    EngineType1.value = EngineType1.value === true ? false : true;
    IOEngineRef.value.IOEngineRef(EngineType1.value);
}

/**
 * 数据寄存器批量修改
 */
let quantityList = () => {
    EngineType2.value = EngineType2.value === true ? false : true;
    IOEngineListRef.value.IOEngineRef(EngineType2.value);
}

/**
 * 添加寄存器
 */
let saveTypeList = () => {
    console.log(formInline.value)
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
:deep(.el-select__placeholder.is-transparent),
:deep(.el-input__inner) {
    font-size: 0.8rem;
}

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
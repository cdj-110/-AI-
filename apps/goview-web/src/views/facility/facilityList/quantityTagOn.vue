<template>
    <div class="quantityTagOn">
        <el-dialog v-model="props.quantityTagOnConfig.bool" title="批量添加设备" width="30%" draggable
            :close-on-press-escape="false" :close-on-click-modal="false">
            <div style="margin: 0 2rem;">
                <div style="display: flex;align-items: center;">
                    <p>通过设备模型可以一键上云</p>
                    <p style="color: #3C93E7;margin-left: 1rem;cursor: pointer;"
                        @click="createPatternConfig.bool = true;">⌜创建设备模型⌟</p>
                </div>
                <el-form :model="quantityiniter" label-width="auto" :rules="rules" ref="rulesRef"
                    style="max-height: 50vh;overflow-y: auto;margin-top: 3rem;">
                    <el-form-item label="设备模型" prop="pattern">
                        <el-select v-model="quantityiniter.pattern" placeholder="请选择设备模型">
                            <el-option label="默认分组" value="默认分组" />
                            <el-option label="分组1" value="分组1" />
                        </el-select>
                    </el-form-item>
                    <el-form-item label="设备分组" prop="grouping">
                        <el-select v-model="quantityiniter.grouping" placeholder="请选择设备分组">
                            <el-option label="默认分组" value="默认分组" />
                            <el-option label="分组1" value="分组1" />
                        </el-select>
                    </el-form-item>
                    <el-form-item label="SN导入" prop="SN">
                        <el-upload drag action="" @http-request="httpRequest" :auto-upload="false" multiple
                            style="width: 100%;">
                            <el-icon><upload-filled /></el-icon>
                            <div>
                                <p>使用文件导入</p>
                                <p>支持XLSX格式</p>
                            </div>
                            <template #tip>
                                <div style="text-align: right;color: #3C93E7;cursor: pointer;">⌜下载导入模板⌟</div>
                            </template>
                        </el-upload>
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <el-button type="primary" @click="quantityTagOnChange">新增设备</el-button>
            </template>
        </el-dialog>
        <createPattern :createPatternConfig="createPatternConfig"></createPattern>
    </div>
</template>

<script lang="ts" setup>
import createPattern from "./createPattern.vue";

let createPatternConfig = ref({ bool: false });//传入子组件中的值

//从父组件传过来的值
let props = withDefaults(defineProps<{
    quantityTagOnConfig: {
        bool: Boolean
    }
}>(), {
    quantityTagOnConfig: () => ({
        bool: false
    })
})

let quantityiniter = ref({ pattern: "", grouping: "" });//批量创建设备表单
let rules = ref({
    pattern: [{ required: true, message: "设备模型不能为空", trigger: "blur" }],
    grouping: [{ required: true, message: "设备分组不能为空", trigger: "blur" }],
    SN: [{ required: true, message: "请上传SN", trigger: "blur" }],
});
let rulesRef: any = ref("");//表单Ref

/**
 * 新增设备
 */
let quantityTagOnChange = () => {
    rulesRef.value.validate((vali: any) => {
        if (vali) {
            props.quantityTagOnConfig.bool = false;
        }
    })
}

/**
 * 自定义上传
 */
let httpRequest = (file: any) => {
    console.log(file)
}
</script>
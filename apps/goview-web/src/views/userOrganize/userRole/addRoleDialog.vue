<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.title" width="500" @close="emit('handleClose', false)">
            <div style="padding: 0.5rem 1rem;">
                <el-form ref="ruleFormRef" style="max-width: 600px" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="$t('common.role_name')" prop="a">
                        <el-input v-model="ruleForm.a" :placeholder="$t('common.pleaseEnterRoleName')" />
                    </el-form-item>
                    <el-form-item :label="$t('common.remark')">
                        <el-input v-model="ruleForm.b" type="textarea" :placeholder="$t('common.pleaseEnterRemark')" />
                    </el-form-item>
                    <el-form-item :label="$t('common.permission')">
                        <div
                            style="width: 100%;height: 25rem;border: 1px #F1F1F1 solid;border-radius: 0.5rem; padding: 0 0.5rem; display: flex;justify-content: space-between;">
                            <div>
                                <el-tree ref="treeRef" style="max-width: 600px" :data="data" show-checkbox node-key="id"
                                    highlight-current :props="defaultProps" :default-expanded-keys="expandedKeys" />
                            </div>
                            <div> 
                                <el-checkbox v-model="checked1" :label="checked1 ? $t('common.collapseAll') : $t('common.expandAll')" size="small" @change="changeTree" />
                            </div>
                        </div>
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div style="display: flex;justify-content: center;margin: 2rem 0;">
                    <el-button @click="emit('handleClose', false)">{{ $t('common.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">{{ $t('common.confirm') }}</el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import type { FormInstance, FormRules } from 'element-plus'

const { t } = useI18n();

const props = defineProps({
    title: {
        type: String,
        required: false
    }
});

let defaultProps = {
    children: 'children',
    label: 'label',
}
let emit = defineEmits(["handleClose"]);

let ruleFormRef = ref<FormInstance>()
let dialogVisible = ref(true)
let ruleForm = ref({ a: '', b: '', c: '', d: '', password: '' })

// 错误消息国际化
let rules = ref<FormRules>({
    a: [{ required: true, message: t('common.pleaseEnterRoleName'), trigger: 'change' }]
})

let checked1 = ref(false) // 展开或者折叠数据

let treeRef = ref(null);
let expandedKeys = ref<number[]>([])


let data = ref([
    {
        id: 1,
        label: 'Level one 1',
        children: [
            {
                id: 4,
                label: 'Level two 1-1',
                children: [
                    {
                        id: 9,
                        label: 'Level three 1-1-1',
                    },
                    {
                        id: 10,
                        label: 'Level three 1-1-2',
                    },
                ],
            },
        ],
    },
    {
        id: 2,
        label: 'Level one 2',
        children: [
            {
                id: 5,
                label: 'Level two 2-1',
            },
            {
                id: 6,
                label: 'Level two 2-2',
            },
        ],
    },
    {
        id: 3,
        label: 'Level one 3',
        children: [
            {
                id: 7,
                label: 'Level two 3-1',
            },
            {
                id: 8,
                label: 'Level two 3-2',
            },
        ],
    },
])

// 递归获取所有节点key的方法
const getAllKeys = (data: any[]): number[] => {
    let keys: number[] = []
    data.forEach(node => {
        keys.push(node.id)
        if (node.children && node.children.length > 0) {
            keys = keys.concat(getAllKeys(node.children))
        }
    })
    return keys
}

// 处理复选框变化的方法
const changeTree = (isChecked: boolean) => {
    if (isChecked) {
        // 展开全部 - 获取所有节点的key
        expandedKeys.value = getAllKeys(data.value)
    } else {
        // 折叠全部 - 清空展开的keys
        expandedKeys.value = []
    }
}
let submitForm = async (formEl: FormInstance | undefined) => {
    if (!formEl) return
    await formEl.validate((valid, fields) => {
        if (valid) {
            console.log('submit!')
            // 这里可以添加提交逻辑
            emit('handleClose', false)
        }
    })
}
</script>
<style lang="scss" scoped>
:deep(.el-input__inner),
:deep(.el-textarea__inner ) {
    font-size: 12px;
}

:deep(.el-button>span) {
    font-size: 13px;
}
</style>
<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.title" width="500" @close="emit('handleUserClose', false)"
            @selection-change="handleSelectionChange">
            <div style="display: flex;justify-content: flex-end; padding: 0.5rem;">
                <el-button type="primary" :icon="Refresh">{{ t('warning.warningGroup.refresh') }}</el-button>
            </div>

            <el-table :data="tableData" style="width: 100%">
                <el-table-column prop="date" :label="t('warning.warningGroup.username')" />
                <el-table-column prop="name" :label="t('warning.warningGroup.account')" />
                <el-table-column :label="t('warning.warningGroup.notificationMethod')" width="170">
                    <template #default="scope">
                        <div style="display: flex;justify-content: space-between;align-items: center;">
                            <span> <el-icon>
                                    <Iphone />
                                </el-icon></span>
                            <span><el-icon>
                                    <Message />
                                </el-icon></span>
                            <span></span>
                        </div>
                    </template>
                </el-table-column>
                <el-table-column type="selection" width="55" />
            </el-table>
            <template #footer>
                <div class="dialog-footer">
                    <el-button @click="emit('handleUserClose', false)">{{ t('warning.warningGroup.cancel') }}</el-button>
                    <el-button type="primary" @click="dialogVisible = false"> {{ t('warning.warningGroup.confirm') }} </el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { Refresh, Iphone, Message } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
    title: {
        type: String,
        required: false
    }
});
let emit = defineEmits(["handleUserClose"]);
let dialogVisible = ref(true)
let tableData = ref([{}])

let handleSelectionChange = (e: any) => { }
</script>

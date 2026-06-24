<template>
    <div class="userLog">
        <div class="userLogTitle">
            {{ t('userLog.pageTitle') }}
        </div>
        <div class="userLog_left">
            <el-form :inline="true" :model="formInline" class="demo-form-inline" ref="ruleFormRef">
                <el-form-item prop="time">
                    <el-date-picker style="width: 17rem;" v-model="formInline.time" type="datetimerange"
                        :start-placeholder="t('userLog.startTime')" :end-placeholder="t('userLog.endTime')"
                        :show-second="true" value-format="YYYY-MM-DD HH:mm:ss" @change="timeChange"
                        :default-time="defaultTime1" />
                </el-form-item>
                <el-form-item prop="action">
                    <el-input v-model="formInline.action" :placeholder="t('userLog.action')" clearable
                        style="width: 13rem;" />
                </el-form-item>
                <el-form-item>
                    <el-button type="primary" :icon="Search" @click="onSubmit">
                        {{ t('userLog.search') }}
                    </el-button>
                    <el-button type="primary" :icon="Refresh" @click="resetForm(ruleFormRef)">
                        {{ t('userLog.reset') }}
                    </el-button>
                </el-form-item>
            </el-form>
        </div>
        <div>
            <el-table :data="tableData" style="width: 100%">
                <el-table-column prop="action" :label="t('userLog.action')" align="center" />
                <el-table-column :label="t('userLog.logLevel')" align="center">
                    <template #default="scope">
                        <el-tag
                            :type="scope.row.level == '1' ? 'success' : scope.row.level == '2' ? 'warning' : 'danger'">
                            {{ scope.row.level == '1' ? t('userLog.low') : scope.row.level == '2' ? t('userLog.medium') :
                            t('userLog.high') }}
                        </el-tag>
                    </template>
                </el-table-column>
                <el-table-column prop="operator" :label="t('userLog.operator')" align="center" />
                <el-table-column :label="t('userLog.status')" align="center">
                    <template #default="scope">
                        <el-tag :type="scope.row.results == 'success' ? 'success' : 'danger'">
                            {{ scope.row.results == 'success' ? t('userLog.success') : t('userLog.failed') }}
                        </el-tag>
                    </template>
                </el-table-column>
                <el-table-column prop="creation_time" :label="t('userLog.time')" align="center"
                    :formatter="((row: any) => itializeUtc(row.creation_time))" />
                <el-table-column prop="message" :label="t('userLog.details')" show-overflow-tooltip width="400"
                    align="center" />
            </el-table>
            <div style="display: flex;justify-content: flex-end;margin: 0.7rem 0 3rem 0;">
                <el-pagination v-model:current-page="pagenumber" v-model:page-size="pagesize"
                    :page-sizes="[10, 50, 100, 300, 500, 1000]" layout="total, sizes, prev, pager,next" :total="total"
                    @size-change="handleSizeChange" @current-change="handleCurrentChange" />
            </div>
        </div>
    </div>
</template>

<script lang="ts" setup>
import { ref, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { operation_log_list } from '@/api/userLog/index'
import { Search, Refresh } from '@element-plus/icons-vue'
import type { FormInstance } from 'element-plus'
import { itializeUtc } from "@/utils/publicFun";

const { t } = useI18n();

const ruleFormRef = ref<FormInstance>()
const formInline = ref({ time: '', action: '' })
const defaultTime1 = [
    new Date(new Date().setHours(new Date().getHours(), new Date().getMinutes(), new Date().getSeconds())),
    new Date(2000, 1, 1, 0, 0, 0)
]
const tableData = ref([])
const pagenumber = ref(1)
const pagesize = ref(10)
const total = ref(0)
const start_time = ref('')
const end_time = ref('')

onMounted(() => {
    onSubmit()
})

const timeChange = (e: any) => {
    if (!e) return;
    const startDate = new Date(e[0]);
    const endDate = new Date(e[1]);

    const formatDateToString = (date: Date): string => {
        if (!(date instanceof Date)) {
            console.error('参数不是 Date 对象:', date)
            date = new Date(date)
        }

        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        const hours = String(date.getHours()).padStart(2, '0');
        const minutes = String(date.getMinutes()).padStart(2, '0');
        const seconds = String(date.getSeconds()).padStart(2, '0');

        return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    };

    start_time.value = formatDateToString(startDate);
    end_time.value = formatDateToString(endDate);
}

const onSubmit = () => {
    const params = {
        pagenumber: pagenumber.value,
        pagesize: pagesize.value,
        start_time: start_time.value,
        end_time: end_time.value,
        action: formInline.value.action
    }
    operation_log_list(params).then((res: any) => {
        tableData.value = res.data
        total.value = res.total
    })
}

const resetForm = (formEl: FormInstance | undefined) => {
    if (!formEl) return
    end_time.value = ''
    start_time.value = ''
    formEl.resetFields()
    onSubmit()
}

const handleSizeChange = (val: number) => {
    onSubmit()
}

const handleCurrentChange = (val: number) => {
    pagenumber.value = val
    onSubmit()
}
</script>

<style lang="scss" scoped>
.userLog {
    background: #fff;
    height: 100%;
    border-radius: 0.7rem;
    padding: 1rem 2rem;
    overflow-y: auto;

    .userLogTitle {
        font-size: 1.3rem;
        font-weight: 700;
    }

    .userLog_left {
        display: flex;
        justify-content: flex-end;
        padding: 0.5rem 0;
    }

    :deep(.el-table th.el-table__cell) {
        background-color: #f5f7fa;
    }

    :deep(.el-table .el-table__cell) {
        padding: 15px 0;
    }
}
</style>

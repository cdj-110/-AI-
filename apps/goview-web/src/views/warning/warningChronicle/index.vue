<template>
    <div class="Alarm">
        <div class="Alarm_title">
            {{ t('warning.warningChronicle.pageTitle') }}
        </div>
        <div class="Alarm_form">
            <el-form :inline="true" :model="formInline" class="demo-form-inline Alarm_splice">
                <div>
                    <el-form-item>
                        <el-select v-model="formInline.region" :placeholder="t('warning.warningChronicle.allStatus')" style="width: 10rem;">
                            <el-option label="Zone one" value="shanghai" />
                            <el-option label="Zone two" value="beijing" />
                        </el-select>
                    </el-form-item>
                    <el-form-item>
                        <el-date-picker style="width: 17rem;" v-model="formInline.user" type="datetimerange"
                            :start-placeholder="t('warning.warningChronicle.startTime')" :end-placeholder="t('warning.warningChronicle.endTime')" />
                    </el-form-item>
                </div>
                <div>
                    <el-form-item>
                        <el-input v-model="formInline.title" :placeholder="t('warning.warningChronicle.searchPlaceholder')" clearable />
                    </el-form-item>
                    <el-form-item>
                        <el-button type="primary" :icon="Search" @click="onSubmit">{{ t('warning.warningChronicle.search') }}</el-button>
                        <el-button type="primary" :icon="Refresh">{{ t('warning.warningChronicle.refresh') }}</el-button>
                    </el-form-item>
                </div>
            </el-form>
        </div>
        <div class="Alarm_cont">
            <el-timeline>
                <el-timeline-item v-for="alert in mockAlerts" :key="alert.id" :timestamp="alert.timestamp"
                    :placement="'top'" :type="getAlertType(alert.level)">
                    <el-card :class="['alert-card', `alert-level-${alert.level}`]"
                        :shadow="alert.level === '1' ? 'always' : 'hover'" style="margin-top: 1rem;">
                        <div class="alert-content">
                            <div class="alert-icon">
                                <el-icon v-if="alert.level === '1'">
                                    <Warning />
                                </el-icon>
                                <el-icon v-else-if="alert.level === '2'">
                                    <InfoFilled />
                                </el-icon>
                                <el-icon v-else-if="alert.level === '3'">
                                    <Bell />
                                </el-icon>
                                <el-icon v-else-if="alert.level === '4'">
                                    <CircleCheck />
                                </el-icon>
                            </div>
                            <div class="alert-details">
                                <div style="display: flex;justify-content: space-between;">
                                    <h4 :class="['alert-title', `alert-title-${alert.level}`]">
                                        {{ getAlertLevelText(alert.level) }}{{ t('warning.warningChronicle.alert.critical') }}
                                    </h4>
                                    <div style="font-size: 0.75rem;padding: 0 2rem 0 0;">
                                        {{ alert.description }}
                                    </div>
                                </div>
                                <p class="alert-metric">{{ t('warning.warningChronicle.alert.co2') }}{{ alert.co2 }}ppm</p>
                                <p class="alert-metric">
                                    <span>
                                        {{ t('warning.warningChronicle.alert.device') }}<span style="color: #79BBFF;">{{ alert.device }}</span>
                                    </span>
                                    <span style="margin-left: 10%;">
                                        <el-button type="primary" link @click="handleViewDetail(alert)"
                                            class="detail-btn"> {{ t('warning.warningChronicle.detail') }} </el-button>
                                    </span>
                                </p>
                            </div>
                        </div>
                    </el-card>
                </el-timeline-item>
            </el-timeline>
        </div>
        <div style="display: flex;justify-content: flex-end;margin: 0.7rem 0 3rem 0;">
            <el-pagination v-model:current-page="page" v-model:page-size="size"
                :page-sizes="[10, 50, 100, 200, 300, 400]" layout="total, sizes, prev, pager,next" :total="total"
                @size-change="handleSizeChange" @current-change="handleCurrentChange" />
        </div>
        <!-- handleClose -->
         <alarmRule v-if="isAlarmRule" @handleClose="handleClose" :objValue="objValue"></alarmRule>
    </div>
</template>

<script setup lang="ts">
import alarmRule from './alarmRule.vue'
import { ref, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Warning, InfoFilled, Bell, CircleCheck, Search, Refresh } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

// 定义告警级别类型
type AlertLevel = '1' | '2' | '3' | '4'
// 表单数据查询
let formInline = ref({ region:'', user:'', title:'' })
let page = ref(1)// 分页 页
let size = ref(10)// 分页 条
let total = ref(20)// 总条数
let isAlarmRule = ref(false) //告警详情弹窗
let objValue = ref({}) // 详情数据

// 模拟告警数据
let mockAlerts = computed(() => [
    {
        id: '1',
        timestamp: '2025-11-19 08:47:51',
        level: '1',
        co2: '16.5',
        device: t('warning.warningChronicle.alert.deviceName'),
        description: t('warning.warningChronicle.alert.desc1')
    },
    {
        id: '2',
        timestamp: '2025-11-19 08:47:51',
        level: '2',
        co2: '16.5',
        device: t('warning.warningChronicle.alert.deviceName'),
        description: t('warning.warningChronicle.alert.desc2')
    },
    {
        id: '3',
        timestamp: '2025-11-19 08:47:51',
        level: '3',
        co2: '16.5',
        device: t('warning.warningChronicle.alert.deviceName'),
        description: t('warning.warningChronicle.alert.desc3')
    },
    {
        id: '4',
        timestamp: '2025-11-19 08:47:51',
        level: '4',
        co2: '16.5',
        device: t('warning.warningChronicle.alert.deviceName'),
        description: t('warning.warningChronicle.alert.desc4')
    },
    {
        id: '1',
        timestamp: '2025-11-19 08:47:51',
        level: '1',
        co2: '16.5',
        device: t('warning.warningChronicle.alert.deviceName'),
        description: t('warning.warningChronicle.alert.desc1')
    },
    {
        id: '2',
        timestamp: '2025-11-19 08:47:51',
        level: '2',
        co2: '16.5',
        device: t('warning.warningChronicle.alert.deviceName'),
        description: t('warning.warningChronicle.alert.desc2')
    },
    {
        id: '3',
        timestamp: '2025-11-19 08:47:51',
        level: '3',
        co2: '16.5',
        device: t('warning.warningChronicle.alert.deviceName'),
        description: t('warning.warningChronicle.alert.desc3')
    },
    {
        id: '4',
        timestamp: '2025-11-19 08:47:51',
        level: '4',
        co2: '16.5',
        device: t('warning.warningChronicle.alert.deviceName'),
        description: t('warning.warningChronicle.alert.desc4')
    }
])

// 获取告警级别对应的颜色类型
let getAlertType = (level: AlertLevel): string => {
    let typeMap: Record<AlertLevel, string> = {
        1: 'danger',
        2: 'warning',
        3: 'info',
        4: 'success'
    }
    return typeMap[level]
}

// 获取告警级别文本
let getAlertLevelText = (level: AlertLevel): string => {
    let textMap: Record<AlertLevel, string> = {
        1: t('warning.warningChronicle.alert.critical'),
        2: t('warning.warningChronicle.alert.important'),
        3: t('warning.warningChronicle.alert.normal'),
        4: t('warning.warningChronicle.alert.resolved')
    }
    return textMap[level]
}

// 查看详情处理
let handleViewDetail = (alert: any) => {
    objValue.value = alert
   isAlarmRule.value = true
}

// 模拟从API获取数据
let fetchAlertData = async (): Promise<void> => {
}

// 分页 条
let handleSizeChange = (val: number) => {
    console.log(`${val} items per page`)
}

// 分页 页
let handleCurrentChange = (val: number) => {
    console.log(`current page: ${val}`)
}

// 详情取消弹窗
let handleClose = (e:any) => { isAlarmRule.value = false }

// 组件挂载时获取数据
onMounted(() => {
    fetchAlertData()
})
</script>

<style lang="scss" scoped>
.Alarm {
    background: #fff;
    height: 100%;
    border-radius: 0.7rem;
    padding: 1rem 2rem;
    overflow-y: auto;

    .Alarm_title {
        font-size: 1.3rem;
        font-weight: 700;
    }

    .Alarm_form {
        padding: 1.5rem 0;

        .Alarm_splice {
            display: flex;
            justify-content: space-between;
        }
    }

    :deep(.el-select__placeholder.is-transparent),
    :deep(.el-date-editor .el-range-input),
    :deep(.el-input__inner) {
        font-size: 0.75rem;
    }

    .Alarm_cont {

        :deep(.el-timeline-item__timestamp.is-top) {
            color: #3F3B37;
        }

        :deep(.alert-card) {
            border: none !important;
            border-radius: 0.3rem;
        }

        :deep(.el-card__body) {
            padding: 0.6rem;
        }

        .alert-content {
            display: flex;
            align-items: flex-start;
            gap: 0.6rem;
        }

        .alert-icon {
            font-size: 1.3rem;
        }

        .alert-level-1 .alert-icon,
        .alert-level-2 .alert-icon,
        .alert-level-3 .alert-icon,
        .alert-level-4 .alert-icon {
            justify-content: center;
            display: flex;
        }

        .alert-level-1 .alert-icon {
            color: #f56c6c;
        }

        .alert-level-2 .alert-icon {
            color: #e6a23c;
        }

        .alert-level-3 .alert-icon {
            color: #909399;
        }

        .alert-level-4 .alert-icon {
            color: #67c23a;
        }

        .alert-details {
            flex: 1;
        }

        .alert-title {
            margin: 0 0 8px 0;
            font-weight: 600;
        }

        .alert-title-1 {
            color: #f56c6c;
        }

        .alert-title-2 {
            color: #e6a23c;
        }

        .alert-title-3 {
            color: #909399;
        }

        .alert-title-4 {
            color: #67c23a;
        }

        .alert-metric {
            margin: 0.7rem 0;
            color: #676362;
            font-size: 12px;
        }

        :deep(.el-button>span) {
            font-size: 12px;
        }

        .alert-level-1 {
            background-color: #FFF8F8;
        }

        .alert-level-2 {
            background-color: #FEFAF4;
        }

        .alert-level-3 {
            background-color: #F6F6F6;
        }

        .alert-level-4 {
            background-color: #F1FEF3;
        }
    }
}
</style>

<template>
    <div style="background: #fff;height: 100%;">
        <facilityHeader :facilityConfig="facilityConfig">
            <template #left>
                <el-button type="primary" icon="Plus" @click="openAddDialog">{{ t('facilityLinkage.addLinkage') }}</el-button>
            </template>
            <template #content>
                <el-table :data="rulesList">
                    <el-table-column prop="a" :label="t('facilityLinkage.ruleName')" align="center" />
                    <el-table-column prop="a" :label="t('facilityLinkage.status')" align="center">
                        <template #default="scope">
                            <el-switch v-model="scope.row.a" />
                        </template>
                    </el-table-column>
                    <el-table-column prop="a" :label="t('facilityLinkage.createTime')" align="center" />
                    <el-table-column fixed="right" :label="t('facilityLinkage.operation')" align="center">
                        <template #default="scope">
                            <el-button link type="primary" @click="historyIsTrue = true;">{{ t('facilityLinkage.executionHistory') }}</el-button>
                            <el-button link type="primary" @click="openEditDialog(scope.row)">{{ t('common.edit') }}</el-button>
                            <el-button link type="danger">{{ t('common.delete') }}</el-button>
                        </template>
                    </el-table-column>
                </el-table>
            </template>
        </facilityHeader>
        <!-- 新增设备联动 -->
        <el-dialog v-model="dialogFormVisible.isTrue" :title="dialogTitle" width="65%">
            <el-form :model="form" label-width="auto">
                <el-row :gutter="20">
                    <el-col :span="12">
                        <el-form-item :label="t('facilityLinkage.ruleName')">
                            <el-input v-model="form.ruleTitle" :placeholder="t('facilityLinkage.pleaseEnterRuleName')" />
                        </el-form-item>
                    </el-col>
                    <el-col :span="12">
                        <el-form-item :label="t('facilityLinkage.ruleNote')">
                            <el-input type="textarea" :rows="3" v-model="form.ruleNote" :placeholder="t('facilityLinkage.pleaseEnterRuleNote')" />
                        </el-form-item>
                    </el-col>
                </el-row>
                <el-row :gutter="20">
                    <el-col :span="12">
                        <div style="font-size: 1.1rem;font-weight: 700;">{{ t('facilityLinkage.triggerCondition') }}</div>
                        <div style="margin-top: 1rem;">
                            <el-form-item :label="t('facilityLinkage.deviceSourceType')">
                                <el-radio-group v-model="form.facilityType">
                                    <el-radio :label="t('facilityLinkage.device')" value="1" />
                                    <el-radio :label="t('facilityLinkage.deviceModel')" value="2" />
                                </el-radio-group>
                            </el-form-item>
                            <el-form-item :label="t('facilityLinkage.deviceSource')">
                                <el-select v-model="form.facilityFountain" multiple :placeholder="t('facilityLinkage.pleaseSelectDeviceSource')">
                                    <el-option v-for="item in facilityList" :key="item.value" :label="item.label" :value="item.value" />
                                </el-select>
                            </el-form-item>
                            <el-form-item :label="t('facilityLinkage.triggerCondition')">
                                <el-row :gutter="5" v-for="(item, index) in triggerList" :key="index" style="width: 100%;">
                                    <el-col :span="5" style="width: 100%;">
                                        <el-select v-model="item.trigger1" :placeholder="t('facilityLinkage.logic')" style="width: 100%;">
                                            <el-option label="AND" value="AND" />
                                            <el-option label="OR" value="OR" />
                                        </el-select>
                                    </el-col>
                                    <el-col :span="7" style="width: 100%;">
                                        <el-select v-model="item.trigger2" :placeholder="t('facilityLinkage.property')" style="width: 100%;">
                                            <el-option :label="t('facilityLinkage.temperature')" value="temperature" />
                                        </el-select>
                                    </el-col>
                                    <el-col :span="5" style="width: 100%;">
                                        <el-select v-model="item.trigger3" :placeholder="t('facilityLinkage.operator')" style="width: 100%;">
                                            <el-option :label="t('facilityLinkage.greaterThan')" value="greaterThan" />
                                            <el-option :label="t('facilityLinkage.lessThan')" value="lessThan" />
                                            <el-option :label="t('facilityLinkage.equalTo')" value="equalTo" />
                                        </el-select>
                                    </el-col>
                                    <el-col :span="6" style="width: 100%;">
                                        <el-select v-model="item.trigger4" :placeholder="t('facilityLinkage.value')" style="width: 100%;">
                                            <el-option label="20" value="20" />
                                        </el-select>
                                    </el-col>
                                    <el-col :span="1" style="width: 100%;">
                                        <el-icon color="#f57c7c" size="20" @click="triggerClick(0,index)" style="margin-top: 0.5rem;cursor: pointer;"><DeleteFilled /></el-icon>
                                    </el-col>
                                </el-row>
                                <el-button type="primary" plain @click="triggerClick(1,0)" style="margin-top: 1rem;">{{ t('facilityLinkage.addCondition') }}</el-button>
                            </el-form-item>
                        </div>
                    </el-col>
                    <el-col :span="12" style="border-left: 1px solid #cecece;">
                        <div style="font-size: 1.1rem;font-weight: 700;">{{ t('facilityLinkage.executionAction') }}</div>
                        <div style="margin-top: 1rem;">
                            <el-form-item :label="t('facilityLinkage.deviceSourceType')">
                                <el-radio-group v-model="form.actionType">
                                    <el-radio :label="t('facilityLinkage.device')" value="1" />
                                    <el-radio :label="t('facilityLinkage.deviceModel')" value="2" />
                                </el-radio-group>
                            </el-form-item>
                            <el-form-item :label="t('facilityLinkage.deviceSource')">
                                <el-select v-model="form.actionFountain" multiple :placeholder="t('facilityLinkage.pleaseSelectDeviceSource')">
                                    <el-option v-for="item in facilityList" :key="item.value" :label="item.label" :value="item.value" />
                                </el-select>
                            </el-form-item>
                            <el-form-item :label="t('facilityLinkage.actionCondition')">
                                <el-row :gutter="5" v-for="(item, index) in actionList" :key="index" style="width: 100%;">
                                    <el-col :span="5" style="width: 100%;">
                                        <el-select v-model="item.action1" :placeholder="t('facilityLinkage.logic')" style="width: 100%;">
                                            <el-option label="AND" value="AND" />
                                            <el-option label="OR" value="OR" />
                                        </el-select>
                                    </el-col>
                                    <el-col :span="7" style="width: 100%;">
                                        <el-select v-model="item.action2" :placeholder="t('facilityLinkage.property')" style="width: 100%;">
                                            <el-option :label="t('facilityLinkage.temperature')" value="temperature" />
                                        </el-select>
                                    </el-col>
                                    <el-col :span="5" style="width: 100%;">
                                        <el-select v-model="item.action3" :placeholder="t('facilityLinkage.operator')" style="width: 100%;">
                                            <el-option :label="t('facilityLinkage.greaterThan')" value="greaterThan" />
                                            <el-option :label="t('facilityLinkage.lessThan')" value="lessThan" />
                                            <el-option :label="t('facilityLinkage.equalTo')" value="equalTo" />
                                        </el-select>
                                    </el-col>
                                    <el-col :span="6" style="width: 100%;">
                                        <el-select v-model="item.action4" :placeholder="t('facilityLinkage.value')" style="width: 100%;">
                                            <el-option label="20" value="20" />
                                        </el-select>
                                    </el-col>
                                    <el-col :span="1" style="width: 100%;">
                                        <el-icon color="#f57c7c" size="20" @click="actionClick(0,index)" style="margin-top: 0.5rem;cursor: pointer;"><DeleteFilled /></el-icon>
                                    </el-col>
                                </el-row>
                                <el-button type="primary" plain @click="actionClick(1,0)" style="margin-top: 1rem;">{{ t('facilityLinkage.addAction') }}</el-button>
                            </el-form-item>
                        </div>
                    </el-col>
                </el-row>
            </el-form>
            <template #footer>
                <el-button type="info" @click="dialogFormVisible.isTrue = false">{{ t('common.cancel') }}</el-button>
                <el-button type="primary" @click="dialogFormVisible.isTrue = false">{{ t('common.submit') }}</el-button>
            </template>
        </el-dialog>
        <el-dialog v-model="historyIsTrue" :title="t('facilityLinkage.executionHistoryData')" width="50%">
            <div style="display: flex;align-items: center;justify-content: space-between;margin-bottom: 1.3rem;">
                <div>{{ t('facilityLinkage.ruleName') }}：qweqwweqwwe</div>
                <div style="text-align: right;">
                    <el-date-picker v-model="ruleTitleSearch" type="datetimerange" :start-placeholder="t('facilityLinkage.startTime')" :end-placeholder="t('facilityLinkage.endTime')" format="YYYY-MM-DD HH:mm:ss" style="width: 50%;margin-right: 1rem;" />
                    <el-button type="primary" icon="RefreshRight" @click="">{{ t('facilityLinkage.refresh') }}</el-button>
                </div>
            </div>
            <el-table :data="historyList">
                <el-table-column prop="a" :label="t('facilityLinkage.time')" align="center" />
                <el-table-column prop="a" :label="t('facilityLinkage.result')" align="center" />
                <el-table-column prop="a" :label="t('facilityLinkage.details')" align="center" />
            </el-table>
            <template #footer>
                <el-button type="primary" @click="historyIsTrue = false">{{ t('common.confirm') }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const facilityConfig = computed(() => ({
    title: t('facilityLinkage.facilityLinkage'),
    search: [
        {
            fields: "c",
            type: "input",
            placeholder: t('facilityLinkage.ruleName')
        },
        {
            fields: "",
            type: "button",
            label: t('common.search'),
            status: "primary",
            icon: "Search",
            onClick: (e: any) => {
                console.log(e)
            }
        }
    ]
}))

let rulesList = ref([ {a:111} ])
let historyList = ref([ {a:111} ])
let dialogFormVisible = ref({ isTrue: false, type: 0, list: {} })
let form = ref({ ruleTitle: "", ruleNote: "", facilityType: "", facilityFountain: [], actionType: "", actionFountain: "", })
let facilityList = ref([ { value: "设备源1", label: "设备源1" }, { value: "设备源2", label: "设备源2" } ])
let triggerList = ref([ { trigger1: "", trigger2: "", trigger3: "", trigger4: "" } ])
let actionList = ref([ { action1: "", action2: "", action3: "", action4: "" } ])
let historyIsTrue = ref(false)
let ruleTitleSearch = ref([])

const dialogTitle = computed(() => {
    return dialogFormVisible.value.type === 0 ? t('facilityLinkage.addLinkage') : t('facilityLinkage.editLinkage')
})

const openAddDialog = () => {
    dialogFormVisible.value = { isTrue: true, type: 0, list: {} }
    form.value = { ruleTitle: "", ruleNote: "", facilityType: "", facilityFountain: [], actionType: "", actionFountain: "" }
    triggerList.value = [ { trigger1: "", trigger2: "", trigger3: "", trigger4: "" } ]
    actionList.value = [ { action1: "", action2: "", action3: "", action4: "" } ]
}

const openEditDialog = (row: any) => {
    dialogFormVisible.value = { isTrue: true, type: 1, list: row }
    form.value = { 
        ruleTitle: row.ruleTitle || "", 
        ruleNote: row.ruleNote || "", 
        facilityType: row.facilityType || "", 
        facilityFountain: row.facilityFountain || [], 
        actionType: row.actionType || "", 
        actionFountain: row.actionFountain || "" 
    }
}

/**
 * 触发条件列表1新增/0删除
 */
let triggerClick = (i: number,index: number) => {
    if(i === 0) {
        triggerList.value.splice(index,1);
    }else{
        triggerList.value.push({ trigger1: "", trigger2: "", trigger3: "", trigger4: "" });
    }
}

/**
 * 触发动作列表1新增/0删除
 */
 let actionClick = (i: number,index: number) => {
    if(i === 0) {
        actionList.value.splice(index,1);
    }else{
        actionList.value.push({ action1: "", action2: "", action3: "", action4: "" });
    }
}
</script>
<style lang="scss" scoped>
:deep(.el-table th.el-table__cell) {
    background-color: #f5f7fa;
}
</style>
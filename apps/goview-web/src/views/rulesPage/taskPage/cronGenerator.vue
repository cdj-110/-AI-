<template>
    <el-dialog v-model="visible" width="40%" align-center @close="emit('updateVisible', false)">
        <el-tabs type="border-card" :query="cronInputValue">
            <el-tab-pane :label="t('taskPage.seconds')">
                <template #label><el-icon style="margin-right: 0.3rem;"><calendar /></el-icon>{{ t('taskPage.seconds') }}</template>
                <el-radio-group v-model="cronInputValue.second.value">
                    <el-radio :value="1" style="width: 100%;">{{ t('taskPage.everySecond') }}</el-radio>
                    <el-radio :value="2" style="width: 100%;margin-top: 1rem;">
                        {{ t('taskPage.every') }} <el-input-number v-model="cronInputValue.second.select2.a" /> {{ t('taskPage.seconds') }} {{ t('taskPage.executeFrom') }} <el-input-number v-model="cronInputValue.second.select2.b" /> {{ t('taskPage.seconds') }} {{ t('taskPage.start') }}
                    </el-radio>
                    <el-radio :value="3" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.period') }} <el-input-number v-model="cronInputValue.second.select3.a" /> {{ t('taskPage.to') }} <el-input-number v-model="cronInputValue.second.select3.b" /> {{ t('taskPage.seconds') }}
                    </el-radio>
                    <el-radio :value="4" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.specificSeconds') }} <el-select v-model="cronInputValue.second.select4.a" :placeholder="t('common.pleaseSelect')">
                            <el-option :label="t('taskPage.greaterThan')" value="greaterThan" />
                            <el-option :label="t('taskPage.lessThan')" value="lessThan" />
                            <el-option :label="t('taskPage.equalTo')" value="equalTo" />
                        </el-select> {{ t('taskPage.multipleSelect') }}
                    </el-radio>
                </el-radio-group>
                <div style="display: flex;align-items: center;margin-top: 2rem;">
                    <p>{{ t('taskPage.cronPreview') }}</p><el-input v-model="cronInputValue.second.cronValue" style="width: 70%;"></el-input>
                </div>
            </el-tab-pane>
            <el-tab-pane :label="t('taskPage.minutes')">
                <template #label><el-icon style="margin-right: 0.3rem;"><calendar /></el-icon>{{ t('taskPage.minutes') }}</template>
                <el-radio-group v-model="cronInputValue.minute.value">
                    <el-radio :value="1" style="width: 100%;">{{ t('taskPage.everyMinute') }}</el-radio>
                    <el-radio :value="2" style="width: 100%;margin-top: 1rem;">
                        {{ t('taskPage.every') }} <el-input-number v-model="cronInputValue.minute.select2.a" /> {{ t('taskPage.minutes') }} {{ t('taskPage.executeFrom') }} <el-input-number v-model="cronInputValue.minute.select2.b" /> {{ t('taskPage.minutes') }} {{ t('taskPage.start') }}
                    </el-radio>
                    <el-radio :value="3" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.period') }} <el-input-number v-model="cronInputValue.minute.select3.a" /> {{ t('taskPage.to') }} <el-input-number v-model="cronInputValue.minute.select3.b" /> {{ t('taskPage.minutes') }}
                    </el-radio>
                    <el-radio :value="4" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.specificMinutes') }} <el-select v-model="cronInputValue.minute.select4.a" :placeholder="t('common.pleaseSelect')">
                            <el-option :label="t('taskPage.greaterThan')" value="greaterThan" />
                            <el-option :label="t('taskPage.lessThan')" value="lessThan" />
                            <el-option :label="t('taskPage.equalTo')" value="equalTo" />
                        </el-select> {{ t('taskPage.multipleSelect') }}
                    </el-radio>
                </el-radio-group>
                <div style="display: flex;align-items: center;margin-top: 2rem;">
                    <p>{{ t('taskPage.cronPreview') }}</p><el-input v-model="cronInputValue.minute.cronValue" style="width: 70%;"></el-input>
                </div>
            </el-tab-pane>
            <el-tab-pane :label="t('taskPage.hours')">
                <template #label><el-icon style="margin-right: 0.3rem;"><calendar /></el-icon>{{ t('taskPage.hours') }}</template>
                <el-radio-group v-model="cronInputValue.hour.value">
                    <el-radio :value="1" style="width: 100%;">{{ t('taskPage.everyHour') }}</el-radio>
                    <el-radio :value="2" style="width: 100%;margin-top: 1rem;">
                        {{ t('taskPage.every') }} <el-input-number v-model="cronInputValue.hour.select2.a" /> {{ t('taskPage.hours') }} {{ t('taskPage.executeFrom') }} <el-input-number v-model="cronInputValue.hour.select2.b" /> {{ t('taskPage.hours') }} {{ t('taskPage.start') }}
                    </el-radio>
                    <el-radio :value="3" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.period') }} <el-input-number v-model="cronInputValue.hour.select3.a" /> {{ t('taskPage.to') }} <el-input-number v-model="cronInputValue.hour.select3.b" /> {{ t('taskPage.hours') }}
                    </el-radio>
                    <el-radio :value="4" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.specificHours') }} <el-select v-model="cronInputValue.hour.select4.a" :placeholder="t('common.pleaseSelect')">
                            <el-option :label="t('taskPage.greaterThan')" value="greaterThan" />
                            <el-option :label="t('taskPage.lessThan')" value="lessThan" />
                            <el-option :label="t('taskPage.equalTo')" value="equalTo" />
                        </el-select> {{ t('taskPage.multipleSelect') }}
                    </el-radio>
                </el-radio-group>
                <div style="display: flex;align-items: center;margin-top: 2rem;">
                    <p>{{ t('taskPage.cronPreview') }}</p><el-input v-model="cronInputValue.hour.cronValue" style="width: 70%;"></el-input>
                </div>
            </el-tab-pane>
            <el-tab-pane :label="t('taskPage.days')">
                <template #label><el-icon style="margin-right: 0.3rem;"><calendar /></el-icon>{{ t('taskPage.days') }}</template>
                <el-radio-group v-model="cronInputValue.day.value">
                    <el-radio :value="1" style="width: 100%;">{{ t('taskPage.everyDay') }}</el-radio>
                    <el-radio :value="2" style="width: 100%;margin-top: 1rem;">
                        {{ t('taskPage.every') }} <el-input-number v-model="cronInputValue.day.select2.a" /> {{ t('taskPage.days') }} {{ t('taskPage.executeFrom') }} <el-input-number v-model="cronInputValue.day.select2.b" /> {{ t('taskPage.days') }} {{ t('taskPage.start') }}
                    </el-radio>
                    <el-radio :value="3" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.every') }} <el-input-number v-model="cronInputValue.day.select3.a" /> {{ t('taskPage.days') }} {{ t('taskPage.executeFrom') }} <el-select v-model="cronInputValue.day.select3.a" :placeholder="t('common.pleaseSelect')" style="width: 50%;">
                            <el-option :label="t('taskPage.monday')" :value="t('taskPage.monday')" />
                            <el-option :label="t('taskPage.tuesday')" :value="t('taskPage.tuesday')" />
                            <el-option :label="t('taskPage.wednesday')" :value="t('taskPage.wednesday')" />
                            <el-option :label="t('taskPage.thursday')" :value="t('taskPage.thursday')" />
                            <el-option :label="t('taskPage.friday')" :value="t('taskPage.friday')" />
                            <el-option :label="t('taskPage.saturday')" :value="t('taskPage.saturday')" />
                            <el-option :label="t('taskPage.sunday')" :value="t('taskPage.sunday')" />
                        </el-select> {{ t('taskPage.start') }}
                    </el-radio>
                    <el-radio :value="4" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.specificDays') }} <el-select v-model="cronInputValue.day.select4.a" :placeholder="t('common.pleaseSelect')">
                            <el-option :label="t('taskPage.greaterThan')" value="greaterThan" />
                            <el-option :label="t('taskPage.lessThan')" value="lessThan" />
                            <el-option :label="t('taskPage.equalTo')" value="equalTo" />
                        </el-select> {{ t('taskPage.multipleSelect') }}
                    </el-radio>
                    <el-radio :value="5" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.specificWeekDays') }} <el-select v-model="cronInputValue.day.select5.a" :placeholder="t('common.pleaseSelect')">
                            <el-option :label="t('taskPage.monday')" :value="t('taskPage.monday')" />
                            <el-option :label="t('taskPage.tuesday')" :value="t('taskPage.tuesday')" />
                            <el-option :label="t('taskPage.wednesday')" :value="t('taskPage.wednesday')" />
                            <el-option :label="t('taskPage.thursday')" :value="t('taskPage.thursday')" />
                            <el-option :label="t('taskPage.friday')" :value="t('taskPage.friday')" />
                            <el-option :label="t('taskPage.saturday')" :value="t('taskPage.saturday')" />
                            <el-option :label="t('taskPage.sunday')" :value="t('taskPage.sunday')" />
                        </el-select> {{ t('taskPage.multipleSelect') }}
                    </el-radio>
                    <el-radio :value="6" style="width: 100%;margin-top: 1.5rem;">{{ t('taskPage.lastDayOfMonth') }}</el-radio>
                    <el-radio :value="7" style="width: 100%;margin-top: 1.5rem;">{{ t('taskPage.lastWorkDayOfMonth') }}</el-radio>
                    <el-radio :value="8" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.lastWeekDayOfMonth') }} <el-select v-model="cronInputValue.day.select8.a" :placeholder="t('common.pleaseSelect')">
                            <el-option :label="t('taskPage.monday')" :value="t('taskPage.monday')" />
                            <el-option :label="t('taskPage.tuesday')" :value="t('taskPage.tuesday')" />
                            <el-option :label="t('taskPage.wednesday')" :value="t('taskPage.wednesday')" />
                            <el-option :label="t('taskPage.thursday')" :value="t('taskPage.thursday')" />
                            <el-option :label="t('taskPage.friday')" :value="t('taskPage.friday')" />
                            <el-option :label="t('taskPage.saturday')" :value="t('taskPage.saturday')" />
                            <el-option :label="t('taskPage.sunday')" :value="t('taskPage.sunday')" />
                        </el-select>
                    </el-radio>
                    <el-radio :value="9" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.daysBeforeEndOfMonth') }} <el-input-number v-model="cronInputValue.day.select9.a" /> {{ t('taskPage.days') }}
                    </el-radio>
                    <el-radio :value="10" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.nthWeekDayOfMonth') }} <el-input-number v-model="cronInputValue.day.select10.a" /> <el-select v-model="cronInputValue.day.select10.b" :placeholder="t('common.pleaseSelect')" style="width: 50%;">
                            <el-option :label="t('taskPage.monday')" :value="t('taskPage.monday')" />
                            <el-option :label="t('taskPage.tuesday')" :value="t('taskPage.tuesday')" />
                            <el-option :label="t('taskPage.wednesday')" :value="t('taskPage.wednesday')" />
                            <el-option :label="t('taskPage.thursday')" :value="t('taskPage.thursday')" />
                            <el-option :label="t('taskPage.friday')" :value="t('taskPage.friday')" />
                            <el-option :label="t('taskPage.saturday')" :value="t('taskPage.saturday')" />
                            <el-option :label="t('taskPage.sunday')" :value="t('taskPage.sunday')" />
                        </el-select>
                    </el-radio>
                </el-radio-group>
                <div style="display: flex;align-items: center;margin-top: 2rem;">
                    <p>{{ t('taskPage.cronPreview') }}</p><el-input v-model="cronInputValue.day.cronValue" style="width: 70%;"></el-input>
                </div>
            </el-tab-pane>
            <el-tab-pane :label="t('taskPage.months')">
                <template #label><el-icon style="margin-right: 0.3rem;"><calendar /></el-icon>{{ t('taskPage.months') }}</template>
                <el-radio-group v-model="cronInputValue.month.value">
                    <el-radio :value="1" style="width: 100%;">{{ t('taskPage.everyMonth') }}</el-radio>
                    <el-radio :value="2" style="width: 100%;margin-top: 1rem;">
                        {{ t('taskPage.every') }} <el-input-number v-model="cronInputValue.month.select2.a" /> {{ t('taskPage.months') }} {{ t('taskPage.executeFrom') }} <el-input-number v-model="cronInputValue.month.select2.b" /> {{ t('taskPage.months') }} {{ t('taskPage.start') }}
                    </el-radio>
                    <el-radio :value="3" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.period') }} <el-input-number v-model="cronInputValue.month.select3.a" /> {{ t('taskPage.to') }} <el-input-number v-model="cronInputValue.month.select3.b" /> {{ t('taskPage.months') }}
                    </el-radio>
                    1
                    <el-radio :value="4" style="width: 100%;margin-top: 1.5rem;">
                        {{ t('taskPage.specificMonths') }} <el-select v-model="cronInputValue.month.select4.a" :placeholder="t('common.pleaseSelect')">
                            <el-option :label="t('taskPage.january')" :value="t('taskPage.january')" />
                            <el-option :label="t('taskPage.february')" :value="t('taskPage.february')" />
                            <el-option :label="t('taskPage.march')" :value="t('taskPage.march')" />
                            <el-option :label="t('taskPage.april')" :value="t('taskPage.april')" />
                            <el-option :label="t('taskPage.may')" :value="t('taskPage.may')" />
                            <el-option :label="t('taskPage.june')" :value="t('taskPage.june')" />
                            <el-option :label="t('taskPage.july')" :value="t('taskPage.july')" />
                            <el-option :label="t('taskPage.august')" :value="t('taskPage.august')" />
                            <el-option :label="t('taskPage.september')" :value="t('taskPage.september')" />
                            <el-option :label="t('taskPage.october')" :value="t('taskPage.october')" />
                            <el-option :label="t('taskPage.november')" :value="t('taskPage.november')" />
                            <el-option :label="t('taskPage.december')" :value="t('taskPage.december')" />
                        </el-select> {{ t('taskPage.multipleSelect') }}
                    </el-radio>
                </el-radio-group>
                <div style="display: flex;align-items: center;margin-top: 2rem;">
                    <p>{{ t('taskPage.cronPreview') }}</p><el-input v-model="cronInputValue.hour.cronValue" style="width: 70%;"></el-input>
                </div>
            </el-tab-pane>
        </el-tabs>
        <template #footer>
            <el-button type="info" @click="handleCancel">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" @click="handleSave">{{ t('taskPage.save') }}</el-button>
        </template>
    </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Calendar } from '@element-plus/icons-vue'

const { t } = useI18n()

let visible = ref(true)

const emit = defineEmits<{
    (e: 'updateVisible', value: boolean): void
    // (e: 'save'): void
}>()

const cronInputValue = ref({
    second: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 } },
    minute: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 } },
    hour: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 } },
    day: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 }, select5: { a: 0 }, select8: { a: 0 }, select9: { a: 0 }, select10: { a: 0, b: 0 } },
    month: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 } }
})

// watch(() => props.visible, (val) => {
//     if (!val) {
//         cronInputValue.value = {
//             second: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 } },
//             minute: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 } },
//             hour: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 } },
//             day: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 }, select5: { a: 0 }, select8: { a: 0 }, select9: { a: 0 }, select10: { a: 0, b: 0 } },
//             month: { value: 1, cronValue: '', select2: { a: 0, b: 0 }, select3: { a: 0, b: 0 }, select4: { a: 0 } }
//         }
//     }
// })

const handleCancel = () => {
    emit('updateVisible', false)
}

const handleSave = () => {
    // emit('save')
    emit('updateVisible', false)
}
</script>
<style lang="scss" scoped>
</style>

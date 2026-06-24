<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="$t('userOrganize.assignDevices')" width="1500" @close="emit('handleCloseEquipment', false)">
            <div style="padding:1rem;">
                <div class="device-assign-container">
                    <el-row :gutter="20">
                        <el-col :span="11">
                            <div class="device-panel">
                                <h3 class="top">{{ $t('userOrganize.notCurrentUserDevices') }}</h3>
                                <div class="panel-header">
                                    <el-input v-model="leftSearch" :placeholder="$t('userOrganize.searchDeviceSN')" clearable>
                                        <template #prefix>
                                            <el-icon>
                                                <Search />
                                            </el-icon>
                                        </template>
                                    </el-input>
                                </div>

                                <el-table ref="leftTable" :data="filteredLeftDevices"
                                    @selection-change="handleLeftSelectionChange" border
                                    style="min-height: 22rem;padding: 0 0.9rem;">
                                    <el-table-column type="selection" width="50" align="center" />
                                    <el-table-column prop="group" :label="$t('userOrganize.group')" align="center" width="80">
                                        <template #header>
                                            <div class="header-with-filter"
                                                style="display: flex;justify-content: center;align-items: center;">
                                                <span>{{ $t('userOrganize.group') }}</span>
                                                <el-dropdown trigger="click">
                                                    <el-icon class="filter-icon" style="margin: 0 0 0 0.5rem;">
                                                        <Filter />
                                                    </el-icon>
                                                    <template #dropdown>
                                                        111111111
                                                    </template>
                                                </el-dropdown>
                                            </div>
                                        </template>
                                    </el-table-column>
                                    <el-table-column prop="name" :label="$t('userOrganize.deviceName')" align="center" width="120" />
                                    <el-table-column prop="sn" :label="$t('userOrganize.deviceSN')" align="center" />
                                    <el-table-column prop="group" :label="$t('userOrganize.deviceType')" align="center" fixed="right">
                                        <template #header>
                                            <div class="header-with-filter"
                                                style="display: flex;justify-content: center;align-items: center;">
                                                <span>{{ $t('userOrganize.deviceType') }}</span>
                                                <el-dropdown trigger="click">
                                                    <el-icon class="filter-icon" style="margin: 0 0 0 0.5rem;">
                                                        <Filter />
                                                    </el-icon>
                                                    <template #dropdown>
                                                        111111111
                                                    </template>
                                                </el-dropdown>
                                            </div>
                                        </template>
                                    </el-table-column>
                                </el-table>

                                <div class="panel-footer">
                                    {{ $t('userOrganize.selected') }} {{ leftSelectedCount }} {{ $t('userOrganize.devices') }}
                                </div>
                            </div>
                        </el-col>

                        <el-col :span="1">
                            <div class="transfer-buttons">
                                <el-button type="primary" :disabled="leftSelectedCount === 0" @click="assignToRight">
                                    >
                                </el-button>
                                <el-button type="primary" :disabled="rightSelectedCount === 0" @click="assignToLeft">
                                    < </el-button>
                            </div>
                        </el-col>

                        <el-col :span="11">
                            <div class="device-panel">
                                <h3 class="top">{{ $t('userOrganize.currentUserDevices') }}</h3>
                                <div class="panel-header">
                                    <el-input v-model="rightSearch" :placeholder="$t('userOrganize.searchDeviceSN')" clearable>
                                        <template #prefix>
                                            <el-icon>
                                                <Search />
                                            </el-icon>
                                        </template>
                                    </el-input>
                                </div>

                                <el-table ref="rightTable" :data="filteredRightDevices"
                                    @selection-change="handleRightSelectionChange" border
                                    style="min-height: 22rem;padding: 0 0.9rem;width: 100%;">
                                    <el-table-column type="selection" width="50" align="center" />
                                    <el-table-column prop="group" :label="$t('userOrganize.group')" align="center" width="80">
                                        <template #header>
                                            <div class="header-with-filter"
                                                style="display: flex;justify-content: center;align-items: center;">
                                                <span>{{ $t('userOrganize.group') }}</span>
                                                <el-dropdown trigger="click">
                                                    <el-icon class="filter-icon" style="margin: 0 0 0 0.5rem;">
                                                        <Filter />
                                                    </el-icon>
                                                    <template #dropdown>
                                                        111111111
                                                    </template>
                                                </el-dropdown>
                                            </div>
                                        </template>
                                    </el-table-column>
                                    <el-table-column prop="name" :label="$t('userOrganize.deviceName')" align="center"  />
                                    <el-table-column prop="sn" :label="$t('userOrganize.deviceSN')" align="center" />
                                    <el-table-column prop="group" :label="$t('userOrganize.deviceType')" align="center"  fixed="right">
                                        <template #header>
                                            <div class="header-with-filter"
                                                style="display: flex;justify-content: center;align-items: center;">
                                                <span>{{ $t('userOrganize.deviceType') }}</span>
                                                <el-dropdown trigger="click">
                                                    <el-icon class="filter-icon" style="margin: 0 0 0 0.5rem;">
                                                        <Filter />
                                                    </el-icon>
                                                    <template #dropdown>
                                                        111111111
                                                    </template>
                                                </el-dropdown>
                                            </div>
                                        </template>
                                    </el-table-column>
                                </el-table>

                                <div class="panel-footer">
                                    {{ $t('userOrganize.selected') }} {{ rightSelectedCount }} {{ $t('userOrganize.devices') }}
                                </div>
                            </div>
                        </el-col>
                    </el-row>
                </div>
            </div>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
const props = defineProps({
    obj: {
        type: Object,
        required: false
    }
});
let emit = defineEmits(["handleCloseEquipment"]);
let dialogVisible = ref(true)
import { Search, Filter } from '@element-plus/icons-vue'

// 数据
const leftDevices = ref([]) // 左侧设备列表
const rightDevices = ref([]) // 右侧设备列表
const leftSearch = ref('')
const rightSearch = ref('')
const leftSelectedDevices = ref([])
const rightSelectedDevices = ref([])
const activeGroup = ref('')

// 分组选项
const groupOptions = ref([
    { label: '默认分组', value: '' },
    { label: '分组1', value: '分组1' },
    { label: '分组2', value: '分组2' },
    { label: '分组3', value: '分组3' }
])

// 计算属性
const leftSelectedCount = computed(() => leftSelectedDevices.value.length)
const rightSelectedCount = computed(() => rightSelectedDevices.value.length)

// 搜索过滤
const filteredLeftDevices = computed(() => {
    if (!leftSearch.value) return leftDevices.value
    return leftDevices.value.filter((device: any) =>
        device.sn.includes(leftSearch.value) ||
        device.name.includes(leftSearch.value)
    )
})

const filteredRightDevices = computed(() => {
    if (!rightSearch.value) return rightDevices.value
    return rightDevices.value.filter((device: any) =>
        device.sn.includes(rightSearch.value) ||
        device.name.includes(rightSearch.value)
    )
})

// 事件处理
const handleLeftSelectionChange = (selection: any) => {
    leftSelectedDevices.value = selection
}

const handleRightSelectionChange = (selection: any) => {
    rightSelectedDevices.value = selection
}

// 分配设备
const assignToRight = () => {
    rightDevices.value.push(...leftSelectedDevices.value)
    leftDevices.value = leftDevices.value.filter(
        device => !leftSelectedDevices.value.includes(device)
    )
    leftSelectedDevices.value = []
}

const assignToLeft = () => {
    leftDevices.value.push(...rightSelectedDevices.value)
    rightDevices.value = rightDevices.value.filter(
        device => !rightSelectedDevices.value.includes(device)
    )
    rightSelectedDevices.value = []
}

// 初始化数据
onMounted(() => {
    // 模拟数据
    leftDevices.value = [
        // { id: 1, group: '分组1', name: '设备A', sn: 'SN001', type: 'A' },
        // { id: 2, group: '分组1', name: '设备B', sn: 'SN002', type: 'B' }
    ]
})
// let submitForm = async (formEl: FormInstance | undefined) => {
//     if (!formEl) return
//     await formEl.validate((valid, fields) => {
//         if (valid) {
//             console.log('submit!')
//         } else {
//             console.log('error submit!', fields)
//         }
//     })
// }
</script>
<style lang="scss" scoped>
:deep(.el-select__placeholder),
:deep(.el-input__inner) {
    font-size: 12px;
}

:deep(.el-button>span) {
    font-size: 13px;
}

.device-assign-container {
    background: white;
}

.device-panel {
    border: 1px solid #e4e7ed;
    border-radius: 0.5rem;
}

.top {
    padding: 0.9rem;
    background: #f5f7fa;
    border-bottom: 1px solid #e4e7ed;
}

.panel-header {
    padding: 15px;
    // border-bottom: 1px solid #e4e7ed;
}

.panel-header h3 {
    margin: 0 0 10px 0;
    color: #303133;
}

.panel-footer {
    padding: 10px 15px;
    // background: #f5f7fa;？
    border-top: 1px solid #e4e7ed;
    color: #606266;
}

.transfer-buttons {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    height: 100%;
    gap: 2rem;
}

:deep(.el-table) {
    border: none;
}

:deep(.el-table th) {
    background-color: #f5f7fa;
}

:deep(.el-button+.el-button) {
    margin-left: 0;
}
</style>
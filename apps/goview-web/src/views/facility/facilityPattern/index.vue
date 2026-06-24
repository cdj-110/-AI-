<template>
    <div class="facilityPattern">
        <div class="facilityPatternTitle">
            {{ t('facilityList.deviceModel') }}
        </div>
        <div class="facilityPattern_left">
            <el-button type="primary" icon="Plus" @click="createPatternConfig.bool = true;">{{ t('facilityList.addDeviceModel') }}</el-button>
            <el-form :inline="true" :model="formInline" class="demo-form-inline" ref="ruleFormRef">
                <el-form-item prop="action">
                    <el-input v-model="formInline.device_name" :placeholder="t('facilityList.modelName')" clearable style="width: 13rem;" />
                </el-form-item>
                <el-form-item>
                    <el-button type="primary" :icon="Search" @click="onSubmit">{{ t('facilityList.query') }}</el-button>
                </el-form-item>
            </el-form>
        </div>
        <div>
            <!-- 最新修改自适应样式 -->
            <div class="container-wrapper">
                <div class="cont_list" v-for="(item, index) in tableData" :key="index">
                    <div>
                        <div class="title">{{ item.model_name }}</div>
                    </div>
                    <div class="flex">
                        <div class="flex_1">
                            <el-image class="device-img" :src="deviceMap[item.device_type]?.img" fit="cover" />
                        </div>
                        <div class="flex_2" style="font-size: 12px;">
                            <div>{{ t('facilityList.deviceType') }}：{{ deviceMap[item.device_type]?.name || t('facilityList.unknownDevice') }}</div>
                            <div>{{ t('facilityList.accessCount') }}：{{ item.device_count }}</div>
                            <div>{{ t('facilityList.createTime') }}：{{ item.create_time.replace('T', ' ') }}</div>
                        </div>
                    </div>
                    <div class="foot">
                        <div class="foot_1" @click="dele(item)">{{ t('facilityList.delete') }}</div>
                        <div class="foot_2" @click="detailsList(item)">{{ t('facilityList.details') }}</div>
                    </div>
                </div>
            </div>
            <div style="display: flex;justify-content: flex-end;margin: 1rem 0 1.25rem 0;">
                <el-pagination v-model:current-page="pagenumber" v-model:page-size="pagesize"
                    :page-sizes="[10, 50, 100, 200]" layout="total, sizes, prev, pager,next" :total="total"
                    @size-change="handleSizeChange" @current-change="handleCurrentChange" />
            </div>
        </div>
        <createPattern :createPatternConfig="createPatternConfig" @refear="list();"></createPattern>
    </div>
</template>
<script lang="ts" setup>
import { Search } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { model_list, delete_devicemodel } from '@/api/facilityPattern/index'
const router = useRouter()
import { ElMessage, ElMessageBox } from 'element-plus'
import img1 from '@/assets/wgmx.png'
import img2 from '@/assets/zsbmx.png'
import img3 from '@/assets/zlsb.png'
import useCounterStore from "@/stores/counter";
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let formInline = ref({ device_name: '' }) // 表单
let createPatternConfig = ref({ bool: false, modal: true, title: t('facilityList.createDeviceModel'), width: "55%", draggable: true, showClose: true });//创建设备模型的配置项 - 传入子组件中的值
// 表格数据
let tableData = ref([]);
// 分页 页
let pagenumber = ref(1)
// 分页 条
let pagesize = ref(10)
// 总条数
let total = ref(0)
onMounted(() => {
    list()
})
// 模型列表user_id.value
let list = () => {
    model_list({ user_id: user_id.value, device_name: formInline.value.device_name, pagenumber: pagenumber.value, pagesize: pagesize.value }).then((res: any) => {
        if (res.code == 200) {
            tableData.value = res.data.data
            total.value = res.data.total
        }
    })
}
// 核心映射表
const deviceMap = {
    GW: { name: t('facilityList.gateway'), img: img1 },
    GSD: { name: t('facilityList.gatewaySubDevice'), img: img2 },
    DD: { name: t('facilityList.directDevice'), img: img3 }
}
// 分页 条
let handleSizeChange = (val: number) => {
    list()
}
// 分页 页
let handleCurrentChange = (val: number) => {
    pagenumber.value = val
    list()
}
// 删除
let dele = (e: any) => {
    ElMessageBox.confirm(t('facilityList.confirmDelete').replace('{deviceName}', deviceMap[e.device_type]?.name || t('facilityList.unknownDevice')), t('common.prompt'), {
        confirmButtonText: t('facilityList.confirm'),
        cancelButtonText: t('facilityList.cancel'),
        type: 'warning',
    }).then(() => {
        delete_devicemodel({ model_id: e.model_id, user_id: user_id.value }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg);
                list()
            } else {
                ElMessage.error(res.msg);
            }
        })
    })
}
// 详情
let detailsList = (e: any) => {
    // 方式1：使用相对路径（推荐）
    router.push({
        path: '/facilityPattern/modelDetails', // 相对路径，自动拼接到当前路径后
        query: {
            id: e.model_id, // 传递详情ID
            typeId: e.device_type,
            model_type: e.model_type,
            model_id: e.model_id,
            model_name: e.model_name,
            communication: e.communication,
        }
    })
}
let onSubmit = () => { list() }

</script>
<style lang="scss" scoped>
.facilityPattern {
    background: #fff;
    height: 100%;
    border-radius: 0.7rem;
    padding: 1rem 2rem;
    overflow-y: auto;

    .facilityPatternTitle {
        font-size: 1.3rem;
        font-weight: 700;
    }

    .facilityPattern_left {
        display: flex;
        justify-content: space-between;
        padding: 0.5rem 0;
    }

    :deep(.el-table th.el-table__cell) {
        background-color: #f5f7fa;
    }

    :deep(.el-table .el-table__cell) {
        padding: 15px 0;
    }

    .container-wrapper {
        display: grid;
        /* 核心：自动填充，最小宽度280px，卡片不拉伸 */
        grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
        /* 两端对齐，卡片均匀分布 */
        justify-content: space-between;
        /* 固定间距，不受缩放影响 */
        gap: 16px;
        width: 100%;
        box-sizing: border-box;

        .cont_list {
            width: auto !important;
            height: auto;
            box-sizing: border-box;
            border: 1px #EBE6EB solid;
            padding: 1rem;
            border-radius: 6px;
            color: #241313;

            .title {
                font-size: 14px;
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
                margin-bottom: 0.5rem;
                padding-bottom: 1rem;
                border-bottom: 1px #EBE6EB solid;
            }

            .flex {
                display: flex;
                gap: 1rem;
                justify-content: space-between;
                padding-top: 0.75rem;
                font-size: 14px;

                .flex_1 {
                    width: 60px;
                    flex-shrink: 0;
                }

                .flex_2 {
                    flex: 1;
                    min-width: 0;

                    div {
                        margin: 0 0 0.5rem 0;
                    }
                }
            }

            .foot {
                display: flex;
                justify-content: flex-end;
                margin-top: 0.6rem;
                font-size: 13px;

                .foot_1,
                .foot_2 {
                    width: 3.6875rem;
                    height: 1.8375rem;
                    text-align: center;
                    line-height: 1.8375rem;
                    cursor: pointer;
                }

                .foot_1 {
                    margin-right: 0.75rem;
                    background-color: #f2f3f5;
                    border-radius: 0.3rem;
                }

                .foot_2 {
                    background-color: #409eff;
                    color: #ffff;
                    border-radius: 0.3rem;
                }
            }
        }
    }
}
</style>
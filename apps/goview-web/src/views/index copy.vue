<template>
    <div class="index" ref="enlargeRef">
        <el-menu mode="horizontal" :ellipsis="false">
            <el-menu-item index="0">
                <img style="width: 7rem;" src="@/assets/logos.png">
            </el-menu-item>
            <el-menu-item index="4">
                <a style="font-size: 1.1rem;text-decoration: none;font-weight: 450;" target="_blank" href="https://www.yuque.com/jiayou-4wqsy/px7zph?#">文档</a>
            </el-menu-item>
            <el-menu-item index="2">
                <el-dropdown ref="dropdown1" trigger="click" placement="bottom-end" @command="handleCommand">
                    <span class="dropdown-trigger">
                        <el-badge :value="3" class="item">
                            <img src="@/assets/tongzhi.png"
                                style="width: 1.7rem; vertical-align: middle; margin-left: 5px;">
                        </el-badge>
                    </span>
                    <template #dropdown>
                        <el-dropdown-menu>
                            <el-dropdown-item command="1">
                                <el-badge is-dot class="item"> 告警通知 </el-badge>
                            </el-dropdown-item>
                            <el-dropdown-item command="2">系统消息</el-dropdown-item>
                        </el-dropdown-menu>
                    </template>
                </el-dropdown>
            </el-menu-item>
            <el-menu-item index="1">
                <img src="@/assets/user.png" style="width: 2rem;border-radius: 50%;" v-if="!img"></img>
                <img :src="'data:image/png;base64,' + img" style="width: 2rem;border-radius: 50%;" v-else />
            </el-menu-item>
            <el-sub-menu index="3">
                <template #title>{{ nameValue }}</template>
                <el-menu-item index="3-1"
                    @click="accountFitConfig = { isTrue: true, title: '账号设置' };">账号设置</el-menu-item>
                <el-menu-item index="3-2" @click="outputChange">退出登录</el-menu-item>
            </el-sub-menu>
        </el-menu>
        <el-row>
            <el-col :span="3">
                <layout></layout>
            </el-col>
            <el-col :span="21">
                <div
                    style="height: var(--page-height);background-color: #eeeeee;padding: 0.7rem 0.5rem 0 0.5rem;overflow: hidden;">
                    <el-breadcrumb separator="/">
                        <el-breadcrumb-item><el-icon>
                                <HomeFilled />
                            </el-icon></el-breadcrumb-item>
                        <el-breadcrumb-item v-for="(item, index) in technical" :key="index">{{ item.title
                            }}</el-breadcrumb-item>
                    </el-breadcrumb>
                    <div style="margin-top: 1rem;height: var(--content-height);">
                        <router-view></router-view>
                    </div>
                </div>
            </el-col>
        </el-row>
        <accountFit :config="accountFitConfig"></accountFit>
        <alertMessage v-if="alertValue === '1'" @alarmHistoryClose="alarmHistoryClose"></alertMessage>
        <system v-if="alertValue === '2'" @alarmHistoryClose="alarmHistoryClose"></system>
    </div>
</template>

<script lang="ts" setup>
import layout from "@/components/layout.vue";
import useCounterStore from "@/stores/counter";
import accountFit from "@/views/userOrganize/accountFit/index.vue";
import alertMessage from '@/views/warning/alertMessage/index.vue'
import system from '@/views/dataCenter/system/index.vue'
import { logout } from '@/api/login/index'
import { ElMessage, ElMessageBox } from "element-plus";
import type { Bottom } from "@element-plus/icons-vue";
let store = useCounterStore();//实例化pinia函数
let { technical }: { technical: any } = storeToRefs(store);//将store转换为响应式数据
let { setToken } = useCounterStore();//实例化pinia函数
let enlargeRef: any = ref("");//根元素Ref
let accountFitConfig = ref({ isTrue: false, title: "" });//账号设置弹窗内容信息
let alertValue = ref('') // 告警历史弹窗
let useStore = useCounterStore();//实例化pinia
let { nameValue } = storeToRefs(useStore);
let { vxImg } = storeToRefs(useStore);
let userName = ref('') // 登录名
let img = ref('') // 登录头像
onMounted(() => {
    userName.value = nameValue.value
    img.value = vxImg.value
})
/**
 * 退出登录
 */
let outputChange = () => {
    ElMessageBox.confirm('确定退出登录吗?', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
    }).then(() => {
        logout().then((res: any) => {
            if (res.code == 200) {
                setToken("");
                nameValue.value = ''
                ElMessage.success(res.msg);
            }
        })
    })
}

/**
 * 告警通知取消弹窗
 */
let alarmHistoryClose = (e: any) => { alertValue.value = '' }

let handleCommand = (e: any) => {
    alertValue.value = e

}
</script>

<style lang="scss" scoped>
.index {
    width: 100vw;
    height: 100vh;

    .el-menu--horizontal>.el-menu-item:nth-child(1) {
        margin-right: auto;
    }
}
</style>

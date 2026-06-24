<template>
    <div class="facilityTryout" style="padding: 0 1rem 0 1rem;display: flex;">
        <div>
            <div style="width: 50vw;height: 35vh;border: #cccccc 1px solid;border-bottom: none;position: relative;">
                <div style="width: 100%;height: 5vh;background-color: #eeeeee;position: absolute;bottom: 0;display: flex;align-items: center;justify-content: space-between;padding: 0 1rem;">
                    <el-checkbox v-model="alter1" @change="alterChange1" label="十六进制显示" size="large" />
                    <el-text type="primary">清屏</el-text>
                </div>
                <div style="height: calc(35vh - 5vh);overflow-y: auto;padding: 0 1rem;">
                    <div v-for="(item, index) in verList" :key="index" style="margin: 1rem 0;">
                        <div :style="`font-size: 1rem;display: flex;align-items: center;color: ${item.type === 0 ? '#507eff' : '#77c23a'};`">
                            <p>{{ item.type === 0 ? "下行数据" : "上行数据" }}</p>
                            <p style="margin-left: 0.5rem;">{{ item.time }}</p>
                        </div>
                        <div style="margin-top: 0.7rem;font-size: 0.9rem;">{{ item.content }}</div>
                    </div>
                </div>
            </div>
            <div style="width: 50vw;height: 15vh;border: #cccccc 1px solid;border-top: none;">
                <el-input v-model="valueText" :rows="7" type="textarea" />
            </div>
            <div style="display: flex;align-items: center;justify-content: space-between;margin-top: 1.3rem;">
                <el-checkbox v-model="alter2" @change="alterChange2" label="十六进制发送" size="large" />
                <div style="font-size: 0.9rem;color: #ACACAC;">按下回车发送指令，按下shift+回车换行</div>
                <div>
                    <el-button type="info">取消</el-button>
                    <el-button type="primary">发送指令</el-button>
                </div>
            </div>
        </div>
        <div class="option" style="display: flex;flex-direction: column;margin-left: 1rem;">
            <el-button type="primary" icon="Setting" @click="dialogFormVisible = true;">设置串联参数</el-button>
            <el-button type="primary" icon="Download" style="margin-top: 3vh;">导出日志</el-button>
        </div>
        <el-dialog v-model="dialogFormVisible" title="设置串口参数" width="30vw">
            <el-form :model="form">
                <el-form-item label="波特率">
                    <el-select v-model="form.a" placeholder="请选择波特率">
                        <el-option label="" value="" />
                    </el-select>
                </el-form-item>
                <el-form-item label="数据位">
                    <el-select v-model="form.a" placeholder="请选择数据位">
                        <el-option label="" value="" />
                    </el-select>
                </el-form-item>
                <el-form-item label="停止位">
                    <el-select v-model="form.a" placeholder="请选择停止位">
                        <el-option label="" value="" />
                    </el-select>
                </el-form-item>
                <el-form-item label="校验位">
                    <el-select v-model="form.a" placeholder="请选择校验位">
                        <el-option label="" value="" />
                    </el-select>
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button type="info" @click="dialogFormVisible = false">取消</el-button>
                <el-button type="primary" @click="dialogFormVisible = false">确认</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
let alter1 = ref(false);//是否以十六进制显示
let alter2 = ref(false);//是否以十六进制发送
let valueText = ref("");//发送的指令
let dialogFormVisible = ref(false);//设置串口参数
let form = ref({a: ""});//设置串口参数表单信息

let verList = ref([
    {type: 0, content: "这是内容这是内容这是内容这是内容这是内容", time: "2025-11-27 17:15:11"},
    {type: 1, content: "这是内容这是内容这是内容这是内容这是内容", time: "2025-11-27 17:15:11"},
    {type: 0, content: "这是内容这是内容这是内容这是内容这是内容", time: "2025-11-27 17:15:11"},
    {type: 1, content: "这是内容这是内容这是内容这是内容这是内容", time: "2025-11-27 17:15:11"},
    {type: 0, content: "这是内容这是内容这是内容这是内容这是内容", time: "2025-11-27 17:15:11"},
])

/**
 * 以十六进制显示
 */
let alterChange1 = (e: any) => {
    console.log(e)
}

/**
 * 以十六进制发送
 */
let alterChange2 = (e: any) => {
    console.log(e)
}
</script>

<style lang="scss" scoped>
.facilityTryout{
    .option{
        :deep(.el-button+.el-button){
            margin-left: 0;
        }
    }
}
</style>
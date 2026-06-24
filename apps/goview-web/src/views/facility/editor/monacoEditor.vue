<template>
    <div>
        <div class="container" :style="`height: ${props.height ? props.height : '200px'}`" ref="container"></div>
    </div>
</template>

<script lang="ts" setup>
import * as monaco from "monaco-editor";

let editorInit: any = ref();//保存编辑器实例
let container: any = ref(null);//获取编辑器Ref
let emit = defineEmits(["onChange"]);
let props = withDefaults(defineProps<{
    height: string,
    config?: {
        value?: string,
        language?: string,
        theme?: string,
    },
    readOnly?: boolean
}>(), {
    config: () => ({
        value: "SELECT * FROM users LIMIT 10;",
        language: "SQL",
        theme: "vs",
    }),
    readOnly: false
})

onMounted(() => {
    nextTick(() => {
        //初始化编辑器
        editorInit.value = monaco.editor.create(container.value, {
            value: props.config.value || '',//编辑器初始值
            language: props.config.language,//编辑器语言
            theme: props.config.theme,//编辑器皮肤vs,hc-black,vs-dark
            readOnly: props.readOnly,//是否只读
            minimap: { enabled: false },//是否启用预览图
            columnSelection: false,//启用列编辑 按下shift键位然后按↑↓键位可以实现列选择
            copyWithSyntaxHighlighting: true,//是否应将语法突出显示复制到剪贴板中
            folding: true,//是否启用代码折叠
            links: true,//是否点击链接
            colorDecorators: true,//颜色装饰器
            automaticLayout: true,//是否开启自动布局
            quickSuggestions: true,//是否有代码提示
            formatOnType: true, // 输入时格式化
            formatOnPaste: true, // 粘贴时格式化
        });

        //当编辑器内容改变时，实时监听编辑器内容改变并向外抛出改变的最新内容
        editorInit.value.onDidChangeModelContent(() => {
            emit("onChange", toRaw(editorInit.value).getValue());
        });

    })
})
// 清空编辑器内容的方法
let clears = () => {
    props.config.value = ''
    console.log(props.config.value, '000000')
    editorInit.value.setValue(''); // 清空编辑器内容
};
// 添加格式化方法
let formatDocument = () => {
   editorInit.value.trigger("anyString", "editor.action.formatDocument");
}
// 确保暴露方法
defineExpose({
    clears,
    formatDocument
})

</script>

<style lang="scss" scoped>
.container {
    width: 100%;
    height: 200px;
    border: 1px #D1D5DB solid;
    padding: 5px 0;
}
</style>
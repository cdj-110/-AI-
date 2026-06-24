<template>
    <div>
        <div class="container" :style="`height: ${props.height ? props.height : '500px'}`" ref="container"></div>
    </div>
</template>

<script lang="ts" setup>
import * as monaco from "monaco-editor";

let editorInit: any = ref();
let container: any = ref(null);
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
        value: ``,
        language: "json",
        theme: "vs",
    }),
    readOnly: false
})

onMounted(() => {
    nextTick(() => {
        // 确保 monaco 已经加载
        if (!monaco) {
            console.error('Monaco Editor not loaded');
            return;
        }

        try {
            editorInit.value = monaco.editor.create(container.value, {
                value: props.config.value || '',
                language: props.config.language || 'json',
                theme: props.config.theme || 'vs-dark',
                readOnly: props.readOnly,
                minimap: { enabled: false },
                columnSelection: false,
                copyWithSyntaxHighlighting: true,
                folding: true,
                links: true,
                colorDecorators: true,
                automaticLayout: true,
                quickSuggestions: true,
            });

            editorInit.value.onDidChangeModelContent(() => {
                emit("onChange", editorInit.value);
            });
        } catch (error) {
            console.error('Monaco Editor initialization failed:', error);
        }
    })
})

watch(() => props.config.value, (newVal) => {
    if (editorInit.value && newVal !== undefined) {
        try {
            editorInit.value.setValue(newVal);
        } catch (error) {
            console.error('Failed to set editor value:', error);
        }
    }
}, { immediate: true });

let onFormatChange = () => {
    if (editorInit.value) {
        editorInit.value.trigger("anyString", "editor.action.formatDocument");
    }
}

defineExpose({ onFormatChange });
</script>

<style lang="scss" scoped>
.container {
    width: 100%;
    height: 500px;
}
</style>
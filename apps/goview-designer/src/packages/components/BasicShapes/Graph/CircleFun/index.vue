<template>
    <div class="go-Shape"
        @mousedown="handleMouseDown">
        <svg :width="widthKey" :height="heightKey">
            <!-- 定义渐变 -->
            <defs v-if="useGradient">
                <linearGradient :gradientTransform="`rotate(${backgroundLinearNumber})`" id="myGradient">
                    <stop offset="0%" :stop-color="gradientColor[0]" />
                    <stop offset="100%" :stop-color="gradientColor[1]" />
                </linearGradient>
            </defs>
            <!-- 核心：椭圆（拉伸时自动变椭圆，始终贴满容器四边） -->
            <ellipse :cx="widthKey / 2" :cy="heightKey / 2" 
                :rx="(widthKey - strokeWidthValue) / 2" :ry="(heightKey - strokeWidthValue) / 2"
                :fill="useGradient ? 'url(#myGradient)' : backgroundColor" :stroke="borderColor" 
                :stroke-width="strokeWidthValue"
                :fill-opacity="fillOpacity" :stroke-opacity="strokeOpacity" 
                :stroke-dasharray="computedStrokeDasharray"
                vector-effect="non-scaling-stroke" />
        </svg>
    </div>
</template>

<script setup lang="ts">
import { PropType, toRefs, computed, ref, onMounted, onUnmounted } from 'vue'
import { CreateComponentType } from '@/packages/index.d'

const props = defineProps({
    chartConfig: {
        type: Object as PropType<CreateComponentType>,
        required: true
    }
})

const emit = defineEmits(['update:chartConfig'])

// 解构容器宽高和样式属性
const { w, h } = toRefs(props.chartConfig.attr)
const {
    borderColor, backgroundColor, gradientColor,
    backgroundLinearNumber, fillOpacity, strokeOpacity,
    borderType, dashLength, gapLength, borderWidth
} = toRefs(props.chartConfig.option)

let widthKey = ref<any>(w);
let heightKey = ref<any>(h);
const strokeWidthValue = computed(() => borderWidth.value || 3);

// 计算是否使用渐变
const useGradient = computed(() => {
    return Array.isArray(gradientColor.value) && gradientColor.value.length >= 2
})

// 计算虚线样式
const computedStrokeDasharray = computed(() => {
    if (borderType.value === 'dashed') {
        const dLen = dashLength.value || 5
        const gLen = gapLength.value || 5
        return `${dLen},${gLen}`
    }
    return ''
})

// 拖拽自由拉伸（不限制比例）
const isDragging = ref(false)
const startPoint = ref({ x: 0, y: 0 })
const startSize = ref({ w: 0, h: 0 })

const handleMouseDown = (e: MouseEvent) => {
    const target = e.target as HTMLElement
    if (target.classList.contains('go-Shape')) {
        isDragging.value = true
        startPoint.value = { x: e.clientX, y: e.clientY }
        startSize.value = { w: w.value, h: h.value }
        document.body.style.cursor = 'se-resize'
    }
}

const handleMouseMove = (e: MouseEvent) => {
    if (!isDragging.value) return
    // 自由拉伸宽高，不限制比例
    const newW = Math.max(100, startSize.value.w + (e.clientX - startPoint.value.x))
    const newH = Math.max(50, startSize.value.h + (e.clientY - startPoint.value.y))
    emit('update:chartConfig', {
        ...props.chartConfig,
        attr: { ...props.chartConfig.attr, w: newW, h: newH }
    })
}

const handleMouseUp = () => {
    isDragging.value = false
    document.body.style.cursor = 'default'
}

onMounted(() => {
    window.addEventListener('mousemove', handleMouseMove)
    window.addEventListener('mouseup', handleMouseUp)

    window.addEventListener("keydown",(e) => {
        if(e.key == "Shift"){
            widthKey.value = w.value;
            heightKey.value = w.value;
        }
    })
})

onUnmounted(() => {
    window.removeEventListener('mousemove', handleMouseMove)
    window.removeEventListener('mouseup', handleMouseUp)
})
</script>

<style scoped>
.go-Shape {
    position: relative;
    display: inline-block;
    box-sizing: border-box;
}

.go-Shape svg {
    display: block;
    width: 100%;
    height: 100%;
}
</style>
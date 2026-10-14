<template>
  <div class="go-Polygon">
    <div class="wk-polygon" :style="{ clipPath: `polygon(${points})`, background: `linear-gradient(to bottom right, ${fillColor[0]}, ${fillColor[1]} ${fillColorNumber}%)` }"></div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, PropType, ref, toRefs } from 'vue'
import { CreateComponentType } from '@/packages/index.d'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  }
})

const { w, h } = toRefs(props.chartConfig.attr)
const { fillColor, fillColorNumber, sidesList } = toRefs(props.chartConfig.option)

let points = computed(() => {
  let vertices: Array<string> = [];
  let center = 50;
  let radius = 50;
  let sides = sidesList.value ? sidesList.value : 0;
  if(sides != 3 && sides != 5 && sides != 6 && sides != 7){
    for (let i = 0; i < sides; i++) {
      let angle = (Math.PI * 2 * i) / sides - Math.PI / 2;
      let x = center + radius * Math.cos(angle);
      let y = center + radius * Math.sin(angle);
      vertices.push(`${x}% ${y}%`);
    }
  }else{
    if(sides == 3){
      vertices = ["50% 0%","100% 100%","0% 100%"];
    }
    if(sides == 5){
      vertices = ["50% 0%","100% 34%","79% 100%","20% 100%","0% 34%"];
    }
    if(sides == 6){
      vertices = ["50% 0%","100% 25%","100% 75%","50% 100%","0% 75%","0% 24%"];
    }
    if(sides == 7){
      vertices = ["50% 0%","89% 18%","100% 61%","71% 100%","28% 100%","0% 61%","10% 18%"];
    }
  }
  
  return vertices.join(', ');
});
</script>

<style lang="scss" scoped>
@include go('Polygon') {
  .wk-polygon{
    width: 100%;
    height: 100%;
  }
}
</style>

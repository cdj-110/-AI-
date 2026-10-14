import { createApp, defineComponent, h, ref } from 'vue'
import PointMetricInput from '../../src/components/PointMetricInput.vue'

createApp(defineComponent({
  setup(){
    const metric=ref('opcua-P01'),commits=ref(0),pulse=ref(0)
    const timer=setInterval(()=>pulse.value++,50)
    window.addEventListener('beforeunload',()=>clearInterval(timer))
    return()=>h('main',[
      // Reproduce the production row key and frequent realtime updates.
      h('div',{key:metric.value},[
        h(PointMetricInput,{modelValue:metric.value,'onUpdate:modelValue':(value:string)=>{metric.value=value;commits.value++}}),
        h('span',{id:'pulse'},String(pulse.value)),
      ]),
      h('output',{id:'committed'},metric.value),
      h('output',{id:'commits'},String(commits.value)),
      h('button',{id:'outside'},'保存配置'),
      h(PointMetricInput,{modelValue:'readonly-P01',disabled:true}),
    ])
  },
})).mount('#app')

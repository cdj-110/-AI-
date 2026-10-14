<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

const barRef=ref<HTMLElement>()
const trackWidth=ref(0)
const barLeft=ref(0)
const barWidth=ref(0)
const visible=ref(false)

let host:HTMLElement|undefined
let target:HTMLElement|undefined
let resizeObserver:ResizeObserver|undefined
let mutationObserver:MutationObserver|undefined
let frame=0
let syncing=false

function isUsable(element:HTMLElement){
  const style=window.getComputedStyle(element),rect=element.getBoundingClientRect()
  return style.display!=='none'&&style.visibility!=='hidden'&&rect.width>0&&rect.height>0
}

function findTarget(){
  if(!host)return undefined
  const candidates=Array.from(host.querySelectorAll<HTMLElement>('.point-sheet-scroll')).filter(isUsable)
  return candidates.find(element=>element.scrollWidth>element.clientWidth+1)||candidates[0]
}

function unbindTarget(){
  if(!target)return
  target.removeEventListener('scroll',syncFromTable)
  target.classList.remove('has-persistent-horizontal-scrollbar')
  resizeObserver?.unobserve(target)
  const table=target.querySelector<HTMLElement>('table')
  if(table)resizeObserver?.unobserve(table)
  target=undefined
}

function bindTarget(next:HTMLElement|undefined){
  if(next===target)return
  unbindTarget()
  target=next
  if(!target)return
  target.addEventListener('scroll',syncFromTable,{passive:true})
  resizeObserver?.observe(target)
  const table=target.querySelector<HTMLElement>('table')
  if(table)resizeObserver?.observe(table)
}

function syncFromTable(){
  if(syncing||!target||!barRef.value)return
  syncing=true
  barRef.value.scrollLeft=target.scrollLeft
  syncing=false
}

function syncFromBar(){
  if(syncing||!target||!barRef.value)return
  syncing=true
  target.scrollLeft=barRef.value.scrollLeft
  syncing=false
}

function update(){
  frame=0
  bindTarget(findTarget())
  if(!target){visible.value=false;return}
  const rect=target.getBoundingClientRect(),overflow=target.scrollWidth>target.clientWidth+1
  const intersectsViewport=rect.bottom>0&&rect.top<window.innerHeight
  visible.value=overflow&&intersectsViewport
  target.classList.toggle('has-persistent-horizontal-scrollbar',visible.value)
  if(!visible.value)return
  barLeft.value=Math.max(0,rect.left)
  barWidth.value=Math.max(0,Math.min(rect.right,window.innerWidth)-barLeft.value)
  trackWidth.value=target.scrollWidth
  requestAnimationFrame(syncFromTable)
}

function scheduleUpdate(){
  if(frame)return
  frame=requestAnimationFrame(update)
}

onMounted(async()=>{
  await nextTick()
  host=barRef.value?.closest<HTMLElement>('.legacy-device-workspace')||undefined
  resizeObserver=new ResizeObserver(scheduleUpdate)
  mutationObserver=new MutationObserver(scheduleUpdate)
  if(host){
    resizeObserver.observe(host)
    mutationObserver.observe(host,{attributes:true,attributeFilter:['class'],childList:true})
  }
  document.addEventListener('scroll',scheduleUpdate,true)
  window.addEventListener('resize',scheduleUpdate,{passive:true})
  update()
})

onBeforeUnmount(()=>{
  if(frame)cancelAnimationFrame(frame)
  unbindTarget()
  resizeObserver?.disconnect()
  mutationObserver?.disconnect()
  document.removeEventListener('scroll',scheduleUpdate,true)
  window.removeEventListener('resize',scheduleUpdate)
})
</script>

<template>
  <div
    ref="barRef"
    class="persistent-point-scrollbar"
    :class="{visible}"
    :style="{left:`${barLeft}px`,width:`${barWidth}px`}"
    role="scrollbar"
    aria-label="点位表横向滚动"
    @scroll.passive="syncFromBar"
  ><div :style="{width:`${trackWidth}px`}"/></div>
</template>

<style>
.persistent-point-scrollbar{position:fixed;z-index:90;bottom:0;height:13px;overflow-x:auto;overflow-y:hidden;border:1px solid #cbd5e1;border-bottom:0;background:#eef3f8;box-shadow:0 -2px 8px rgba(15,23,42,.08);opacity:0;pointer-events:none;transition:opacity .12s ease}.persistent-point-scrollbar.visible{opacity:1;pointer-events:auto}.persistent-point-scrollbar>div{height:1px}.persistent-point-scrollbar::-webkit-scrollbar{height:11px}.persistent-point-scrollbar::-webkit-scrollbar-thumb{border:2px solid transparent;border-radius:999px;background:#8fa4bd;background-clip:padding-box}.persistent-point-scrollbar::-webkit-scrollbar-track{background:#e8eef5}.point-sheet-scroll.has-persistent-horizontal-scrollbar{scrollbar-width:none!important;-ms-overflow-style:none}.point-sheet-scroll.has-persistent-horizontal-scrollbar::-webkit-scrollbar{width:0!important;height:0!important}@media(max-width:900px){.persistent-point-scrollbar{display:none}}
</style>

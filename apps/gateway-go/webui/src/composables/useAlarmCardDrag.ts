import {computed,nextTick,onBeforeUnmount,ref,watch} from 'vue'
import {readAlarmCardPosition,saveAlarmCardPosition,removeAlarmCardPosition} from '../alarmCardPositions.ts'

let front=1
// Keep a grid placeholder while moving the actual card within the visible list.
// State belongs to the keyed rule card, never to the changing event payload.
export function useAlarmCardDrag(bounds:()=>HTMLElement|undefined,resetKey:()=>number,scope:()=>string,group:()=>string,pinned:()=>boolean=()=>false) {
  const element=ref<HTMLElement>(),slot=ref<HTMLElement>()
  const position=ref<{x:number;y:number}>(),dragging=ref(false)
  const width=ref(0),height=ref(0),limitHeight=ref(0),z=ref(1)
  let origin:{id:number;x:number;y:number;left:number;top:number}|undefined
  let observer:ResizeObserver|undefined
  let restored=false
  function remember(){
    const area=bounds()?.getBoundingClientRect(),p=position.value
    if(area&&p)saveAlarmCardPosition(scope(),group(),{x:Math.max(0,p.x-area.left),y:Math.max(0,p.y-area.top)})
  }
  function geometry(){
    const area=bounds()?.getBoundingClientRect(),card=element.value?.getBoundingClientRect()
    if(!area||!card)return
    return {area,card}
  }
  function place(x:number,y:number){
    const g=geometry();if(!g)return
    const {area,card}=g
    position.value={x:Math.max(area.left+8,Math.min(x,area.right-card.width-8)),y:Math.max(area.top+8,Math.min(y,area.bottom-card.height-8))}
  }
  function measure(){
    const area=bounds()?.getBoundingClientRect()
    if(!area||area.width<16||area.height<40||!slot.value||!element.value)return
    width.value=Math.max(1,Math.min(slot.value.clientWidth,area.width-16))
    limitHeight.value=Math.max(40,area.height-16)
    height.value=element.value.getBoundingClientRect().height
    if(!restored){
      restored=true
      const saved=readAlarmCardPosition(scope(),group())
      if(saved){z.value=++front;place(area.left+saved.x,area.top+saved.y)}
    }
    if(position.value)void nextTick(()=>{if(position.value)place(position.value.x,position.value.y)})
  }
  function raise(){if(position.value)z.value=++front}
  function stop(){if(origin)remember();origin=undefined;dragging.value=false;window.removeEventListener('pointermove',move);window.removeEventListener('pointerup',stop);window.removeEventListener('pointercancel',stop);window.removeEventListener('blur',stop)}
  function move(event:PointerEvent){if(origin&&event.pointerId===origin.id){event.preventDefault();place(origin.left+event.clientX-origin.x,origin.top+event.clientY-origin.y)}}
  function start(event:PointerEvent){
    if(event.button!==0||!element.value)return
    const rect=element.value.getBoundingClientRect()
    stop();measure();z.value=++front
    origin={id:event.pointerId,x:event.clientX,y:event.clientY,left:rect.left,top:rect.top};dragging.value=true
    place(rect.left,rect.top);event.preventDefault()
    window.addEventListener('pointermove',move,{passive:false});window.addEventListener('pointerup',stop);window.addEventListener('pointercancel',stop);window.addEventListener('blur',stop)
  }
  function reset(){stop();position.value=undefined;removeAlarmCardPosition(scope(),group())}
  function keyboard(event:KeyboardEvent){
    if(event.key==='Home'){event.preventDefault();reset();return}
    const steps:Record<string,[number,number]>={ArrowLeft:[-16,0],ArrowRight:[16,0],ArrowUp:[0,-16],ArrowDown:[0,16]}
    const step=steps[event.key],rect=element.value?.getBoundingClientRect()
    if(!step||!rect)return
    event.preventDefault();measure();z.value=++front;place(rect.left+step[0],rect.top+step[1]);remember()
  }
  watch([element,slot,bounds],()=>{
    observer?.disconnect()
    observer=new ResizeObserver(measure)
    for(const node of [element.value,slot.value,bounds()])if(node)observer.observe(node)
    measure()
  },{flush:'post'})
  watch(resetKey,reset)
  onBeforeUnmount(()=>{stop();observer?.disconnect()})
  const style=computed(()=>position.value?{position:'fixed' as const,left:`${position.value.x}px`,top:`${position.value.y}px`,width:`${width.value}px`,maxHeight:`${limitHeight.value}px`,overflow:'auto',zIndex:z.value,margin:'0'}:{})
  // Each slot owns a stacking context: ordinary drag/raise never covers a pinned card.
  const slotStyle=computed(()=>({position:'relative' as const,zIndex:(pinned()?1000000:0)+z.value,...(position.value?{height:`${height.value}px`}:{})}))
  return {element,slot,style,slotStyle,dragging,position,start,keyboard,raise}
}

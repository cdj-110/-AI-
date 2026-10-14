import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

export function useDraggablePanel() {
  const element = ref<HTMLElement>()
  const position = ref<{x:number;y:number}>()
  const dragging = ref(false)
  let origin: {id:number;x:number;y:number;left:number;top:number}|undefined
  let observer: ResizeObserver|undefined
  const style = computed(() => position.value ? {position:'fixed' as const,left:`${position.value.x}px`,top:`${position.value.y}px`,right:'auto',bottom:'auto',margin:'0'} : {})
  function place(x:number,y:number) {
    const rect=element.value?.getBoundingClientRect()
    if(!rect)return
    position.value={x:Math.max(8,Math.min(x,window.innerWidth-rect.width-8)),y:Math.max(8,Math.min(y,window.innerHeight-rect.height-8))}
  }
  function clamp(){if(position.value)place(position.value.x,position.value.y)}
  function move(event:PointerEvent){if(origin&&event.pointerId===origin.id){event.preventDefault();place(origin.left+event.clientX-origin.x,origin.top+event.clientY-origin.y)}}
  function stop(){origin=undefined;dragging.value=false;window.removeEventListener('pointermove',move);window.removeEventListener('pointerup',stop);window.removeEventListener('pointercancel',stop)}
  function start(event:PointerEvent){
    if(event.button!==0||!element.value)return
    const rect=element.value.getBoundingClientRect()
    stop();origin={id:event.pointerId,x:event.clientX,y:event.clientY,left:rect.left,top:rect.top};dragging.value=true
    event.preventDefault()
    window.addEventListener('pointermove',move,{passive:false});window.addEventListener('pointerup',stop);window.addEventListener('pointercancel',stop)
  }
  function keyboard(event:KeyboardEvent){
    const steps:Record<string,[number,number]>={ArrowLeft:[-16,0],ArrowRight:[16,0],ArrowUp:[0,-16],ArrowDown:[0,16]}
    const step=steps[event.key],rect=element.value?.getBoundingClientRect()
    if(step&&rect){event.preventDefault();place(rect.left+step[0],rect.top+step[1])}
    if(event.key==='Home'){event.preventDefault();reset()}
  }
  function reset(){stop();position.value=undefined}
  watch(element,(value)=>{observer?.disconnect();reset();if(value){observer=new ResizeObserver(clamp);observer.observe(value)}},{flush:'post'})
  onMounted(()=>window.addEventListener('resize',clamp))
  onBeforeUnmount(()=>{stop();observer?.disconnect();window.removeEventListener('resize',clamp)})
  return {element,style,dragging,start,keyboard,reset}
}

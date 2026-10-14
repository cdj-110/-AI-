export type AlarmCardPosition={x:number;y:number}
const prefix='gateway-alarm-card-position-v1:'
const scopePrefix=(scope:string)=>prefix+encodeURIComponent(scope)+':'
const key=(scope:string,group:string)=>scopePrefix(scope)+encodeURIComponent(group)
export function readAlarmCardPosition(scope:string,group:string):AlarmCardPosition|undefined {
  try{
    const value=JSON.parse(localStorage.getItem(key(scope,group))||'null')
    if(value&&typeof value.x==='number'&&typeof value.y==='number'&&Number.isFinite(value.x)&&Number.isFinite(value.y)&&value.x>=0&&value.y>=0)return{x:value.x,y:value.y}
  }catch{/* Browser storage may be unavailable; dragging must still work. */}
}
export function saveAlarmCardPosition(scope:string,group:string,value:AlarmCardPosition){
  try{localStorage.setItem(key(scope,group),JSON.stringify(value))}catch{/* Best effort local preference. */}
}
export function removeAlarmCardPosition(scope:string,group:string){try{localStorage.removeItem(key(scope,group))}catch{}}
export function clearAlarmCardPositions(scope:string){
  try{
    const keys=[]
    for(let i=0;i<localStorage.length;i++){const k=localStorage.key(i);if(k?.startsWith(scopePrefix(scope)))keys.push(k)}
    for(const k of keys)localStorage.removeItem(k)
  }catch{}
}

export function flattenEventComponents(items:any[]):any[] {
  return (Array.isArray(items)?items:[]).flatMap(c=>[c,...flattenEventComponents(c?.groupList||[])])
}
const validValue=(value:any)=>typeof value==='boolean'||typeof value==='string'&&value.trim()!==''||typeof value==='number'&&Number.isFinite(value)
function readProperty(components:any[],id:string,key:string){
  if(['__proto__','constructor','prototype'].includes(key))return undefined
  const component=components.find(c=>c.id===id)
  if(!component)return undefined
  return {...component.attr,...component.option,...component.styles}[key]
}
export function evaluateConditions(group:any,components:any[]):boolean|undefined {
  if(!Array.isArray(group?.tiaoList)||!group.tiaoList.length)return undefined
  const outcomes=group.tiaoList.map((c:any)=>{
    const left=readProperty(components,c.id1,c.content1)
    let right=c.tiaojianleixingType==='组件联动'?readProperty(components,c.id2,c.content2):c.tiaojianleixingType==='固定值'?c.value:undefined
    if(!validValue(left)||!validValue(right))return undefined
    if(c.tiaojianleixingType==='固定值'){
      if(typeof left==='number'){const n=Number(right);if(!Number.isFinite(n))return undefined;right=n}
      if(typeof left==='boolean'){if(right==='true')right=true;else if(right==='false')right=false;else if(typeof right!=='boolean')return undefined}
    }
    if(c.fu==='等于')return left===right
    if(c.fu==='不等于')return left!==right
    if(typeof left!=='number'||typeof right!=='number')return undefined
    switch(c.fu){case '大于':return left>right;case '大于等于':return left>=right;case '小于':return left<right;case '小于等于':return left<=right;default:return undefined}
  })
  if(outcomes.some((v:any)=>v===undefined))return undefined
  if(group.leixingType==='满足任意条件')return outcomes.some((v:any)=>v===true)
  if(group.leixingType==='满足全部条件')return outcomes.every((v:any)=>v===true)
  return undefined
}
export const conditionMatches=(group:any,components:any[])=>evaluateConditions(group,components)===true
export function conditionEventSnapshot(items:any[]):string {
  const components=flattenEventComponents(items)
  return JSON.stringify(components.flatMap(c=>(c.interactActions||[]).filter((e:any)=>e.changeType==='满足条件').map((e:any)=>({
    owner:c.id,event:e,values:(e.tiaoJianList?.tiaoList||[]).map((rule:any)=>[readProperty(components,rule.id1,rule.content1),rule.tiaojianleixingType==='组件联动'?readProperty(components,rule.id2,rule.content2):rule.value])
  }))))
}
export function createConditionEventRunner(){
  const previous=new Map<string,{signature:string;met:boolean}>()
  return {
    reset:()=>previous.clear(),
    evaluate:(items:any[],run:(component:any,event:any)=>void)=>{
      const components=flattenEventComponents(items),active=new Set<string>()
      for(const component of components)for(const event of component.interactActions||[]){
        if(event.changeType!=='满足条件'||(event.startTrue!=='启用'&&event.changeAttr!=='禁启用')||event.isNewly||!event.id)continue
        const key=`${component.id}:${event.id}`;active.add(key)
        const signature=JSON.stringify([event.tiaoJianList,event.changeAttr,event.form?.targetComponent])
        const old=previous.get(key),met=evaluateConditions(event.tiaoJianList,components)
        if(met===undefined)continue // 缺失/无效数据不视为“不满足”，避免断流后恢复时重复触发。
        previous.set(key,{signature,met}) // 先锁存再执行，防止动作改变属性引起递归重复触发。
        if(met&&(!old||old.signature!==signature||!old.met))run(component,event)
      }
      for(const key of previous.keys())if(!active.has(key))previous.delete(key)
    }
  }
}

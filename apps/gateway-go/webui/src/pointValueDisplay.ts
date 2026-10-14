// Match the gateway's transport rule: shorten only redundant decimal tails
// within two float64 steps, without capping genuine measurement precision.
export function normalizePointNumber(value:number):number {
  if(value===0)return 0
  if(!Number.isFinite(value)||Number.isInteger(value))return value
  const digits=value.toExponential().split('e')[0].replace(/[-.]/g,'').length
  const bytes=new DataView(new ArrayBuffer(8))
  bytes.setFloat64(0,Math.abs(value))
  const bits=bytes.getBigUint64(0)
  bytes.setBigUint64(0,bits+1n)
  const next=bytes.getFloat64(0)
  bytes.setBigUint64(0,bits-1n)
  const previous=bytes.getFloat64(0)
  const step=Math.max(next-Math.abs(value),Math.abs(value)-previous)
  for(let precision=1;precision<=digits-4;precision++){
    const candidate=Number(value.toPrecision(precision))
    if(candidate!==0&&Math.abs(candidate-value)<=2*step)return candidate
  }
  return value
}

function displayJSON(value:unknown):string {
  return JSON.stringify(value,(_key,item)=>typeof item==='number'?normalizePointNumber(item):item)
}

// Presentation only: preserve raw measurements, exact integer text and strings.
export function formatPointValueDisplay(value:unknown, decimals:unknown=0, valueText?:string):string {
  if(value===undefined||value===null)return '-'
  if(valueText)return valueText
  if(typeof value==='object')return displayJSON(value)
  if(typeof value!=='number')return String(value)
  if(!Number.isFinite(value))return '-'
  const places=Number(decimals)
  if(Number.isInteger(places)&&places>0&&places<=20)return value.toFixed(places)
  // Zero is the existing automatic-precision mode, not integer rounding.
  if(Number.isInteger(value))return String(value)
  return String(normalizePointNumber(value))
}

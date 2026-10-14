/** Display-only formatting: never round the stored/plot source values. */
export function formatMetricNumber(value:number,decimals?:number,trimZeros=false):string {
  if(!Number.isFinite(value))return '-';
  const fixed=typeof decimals==='number'&&Number.isInteger(decimals)&&decimals>=0&&decimals<=6;
  let text=fixed?value.toFixed(decimals):Number.isInteger(value)?String(value):String(Number(value.toPrecision(12)));
  if(text.startsWith('-')&&Number(text)===0)text=text.slice(1);
  if(trimZeros&&text.includes('.')&&!/[eE]/.test(text))text=text.replace(/0+$/,'').replace(/\.$/,'');
  return text;
}

/** Numeric text is also used by some gateways. Preserve IDs, large integers and ordinary text. */
export function formatMetricValue(value:unknown,decimals?:number):string {
  if(typeof value==='number')return formatMetricNumber(value,decimals);
  if(typeof value==='string'&&/^-?(?:0|[1-9]\d*)(?:\.\d+|[eE][+-]?\d+|\.\d+[eE][+-]?\d+)$/.test(value)) {
    const number=Number(value);
    if(Number.isFinite(number)&&Math.abs(number)<=Number.MAX_SAFE_INTEGER)return formatMetricNumber(number,decimals);
  }
  return typeof value==='object'?JSON.stringify(value):String(value);
}

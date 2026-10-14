import { clearSession, serverURL, token } from './session'

type Method='GET'|'POST'|'PATCH'|'PUT'|'DELETE'
type ApiEnvelope<T>={code?:number;message?:string;data?:T}

function messageOf(value:any,fallback:string){
  if(Array.isArray(value?.message))return value.message.join('；')
  return String(value?.message||fallback)
}

export async function request<T>(path:string,options:{method?:Method;data?:any;query?:Record<string,unknown>;timeout?:number}={}):Promise<T>{
  const query=Object.entries(options.query||{}).filter(([,value])=>value!==''&&value!==undefined&&value!==null)
    .map(([key,value])=>`${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`).join('&')
  const url=`${serverURL()}${path.startsWith('/')?path:`/${path}`}${query?`?${query}`:''}`
  const result=await uni.request({
    url,
    method:options.method||'GET',
    data:options.data,
    timeout:options.timeout||12000,
    header:{
      'Content-Type':'application/json',
      ...(token()?{Authorization:`Bearer ${token()}`}:{})
    }
  })
  const status=Number(result.statusCode||0)
  const body=result.data as ApiEnvelope<T>|any
  if(status===401){
    clearSession()
    uni.reLaunch({url:'/pages/login/index'})
    throw new Error('登录已失效，请重新登录')
  }
  if(status<200||status>=300)throw new Error(messageOf(body,`请求失败（${status||'网络异常'}）`))
  if(body&&typeof body==='object'&&'data'in body)return body.data as T
  return body as T
}

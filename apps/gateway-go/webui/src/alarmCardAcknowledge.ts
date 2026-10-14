import type {AlarmEvent} from './alarmNotifications.ts'

// Capture only the events visible when clicked. Newly triggered events must
// never be acknowledged implicitly while this request is in flight.
export async function acknowledgeCardEvents(events:readonly AlarmEvent[],request:(eventId:string)=>Promise<unknown>){
  const pending=[...new Map(events.filter(e=>e.state==='ACTIVE'&&!e.acknowledged).map(e=>[e.eventId,e])).values()]
  const acknowledged:string[]=[],errors:string[]=[]
  for(const event of pending){
    try{await request(event.eventId);acknowledged.push(event.eventId)}
    catch(error){errors.push(`${event.limitName||event.ruleName}：${error instanceof Error?error.message:String(error)}`)}
  }
  return {acknowledged,error:errors.join('；')}
}

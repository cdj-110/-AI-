// Keep the already-unlocked page playing; opening a panel is not an audio handoff.
export class AlarmAudioOwner {
  private user=''
  private ready=false
  private id:string
  private storage:Pick<Storage,'getItem'|'setItem'|'removeItem'>
  private now:()=>number
  constructor(id:string,storage:Pick<Storage,'getItem'|'setItem'|'removeItem'>,now:()=>number=Date.now){this.id=id;this.storage=storage;this.now=now}
  key(user=this.user){return `gateway-alarm-audio-owner-v1:${encodeURIComponent(user)}`}
  private read(){try{const v=JSON.parse(this.storage.getItem(this.key())||'null');return v&&typeof v.id==='string'&&Number.isFinite(v.at)&&this.now()-v.at<45000&&v.at<=this.now()+5000?v:undefined}catch{return}}
  setScope(user:string){if(user!==this.user){this.release();this.user=user;this.ready=false}}
  setReady(ready:boolean){this.ready=ready;if(!ready)this.release()}
  check(renew=false){
    if(!this.user)return {owns:false,available:false}
    let owner=this.read()
    if(this.ready&&(!owner||(owner.id===this.id&&renew))){
      try{this.storage.setItem(this.key(),JSON.stringify({id:this.id,at:this.now()}));owner=this.read()}catch{}
    }
    return {owns:this.ready&&owner?.id===this.id,available:!!owner}
  }
  release(){try{if(this.read()?.id===this.id)this.storage.removeItem(this.key())}catch{}}
}

export const channelRoles = [
  {key:'collect',label:'采集'},
  {key:'forward',label:'转发'},
] as const

export function channelRole(channel:{role?:string;forwardProtocol?:string}) {
  if(channel.role==='collect'||channel.role==='forward')return channel.role
  const protocol=channel.forwardProtocol?.trim().toLowerCase()
  return protocol&&protocol!=='none'?'forward':'collect'
}

export function groupChannels<T extends {role?:string}>(channels:T[]) {
  return channelRoles.map(role=>({...role,channels:channels.filter(channel=>channelRole(channel)===role.key)}))
}

// Presentation only: preserve source order and identifiers, never sort config arrays.
export function groupResourceTree<T extends {resourceKey:string;channels:any[]}>(resources:T[]) {
  return resources.map(resource=>({...resource,groups:groupChannels(resource.channels).map(group=>({...group,treeKey:`${resource.resourceKey}:${group.key}`}))}))
}

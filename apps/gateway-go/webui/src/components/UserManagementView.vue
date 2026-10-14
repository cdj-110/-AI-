<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { IconChevronDown } from '@tabler/icons-vue'
import { api } from '../api'
import {usePageDraftGuard} from '../composables/usePageDraftGuard.ts'
import { confirmAction } from '../confirm'

type UserItem = { username:string; displayName:string; password?:string; roleKey:string; enabled:boolean; mustChangePassword?:boolean; mfaEnabled?:boolean }
type RoleItem = { roleKey:string; name:string; description:string; permissions:string[]; builtIn?:boolean }
type PermissionItem = { key:string; name:string; group:string; description?:string }
type SecurityPayload = { users:UserItem[]; roles:RoleItem[]; permissions:PermissionItem[] }
type HTTPSSettings = { enabled:boolean; requiresRestart?:boolean; url?:string; certificateFile?:string; expiresAt?:string; fingerprint?:string; certificateOK:boolean }
type CurrentUser = { username:string; displayName?:string; roleKey:string }
type MFAStatus = { enabled:boolean; username:string; recoveryRemaining:number }
type MFASetup = { secret:string; otpauthUri:string; qrCodeDataUrl:string; expiresIn:number }

const loading=ref(false), saving=ref(false), message=ref(''), error=ref('')
const activePanel=ref<'users'|'roles'|'security'>('users')
const users=ref<UserItem[]>([]), roles=ref<RoleItem[]>([]), permissions=ref<PermissionItem[]>([])
const httpsSettings=ref<HTTPSSettings|null>(null), httpsSaving=ref(false)
const mfaStatus=ref<MFAStatus|null>(null), mfaSaving=ref(false), mfaDialog=ref<'setup-password'|'setup-scan'|'recovery'|'disable'|''>('')
const expandedSecurity=ref<'mfa'|'https'|''>('')
const mfaPassword=ref(''), mfaCode=ref(''), mfaSetup=ref<MFASetup|null>(null), recoveryCodes=ref<string[]>([]), recoverySaved=ref(false)
const currentUser=ref<CurrentUser|null>(null)
const editingUser=ref(-1), editingRole=ref(-1)
const userEditorOpen=ref(false)
const roleEditorOpen=ref(false), roleError=ref('')
const userDraft=reactive<UserItem>({username:'',displayName:'',password:'',roleKey:'viewer',enabled:true})
const roleDraft=reactive<RoleItem>({roleKey:'',name:'',description:'',permissions:[]})
const savedSecurityDraft=ref(''),initialUserDraft=ref(''),initialRoleDraft=ref('')
const usersDirty=()=>userEditorOpen.value&&JSON.stringify(userDraft)!==initialUserDraft.value
const rolesDirty=()=>roleEditorOpen.value&&JSON.stringify(roleDraft)!==initialRoleDraft.value
const securitySnapshot=()=>JSON.stringify({users:users.value,roles:roles.value})
const requestSecurityLeave=usePageDraftGuard(()=>!!savedSecurityDraft.value&&(securitySnapshot()!==savedSecurityDraft.value||usersDirty()||rolesDirty()),()=>{
  const saved=JSON.parse(savedSecurityDraft.value);users.value=saved.users;roles.value=saved.roles;cancelUser();cancelRole()
},async()=>{
  if(usersDirty()){commitUser();if(userEditorOpen.value)return false}
  if(rolesDirty()){commitRole();if(roleEditorOpen.value)return false}
  await save()
},'#panel-users .sub-tabs button')
const permissionTreeExpanded=ref(true)
const expandedGroups=reactive<Record<string,boolean>>({})
const permissionGroups=computed(()=>{const groups=new Map<string,PermissionItem[]>();for(const item of permissions.value){if(item.key==='*')continue;if(!groups.has(item.group))groups.set(item.group,[]);groups.get(item.group)!.push(item)}return [...groups.entries()].map(([name,items])=>({name,items}))})
const concretePermissionKeys=computed(()=>permissions.value.filter(item=>item.key!=='*').map(item=>item.key))
const effectivePermissions=computed(()=>new Set(roleDraft.permissions.includes('*')?concretePermissionKeys.value:roleDraft.permissions))
const selectedPermissionCount=computed(()=>concretePermissionKeys.value.filter(key=>effectivePermissions.value.has(key)).length)
const allPermissionsChecked=computed(()=>concretePermissionKeys.value.length>0&&selectedPermissionCount.value===concretePermissionKeys.value.length)
const somePermissionsChecked=computed(()=>selectedPermissionCount.value>0&&!allPermissionsChecked.value)

async function load(){loading.value=true;error.value='';try{const [data,https,session,mfa]=await Promise.all([api<SecurityPayload>('/api/security'),api<HTTPSSettings>('/api/security/https'),api<CurrentUser>('/api/session'),api<MFAStatus>('/api/security/mfa')]);users.value=(data.users||[]).map(item=>({...item,password:''}));roles.value=(data.roles||[]).map(item=>({...item,permissions:[...(item.permissions||[])]}));permissions.value=data.permissions||[];httpsSettings.value=https;currentUser.value=session;mfaStatus.value=mfa;savedSecurityDraft.value=securitySnapshot();for(const group of permissionGroups.value)if(expandedGroups[group.name]===undefined)expandedGroups[group.name]=true}catch(cause){error.value=cause instanceof Error?cause.message:'用户和角色读取失败'}finally{loading.value=false}}
function beginUser(index=-1){editingUser.value=index;userEditorOpen.value=true;const item=index>=0?users.value[index]:{username:'',displayName:'',password:'',roleKey:roles.value.find(role=>role.roleKey==='viewer')?.roleKey||roles.value[0]?.roleKey||'',enabled:true,mustChangePassword:true};Object.assign(userDraft,item,{password:''});initialUserDraft.value=JSON.stringify(userDraft)}
function cancelUser(){userEditorOpen.value=false;editingUser.value=-1;Object.assign(userDraft,{username:'',displayName:'',password:'',roleKey:'viewer',enabled:true,mustChangePassword:false})}
function passwordError(username:string,password:string){if([...password].length<6)return'密码至少需要 6 个字符';if(!password.trim())return'密码不能全部为空格';const lower=password.toLowerCase();if(lower.includes('123456')||lower.includes('abcdef')||['password','password123','admin123','qwerty123'].includes(lower))return'密码过于简单，请避免常见密码和连续字符';if(username.length>=3&&lower.includes(username.toLowerCase()))return'密码不能包含用户账号';return''}
function commitUser(){const username=userDraft.username.trim();if(!username){error.value='账号不能为空';return}if(editingUser.value<0&&users.value.some(item=>item.username===username)){error.value=`账号已存在：${username}`;return}if(editingUser.value<0&&!userDraft.password){error.value='新增用户必须设置临时密码';return}if(userDraft.password){const problem=passwordError(username,userDraft.password);if(problem){error.value=problem;return}}const next={...userDraft,username,displayName:userDraft.displayName.trim(),mustChangePassword:editingUser.value<0||!!userDraft.password||userDraft.mustChangePassword};if(editingUser.value>=0)users.value.splice(editingUser.value,1,next);else users.value.push(next);error.value='';cancelUser()}
async function removeUser(index:number){const user=users.value[index];if(!user||user.username==='admin')return;if(await confirmAction({title:'删除账号',message:`确认删除账号“${user.username}”？`,detail:'删除后，该账号将无法继续登录网关。',confirmText:'确认删除',danger:true}))users.value.splice(index,1)}
function temporaryPassword(){const alphabet='ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789';const bytes=new Uint32Array(12);crypto.getRandomValues(bytes);return `Wk!9${[...bytes].map(value=>alphabet[value%alphabet.length]).join('')}`}
async function resetPassword(index:number){const user=users.value[index];if(!user||!await confirmAction({title:'重置密码',message:`确认重置账号“${user.username}”的密码？`,detail:'系统将生成一次性临时密码，该账号下次登录时必须修改。',confirmText:'确认重置'}))return;const password=temporaryPassword();saving.value=true;error.value='';message.value='';try{const nextUsers=users.value.map((item,itemIndex)=>itemIndex===index?{...item,password,mustChangePassword:true}:item);await api('/api/security',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({users:nextUsers,roles:roles.value})});message.value=`账号 ${user.username} 已重置。临时密码：${password}（请立即复制，页面刷新后不再显示）`;await load()}catch(cause){error.value=cause instanceof Error?cause.message:'密码重置失败'}finally{saving.value=false}}
function beginRole(index=-1){editingRole.value=index;roleEditorOpen.value=true;roleError.value='';error.value='';permissionTreeExpanded.value=true;const item=index>=0?roles.value[index]:{roleKey:'',name:'',description:'',permissions:['status.view'],builtIn:false};Object.assign(roleDraft,item,{permissions:[...(item.permissions||[])]});initialRoleDraft.value=JSON.stringify(roleDraft)}
function cancelRole(){roleEditorOpen.value=false;roleError.value='';editingRole.value=-1;Object.assign(roleDraft,{roleKey:'',name:'',description:'',permissions:[],builtIn:false})}
function nextRoleKey(name:string){const normalized=name.normalize('NFKD').toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-+|-+$/g,'');const stem=normalized||'custom-role';let candidate=stem,index=2;while(roles.value.some(item=>item.roleKey===candidate))candidate=`${stem}-${index++}`;return candidate}
function commitRole(){const name=roleDraft.name.trim();if(!name){roleError.value='请填写角色名称';return}const roleKey=editingRole.value>=0?roleDraft.roleKey.trim():nextRoleKey(name);const next={...roleDraft,roleKey,name,description:roleDraft.description.trim(),permissions:[...roleDraft.permissions]};if(editingRole.value>=0)roles.value.splice(editingRole.value,1,next);else roles.value.push(next);error.value='';roleError.value='';cancelRole()}
async function removeRole(index:number){const role=roles.value[index];if(role.builtIn)return;if(users.value.some(user=>user.roleKey===role.roleKey)){error.value=`角色 ${role.name} 仍被用户使用，不能删除`;return}if(await confirmAction({title:'删除角色',message:`确认删除角色“${role.name}”？`,confirmText:'确认删除',danger:true}))roles.value.splice(index,1)}
function groupState(group:{items:PermissionItem[]}){const selected=group.items.filter(item=>effectivePermissions.value.has(item.key)).length;return{checked:selected===group.items.length&&group.items.length>0,indeterminate:selected>0&&selected<group.items.length,selected}}
function expandWildcard(){if(roleDraft.permissions.includes('*'))roleDraft.permissions=[...concretePermissionKeys.value]}
function toggleAllPermissions(){if(roleDraft.builtIn)return;roleDraft.permissions=allPermissionsChecked.value?[]:['*']}
function toggleGroup(group:{items:PermissionItem[]}){if(roleDraft.builtIn)return;expandWildcard();const keys=new Set(roleDraft.permissions);if(groupState(group).checked)for(const item of group.items)keys.delete(item.key);else for(const item of group.items)keys.add(item.key);roleDraft.permissions=[...keys]}
function togglePermission(key:string){if(roleDraft.builtIn)return;expandWildcard();const keys=new Set(roleDraft.permissions);if(keys.has(key))keys.delete(key);else keys.add(key);roleDraft.permissions=[...keys]}
function toggleEveryGroup(){const shouldExpand=permissionGroups.value.some(group=>expandedGroups[group.name]===false);for(const group of permissionGroups.value)expandedGroups[group.name]=shouldExpand}
async function save(){saving.value=true;error.value='';message.value='';try{await api('/api/security',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({users:users.value,roles:roles.value})});message.value='用户和角色已保存';await load()}catch(cause){error.value=cause instanceof Error?cause.message:'保存失败'}finally{saving.value=false}}
async function setHTTPSEnabled(enabled:boolean){const action=enabled?'启用 HTTPS':'停用 HTTPS';if(!await confirmAction({title:action,message:`确认${action}？`,detail:'配置保存后需要重启网关服务才能生效。启用内置证书时，浏览器首次访问会提示确认自签名证书。',confirmText:'确认'}))return;httpsSaving.value=true;error.value='';message.value='';try{httpsSettings.value=await api<HTTPSSettings>('/api/security/https',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({enabled})});message.value=`${action}配置已保存，请重启网关服务后通过 ${httpsSettings.value.url||'新地址'} 访问。`}catch(cause){error.value=cause instanceof Error?cause.message:`${action}失败`}finally{httpsSaving.value=false}}
function openMFASetup(){mfaPassword.value='';mfaCode.value='';mfaSetup.value=null;recoveryCodes.value=[];recoverySaved.value=false;mfaDialog.value='setup-password'}
function toggleSecurityDetail(panel:'mfa'|'https'){expandedSecurity.value=expandedSecurity.value===panel?'':panel}
function openMFADisable(){mfaPassword.value='';mfaCode.value='';mfaDialog.value='disable'}
function closeMFADialog(){if(mfaDialog.value==='recovery'&&!recoverySaved.value)return;mfaDialog.value='';mfaPassword.value='';mfaCode.value='';mfaSetup.value=null;recoveryCodes.value=[]}
async function beginMFASetup(){if(!mfaPassword.value){error.value='请输入当前密码';return}mfaSaving.value=true;error.value='';try{mfaSetup.value=await api<MFASetup>('/api/security/mfa/setup',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({password:mfaPassword.value})});mfaPassword.value='';mfaDialog.value='setup-scan'}catch(cause){error.value=cause instanceof Error?cause.message:'创建绑定信息失败'}finally{mfaSaving.value=false}}
async function enableMFA(){if(!mfaCode.value.trim()){error.value='请输入身份验证器中的 6 位动态码';return}mfaSaving.value=true;error.value='';try{const result=await api<{ok:boolean;recoveryCodes:string[]}>('/api/security/mfa/enable',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({code:mfaCode.value})});recoveryCodes.value=result.recoveryCodes||[];recoverySaved.value=false;mfaDialog.value='recovery';message.value='身份验证器二次登录已启用';await load()}catch(cause){error.value=cause instanceof Error?cause.message:'启用二次验证失败'}finally{mfaSaving.value=false}}
async function disableMFA(){if(!mfaPassword.value||!mfaCode.value.trim()){error.value='请输入当前密码和动态验证码（或恢复码）';return}mfaSaving.value=true;error.value='';try{await api('/api/security/mfa/disable',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({password:mfaPassword.value,code:mfaCode.value})});mfaDialog.value='';message.value='身份验证器二次登录已停用';await load()}catch(cause){error.value=cause instanceof Error?cause.message:'停用二次验证失败'}finally{mfaSaving.value=false}}
async function copyMFA(value:string){await navigator.clipboard.writeText(value);message.value='已复制到剪贴板'}
async function copyRecoveryCodes(){await copyMFA(recoveryCodes.value.join('\n'));recoverySaved.value=true}
function openStable(){localStorage.setItem('gateway-active-tab','users');window.location.assign('/')}
onMounted(load)
async function guardedReload(){if(await requestSecurityLeave())await load()}
</script>

<template><section id="panel-users"><p v-if="message" class="notice good">{{message}}</p><p v-if="error" class="notice bad">{{error}}</p><section class="card"><div class="section-title"><div><h2>用户管理</h2><p class="meta">按角色分配最小权限；修改密码时才填写密码框，留空会保留原密码。</p></div><div><button :disabled="loading" @click="guardedReload">重新加载</button><button class="primary" :disabled="saving||loading" @click="save">{{saving?'保存中…':'保存用户和角色'}}</button></div></div>
<div class="sub-tabs"><button :class="{active:activePanel==='users'}" @click="activePanel='users'">用户管理</button><button :class="{active:activePanel==='roles'}" @click="activePanel='roles'">角色管理</button><button :class="{active:activePanel==='security'}" @click="activePanel='security'">登录安全</button></div>
<template v-if="activePanel==='users'"><div class="panel-toolbar"><span>共 {{users.length}} 个账号</span><button class="primary" @click="beginUser()">新增账号</button></div><div class="table-scroll"><table><thead><tr><th>账号</th><th>姓名</th><th>角色</th><th>状态</th><th>操作</th></tr></thead><tbody><tr v-for="(user,index) in users" :key="user.username"><td class="mono">{{user.username}}</td><td>{{user.displayName||'-'}}</td><td>{{roles.find(role=>role.roleKey===user.roleKey)?.name||user.roleKey}}</td><td><span class="state-tag" :class="user.enabled?'ok':'bad'">{{user.enabled?'启用':'停用'}}</span> <span v-if="user.mfaEnabled" class="state-tag ok">二次验证</span> <span v-if="user.mustChangePassword" class="state-tag">需改密</span></td><td class="row-actions"><button @click="beginUser(index)">编辑</button><button :disabled="saving||user.username===currentUser?.username" :title="user.username===currentUser?.username?'本人请使用右上角“修改密码”':''" @click="resetPassword(index)">重置密码</button><button :disabled="user.username==='admin'" @click="removeUser(index)">删除</button></td></tr></tbody></table></div>
<div v-if="userEditorOpen" class="user-edit-modal"><div class="user-edit-dialog"><div><h3>{{editingUser>=0?'编辑账号':'新增账号'}}</h3><p class="meta">临时密码至少 6 位，可使用纯数字、字母或中文，无需组合；用户下次登录时必须修改。</p></div><div class="user-edit-grid"><label><span>账号<b class="required-mark">*</b></span><input v-model.trim="userDraft.username" :disabled="editingUser>=0" autocomplete="off" required/></label><label>姓名<input v-model.trim="userDraft.displayName"/></label><label><span>角色<b class="required-mark">*</b></span><select v-model="userDraft.roleKey" :disabled="userDraft.username==='admin'"><option v-for="role in roles" :key="role.roleKey" :value="role.roleKey">{{role.name}}</option></select></label><label>状态<select v-model="userDraft.enabled" :disabled="userDraft.username==='admin'"><option :value="true">启用</option><option :value="false">停用</option></select></label><label class="span-2"><span>{{editingUser>=0?'新密码（留空表示不修改）':'临时密码'}}<b v-if="editingUser<0" class="required-mark">*</b></span><input v-model="userDraft.password" type="password" minlength="6" maxlength="128" autocomplete="new-password" :disabled="editingUser>=0&&userDraft.username===currentUser?.username" :placeholder="editingUser>=0&&userDraft.username===currentUser?.username?'本人请使用右上角“修改密码”':'至少 6 位，无需字母、数字或符号组合'"/></label></div><div class="dialog-actions"><button @click="cancelUser">取消</button><button class="primary" @click="commitUser">保存</button></div></div></div></template>
<template v-else-if="activePanel==='roles'"><div class="panel-toolbar"><span>内置角色不可删除，权限由系统固定。</span><button class="primary" @click="beginRole()">新增角色</button></div><div class="role-grid"><article v-for="(role,index) in roles" :key="role.roleKey"><div><strong>{{ role.name }}</strong><span v-if="role.builtIn" class="state-tag">内置</span></div><code>{{ role.roleKey }}</code><p>{{ role.description||'暂无说明' }}</p><small>{{ role.permissions.includes('*')?'全部权限':`${role.permissions.length} 项权限` }}</small><div class="row-actions"><button @click="beginRole(index)">查看{{ role.builtIn?'':'/编辑' }}</button><button :disabled="role.builtIn" @click="removeRole(index)">删除</button></div></article></div>
<div v-if="roleEditorOpen" class="user-edit-modal role-edit-modal" role="dialog" aria-modal="true" :aria-label="editingRole>=0?'角色详情':'新增角色'">
 <div class="user-edit-dialog role-edit-dialog">
  <div class="role-edit-head"><div><h3>{{editingRole>=0?'角色详情':'新增角色'}}</h3><p class="meta">按功能模块分配最小权限；父节点支持全选，部分选中时显示半选状态。</p></div><button type="button" class="dialog-close" aria-label="关闭" @click="cancelRole">×</button></div>
  <div class="role-edit-body">
   <div class="role-editor-heading"><span class="permission-count">已选 {{selectedPermissionCount}} / {{concretePermissionKeys.length}}</span></div>
   <div class="role-edit-fields"><label><span>角色名称<b class="required-mark">*</b></span><input v-model.trim="roleDraft.name" :class="{invalid:roleError&&!roleDraft.name.trim()}" :disabled="roleDraft.builtIn" required aria-required="true" placeholder="请输入角色名称"/></label><label><span>角色标识</span><input v-model="roleDraft.roleKey" disabled :placeholder="editingRole>=0?'':'保存时由系统自动生成'"/><small>用于系统关联用户和权限，创建后保持不变。</small></label><label class="span-2"><span>说明</span><input v-model.trim="roleDraft.description" :disabled="roleDraft.builtIn" placeholder="选填，说明该角色的职责范围"/></label></div>
   <p v-if="roleError" class="role-edit-error" role="alert">{{roleError}}</p>
   <div class="permission-tree-toolbar"><strong>功能权限</strong><span v-if="roleDraft.builtIn">内置角色权限由系统固定</span><button type="button" @click="toggleEveryGroup">展开/收起全部模块</button></div>
   <div class="permission-tree" :class="{readonly:roleDraft.builtIn}">
    <div class="permission-tree-row permission-root-row">
      <button type="button" class="permission-chevron" :class="{collapsed:!permissionTreeExpanded}" :aria-label="permissionTreeExpanded?'收起权限树':'展开权限树'" @click="permissionTreeExpanded=!permissionTreeExpanded"/>
      <label><input type="checkbox" :checked="allPermissionsChecked" :indeterminate="somePermissionsChecked" :disabled="roleDraft.builtIn" @change="toggleAllPermissions"/><span><strong>全部权限</strong><small>包含当前全部 {{concretePermissionKeys.length}} 项权限</small></span></label>
      <code>*</code>
    </div>
    <div v-if="permissionTreeExpanded" class="permission-tree-branches">
      <section v-for="group in permissionGroups" :key="group.name" class="permission-tree-group">
        <div class="permission-tree-row permission-group-row">
          <button type="button" class="permission-chevron" :class="{collapsed:expandedGroups[group.name]===false}" :aria-label="expandedGroups[group.name]===false?'展开模块':'收起模块'" @click="expandedGroups[group.name]=expandedGroups[group.name]===false"/>
          <label><input type="checkbox" :checked="groupState(group).checked" :indeterminate="groupState(group).indeterminate" :disabled="roleDraft.builtIn" @change="toggleGroup(group)"/><span><strong>{{group.name}}</strong><small>{{groupState(group).selected}} / {{group.items.length}} 项</small></span></label>
        </div>
        <div v-if="expandedGroups[group.name]!==false" class="permission-tree-leaves">
          <label v-for="item in group.items" :key="item.key" class="permission-leaf-row">
            <span class="permission-tree-line" aria-hidden="true"/>
            <input type="checkbox" :checked="effectivePermissions.has(item.key)" :disabled="roleDraft.builtIn" @change="togglePermission(item.key)"/>
            <span class="permission-leaf-copy"><strong>{{item.name}}</strong><small>{{item.description||'允许使用该功能'}}</small></span>
            <code>{{item.key}}</code>
          </label>
        </div>
      </section>
    </div>
   </div>
  </div>
  <div class="dialog-actions"><button @click="cancelRole">关闭</button><button v-if="!roleDraft.builtIn" class="primary" @click="commitRole">确认</button></div>
 </div>
</div></template>
<template v-else><div class="login-security-stack">
<section class="security-setting mfa-security-panel">
 <div class="security-setting-main">
  <span class="state-tag" :class="mfaStatus?.enabled?'ok':'bad'">{{mfaStatus?.enabled?'已启用':'未启用'}}</span>
  <div class="security-setting-copy"><h3>身份验证器二次登录</h3><p>登录时增加动态验证码，降低密码泄露后的账号风险。</p></div>
  <div class="security-setting-actions"><button type="button" class="security-detail-toggle" :class="{expanded:expandedSecurity==='mfa'}" :aria-expanded="expandedSecurity==='mfa'" @click="toggleSecurityDetail('mfa')"><span>详情</span><IconChevronDown aria-hidden="true"/></button><button v-if="!mfaStatus?.enabled" class="primary" :disabled="mfaSaving" @click="openMFASetup">绑定身份验证器</button><button v-else class="danger-outline" :disabled="mfaSaving" @click="openMFADisable">停用二次验证</button></div>
 </div>
 <div v-if="expandedSecurity==='mfa'" class="security-detail-body"><p>使用 Microsoft Authenticator、Google Authenticator 等应用生成的 6 位动态验证码；动态码每 30 秒更新，启用后下次登录生效，无需重启网关。</p><dl><div><dt>当前账号</dt><dd class="mono">{{mfaStatus?.username||currentUser?.username||'-'}}</dd></div><div><dt>验证方式</dt><dd>TOTP 动态验证码</dd></div><div v-if="mfaStatus?.enabled"><dt>剩余恢复码</dt><dd>{{mfaStatus.recoveryRemaining}} 条</dd></div></dl></div>
</section>
<section class="security-setting">
 <div class="security-setting-main">
  <span class="state-tag" :class="httpsSettings?.enabled&&httpsSettings?.certificateOK?'ok':'bad'">{{httpsSettings?.enabled?'HTTPS 已配置':'当前使用 HTTP'}}</span>
  <div class="security-setting-copy"><h3>Web 管理页面传输加密</h3><p>加密账号、密码和配置内容的传输，避免局域网明文泄露。</p></div>
  <div class="security-setting-actions"><button type="button" class="security-detail-toggle" :class="{expanded:expandedSecurity==='https'}" :aria-expanded="expandedSecurity==='https'" @click="toggleSecurityDetail('https')"><span>详情</span><IconChevronDown aria-hidden="true"/></button><button v-if="!httpsSettings?.enabled" class="primary" :disabled="httpsSaving" @click="setHTTPSEnabled(true)">{{httpsSaving?'处理中…':'生成证书并启用 HTTPS'}}</button><button v-else :disabled="httpsSaving" @click="setHTTPSEnabled(false)">{{httpsSaving?'处理中…':'停用 HTTPS'}}</button></div>
 </div>
 <div v-if="expandedSecurity==='https'" class="security-detail-body"><p>内置证书适合现场快速启用，也可在工程配置中填写正式证书路径。配置修改后需在“系统维护”中重启服务。</p><dl><div><dt>重启后访问地址</dt><dd>{{httpsSettings?.url||'-'}}</dd></div><div v-if="httpsSettings?.expiresAt"><dt>证书有效期</dt><dd>{{new Date(httpsSettings.expiresAt).toLocaleString()}}</dd></div><div v-if="httpsSettings?.fingerprint"><dt>SHA-256 指纹</dt><dd class="mono">{{httpsSettings.fingerprint}}</dd></div></dl></div>
</section>
</div></template></section></section>
<div v-if="mfaDialog" class="user-edit-modal mfa-modal" role="dialog" aria-modal="true" aria-label="身份验证器二次登录"><div class="user-edit-dialog mfa-dialog">
 <div class="mfa-dialog-head"><div><h3>{{mfaDialog==='disable'?'停用二次验证':mfaDialog==='recovery'?'保存恢复码':'绑定身份验证器'}}</h3><p class="meta">{{mfaDialog==='setup-scan'?'使用身份验证器扫描二维码并输入动态码完成绑定':mfaDialog==='recovery'?'恢复码仅显示一次，请保存在安全位置':mfaDialog==='disable'?'验证身份后停用当前账号的二次登录':'先验证当前登录密码'}}</p></div><button v-if="mfaDialog!=='recovery'||recoverySaved" type="button" class="dialog-close" @click="closeMFADialog">×</button></div>
 <div class="mfa-dialog-body">
  <template v-if="mfaDialog==='setup-password'"><label>当前密码<b class="required-mark">*</b><input v-model="mfaPassword" type="password" autocomplete="current-password" @keyup.enter="beginMFASetup"></label></template>
  <template v-else-if="mfaDialog==='setup-scan'"><div class="mfa-scan-layout"><img :src="mfaSetup?.qrCodeDataUrl" alt="身份验证器绑定二维码"><div><strong>无法扫码时手动输入密钥</strong><code>{{mfaSetup?.secret}}</code><button type="button" @click="copyMFA(mfaSetup?.secret||'')">复制密钥</button><small>类型选择“基于时间”，位数 6 位，周期 30 秒。</small></div></div><label>6 位动态验证码<b class="required-mark">*</b><input v-model.trim="mfaCode" inputmode="numeric" autocomplete="one-time-code" maxlength="6" placeholder="000000" @keyup.enter="enableMFA"></label></template>
  <template v-else-if="mfaDialog==='recovery'"><p class="mfa-recovery-warning">手机遗失或身份验证器不可用时，可使用恢复码登录。每条只能使用一次。</p><div class="mfa-recovery-grid"><code v-for="code in recoveryCodes" :key="code">{{code}}</code></div><label class="mfa-saved-check"><input v-model="recoverySaved" type="checkbox">我已将恢复码保存到安全位置</label></template>
  <template v-else><label>当前密码<b class="required-mark">*</b><input v-model="mfaPassword" type="password" autocomplete="current-password"></label><label>动态验证码或恢复码<b class="required-mark">*</b><input v-model.trim="mfaCode" autocomplete="one-time-code" @keyup.enter="disableMFA"></label></template>
 </div>
 <div class="dialog-actions"><button v-if="mfaDialog!=='recovery'" @click="closeMFADialog">取消</button><button v-if="mfaDialog==='setup-password'" class="primary" :disabled="mfaSaving" @click="beginMFASetup">{{mfaSaving?'验证中…':'下一步'}}</button><button v-else-if="mfaDialog==='setup-scan'" class="primary" :disabled="mfaSaving" @click="enableMFA">{{mfaSaving?'启用中…':'确认并启用'}}</button><button v-else-if="mfaDialog==='recovery'" @click="copyRecoveryCodes">复制全部</button><button v-if="mfaDialog==='recovery'" class="primary" :disabled="!recoverySaved" @click="closeMFADialog">完成</button><button v-else-if="mfaDialog==='disable'" class="danger" :disabled="mfaSaving" @click="disableMFA">{{mfaSaving?'停用中…':'确认停用'}}</button></div>
</div></div></template>

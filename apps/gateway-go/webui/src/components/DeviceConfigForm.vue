<script setup lang="ts">
import { computed, watch } from 'vue'
import { confirmAction } from '../confirm'
import { initializeDeviceDraftDefaults } from '../configDraftDefaults.ts'

const props=defineProps<{device:any;readonly:boolean;isForward:boolean}>()
const emit=defineEmits<{test:[];readAddress:[];remove:[]}>()
const prefix=computed(()=>props.device.protocol==='opcua'?'opc.tcp://':'')
const stripped=computed(()=>String(props.device.address||'').replace(/^[a-z][a-z0-9+.-]*:\/\//i,''))
const host=computed({get:()=>{const value=stripped.value,index=value.lastIndexOf(':');return index>0?value.slice(0,index):value},set:(value:string)=>{props.device.address=`${prefix.value}${value}${port.value?`:${port.value}`:''}`}})
const port=computed({get:()=>{const value=stripped.value,index=value.lastIndexOf(':');return index>0?value.slice(index+1):''},set:(value:string|number)=>{props.device.address=`${prefix.value}${host.value}${value?`:${value}`:''}`}})
const splitAddress=computed(()=>['modbus-tcp','iec104','siemens-s7','opcua','iec61850'].includes(props.device.protocol))
const isOPCDA=computed(()=>props.device.protocol==='opcda')
const isDLT645=computed(()=>props.device.protocol==='dlt645')
const isGooseCollect=computed(()=>props.device.protocol==='iec61850-goose')
const isGooseForward=computed(()=>props.device.protocol==='iec61850-goose-publisher')

async function changeOPCDASecurityMode(){
  if(props.device.securityMode==='legacy_connect'){
    const accepted=await confirmAction({title:'启用 Legacy Connect',message:'Legacy Connect 仅用于旧版 DCOM 系统，安全性较低。',detail:'Packet Integrity 不会在认证失败后自动降级。仅应在隔离的旧系统环境中主动选择 Legacy。',confirmText:'继续使用 Legacy',danger:true})
    if(!accepted)props.device.securityMode='packet_integrity'
  }
  props.device.authenticationLevel=props.device.securityMode==='legacy_connect'?'connect':props.device.securityMode==='packet_privacy'?'packet_privacy':'packet_integrity'
}
watch(()=>[props.device?.deviceKey,props.device?.protocol],()=>{if(!props.readonly)initializeDeviceDraftDefaults(props.device)},{immediate:true})
</script>

<template>
  <div class="exact-device-config">
    <div v-if="isDLT645&&!isForward" class="dlt-device-config">
      <section class="dlt-config-section" aria-label="基本信息">
        <div class="dlt-config-heading"><h4>基本信息</h4><span>设备名称、标识与启用状态</span></div>
        <div class="dlt-config-grid">
          <label>设备名称<input v-model.trim="device.name" :disabled="readonly"/></label>
          <label>设备编号<input :value="device.deviceKey" readonly/></label>
          <label>设备状态<select v-model="device.enabled" :disabled="readonly"><option :value="true">启用</option><option :value="false">停用</option></select></label>
        </div>
      </section>
      <section class="dlt-config-section" aria-label="通信参数">
        <div class="dlt-config-heading"><h4>通信参数</h4><span>DL/T 645-2007 / 1997</span></div>
        <div class="dlt-config-grid">
          <label>串口<input v-model.trim="device.address" class="mono" :disabled="readonly"/></label>
          <label>协议版本<select v-model="device.dlt645.version" :disabled="readonly"><option value="auto">自动识别（优先 2007）</option><option value="2007">DL/T 645-2007</option><option value="1997">DL/T 645-1997</option></select></label>
          <label>电表地址<input v-model.trim="device.dlt645.meterAddress" class="mono" maxlength="12" placeholder="000000000001" :disabled="readonly"/></label>
          <label>单次响应超时（ms）<input v-model.number="device.dlt645.responseTimeoutMilliseconds" type="number" min="100" max="10000" :disabled="readonly"/></label>
          <label>失败重试次数<input v-model.number="device.dlt645.retries" type="number" min="0" max="3" :disabled="readonly"/></label>
          <label>前导 FE 字节数<input v-model.number="device.dlt645.wakeupBytes" type="number" min="0" max="8" :disabled="readonly"/></label>
        </div>
        <div class="dlt-config-note">
          <p>电表地址为 12 位十进制数字，需保留前导零。</p>
          <p>推荐串口参数 2400 / 8E1，已有串口资源不会自动修改。单次响应超时从请求发出后计时，不包含等待总线时间。</p>
        </div>
      </section>
      <details class="dlt-config-section dlt-config-credentials">
        <summary>写入凭据<span>高级配置 · 仅写入数据时需要</span></summary>
        <div class="dlt-config-grid">
          <label>4 字节密码<input v-model="device.dlt645.writePassword" type="password" maxlength="8" autocomplete="new-password" placeholder="00000000" :disabled="readonly"/></label>
          <label>4 字节操作者代码<input v-model="device.dlt645.operatorCode" type="password" maxlength="8" autocomplete="new-password" placeholder="00000000" :disabled="readonly"/></label>
        </div>
        <div class="dlt-config-note"><p>只用于普通数据项写入；系统不开放校时、拉合闸、清零和修改地址。</p></div>
      </details>
      <footer class="dlt-config-actions">
        <span>自动识别结果仅用于运行状态展示，不会覆盖配置。</span>
        <div class="point-actions"><button @click="emit('test')">测试连接</button><button :disabled="readonly" @click="emit('readAddress')">读取通信地址</button><button class="danger" :disabled="readonly" @click="emit('remove')">删除设备</button></div>
      </footer>
    </div>
    <template v-else-if="isOPCDA&&!isForward">
      <div class="opcda-config-sections">
        <section class="opcda-config-section">
          <div class="opcda-config-heading"><div><strong>基本信息</strong><small>设备标识、名称和运行状态</small></div></div>
          <div class="opcda-config-row opcda-basic-row">
            <label>设备编号<input :value="device.deviceKey" readonly/></label>
            <label>设备名称<input v-model.trim="device.name" :disabled="readonly"/></label>
            <label>协议<input :value="device.protocol" readonly/></label>
            <label>设备状态<select v-model="device.enabled" :disabled="readonly"><option :value="true">启用</option><option :value="false">停用</option></select></label>
          </div>
        </section>

        <section class="opcda-config-section">
          <div class="opcda-config-heading"><div><strong>OPC DA 服务器</strong><small>服务器地址及 COM 类标识</small></div></div>
          <div class="opcda-config-row opcda-server-row">
            <label>服务器 IP<input v-model.trim="device.address" placeholder="例如 192.168.1.141" :disabled="readonly"/></label>
            <label>ProgID<input v-model.trim="device.progId" class="mono" placeholder="Matrikon.OPC.Simulation.1" :disabled="readonly"/></label>
            <label>CLSID<input v-model.trim="device.clsid" class="mono" placeholder="F8582CF2-88FB-11D0-B850-00C0F0104305" :disabled="readonly"/></label>
          </div>
        </section>

        <section class="opcda-config-section">
          <div class="opcda-config-heading"><div><strong>身份认证</strong><small>Windows 账户与 DCOM RPC 安全参数</small></div></div>
          <div class="opcda-config-row opcda-identity-row">
            <label>域 / 工作组<input v-model.trim="device.domain" placeholder="DESKTOP-XXXX" :disabled="readonly"/></label>
            <label>客户端工作站<input v-model.trim="device.workstation" placeholder="EG100" :disabled="readonly"/></label>
            <label>用户名<input v-model.trim="device.username" autocomplete="username" :disabled="readonly"/></label>
            <label>密码<input v-model="device.password" type="password" autocomplete="new-password" :disabled="readonly"/></label>
          </div>
          <div class="opcda-config-row opcda-security-row">
            <label>认证方式<select v-model="device.authentication" :disabled="readonly"><option value="ntlm">NTLMv2</option></select></label>
            <label>DCOM 安全模式<select v-model="device.securityMode" :disabled="readonly" @change="changeOPCDASecurityMode"><option value="packet_integrity">Packet Integrity（默认，签名校验，防篡改）</option><option value="packet_privacy">Packet Privacy（加密并签名，防窃听和篡改）</option><option value="legacy_connect">Legacy Connect（仅连接认证，兼容旧系统）</option></select></label>
            <label>RPC 认证级别<input :value="device.authenticationLevel" class="mono" readonly/></label>
          </div>
          <div v-if="device.securityMode==='legacy_connect'" class="form-hint warning">Legacy Connect 仅用于旧版 DCOM 系统；Packet Integrity 不会自动降级到此模式。</div>
        </section>

        <section class="opcda-config-section">
          <div class="opcda-config-heading"><div><strong>采集组</strong><small>OPC Group、更新频率和本地绑定参数</small></div></div>
          <div class="opcda-config-row opcda-group-row">
            <label>Group 名称<input v-model.trim="device.groupName" class="mono" :disabled="readonly"/></label>
            <label>Group 更新率(ms)<input v-model.number="device.requestedUpdateRateMilliseconds" type="number" min="10" :disabled="readonly"/></label>
            <label>Deadband(%)<input v-model.number="device.deadband" type="number" min="0" max="100" step="0.1" :disabled="readonly"/></label>
            <label>Locale ID<input v-model.number="device.localeId" type="number" min="0" :disabled="readonly"/></label>
            <label>本地绑定 IP<input v-model.trim="device.localBindIp" placeholder="留空自动选择" :disabled="readonly"/></label>
          </div>
        </section>

        <section class="opcda-config-section">
          <div class="opcda-config-heading"><div><strong>读取策略</strong><small>同步读取或 OPC DA 2.0 数据变化订阅</small></div></div>
          <div class="opcda-config-row opcda-read-row">
            <label>读取模式<select v-model="device.readMode" :disabled="readonly"><option value="sync">同步读取（IOPCSyncIO）</option><option value="subscription">异步订阅（IOPCDataCallback）</option></select></label>
            <label>同步读取源<select v-model="device.readSource" :disabled="readonly || device.readMode==='subscription'"><option value="device">DEVICE</option><option value="cache">CACHE</option></select></label>
            <label>单批点数<input v-model.number="device.batchSize" type="number" min="1" max="10000" :disabled="readonly"/></label>
            <label>连接超时(ms)<input v-model.number="device.connectTimeoutMilliseconds" type="number" min="100" max="300000" :disabled="readonly"/></label>
            <label>请求超时(ms)<input v-model.number="device.requestTimeoutMilliseconds" type="number" min="100" max="300000" :disabled="readonly"/></label>
          </div>
        </section>

        <section class="opcda-config-section">
          <div class="opcda-config-heading"><div><strong>重连策略</strong><small>连接异常后的自动恢复参数</small></div></div>
          <div class="opcda-config-row opcda-reconnect-row">
            <label>自动重连<select v-model="device.autoReconnect" :disabled="readonly"><option :value="true">启用</option><option :value="false">停用</option></select></label>
            <label>重连间隔(s)<input v-model.number="device.reconnectIntervalSeconds" type="number" min="1" :disabled="readonly"/></label>
          </div>
        </section>
      </div>
      <div class="opcda-config-actions"><span>配置修改后点击页面上方“保存配置”生效。</span><div class="point-actions"><button @click="emit('test')">测试连接</button><button class="danger" :disabled="readonly" @click="emit('remove')">删除设备</button></div></div>
    </template>
    <div v-else class="device-head"><label>设备编号<input :value="device.deviceKey" readonly/></label><label>设备名称<input v-model.trim="device.name" :disabled="readonly"/></label><label>协议<input :value="device.protocol" readonly/></label>
    <template v-if="isForward"><label v-if="device.protocol.includes('modbus')">从站ID<input v-model.number="device.unitId" type="number" min="1" max="247" :disabled="readonly"/></label><label v-if="device.protocol.includes('iec104')">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="readonly"/></label><label v-if="device.protocol==='iec61850-mms-server'">IED 名称<input v-model.trim="device.iedName" :disabled="readonly"/></label><label v-if="device.protocol==='iec61850-mms-server'">逻辑设备<input v-model.trim="device.logicalDevice" :disabled="readonly"/></label><template v-if="isGooseForward"><label>网卡<input v-model.trim="device.interfaceName" :disabled="readonly"/></label><label>GoCBRef<input v-model.trim="device.goCbRef" class="mono" :disabled="readonly"/></label><label>DataSetRef<input v-model.trim="device.dataSetRef" class="mono" :disabled="readonly"/></label><label>APPID<input v-model.number="device.appId" type="number" min="1" max="65535" :disabled="readonly"/></label><label>目标 MAC<input v-model.trim="device.destinationMac" class="mono" :disabled="readonly"/></label><label>VLAN ID<input v-model.number="device.vlanId" type="number" min="0" max="4094" :disabled="readonly"/></label><label>VLAN 优先级<input v-model.number="device.vlanPriority" type="number" min="0" max="7" :disabled="readonly"/></label><label>允许存活时间(ms)<input v-model.number="device.timeAllowedToLiveMilliseconds" type="number" min="1" :disabled="readonly"/></label><label>ConfRev<input v-model.number="device.confRev" type="number" min="1" :disabled="readonly"/></label></template></template>
    <template v-else>
      <label v-if="device.protocol==='modbus-rtu'">串口<input v-model.trim="device.address" :disabled="readonly"/></label>
      <template v-else-if="splitAddress"><label>设备地址<input v-model.trim="host" :disabled="readonly"/></label><label>端口<input v-model="port" inputmode="numeric" :disabled="readonly"/></label></template>
      <label v-else>{{isGooseCollect?'网卡':isOPCDA?'OPC DA 服务器 IP':'连接地址'}}<input v-model.trim="device.address" :placeholder="isOPCDA?'例如 192.168.1.141':''" :disabled="readonly"/></label>
      <template v-if="isGooseCollect"><label>GoCBRef<input v-model.trim="device.goCbRef" class="mono" :disabled="readonly"/></label><label>DataSetRef<input v-model.trim="device.dataSetRef" class="mono" :disabled="readonly"/></label><label>APPID<input v-model.number="device.appId" type="number" min="1" max="65535" :disabled="readonly"/></label><label>目标 MAC<input v-model.trim="device.destinationMac" class="mono" :disabled="readonly"/></label><label>VLAN ID<input v-model.number="device.vlanId" type="number" min="0" max="4094" :disabled="readonly"/></label><label>VLAN 优先级<input v-model.number="device.vlanPriority" type="number" min="0" max="7" :disabled="readonly"/></label></template>
      <label v-if="device.protocol.startsWith('modbus')">从站ID<input v-model.number="device.slaveId" type="number" min="1" max="247" :disabled="readonly"/></label><label v-if="device.protocol==='iec104'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="readonly"/></label>
      <template v-if="isOPCDA">
        <label>ProgID<input v-model.trim="device.progId" placeholder="Matrikon.OPC.Simulation.1" :disabled="readonly"/></label>
        <label>CLSID<input v-model.trim="device.clsid" class="mono" placeholder="F8582CF2-88FB-11D0-B850-00C0F0104305" :disabled="readonly"/></label>
        <label>域 / 工作组<input v-model.trim="device.domain" placeholder="DESKTOP-XXXX" :disabled="readonly"/></label>
        <label>客户端工作站<input v-model.trim="device.workstation" placeholder="EG100" :disabled="readonly"/></label>
        <label>用户名<input v-model.trim="device.username" autocomplete="username" :disabled="readonly"/></label>
        <label>密码<input v-model="device.password" type="password" autocomplete="new-password" :disabled="readonly"/></label>
        <label>认证方式<select v-model="device.authentication" :disabled="readonly"><option value="ntlm">NTLMv2</option></select></label>
        <label>DCOM 安全模式<select v-model="device.securityMode" :disabled="readonly" @change="changeOPCDASecurityMode"><option value="packet_integrity">Packet Integrity（默认，签名校验，防篡改）</option><option value="packet_privacy">Packet Privacy（加密并签名，防窃听和篡改）</option><option value="legacy_connect">Legacy Connect（仅连接认证，兼容旧系统）</option></select></label>
        <div v-if="device.securityMode==='legacy_connect'" class="span-2 form-hint warning">Legacy Connect 仅用于旧版 DCOM 系统；Packet Integrity 不会自动降级到此模式。</div>
        <label>Group 名称<input v-model.trim="device.groupName" :disabled="readonly"/></label>
        <label>单批点数<input v-model.number="device.batchSize" type="number" min="1" max="10000" :disabled="readonly"/></label>
        <label>同步读取源<select v-model="device.readSource" :disabled="readonly || device.readMode==='subscription'"><option value="device">DEVICE</option><option value="cache">CACHE</option></select></label>
        <label>读取模式<select v-model="device.readMode" :disabled="readonly"><option value="sync">同步读取（IOPCSyncIO）</option><option value="subscription">异步订阅（IOPCDataCallback）</option></select></label>
        <label>连接超时(ms)<input v-model.number="device.connectTimeoutMilliseconds" type="number" min="100" max="300000" :disabled="readonly"/></label>
        <label>请求超时(ms)<input v-model.number="device.requestTimeoutMilliseconds" type="number" min="100" max="300000" :disabled="readonly"/></label>
        <label>Group 更新率(ms)<input v-model.number="device.requestedUpdateRateMilliseconds" type="number" min="10" :disabled="readonly"/></label>
        <label>Locale ID<input v-model.number="device.localeId" type="number" min="0" :disabled="readonly"/></label>
        <label>本地绑定 IP<input v-model.trim="device.localBindIp" placeholder="留空自动选择" :disabled="readonly"/></label>
        <label>自动重连<select v-model="device.autoReconnect" :disabled="readonly"><option :value="true">启用</option><option :value="false">停用</option></select></label>
      </template>
      <template v-else><label v-if="device.protocol==='opcua'">用户名<input v-model.trim="device.username" :disabled="readonly"/></label><label v-if="device.protocol==='opcua'">密码<input v-model="device.password" type="password" autocomplete="new-password" :disabled="readonly"/></label></template>
      <label v-if="device.protocol==='siemens-s7'">Rack<input v-model.number="device.rack" type="number" min="0" :disabled="readonly"/></label><label v-if="device.protocol==='siemens-s7'">Slot<input v-model.number="device.slot" type="number" min="0" :disabled="readonly"/></label><label>重连间隔(s)<input v-model.number="device.reconnectIntervalSeconds" type="number" min="1" :disabled="readonly"/></label>
    </template>
    <div class="point-actions"><button v-if="!isForward" @click="emit('test')">测试连接</button><button class="danger" :disabled="readonly" @click="emit('remove')">删除设备</button></div>
  </div></div>
</template>

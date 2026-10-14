<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { CircleCheck, CopyDocument, Download, Grid, Plus, Printer, RefreshRight, Search, Setting } from '@element-plus/icons-vue';
import { useRouter } from 'vue-router';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';
import { isAdminPortal } from '../portal';
import GatewayBindQRCode from '../components/GatewayBindQRCode.vue';
import { gatewayLabelSizes, printGatewayLabels } from '../utils/gateway-label-print';
import { parseGatewayIdentity } from '../utils/gateway-identity';
import { formatLicenseExpiry } from '../utils/license-expiry';
import { type ProductModel, productSpecification } from '../types/products';
import { getLocalPrintAgentSettings, loadLocalPrinters, printWithLocalAgent, saveLocalPrintAgentSettings, testLocalPrintAgent } from '../utils/local-print-agent';

interface FactoryGatewayItem {
  id: string;
  hardwareId: string;
  chipUsid?: string;
  sn: string;
  status: string;
  licenseSequence?: number;
  authorizationState?: string;
  authorizationReportFresh?: boolean;
  licenseActivatedAt?: string;
  licenseReportedAt?: string;
  issuedLicense?: LicenseMetadata;
  reportedLicense?: LicenseMetadata;
  product?: ProductModel;
  batchNo?: string;
  model?: string;
  producedAt?: string;
  currentVersion?: string;
  currentRelease?: string;
  versionConfirmedAt?: string;
  firstSeenAt?: string;
  lastSeenAt?: string;
  boundAt?: string;
  device?: {
    id: string;
    name: string;
    deviceKey: string;
    tenantId: string;
    status: string;
    tenant?: { id: string; name: string };
  };
}

interface LicenseMetadata {
  licenseSequence: number;
  expiresAt: string;
  issuedAt?: string;
  licenseVersion: number;
  keyId: string;
  features: string[];
}

interface ActivationFile {
  enabled: boolean;
  licenseVersion: number;
  keyId: string;
  licenseSequence: number;
  hardwareId: string;
  chipUsid: string;
  sn: string;
  deviceSecret: string;
  broker: string;
  cloudApi: string;
  remoteAccessEndpoint: string;
  issuedAt: string;
  expiresAt: string;
  features: string[];
  nonce: string;
  signature: string;
}

interface RegisterResult extends FactoryGatewayItem {
  deviceSecret: string;
  bindCode: string;
  activationFile: ActivationFile;
}

interface ResetBindCodeResult extends FactoryGatewayItem {
  bindCode: string;
}

interface ActivationResetResult extends FactoryGatewayItem {
  deviceSecret: string;
  activationFile: ActivationFile;
}

interface BindVerification {
  sn: string;
  batchNo?: string;
  producedAt?: string;
  firstSeenAt?: string;
  lastSeenAt?: string;
  online: boolean;
  suggestedName: string;
}

interface BindResult {
  device: {
    id: string;
    name: string;
    deviceKey: string;
    status: string;
  };
}

interface CloudPrintAgent {
  id: string;
  name: string;
  printerName?: string;
  online: boolean;
  lastSeenAt?: string;
  lastError?: string;
  token?: string;
}

const userStore = useUserStore();
const router = useRouter();
const loading = ref(false);
const detailsVisible = ref(false);
const detailsId = ref('');
const registering = ref(false);
const binding = ref(false);
const registerDialogVisible = ref(false);
const bindQrDialogVisible = ref(false);
const batchPrintDialogVisible = ref(false);
const printSettingsDialogVisible = ref(false);
const testingPrintAgent = ref(false);
const printerOptions = ref<string[]>([]);
const printAgentStatus = ref<'idle' | 'online' | 'offline'>('idle');
const cloudPrintAgents = ref<CloudPrintAgent[]>([]);
const creatingCloudAgent = ref(false);
const newCloudAgentName = ref('生产标签打印机');
const latestCloudAgentToken = ref('');
const printAgentForm = reactive(getLocalPrintAgentSettings());
const batchPrinting = ref(false);
const selectedGateways = ref<FactoryGatewayItem[]>([]);
const batchPrintForm = reactive({
  size: localStorage.getItem('gateway-label-size') === '50x30'
    ? '70x50'
    : (localStorage.getItem('gateway-label-size') || '70x50'),
  copies: Number(localStorage.getItem('gateway-label-copies') || 1),
});
const bindCredentialRotated = ref(false);
const resettingId = ref('');
const resettingActivationId = ref('');
const removingId = ref('');
const reclaimingId = ref('');
const gateways = ref<FactoryGatewayItem[]>([]);
const detailGateway = computed(() => gateways.value.find(row => row.id === detailsId.value));
const total = ref(0);
const latestResult = ref<RegisterResult | null>(null);
const latestResetResult = ref<ResetBindCodeResult | null>(null);
const latestActivationResetResult = ref<ActivationResetResult | null>(null);
const bindStep = ref(0);
const verifying = ref(false);
const verifiedGateway = ref<BindVerification | null>(null);
const boundDevice = ref<BindResult['device'] | null>(null);
const query = reactive({ page: 1, pageSize: 10, keyword: '' });
const registerForm = reactive({ hardwareId: '', chipUsid: '', model: 'EG100', batchNo: '' });
const identityText = ref('');
const identityError = ref('');
const identityRecognized = ref(false);
const products = ref<ProductModel[]>([]);
const loadingProducts = ref(false);
interface OtaPackageItem { id: string; model: string; release: string; version: string; createdAt: string; size: number; fileAvailable: boolean }
interface OtaTaskItem { id: string; status: string; message?: string; createdAt: string; completedAt?: string; package: OtaPackageItem }
interface MaintenanceTaskItem { id: string; action: 'SERVICE_RESTART' | 'GATEWAY_REBOOT'; status: string; message?: string; createdAt: string; completedAt?: string }
const otaVisible = ref(false);
const otaGatewayId = ref('');
const otaPackageId = ref('');
const otaPackages = ref<OtaPackageItem[]>([]);
const otaTasks = ref<OtaTaskItem[]>([]);
const otaSending = ref(false);
const otaReadiness = ref<{ enabled: boolean; reason: string }>({ enabled: false, reason: '正在检查 OTA 配置' });
let otaPoll: ReturnType<typeof setInterval> | undefined;
const maintenanceVisible = ref(false);
const maintenanceGatewayId = ref('');
const maintenanceTasks = ref<MaintenanceTaskItem[]>([]);
const maintenanceSending = ref<MaintenanceTaskItem['action'] | ''>('');
interface MaintenanceState { online:boolean; activated:boolean; authorizationState:string; authorizationReportFresh:boolean; channelReady:boolean; canMaintain:boolean; reason:string; lastSeenAt?:string; licenseActivatedAt?:string; serverTime:string }
const maintenanceState = ref<MaintenanceState>();
const maintenanceError = ref('');
let maintenanceGeneration = 0;
let maintenanceInFlight = -1;
let maintenancePoll: ReturnType<typeof setInterval> | undefined;
const selectedMaintenanceGateway = computed(() => gateways.value.find(item => item.id === maintenanceGatewayId.value));
const selectedOtaGateway = computed(() => gateways.value.find(item => item.id === otaGatewayId.value));
const selectedOtaPackage = computed(() => otaPackages.value.find(item => item.id === otaPackageId.value));
const otaInstalledRelease = computed(() => otaTasks.value.reduce<bigint | undefined>((latest, task) => {
  if (task.status !== 'SUCCEEDED' || !/^\d+$/.test(task.package?.release || '')) return latest;
  const release = BigInt(task.package.release);
  return latest === undefined || release > latest ? release : latest;
}, undefined));
function otaPackageBlocked(item: OtaPackageItem) {
  if (!item.fileAvailable) return true;
  if (selectedOtaGateway.value && item.model !== (selectedOtaGateway.value.model || 'EG100')) return true;
  if (otaInstalledRelease.value === undefined || !/^\d+$/.test(item.release)) return false;
  return BigInt(item.release) <= otaInstalledRelease.value;
}
const otaReleaseWarning = computed(() => {
  const pack = selectedOtaPackage.value;
  if (!selectedOtaGateway.value || !pack) return '';
  if (!pack.fileAvailable) return '升级包文件已清理，请到升级包管理重新上传同一签名包。';
  if (pack.model !== (selectedOtaGateway.value.model || 'EG100')) return '升级包型号与目标网关不匹配。';
  if (otaInstalledRelease.value !== undefined && /^\d+$/.test(pack.release) && BigInt(pack.release) <= otaInstalledRelease.value) {
    return `设备当前已安装发布序号 ${otaInstalledRelease.value.toString()}，请选择更高发布序号的升级包。`;
  }
  return '';
});
const otaCanSend = computed(() => otaReadiness.value.enabled && !!selectedOtaGateway.value && !!selectedOtaPackage.value && !otaReleaseWarning.value);
const activeOtaTask = computed(() => otaTasks.value.find(task => ['PENDING', 'SENT', 'INSTALLING'].includes(task.status)));
const otaProgress = computed(() => {
  const task = activeOtaTask.value;
  if (!task) return { percentage: 0, title: '', detail: '', step: 0 };
  if (task.status === 'PENDING') return { percentage: 10, title: '正在创建升级任务', detail: '平台正在准备设备专属升级指令。', step: 1 };
  if (task.status === 'SENT') return { percentage: 35, title: '升级任务已下发', detail: '等待网关下载并验证签名升级包。', step: 2 };
  return { percentage: 75, title: '网关正在安装', detail: '升级包已通过设备校验，正在备份旧程序、替换并检查新服务。', step: 3 };
});
const otaProgressSteps = ['创建任务', '远程下发', '校验与安装', '完成'];
const enabledProducts = computed(() => products.value.filter(product => product.enabled));
const selectedProduct = computed(() => enabledProducts.value.find(product => product.code === registerForm.model));
const bindForm = reactive({ sn: '', bindCode: '', name: '', location: '' });

const isSuperAdmin = computed(() => userStore.userInfo?.role === 'SUPER_ADMIN');
const activationText = computed(() => JSON.stringify(latestResult.value?.activationFile ?? {}, null, 2));
const resetActivationText = computed(() => JSON.stringify(latestActivationResetResult.value?.activationFile ?? {}, null, 2));
const automaticBatchExample = computed(() => {
  const parts = new Intl.DateTimeFormat('en', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date());
  const part = (type: string) => parts.find(item => item.type === type)?.value;
  return `${part('year')}${part('month')}${part('day')}-${registerForm.model.trim().toUpperCase() || 'EG100'}-001`;
});

function lifecycleText(row: FactoryGatewayItem) {
  if (row.status === 'BOUND') return '已绑定';
  if (row.status === 'DISABLED') return '已停用';
  return '未绑定';
}

function authorizationText(row:FactoryGatewayItem){
 const labels:Record<string,string>={WAITING_CONFIRMATION:'待设备确认',ACTIVE:'已激活',UPDATE_PENDING:'待更新确认',EXPIRED:'授权已过期',INVALID:'授权异常',DISABLED:'已停用'};
 return labels[row.authorizationState||'WAITING_CONFIRMATION']||'待设备确认';
}
function authorizationType(row:FactoryGatewayItem){return row.authorizationState==='ACTIVE'?'success':row.authorizationState==='EXPIRED'||row.authorizationState==='INVALID'?'danger':row.authorizationState==='UPDATE_PENDING'?'warning':'info'}
function authorizationTooltip(row:FactoryGatewayItem){
 if(!row.licenseReportedAt)return '授权文件已签发，尚未收到设备回执；设备离线或尚未导入均可能造成待确认。';
 return `首次激活：${formatTime(row.licenseActivatedAt)}；最近回执：${formatTime(row.licenseReportedAt)}；有效期：${formatLicenseExpiry(row.reportedLicense?.expiresAt)}。${row.authorizationReportFresh?'设备近期已确认授权状态。':'回执暂未刷新，以上为最后确认状态，不代表设备当前在线。'}`;
}

function lifecycleType(row: FactoryGatewayItem) {
  if (row.status === 'BOUND') return 'success';
  if (row.status === 'DISABLED') return 'danger';
  return row.firstSeenAt ? 'primary' : 'warning';
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '-';
}

function openDetails(row: FactoryGatewayItem) {
  detailsId.value = row.id;
  detailsVisible.value = true;
}

function featureText(feature: string) {
  const labels: Record<string, string> = {collection:'数据采集',forward:'数据转发',history:'历史存储','remote-storage':'远程数据库存储',edge:'边缘计算',alarms:'告警管理',cloud:'云平台连接','remote-access':'远程访问','remote-ssh':'远程 SSH'};
  return labels[feature] || feature;
}

function openTenant(row: FactoryGatewayItem) {
  const tenant = row.device?.tenant;
  if (!tenant) return;
  void router.push({ path: '/tenants', query: { tenantId: tenant.id, tenantName: tenant.name } });
}

async function loadGateways() {
  if (!isSuperAdmin.value) return;
  loading.value = true;
  try {
    const data = await apiRequest<{ items: FactoryGatewayItem[]; total: number }>({
      url: '/api/factory-gateways',
      method: 'GET',
      params: query,
    });
    gateways.value = data.items;
    total.value = data.total;
  } finally {
    loading.value = false;
  }
}

async function searchGateways() {
  query.page = 1;
  await loadGateways();
}

async function registerGateway() {
  if (loadingProducts.value || !selectedProduct.value) {
    ElMessage.warning('请先从产品库选择一个启用的型号');
    return;
  }
  if (identityError.value) {
    ElMessage.warning(identityError.value);
    return;
  }
  const model = registerForm.model.trim().toUpperCase() || 'EG100';
  if (!/^[A-Z0-9][A-Z0-9_-]{0,31}$/.test(model)) {
    ElMessage.warning('型号限 32 位字母、数字、横线或下划线，且以字母或数字开头');
    return;
  }
  const hardwareId = registerForm.hardwareId.trim().toUpperCase();
  const chipUsid = registerForm.chipUsid.trim().toUpperCase();
  if (!hardwareId) {
    ElMessage.warning('请先填写 HardwareId');
    return;
  }
  if (!/^[0-9A-F]{32}$/.test(hardwareId)) {
    ElMessage.warning('请填写网关设备激活页显示的完整 32 位 HardwareId');
    return;
  }
  if (!/^[0-9A-F]{16}$/.test(chipUsid)) {
    ElMessage.warning('请填写网关授权页显示的 16 位 RJGT102 芯片 USID');
    return;
  }
  registering.value = true;
  try {
    latestResult.value = await apiRequest<RegisterResult>({
      url: '/api/factory-gateways',
      method: 'POST',
      data: {
        hardwareId,
        chipUsid,
        model,
        batchNo: registerForm.batchNo.trim() || undefined,
      },
    });
    registerForm.hardwareId = '';
    registerForm.chipUsid = '';
    ElMessage.success('出厂网关登记成功，请保存一次性凭证');
    await loadGateways();
  } finally {
    registering.value = false;
  }
}

async function openRegisterDialog() {
  Object.assign(registerForm, { hardwareId: '', chipUsid: '', model: 'EG100', batchNo: '' });
  identityText.value = '';
  identityError.value = '';
  identityRecognized.value = false;
  latestResult.value = null;
  registerDialogVisible.value = true;
  loadingProducts.value = true;
  products.value = [];
  try {
    products.value = await apiRequest<ProductModel[]>({ url: '/api/products', method: 'GET' });
    if (!enabledProducts.value.some(product => product.code === registerForm.model)) {
      registerForm.model = enabledProducts.value[0]?.code || '';
    }
  } finally { loadingProducts.value = false; }
}

function fillDeviceIdentity(value: string) {
  identityText.value = value;
  identityError.value = '';
  identityRecognized.value = false;
  if (!value.trim()) return;
  try {
    const identity = parseGatewayIdentity(value);
    registerForm.hardwareId = identity.hardwareId;
    registerForm.chipUsid = identity.chipUsid;
    identityRecognized.value = true;
  } catch (error) {
    identityError.value = (error as Error).message;
  }
}

function pasteDeviceIdentity(event: ClipboardEvent) {
  const value = event.clipboardData?.getData('text') || '';
  // Single-field pastes still work normally; intercept the complete copied block
  // before the input's maxlength can truncate it.
  if (!/HardwareId\s*[:：]/i.test(value) && !/(?:芯片\s*USID|chipUsid)\s*[:：]/i.test(value)) return;
  event.preventDefault();
  fillDeviceIdentity(value);
}

function closeRegisterDialog() {
  registerDialogVisible.value = false;
  latestResult.value = null;
}

async function bindGateway() {
  if (!verifiedGateway.value) {
    ElMessage.warning('请先校验 SN 和绑定码');
    return;
  }
  await ElMessageBox.confirm(
    `确认将 ${verifiedGateway.value.sn} 绑定为“${bindForm.name.trim() || verifiedGateway.value.suggestedName}”吗？绑定后如需转移租户，必须由管理员处理。`,
    '确认绑定网关',
    { confirmButtonText: '确认绑定', cancelButtonText: '返回检查', type: 'warning' },
  );
  binding.value = true;
  try {
    const result = await apiRequest<BindResult>({
      url: '/api/factory-gateways/bind',
      method: 'POST',
      data: {
        sn: bindForm.sn.trim(),
        bindCode: bindForm.bindCode.trim(),
        name: bindForm.name.trim() || undefined,
        location: bindForm.location.trim() || undefined,
      },
    });
    boundDevice.value = result.device;
    bindStep.value = 2;
    ElMessage.success('网关绑定成功，已加入当前租户');
  } finally {
    binding.value = false;
  }
}

async function verifyGateway() {
  const sn = bindForm.sn.trim().toUpperCase();
  const bindCode = bindForm.bindCode.trim();
  if (!sn || !bindCode) {
    ElMessage.warning('请填写 SN 和 6 位绑定码');
    return;
  }
  if (!/^\d{6}$/.test(bindCode)) {
    ElMessage.warning('绑定码必须是 6 位数字');
    return;
  }
  verifying.value = true;
  try {
    const result = await apiRequest<BindVerification>({
      url: '/api/factory-gateways/bind/verify',
      method: 'POST',
      data: { sn, bindCode },
    });
    verifiedGateway.value = result;
    bindForm.sn = result.sn;
    if (!bindForm.name.trim()) bindForm.name = result.suggestedName;
    bindStep.value = 1;
  } finally {
    verifying.value = false;
  }
}

function invalidateVerification() {
  if (bindStep.value === 0) return;
  verifiedGateway.value = null;
  boundDevice.value = null;
  bindStep.value = 0;
}

function restartBinding() {
  Object.assign(bindForm, { sn: '', bindCode: '', name: '', location: '' });
  verifiedGateway.value = null;
  boundDevice.value = null;
  bindStep.value = 0;
}

async function viewBindQrCode(row: FactoryGatewayItem) {
  resettingId.value = row.id;
  try {
    latestResetResult.value = await apiRequest<ResetBindCodeResult>({
      url: `/api/factory-gateways/${row.id}/bind-credential`,
      method: 'GET',
      silentError: true,
    });
    bindCredentialRotated.value = false;
    bindQrDialogVisible.value = true;
  } catch (reason) {
    const error = reason as { response?: { status?: number; data?: { message?: string } } };
    if (error.response?.status !== 409) {
      ElMessage.error(error.response?.data?.message || '绑定二维码读取失败');
      return;
    }
    await ElMessageBox.confirm(
      '这是升级前生成的绑定码，平台只有哈希，无法恢复原二维码。是否生成一份可重复查看的新凭证？生成后旧二维码会失效。',
      '旧版凭证无法恢复',
      { confirmButtonText: '重置并查看', cancelButtonText: '保留旧凭证', type: 'warning' },
    );
    await resetBindCode(row, false);
  } finally {
    resettingId.value = '';
  }
}

function selectableForPrint(row: FactoryGatewayItem) {
  return row.status !== 'BOUND' && !row.device;
}

function handleGatewaySelection(rows: FactoryGatewayItem[]) {
  selectedGateways.value = rows;
}

function openBatchPrintDialog() {
  if (!selectedGateways.value.length) {
    ElMessage.warning('请先勾选需要打印的未绑定网关');
    return;
  }
  batchPrintDialogVisible.value = true;
}

async function loadBindCredential(row: FactoryGatewayItem) {
  return apiRequest<ResetBindCodeResult>({
    url: `/api/factory-gateways/${row.id}/bind-credential`,
    method: 'GET',
    silentError: true,
  });
}

async function quickPrint(row: FactoryGatewayItem) {
  try {
    const credential = await loadBindCredential(row);
    if (await printWithLocalAgent([credential], batchPrintForm.size, batchPrintForm.copies)) {
      ElMessage.success(`${row.sn} 标签已发送到本地打印机`);
      return;
    }
    const targetWindow = window.open('', '_blank', 'popup,width=720,height=560');
    if (!targetWindow) throw new Error('浏览器阻止了打印窗口，请允许本站弹出窗口后重试');
    await printGatewayLabels([credential], {
      size: batchPrintForm.size,
      copies: batchPrintForm.copies,
      targetWindow,
    });
  } catch (reason) {
    const error = reason as { response?: { data?: { message?: string } }; message?: string };
    ElMessage.error(error.response?.data?.message || error.message || '标签打印失败');
  }
}

async function confirmBatchPrint() {
  batchPrinting.value = true;
  localStorage.setItem('gateway-label-size', batchPrintForm.size);
  localStorage.setItem('gateway-label-copies', String(batchPrintForm.copies));
  const credentials: ResetBindCodeResult[] = [];
  const failed: string[] = [];
  for (const row of selectedGateways.value) {
    try {
      credentials.push(await loadBindCredential(row));
    } catch {
      failed.push(row.sn);
    }
  }
  try {
    if (!credentials.length) {
      ElMessage.error('所选网关均无法读取有效绑定凭证，请先重置旧版凭证');
      return;
    }
    if (await printWithLocalAgent(credentials, batchPrintForm.size, batchPrintForm.copies)) {
      batchPrintDialogVisible.value = false;
      if (failed.length) ElMessage.warning(`${credentials.length} 个标签已发送到本地打印机，${failed.length} 个凭证读取失败`);
      else ElMessage.success(`${credentials.length} 个网关标签已发送到本地打印机`);
      return;
    }
    const targetWindow = window.open('', '_blank', 'popup,width=720,height=560');
    if (!targetWindow) throw new Error('浏览器阻止了打印窗口，请允许本站弹出窗口后重试');
    await printGatewayLabels(credentials, {
      size: batchPrintForm.size,
      copies: batchPrintForm.copies,
      targetWindow,
    });
    batchPrintDialogVisible.value = false;
    if (failed.length) ElMessage.warning(`${credentials.length} 个标签已进入打印，${failed.length} 个凭证读取失败：${failed.join('、')}`);
    else ElMessage.success(`${credentials.length} 个网关标签已进入打印`);
  } finally {
    batchPrinting.value = false;
  }
}

function openPrintSettings() {
  Object.assign(printAgentForm, getLocalPrintAgentSettings());
  printAgentStatus.value = 'idle';
  printerOptions.value = [];
  printSettingsDialogVisible.value = true;
  void loadCloudPrintAgents().catch(() => undefined);
}

async function detectPrintAgent() {
  testingPrintAgent.value = true;
  try {
    const health = await testLocalPrintAgent(printAgentForm);
    printAgentStatus.value = 'online';
    const printers = await loadLocalPrinters(printAgentForm);
    printerOptions.value = printers.printers;
    if (!printAgentForm.printer) printAgentForm.printer = printers.defaultPrinter || health.defaultPrinter || printers.printers[0] || '';
    ElMessage.success(`打印服务已连接（v${health.version}）`);
  } catch (error) {
    printAgentStatus.value = 'offline';
    ElMessage.error(error instanceof Error ? error.message : '打印服务连接失败');
  } finally {
    testingPrintAgent.value = false;
  }
}

async function loadCloudPrintAgents() {
  cloudPrintAgents.value = await apiRequest<CloudPrintAgent[]>({ url: '/api/label-print/agents', method: 'GET' });
}

async function createCloudPrintAgent() {
  if (!newCloudAgentName.value.trim()) {
    ElMessage.warning('请填写打印终端名称');
    return;
  }
  creatingCloudAgent.value = true;
  try {
    const result = await apiRequest<CloudPrintAgent>({
      url: '/api/label-print/agents', method: 'POST',
      data: { name: newCloudAgentName.value.trim(), printerName: printAgentForm.printer || 'Godex G500 GZPL' },
    });
    latestCloudAgentToken.value = result.token || '';
    await loadCloudPrintAgents();
    ElMessage.success('云端打印终端已创建，请立即复制终端令牌');
  } finally {
    creatingCloudAgent.value = false;
  }
}

async function copyCloudAgentToken() {
  await navigator.clipboard.writeText(latestCloudAgentToken.value);
  ElMessage.success('终端令牌已复制');
}

async function removeCloudPrintAgent(agent: CloudPrintAgent) {
  await ElMessageBox.confirm(`确认移除打印终端“${agent.name}”吗？该终端将不能再领取 App 打印任务。`, '移除打印终端', {
    confirmButtonText: '确认移除', cancelButtonText: '取消', type: 'warning',
  });
  await apiRequest({ url: `/api/label-print/agents/${agent.id}`, method: 'DELETE' });
  await loadCloudPrintAgents();
  ElMessage.success('打印终端已移除');
}

function savePrintAgentSettings() {
  if (printAgentForm.enabled && !printAgentForm.pairingKey.trim()) {
    ElMessage.warning('请填写安装打印服务时生成的配对密钥');
    return;
  }
  saveLocalPrintAgentSettings({ ...printAgentForm });
  printSettingsDialogVisible.value = false;
  ElMessage.success(printAgentForm.enabled ? '本地静默打印已启用' : '已改用系统打印窗口');
}

async function resetBindCode(row: FactoryGatewayItem, confirm = true) {
  if (confirm) {
    await ElMessageBox.confirm(
      `确认重置 ${row.sn} 的绑定凭证吗？现有二维码和手工绑定码会立即失效，NFC 凭证不受影响。`,
      '重置二维码凭证',
      { confirmButtonText: '确认重置', cancelButtonText: '取消', type: 'warning' },
    );
  }
  resettingId.value = row.id;
  try {
    latestResetResult.value = await apiRequest<ResetBindCodeResult>({
      url: `/api/factory-gateways/${row.id}/bind-code/reset`,
      method: 'POST',
    });
    bindCredentialRotated.value = true;
    bindQrDialogVisible.value = true;
    ElMessage.success('新绑定二维码已生成，后续可直接重复查看');
    await loadGateways();
  } finally {
    resettingId.value = '';
  }
}

async function resetActivation(row: FactoryGatewayItem) {
  await ElMessageBox.confirm(
    `确认重新签发 ${row.sn} 的授权文件吗？保留现有设备密钥和云连接；导入新授权后，网关将拒绝旧授权。`,
    '重发激活文件',
    { confirmButtonText: '确认重新生成', cancelButtonText: '取消', type: 'warning' },
  );
  resettingActivationId.value = row.id;
  try {
    latestActivationResetResult.value = await apiRequest<ActivationResetResult>({
      url: `/api/factory-gateways/${row.id}/activation/reset`,
      method: 'POST',
    });
    ElMessage.success('新激活文件已生成，请立即下载并重新导入网关');
  } finally {
    resettingActivationId.value = '';
  }
}

async function backfillIdentity(row: FactoryGatewayItem) {
  const { value: chipUsid } = await ElMessageBox.prompt('请输入网关运行状态页显示的 RJGT102 芯片 USID；已登记身份不可更改。', '补录设备授权身份', {
    inputValue: row.chipUsid || '', inputPattern: /^[0-9A-Fa-f]{16}$/, inputErrorMessage: '请输入 16 位十六进制 USID',
  });
  const { value: deviceSecret } = await ElMessageBox.prompt('填写原激活文件中的 deviceSecret，用于安全重发授权；留空只补录芯片身份。不会修改设备密钥。', '恢复旧设备授权凭据', { inputType: 'password' });
  await apiRequest({ url: `/api/factory-gateways/${row.id}/chip-identity`, method: 'POST', data: { hardwareId: row.hardwareId, chipUsid: chipUsid.toUpperCase(), ...(deviceSecret ? { deviceSecret } : {}) } });
  ElMessage.success('设备授权身份已补录');
  await loadGateways();
}

async function removeFactoryGateway(row: FactoryGatewayItem) {
  await ElMessageBox.confirm(
    `确认移除出厂网关 ${row.sn} 吗？系统会保留 HardwareId 与 SN 档案，旧 NFC 凭证和设备密钥将失效；再次登记相同 HardwareId 时仍恢复原 SN。`,
    '移除未绑定网关',
    {
      confirmButtonText: '确认移除',
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: 'el-button--danger',
    },
  );
  removingId.value = row.id;
  try {
    const result = await apiRequest<{ message: string }>({
      url: `/api/factory-gateways/${row.id}`,
      method: 'DELETE',
    });
    ElMessage.success(result.message || '出厂网关已移除');
    if (gateways.value.length === 1 && query.page > 1) query.page -= 1;
    await loadGateways();
  } finally {
    removingId.value = '';
  }
}

async function reclaimFactoryGateway(row: FactoryGatewayItem) {
  await ElMessageBox.confirm(
    `确认解除 ${row.sn} 的租户绑定并回收网关吗？对应的云端网关设备及其关联配置、事件将被删除，出厂档案和 NFC 凭证保留，之后可重新绑定。下属子设备将解除网关关联并标记离线，子设备档案和已有数据保留在原租户，不会转移给新租户。`,
    '解除绑定并回收',
    {
      confirmButtonText: '确认解除绑定',
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: 'el-button--danger',
    },
  );
  reclaimingId.value = row.id;
  try {
    const result = await apiRequest<{ message: string }>({
      url: `/api/factory-gateways/${row.id}/reclaim`,
      method: 'POST',
    });
    ElMessage.success(result.message || '网关已解除绑定并恢复为待绑定');
    await loadGateways();
  } finally {
    reclaimingId.value = '';
  }
}

function moreActionLoading(row: FactoryGatewayItem) {
  return resettingId.value === row.id
    || resettingActivationId.value === row.id
    || reclaimingId.value === row.id
    || removingId.value === row.id;
}

async function loadOtaData() {
  if (!otaVisible.value) return;
  otaReadiness.value = await apiRequest<{ enabled: boolean; reason: string }>({ url: '/api/ota/readiness', method: 'GET' });
  const packages = await apiRequest<OtaPackageItem[]>({ url: '/api/ota/packages', method: 'GET' });
  otaPackages.value = packages.filter(item => item.fileAvailable);
  if (otaPackageId.value && !otaPackages.value.some(item => item.id === otaPackageId.value)) {
    otaPackageId.value = '';
  }
  if (otaGatewayId.value) otaTasks.value = await apiRequest<OtaTaskItem[]>({ url: `/api/ota/gateways/${otaGatewayId.value}/tasks`, method: 'GET' });
  else otaTasks.value = [];
}

async function openOta(row?: FactoryGatewayItem) {
  otaGatewayId.value = row?.id || '';
  otaTasks.value = [];
  otaVisible.value = true;
  await loadOtaData();
  if (otaPoll) clearInterval(otaPoll);
  otaPoll = setInterval(() => { void loadOtaData().catch(() => undefined); }, 2000);
}

async function handleOtaGatewayChange() {
  otaTasks.value = [];
  await loadOtaData();
}

function closeOta() { otaVisible.value = false; if (otaPoll) clearInterval(otaPoll); otaPoll = undefined; }

async function loadMaintenanceTasks() {
  if (!maintenanceVisible.value || !maintenanceGatewayId.value) return;
  const generation = maintenanceGeneration, id = maintenanceGatewayId.value;
  if (maintenanceInFlight === generation) return;
  maintenanceInFlight = generation;
  try {
    const [state, tasks] = await Promise.all([
      apiRequest<MaintenanceState>({ url: `/api/gateway-maintenance/gateways/${id}/status`, method:'GET', timeout:10000 }),
      apiRequest<MaintenanceTaskItem[]>({ url: `/api/gateway-maintenance/gateways/${id}/tasks`, method:'GET', timeout:10000 }),
    ]);
    if (!maintenanceVisible.value || generation !== maintenanceGeneration || id !== maintenanceGatewayId.value) return;
    maintenanceState.value = state; maintenanceTasks.value = tasks; maintenanceError.value = '';
    const row = gateways.value.find(item => item.id === id);
    if (row) Object.assign(row, { lastSeenAt:state.lastSeenAt, licenseActivatedAt:state.licenseActivatedAt, authorizationState:state.authorizationState, authorizationReportFresh:state.authorizationReportFresh });
  } catch {
    if (generation === maintenanceGeneration) { maintenanceState.value = undefined; maintenanceError.value = '状态获取失败，暂不能执行维护操作，正在重试'; }
  } finally { if (maintenanceInFlight === generation) maintenanceInFlight = -1; }
}

async function openMaintenance(row: FactoryGatewayItem) {
  closeMaintenance();
  maintenanceGeneration++;
  const generation = maintenanceGeneration;
  maintenanceGatewayId.value = row.id;
  maintenanceState.value = undefined; maintenanceError.value = '';
  maintenanceTasks.value = [];
  maintenanceVisible.value = true;
  await loadMaintenanceTasks();
  if (!maintenanceVisible.value || maintenanceGatewayId.value !== row.id || generation !== maintenanceGeneration) return;
  if (maintenancePoll) clearInterval(maintenancePoll);
  maintenancePoll = setInterval(() => { void loadMaintenanceTasks(); }, 5000);
}

function closeMaintenance() {
  maintenanceGeneration++;
  maintenanceState.value = undefined; maintenanceError.value = '';
  maintenanceVisible.value = false;
  maintenanceGatewayId.value = '';
  if (maintenancePoll) clearInterval(maintenancePoll);
  maintenancePoll = undefined;
}

async function issueMaintenance(action: MaintenanceTaskItem['action']) {
  const gateway = selectedMaintenanceGateway.value;
  if (!gateway) return;
  if (!maintenanceState.value?.canMaintain) { ElMessage.warning(maintenanceState.value?.reason || '正在确认网关状态，请稍后重试'); return; }
  const service = action === 'SERVICE_RESTART';
  await ElMessageBox.confirm(
    service
      ? `确认重启 ${gateway.sn} 的网关服务吗？采集、转发和云连接会短暂中断，设备系统不会重启。`
      : `确认重启整台网关 ${gateway.sn} 吗？所有采集和网络连接将中断，设备重新上线前请勿断电。`,
    service ? '确认重启网关服务' : '确认重启整台网关',
    { type: 'warning', confirmButtonText: service ? '重启服务' : '重启网关', cancelButtonText: '取消', confirmButtonClass: service ? '' : 'el-button--danger' },
  );
  maintenanceSending.value = action;
  try {
    await apiRequest({ url: `/api/gateway-maintenance/gateways/${gateway.id}/tasks`, method: 'POST', data: { action } });
    ElMessage.success('远程维护指令已下发，正在等待网关受理');
    await loadMaintenanceTasks();
  } finally { maintenanceSending.value = ''; }
}

function maintenanceActionText(action: string) { return action === 'SERVICE_RESTART' ? '重启服务' : '重启网关'; }
function maintenanceStatusText(status: string) { return ({ PENDING: '待发送', SENT: '等待受理', ACCEPTED: '已受理', FAILED: '失败', TIMEOUT: '未确认' } as Record<string, string>)[status] || status; }
function maintenanceStatusType(status: string) { return status === 'ACCEPTED' ? 'success' : status === 'FAILED' ? 'danger' : status === 'TIMEOUT' ? 'warning' : 'info'; }

onUnmounted(() => { closeOta(); closeMaintenance(); });

async function issueOta() {
  const gateway = selectedOtaGateway.value;
  const pack = selectedOtaPackage.value;
  if (!gateway || !pack) { ElMessage.warning('请选择网关和升级包'); return; }
  if (otaReleaseWarning.value) { ElMessage.warning(otaReleaseWarning.value); return; }
  await ElMessageBox.confirm(`将 ${pack.version} 下发到 ${gateway.sn}。设备安装期间会短暂断线，失败时会尝试恢复旧版本。确认继续？`, '确认应用升级', { type: 'warning', confirmButtonText: '开始升级', cancelButtonText: '取消' });
  otaSending.value = true;
  try {
    await apiRequest({ url: `/api/ota/gateways/${gateway.id}/tasks`, method: 'POST', data: { packageId: pack.id } });
    ElMessage.success('升级任务已下发');
    await loadOtaData();
  } finally { otaSending.value = false; }
}

function otaStatus(status: string) {
  return ({ PENDING: '待发送', SENT: '已下发', INSTALLING: '安装中', SUCCEEDED: '成功', FAILED: '失败', ROLLED_BACK: '已回退', TIMEOUT: '待人工确认' } as Record<string, string>)[status] || status;
}

async function handleMoreCommand(command: string, row: FactoryGatewayItem) {
  if (command === 'reset-bind') await resetBindCode(row);
  else if (command === 'reset-activation') await resetActivation(row);
  else if (command === 'backfill-identity') await backfillIdentity(row);
  else if (command === 'reclaim') await reclaimFactoryGateway(row);
  else if (command === 'remove') await removeFactoryGateway(row);
  else if (command === 'ota') await openOta(row);
  else if (command === 'maintenance') await openMaintenance(row);
}

async function copyText(text: string, label: string) {
  await navigator.clipboard.writeText(text);
  ElMessage.success(`${label} 已复制`);
}

async function copyResetHandover() {
  if (!latestResetResult.value) return;
  await copyText(`网关 SN：${latestResetResult.value.sn}\n绑定码：${latestResetResult.value.bindCode}`, '新客户交付信息');
}

function downloadActivationFile() {
  if (!latestResult.value) return;
  const blob = new Blob([activationText.value], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `activation-${latestResult.value.sn}.lic`;
  link.click();
  URL.revokeObjectURL(url);
}

async function copyHandoverInfo() {
  if (!latestResult.value) return;
  await copyText(`网关 SN：${latestResult.value.sn}\n绑定码：${latestResult.value.bindCode}`, '客户交付信息');
}

function downloadResetActivationFile() {
  if (!latestActivationResetResult.value) return;
  const blob = new Blob([resetActivationText.value], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `activation-${latestActivationResetResult.value.sn}.lic`;
  link.click();
  URL.revokeObjectURL(url);
}

onMounted(loadGateways);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">{{ isAdminPortal ? '出厂网关' : '网关绑定' }}</h1>
        <p class="page-description">
          {{ isAdminPortal
            ? '管理 HardwareId、SN、DeviceSecret 和绑定码，让网关出厂后可以自动接入云平台。'
            : '使用网关铭牌或交付单上的 SN 与绑定码，将网关安全绑定到当前租户。' }}
        </p>
      </div>
      <div v-if="isSuperAdmin" class="page-action-buttons">
        <el-button :icon="RefreshRight" @click="loadGateways">刷新</el-button>
        <el-button @click="openOta()">应用升级</el-button>
        <el-button type="primary" :icon="Plus" @click="openRegisterDialog">登记出厂网关</el-button>
      </div>
    </div>

    <template v-if="!isAdminPortal">
      <section class="table-panel bind-panel">
        <el-steps :active="bindStep" simple finish-status="success" class="bind-steps">
          <el-step title="校验交付凭证" />
          <el-step title="确认设备信息" />
          <el-step title="完成绑定" />
        </el-steps>

        <div v-if="bindStep === 0" class="bind-stage">
          <div class="stage-heading">
            <h2>输入交付凭证</h2>
            <p>SN 和绑定码位于网关铭牌或设备交付单，校验不会改变现有配置。</p>
          </div>
          <el-form label-position="top" class="bind-form" @keyup.enter="verifyGateway">
            <el-form-item label="网关 SN">
              <el-input v-model="bindForm.sn" maxlength="32" placeholder="例如 WK2026A1B2C3D4" @input="invalidateVerification" />
            </el-form-item>
            <el-form-item label="6 位绑定码">
              <el-input v-model="bindForm.bindCode" maxlength="6" inputmode="numeric" show-password placeholder="请输入 6 位数字" @input="invalidateVerification" />
            </el-form-item>
            <el-button type="primary" :loading="verifying" @click="verifyGateway">校验并继续</el-button>
          </el-form>
          <p class="bind-help">提示：连续校验失败时，请检查是否使用了其他网关的绑定码，或联系设备供应方重置绑定码。</p>
        </div>

        <div v-else-if="bindStep === 1 && verifiedGateway" class="bind-stage">
          <div class="verified-card">
            <div class="verified-icon"><el-icon><CircleCheck /></el-icon></div>
            <div>
              <strong>交付凭证校验通过</strong>
              <p>{{ verifiedGateway.sn }}</p>
            </div>
            <el-tag :type="verifiedGateway.online ? 'success' : 'info'">{{ verifiedGateway.online ? '网关在线' : '当前离线' }}</el-tag>
          </div>
          <div class="gateway-summary">
            <div><span>生产批次</span><strong>{{ verifiedGateway.batchNo || '未记录' }}</strong></div>
            <div><span>出厂时间</span><strong>{{ formatTime(verifiedGateway.producedAt) }}</strong></div>
            <div><span>最近上线</span><strong>{{ formatTime(verifiedGateway.lastSeenAt) }}</strong></div>
          </div>
          <el-alert v-if="!verifiedGateway.online" type="info" :closable="false" title="网关当前离线也可以绑定，上线后会自动出现在设备状态中。" />
          <el-form label-position="top" class="bind-form device-form">
            <el-form-item label="网关名称">
              <el-input v-model="bindForm.name" maxlength="64" placeholder="例如 1 号泵站网关" />
            </el-form-item>
            <el-form-item label="安装位置">
              <el-input v-model="bindForm.location" maxlength="128" placeholder="例如 北区 1 号泵房，可选" />
            </el-form-item>
            <div class="form-actions">
              <el-button @click="invalidateVerification">返回修改凭证</el-button>
              <el-button type="primary" :loading="binding" @click="bindGateway">确认绑定到当前租户</el-button>
            </div>
          </el-form>
        </div>

        <div v-else class="bind-stage bind-success">
          <el-icon class="success-icon"><CircleCheck /></el-icon>
          <h2>网关绑定成功</h2>
          <p>{{ boundDevice?.name }} 已加入当前租户，后续设备数据和状态只对本租户可见。</p>
          <div class="success-actions">
            <el-button @click="restartBinding">继续绑定网关</el-button>
            <el-button type="primary" @click="boundDevice && router.push(`/devices/${boundDevice.id}`)">查看网关详情</el-button>
          </div>
        </div>
      </section>
    </template>

    <el-dialog
      v-model="registerDialogVisible"
      :title="latestResult ? '出厂网关登记成功' : '登记出厂网关'"
      width="min(960px, 94vw)"
      :close-on-click-modal="!registering"
      :close-on-press-escape="!registering"
      class="factory-register-dialog"
      @closed="latestResult = null"
    >
      <template v-if="!latestResult">
        <p class="dialog-description">在网关待激活页面点击“复制设备身份”，将完整内容粘贴到下方即可自动填入，无需逐项复制。系统将生成只对该实体网关有效的签名授权文件。</p>
        <el-form label-position="top" class="register-dialog-form">
          <el-form-item label="粘贴完整设备信息" :error="identityError">
            <el-input
              :model-value="identityText"
              type="textarea"
              :rows="3"
              aria-label="粘贴完整设备信息"
              placeholder="将网关“复制设备身份”的完整内容一次粘贴到这里"
              @input="fillDeviceIdentity"
            />
            <span class="identity-paste-hint" :class="{ recognized: identityRecognized }" role="status">
              {{ identityRecognized ? '已识别设备身份，HardwareId 和芯片 USID 已自动填入，请核对后登记。' : '也可在下面的身份输入框中直接粘贴完整内容，或手动填写。' }}
            </span>
          </el-form-item>
          <el-form-item label="HardwareId（32 位硬件指纹）">
            <el-input
              v-model="registerForm.hardwareId"
              maxlength="32"
              show-word-limit
              placeholder="扫码或粘贴网关显示的 HardwareId"
              @input="registerForm.hardwareId = registerForm.hardwareId.toUpperCase()"
              @paste="pasteDeviceIdentity"
              @keyup.enter="registerGateway"
            />
          </el-form-item>
          <el-form-item label="RJGT102 芯片 USID（16 位）">
            <el-input
              v-model="registerForm.chipUsid"
              maxlength="16"
              show-word-limit
              placeholder="粘贴网关授权页显示的芯片 USID"
              @input="registerForm.chipUsid = registerForm.chipUsid.toUpperCase()"
              @paste="pasteDeviceIdentity"
              @keyup.enter="registerGateway"
            />
          </el-form-item>
          <el-form-item label="网关型号">
            <el-select v-model="registerForm.model" :loading="loadingProducts" :disabled="loadingProducts" aria-label="网关型号" placeholder="请选择产品库中的型号" filterable style="width: 100%">
              <el-option v-for="product in enabledProducts" :key="product.code" :value="product.code" :label="`${product.code} · ${product.name}`" />
            </el-select>
            <span v-if="selectedProduct" class="identity-paste-hint">{{ productSpecification(selectedProduct) }}</span>
            <span v-else-if="!loadingProducts" class="identity-paste-hint">暂无可登记型号，请先在<router-link to="/products" @click="closeRegisterDialog">产品库</router-link>新增或启用产品。</span>
          </el-form-item>
          <el-form-item label="生产批次">
            <el-input v-model="registerForm.batchNo" maxlength="64" :placeholder="`留空自动生成，例如 ${automaticBatchExample}`" />
            <span class="identity-paste-hint">默认：北京时间当天日期＋型号＋递增序号。同一天、同型号连续编号，登记成功时分配；手动填写则使用指定批次。</span>
          </el-form-item>
        </el-form>
      </template>
      <template v-else>
        <el-alert type="success" :closable="false" show-icon :title="`登记成功，生产批次：${latestResult.batchNo || '未记录'}。请在关闭弹窗前下载并保存交付资料。`" />
        <div class="handover-grid dialog-handover-grid">
          <div class="handover-card activation-handover">
            <span class="step-badge">给网关</span>
            <h3>激活文件</h3>
            <p>下载后导入对应实体网关，完成设备激活。</p>
            <div class="activation-identity"><span>HardwareId</span><code>{{ latestResult.hardwareId }}</code></div>
            <el-button type="primary" :icon="Download" @click="downloadActivationFile">下载授权文件（.lic）</el-button>
            <el-collapse class="activation-collapse">
              <el-collapse-item title="查看文件内容"><pre>{{ activationText }}</pre></el-collapse-item>
            </el-collapse>
          </div>
          <div class="handover-card customer-handover">
            <span class="step-badge">给客户</span>
            <h3>租户绑定凭证</h3>
            <div class="credential-line"><span>SN</span><code>{{ latestResult.sn }}</code></div>
            <div class="credential-line"><span>绑定码</span><code>{{ latestResult.bindCode }}</code></div>
            <el-button :icon="CopyDocument" @click="copyHandoverInfo">复制客户交付信息</el-button>
          </div>
        </div>
        <div class="handover-label-section">
          <GatewayBindQRCode class="handover-qr" :sn="latestResult.sn" :bind-code="latestResult.bindCode" />
        </div>
      </template>
      <template #footer>
        <el-button v-if="!latestResult" :disabled="registering" @click="closeRegisterDialog">取消</el-button>
        <el-button v-if="!latestResult" type="primary" :loading="registering" :disabled="loadingProducts || !selectedProduct" @click="registerGateway">登记并生成交付资料</el-button>
        <el-button v-else type="primary" @click="closeRegisterDialog">资料已保存，关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="bindQrDialogVisible"
      title="网关绑定二维码"
      width="min(860px, 94vw)"
      class="gateway-qr-dialog"
      @closed="latestResetResult = null"
    >
      <template v-if="latestResetResult">
        <el-alert
          :type="bindCredentialRotated ? 'warning' : 'success'"
          :closable="false"
          show-icon
          :title="bindCredentialRotated
            ? '绑定凭证已重置，旧二维码已失效；本二维码以后可以重复查看。'
            : '当前二维码使用已持久化凭证，关闭后仍可再次查看，不会自动失效。'"
        />
        <div class="qr-credential-summary">
          <div><span>网关 SN</span><code>{{ latestResetResult.sn }}</code></div>
          <div><span>绑定码</span><code>{{ latestResetResult.bindCode }}</code></div>
        </div>
        <GatewayBindQRCode :sn="latestResetResult.sn" :bind-code="latestResetResult.bindCode" />
      </template>
      <template #footer>
        <el-button :icon="CopyDocument" @click="copyResetHandover">复制交付信息</el-button>
        <el-button type="primary" @click="bindQrDialogVisible = false">完成</el-button>
      </template>
    </el-dialog>

    <section v-if="latestActivationResetResult" class="table-panel credential-panel">
      <div class="section-title">
        <h2>新激活文件</h2>
        <span>DeviceSecret 已轮换，旧激活文件已失效，请立即下载并重新导入网关</span>
      </div>
      <div class="activation-box">
        <div class="activation-header">
          <strong>activation-{{ latestActivationResetResult.sn }}.lic</strong>
          <el-button type="primary" :icon="Download" @click="downloadResetActivationFile">下载新激活文件</el-button>
        </div>
        <pre>{{ resetActivationText }}</pre>
      </div>
    </section>

    <section v-if="isSuperAdmin" class="table-panel">
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索 SN、HardwareId、USID 或批次" style="max-width: 320px" @keyup.enter="searchGateways">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button @click="searchGateways">查询</el-button>
        <div class="table-toolbar-spacer" />
        <span class="selection-summary">已选 {{ selectedGateways.length }} 项</span>
        <el-button type="primary" plain :icon="Printer" :disabled="!selectedGateways.length" @click="openBatchPrintDialog">批量打印</el-button>
        <el-button :icon="Setting" @click="openPrintSettings">打印设置</el-button>
      </div>

      <el-table class="factory-gateway-table" v-loading="loading" :data="gateways" scrollbar-always-on empty-text="暂无出厂网关" @selection-change="handleGatewaySelection">
        <el-table-column type="selection" width="46" :selectable="selectableForPrint" />
        <el-table-column prop="sn" label="SN" min-width="185" show-overflow-tooltip>
          <template #default="{ row }"><el-button link type="primary" @click="openDetails(row)">{{ row.sn }}</el-button></template>
        </el-table-column>
        <el-table-column prop="model" label="产品型号" min-width="105" show-overflow-tooltip />
        <el-table-column label="当前版本" min-width="115">
          <template #default="{ row }">
            <el-tooltip v-if="row.currentVersion" :content="`发布序号 ${row.currentRelease || '未记录'} · 确认于 ${formatTime(row.versionConfirmedAt)}`" placement="top">
              <el-tag type="primary" effect="plain">{{ row.currentVersion }}</el-tag>
            </el-tooltip>
            <span v-else class="muted">未上报</span>
          </template>
        </el-table-column>
        <el-table-column label="绑定状态" width="110">
          <template #default="{ row }"><el-tag :type="lifecycleType(row)">{{ lifecycleText(row) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="授权状态" min-width="130">
          <template #default="{ row }">
            <el-tooltip :content="authorizationTooltip(row)" placement="top">
              <div>
                <el-tag :type="authorizationType(row)">{{ authorizationText(row) }}</el-tag>
              </div>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="归属租户" min-width="130">
          <template #default="{ row }">
            <el-button v-if="row.device?.tenant" link type="primary" class="tenant-link" @click="openTenant(row)">
              {{ row.device.tenant.name }}
            </el-button>
            <span v-else class="muted">未绑定</span>
          </template>
        </el-table-column>
        <el-table-column label="最近心跳" min-width="180">
          <template #default="{ row }">{{ formatTime(row.lastSeenAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <div class="gateway-operation-group">
              <el-button text type="primary" :icon="Printer" :disabled="!selectableForPrint(row)" @click="quickPrint(row)">打印</el-button>
              <el-button
                text
                type="primary"
                :icon="Grid"
                :disabled="row.status === 'BOUND'"
                :loading="resettingId === row.id"
                @click="viewBindQrCode(row)"
              >
                查看二维码
              </el-button>
              <el-dropdown
                trigger="click"
                :disabled="moreActionLoading(row)"
                @command="handleMoreCommand($event, row)"
              >
                <el-button text :loading="moreActionLoading(row)">更多</el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="reset-bind" :disabled="row.status === 'BOUND'">重置凭证</el-dropdown-item>
                    <el-dropdown-item command="backfill-identity">补录授权身份 / 凭据</el-dropdown-item>
                    <el-dropdown-item command="reset-activation">重发授权文件</el-dropdown-item>
                    <el-dropdown-item command="ota">应用升级</el-dropdown-item>
                    <el-dropdown-item command="maintenance">远程维护</el-dropdown-item>
                    <el-dropdown-item
                      :command="row.status === 'BOUND' || row.device ? 'reclaim' : 'remove'"
                      divided
                      class="danger-dropdown-item"
                    >
                      {{ row.status === 'BOUND' || row.device ? '解除绑定并回收' : '移除登记' }}
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadGateways" />
      </div>
    </section>

    <el-dialog v-if="isSuperAdmin" v-model="otaVisible" title="EG100 应用升级" width="min(700px, 94vw)" @closed="closeOta">
      <p class="factory-detail-note">仅升级网关程序和内置管理页面；配置、授权及历史数据保留。网关会再次验证签名，升级失败自动回退。</p>
      <el-alert v-if="!otaReadiness.enabled" :title="otaReadiness.reason" type="warning" :closable="false" />
      <div class="ota-fields">
        <label>目标网关<el-select v-model="otaGatewayId" placeholder="选择在线网关" @change="handleOtaGatewayChange"><el-option v-for="item in gateways" :key="item.id" :label="`${item.sn} · ${item.model || 'EG100'}`" :value="item.id" /></el-select></label>
        <label>升级包<el-select v-model="otaPackageId" placeholder="选择已验证升级包" no-data-text="暂无可下发升级包"><el-option v-for="item in otaPackages" :key="item.id" :label="`${item.model} ${item.version} · 发布序号 ${item.release}`" :value="item.id" :disabled="otaPackageBlocked(item)" /></el-select></label>
      </div>
      <el-alert v-if="otaReleaseWarning" :title="otaReleaseWarning" type="warning" :closable="false" show-icon />
      <div class="ota-actions"><el-button @click="router.push('/ota-packages')">升级包管理</el-button><el-button type="primary" :loading="otaSending" :disabled="!otaCanSend" @click="issueOta">远程下发</el-button></div>
      <section v-if="activeOtaTask" class="ota-progress-card" aria-live="polite">
        <div class="ota-progress-heading"><div><strong>{{ otaProgress.title }}</strong><span>{{ activeOtaTask.package?.version }} · 发布序号 {{ activeOtaTask.package?.release }}</span></div><em>{{ otaProgress.percentage }}%</em></div>
        <el-progress :percentage="otaProgress.percentage" :show-text="false" :stroke-width="10" :indeterminate="activeOtaTask.status === 'INSTALLING'" :duration="2" />
        <div class="ota-progress-steps"><span v-for="(step, index) in otaProgressSteps" :key="step" :class="{ done: index + 1 < otaProgress.step, active: index + 1 === otaProgress.step }"><i>{{ index + 1 }}</i>{{ step }}</span></div>
        <p>{{ otaProgress.detail }}升级期间设备可能短暂离线，请勿断电或重复下发。</p>
      </section>
      <h3 v-if="otaGatewayId">最近升级任务</h3>
      <el-table v-if="otaGatewayId" :data="otaTasks" size="small" empty-text="暂无升级记录"><el-table-column label="版本" min-width="120"><template #default="{ row }">{{ row.package?.version }}</template></el-table-column><el-table-column label="状态" width="110"><template #default="{ row }"><el-tag :type="row.status === 'SUCCEEDED' ? 'success' : ['FAILED', 'ROLLED_BACK', 'TIMEOUT'].includes(row.status) ? 'danger' : 'warning'">{{ otaStatus(row.status) }}</el-tag></template></el-table-column><el-table-column label="发起时间" min-width="160"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column><el-table-column prop="message" label="结果" min-width="230" show-overflow-tooltip /></el-table>
    </el-dialog>

    <el-dialog v-if="isSuperAdmin" v-model="maintenanceVisible" title="远程维护" width="min(680px, 94vw)" @closed="closeMaintenance">
      <template v-if="selectedMaintenanceGateway">
        <div class="maintenance-target">
          <div><strong>{{ selectedMaintenanceGateway.sn }}</strong><span>{{ selectedMaintenanceGateway.model || 'EG100' }}</span></div>
          <div style="display:flex;gap:8px;flex-wrap:wrap"><el-tag :type="!maintenanceState ? 'info' : maintenanceState.online ? 'success' : 'danger'">{{ !maintenanceState ? (maintenanceError ? '状态未知' : '正在检查') : maintenanceState.online ? '连接在线' : '连接离线' }}</el-tag><el-tag :type="maintenanceState?.activated ? 'success' : 'info'">{{ maintenanceState ? (maintenanceState.activated ? '授权已激活' : '授权未激活') : '授权待检查' }}</el-tag></div>
        </div>
        <el-alert type="warning" :closable="false" show-icon title="重启会中断采集、转发和云连接，请避开关键生产时段操作。" />
        <p style="font-size:12px;color:#7f8da2" role="status">最新心跳：{{ formatTime(maintenanceState?.lastSeenAt) }} · 每5秒刷新 <span v-if="maintenanceError"> · {{ maintenanceError }}</span><span v-else-if="maintenanceState?.reason"> · {{ maintenanceState.reason }}</span></p>
        <div class="maintenance-actions">
          <section>
            <div><strong>重启网关服务</strong><p>仅重新启动采集与管理程序，网络配置和设备系统保持运行，恢复速度较快。</p></div>
            <el-button type="primary" :loading="maintenanceSending === 'SERVICE_RESTART'" :disabled="!!maintenanceSending || !maintenanceState?.canMaintain" @click="issueMaintenance('SERVICE_RESTART')">重启服务</el-button>
          </section>
          <section>
            <div><strong>重启整台网关</strong><p>重新启动 EG100 操作系统，网络和全部业务会暂时中断，恢复时间更长。</p></div>
            <el-button type="danger" :loading="maintenanceSending === 'GATEWAY_REBOOT'" :disabled="!!maintenanceSending || !maintenanceState?.canMaintain" @click="issueMaintenance('GATEWAY_REBOOT')">重启网关</el-button>
          </section>
        </div>
        <h3>最近维护记录</h3>
        <el-table :data="maintenanceTasks" size="small" empty-text="暂无远程维护记录">
          <el-table-column label="操作" width="120"><template #default="{ row }">{{ maintenanceActionText(row.action) }}</template></el-table-column>
          <el-table-column label="状态" width="110"><template #default="{ row }"><el-tag :type="maintenanceStatusType(row.status)">{{ maintenanceStatusText(row.status) }}</el-tag></template></el-table-column>
          <el-table-column label="发起时间" min-width="170"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
          <el-table-column prop="message" label="结果" min-width="220" show-overflow-tooltip />
        </el-table>
      </template>
    </el-dialog>

    <el-drawer v-if="isSuperAdmin" v-model="detailsVisible" title="出厂网关详情" size="min(720px, 100vw)" class="factory-details-drawer">
      <template v-if="detailGateway">
        <div class="factory-details-heading">
          <div><strong>{{ detailGateway.sn }}</strong><span>{{ detailGateway.model || '型号未记录' }}</span></div>
          <el-button :icon="RefreshRight" :loading="loading" @click="loadGateways">刷新详情</el-button>
        </div>
        <div class="factory-details-status"><el-tag :type="lifecycleType(detailGateway)">{{ lifecycleText(detailGateway) }}</el-tag><el-tag :type="authorizationType(detailGateway)">{{ authorizationText(detailGateway) }}</el-tag></div>
        <section class="factory-detail-section">
          <h2>设备身份</h2>
          <dl><div><dt>完整 SN</dt><dd><code>{{ detailGateway.sn }}</code></dd></div><div><dt>HardwareId</dt><dd><code>{{ detailGateway.hardwareId }}</code></dd></div><div><dt>芯片 USID</dt><dd><code>{{ detailGateway.chipUsid || '旧设备未登记' }}</code></dd></div></dl>
        </section>
        <section class="factory-detail-section">
          <h2>生产信息</h2>
          <dl><div><dt>生产批次</dt><dd>{{ detailGateway.batchNo || '未记录' }}</dd></div><div><dt>生产时间</dt><dd>{{ formatTime(detailGateway.producedAt) }}</dd></div><div><dt>产品型号</dt><dd>{{ detailGateway.model || '未记录' }}<span v-if="detailGateway.product"> · {{ detailGateway.product.name }}</span></dd></div><div><dt>硬件规格</dt><dd>{{ detailGateway.product ? productSpecification(detailGateway.product) : '产品规格未记录' }}</dd></div></dl>
          <p class="factory-detail-note">硬件规格按产品库当前记录展示。</p>
        </section>
        <section class="factory-detail-section">
          <h2>软件版本</h2>
          <dl><div><dt>当前应用版本</dt><dd>{{ detailGateway.currentVersion || '未上报' }}</dd></div><div><dt>发布序号</dt><dd>{{ detailGateway.currentRelease || '未上报' }}</dd></div><div><dt>版本确认时间</dt><dd>{{ formatTime(detailGateway.versionConfirmedAt) }}</dd></div></dl>
          <p class="factory-detail-note">以网关最近一次成功升级回执为准，清理升级包不会影响版本记录。</p>
        </section>
        <section class="factory-detail-section">
          <h2>授权信息</h2>
          <p class="factory-detail-note">{{ authorizationTooltip(detailGateway) }}</p>
          <dl><div><dt>当前签发序号</dt><dd>{{ detailGateway.licenseSequence ?? 0 }}</dd></div><div><dt>设备实际序号</dt><dd>{{ detailGateway.reportedLicense?.licenseSequence ?? '未确认' }}</dd></div><div><dt>当前授权签发时间</dt><dd>{{ formatTime(detailGateway.issuedLicense?.issuedAt) }}</dd></div><div><dt>当前签发有效期</dt><dd>{{ formatLicenseExpiry(detailGateway.issuedLicense?.expiresAt) }}</dd></div><div><dt>设备回报有效期</dt><dd>{{ formatLicenseExpiry(detailGateway.reportedLicense?.expiresAt) }}</dd></div><div><dt>首次确认激活时间</dt><dd>{{ formatTime(detailGateway.licenseActivatedAt) }}</dd></div><div><dt>最近授权回执</dt><dd>{{ formatTime(detailGateway.licenseReportedAt) }}</dd></div><div><dt>授权版本 / 签名密钥</dt><dd>{{ detailGateway.issuedLicense ? `V${detailGateway.issuedLicense.licenseVersion} / ${detailGateway.issuedLicense.keyId}` : '未记录' }}</dd></div></dl>
          <h3>当前签发授权功能</h3><div class="factory-detail-features"><el-tag v-for="feature in detailGateway.issuedLicense?.features || []" :key="feature" type="info">{{ featureText(feature) }}</el-tag><span v-if="!detailGateway.issuedLicense?.features?.length" class="muted">未记录</span></div>
          <h3>设备回报授权功能</h3><div class="factory-detail-features"><el-tag v-for="feature in detailGateway.reportedLicense?.features || []" :key="feature" type="success">{{ featureText(feature) }}</el-tag><span v-if="!detailGateway.reportedLicense?.features?.length" class="muted">尚未收到设备功能回报</span></div>
        </section>
        <section class="factory-detail-section">
          <h2>绑定信息</h2>
          <dl><div><dt>绑定设备</dt><dd>{{ detailGateway.device?.name || '未绑定' }}<span v-if="detailGateway.device">（{{ detailGateway.device.deviceKey }}）</span></dd></div><div><dt>归属租户</dt><dd><el-button v-if="detailGateway.device?.tenant" link type="primary" @click="openTenant(detailGateway)">{{ detailGateway.device.tenant.name }}</el-button><span v-else>未绑定</span></dd></div><div><dt>绑定时间</dt><dd>{{ formatTime(detailGateway.boundAt) }}</dd></div></dl>
        </section>
        <section class="factory-detail-section">
          <h2>连接信息</h2>
          <dl><div><dt>首次上线时间</dt><dd>{{ formatTime(detailGateway.firstSeenAt) }}</dd></div><div><dt>最近心跳</dt><dd>{{ formatTime(detailGateway.lastSeenAt) }}</dd></div><div><dt>授权上报状态</dt><dd>{{ !detailGateway.licenseReportedAt ? '尚未收到回执' : detailGateway.authorizationReportFresh ? '近期收到回执' : '回执暂未刷新' }}</dd></div></dl>
          <p class="factory-detail-note">授权状态是最后确认结果，不等同于设备当前在线状态。</p>
        </section>
      </template>
      <el-empty v-else description="该网关已不在当前列表，请关闭详情后重新查询" />
    </el-drawer>

    <el-dialog v-model="batchPrintDialogVisible" title="批量打印网关标签" width="min(520px, 92vw)" class="batch-print-dialog">
      <div class="batch-print-summary">
        <el-icon><Printer /></el-icon>
        <div><strong>将打印 {{ selectedGateways.length }} 个网关</strong><span>每个标签包含二维码、SN 与绑定码</span></div>
      </div>
      <el-form label-position="top" class="batch-print-form">
        <el-form-item label="标签纸规格">
          <el-radio-group v-model="batchPrintForm.size">
            <el-radio-button v-for="item in gatewayLabelSizes" :key="item.value" :value="item.value">{{ item.label }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="每个网关打印份数">
          <el-input-number v-model="batchPrintForm.copies" :min="1" :max="20" controls-position="right" />
        </el-form-item>
        <el-alert type="info" :closable="false" title="进入打印窗口后选择 Godex G500 GZPL，缩放设置为 100%，边距设置为无。" />
      </el-form>
      <template #footer>
        <el-button :disabled="batchPrinting" @click="batchPrintDialogVisible = false">取消</el-button>
        <el-button type="primary" :icon="Printer" :loading="batchPrinting" @click="confirmBatchPrint">开始打印</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="printSettingsDialogVisible" title="本地打印服务设置" width="min(560px, 92vw)" class="print-agent-dialog">
      <el-form label-position="top">
        <el-form-item label="打印方式">
          <el-switch v-model="printAgentForm.enabled" active-text="启用本地静默打印" inactive-text="使用系统打印窗口" />
        </el-form-item>
        <template v-if="printAgentForm.enabled">
          <el-form-item label="本地服务地址"><el-input v-model="printAgentForm.url" placeholder="http://127.0.0.1:17896" /></el-form-item>
          <el-form-item label="配对密钥"><el-input v-model="printAgentForm.pairingKey" type="password" show-password placeholder="粘贴安装脚本输出的配对密钥" /></el-form-item>
          <el-form-item label="打印机">
            <el-select v-model="printAgentForm.printer" filterable allow-create default-first-option placeholder="检测后选择 Godex G500 GZPL" style="width: 100%">
              <el-option v-for="name in printerOptions" :key="name" :label="name" :value="name" />
            </el-select>
          </el-form-item>
          <div class="print-agent-test-row">
            <el-button :loading="testingPrintAgent" @click="detectPrintAgent">检测服务与打印机</el-button>
            <el-tag v-if="printAgentStatus === 'online'" type="success">服务在线</el-tag>
            <el-tag v-else-if="printAgentStatus === 'offline'" type="danger">连接失败</el-tag>
          </div>
          <el-alert type="info" :closable="false" title="打印服务仅监听本机 127.0.0.1；配对密钥只保存在当前浏览器。" />
        </template>
        <el-divider content-position="left">管理 App 云端打印</el-divider>
        <div class="cloud-agent-create">
          <el-input v-model="newCloudAgentName" maxlength="64" placeholder="打印终端名称，例如 生产线 1" />
          <el-button type="primary" plain :loading="creatingCloudAgent" @click="createCloudPrintAgent">创建终端</el-button>
        </div>
        <el-alert v-if="latestCloudAgentToken" type="warning" :closable="false" title="终端令牌只在创建后显示一次，请立即复制并用于安装 Windows 打印服务。" />
        <div v-if="latestCloudAgentToken" class="cloud-agent-token"><code>{{ latestCloudAgentToken }}</code><el-button :icon="CopyDocument" @click="copyCloudAgentToken">复制</el-button></div>
        <div class="cloud-agent-list">
          <div v-for="agent in cloudPrintAgents" :key="agent.id" class="cloud-agent-item">
            <div><strong>{{ agent.name }}</strong><span>{{ agent.printerName || '未上报打印机' }} · {{ formatTime(agent.lastSeenAt) }}</span></div>
            <el-tag :type="agent.online ? 'success' : 'info'">{{ agent.online ? '在线' : '离线' }}</el-tag>
            <el-button link type="danger" @click="removeCloudPrintAgent(agent)">移除</el-button>
          </div>
          <el-empty v-if="!cloudPrintAgents.length" :image-size="54" description="尚未创建云端打印终端" />
        </div>
      </el-form>
      <template #footer>
        <el-button @click="printSettingsDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="savePrintAgentSettings">保存设置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ota-fields { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin: 18px 0; }
.ota-fields label { display: grid; gap: 7px; font-size: 13px; color: #53647c; }
.ota-actions { display: flex; justify-content: flex-end; gap: 8px; margin-bottom: 22px; }
.ota-progress-card { margin: 0 0 20px; padding: 15px 16px 13px; border: 1px solid #cfe1ff; border-radius: 10px; background: #f7faff; }
.ota-progress-heading { display: flex; align-items: center; justify-content: space-between; gap: 14px; margin-bottom: 11px; }
.ota-progress-heading>div { display: grid; gap: 3px; }
.ota-progress-heading strong { color: #1f3553; font-size: 14px; }
.ota-progress-heading span { color: #71839b; font-size: 12px; }
.ota-progress-heading em { color: #1677ff; font-size: 20px; font-style: normal; font-weight: 700; font-variant-numeric: tabular-nums; }
.ota-progress-steps { display: grid; grid-template-columns: repeat(4, 1fr); margin-top: 12px; }
.ota-progress-steps span { position: relative; display: flex; align-items: center; gap: 6px; color: #94a3b8; font-size: 11px; }
.ota-progress-steps span:not(:last-child)::after { position: absolute; right: 8px; left: 78px; height: 1px; background: #dce6f2; content: ''; }
.ota-progress-steps i { display: inline-grid; width: 20px; height: 20px; place-items: center; border: 1px solid #d7e1ed; border-radius: 50%; background: #fff; font-style: normal; }
.ota-progress-steps span.done,.ota-progress-steps span.active { color: #1677ff; font-weight: 600; }
.ota-progress-steps span.done i { border-color: #70b0ff; color: #fff; background: #409eff; }
.ota-progress-steps span.active i { border-color: #409eff; color: #1677ff; background: #eef6ff; box-shadow: 0 0 0 3px rgba(64,158,255,.12); }
.ota-progress-card p { margin: 11px 0 0; color: #657791; font-size: 12px; line-height: 1.6; }
@media (max-width: 620px) { .ota-fields { grid-template-columns: 1fr; } }
.page-action-buttons { display: flex; align-items: center; gap: 10px; }
.page-action-buttons :deep(.el-button) { margin-left: 0; }
.table-toolbar-spacer { flex: 1; }
.selection-summary { color: #64748b; font-size: 13px; }
.batch-print-summary { display: flex; align-items: center; gap: 13px; padding: 14px; border: 1px solid #dbeafe; border-radius: 12px; background: #f8fbff; }
.batch-print-summary > .el-icon { display: grid; width: 42px; height: 42px; place-items: center; border-radius: 10px; background: #eaf3ff; color: #1677ff; font-size: 22px; }
.batch-print-summary div { display: grid; gap: 4px; }
.batch-print-summary strong { color: #172033; font-size: 15px; }
.batch-print-summary span { color: #64748b; font-size: 12px; }
.batch-print-form { margin-top: 18px; }
.print-agent-test-row { display: flex; align-items: center; gap: 10px; margin: -2px 0 16px; }
.cloud-agent-create { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 10px; margin-bottom: 12px; }
.cloud-agent-token { display: flex; align-items: center; gap: 10px; margin: 10px 0 14px; padding: 10px 12px; border: 1px solid #fde68a; border-radius: 10px; background: #fffbeb; }
.cloud-agent-token code { min-width: 0; flex: 1; overflow: hidden; color: #92400e; text-overflow: ellipsis; white-space: nowrap; }
.cloud-agent-list { display: grid; gap: 8px; max-height: 210px; margin-top: 12px; overflow: auto; }
.cloud-agent-item { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 10px; padding: 10px 12px; border: 1px solid #e2e8f0; border-radius: 10px; }
.cloud-agent-item > div { display: grid; min-width: 0; gap: 3px; }
.cloud-agent-item strong { color: #172033; font-size: 13px; }
.cloud-agent-item span { overflow: hidden; color: #8b96a8; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.section-title { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
.section-title h2 { margin: 0; color: #172033; font-size: 18px; }
.section-title p { margin: 6px 0 0; color: #64748b; font-size: 13px; line-height: 1.6; }
.section-title span { color: #8b96a8; font-size: 13px; }
.dialog-description { margin: 0 0 20px; color: #64748b; font-size: 14px; line-height: 1.7; }
.register-dialog-form { max-width: 680px; }
.identity-paste-hint { margin-top: 6px; color: #8b96a8; font-size: 12px; line-height: 1.6; }
.identity-paste-hint.recognized { color: #15803d; }
.factory-register-dialog :deep(.el-dialog__body),
.gateway-qr-dialog :deep(.el-dialog__body) { padding-top: 12px; }
.dialog-handover-grid { margin-top: 16px; align-items: start; }
.dialog-handover-grid .handover-card { padding: 14px 16px; }
.dialog-handover-grid .handover-card h3 { margin-top: 8px; }
.dialog-handover-grid .handover-card > p { margin-bottom: 10px; }
.activation-identity { display: grid; gap: 5px; margin-bottom: 14px; padding: 10px 12px; border-radius: 8px; background: #f8fafc; }
.activation-identity span { color: #64748b; font-size: 12px; }
.activation-identity code { color: #334155; font-size: 12px; overflow-wrap: anywhere; }
.handover-label-section { margin-top: 14px; }
.handover-qr { grid-template-columns: 240px minmax(0, 1fr); margin-top: 0; padding: 12px 16px; gap: 24px; }
.handover-qr :deep(.label-preview) { max-width: 240px; box-shadow: 0 4px 14px rgba(15, 23, 42, .06); }
.qr-credential-summary { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin: 16px 0 4px; }
.qr-credential-summary > div { display: grid; gap: 6px; padding: 12px 14px; border: 1px solid #e2e8f0; border-radius: 10px; background: #f8fafc; }
.qr-credential-summary span { color: #64748b; font-size: 12px; }
.qr-credential-summary code { color: #172033; font-weight: 700; }
.bind-panel { max-width: 820px; margin: 0 auto; padding: 0; overflow: hidden; }
.bind-steps { padding: 18px 24px; border-bottom: 1px solid #e8eef6; background: #f8fbff; }
.bind-stage { padding: 28px 32px 32px; }
.stage-heading h2 { margin: 0; color: #172033; font-size: 20px; }
.stage-heading p { margin: 8px 0 22px; color: #64748b; font-size: 14px; line-height: 1.7; }
.bind-form { max-width: 590px; }
.bind-help { margin: 18px 0 0; padding-top: 16px; border-top: 1px solid #edf1f7; color: #8b96a8; font-size: 13px; line-height: 1.6; }
.verified-card { display: flex; align-items: center; gap: 12px; padding: 16px; border: 1px solid #bbf7d0; border-radius: 14px; background: #f0fdf4; }
.verified-card .verified-icon { display: grid; width: 38px; height: 38px; place-items: center; border-radius: 50%; background: #16a34a; color: #fff; font-size: 22px; }
.verified-card > div:nth-child(2) { flex: 1; min-width: 0; }
.verified-card strong { color: #166534; font-size: 15px; }
.verified-card p { margin: 3px 0 0; color: #15803d; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }
.gateway-summary { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 16px 0; }
.gateway-summary div { padding: 13px 14px; border: 1px solid #e2e8f0; border-radius: 12px; background: #fff; }
.gateway-summary span { display: block; margin-bottom: 5px; color: #94a3b8; font-size: 12px; }
.gateway-summary strong { color: #334155; font-size: 13px; }
.device-form { margin-top: 20px; }
.form-actions, .success-actions { display: flex; gap: 10px; }
.bind-success { padding: 46px 32px 50px; text-align: center; }
.success-icon { color: #16a34a; font-size: 58px; }
.bind-success h2 { margin: 12px 0 8px; color: #172033; }
.bind-success p { margin: 0 auto 24px; max-width: 560px; color: #64748b; line-height: 1.7; }
.success-actions { justify-content: center; }
.credential-panel { margin-bottom: 18px; border-color: #bfdbfe; background: #f8fbff; }
.handover-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.handover-card { padding: 18px; border: 1px solid #dbeafe; border-radius: 14px; background: #fff; }
.handover-card .step-badge { display: inline-flex; padding: 3px 9px; border-radius: 999px; background: #eff6ff; color: #2563eb; font-size: 12px; }
.handover-card h3 { margin: 12px 0 6px; color: #172033; }
.handover-card > p { margin: 0 0 16px; color: #64748b; font-size: 13px; line-height: 1.6; }
.activation-collapse { margin-top: 10px; }
.activation-collapse pre { max-height: 220px; margin: 0; overflow: auto; color: #334155; font-size: 12px; white-space: pre-wrap; word-break: break-all; }
.credential-line { display: grid; grid-template-columns: 70px minmax(0, 1fr); align-items: center; gap: 10px; margin-bottom: 10px; padding: 11px 12px; border-radius: 10px; background: #f8fafc; }
.credential-line span { color: #64748b; font-size: 13px; }
.credential-line code { color: #172033; font-weight: 700; }
.customer-handover .el-button { margin-top: 6px; }
.credential-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.credential-grid label { display: block; margin-bottom: 8px; color: #64748b; font-size: 13px; }
.reset-actions { display: flex; align-items: flex-end; }
.reset-actions .el-button { width: 100%; }
.copy-line { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 10px; padding: 10px 12px; border: 1px solid #e2e8f0; border-radius: 10px; background: #fff; }
.copy-line code { overflow: hidden; color: #172033; text-overflow: ellipsis; white-space: nowrap; }
.activation-box { margin-top: 16px; border: 1px solid #e2e8f0; border-radius: 12px; background: #fff; overflow: hidden; }
.activation-header { display: flex; align-items: center; justify-content: space-between; padding: 12px 14px; border-bottom: 1px solid #e2e8f0; }
.activation-box pre { margin: 0; padding: 14px; overflow: auto; color: #334155; font-size: 13px; line-height: 1.7; }
.muted { color: #94a3b8; }
.gateway-operation-group { display: flex; align-items: center; flex-wrap: nowrap; gap: 0 4px; white-space: nowrap; }
.gateway-operation-group :deep(.el-button) { margin-left: 0; padding-right: 7px; padding-left: 7px; }
.gateway-operation-group :deep(.el-dropdown) { flex: 0 0 auto; }
.factory-details-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.factory-details-heading > div { display: grid; min-width: 0; gap: 6px; }
.factory-details-heading strong { color: #172033; font-size: 18px; overflow-wrap: anywhere; }
.factory-details-heading span { color: #64748b; font-size: 13px; }
.factory-details-status { display: flex; gap: 8px; margin: 16px 0 20px; }
.factory-detail-section { margin-bottom: 18px; padding: 18px; border: 1px solid #e8eef6; border-radius: 12px; background: #fff; }
.factory-detail-section h2 { margin: 0 0 14px; color: #172033; font-size: 15px; }
.factory-detail-section h3 { margin: 16px 0 10px; color: #64748b; font-size: 12px; font-weight: 500; }
.factory-detail-section dl { display: grid; gap: 12px; margin: 0; }
.factory-detail-section dl > div { display: grid; grid-template-columns: 140px minmax(0, 1fr); gap: 12px; }
.factory-detail-section dt { color: #8b96a8; font-size: 13px; }
.factory-detail-section dd { min-width: 0; margin: 0; color: #334155; font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.factory-detail-section code { font-size: 12px; }
.factory-detail-note { margin: 12px 0; color: #8b96a8; font-size: 12px; line-height: 1.7; }
.factory-detail-features { display: flex; flex-wrap: wrap; gap: 8px; }
:global(.danger-dropdown-item) { color: var(--el-color-danger) !important; }
:global(.danger-dropdown-item:hover) { background: var(--el-color-danger-light-9) !important; }
.tenant-link { max-width: 100%; margin-left: 0; }
.tenant-link :deep(span) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.maintenance-target { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.maintenance-target > div { display: grid; gap: 4px; }
.maintenance-target strong { color: #172033; font-size: 16px; }
.maintenance-target span { color: #64748b; font-size: 12px; }
.maintenance-actions { display: grid; gap: 10px; margin: 16px 0 20px; }
.maintenance-actions section { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 15px 16px; border: 1px solid #e2e8f0; border-radius: 12px; background: #f8fafc; }
.maintenance-actions section > div { min-width: 0; }
.maintenance-actions strong { color: #172033; font-size: 14px; }
.maintenance-actions p { margin: 5px 0 0; color: #64748b; font-size: 12px; line-height: 1.6; }
@media (max-width: 1000px) {
  .credential-grid, .handover-grid { grid-template-columns: 1fr; }
  .dialog-handover-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 640px) {
  .factory-detail-section { padding: 14px; }
  .factory-detail-section dl > div { grid-template-columns: 1fr; gap: 4px; }
  .dialog-handover-grid, .handover-qr { grid-template-columns: 1fr; }
  .page-action-buttons { width: 100%; }
  .page-action-buttons :deep(.el-button) { flex: 1; }
  .qr-credential-summary { grid-template-columns: 1fr; }
  .bind-steps { padding: 12px; }
  .bind-stage { padding: 22px 18px 24px; }
  .gateway-summary { grid-template-columns: 1fr; }
  .form-actions, .success-actions { flex-direction: column; }
  .form-actions .el-button, .success-actions .el-button { width: 100%; margin-left: 0; }
  .maintenance-actions section { align-items: stretch; flex-direction: column; }
}
</style>

<style>
.factory-register-dialog { display: flex; flex-direction: column; max-height: 92vh; max-height: 92dvh; margin: 4vh auto; }
.factory-register-dialog .el-dialog__header { flex: 0 0 auto; margin-right: 0; padding-bottom: 16px; }
.factory-register-dialog .el-dialog__body { min-height: 0; overflow-y: auto; overscroll-behavior: contain; }
.factory-register-dialog .el-dialog__footer { flex: 0 0 auto; padding-top: 16px; margin-top: 14px; border-top: 1px solid #edf1f7; }
</style>

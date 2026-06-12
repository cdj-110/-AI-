<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { RefreshRight } from '@element-plus/icons-vue';
import { useRouter } from 'vue-router';
import { apiRequest } from '../api/request';

interface DeviceItem {
  id: string;
  deviceKey: string;
  name: string;
  deviceType: string;
  status: string;
  location?: string;
  latitude?: number;
  longitude?: number;
  lastSeenAt?: string;
}

type AMapNamespace = {
  Map: new (container: HTMLElement, options: Record<string, unknown>) => AMapMap;
  Marker: new (options: Record<string, unknown>) => AMapMarker;
  InfoWindow: new (options: Record<string, unknown>) => AMapInfoWindow;
};

type AMapMap = {
  add: (overlays: AMapMarker[]) => void;
  remove: (overlays: AMapMarker[]) => void;
  setFitView: (overlays?: AMapMarker[], immediately?: boolean, avoid?: number[], maxZoom?: number) => void;
  setCenter: (center: [number, number]) => void;
  setZoom: (zoom: number) => void;
  destroy: () => void;
};

type AMapMarker = {
  on: (eventName: string, handler: () => void) => void;
};

type AMapInfoWindow = {
  open: (map: AMapMap, position: [number, number]) => void;
};

declare global {
  interface Window {
    AMap?: AMapNamespace;
    _AMapSecurityConfig?: { securityJsCode: string };
  }
}

const router = useRouter();
const loading = ref(false);
const mapLoading = ref(false);
const mapError = ref('');
const devices = ref<DeviceItem[]>([]);
const selectedStatus = ref('');
const mapRef = ref<HTMLDivElement>();
let amap: AMapNamespace | undefined;
let map: AMapMap | undefined;
let infoWindow: AMapInfoWindow | undefined;
let markers: AMapMarker[] = [];
let refreshTimer: ReturnType<typeof setInterval> | undefined;
let amapLoader: Promise<AMapNamespace> | undefined;

const amapKey = (import.meta.env.VITE_AMAP_KEY as string | undefined) ?? '99b5c6fb59ffb2024fbccc5747595fde';
const amapSecurityCode = (import.meta.env.VITE_AMAP_SECURITY_CODE as string | undefined) ?? '02eb711b9457def0e71bb5933c068d6f';
const defaultCenter: [number, number] = [
  Number(import.meta.env.VITE_AMAP_CENTER_LNG ?? 116.397428),
  Number(import.meta.env.VITE_AMAP_CENTER_LAT ?? 39.90923),
];
const defaultZoom = Number(import.meta.env.VITE_AMAP_ZOOM ?? 16);

const locatedDevices = computed(() => devices.value.filter((device) => Number.isFinite(device.latitude) && Number.isFinite(device.longitude)));
const filteredDevices = computed(() => locatedDevices.value.filter((device) => !selectedStatus.value || device.status === selectedStatus.value));
const onlineCount = computed(() => locatedDevices.value.filter((device) => device.status === 'ONLINE').length);
const offlineCount = computed(() => locatedDevices.value.filter((device) => device.status === 'OFFLINE').length);
const missingCoordinateCount = computed(() => devices.value.length - locatedDevices.value.length);

function statusLabel(status: string) {
  return { ONLINE: '在线', OFFLINE: '离线', DISABLED: '停用' }[status] ?? status;
}

function deviceTypeLabel(type: string) {
  return { GATEWAY: '网关', GATEWAY_CHILD: '子设备', DIRECT: '直连' }[type] ?? type;
}

function markerClass(status: string) {
  if (status === 'ONLINE') return 'online';
  if (status === 'DISABLED') return 'disabled';
  return 'offline';
}

function escapeHtml(value: unknown) {
  return String(value ?? '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }[char] ?? char));
}

function loadAmap() {
  if (window.AMap) return Promise.resolve(window.AMap);
  amapLoader ??= new Promise<AMapNamespace>((resolve, reject) => {
    window._AMapSecurityConfig = { securityJsCode: amapSecurityCode };
    const script = document.createElement('script');
    script.src = `https://webapi.amap.com/maps?v=2.0&key=${amapKey}`;
    script.async = true;
    script.onload = () => window.AMap ? resolve(window.AMap) : reject(new Error('高德地图 SDK 加载失败'));
    script.onerror = () => reject(new Error('高德地图 SDK 加载失败，请检查网络或 Key 配置'));
    document.head.appendChild(script);
  });
  return amapLoader;
}

async function initMap() {
  if (map || !mapRef.value) return;
  mapLoading.value = true;
  mapError.value = '';
  try {
    amap = await loadAmap();
    map = new amap.Map(mapRef.value, {
      center: defaultCenter,
      zoom: defaultZoom,
      viewMode: '2D',
      resizeEnable: true,
      mapStyle: 'amap://styles/normal',
    });
    infoWindow = new amap.InfoWindow({ offset: [0, -30], closeWhenClickMap: true });
  } catch (error) {
    mapError.value = error instanceof Error ? error.message : '高德地图加载失败';
  } finally {
    mapLoading.value = false;
  }
}

function markerContent(device: DeviceItem) {
  return `<div class="amap-device-marker ${markerClass(device.status)}">
    <span></span>
    <strong>${escapeHtml(device.name)}</strong>
  </div>`;
}

function infoContent(device: DeviceItem) {
  return `<div class="amap-device-info">
    <strong>${escapeHtml(device.name)}</strong>
    <p>编号：${escapeHtml(device.deviceKey)}</p>
    <p>类型：${escapeHtml(deviceTypeLabel(device.deviceType))}</p>
    <p>状态：${escapeHtml(statusLabel(device.status))}</p>
    <p>位置：${escapeHtml(device.location || '-')}</p>
    <p>坐标：${device.longitude?.toFixed(6)}, ${device.latitude?.toFixed(6)}</p>
    <button type="button">进入详情</button>
  </div>`;
}

async function renderMap() {
  await nextTick();
  await initMap();
  if (!map || !amap) return;
  if (markers.length) map.remove(markers);
  markers = filteredDevices.value.map((device) => {
    const position: [number, number] = [Number(device.longitude), Number(device.latitude)];
    const marker = new amap.Marker({
      position,
      content: markerContent(device),
      anchor: 'bottom-center',
      extData: device,
    });
    marker.on('click', () => {
      infoWindow?.open(map as AMapMap, position);
      infoWindow = new (amap as AMapNamespace).InfoWindow({
        content: infoContent(device),
        offset: [0, -32],
        closeWhenClickMap: true,
      });
      infoWindow.open(map as AMapMap, position);
      setTimeout(() => {
        document.querySelector('.amap-device-info button')?.addEventListener('click', () => router.push(`/devices/${device.id}`), { once: true });
      });
    });
    return marker;
  });
  if (markers.length) {
    map.add(markers);
    map.setFitView(markers, false, [80, 80, 80, 80], defaultZoom);
    return;
  }
  map.setCenter(defaultCenter);
  map.setZoom(defaultZoom);
}

async function loadDevices(silentError = false) {
  loading.value = true;
  try {
    const data = await apiRequest<{ items: DeviceItem[] }>({
      url: '/api/devices',
      method: 'GET',
      params: { page: 1, pageSize: 100 },
      silentError,
    });
    devices.value = data.items;
    await renderMap();
  } finally {
    loading.value = false;
  }
}

function handleDeviceStatusEvent(event: Event) {
  const detail = (event as CustomEvent<Partial<DeviceItem>>).detail;
  if (!detail?.id || !detail.status) return;
  const target = devices.value.find((device) => device.id === detail.id);
  if (!target) return;
  target.status = detail.status;
  if (detail.lastSeenAt) target.lastSeenAt = detail.lastSeenAt;
  void renderMap();
}

onMounted(async () => {
  window.addEventListener('device-status-change', handleDeviceStatusEvent);
  await initMap();
  await loadDevices();
  refreshTimer = setInterval(() => void loadDevices(true), 30000);
});

onBeforeUnmount(() => {
  window.removeEventListener('device-status-change', handleDeviceStatusEvent);
  if (refreshTimer) clearInterval(refreshTimer);
  if (markers.length) map?.remove(markers);
  map?.destroy();
});
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">设备地图</h1>
        <p class="page-description">使用高德地图展示设备位置，点击点位可查看详情</p>
      </div>
      <div class="map-actions">
        <el-select v-model="selectedStatus" clearable placeholder="全部状态" style="width: 130px" @change="renderMap">
          <el-option label="在线" value="ONLINE" />
          <el-option label="离线" value="OFFLINE" />
          <el-option label="停用" value="DISABLED" />
        </el-select>
        <el-button :icon="RefreshRight" @click="loadDevices()">刷新</el-button>
      </div>
    </div>

    <section class="map-stats">
      <div><span>有坐标设备</span><strong>{{ locatedDevices.length }}</strong></div>
      <div><span>在线</span><strong class="online">{{ onlineCount }}</strong></div>
      <div><span>离线</span><strong>{{ offlineCount }}</strong></div>
      <div><span>未配置坐标</span><strong>{{ missingCoordinateCount }}</strong></div>
    </section>

    <section v-loading="loading || mapLoading" class="map-panel">
      <div ref="mapRef" class="device-map" />
      <el-alert v-if="mapError" class="map-error" type="error" :closable="false" show-icon :title="mapError" />
      <el-empty v-else-if="!filteredDevices.length" class="map-empty" description="暂无可展示的设备坐标，请先在设备编辑中填写经纬度" />
    </section>

    <section class="table-panel map-list">
      <div class="section-heading">
        <h2>地图点位</h2>
        <p>只展示已配置经纬度的设备</p>
      </div>
      <el-table :data="filteredDevices" empty-text="暂无设备点位">
        <el-table-column label="设备名称" min-width="160">
          <template #default="{ row }"><router-link :to="`/devices/${row.id}`">{{ row.name }}</router-link></template>
        </el-table-column>
        <el-table-column prop="deviceKey" label="设备编号" min-width="150" />
        <el-table-column label="类型" width="90"><template #default="{ row }">{{ deviceTypeLabel(row.deviceType) }}</template></el-table-column>
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'ONLINE' ? 'success' : 'info'">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column prop="location" label="位置" min-width="150" />
        <el-table-column label="坐标" min-width="190"><template #default="{ row }">{{ row.longitude?.toFixed(6) }}, {{ row.latitude?.toFixed(6) }}</template></el-table-column>
      </el-table>
    </section>
  </div>
</template>

<style scoped>
.map-actions { display: flex; gap: 10px; align-items: center; }
.map-stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; margin-bottom: 18px; }
.map-stats div { padding: 16px 18px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.map-stats span { display: block; color: #94a3b8; font-size: 13px; }
.map-stats strong { display: block; margin-top: 6px; color: #111827; font-size: 24px; }
.map-stats strong.online { color: #22c55e; }
.map-panel { position: relative; min-height: 560px; padding: 16px; border: 1px solid #e9eef5; border-radius: 14px; background: #fff; }
.device-map { height: 560px; border-radius: 10px; overflow: hidden; }
.map-empty { position: absolute; inset: 80px 24px 24px; pointer-events: none; }
.map-error { position: absolute; top: 28px; left: 28px; right: 28px; z-index: 2; }
.map-list { margin-top: 18px; }
.section-heading { display: flex; align-items: baseline; justify-content: space-between; margin-bottom: 14px; }
.section-heading h2 { margin: 0; color: #111827; font-size: 16px; }
.section-heading p { margin: 0; color: #94a3b8; font-size: 13px; }
:deep(.amap-device-marker) { display: flex; gap: 7px; align-items: center; padding: 7px 10px; border: 1px solid rgb(255 255 255 / 86%); border-radius: 999px; color: #172033; background: rgb(255 255 255 / 94%); box-shadow: 0 10px 26px rgb(15 23 42 / 18%); white-space: nowrap; }
:deep(.amap-device-marker span) { width: 10px; height: 10px; border-radius: 50%; background: #9ca3af; box-shadow: 0 0 0 4px rgb(156 163 175 / 16%); }
:deep(.amap-device-marker.online span) { background: #22c55e; box-shadow: 0 0 0 4px rgb(34 197 94 / 16%); }
:deep(.amap-device-marker.disabled span) { background: #94a3b8; box-shadow: 0 0 0 4px rgb(148 163 184 / 16%); }
:deep(.amap-device-marker strong) { font-size: 12px; font-weight: 700; }
:deep(.amap-device-info) { min-width: 210px; color: #172033; line-height: 1.6; }
:deep(.amap-device-info strong) { display: block; margin-bottom: 6px; font-size: 15px; }
:deep(.amap-device-info p) { margin: 0; color: #64748b; font-size: 12px; }
:deep(.amap-device-info button) { margin-top: 10px; padding: 6px 10px; border: 0; border-radius: 8px; color: #fff; background: #2563eb; cursor: pointer; }
@media (max-width: 900px) { .map-stats { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 560px) { .map-stats { grid-template-columns: 1fr; } .map-actions { align-items: flex-end; flex-direction: column; } }
</style>

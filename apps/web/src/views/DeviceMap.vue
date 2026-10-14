<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { Expand, Fold, RefreshRight } from '@element-plus/icons-vue';
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
  groups?: Array<{ id: string; name: string }>;
}

interface MapCategory {
  id: string;
  name: string;
  count: number;
}

type AMapNamespace = {
  Map: new (container: HTMLElement, options: Record<string, unknown>) => AMapMap;
  Marker: new (options: Record<string, unknown>) => AMapMarker;
  InfoWindow: new (options: Record<string, unknown>) => AMapInfoWindow;
  Geocoder?: new (options?: Record<string, unknown>) => AMapGeocoder;
  plugin: (plugins: string | string[], callback: () => void) => void;
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

type AMapGeocoder = {
  getAddress: (position: [number, number], callback: (status: string, result: {
    regeocode?: { addressComponent?: { province?: string; city?: string | string[]; district?: string } };
  }) => void) => void;
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
const displayMode = ref<'all' | 'group' | 'region'>('all');
const selectedCategory = ref('');
const panelCollapsed = ref(false);
const resolvedRegions = ref<Record<string, string>>({});
const resolvingRegions = ref(false);
const mapRef = ref<HTMLDivElement>();
let amap: AMapNamespace | undefined;
let map: AMapMap | undefined;
let infoWindow: AMapInfoWindow | undefined;
let geocoder: AMapGeocoder | undefined;
let geocoderLoader: Promise<AMapGeocoder> | undefined;
let markers: AMapMarker[] = [];
let mapViewportInitialized = false;
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
const mapCategories = computed<MapCategory[]>(() => {
  const categories = new Map<string, MapCategory>();
  if (displayMode.value === 'group') {
    for (const device of locatedDevices.value) {
      const memberships = device.groups?.length ? device.groups : [{ id: '__ungrouped__', name: '未分组' }];
      for (const group of memberships) {
        const item = categories.get(group.id) ?? { id: group.id, name: group.name, count: 0 };
        item.count++;
        categories.set(group.id, item);
      }
    }
  } else if (displayMode.value === 'region') {
    for (const device of locatedDevices.value) {
      const name = deviceRegion(device);
      const item = categories.get(name) ?? { id: name, name, count: 0 };
      item.count++;
      categories.set(name, item);
    }
  }
  return [...categories.values()].sort((left, right) => {
    if (left.id.startsWith('__')) return 1;
    if (right.id.startsWith('__')) return -1;
    return left.name.localeCompare(right.name, 'zh-CN');
  });
});
const filteredDevices = computed(() => locatedDevices.value.filter((device) => {
  if (selectedStatus.value && device.status !== selectedStatus.value) return false;
  if (!selectedCategory.value || displayMode.value === 'all') return true;
  if (displayMode.value === 'group') {
    return selectedCategory.value === '__ungrouped__'
      ? !device.groups?.length
      : device.groups?.some((group) => group.id === selectedCategory.value);
  }
  return deviceRegion(device) === selectedCategory.value;
}));
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

function deviceRegion(device: DeviceItem) {
  if (resolvedRegions.value[device.id]) return resolvedRegions.value[device.id];
  const location = device.location?.trim();
  if (!location) return '未设置地域';
  const municipality = ['北京市', '上海市', '天津市', '重庆市'].find((name) => location.includes(name));
  if (municipality) return municipality.slice(0, -1);
  const province = location.match(/^(.{2,12}?(?:省|自治区|特别行政区))/)?.[1];
  const remainder = province ? location.slice(province.length) : location;
  const city = remainder.match(/^(.{2,12}?(?:市|自治州|地区|盟))/)?.[1];
  if (city) return city.replace(/(?:市|自治州|地区|盟)$/, '');
  if (province) return province.replace(/(?:省|自治区|特别行政区)$/, '');
  return location.split(/[，,、/\\\s-]+/)[0]?.slice(0, 18) || '未设置地域';
}

function normalizedRegion(province?: string, city?: string | string[]) {
  const cityName = Array.isArray(city) ? city[0] : city;
  const source = cityName || province || '';
  return source.replace(/(?:市|省|自治区|特别行政区|自治州|地区|盟)$/, '') || undefined;
}

async function loadGeocoder() {
  if (geocoder) return geocoder;
  await initMap();
  if (!amap) throw new Error('地图尚未加载');
  geocoderLoader ??= new Promise<AMapGeocoder>((resolve, reject) => {
    amap?.plugin('AMap.Geocoder', () => {
      if (!amap?.Geocoder) {
        reject(new Error('地域识别组件加载失败'));
        return;
      }
      geocoder = new amap.Geocoder({ radius: 1000, extensions: 'base' });
      resolve(geocoder);
    });
  });
  return geocoderLoader;
}

function reverseGeocode(device: DeviceItem, coder: AMapGeocoder) {
  return new Promise<string | undefined>((resolve) => {
    const timer = window.setTimeout(() => resolve(undefined), 5000);
    coder.getAddress([Number(device.longitude), Number(device.latitude)], (status, result) => {
      window.clearTimeout(timer);
      if (status !== 'complete') {
        resolve(undefined);
        return;
      }
      const component = result.regeocode?.addressComponent;
      resolve(normalizedRegion(component?.province, component?.city));
    });
  });
}

async function resolveDeviceRegions() {
  const targets = locatedDevices.value.filter((device) => !resolvedRegions.value[device.id]);
  if (!targets.length) return;
  resolvingRegions.value = true;
  try {
    const coder = await loadGeocoder();
    const next = { ...resolvedRegions.value };
    for (let index = 0; index < targets.length; index += 5) {
      const batch = targets.slice(index, index + 5);
      const regions = await Promise.all(batch.map((device) => reverseGeocode(device, coder)));
      batch.forEach((device, batchIndex) => {
        if (regions[batchIndex]) next[device.id] = regions[batchIndex] as string;
      });
    }
    resolvedRegions.value = next;
    if (selectedCategory.value && !mapCategories.value.some((category) => category.id === selectedCategory.value)) selectedCategory.value = '';
    await renderMap();
  } catch {
    // 地域识别失败时继续使用安装位置文本，不影响地图和点位展示。
  } finally {
    resolvingRegions.value = false;
  }
}

function categoryDescription() {
  if (displayMode.value === 'group') return '一个设备可同时出现在多个分组中，数量以当前有坐标的设备计算';
  if (displayMode.value === 'region') return '根据设备安装位置中的省市信息自动归类';
  return '';
}

function applyStatusFilter() {
  void renderMap({ fitView: true });
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
    <p>分组：${escapeHtml(device.groups?.map((group) => group.name).join('、') || '未分组')}</p>
    <p>地域：${escapeHtml(deviceRegion(device))}</p>
    <p>位置：${escapeHtml(device.location || '-')}</p>
    <p>坐标：${device.longitude?.toFixed(6)}, ${device.latitude?.toFixed(6)}</p>
    <button type="button">进入详情</button>
  </div>`;
}

function openDeviceInfo(device: DeviceItem, focus = false) {
  if (!map || !amap || !Number.isFinite(device.longitude) || !Number.isFinite(device.latitude)) return;
  const position: [number, number] = [Number(device.longitude), Number(device.latitude)];
  infoWindow = new amap.InfoWindow({
    content: infoContent(device),
    offset: [0, -32],
    closeWhenClickMap: true,
  });
  infoWindow.open(map, position);
  if (focus) {
    map.setCenter(position);
    map.setZoom(Math.max(defaultZoom, 15));
  }
  setTimeout(() => {
    document.querySelector('.amap-device-info button')?.addEventListener('click', () => router.push(`/devices/${device.id}`), { once: true });
  });
}

async function renderMap(options: { fitView?: boolean } = {}) {
  await nextTick();
  await initMap();
  if (!map || !amap) return;
  if (markers.length) map.remove(markers);
  const mapApi = amap;
  markers = filteredDevices.value.map((device) => {
    const position: [number, number] = [Number(device.longitude), Number(device.latitude)];
    const marker = new mapApi.Marker({
      position,
      content: markerContent(device),
      anchor: 'bottom-center',
      extData: device,
    });
    marker.on('click', () => openDeviceInfo(device));
    return marker;
  });
  if (markers.length) {
    map.add(markers);
    if (options.fitView || !mapViewportInitialized) {
      map.setFitView(markers, false, [80, 80, 80, 80], defaultZoom);
      mapViewportInitialized = true;
    }
    return;
  }
  if (!mapViewportInitialized) {
    map.setCenter(defaultCenter);
    map.setZoom(defaultZoom);
    mapViewportInitialized = true;
  }
}

async function loadDevices(silentError = false) {
  loading.value = true;
  try {
    const items: DeviceItem[] = [];
    let page = 1;
    let total = 0;
    do {
      const data = await apiRequest<{ items: DeviceItem[]; total: number }>({
        url: '/api/devices',
        method: 'GET',
        params: { page, pageSize: 100 },
        silentError,
      });
      items.push(...data.items);
      total = data.total;
      page++;
    } while (items.length < total);
    devices.value = items;
    if (selectedCategory.value && !mapCategories.value.some((category) => category.id === selectedCategory.value)) selectedCategory.value = '';
    await renderMap();
    void resolveDeviceRegions();
  } finally {
    loading.value = false;
  }
}

watch(displayMode, () => {
  selectedCategory.value = '';
  void renderMap({ fitView: true });
});

watch(selectedCategory, () => void renderMap({ fitView: true }));

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
  <div class="device-map-page">
    <div class="map-page-heading">
      <div>
        <h1 class="page-title">设备地图</h1>
        <p class="page-description">以地图为主视图，设备、分组与地域信息集中在悬浮面板中</p>
      </div>
    </div>

    <section v-loading="loading || mapLoading" class="map-workspace">
      <div ref="mapRef" class="device-map" />
      <el-alert v-if="mapError" class="map-error" type="error" :closable="false" show-icon :title="mapError" />
      <el-empty v-else-if="!filteredDevices.length" class="map-empty" description="暂无可展示的设备坐标，请先在设备编辑中填写经纬度" />

      <aside v-if="!panelCollapsed" class="map-floating-panel">
        <div class="floating-panel-heading">
          <div><strong>设备视图</strong><span>当前地图显示 {{ filteredDevices.length }} 台</span></div>
          <el-button text circle :icon="Fold" aria-label="收起设备面板" @click="panelCollapsed = true" />
        </div>

        <el-radio-group v-model="displayMode" class="map-mode-switch">
          <el-radio-button value="all">全部</el-radio-button>
          <el-radio-button value="group">分组</el-radio-button>
          <el-radio-button value="region">地域</el-radio-button>
        </el-radio-group>

        <div class="floating-stats">
          <div><span>有坐标</span><strong>{{ locatedDevices.length }}</strong></div>
          <div><span>在线</span><strong class="online">{{ onlineCount }}</strong></div>
          <div><span>离线</span><strong>{{ offlineCount }}</strong></div>
          <div><span>无坐标</span><strong>{{ missingCoordinateCount }}</strong></div>
        </div>

        <div class="floating-toolbar">
          <el-select v-model="selectedStatus" clearable placeholder="全部状态" @change="applyStatusFilter">
            <el-option label="在线" value="ONLINE" />
            <el-option label="离线" value="OFFLINE" />
            <el-option label="停用" value="DISABLED" />
          </el-select>
          <el-button :icon="RefreshRight" :loading="loading" @click="loadDevices()">刷新</el-button>
        </div>

        <div v-if="displayMode !== 'all'" class="floating-categories">
          <div class="floating-section-title">
            <strong>{{ displayMode === 'group' ? '设备分组' : '地域分布' }}</strong>
            <span :title="categoryDescription()">{{ displayMode === 'region' && resolvingRegions ? '正在根据坐标识别地域…' : categoryDescription() }}</span>
          </div>
          <div class="map-category-list">
            <button type="button" :class="{ active: !selectedCategory }" @click="selectedCategory = ''">
              <span>全部</span><em>{{ locatedDevices.length }}</em>
            </button>
            <button v-for="category in mapCategories" :key="category.id" type="button" :class="{ active: selectedCategory === category.id }" @click="selectedCategory = category.id">
              <span :title="category.name">{{ category.name }}</span><em>{{ category.count }}</em>
            </button>
          </div>
        </div>

        <div class="floating-device-list">
          <div class="floating-section-title device-list-title"><strong>设备点位</strong><span>点击设备可在地图中定位</span></div>
          <div class="device-list-scroll">
            <button v-for="device in filteredDevices" :key="device.id" type="button" class="floating-device-item" @click="openDeviceInfo(device, true)">
              <span class="device-item-main">
                <strong :title="device.name">{{ device.name }}</strong>
                <small>{{ device.location || deviceRegion(device) }} · {{ deviceTypeLabel(device.deviceType) }}</small>
              </span>
              <el-tag size="small" :type="device.status === 'ONLINE' ? 'success' : device.status === 'DISABLED' ? 'info' : 'info'">{{ statusLabel(device.status) }}</el-tag>
            </button>
            <el-empty v-if="!filteredDevices.length" :image-size="54" description="当前筛选下暂无设备点位" />
          </div>
        </div>
      </aside>

      <el-button v-else class="map-panel-trigger" :icon="Expand" @click="panelCollapsed = false">设备面板 · {{ filteredDevices.length }}</el-button>
    </section>
  </div>
</template>

<style scoped>
.device-map-page { min-width: 0; }
.map-page-heading { margin-bottom: 14px; }
.map-page-heading .page-description { margin-bottom: 0; }
.map-workspace { position: relative; height: calc(100vh - 190px); min-height: 640px; overflow: hidden; border: 1px solid #dfe7f1; border-radius: 16px; background: #eef3f8; box-shadow: 0 12px 32px rgb(15 23 42 / 7%); }
.device-map { width: 100%; height: 100%; }
.map-empty { position: absolute; inset: 80px 24px 24px; z-index: 2; pointer-events: none; }
.map-error { position: absolute; top: 18px; right: 18px; left: 420px; z-index: 5; }
.map-floating-panel { position: absolute; top: 16px; bottom: 16px; left: 16px; z-index: 4; display: flex; width: 376px; min-height: 0; flex-direction: column; padding: 16px; overflow: hidden; border: 1px solid rgb(255 255 255 / 72%); border-radius: 14px; background: rgb(255 255 255 / 86%); box-shadow: 0 18px 46px rgb(15 23 42 / 16%); backdrop-filter: blur(18px) saturate(1.12); }
.floating-panel-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.floating-panel-heading > div { min-width: 0; }
.floating-panel-heading strong { display: block; color: #172033; font-size: 16px; }
.floating-panel-heading span { display: block; margin-top: 4px; color: #8a98ad; font-size: 12px; }
.floating-panel-heading :deep(.el-button) { margin-top: -6px; color: #64748b; }
.map-mode-switch { display: flex; width: 100%; margin-top: 14px; }
.map-mode-switch :deep(.el-radio-button) { flex: 1; }
.map-mode-switch :deep(.el-radio-button__inner) { width: 100%; padding-right: 10px; padding-left: 10px; }
.floating-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 7px; margin-top: 12px; }
.floating-stats > div { min-width: 0; padding: 9px 8px; border-radius: 9px; background: #f6f8fb; text-align: center; }
.floating-stats span { display: block; overflow: hidden; color: #8a98ad; text-overflow: ellipsis; white-space: nowrap; font-size: 10px; }
.floating-stats strong { display: block; margin-top: 3px; color: #1e293b; font-size: 17px; }
.floating-stats strong.online { color: #16a34a; }
.floating-toolbar { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; margin-top: 12px; }
.floating-categories { margin-top: 13px; padding-top: 12px; border-top: 1px solid #edf1f6; }
.floating-section-title { display: flex; min-width: 0; align-items: baseline; justify-content: space-between; gap: 10px; }
.floating-section-title strong { flex: 0 0 auto; color: #334155; font-size: 12px; }
.floating-section-title span { overflow: hidden; color: #94a3b8; text-overflow: ellipsis; white-space: nowrap; font-size: 10px; }
.map-category-list { display: flex; max-height: 78px; flex-wrap: wrap; gap: 6px; margin-top: 8px; overflow-y: auto; }
.map-category-list button { display: inline-flex; max-width: 210px; gap: 7px; align-items: center; padding: 5px 8px; border: 1px solid #dfe7f1; border-radius: 7px; color: #526075; background: #f8fafc; cursor: pointer; transition: border-color .16s ease, color .16s ease, background .16s ease; }
.map-category-list button:hover { border-color: #93c5fd; color: #2563eb; }
.map-category-list button.active { border-color: #93c5fd; color: #1677ff; background: #eff6ff; }
.map-category-list button span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; }
.map-category-list button em { display: inline-grid; min-width: 19px; height: 17px; padding: 0 5px; place-items: center; border-radius: 999px; color: #64748b; background: #e9eef5; font-size: 10px; font-style: normal; }
.map-category-list button.active em { color: #1677ff; background: #dbeafe; }
.floating-device-list { display: flex; min-height: 0; flex: 1; flex-direction: column; margin-top: 13px; padding-top: 12px; border-top: 1px solid #edf1f6; }
.device-list-title { margin-bottom: 8px; }
.device-list-scroll { min-height: 0; flex: 1; overflow-y: auto; overscroll-behavior: contain; }
.floating-device-item { display: flex; width: 100%; gap: 10px; align-items: flex-start; justify-content: space-between; padding: 10px; border: 0; border-radius: 9px; color: inherit; background: transparent; text-align: left; cursor: pointer; transition: background .16s ease; }
.floating-device-item + .floating-device-item { margin-top: 3px; }
.floating-device-item:hover, .floating-device-item:focus-visible { outline: none; background: #f1f6fc; }
.device-item-main { display: block; min-width: 0; flex: 1; }
.device-item-main > strong { display: block; overflow: hidden; color: #1e293b; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }
.device-item-main > small { display: block; margin-top: 4px; overflow: hidden; color: #8492a6; text-overflow: ellipsis; white-space: nowrap; font-size: 10px; }
.floating-device-item > :deep(.el-tag) { flex: 0 0 auto; }
.map-panel-trigger { position: absolute; top: 16px; left: 16px; z-index: 4; border: 1px solid rgb(226 232 240 / 95%); background: rgb(255 255 255 / 96%); box-shadow: 0 10px 28px rgb(15 23 42 / 16%); backdrop-filter: blur(10px); }
:deep(.amap-device-marker) { display: flex; gap: 7px; align-items: center; padding: 7px 10px; border: 1px solid rgb(255 255 255 / 86%); border-radius: 999px; color: #172033; background: rgb(255 255 255 / 94%); box-shadow: 0 10px 26px rgb(15 23 42 / 18%); white-space: nowrap; }
:deep(.amap-device-marker span) { width: 10px; height: 10px; border-radius: 50%; background: #9ca3af; box-shadow: 0 0 0 4px rgb(156 163 175 / 16%); }
:deep(.amap-device-marker.online span) { background: #22c55e; box-shadow: 0 0 0 4px rgb(34 197 94 / 16%); }
:deep(.amap-device-marker.disabled span) { background: #94a3b8; box-shadow: 0 0 0 4px rgb(148 163 184 / 16%); }
:deep(.amap-device-marker strong) { font-size: 12px; font-weight: 700; }
:deep(.amap-device-info) { min-width: 210px; color: #172033; line-height: 1.6; }
:deep(.amap-device-info strong) { display: block; margin-bottom: 6px; font-size: 15px; }
:deep(.amap-device-info p) { margin: 0; color: #64748b; font-size: 12px; }
:deep(.amap-device-info button) { margin-top: 10px; padding: 6px 10px; border: 0; border-radius: 8px; color: #fff; background: #2563eb; cursor: pointer; }
@media (max-width: 760px) {
  .map-workspace { height: calc(100dvh - 160px); min-height: 600px; }
  .map-floating-panel { top: auto; right: 10px; bottom: 10px; left: 10px; width: auto; max-height: 58%; padding: 13px; }
  .map-panel-trigger { top: auto; bottom: 12px; left: 12px; }
  .map-error { top: 12px; right: 12px; left: 12px; }
  .floating-stats { gap: 5px; }
  .floating-stats > div { padding: 7px 5px; }
}
</style>

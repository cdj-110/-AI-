<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import { Bell, Connection, OfficeBuilding, TrendCharts } from '@element-plus/icons-vue';
import * as echarts from 'echarts';
import { apiRequest } from '../api/request';

interface Summary {
  tenantCount: number;
  deviceCount: number;
  onlineDeviceCount: number;
  offlineDeviceCount: number;
  alarmCount: number;
}

const summary = ref<Summary>({ tenantCount: 0, deviceCount: 0, onlineDeviceCount: 0, offlineDeviceCount: 0, alarmCount: 0 });
const statusChartRef = ref<HTMLDivElement>();
const trendChartRef = ref<HTMLDivElement>();
let statusChart: echarts.ECharts | undefined;
let trendChart: echarts.ECharts | undefined;
const cards = computed(() => [
  { label: '租户总数', value: summary.value.tenantCount, icon: OfficeBuilding, tone: 'blue' },
  { label: '设备总数', value: summary.value.deviceCount, icon: TrendCharts, tone: 'violet' },
  { label: '在线设备', value: summary.value.onlineDeviceCount, icon: Connection, tone: 'green' },
  { label: '告警数量', value: summary.value.alarmCount, icon: Bell, tone: 'orange' },
]);

function renderCharts(points: Array<{ time: string; deviceKey: string; value: number | null }>) {
  if (!statusChartRef.value || !trendChartRef.value) return;
  statusChart = echarts.init(statusChartRef.value);
  trendChart = echarts.init(trendChartRef.value);
  statusChart.setOption({
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, textStyle: { color: '#697386' } },
    series: [{
      type: 'pie',
      radius: ['50%', '72%'],
      center: ['50%', '43%'],
      label: { show: false },
      data: [
        { value: summary.value.onlineDeviceCount, name: '在线', itemStyle: { color: '#22c55e' } },
        { value: summary.value.offlineDeviceCount, name: '离线', itemStyle: { color: '#cbd5e1' } },
      ],
    }],
  });
  trendChart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 40, right: 18, top: 22, bottom: 28 },
    xAxis: { type: 'category', boundaryGap: false, data: points.map((point) => new Date(point.time).toLocaleTimeString()), axisLabel: { color: '#94a3b8' } },
    yAxis: { type: 'value', name: '°C', axisLabel: { color: '#94a3b8' }, splitLine: { lineStyle: { color: '#edf1f5' } } },
    series: [{ type: 'line', smooth: true, symbol: 'circle', data: points.map((point) => point.value), lineStyle: { color: '#2563eb', width: 3 }, itemStyle: { color: '#2563eb' }, areaStyle: { color: 'rgba(37, 99, 235, 0.08)' } }],
  });
}

function resizeCharts() {
  statusChart?.resize();
  trendChart?.resize();
}

onMounted(async () => {
  const [summaryData, trendData] = await Promise.all([
    apiRequest<Summary>({ url: '/api/dashboard/summary', method: 'GET' }),
    apiRequest<{ temperature: Array<{ time: string; deviceKey: string; value: number | null }> }>({ url: '/api/dashboard/trends', method: 'GET' }),
  ]);
  summary.value = summaryData;
  await nextTick();
  renderCharts(trendData.temperature);
  window.addEventListener('resize', resizeCharts);
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCharts);
  statusChart?.dispose();
  trendChart?.dispose();
});
</script>

<template>
  <div>
    <div class="page-heading">
      <div>
        <h1>数据概览</h1>
        <p>查看平台当前运行情况</p>
      </div>
      <span class="status"><i /> 服务运行正常</span>
    </div>
    <div class="stats">
      <article v-for="card in cards" :key="card.label" class="stat-card">
        <div :class="['stat-icon', card.tone]"><el-icon><component :is="card.icon" /></el-icon></div>
        <div><p>{{ card.label }}</p><strong>{{ card.value }}</strong></div>
      </article>
    </div>
    <div class="charts">
      <section class="panel">
        <h2>设备状态</h2>
        <div ref="statusChartRef" class="chart" />
      </section>
      <section class="panel">
        <h2>温度趋势</h2>
        <div ref="trendChartRef" class="chart" />
      </section>
    </div>
  </div>
</template>

<style scoped>
.page-heading { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
h1 { margin: 0; color: #111827; font-size: 25px; } .page-heading p { margin: 8px 0 0; color: #8b96a8; font-size: 14px; }
.status { color: #4b5563; font-size: 13px; } .status i { display: inline-block; width: 8px; height: 8px; margin-right: 7px; border-radius: 50%; background: #22c55e; }
.stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; }
.stat-card { display: flex; gap: 14px; align-items: center; padding: 20px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.stat-card p { margin: 0 0 8px; color: #8b96a8; font-size: 13px; } .stat-card strong { color: #111827; font-size: 26px; }
.stat-icon { display: grid; width: 42px; height: 42px; place-items: center; border-radius: 11px; font-size: 19px; }
.blue { color: #2563eb; background: #eff6ff; } .violet { color: #7c3aed; background: #f5f3ff; } .green { color: #16a34a; background: #f0fdf4; } .orange { color: #ea580c; background: #fff7ed; }
.charts { display: grid; grid-template-columns: minmax(280px, 0.75fr) minmax(0, 1.6fr); gap: 18px; }
.panel { margin-top: 18px; padding: 22px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; } h2 { margin: 0; font-size: 16px; }
.chart { height: 270px; }
@media (max-width: 1000px) { .stats { grid-template-columns: repeat(2, 1fr); } .charts { grid-template-columns: 1fr; } } @media (max-width: 540px) { .stats { grid-template-columns: 1fr; } }
</style>

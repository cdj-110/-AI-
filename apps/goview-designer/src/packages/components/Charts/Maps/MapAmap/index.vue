<template>
  <div ref="vChartRef" style="width:100%;height:100%"></div>
</template>
<script setup lang="ts">
import { ref, PropType, toRefs, watch, onBeforeUnmount } from 'vue'
import AMapLoader from '@amap/amap-jsapi-loader'
import { CreateComponentType } from '@/packages/index.d'
import { useChartDataFetch } from '@/hooks'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { MarkerEnum, ThemeEnum } from './config'
import { isArray } from '@/utils'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  }
})

let {
  amapKey,
  amapStyleKey,
  amapLon,
  amapLat,
  amapZindex,
  mapMarkerType,
  lang,
  amapStyleKeyCustom,
  features,
  viewMode,
  showLabel,
  pitch,
  skyColor,
  marker,
  satelliteTileLayer,
  roadNetTileLayer,
  trafficTileLayer
} = toRefs(props.chartConfig.option.mapOptions)

let mapIns: any = null
let markers: any[] = []
let AMapIns: any = null
// 全局信息窗体实例，复用避免多弹窗
let infoWindow: any = null
const vChartRef = ref<HTMLElement>()

/**
 * 初始化地图
 */
const initMap = async () => {
  if (!vChartRef.value) return
  // 销毁旧地图实例，防止重复创建
  if (mapIns) {
    mapIns.destroy()
    mapIns = null
    markers = []
  }

  try {
    AMapIns = await AMapLoader.load({
      key: amapKey.value,
      version: '1.4.15',
      plugins: ['AMap.PlaceSearch', 'AMap.AutoComplete', 'AMap.InfoWindow']
    })

    mapIns = new AMapIns.Map(vChartRef.value, {
      resizeEnable: true,
      zoom: amapZindex.value,
      center: [amapLon.value, amapLat.value],
      lang: lang.value,
      features: features.value,
      pitch: pitch.value,
      skyColor: skyColor.value,
      viewMode: viewMode.value,
      showLabel: showLabel.value,
      willReadFrequently: true
    })

    // 初始化全局信息窗体（高德原生弹窗）
    infoWindow = new AMapIns.InfoWindow({
      offset: new AMapIns.Pixel(0, -30),
      // 自定义弹窗样式可在这里配置
      isCustom: false
    })

    // 图层处理
    const satelliteLayer = new AMapIns.TileLayer.Satellite({
      zIndex: satelliteTileLayer.value.zIndex,
      opacity: satelliteTileLayer.value.opacity,
      zooms: satelliteTileLayer.value.zooms
    })
    const roadNetLayer = new AMapIns.TileLayer.RoadNet({
      zIndex: roadNetTileLayer.value.zIndex,
      opacity: roadNetTileLayer.value.opacity,
      zooms: roadNetTileLayer.value.zooms
    })
    const trafficLayer = new AMapIns.TileLayer.Traffic({
      zIndex: trafficTileLayer.value.zIndex,
      opacity: trafficTileLayer.value.opacity,
      zooms: trafficTileLayer.value.zooms
    })

    mapIns.remove([satelliteLayer, roadNetLayer, trafficLayer])
    satelliteTileLayer.value.show && mapIns.add(satelliteLayer)
    roadNetTileLayer.value.show && mapIns.add(roadNetLayer)
    trafficTileLayer.value.show && mapIns.add(trafficLayer)

    // 地图样式
    const styleId = amapStyleKeyCustom.value || amapStyleKey.value
    mapIns.setMapStyle(`amap://styles/${styleId}`)

    // 渲染标记数据
    dataHandle(props.chartConfig.option.dataset)
  } catch (e) {
    console.error('地图初始化失败', e)
  }
}

/**
 * 渲染标记 + 绑定点击弹窗（核心优化点击灵敏度）
 */
const dataHandle = (newData: any) => {
  if (!mapIns || !AMapIns) {
    initMap()
    return
  }
  if (!isArray(newData?.markers)) return

  // 清空旧标记
  mapIns.remove(markers)
  markers = []

  newData.markers.forEach((markerItem: any) => {
    let markerInstance: any = null

    if (mapMarkerType.value === MarkerEnum.MARKER) {
      markerInstance = new AMapIns.Marker({
        position: markerItem.position,
        offset: new AMapIns.Pixel(-13, -30),
        content:"12313",
        clickable: true,
        bubble: true,
        extData: markerItem
      })
    } else if (mapMarkerType.value === MarkerEnum.CIRCLE_MARKER) {
      // 圆点标记：增大radius提升点击面积
      markerInstance = new AMapIns.CircleMarker({
        center: markerItem.position,
        radius: Math.max(markerItem.value || 10, 10),
        ...marker.value,
        clickable: true,
        bubble: true,
        extData: markerItem
      })
    }

    if (!markerInstance) return
    markers.push(markerInstance)
    markerInstance.setMap(mapIns)

    markerInstance.on('click', (e: any) => {
      const pointData = markerInstance.getExtData()
      console.log(pointData)
      // 组装弹窗HTML，自定义详情内容
      const content = `
        <div style="padding: 10px;width: max-content;">
          <h4 style="margin:0 0 10px;">点位详情</h4>
          <p>设备名称：${pointData.dev_name}</p>
          <p>设备模型：${pointData.dev_model_name}</p>
          <p>在线状态：${pointData.dev_status === 0 ? "离线" : "在线"}</p>
          <p>设备唯一码：${pointData.dev_sn}</p>
          <p>创建时间：${pointData.create_time}</p>
          <p>详细地址：${pointData.install_location}</p>
        </div>
      `
      infoWindow.setContent(content)
      infoWindow.open(mapIns, markerInstance.getPosition())
    })
  })
}

// 监听地图配置变化
const stopWatchConfig = watch(
  () => props.chartConfig.option.mapOptions,
  () => initMap(),
  { immediate: true, deep: true }
)

// 监听点位数据变化
watch(
  () => props.chartConfig.option.dataset,
  (newData) => {
    try {
      dataHandle(newData)
    } catch (err) {
      console.error('点位渲染异常', err)
    }
  },
  { deep: true }
)

// 图表数据刷新回调
useChartDataFetch(props.chartConfig, useChartEditStore, (newData: any) => {
  stopWatchConfig()
  dataHandle(newData)
})

// 组件销毁销毁地图、解绑监听，释放内存
onBeforeUnmount(() => {
  stopWatchConfig()
  if (mapIns) {
    mapIns.destroy()
    mapIns = null
  }
})
</script>

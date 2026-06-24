<template>
    <div>
        <el-dialog v-model="props.alterConfig.isTrue" title="修改设备信息" width="550">
            <div style="display: flex; justify-content: center;margin: 0 2rem;">
                <el-form :model="form" label-width="auto">
                    <el-form-item label="设备名称">
                        <el-input v-model="form.dev_name" placeholder="请输入设备名称" />
                    </el-form-item>
                    <el-form-item label="设备模型">
                        <el-input v-model="form.device_type" disabled placeholder="请输入设备模型" />
                    </el-form-item>
                    <el-form-item label="定位方式">
                        <el-select v-model="form.location_method" placeholder="请选择定位方式">
                            <el-option label="手动定位" :value="0" />
                        </el-select>
                    </el-form-item>
                    <div style="display: flex;align-items: center;justify-content: space-between;">
                        <el-form-item label="经度" style="width: 50%;">
                            <el-input v-model="form.lng" placeholder="请输入经度" @click="geoTrue = true;" />
                        </el-form-item>
                        <el-form-item label="纬度" style="width: 50%;">
                            <el-input v-model="form.lat" placeholder="请输入纬度" @click="geoTrue = true;" />
                        </el-form-item>
                    </div>
                    <el-form-item label="设备描述">
                        <el-input type="textarea" :rows="5" v-model="form.dev_desc" placeholder="请输入设备描述" maxlength="50"
                            show-word-limit />
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <!-- props.alterConfig.isTrue = false; -->
                <el-button type="danger" @click="dev_dele">删除设备</el-button>
                <el-button type="primary" @click="saveAlterChange">保存修改</el-button>
            </template>
        </el-dialog>

        <el-dialog v-model="geoTrue" width="70vw" @open="openChange" :show-close="false" :close-on-press-escape="false"
            :close-on-click-modal="false" draggable align-center>
            <div style="width: 100%;height: 70vh;">
                <el-form :inline="true" :model="formInline">
                    <el-form-item label="经度" style="width: 15%;">
                        <el-input v-model="formInline.log" placeholder="请输入经度" clearable />
                    </el-form-item>
                    <el-form-item label="纬度" style="width: 15%;margin-right: 1rem;">
                        <el-input v-model="formInline.lat" placeholder="请输入纬度" clearable />
                    </el-form-item>
                    <img src="@/assets/dw.png" @click="geoRefer"
                        style="cursor: pointer;width: 1.5rem;margin-right: 1rem;">
                    <el-form-item label="地址" style="width: 40%;">
                        <el-input v-model="formInline.local" placeholder="请输入经纬度或者选择地址" disabled clearable />
                    </el-form-item>
                    <el-form-item style="text-align: right;">
                        <el-button type="info" @click="geoTrue = false;">取消</el-button>
                        <el-button type="primary" @click="geoLatLngChange">确认</el-button>
                    </el-form-item>
                </el-form>
                <div ref="geoRef" style="height: 93%;width: 100%;background: #eeeeee;" @wheel="handleMapWheel">
                </div>
            </div>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { geoInit } from "@/utils/publicFun";
import { ElSelect } from "element-plus";
import { update_device_detail, soft_delete_device } from "@/api/facilityList/index";
import useCounterStore from "@/stores/counter";
import { useRouter } from 'vue-router'
const router = useRouter()
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let props = withDefaults(defineProps<{
    alterConfig: {
        isTrue: Boolean,
        list: Object
    }
}>(), {
    alterConfig: () => ({
        isTrue: false,
        list: {}
    })
});
let emit = defineEmits(["refaer"])
let form: any = ref({ dev_name: "", device_type: "", location_method: 0, lng: "", lat: "", dev_desc: "", install_location: "", province: "", city: "", district: "", township: "", street: "", adcode: "" });//修改详情
let geoTrue = ref(false);//打开地图
let formInline = ref({ log: "", lat: "", local: "", province: "", city: "", district: "", township: "", street: "", adcode: "" });//地图搜索
let geoRef = ref("");//地图Ref
let mapVer: any = undefined;

watch(props.alterConfig, (val: any) => {
    if (val.isTrue == true) {
        form.value = JSON.parse(JSON.stringify(val.list));
        form.value.device_type = val.list.device_type == "GW" ? "网关" : val.list.device_type == "GSD" ? "网关子设备" : val.list.device_type == "DD" ? "直连设备" : "无";
        form.value.location_method = 0;
    }
}, { immediate: true, deep: true });

/**
 * 获取经纬度和地址之后，给表单赋值
 */
let geoLatLngChange = () => {
    if (!formInline.value.local || !formInline.value.log || !formInline.value.lat) {
        ElMessage.error("请输入经纬度并点击标点！");
        return false;
    }
    form.value.lng = formInline.value.log;
    form.value.lat = formInline.value.lat;
    form.value.install_location = formInline.value.local;
    form.value.province = formInline.value.province;
    form.value.city = formInline.value.city;
    form.value.district = formInline.value.district;
    form.value.township = formInline.value.township;
    form.value.street = formInline.value.street;
    form.value.adcode = formInline.value.adcode;
    geoTrue.value = false;
}

/**
 * 保存修改
 */
let saveAlterChange = () => {
    let params = {
        user_id: user_id.value,
        dev_id: form.value.dev_id,
        dev_name: form.value.dev_name,
        location_method: form.value.location_method,
        coord: `${form.value.lng},${form.value.lat}`,
        install_location: form.value.install_location,
        province: form.value.province,
        city: form.value.city ? form.value.city : form.value.province,
        district: form.value.district,
        township: form.value.township,
        street: form.value.street,
        adcode: form.value.adcode,
        dev_desc: form.value.dev_desc
    }
    update_device_detail(params).then((res: any) => {
        if (res.code == 200) {
            props.alterConfig.isTrue = false;
            ElMessage.success("修改成功！");
            emit("refaer");
        }
    })
}

/**
 * 当经纬度输入框改变后重新标点
 */
let geoRefer = () => {
    if (!formInline.value.lat || !formInline.value.log) {
        ElMessage.error("经纬度为空无法确认地址！");
        return false;
    }
    mapVer.map.clearMap();
    poiGetChange(mapVer.AMap, mapVer.map, [formInline.value.log, formInline.value.lat]).then((res: any) => {
        formInline.value = {
            log: res.log,
            lat: res.lat,
            local: res.regeocode.formattedAddress,
            province: res.regeocode.addressComponent.province,
            city: res.regeocode.addressComponent.city ? res.regeocode.addressComponent.city : res.regeocode.addressComponent.province,
            district: res.regeocode.addressComponent.district,
            township: res.regeocode.addressComponent.township,
            street: res.regeocode.addressComponent.street,
            adcode: res.regeocode.addressComponent.adcode,
        };
    })
}

/**
 * 地图弹窗打开时触发
 */
let openChange = () => {
    nextTick(() => {
        geoInit({ dom: geoRef.value, zoom: 12 }).then((res: any) => {
            mapVer = res.map;
            mapVer.map.on("click", (e: any) => {
                mapVer.map.clearMap();
                poiGetChange(mapVer.AMap, mapVer.map, [e.lnglat.getLng(), e.lnglat.getLat()]).then((res: any) => {
                    formInline.value = {
                        log: res.log,
                        lat: res.lat,
                        local: res.regeocode.formattedAddress,
                        province: res.regeocode.addressComponent.province,
                        city: res.regeocode.addressComponent.city ? res.regeocode.addressComponent.city : res.regeocode.addressComponent.province,
                        district: res.regeocode.addressComponent.district,
                        township: res.regeocode.addressComponent.township,
                        street: res.regeocode.addressComponent.street,
                        adcode: res.regeocode.addressComponent.adcode,
                    };
                })
            })
        })
    });
}
/**
 * 删除设备
 */
let dev_dele = () => {
     ElMessageBox.confirm("是否删除该设备", "提示", {
        confirmButtonText: "确认",
        cancelButtonText: "取消",
        type: "warning",
    }).then(() => {
    soft_delete_device({ device_id: form.value.dev_id, user_id: user_id.value }).then((res: any) => {
        if (res.code == 200) {
            router.push('/facilityList');
            ElMessage.success(res.msg);
        } else {
            ElMessage.error(res.msg);
        }
    })
    })

}

/**
 * 通过经纬度标点，传经纬度就显示经纬度的标点，不传就点击标点
 */
let poiGetChange = (AMap: any, map: any, arr: Array<string>) => {
    return new Promise(resolve => {
        let marker = new AMap.Marker({
            position: new AMap.LngLat(arr[0], arr[1])
        });
        map.add(marker);
        let geocoder = new AMap.Geocoder({ radius: 1000, extensions: "all" });
        geocoder.getAddress([arr[0], arr[1]], (status: any, result: any) => {
            if (status === "complete" && result.regeocode) {
                resolve({ log: arr[0], lat: arr[1], regeocode: result.regeocode });
            } else {
                ElMessage.error("获取详细地址失败！");
            }
        });
    })
}

/**
 * 只做一件事：按住Ctrl+滚轮时，禁用浏览器页面缩放
 * 不按Ctrl时，地图完全正常缩放
 */
const handleMapWheel = (e: WheelEvent) => {
    // 只有按住Ctrl键的时候，才阻止浏览器的默认缩放行为
    if (e.ctrlKey) {
        e.preventDefault();
        // 可选：阻止事件冒泡，防止父元素也触发
        e.stopPropagation();
    }
    // 其他情况什么都不做，让高德地图自己处理滚轮缩放
};
</script>
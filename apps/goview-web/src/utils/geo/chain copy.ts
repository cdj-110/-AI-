// import beijing from "./beijing.json";
// import tianjing from "./tianjing.json";
// import hebei from "./hebei.json";
// import sanxi from "./sanxi.json";
// import neiMengGu from "./neiMengGu.json";
// import liaoning from "./liaoning.json";
// import jilin from "./jiLin.json";
// import heiLongJIang from "./heiLongJIang.json";
// import shanghai from "./shanghai.json";
// import jiangshu from "./jiangshu.json";
// import zhejiang from "./zhejiang.json";
// import anhui from "./anhui.json";
// import fujian from "./fujian.json";
// import jiangxi from "./jiangxi.json";
// import shandong from "./shandong.json";
// import henan from "./henan.json";
// import hubei from "./hubei.json";
// import hunan from "./hunan.json";
// import guangdong from "./guangdong.json";
// import guangxi from "./guangxi.json";
// import hainan from "./hainan.json";
// import chongqing from "./chongqing.json";
// import shichuan from "./shichuan.json";
// import guizhou from "./guizhou.json";
// import yunnan from "./yunnan.json";
// import xizhang from "./xizhang.json";
// import shanxi from "./shanxi.json";
// import ganshu from "./ganshu.json";
// import qinghai from "./qinghai.json";
// import ningxia from "./ningxia.json";
// import xinjiang from "./xinjiang.json";
// import taiwan from "./taiwan.json";
// import xianggang from "./xianggang.json";
// import aomen from "./aomen.json";
// import chain from "./chain.json";

/**
 * 获取省市区详细坐标
 * @inner(e) 传入对应的区号
 */
export function geoLoactionVer(e?: number) {
    let area = ref([
        { adcode: 110000, name: "北京市", value:"123", url: import("./beijing.json") },
        { adcode: 120000, name: "天津市", value:"123", url: import("./tianjing.json") },
        { adcode: 130000, name: "河北省", value:"123", url: import("./hebei.json") },
        { adcode: 140000, name: "山西省", value:"123", url: import("./sanxi.json") },
        { adcode: 150000, name: "内蒙古自治区", value:"123", url: import("./neiMengGu.json") },
        { adcode: 210000, name: "辽宁省", value:"123", url: import("./liaoning.json") },
        { adcode: 220000, name: "吉林省", value:"123", url: import("./jiLin.json") },
        { adcode: 230000, name: "黑龙江省", value:"123", url: import("./heiLongJIang.json") },
        { adcode: 310000, name: "上海市", value:"123", url: import("./shanghai.json") },
        { adcode: 320000, name: "江苏省", value:"123", url: import("./jiangshu.json") },
        { adcode: 330000, name: "浙江省", value:"123", url: import("./zhejiang.json") },
        { adcode: 340000, name: "安徽省", value:"123", url: import("./anhui.json") },
        { adcode: 350000, name: "福建省", value:"123", url: import("./fujian.json") },
        { adcode: 360000, name: "江西省", value:"123", url: import("./jiangxi.json") },
        { adcode: 370000, name: "山东省", value:"123", url: import("./shandong.json") },
        { adcode: 410000, name: "河南省", value:"123", url: import("./henan.json") },
        { adcode: 420000, name: "湖北省", value:"123", url: import("./hubei.json") },
        { adcode: 430000, name: "湖南省", value:"123", url: import("./hunan.json") },
        { adcode: 440000, name: "广东省", value:"123", url: import("./guangdong.json") },
        { adcode: 450000, name: "广西壮族自治区", value:"123", url: import("./guangxi.json") },
        { adcode: 460000, name: "海南省", value:"123", url: import("./hainan.json") },
        { adcode: 500000, name: "重庆市", value:"123", url: import("./chongqing.json") },
        { adcode: 510000, name: "四川省", value:"123", url: import("./shichuan.json") },
        { adcode: 520000, name: "贵州省", value:"123", url: import("./guizhou.json") },
        { adcode: 530000, name: "云南省", value:"123", url: import("./yunnan.json") },
        { adcode: 540000, name: "西藏自治区", value:"123", url: import("./xizhang.json") },
        { adcode: 610000, name: "陕西省", value:"123", url: import("./shanxi.json") },
        { adcode: 620000, name: "甘肃省", value:"123", url: import("./ganshu.json") },
        { adcode: 630000, name: "青海省", value:"123", url: import("./qinghai.json") },
        { adcode: 640000, name: "宁夏回族自治区", value:"123", url: import("./ningxia.json") },
        { adcode: 650000, name: "新疆维吾尔自治区", value:"123", url: import("./xinjiang.json") },
        { adcode: 710000, name: "台湾省", value:"123", url: import("./taiwan.json") },
        { adcode: 810000, name: "香港特别行政区", value:"123", url: import("./xianggang.json") },
        { adcode: 820000, name: "澳门特别行政区", value:"123", url: import("./aomen.json") }
    ])

    if(!e){
        return {
            arr:area.value,
            res:import("./chain.json").then( (res:any) => res.default)
        };
    }else{
        return {
            arr:[],
            res:area.value.filter(item => item.adcode == e)[0].url.then( res => res.default)
        }
    }
}
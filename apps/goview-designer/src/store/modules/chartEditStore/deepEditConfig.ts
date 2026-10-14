import { defaultTheme, globalThemeJson } from '@/settings/chartThemes/index';
import { requestInterval, previewScaleType, requestIntervalUnit } from '@/settings/designSetting'

export let copyEditConfig = {
    editCanvasConfig: {
        // 项目名称
        projectName: undefined,
        // 默认宽度
        width: 1920,
        // 默认高度
        height: 1080,
        // 启用滤镜
        filterShow: false,
        // 色相
        hueRotate: 0,
        // 饱和度
        saturate: 1,
        // 对比度
        contrast: 1,
        // 亮度
        brightness: 1,
        // 透明度
        opacity: 1,
        // 变换（暂不更改）
        rotateZ: 0,
        rotateX: 0,
        rotateY: 0,
        skewX: 0,
        skewY: 0,
        // 混合模式
        blendMode: 'normal',
        // 默认背景色
        background: undefined,
        backgroundImage: undefined,
        // 是否使用纯颜色
        selectColor: true,
        // chart 主题色
        chartThemeColor: defaultTheme || 'dark',
        // 自定义颜色列表
        chartCustomThemeColorInfo: undefined,
        // 全局配置
        chartThemeSetting: globalThemeJson,
        // vChart 主题
        vChartThemeName: 'vScreenVolcanoBlue',
        // 适配方式
        previewScaleType: previewScaleType
    },
    requestGlobalConfig: {
        requestDataPond: [],
        requestOriginUrl: '',
        requestInterval: requestInterval,
        requestIntervalUnit: requestIntervalUnit,
        requestParams: {
            Body: {
                'form-data': {},
                'x-www-form-urlencoded': {},
                json: '',
                xml: ''
            },
            Header: {},
            Params: {}
        }
    },
}

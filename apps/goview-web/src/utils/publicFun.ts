import AMapLoader from "@amap/amap-jsapi-loader";

/**
 * 公共变量合集
 */
export let publicAvow = Object.freeze({
    key: "bfd40e7b71c4094a2e777651bd18f2af",//查询高德地图
    // weartherKey: "6857a2dd2057c1276fc8185c5b4d529a",//查询高德天气
});

/**
 * 深拷贝
 * @inner(obj) 要拷贝的数据
 * @returns { any } 返回拷贝之后的数据
 */
export let deepClone = (obj: any): any => {
    let cloneObj: any = Array.isArray(obj) ? [] : {};
    for (let k in obj) {
        if (obj.hasOwnProperty(k)) {
            if (typeof obj[k] === "object") {
                cloneObj[k] = deepClone(obj[k]);
            } else {
                cloneObj[k] = obj[k];
            }
        }
    }
    return cloneObj;
}

/**
 * 下载图片
 * @inner(verUrl) 要下载的图片链接
 * @inner(name) 下载后的图片名称
 * @returns { Promise<string> } 返回下载状态
 */
export let downloadImage = (verUrl: string, name: string): Promise<string> => {
    return new Promise(resolve => {
        let image = new Image();//创建一个图片对象
        image.setAttribute("crossOrigin", "anonymous");//解决图片下载跨域污染
        image.src = verUrl;//将下载路径保存到图片对象上
        image.onload = function () {
            let canvas = document.createElement("canvas");//创建画布用来保存图片
            canvas.width = image.width;//设置画布宽度
            canvas.height = image.height;//设置画布高度
            let context: any = canvas.getContext("2d");//设置画笔
            context.drawImage(image, 0, 0, image.width, image.height);//将图片对象绘画到画布上
            let url = canvas.toDataURL("image/png"); //得到图片的base64编码数据
            let a = document.createElement("a"); // 生成一个a元素
            a.download = name || "下载.jpg"; // 设置图片名称
            a.href = url; // 将生成的URL设置为a.href属性
            a.click();//触发单击事件
        };
        resolve("下载成功！");
    })
}

/**
 * 将时间戳转换为时间
 * @inner(Timestamp) 要转换的时间戳
 * @inner(type) 要返回的时间类型 (年- 月 - 日) or (年 - 月 - 日 时 : 分 : 秒)
 * @returns { string } 转换后的时间
 */
export let itializeUtc = (Timestamp: string, type?: string): string => {
    let now = new Date(Timestamp),
        y = now.getFullYear(),
        m = now.getMonth() + 1,
        d = now.getDate();
    if (type == "yyyy-MM-dd" || type == "yyyy-mm-dd") {
        return y + "-" + (m < 10 ? "0" + m : m) + "-" + (d < 10 ? "0" + d : d);
    } else {
        return y + "-" + (m < 10 ? "0" + m : m) + "-" + (d < 10 ? "0" + d : d) + " " + now.toTimeString().substr(0, 8);
    }
}

/**
 * 获取指定日期
 * @function(today) 获取今天的日期
 * @function(tomorrow) 获取明天的日期数组
 * @function(yesterday) 获取昨天的日期数组
 * @function(thisWeek) 获取这周的日期数组
 * @function(thisMonth) 获取这个月的日期数组
 * @function(lastMonth) 获取上个月的日期数组
 * @function(thisYear) 获取今年的日期数组
 * @returns { Array } [返回指定的开始日期, 返回指定的结束日期]
 */
export let itializeWeek = {
    /**
     * 直接调用此函数可获取 (今天) 的日期数组
     */
    today: () => {
        let start = new Date();
        start.setHours(0, 0, 0, 0);
        let end = new Date();
        end.setHours(23, 59, 59, 999);
        return [start, end];
    },

    /**
     * 直接调用此函数可获取 (明天) 的日期数组
     */
    tomorrow: () => {
        let start = new Date();
        start.setDate(start.getDate() + 1);
        start.setHours(0, 0, 0, 0);
        let end = new Date(start);
        end.setHours(23, 59, 59, 999);
        return [start, end];
    },

    /**
     * 直接调用此函数可获取 (昨天) 的日期数组
     */
    yesterday: () => {
        let start = new Date();
        start.setDate(start.getDate() - 1);
        start.setHours(0, 0, 0, 0);
        let end = new Date(start);
        end.setHours(23, 59, 59, 999);
        return [start, end];
    },

    /**
     * 直接调用此函数可获取 [这周 {到今天}] 的日期数组
     */
    thisWeek: () => {
        let start = new Date();
        let end = new Date();
        start.setDate(start.getDate() - start.getDay()); // 设置为本周的开始时间（周日）
        start.setHours(0, 0, 0, 0);
        end.setDate(end.getDate() + (6 - end.getDay())); // 设置为本周的结束时间（周六）
        end.setHours(23, 59, 59, 999);
        return [start, end];
    },

    /**
     * 直接调用此函数可获取 [这个月 {到今天}] 的日期数组
     */
    thisMonth: () => {
        let start = new Date();
        start.setDate(1);
        start.setHours(0, 0, 0, 0);
        let end = new Date(start);
        end.setMonth(end.getMonth() + 1);
        end.setDate(0);
        end.setHours(23, 59, 59, 999);
        return [start, end];
    },

    /**
     * 直接调用此函数可获取 [上个月] 的日期数组
     */
    lastMonth: () => {
        let start = new Date();
        start.setMonth(start.getMonth() - 1);
        start.setDate(1);
        start.setHours(0, 0, 0, 0);
        let end = new Date(start);
        end.setMonth(end.getMonth() + 1);
        end.setDate(0);
        end.setHours(23, 59, 59, 999);
        return [start, end];
    },

    /**
     * 直接调用此函数可获取 [今年 {到今天}] 的日期数组
     */
    thisYear: () => {
        let start = new Date();
        start.setMonth(0, 1);
        start.setHours(0, 0, 0, 0);
        let end = new Date();
        end.setMonth(11, 31);
        end.setHours(23, 59, 59, 999);
        return [start, end];
    }
}

/**
 * 将对象数组中相同keyVal的对象合并，合并后的结构是一个以keyVal为键的对象
 * @inner(arr) 要合并的数组
 * @inner(keyVal) 以什么字段为合并条件
 * @returns { Array<object> } 返回合并后的数组
 */
export let conflateFun = (arr: any, keyVal: string) => {
    return arr.reduce((acc: any, current: any) => {
        let key = current[keyVal];
        if (!acc[key]) {
            acc[key] = [];
        }
        acc[key].push(current);
        return acc;
    }, {});
};

/**
 * 地图初始化，此函数主要目的是地图key归一化
 * @inner(dom) 要渲染地图的dom元素
 * @inner(center) 地图的中心点坐标
 * @inner(zoom) 地图缩放倍数
 * @returns { Promise<{errText:"描述信息",map:"地图实例",err:"错误信息"}> } 返回地图实例
 */
export let geoInit = ({ dom, center, zoom }: { dom: any, center?: Array<number>, zoom?: number }): Promise<{ errText: string, map: object, err: string }> => {
    return new Promise((resolve, reject) => {
        AMapLoader.load({
            key: publicAvow.key,
            version: "2.0",
            plugins: ["AMap.Map", "AMap.TileLayer", "AMap.Autocomplete", "AMap.Geocoder"], // 引入图层插件
            AMapUI: { version: "1.1", plugins: [] },//使用AMapUI版本
            Loca: { version: '2.0' },//使用Loca地图版本
        }).then(AMap => {
            let traffic = new AMap.TileLayer.Traffic({
                autoRefresh: true, //路况是否自动刷新
                interval: 10, //刷新间隔
            });
            let map = new AMap.Map(dom, {
                zoom: zoom ? zoom : 7,
                center: center && Array.isArray(center) ? center : [116.397428, 39.90923],
                // 👇 关键配置：只保留滚轮缩放，禁用其他缩放方式
                zoomEnable: true,        // 允许地图缩放
                scrollWheel: true,       // 开启滚轮缩放
                doubleClickZoom: false,  // 禁用双击缩放（可选，看你需求）
                keyboardEnable: false,   // 禁用键盘 +/- 缩放
            });
            map.add(traffic);
            resolve({ errText: "地图渲染成功！", map: { AMap, map }, err: "" });
        }).catch(err => {
            reject({ errText: "地图初始化出错！", map: {}, err: err });
        })
    })
}
/**
 * 元素全屏操作
 * @inner(element) 要全屏的元素
 * @returns { Promise<{code:number, type: string}> } 返回状态,请使用type字段判断当前是全屏(full)还是退出全屏(exit)！
 */
export let openFullScreen = (element: any): Promise<{ code: number, type: string }> => {
    return new Promise(resolve => {
        let el: any = document;
        if (
            el.fullscreenElement === element ||
            el.webkitFullscreenElement === element ||
            el.mozFullscreenElement === element ||
            el.msFullscreenElement === element
        ) {
            if (el.exitFullscreen) {
                el.exitFullscreen();
            } else if (el.mozCancelFullScreen) {
                el.mozCancelFullScreen();
            } else if (el.webkitExitFullscreen) {
                el.webkitExitFullscreen();
            } else if (el.msExitFullscreen) {
                el.msExitFullscreen();
            }
            setTimeout(() => resolve({ code: 0, type: "exit" }), 500);
        } else {
            if (element.requestFullscreen) {
                element.requestFullscreen();
            } else if (element.mozRequestFullScreen) {
                element.mozRequestFullScreen();
            } else if (element.webkitRequestFullscreen) {
                element.webkitRequestFullscreen();
            } else if (element.msRequestFullscreen) {
                element.msRequestFullscreen();
            }
            setTimeout(() => resolve({ code: 0, type: "full" }), 500);
        }
    })
}

/**
 * 将图片转换为指定格式，例如将png格式转为jpg格式
 * @inner(file) 要转换的图片文件二进制对象
 * @inner(after) 要将图片文件转换成什么格式
 * @returns { File } 返回转换后的新文件
 */
export let convertImageFormat = (file: File, after: string) => {
    return new Promise((resolve, reject) => {
        let mimeMap: any = {
            jpg: "image/jpeg",
            jpeg: "image/jpeg",
            png: "image/png",
            webp: "image/webp",
            gif: "image/gif"
        };
        let mimeType = mimeMap[after.toLowerCase()];
        if (!mimeType) return reject("使用了不支持的格式:支持的格式有(jpg、jpeg、png、webp、gif)");
        let img = new Image();
        img.onload = () => {
            let canvas: HTMLCanvasElement = document.createElement("canvas") as HTMLCanvasElement;
            canvas.width = img.width;
            canvas.height = img.height;
            canvas.getContext("2d")?.drawImage(img, 0, 0);
            canvas.toBlob((blob: any) => {
                let newFile = new File(
                    [blob],
                    file.name.replace(/\.[^/.]+$/, `.${after}`),
                    {
                        type: mimeType,
                        lastModified: new Date().getTime(),
                    }
                );
                resolve(newFile);
            }, mimeType);
        };
        img.onerror = () => reject("转换失败！！！");
        img.src = URL.createObjectURL(file);
    });
}

// utils/file.ts

/**
 * 从 Content-Disposition 响应头解析文件名
 * @param header 响应头字符串
 * @returns 解析后的文件名，失败返回空字符串
 */
export let parseFilenameFromContentDisposition = (header: string | null): string => {
    if (!header) return "";

    // 1. 优先解析 RFC 5987 标准的 filename*（支持中文）
    const starMatch = header.match(/filename\*\s*=\s*([^;]+)/i);
    if (starMatch) {
        try {
            const parts = starMatch[1].trim().split("'");
            if (parts.length === 3 && parts[0].toLowerCase() === "utf-8") {
                return decodeURIComponent(parts[2]);
            }
        } catch (e) {
            console.warn("解析标准文件名失败，降级使用备用名", e);
        }
    }

    // 2. 降级解析传统 filename
    const simpleMatch = header.match(/filename\s*=\s*"?([^";]+)"?/i);
    if (simpleMatch) {
        return simpleMatch[1].trim();
    }

    // 3. 最终兜底
    return "";
}

/**
 * 通过接口导出为Execl
 * @inner(res) 传入接口返回的二进制数据
 * @inner(name) 传入要下载的名称
 * @inner(type) 文件的协议(默认为Execl的协议，会下载成Execl文件，如果需要解析成其他文件的话改为对应的协议即可)
 * @returns 直接下载文件
 */
// export let postDownload = (res: Blob | null, name: string = "模版文件.xls", type: string = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") => {
//     if (res) {
//         let blob = new Blob([res], { type });
//         let downloadElement = document.createElement("a");
//         let href = window.URL.createObjectURL(blob);
//         downloadElement.href = href;
//         downloadElement.download = name;
//         document.body.appendChild(downloadElement);
//         downloadElement.click();
//         document.body.removeChild(downloadElement);
//         window.URL.revokeObjectURL(href);
//     } else {
//         let message = ref('');
//         message.value = '文件无法下载或暂未获取文件!';
//         console.warn(message.value);
//     }
// }
export let postDownload = (
    res: Blob | null,
    name: string = "历史数据.csv",
    type: string = "text/csv;charset=utf-8"
) => {
    if (res) {
        let blob = new Blob([res], { type });
        let downloadElement = document.createElement("a");
        let href = window.URL.createObjectURL(blob);
        downloadElement.href = href;
        downloadElement.download = name;
        document.body.appendChild(downloadElement);
        downloadElement.click();
        document.body.removeChild(downloadElement);
        window.URL.revokeObjectURL(href);
    } else {
        // 移除无效的 ref，直接打印警告
        console.warn('文件无法下载或暂未获取文件!');
    }
}

/**
 * 复制文本
 * @inner(text) 传入要复制的文本
 * @returns 复制成功或失败
 */
export let copyPublicChange = (text: string) => {
    navigator.clipboard.writeText(text).then(function () {
        ElMessage.success("复制成功！");
    }).catch(function (err) {
        let textarea: any = document.createElement('textarea');
        textarea.style.position = 'fixed';
        textarea.style.opacity = 0;
        textarea.value = text;
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
        ElMessage.success("复制成功！");
    });
}

/**
 * 上传图片
 * @inner(file) 要上传的文件
 * @inner(fileName) 要上传的文件字段
 * @returns 返回上传文件的路径
 */
export let picturePublicImage = (file: File, fileName?: string, ip?: string): Promise<string> => {
    return new Promise(async (resolve, reject) => {
        if (file && file instanceof File) {
            let formData = new FormData();
            formData.append(fileName ? fileName : "file", file);
            let PicturePublicApi: any = await picturePublicApi(formData);
            resolve(ip ? ip : "https://182.92.141.39/" + PicturePublicApi.url);
        } else {
            reject("未传入文件或传入的不是一个文件！");
        }
    })
}

// 计算并格式化两个时间的差，返回友好的提示文本
export let formatTime = (collectionTime: any) => {
    if (!collectionTime) return '未知时间'

    const now = new Date()
    const collect = new Date(collectionTime)
    const diff = now - collect // 毫秒差

    const sec = Math.floor(diff / 1000)
    const min = Math.floor(sec / 60)
    const hour = Math.floor(min / 60)
    const day = Math.floor(hour / 24)

    if (sec < 60) return '刚刚'
    if (min < 60) return `${min}分钟前`
    if (hour < 24) return `${hour}小时前`
    if (day < 30) return `${day}天前`

    // 超过30天直接显示年月日时分秒
    return collect.toLocaleString()
}
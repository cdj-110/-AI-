/**
 * 匹配事件类型的操作函数
 */
export let eventTypeChange: any = {
    "单击": "click",
    "双击": "dblclick",
    "鼠标移入": "mouseenter",
    "鼠标移出": "mouseleave",
    "渲染前": "vnodeBeforeMount",
    "渲染后": "vnodeMounted"
};

/**
 * 获取操作对应的函数
 */
export let functionTypeChange: any = {
    "更新属性": "attrChange",
    "页面跳转": "openPage",
    "打开链接": "openLink",
    "刷新页面": "renovate",
    "刷新数据": "refreshListChange",
    "禁启用": "noGoLive",
    "显隐切换": "gotoShow"
};

/**
 * 根据相同的事件类型分组
 * @inner(_arr) 传入要转换的扁平化数组
 * @inner(_key) 传入要根据哪个字段转换
 * @inner(_isN) 传入是否要根据@inner(eventTypeChange)转换为英文事件类型
 */
export let eventTypeGroup = (_arr: Array<any>, _key: string, _isN?: boolean) => {
    if(!_arr || !Array.isArray(_arr) || _arr.length <= 0){
        console.warn("eventTypeGroup函数：传入的数组为空，无法进行分组！");
        return false;
    }
    let result: any = {};
    let arr = _arr.reduce((acc, obj) => {
        let key = obj[_key];
        if (!acc[key]) {
          acc[key] = [];
        }
        acc[key].push(obj);
        return acc;
    }, {});
    for(let key in arr){
        for (let element in eventTypeChange) {
            if(key == element){
                result[eventTypeChange[element]] = arr[key];
            }
        }
    }
    initializeJSON(result);
    return _isN ? result : arr;
}

/**
 * 事件函数区分，封装为以下格式
 * @json {
    "id": "button001",
    "type": "Button",
    "props": {
        "text": "打开阀门"
    },
    "events": {
        "click": [
            {
                "action": "函数名称1",
                "params": {
                "point": "阀门1",
                    "value": 1
                }
            }
            ],
            "dblclick": [
            {
                "action": "函数名称2",
                "params": {
                    "text": "双击成功"
                }
            }
        ],
    }
 * }
 */
let initializeJSON = (_arr: any) => {
    for(let key in _arr){
        if(Array.isArray(_arr[key])){
            _arr[key] = _arr[key].map(item => {
                if(item.startTrue != "启用" && item.changeAttr != "禁启用"){
                    return false;
                }
                return {
                    action: functionTypeChange[item.changeAttr],
                    params: item
                };
            })
        }
    }
    return _arr;
}
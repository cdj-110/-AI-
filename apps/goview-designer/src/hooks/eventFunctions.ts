import { eventHighway, JSONParse, JSONStringify } from "@/utils";

let startTrueVVV = "启用";
let xianTrueVVV = "显示";

/**
 * 组件事件[函数]注册hooks
 */
export let eventFunctions: any = {
    
    /**
     * 跳转页面
     */
    openPage(ctx: any, list: any) {
        eventHighway.on("openPage",list);
    },

    /**
     * 禁启用
     */
    noGoLive(ctx: any, list: any) {
        switch(list.startTrue){
            case "切换":
                startTrueVVV == "启用" ? startTrueVVV = "禁用" : startTrueVVV = "启用";
                list.startTrueVVV = startTrueVVV;
                break;
            case "启用":
                list.startTrueVVV = "启用";
                break;
            case "禁用":
                list.startTrueVVV = "禁用";
                break;
        }
        eventHighway.on("noGoLive", list);
    },

    /**
     * 显隐切换
     */
    gotoShow(ctx: any, list: any) {
        switch(list.xianTrue){
            case "切换":
                xianTrueVVV == "显示" ? xianTrueVVV = "隐藏" : xianTrueVVV = "显示";
                list.xianTrueVVV = xianTrueVVV;
                break;
            case "显示":
                list.xianTrueVVV = "显示";
                break;
            case "隐藏":
                list.xianTrueVVV = "隐藏";
                break;
        }
        eventHighway.on("gotoShow", list);
    },
    
    /**
     * 刷新数据
     */
    refreshListChange(ctx: any, list: any) {
        console.log("刷新数据具体的操作",list)
    },

    /**
     * 更新属性
     */
    attrChange(ctx: any, list: any) {
        eventHighway.on("attrChange", list);
    },

    /**
     * 打开链接
     * @inner(ctx) 执行上下文(包含所有组件的事件)
     * @inner(list) 操作的组件信息
     */
    openLink(ctx: any, list: any) {
        if(list.startType == "当前窗口"){
            location.href = list.urlLink;
        }else{
            window.open(list.urlLink);
        }
    },

    /**
     * 刷新页面
     */
    renovate() {
        location.reload();
    }
}
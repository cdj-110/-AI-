import { eventFunctions } from "./eventFunctions";
/**
 * 事件函数的注册处理中心，事件的核心执行器(事件中转站)
 * @inner component 单个组件信息
 * @inner eventName 组件要触发的事件
 * @inner e 触发组件时传入的值
 */
export function executeEvent(component: any, eventName: string, e: any) {
    let actions = component.events.baseEvent?.[eventName];
    // console.log("具体执行器",actions)
    if (actions){
        actions.forEach((action: any) => {
            let fn = eventFunctions[action.action];
            if (!fn) {
                console.warn("不存在事件:", action.action ? action.action : action.changeType)
                return;
            }
            fn({ event: e, component }, action.params);
        })
    }
}
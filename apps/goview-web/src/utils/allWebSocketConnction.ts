import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
import { ref } from "vue";  
// 响应式数据  
const messages = ref([]);  
const isConnected = ref(false);  
const connectionStatus = ref('未连接');  

let socket: WebSocket | null = null;  

/**  
 * 连接  
 */  
export function allConnection(dev_id?: string) {  
    const tokenStr: any = sessionStorage.getItem("wk_Token");  
    let token = "";
    if (tokenStr && typeof tokenStr === "string") {
        const parsed = JSON.parse(tokenStr);
        if (parsed && parsed.token) {
            token = parsed.token;
        }
    }  

    return new Promise<WebSocket>((resolve, reject) => { 
        const idParam = dev_id || ''; 
        const wsUrl = idParam;  
        const ws = token ? new WebSocket(wsUrl, ["bearer", token]) : new WebSocket(wsUrl);
        socket = ws;
        let settled = false;

        ws.onopen = () => {  
            isConnected.value = true;  
            connectionStatus.value = '已连接';  
            console.log('链接成功')
            if (!settled) {
                settled = true;
                resolve(ws);
            }
        };  

        ws.onmessage = (event: MessageEvent) => {  
            messages.value.push(`收到: ${event.data}`);  
        };  

        ws.onerror = (error: Event) => {
            connectionStatus.value = '连接错误';  
            if (!settled) {
                settled = true;
                reject(new Error(connectionStatus.value));
            }
        };  

        ws.onclose = (event: CloseEvent) => {  
            isConnected.value = false;  
            connectionStatus.value = '连接已关闭';  
            console.log('WebSocket 连接关闭', { code: event.code, reason: event.reason });  
            if (!settled) {
                settled = true;
                reject(new Error(connectionStatus.value));
            }
        };  
    });  
}

import useCounterStore from "@/stores/counter";
let store = useCounterStore();//实例化pinia函数
import { ref } from "vue";  
// let { user_id } = storeToRefs(store);  
// 响应式数据  
const messages = ref([]);  
const isConnected = ref(false);  
const connectionStatus = ref('未连接');  

let socket: WebSocket | null = null;  

/**  
 * 连接  
 */  
export function connection(dev_id?: string) {  
    const tokenStr: any = sessionStorage.getItem("wk_Token");  
    let token = "";
    if (tokenStr && typeof tokenStr === "string") {
        const parsed = JSON.parse(tokenStr);
        if (parsed && parsed.token) {
            token = parsed.token;
        }
    }  

    return new Promise<WebSocket>((resolve, reject) => {  
        // const wsUrl = "wss://192.168.1.101/api/mqtt_collection/status/realtime/all/devices";  
        const idParam = dev_id || ''; 
        const wsUrl = idParam;
        socket = token ? new WebSocket(wsUrl, ["bearer", token]) : new WebSocket(wsUrl);  

        socket.onopen = () => {  
            isConnected.value = true;  
            connectionStatus.value = '已连接';  
            console.log('链接成功')
            resolve(socket);  
        };  

        socket.onmessage = (event: MessageEvent) => {  
            messages.value.push(`收到: ${event.data}`);  
        };  

        socket.onerror = (error: Event) => {
            connectionStatus.value = '连接错误';  
            reject(new Error(connectionStatus.value));  
        };  

        socket.onclose = (event: CloseEvent) => {  
            isConnected.value = false;  
            connectionStatus.value = '连接已关闭';  
            console.log('WebSocket 连接关闭', { code: event.code, reason: event.reason });  
            reject(new Error(connectionStatus.value));  
        };  
    });  
}
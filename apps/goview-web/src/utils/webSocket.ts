import { connection } from "@/utils/webSocketConnction";
export function webSocket() {
    connection().then((res: any) => {
        res.text.on("message", (text: any) => {
            sessionStorage.setItem('webSocketData', JSON.stringify(text));
        })
    });
}
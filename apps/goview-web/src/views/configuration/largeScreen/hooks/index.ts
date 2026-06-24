import useCounterStore from "@/stores/counter";
import { getApiV1ConfigCatalog } from "@/api/configuration/index";
import { ref } from "vue";
import { storeToRefs } from "pinia";

export default function screenRootFun(){
    let store = useCounterStore();//实例化pinia函数
    let { user_id } = storeToRefs(store);

    let screenList:any = ref([]);//大屏列表

    /**
     * 查询数据大屏
     */
    let getList = ({ config_name, userId, type }: { config_name: any, userId?: any, type?: number }) => {
        getApiV1ConfigCatalog({ user_id: !userId ? user_id.value : userId, config_name: config_name }).then((res: any) => {
            screenList.value = Array.isArray(res) ? res : [];
            if(type === 1){
                store.setCreateWorkChange(screenList.value.filter((item: any) => item.on_workbench == 1));
            }
        }).catch((err: any) => {
            screenList.value = [];
            console.error("加载数据大屏列表失败", err);
        })
    }

    return {
        screenList,
        getList
    }
}

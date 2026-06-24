import { ref } from "vue";
import { defineStore } from "pinia";
import router from "@/router";

const readTokenState = () => {
  const raw = sessionStorage.getItem("wk_Token");
  if (!raw) return {};

  try {
    return JSON.parse(raw);
  } catch {
    return {};
  }
};

export default defineStore(
  "wk_Token",
  () => {
    const tokenState = readTokenState();
    const token = ref(tokenState.token || "");
    const technical = ref<Array<any>>(tokenState.technical || []);
    const nameValue = ref(tokenState.nameValue || "");
    const vxImg = ref(tokenState.vxImg || "");
    const user_id = ref(tokenState.user_id || "");
    const createWorkList = ref<Array<object>>([]);

    const persistLoginState = () => {
      sessionStorage.setItem(
        "wk_Token",
        JSON.stringify({
          token: token.value,
          technical: technical.value,
          nameValue: nameValue.value,
          vxImg: vxImg.value,
          user_id: user_id.value
        })
      );
    };

    const setToken = (e = "", name = "", img = "", uID = ""): void => {
      if (e) {
        token.value = e;
        nameValue.value = name;
        vxImg.value = img;
        user_id.value = uID;
        persistLoginState();
        return;
      }

      sessionStorage.removeItem("wk_Token");
      token.value = "";
      nameValue.value = "";
      vxImg.value = "";
      user_id.value = "";
      router.push("/login");
    };

    const getToken = (): string => {
      const state = readTokenState();
      token.value = state.token || "";
      return token.value;
    };

    const setTechnical = (e: Array<any>): void => {
      technical.value = e;
      persistLoginState();
    };

    const getTechnical = (): Array<any> => {
      const state = readTokenState();
      technical.value = state.technical || [];
      return technical.value;
    };

    const setCreateWorkChange = (e: Array<object>): void => {
      createWorkList.value = e;
    };

    return {
      token,
      technical,
      nameValue,
      vxImg,
      user_id,
      createWorkList,
      setToken,
      getToken,
      setTechnical,
      getTechnical,
      setCreateWorkChange
    };
  },
  {
    persist: {
      storage: sessionStorage
    }
  }
);

<template>
    <div class="login">
        <div class="language-switch">
            <span :class="{ 'lang-active': currentLang === 'zh-CN' }" @click="changeLang('zh-CN')">中</span>
            <span :class="{ 'lang-active': currentLang === 'en-US' }" @click="changeLang('en-US')">EN</span>
        </div>
        <div class="right">
            <div class="logo"> <img src="@/assets/simallTop.png" alt="Logo" style="width: 50px; height: 50px;" /> </div>
            <h4 class="title"><span class="title_text">WK</span> <span class="title_text"
                    style="color: black;">Cloud</span><span style="color: black;">{{ t('login.title') }}</span></h4>
            <h2 class="subtitle">{{ t('login.subtitle') }}</h2>
            <p class="welcome">{{ t('login.welcome') }}</p>
            <div style="display: flex;justify-content: space-between;align-items: center;margin-bottom: 2rem;"
                v-if="loginType == 'wj'">
                <span :class="{ ptClass: loginType == 'wj' }" @click="loginType = 'wj';" style="white-space: nowrap;">{{ t('login.forgetPasswordTitle') }}</span>
                <span :class="{ vxClass: true }" @click="goBack" style="white-space: nowrap;">{{ t('login.backToLogin') }}</span>
            </div>
            <div style="display: flex;justify-content: space-between;align-items: center;margin-bottom: 2rem;"
                v-if="loginType == 'iphoneZc' || loginType == 'yxZc'">
                <span :class="{ vxClass: loginType == 'yxZc', ptClass: loginType == 'iphoneZc' }"
                    @click="ZcPhone">{{ t('login.phoneRegister') }}</span>
                <span :class="{ vxClass: loginType == 'iphoneZc', ptClass: loginType == 'yxZc' }"
                    @click="zCemail">{{ t('login.emailRegister') }}</span>
            </div>
            <div v-if="recoverShet == 3">
                <div class="vxText">{{ t('login.bindPhone') }}</div>
                <div class="wxForm">
                    <el-form :model="recoverForm" label-position="top" ref="enrollFormRef" :rules="formInlineRule">
                        <div v-if="status_code == 0">
                            <el-form-item prop="password">
                                <el-input v-model="recoverForm.password" @keyup.enter="loginChange" :placeholder="t('login.password')"
                                    clearable show-password autocomplete="new-password">
                                    <template #prefix>
                                        <div style="color: red;font-size: 1.2rem;">*</div>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <div style="display: flex;justify-content: center;">
                                <el-button style="width: 100%" type="primary" @click="newPpassword">{{ t('login.login') }}</el-button>
                            </div>
                        </div>
                        <div v-if="status_code !== 0">
                            <el-form-item prop="phone">
                                <el-input v-model="recoverForm.phone" @keyup.enter="loginChange" :placeholder="t('login.phone')"
                                    clearable>
                                    <template #prefix>
                                        <div style="color: red;font-size: 1.2rem;">*</div>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <el-form-item prop="verification_code">
                                <el-input v-model="recoverForm.verification_code" @keyup.enter="loginChange"
                                    :placeholder="t('login.verificationCode')" clearable>
                                    <template #prefix>
                                        <span style="color: red; font-size: 1.2rem; line-height: 1;">*</span>
                                        <span style=" display: inline-block;"></span>
                                    </template>
                                    <template #suffix>
                                        <span style="color: #3C93E7; cursor: pointer; font-size: 14px;"
                                            @click="vxVerification">
                                            {{ verificationText }}
                                        </span>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <div style="display: flex;justify-content: center;">
                                <el-button style="width: 100%" type="primary" @click="bind">{{ t('login.bind') }}</el-button>
                            </div>
                        </div>
                        <el-checkbox v-model="recoverForm.type" value="1" style="margin-top: 0.5rem;">
                            <span style="font-size: 0.7rem; color: #707070;">
                                {{ t('login.iAgree') }}
                            </span>
                        </el-checkbox>
                        <div style="color: #3C93E7;font-size: 0.7rem;"> {{ t('login.switchAccount') }}></div>
                    </el-form>
                </div>
            </div>
            <div v-if="recoverShet !== 3" style="margin-top: 1rem;">
                <el-form :model="formInline" label-position="top" ref="loginRef" :rules="formInlineRule"
                    v-if="loginType == 'pt'">
                    <el-form-item prop="username">
                        <el-input v-model="formInline.username" prefix-icon="User" @keyup.enter="loginChange"
                            :placeholder="t('login.phoneOrEmail')" clearable>
                        </el-input>
                    </el-form-item>
                    <el-form-item v-if="recoverShet == 1" prop="password">
                        <el-input v-model="formInline.password" prefix-icon="Lock" @keyup.enter="loginChange"
                            :placeholder="t('login.password')" clearable show-password autocomplete="new-password">
                        </el-input>
                    </el-form-item>
                    <el-form-item v-if="recoverShet == 2" prop="verification_code">
                        <el-input v-model="formInline.verification_code"  prefix-icon="Lock" @keyup.enter="loginChange" :placeholder="t('login.verificationCode')"
                            clearable>
                            <template #suffix>
                                <span style="color: #3C93E7; cursor: pointer; font-size: 14px;"
                                    @click="verificationChange">
                                    {{ verificationText }}
                                </span>
                            </template>
                        </el-input>
                    </el-form-item>
                    <el-form-item prop="image_code">
                        <el-input v-model="formInline.image_code" clearable style="position: relative;"
                            @keyup.enter="loginChange">
                            <template #prefix>
                                <div style="color: red;font-size: 1.2rem;">*</div>
                            </template>
                            <template #append>
                                <img :src="'data:image/png;base64,' + imageVerificationText"
                                    @click="imageVerificationTextChange" />
                            </template>
                        </el-input>
                        <div
                            style="text-align: right;width: 100%;color: #3C93E7;cursor: pointer;font-size: 0.8rem;position: absolute;top: 77%;margin-top: 0.2rem;">
                            <span @click="loginType = 'wj'; recoverShip = 1; recoverType = 0;">{{ t('login.forgetPassword') }}</span>
                        </div>
                    </el-form-item>
                    <el-button type="primary" @click="loginChange"
                        style="width: 100%;margin: 1rem 0;background: linear-gradient(270deg, #2AC2FF 0%, #409eff 70%);height: 40px;border: none;">{{ t('login.login') }}</el-button>
                    <div style="font-size: 0.7em;">{{ t('login.noAccount') }}<span style="cursor: pointer;color: #3C93E7;"
                            @click="loginType = 'iphoneZc'">{{ t('login.registerNow') }}</span></div>
                    <div class="login-type-tabs">
                        <div class="login-type-item" @click="recoverShet = 1">
                            <img class="login-type-icon" :src="recoverShet == 1 ? qq1 : qq" />
                            <p class="login-type-text">{{ t('login.passwordLogin') }}</p>
                        </div>
                        <div class="login-type-item" @click="recoverShet = 2">
                            <img class="login-type-icon" :src="recoverShet == 2 ? qqq1 : qqq" />
                            <p class="login-type-text">{{ t('login.codeLogin') }}</p>
                        </div>
                        <div class="login-type-item" @click="vxClick">
                            <img class="login-type-icon" :src="recoverShet == 3 ? wx1 : wx1" />
                            <p class="login-type-text">{{ t('login.wechatLogin') }}</p>
                        </div>
                    </div>
                </el-form>
            </div>
            <el-form :model="recoverForm" label-position="top" ref="enrollFormRef" :rules="formInlineRule"
                v-if="loginType == 'iphoneZc' || loginType == 'yxZc'">
                <el-form-item prop="phone" v-if="loginType == 'iphoneZc'">
                    <el-input v-model="recoverForm.phone" :placeholder="t('login.pleaseEnterPhone')" clearable>
                        <template #prefix>
                            <div style="color: red;font-size: 1.2rem;">*</div>
                        </template>
                    </el-input>
                </el-form-item>
                <el-form-item prop="email" v-if="loginType == 'yxZc'">
                    <el-input v-model="recoverForm.email" :placeholder="t('login.pleaseEnterEmail')" clearable>
                        <template #prefix>
                            <div style="color: red;font-size: 1.2rem;">*</div>
                        </template>
                    </el-input>
                </el-form-item>
                <el-form-item prop="verification_code">
                    <el-input v-model="recoverForm.verification_code" @keyup.enter="loginChange" :placeholder="t('login.verificationCode')"
                        clearable>
                        <template #prefix>
                            <span style="color: red; font-size: 1.2rem; line-height: 1;">*</span>
                            <span style=" display: inline-block;"></span>
                        </template>
                        <template #suffix>
                            <span style="color: #3C93E7; cursor: pointer; font-size: 14px;" @click="zCsendClick">
                                {{ zcVerificationText }}
                            </span>
                        </template>
                    </el-input>
                </el-form-item>
                <el-form-item prop="password">
                    <el-input v-model="recoverForm.password" :placeholder="t('login.password')" clearable show-password
                        autocomplete="new-password">
                        <template #prefix>
                            <div style="color: red;font-size: 1.2rem;">*</div>
                        </template>
                    </el-input>
                </el-form-item>
                <el-form-item prop="confirm_password">
                    <el-input v-model="recoverForm.confirm_password" :placeholder="t('login.confirmPassword')" clearable show-password
                        autocomplete="new-password">
                        <template #prefix>
                            <div style="color: red;font-size: 1.2rem;">*</div>
                        </template>
                    </el-input>
                </el-form-item>
                <div style="display: flex;justify-content: space-between;align-items: center;margin-top: 2.5rem;">
                    <div style="font-size: 0.7em;">{{ t('login.iHaveAccount') }} <span style="cursor: pointer;color: #3C93E7;"
                            @click="goBack">{{ t('login.backToLogin') }}</span>
                    </div>
                    <el-button type="primary" @click="enrollChange">{{ t('login.register') }}</el-button>
                </div>
            </el-form>
            <div v-if="loginType == 'wj'">
                <el-form :model="recoverForm" label-position="top" ref="recoverFormRef" :rules="formInlineRule">
                    <div>
                        <div v-if="recoverType === 0" style="font-size: 0.9rem;font-weight: 700;">
                            <el-form-item :label="t('login.verificationMethod')" prop="VerificationMethod">
                                <el-select v-model="recoverForm.VerificationMethod" :placeholder="t('login.selectVerificationMethod')">
                                    <el-option :label="t('login.phoneVerification')" :value="1" />
                                    <el-option :label="t('login.emailVerification')" :value="2" />
                                </el-select>
                            </el-form-item>
                            <el-form-item :label="t('login.account')" prop="account">
                                <el-input v-model="recoverForm.account" :placeholder="t('login.pleaseEnterAccount')" />
                            </el-form-item>
                            <div style="display: flex;justify-content: flex-end;">
                                <el-button type="primary" @click="VerificationClick">{{ t('login.startVerification') }}</el-button>
                            </div>
                        </div>
                        <div v-if="recoverType !== 0">
                            <el-form-item prop="email" v-if="recoverType === 2">
                                <el-input v-model="recoverForm.email" :placeholder="t('login.pleaseEnterEmail')" clearable>
                                    <template #prefix>
                                        <div style="color: red;font-size: 1.2rem;">*</div>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <el-form-item prop="phone" v-if="recoverType === 1">
                                <el-input v-model="recoverForm.phone" :placeholder="t('login.pleaseEnterPhone')" clearable>
                                    <template #prefix>
                                        <div style="color: red;font-size: 1.2rem;">*</div>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <el-form-item prop="verification_code">
                                <el-input v-model="recoverForm.verification_code" @keyup.enter="loginChange"
                                    :placeholder="t('login.verificationCode')" clearable>
                                    <template #prefix>
                                        <span style="color: red; font-size: 1.2rem; line-height: 1;">*</span>
                                        <span style=" display: inline-block;"></span>
                                    </template>
                                    <template #suffix>
                                        <span style="color: #3C93E7; cursor: pointer; font-size: 14px;"
                                            @click="sendClick">
                                            {{ wjVerificationText }}
                                        </span>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <el-form-item prop="password">
                                <el-input v-model="recoverForm.password" :placeholder="t('login.password')" clearable show-password
                                    autocomplete="new-password">
                                    <template #prefix>
                                        <div style="color: red;font-size: 1.2rem;">*</div>
                                    </template>
                                </el-input>
                            </el-form-item>
                            <el-form-item prop="confirm_password">
                                <el-input v-model="recoverForm.confirm_password" :placeholder="t('login.confirmPassword')" clearable
                                    show-password autocomplete="new-password">
                                    <template #prefix>
                                        <div style="color: red;font-size: 1.2rem;">*</div>
                                    </template>
                                </el-input>
                            </el-form-item>
                        </div>
                    </div>
                    <div style="margin-top: 2.5rem;text-align: right;">
                        <el-button type="primary" v-if="recoverShip === 3" @click="recoveChange">{{ t('login.submit') }}</el-button>
                    </div>
                </el-form>
            </div>
        </div>
    </div>
</template>

<script lang="ts" setup>
import 'vue3-slide-verify/dist/style.css';
import qq from "@/assets/qq.png";
import qqq from "@/assets/qqq.png";
import qq1 from "@/assets/qq1.png";
import qqq1 from "@/assets/qqq1.png";
import wx1 from "@/assets/wx1.png"
import loginFunction from "./hooks/login";
import { ref, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useAppStore } from '@/stores/usePinia';

const { locale, t } = useI18n();
const currentLang = ref(locale.value);
const appStore = useAppStore();

const changeLang = (lang: string) => {
    locale.value = lang;
    currentLang.value = lang;
    localStorage.setItem('locale', lang);
    appStore.changeLocale(lang);
};

onMounted(() => {
    const savedLang = localStorage.getItem('locale');
    if (savedLang) {
        locale.value = savedLang;
        currentLang.value = savedLang;
        appStore.changeLocale(savedLang);
    }
});

let {
    formInline,
    recoverForm,
    loginRef,
    recoverFormRef,
    enrollFormRef,
    loginType,
    recoverShip,
    recoverType,
    formInlineRule,
    recoverShet,
    verificationText,
    wjVerificationText,
    dialogVisible,
    imageVerificationText,
    loading,
    qrCodeUrl,
    zcVerificationText,
    status_code,
    sendClick,
    zCsendClick,
    vxClick,
    recoveChange,
    enrollChange,
    loginChange,
    verificationChange,
    vxVerification,
    ZcPhone,
    zCemail,
    goBack,
    imageVerificationTextChange,
    bind,
    newPpassword,
    VerificationClick,
} = loginFunction();
</script>

<style lang="scss" scoped>
:deep(.el-checkbox__label) {
    white-space: normal !important;
    word-wrap: break-word;
    display: inline-block;
}

.login {
    width: 100vw;
    height: 100vh;
    display: flex;
    background-image: url("@/assets/bg.png");
    background-size: cover;
    display: flex;
    justify-content: center;
    align-items: center;
    position: relative;

    .language-switch {
        position: absolute;
        top: 20px;
        right: 20px;
        display: flex;
        gap: 8px;
        align-items: center;

        span {
            padding: 6px 16px;
            border-radius: 4px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.3s ease;
            background: rgba(255, 255, 255, 0.8);
            color: #666;
        }

        // span:hover {
        //     background: rgba(255, 255, 255, 1);
        // }

        .lang-active {
            background: #409eff;
            color: #fff;
        }
    }

    .logo {
        text-align: center;
    }

    .title {
        font-weight: bold;
        color: #3C93E7;
        text-align: center;

        .title_text {
            font-style: italic;
        }

        span {
            margin: 0 0.5rem 0 0;
        }
    }

    .subtitle {
        font-weight: 700;
        color: #000;
        text-align: center;
        margin: 10px 0;
    }

    .welcome {
        font-size: 0.8rem;
        color: #BABEC2;
        text-align: center;
        margin-bottom: 0.7rem;
    }

    .right {
        width: 360px;
        height: 650px;
        padding: 5vh 2vw;
        border-radius: 1.3rem;
        background-image: url("@/assets/smallBg.png");
        background-size: cover;
        background-position: center;
        background-repeat: no-repeat;
        overflow: hidden;

        .vxClass {
            font-size: 0.9rem;
            color: #3C93E7;
            text-decoration: underline;
            cursor: pointer;
            font-weight: 700;
        }

        .ptClass {
            font-size: 1.5rem;
            font-weight: 700;
            color: #000000;
            cursor: pointer;
        }

        :deep(.el-input__inner) {
            height: 40px;
        }

        :deep(.el-select__wrapper) {
            height: 42px;
        }
    }
}

.login-container {
    display: flex;
    justify-content: center;
    align-items: center;

    .qr-login-section {
        text-align: center;
        padding: 20px;
    }

    .qr-code-container {
        position: relative;
        display: inline-block;
        margin-bottom: 20px;
    }

    .qr-code {
        width: 200px;
        height: 200px;
        border: 1px solid #e0e0e0;
        border-radius: 8px;
    }

    .loading,
    .expired-tip {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(255, 255, 255, 0.9);
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 8px;
    }

    .scan-tip {
        color: #666;
        font-size: 14px;
        line-height: 1.5;
    }

    .scan-tip p {
        margin: 4px 0;
    }

    .refresh-btn {
        margin-top: 15px;
        padding: 8px 16px;
        background: #07c160;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }

    .refresh-btn:hover {
        background: #06ae56;
    }
}

.vxText {
    font-size: 0.9rem;
    color: #707070;
    margin-top: 1.5rem;
}

.wxForm {
    margin-top: 1.5rem;
}

:deep(.impowerBox),
:deep(.qrcode) {
    width: 350px;
}

:deep(.el-select__placeholder) {
    font-weight: 400;
}

:deep(.el-select__placeholder.is-transparent),
:deep(.el-input__inner) {
    font-size: 0.8rem;
}

:deep(.el-input__wrapper),
:deep(.el-input-group__append),
:deep(.el-select__wrapper) {
    box-shadow: none !important;
    border: none !important;
    background: #F8FAFC;
}

.login-type-tabs {
    display: flex;
    justify-content: space-between;
    width: 90%;
    margin: 50px auto 1rem;
    gap: 8px;
}

.login-type-item {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    cursor: pointer;
}

.login-type-icon {
    width: 2rem;
    height: 2rem;
    object-fit: contain;
    margin-bottom: 8px;
}

.login-type-text {
    font-size: 0.7rem;
    margin: 0;
    line-height: 1.2;
    color: inherit;
}
</style>

<template>
    <div>
        <el-dialog v-model="props.config.isTrue" :title="props.config.title" width="30%" draggable
            :close-on-press-escape="false" :close-on-click-modal="false">
            <el-tabs v-model="activeName" @click="handleClick(activeName)" style="padding:0 1rem;">
                <el-tab-pane :label="$t('userOrganize.basicInfo')" name="1">
                    <el-form :model="form1" label-width="130">
                        <el-form-item :label="$t('userOrganize.userName')">
                            <el-input v-model="form1.username" :placeholder="$t('userOrganize.pleaseEnterUsername')"
                                maxlength="15" show-word-limit />
                        </el-form-item>
                        <el-form-item :label="$t('userOrganize.account')" style="position: relative;">
                            <el-input v-model="form1.account" disabled
                                :placeholder="$t('userOrganize.pleaseEnterAccount')" clearable
                                style="position: relative;" />
                        </el-form-item>
                        <el-form-item :label="$t('userOrganize.companyName')" style="position: relative;">
                            <el-input v-model="form1.company"
                                :placeholder="$t('userOrganize.pleaseEnter') + $t('userOrganize.companyName')" clearable
                                style="position: relative;" maxlength="20" show-word-limit />
                        </el-form-item>
                        <el-form-item :label="$t('userOrganize.wechat')">
                            <p style="margin-right: 1rem;font-weight: 700;color: #3C93E7;cursor: pointer;font-size: 0.8rem;"
                                @click="vxTrue1 = true;">{{ form1.wechat_status == 0 ? $t('userOrganize.bind') :
                                    form1.wechat_status == 1
                                        ? $t('userOrganize.unbind') : '' }}</p>
                        </el-form-item>
                        <el-dialog v-model="vxTrue1" :title="$t('userOrganize.bindWechat')" width="20%" draggable
                            :close-on-press-escape="false" :close-on-click-modal="false">
                            <div style="text-align: center;">
                                <img src="@/assets/qr.png">
                                <p style="margin-top: 1rem;">{{ $t('userOrganize.qrCodeScan') }}</p>
                            </div>
                        </el-dialog>
                    </el-form>
                    <div style="display: flex;justify-content: flex-end;">
                        <el-button type="info" @click="props.config.isTrue = false">{{ $t('common.cancel')
                            }}</el-button>
                        <el-button type="primary" @click="onSubmit">{{ $t('common.save') }}</el-button>
                    </div>
                </el-tab-pane>
                <el-tab-pane :label="$t('userOrganize.changePassword')" name="2">
                    <el-form :model="alterInit" label-width="auto" ref="recoverFormRef" :rules="formInlineRule">
                        <el-form-item :label="$t('userOrganize.originalPassword')" prop="old_password">
                            <el-input v-model="alterInit.old_password"
                                :placeholder="$t('userOrganize.pleaseEnterOldPassword')" style="position: relative;"
                                clearable show-password autocomplete="new-password">
                                <template #prefix>
                                    <p
                                        style="text-align: right;width: 100%;color: #3C93E7;font-size: 0.7rem;cursor: pointer;position: absolute;left: 0;top: 1.5rem;">
                                        <span @click="forgotChange">{{ $t('userOrganize.forgotPassword') }}</span>
                                    </p>
                                </template>
                            </el-input>
                        </el-form-item>
                        <el-form-item :label="$t('userOrganize.newPassword')" prop="new_password">
                            <el-input v-model="alterInit.new_password"
                                :placeholder="$t('userOrganize.pleaseEnterPassword')" clearable show-password
                                autocomplete="new-password">
                            </el-input>
                        </el-form-item>
                        <el-form-item :label="$t('userOrganize.confirmNewPassword')" prop="confirm_password">
                            <el-input v-model="alterInit.confirm_password"
                                :placeholder="$t('userOrganize.pleaseConfirmPassword')" clearable show-password
                                autocomplete="new-password">
                            </el-input>
                        </el-form-item>
                    </el-form>
                    <div style="display: flex;justify-content: flex-end;">
                        <el-button type="info" @click="props.config.isTrue = false">{{ $t('common.cancel')
                            }}</el-button>
                        <el-button type="primary" @click="onSubmitPassword">{{ $t('common.save') }}</el-button>
                    </div>
                </el-tab-pane>
                <el-tab-pane :label="$t('userOrganize.alarmAccountSettings')" name="3">
                    <el-form :model="form3" label-width="130">
                        <el-form-item :label="$t('userOrganize.userName')">
                            <el-input v-model="form3.username" disabled
                                :placeholder="$t('userOrganize.pleaseEnterUsername')" />
                        </el-form-item>
                        <el-form-item :label="$t('userOrganize.phoneNumber')" style="position: relative;">
                            <el-input v-model="form3.alarm_phone" :placeholder="$t('userOrganize.pleaseEnterPhone')"
                                clearable style="position: relative;" :disabled="disabledPhone">
                                <template #prefix>
                                    <p style="position: absolute;right: 0;color: #3C93E7;cursor: pointer;margin-right: 1.6rem;font-size: 0.8rem;"
                                        @click="iphoneTextChange" v-if="form3.alarm_phone">{{ iphoneText }}</p>
                                </template>
                            </el-input>
                        </el-form-item>
                        <el-form-item :label="$t('userOrganize.emailAddress')" style="position: relative;">
                            <el-input v-model="form3.alarm_email" :placeholder="$t('userOrganize.pleaseEnterEmail')"
                                clearable style="position: relative;" :disabled="disabledEmail">
                                <template #prefix>
                                    <p style="position: absolute;right: 0;color: #3C93E7;cursor: pointer;margin-right: 1.6rem;font-size: 0.8rem;"
                                        @click="emailTextChange" v-if="form3.alarm_email">{{ emailText }}</p>
                                </template>
                            </el-input>
                        </el-form-item>
                        <el-dialog v-model="iphoneTrue" :title="$t('userOrganize.inputPhoneVerificationCode')"
                            width="20%" center draggable :close-on-press-escape="false" :close-on-click-modal="false">
                            <el-form-item :label="$t('userOrganize.verificationCode')">
                                <el-input v-model="form3.identifier"
                                    :placeholder="$t('userOrganize.pleaseEnterVerificationCode')" />
                            </el-form-item>
                            <template #footer>
                                <el-button type="info" @click="form3.identifier = ''; iphoneTrue = false;">{{
                                    $t('common.cancel') }}</el-button>
                                <el-button type="primary" @click="iphoneVerifyChange('iphone')">{{ $t('common.submit')
                                    }}</el-button>
                            </template>
                        </el-dialog>
                        <el-dialog v-model="emailTrue" :title="$t('userOrganize.inputEmailVerificationCode')"
                            width="20%" center draggable :close-on-press-escape="false" :close-on-click-modal="false">
                            <el-form-item :label="$t('userOrganize.verificationCode')">
                                <el-input v-model="form3.identifier"
                                    :placeholder="$t('userOrganize.pleaseEnterVerificationCode')" />
                            </el-form-item>
                            <template #footer>
                                <el-button type="info" @click="form3.identifier = ''; emailTrue = false;">{{
                                    $t('common.cancel') }}</el-button>
                                <el-button type="primary" @click="iphoneVerifyChange('email')">{{ $t('common.submit')
                                    }}</el-button>
                            </template>
                        </el-dialog>
                        <el-form-item :label="$t('userOrganize.wechatOfficialAccount')">
                            <p style="margin-right: 1rem;font-weight: 700;color: #3C93E7;cursor: pointer;"
                                @click="vxTrue3 = true;">{{
                                    form3.alarm_wechat }}
                            </p>
                            <p>{{ $t('userOrganize.wechatAlertNotice') }}</p>
                        </el-form-item>
                        <el-dialog v-model="vxTrue3" :title="$t('userOrganize.bindWechat')" width="20%" draggable
                            :close-on-press-escape="false" :close-on-click-modal="false">
                            <div style="text-align: center;">
                                <img src="@/assets/qr.png">
                                <p style="margin-top: 1rem;">{{ $t('userOrganize.wechatScanFollow') }}</p>
                            </div>
                        </el-dialog>
                    </el-form>
                    <div style="display: flex;justify-content: flex-end;">
                        <el-button type="info" @click="props.config.isTrue = false">{{ $t('common.cancel')
                            }}</el-button>
                        <el-button type="primary" @click="submit">{{ $t('common.save') }}</el-button>
                    </div>
                </el-tab-pane>
            </el-tabs>
        </el-dialog>
    </div>
</template>

<script lang="ts" setup>
import { ElMessage } from 'element-plus'
import useCounterStore from "@/stores/counter";
import { user_info, update_user_infog, update_pwd, sendcode, check_alert_account, set_alert_account } from '@/api/login/index'
import type { TabsPaneContext } from 'element-plus'
let recoverFormRef: any = ref("");//忘记密码ref
let props = withDefaults(defineProps<{
    config: {
        isTrue: Boolean,
        title: String
    }
}>(), {
    config: () => ({
        isTrue: false,
        title: ""
    })
});
let store = useCounterStore();//实例化pinia函数
let { user_id } = storeToRefs(store);
let { setToken } = useCounterStore();//实例化pinia函数
let activeName = ref('1');//默认选中哪一个
let form1 = ref({ username: "", account: "", company: "", wechat_status: '' });//基本信息
let form3 = ref({ username: "", alarm_phone: "", alarm_email: "", alarm_wechat: '', identifier: "" });//告警账号设置
let iphoneText = ref("发送验证码");//告警账号设置 - 手机号点击更改绑定
let emailText = ref("发送验证码");//告警账号设置 - 邮箱号点击更改绑定
let iphoneTrue = ref(false);//手机验证码的弹窗
let emailTrue = ref(false);//邮箱验证码的弹窗
let vxTrue1 = ref(false);//基本信息绑定微信的弹窗
let vxTrue3 = ref(false);//告警账号设置绑定微信的弹窗
let alterInit = ref({ confirm_password: "", new_password: "", old_password: "", user_id: '' });//修改密码
let emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/  // 邮箱验证正则
let phoneRegex = /^1[3-9]\d{9}$/ // 手机号验证正则（中国手机号）
let disabledPhone = ref(false)
let disabledEmail = ref(false)
let vvv = ref(60);
let disAlarm_emails = ref(false)

// 自定义密码校验规则
const validatePassword = (rule: any, value: any, callback: any) => {
    if (!value) {
        callback(new Error('请输入密码'))
    } else if (value.length < 8) {
        callback(new Error('密码长度不能少于8位'))
    } else if (value.length > 32) {
        callback(new Error('密码长度不能超过32位'))
    } else if (!validatePasswordComplexity(value)) {
        callback(new Error('密码必须包含大写字母、小写字母或特殊字符中的至少一种'))
    } else {
        callback()
    }
}
let handleClick = (e: any) => { }

// 自定义确认密码校验规则
const validateConfirmPassword = (rule: any, value: any, callback: any) => {
    if (!value) {
        callback(new Error('请再次输入密码'))
    } else if (value !== alterInit.value.new_password) {
        callback(new Error('两次输入的密码不一致'))
    } else {
        callback()
    }
}

// 密码复杂度验证函数
const validatePasswordComplexity = (password: string): boolean => {
    // 至少包含大写字母、小写字母或特殊字符中的一种
    const hasUpperCase = /[A-Z]/.test(password)
    const hasLowerCase = /[a-z]/.test(password)
    const hasSpecialChar = /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(password)

    return hasUpperCase || hasLowerCase || hasSpecialChar
}

//验证账号是否为空
let formInlineRule = ref({
    new_password: [{ required: true, message: "密码不能为空！", trigger: "change" }, { required: true, validator: validatePassword, trigger: 'change' }],
    confirm_password: [{ required: true, message: "确认密码不能为空！", trigger: "change" }, { validator: validateConfirmPassword, trigger: 'change' }],
    old_password: [{ required: true, message: "请输入旧密码", trigger: "change" }],
})

onMounted(() => {
    userList()
})

let userList = () => {
    user_info(user_id.value).then((res: any) => {
        if (res.code == 200) {
            form1.value = res.data
            form3.value = res.data
        }
    })
}
/**
 * 基本信息 - 手机号点击更改绑定后要执行的东西
 */
let iphoneTextChange = () => {
    // iphoneTrue.value = true
    chronography("iphone", 1);
}

/**
 * 告警账号设置 - 邮箱号点击更改绑定后要执行的东西
 */
let emailTextChange = () => {
    chronography("email", 2);
}

/**
 * 发送验证码的倒计时
 */
let chronography = (e: string, val: any) => {
    yzm(val, e)
}
let yzm = (val: any, e: any) => {
    // let vvv = 60;
    if (e === "iphone" ? iphoneText.value === "发送验证码" : emailText.value === "发送验证码") {
        let ver = setInterval(() => {
            if (vvv.value <= 0) {
                vvv.value = 60;
                clearInterval(ver);
                e === "iphone" ? iphoneText.value = `发送验证码` : emailText.value = `发送验证码`;
                e === "iphone" ? iphoneTrue.value = false : emailTrue.value = false;
            } else {
                vvv.value--;
                e === "iphone" ? iphoneText.value = `${vvv.value}秒后结束` : emailText.value = `${vvv.value}秒后结束`;
            }
        }, 1000)
        let params = {
            type: val
        }
        if (val == 1) {
            params.phone = form3.value.alarm_phone
        } else {
            params.email = form3.value.alarm_email
        }

        sendcode(params).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg)
                if (val == 1) {
                    iphoneTrue.value = true
                    disabledPhone.value = true
                } else {
                    emailTrue.value = true
                    disabledEmail.value = true
                }
            } else {
                vvv.value = 0
                disabledEmail.value = false
                disabledPhone.value = false
                ElMessage.error(res.msg)
            }
        })
    } else {
        ElMessage.error("验证码倒计时结束后才嫩再次发送！");
    }
}

/**
 * 告警账号设置 - 验证码确认(判断验证码是否通过，通过之后修改手机号)
 */
let iphoneVerifyChange = (e: string) => {
    if (e === "iphone") {
        check_alert_account({ identifier: form3.value.alarm_phone, verification_code: form3.value.identifier }).then((res: any) => {
            if (res.code == 200) {
                userList()
                ElMessage.success(res.msg)
                vvv.value = 0
                iphoneTrue.value = false;
                disAlarm_emails.value = false
            } else {
                ElMessage.error(res.msg)
            }
        })
    } else {
        check_alert_account({ identifier: form3.value.alarm_email, verification_code: form3.value.identifier }).then((res: any) => {
            if (res.code == 200) {
                ElMessage.success(res.msg)
                vvv.value = 0
                emailTrue.value = false
            } else {
                ElMessage.error(res.msg)
            }
        })
    }
}
/**
 * 账号设置保存
 */
let submit = () => {
    set_alert_account({ user_id: user_id.value, alarm_phone: form3.value.alarm_phone, alarm_email: form3.value.alarm_email }).then((res: any) => {
        if (res.code == 200) {
            ElMessage.success(res.msg)
            props.config.isTrue = false
        } else {
            ElMessage.error(res.msg)
        }
    })
}

/**
 * 忘记密码
 */
let forgotChange = () => {
    setToken("");
}
/**
 * 修改保存基本信息
 */
let onSubmit = () => {
    let params = {
        user_id: user_id.value,
        username: form1.value.username,
        company: form1.value.company
    }
    update_user_infog(params).then((res: any) => {
        if (res.code == 200) {
            userList()
            ElMessage.success(res.msg);
            props.config.isTrue = false
        } else {
            ElMessage.error(res.msg);
        }
    })
}
/**
 * 修改密码
 */
let onSubmitPassword = () => {
    recoverFormRef.value.validate((vali: any) => {
        if (vali) {
            alterInit.value.user_id = user_id.value
            update_pwd(alterInit.value).then((res: any) => {
                if (res.code == 200) {
                    userList()
                    ElMessage.success(res.msg);
                    props.config.isTrue = false
                } else {
                    ElMessage.error(res.msg);
                }
            })
        }
    })
}
</script>
<style lang="scss" scoped>
:deep(.el-input__inner) {
    font-size: 0.8rem;
}
</style>
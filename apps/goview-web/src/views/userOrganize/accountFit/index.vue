<template>
    <div>
        <el-dialog v-model="props.config.isTrue" :title="props.config.title" width="25%" draggable
            :close-on-press-escape="false" :close-on-click-modal="false">
            <el-tabs v-model="activeName" @click="handleClick(activeName)" style="padding:0 1rem;">
                <el-tab-pane :label="$t('userOrganize.basicInfo')" name="1">
                    <el-form :model="form1" label-width="100">
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
                    <el-form :model="alterInit" label-width="100" ref="recoverFormRef" :rules="formInlineRule">
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
                    <el-form :model="form3" label-width="100">
                        <el-form-item :label="$t('userOrganize.userName')">
                            <el-input v-model="form3.username" disabled
                                :placeholder="$t('userOrganize.pleaseEnterUsername')" />
                        </el-form-item>
                        
                        <!-- ========== 手机号输入框 ========== -->
                        <el-form-item :label="$t('userOrganize.phoneNumber')" style="position: relative;">
                            <div style="position: relative; flex: 1;">
                                <el-input v-model="form3.alarm_phone" :placeholder="$t('userOrganize.pleaseEnterPhone')"
                                    clearable :disabled="!isEditingPhone">
                                    <template #prefix>
                                        <!-- 发送验证码：仅编辑状态+有值时显示 -->
                                        <p v-if="isEditingPhone && form3.alarm_phone" 
                                           style="position: absolute;right: 1rem;top: 50%;transform: translateY(-50%);color: #3C93E7;cursor: pointer;font-size: 0.8rem;white-space: nowrap;"
                                           @click="iphoneTextChange">{{ iphoneText }}</p>
                                    </template>
                                </el-input>
                                <!-- 编辑/保存按钮：永远可点击 -->
                                <p v-if="isEditingPhone || form3.alarm_phone"
                                   style="position: absolute;right: 0.2rem;top: 50%;transform: translateY(-50%);color: #3C93E7;cursor: pointer;font-size: 0.8rem;white-space: nowrap;"
                                   @click="togglePhoneEdit">{{ isEditingPhone ? '' : '编辑' }}</p>
                            </div>
                        </el-form-item>
                        
                        <!-- ========== ：邮箱输入框 ========== -->
                        <el-form-item :label="$t('userOrganize.emailAddress')" style="position: relative;">
                            <div style="position: relative; flex: 1;">
                                <el-input v-model="form3.alarm_email" :placeholder="$t('userOrganize.pleaseEnterEmail')"
                                    clearable :disabled="!isEditingEmail">
                                    <template #prefix>
                                        <!-- 发送验证码：仅编辑状态+有值时显示 -->
                                        <p v-if="isEditingEmail && form3.alarm_email" 
                                           style="position: absolute;right: 1rem;top: 50%;transform: translateY(-50%);color: #3C93E7;cursor: pointer;font-size: 0.8rem;white-space: nowrap;"
                                           @click="emailTextChange">{{ emailText }}</p>
                                    </template>
                                </el-input>
                                <!-- 编辑/保存按钮：永远可点击 -->
                                <p v-if="isEditingEmail || form3.alarm_email"
                                   style="position: absolute;right: 0.2rem;top: 50%;transform: translateY(-50%);color: #3C93E7;cursor: pointer;font-size: 0.8rem;white-space: nowrap;"
                                   @click="toggleEmailEdit">{{ isEditingEmail ? '' : '编辑' }}</p>
                            </div>
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
let recoverFormRef: any = ref("");
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
let store = useCounterStore();
let { user_id } = storeToRefs(store);
let { setToken } = useCounterStore();
let activeName = ref('1');
let form1 = ref({ username: "", account: "", company: "", wechat_status: '' });
let form3 = ref({ username: "", alarm_phone: "", alarm_email: "", alarm_wechat: '', identifier: "" });
let iphoneText = ref("发送验证码");
let emailText = ref("发送验证码");
let iphoneTrue = ref(false);
let emailTrue = ref(false);
let vxTrue1 = ref(false);
let vxTrue3 = ref(false);
let alterInit = ref({ confirm_password: "", new_password: "", old_password: "", user_id: '' });
let emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/
let phoneRegex = /^1[3-9]\d{9}$/
let vvv = ref(60);
let disAlarm_emails = ref(false)

// 编辑状态控制：true=可编辑 false=置灰
let isEditingPhone = ref(false)
let isEditingEmail = ref(false)

// 密码校验规则（不变）
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

const validateConfirmPassword = (rule: any, value: any, callback: any) => {
    if (!value) {
        callback(new Error('请再次输入密码'))
    } else if (value !== alterInit.value.new_password) {
        callback(new Error('两次输入的密码不一致'))
    } else {
        callback()
    }
}

const validatePasswordComplexity = (password: string): boolean => {
    const hasUpperCase = /[A-Z]/.test(password)
    const hasLowerCase = /[a-z]/.test(password)
    const hasSpecialChar = /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(password)
    return hasUpperCase || hasLowerCase || hasSpecialChar
}

let formInlineRule = ref({
    new_password: [{ required: true, message: "密码不能为空！", trigger: "change" }, { required: true, validator: validatePassword, trigger: 'change' }],
    confirm_password: [{ required: true, message: "确认密码不能为空！", trigger: "change" }, { validator: validateConfirmPassword, trigger: 'change' }],
    old_password: [{ required: true, message: "请输入旧密码", trigger: "change" }],
})

onMounted(() => {
    userList()
})

// 核心逻辑：根据值动态设置初始编辑状态
let userList = () => {
    user_info(user_id.value).then((res: any) => {
        if (res.code == 200) {
            form1.value = res.data
            form3.value = res.data
            // 关键逻辑：有值=置灰(false) 空值=可编辑(true)
            isEditingPhone.value = !form3.value.alarm_phone
            isEditingEmail.value = !form3.value.alarm_email
        }
    })
}

// 手机号编辑/保存切换
const togglePhoneEdit = () => {
    if (isEditingPhone.value) {
        // 点击保存：根据当前值决定是否置灰
        isEditingPhone.value = !form3.value.alarm_phone
        // 重置验证码倒计时
        if (vvv.value < 60 && vvv.value > 0) {
            vvv.value = 0
            iphoneText.value = "发送验证码"
        }
    } else {
        // 点击编辑：进入可编辑状态
        isEditingPhone.value = true
    }
}

// 邮箱编辑/保存切换
const toggleEmailEdit = () => {
    if (isEditingEmail.value) {
        // 点击保存：根据当前值决定是否置灰
        isEditingEmail.value = !form3.value.alarm_email
        // 重置验证码倒计时
        if (vvv.value < 60 && vvv.value > 0) {
            vvv.value = 0
            emailText.value = "发送验证码"
        }
    } else {
        // 点击编辑：进入可编辑状态
        isEditingEmail.value = true
    }
}

// 发送验证码（不变）
let iphoneTextChange = () => {
    chronography("iphone", 1);
}

let emailTextChange = () => {
    chronography("email", 2);
}

let chronography = (e: string, val: any) => {
    yzm(val, e)
}
let yzm = (val: any, e: any) => {
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
                } else {
                    emailTrue.value = true
                }
            } else {
                vvv.value = 0
                ElMessage.error(res.msg)
            }
        })
    } else {
        ElMessage.error("验证码倒计时结束后才能再次发送！");
    }
}

// 验证码验证（不变）
let iphoneVerifyChange = (e: string) => {
    if (e === "iphone") {
        check_alert_account({ identifier: form3.value.alarm_phone, verification_code: form3.value.identifier }).then((res: any) => {
            if (res.code == 200) {
                userList()
                ElMessage.success(res.msg)
                vvv.value = 0
                iphoneTrue.value = false;
                // 验证成功：有值自动置灰
                isEditingPhone.value = false
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
                // 验证成功：有值自动置灰
                isEditingEmail.value = false
            } else {
                ElMessage.error(res.msg)
            }
        })
    }
}

// 底部保存按钮（不变）
let submit = () => {
    set_alert_account({ user_id: user_id.value, alarm_phone: form3.value.alarm_phone, alarm_email: form3.value.alarm_email }).then((res: any) => {
        if (res.code == 200) {
            ElMessage.success(res.msg)
            props.config.isTrue = false
            // 保存成功：根据最终值重置所有状态
            isEditingPhone.value = !form3.value.alarm_phone
            isEditingEmail.value = !form3.value.alarm_email
        } else {
            ElMessage.error(res.msg)
        }
    })
}

// 其他函数（不变）
let forgotChange = () => {
    setToken("");
}

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

// 统一所有el-form-item的样式，确保垂直居中
:deep(.el-form-item) {
    display: flex;
    align-items: center;
    margin-bottom: 18px;
}

:deep(.el-form-item__label) {
    line-height: 32px;
}
</style>
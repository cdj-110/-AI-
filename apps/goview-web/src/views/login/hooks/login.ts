import router from "@/router";
import useCounterStore from "@/stores/counter";
import { login, sendcode, register, forgetpwd, loginCode, get_verification_code, loginrul, login_wechat, wechat_binding, set_pwd } from '@/api/login/index'
import { ElMessage } from 'element-plus'
import { Phone } from "@element-plus/icons-vue";
import { useI18n } from 'vue-i18n';
import { ref, onMounted, nextTick } from 'vue';
import { useRouter } from 'vue-router';

export default function loginFunction() {
    const { t } = useI18n();
    
    let formInline = ref({ username: "", password: "", verification_code: '', image_code: '' });//登录
    let recoverForm = ref({ username: "", iphone: "", phone: '', verification_code: "", password: "", confirm_password: "", email: "", type: false, account: '', VerificationMethod: 1 });//找回账号
    let loginRef: any = ref("");//登录ref
    let recoverFormRef: any = ref("");//忘记密码ref
    let enrollFormRef: any = ref("");//注册Ref
    let loginType = ref("pt");// 记录当前操作的状态：pt:登录 vx:微信扫码登录 wj:忘记密码 iphoneZc:手机号注册 yxZc:邮箱注册
    let recoverShip = ref(1);//记录当前忘记密码的步数
    let recoverType = ref(0);//通过哪种方式修改密码 0:没有选择 1:通过手机号修改密码 2:通过邮箱修改密码
    let recoverShet = ref(1);//通过哪种方式登录系统 1:账号密码 2:验证码 3:微信扫码手机号验证
    let verificationText = ref(t('login.getVerificationCode'));//验证码倒计时的文字
    let wjVerificationText = ref(t('login.getVerificationCode'));//忘记密码验证码倒计时的文字
    let zcVerificationText = ref(t('login.getVerificationCode'));//注册验证码倒计时的文字
    let emailRegex = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/  // 邮箱验证正则
    let phoneRegex = /^1[3-9]\d{9}$/ // 手机号验证正则（中国手机号）
    let { setToken } = useCounterStore();//实例化pinia函数
    let dialogVisible = ref(false)
    let imageVerificationText = ref("");//存储图片验证码
    let verifyKey = ref(0); // 用于强制刷新的 key
    let type = ref(0)
    const qrCodeUrl = ref<string>('')  // 存储二维码URL
    const loading = ref<boolean>(false)// 加载状态
    const sceneId = ref<string>('')
    let code = ref('') // 微信官方回调code
    let isCooldown = ref('') // 判断是否首次登录
    let token = ref('') // 微信首次登录获取存储token
    let wechat_avatar = ref('') // 微信登录头像
    let status_code = ref(3) // 微信扫码设置密码
    let user_name = ref('')
    let router = useRouter()
    const applyLoginToken = (res: any, fallbackAvatar = '') => {
        const data = res?.data || {}
        const loginToken = res?.token || data?.token
        const username = res?.username || data?.username || data?.user_name || ''
        const userId = res?.user_id || data?.user_id || data?.id || ''
        const avatar = res?.wechat_avatar || data?.wechat_avatar || fallbackAvatar

        if (!loginToken) {
            ElMessage.error('登录成功但接口未返回 token')
            console.warn('login response missing token', res)
            return false
        }

        setToken(loginToken, username, avatar, userId)
        localStorage.setItem('app_lock_screen_status', 'false')
        const targetUrl = `${window.location.origin}${window.location.pathname}#/workPageOverview`
        console.info('login token saved, forcing navigation to workPageOverview', {
            hasToken: !!sessionStorage.getItem('wk_Token'),
            userId,
            from: window.location.href,
            to: targetUrl
        })
        window.location.href = targetUrl
        setTimeout(() => {
            console.info('login navigation check', {
                href: window.location.href,
                hash: window.location.hash
            })
            if (!window.location.hash.includes('/workPageOverview')) {
                window.location.replace(targetUrl)
            }
        }, 50)
        return true
    }
    onMounted(() => {
        imageVerificationTextChange()
        setTimeout(() => {
            code.value = (location.search.split("&")[0]).slice(6)
            if (code.value) {
                wxPhoneverify()
            }
        }, 2000)
    })
    // 微信扫码登录判断有没有绑定手机号  有则登录  没有则弹窗绑定
    let wxPhoneverify = () => {
        login_wechat({ code: code.value }).then((res: any) => {
            if (res.code == 200) {
                isCooldown.value = res.user_id
                token.value = res.token
                wechat_avatar.value = res.wechat_avatar
                user_name.value = res.username
                if (res.is_cooldown == false) {
                    recoverShet.value = 3
                } else {
                    if (res.password_status == 0) {
                        recoverShet.value = 3
                        status_code.value = res.password_status

                    } else {
                        applyLoginToken(res, res.wechat_avatar);
                        ElMessage.success(res.msg)
                        let currentUrl = window.location.href // 获取当前URL
                        // 移除code和state参数
                        let urlWithoutParams = removeUrlParams(currentUrl, ['code', 'state', 'wechat_code'])
                        // 替换浏览器历史记录，不刷新页面
                        window.history.replaceState({}, document.title, urlWithoutParams)
                    }
                }
            }
        })
    }
    // 自定义验证函数 - 邮箱或手机号
    let validateAccount = (rule: any, value: string, callback: any) => {
        if (!value) {
            callback(new Error(t('login.errors.phoneOrEmailRequired')))
        } else if (!emailRegex.test(value) && !phoneRegex.test(value)) {
            callback(new Error(t('login.errors.invalidPhoneOrEmail')))
        } else {
            callback()
        }
    }
    // 自定义验证函数 - 邮箱
    let validateemail = (rule: any, value: string, callback: any) => {
        if (!value) {
            callback(new Error(t('login.errors.phoneOrEmailRequired')))
        } else if (!emailRegex.test(value) && !phoneRegex.test(value)) {
            callback(new Error(t('login.errors.invalidEmail')))
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

    // 自定义密码校验规则
    const validatePassword = (rule: any, value: any, callback: any) => {
        if (!value) {
            callback(new Error(t('login.errors.passwordRequired')))
        } else if (value.length < 8) {
            callback(new Error(t('login.errors.passwordMinLength')))
        } else if (value.length > 32) {
            callback(new Error(t('login.errors.passwordMaxLength')))
        } else if (!validatePasswordComplexity(value)) {
            callback(new Error(t('login.errors.passwordComplexity')))
        } else {
            callback()
        }
    }

    // 自定义确认密码校验规则
    const validateConfirmPassword = (rule: any, value: any, callback: any) => {
        if (!value) {
            callback(new Error(t('login.errors.confirmPasswordRequired')))
        } else if (value !== recoverForm.value.password) {
            callback(new Error(t('login.errors.confirmPasswordMismatch')))
        } else {
            callback()
        }
    }
    //验证账号是否为空
    let formInlineRule = ref({
        username: [{ required: true, message: t('login.errors.phoneOrEmailRequired'), trigger: "change" }, { validator: validateAccount, trigger: "change" }],
        phone: [{ required: true, message: t('login.errors.phoneRequired'), trigger: "change" }, { validator: validateAccount, trigger: "change" }],
        password: [{ required: true, message: t('login.errors.passwordRequired'), trigger: "change" }, { required: true, validator: validatePassword, trigger: 'change' }],
        iphone: [{ required: true, message: t('login.errors.phoneRequired'), trigger: "change" }],
        verification_code: [{ required: true, message: t('login.errors.verificationCodeRequired'), trigger: "change" }],
        confirm_password: [{ required: true, message: t('login.errors.confirmPasswordRequired'), trigger: "change" }, { validator: validateConfirmPassword, trigger: 'change' }],
        email: [{ required: true, message: t('login.errors.emailInvalid'), trigger: "change" }, { validator: validateemail, trigger: "change" }],
        image_code: [{ required: true, message: t('login.errors.imageCodeRequired'), trigger: "change" }],
        account: [{ required: true, message: t('login.errors.accountRequired'), trigger: "change" }, { validator: validateAccount, trigger: "change" }],
        VerificationMethod: [{ required: true, message: t('login.errors.verificationMethodRequired'), trigger: "change" }],
    })
    /**
     * 忘记密码
     */
    let recoveChange = () => {
        recoverFormRef.value.validate((vali: any) => {
            if (vali) {
                let params = {
                    new_password: recoverForm.value.password,
                    verification_code: recoverForm.value.verification_code,
                    confirm_password: recoverForm.value.confirm_password,
                    identifier: recoverForm.value.account
                }
                if (recoverType.value == 1) {
                    params.type = 1;
                    params.phone = recoverForm.value.phone
                } else if (recoverType.value == 2) {
                    params.type = 2;
                    params.email = recoverForm.value.email
                }
                forgetpwd(params).then((res: any) => {
                    if (res.code == 200) {
                        loginType.value = 'pt'
                        ElMessage.success(res.msg)
                    } else {
                        ElMessage.error(res.msg)
                    }
                })
            }
        })
    }

    /**
     * 注册 iphoneZc手机号 yxZc是邮箱
     */
    let enrollChange = () => {
        enrollFormRef.value.validate((vali: any) => {
            if (vali) {
                let params = {
                    password: recoverForm.value.password,
                    verification_code: recoverForm.value.verification_code,
                    confirm_password: recoverForm.value.confirm_password
                }
                if (loginType.value === 'iphoneZc') {
                    params.type = 1;
                    params.phone = recoverForm.value.phone
                } else if (loginType.value === 'yxZc') {
                    params.type = 2;
                    params.email = recoverForm.value.email
                }

                register(params).then((res: any) => {
                    if (res.code == 200) {
                        loginType.value = 'pt'
                        ElMessage.success(res.msg)
                    } else {
                        ElMessage.error(res.msg)
                    }
                })
            }
        })
    }
    /**
     * 验证方式
     */
    let VerificationClick = () => {
        recoverFormRef.value.validate((vali: any) => {
            if (vali) {
                if(recoverForm.value.VerificationMethod == 1 && !phoneRegex.test(recoverForm.value.account)){
                    ElMessage.error(t('login.errors.phoneInvalid'));
                }else if(recoverForm.value.VerificationMethod == 2 && !emailRegex.test(recoverForm.value.account)){
                    ElMessage.error(t('login.errors.emailInvalid'));
                }else{
                    recoverType.value = recoverForm.value.VerificationMethod
                    recoverShip.value = 3

                    if(recoverType.value == 1){
                        recoverForm.value.phone = recoverForm.value.account;
                    }else if(recoverType.value == 2){
                        recoverForm.value.email = recoverForm.value.account;
                    }
                }
            }
        })
    }

    /**
     * 登录
     */
    let loginChange = () => {
        loginRef.value.validate((vali: any) => {
            if (vali) {
                // 判断登录类型
                if (formInline.value.username.length === 11) {
                    // 手机号登录
                    type.value = 1;
                } else {
                    // 邮箱登录
                    type.value = 2;
                }
                if (recoverShet.value == 1) {
                    login(formInline.value).then((res: any) => {
                        if (res.code == 200) {
                            applyLoginToken(res);
                            ElMessage.success(res.msg)

                        } else {
                            ElMessage.error(res.msg)
                            imageVerificationTextChange()
                            formInline.value.image_code = ''
                        }
                    })
                } else if (recoverShet.value == 2) {
                    loginCode({ verification_code: formInline.value.verification_code, type: type.value, identifier: formInline.value.username, image_code: formInline.value.image_code }).then((res: any) => {
                        if (res.code == 200) {
                            // setToken(JSON.stringify({ token: res.token }));
                            applyLoginToken(res);
                            ElMessage.success(res.msg)

                        } else {
                            ElMessage.error(res.msg)
                            imageVerificationTextChange()
                            formInline.value.image_code = ''
                        }
                    })
                }
            }

        })
    }
    /**
   * 注册 邮箱注册 获取验证码
   */
    let zCsendClick = () => {
        let text = ''
        let params = {}
        if (loginType.value === 'iphoneZc') {
            params.type = 1;
            params.phone = recoverForm.value.phone
            text = recoverForm.value.phone
        } else if (loginType.value === 'yxZc') {
            params.type = 2;
            params.email = recoverForm.value.email
            text = recoverForm.value.email
        }
        if (text) {
            if (zcVerificationText.value == t('login.getVerificationCode')) {
                let vvv = 60;
                let ver = setInterval(() => {
                    if (vvv <= 0) {
                        vvv = 60;
                        clearInterval(ver);
                        zcVerificationText.value = t('login.getVerificationCode');
                    } else {
                        vvv--;
                        zcVerificationText.value = t('login.secondsLater', { seconds: vvv });
                    }
                }, 1000)
                sendcode(params).then((res: any) => {
                    if (res.code == 200) {
                        ElMessage.success(res.msg)
                    } else {
                        ElMessage.error(res.msg)
                    }
                })
            } else {
                ElMessage.error(t('login.errors.cooldown'));
            }
        } else {
            ElMessage.error(t('login.errors.phoneOrEmailEmpty'));
        }
    }
    /**
     * 获取验证码
     */
    let verificationChange = () => {
        let params = {}
        if (formInline.value.username.length === 11) {
            // 手机号登录
            params.type = 1;
            params.phone = formInline.value.username
        } else {
            // 邮箱登录
            params.type = 2;
            params.email = formInline.value.username
        }
        if (formInline.value.username) {
            if (verificationText.value == t('login.getVerificationCode')) {
                let vvv = 60;
                let ver = setInterval(() => {
                    if (vvv <= 0) {
                        vvv = 60;
                        clearInterval(ver);
                        verificationText.value = t('login.getVerificationCode');
                    } else {
                        vvv--;
                        verificationText.value = t('login.secondsLater', { seconds: vvv });
                    }
                }, 1000)
                sendcode(params).then((res: any) => {
                    if (res.code == 200) {
                        ElMessage.success(res.msg)
                    } else {
                        ElMessage.error(res.msg)
                    }
                })
            } else {
                ElMessage.error(t('login.errors.cooldown'));
            }
        } else {
            ElMessage.error(t('login.errors.phoneEmpty'));
        }
    }
    /**
     * 微信验证码
     */
    let vxVerification = () => {
        let params = {
            type: 1,
            phone: recoverForm.value.phone
        }
        if (recoverForm.value.phone) {
            if (verificationText.value == t('login.getVerificationCode')) {
                let vvv = 60;
                let ver = setInterval(() => {
                    if (vvv <= 0) {
                        vvv = 60;
                        clearInterval(ver);
                        verificationText.value = t('login.getVerificationCode');
                    } else {
                        vvv--;
                        verificationText.value = t('login.secondsLater', { seconds: vvv });
                    }
                }, 1000)
                sendcode(params).then((res: any) => {
                    if (res.code == 200) {
                        ElMessage.success(res.msg)
                    } else {
                        ElMessage.error(res.msg)
                    }
                })
            } else {
                ElMessage.error(t('login.errors.cooldown'));
            }
        } else {
            ElMessage.error(t('login.errors.phoneEmpty'));
        }

    }
    /**
    * 忘记密码 手机号注册 获取验证码
    */
    let sendClick = () => {
        let params = {}
        let text = ''
        if (recoverType.value == 1) {
            params.phone = recoverForm.value.phone
            params.type = 1;
            text = recoverForm.value.phone
        } else if (recoverType.value == 2) {
            params.email = recoverForm.value.email
            params.type = 2;
            text = recoverForm.value.email
        }
        if (text) {
            if (wjVerificationText.value == t('login.getVerificationCode')) {
                let vvv = 60;
                let ver = setInterval(() => {
                    if (vvv <= 0) {
                        vvv = 60;
                        clearInterval(ver);
                        wjVerificationText.value = t('login.getVerificationCode');
                    } else {
                        vvv--;
                        wjVerificationText.value = t('login.secondsLater', { seconds: vvv });
                    }
                }, 1000)
                sendcode(params).then((res: any) => {
                    if (res.code == 200) {
                        ElMessage.success(res.msg)
                    } else {
                        ElMessage.error(res.msg)
                    }
                })
            } else {
                ElMessage.error(t('login.errors.cooldown'));
            }
        }
    }
    let goBack = () => {
        loginType.value = 'pt'
        recoverForm.value = {}
        formInline.value = {}

    }
    let ZcPhone = () => {
        loginType.value = 'iphoneZc'
        recoverForm.value = {}
    }
    let zCemail = () => {
        loginType.value = 'yxZc'
        recoverForm.value = {}
    }
    /**
    * 改变图片验证码
    */
    let imageVerificationTextChange = () => {
        get_verification_code().then(async (res: any) => {
            imageVerificationText.value = res.data.base64

        })
    }
    /**
     * 弹窗打开时的处理
     */
    let handleDialogOpen = () => {
        // 每次打开弹窗时，改变 key 值，强制重新创建验证组件
        verifyKey.value++;
    };

    let createWeChatIframe = (url: string) => {
        nextTick(() => {
            const container = document.getElementById('wx_login_container')
            if (container) {
                container.innerHTML = ''
                const iframe = document.createElement('iframe')
                iframe.src = url
                iframe.width = '300'  // 改为 150（原为300）
                iframe.height = '400' // 改为 200（原为400）
                iframe.frameBorder = '0'
                iframe.style.border = 'none'
                iframe.style.borderRadius = '8px'
                container.appendChild(iframe)
            }
        })
    }
    // 辅助函数：从URL中移除指定参数
    let removeUrlParams = (url: any, paramsToRemove: any) => {
        let urlObj = new URL(url)
        paramsToRemove.forEach((param: any) => {
            urlObj.searchParams.delete(param)
        })

        return urlObj.toString()
    }
    /**
    * 获取微信二维码的函数
    */
    let vxClick = () => {
        loginrul().then((res: any) => {
            window.location.href = res.data
            createWeChatIframe(res.data)
        })
    }
    /**
     * 绑定
     */
    let bind = () => {
        enrollFormRef.value.validate((vali: any) => {
            if (vali) {
                if (recoverForm.value.type == true) {
                    let params = {
                        phone: recoverForm.value.phone,
                        verification_code: recoverForm.value.verification_code,
                        user_id: isCooldown.value,
                    }
                    wechat_binding(params).then((res: any) => {
                        if (res.code == 200) {
                            if (res.password_status == 0) {
                                status_code.value = res.password_status
                            } else {
                                setToken(token.value, res.username, wechat_avatar.value, res.user_id);
                                ElMessage.success(res.msg)
                                let currentUrl = window.location.href // 获取当前URL
                                // 移除code和state参数
                                let urlWithoutParams = removeUrlParams(currentUrl, ['code', 'state', 'wechat_code'])
                                // 替换浏览器历史记录，不刷新页面
                                window.history.replaceState({}, document.title, urlWithoutParams)
                            }
                        } else {
                            ElMessage.error(res.msg)
                        }
                    })
                } else {
                    ElMessage.warning(t('login.errors.agreeTerms'))
                }

            }
        })

    }
    /**
     * 微信扫码密码登录
     */
    let newPpassword = () => {
        enrollFormRef.value.validate((vali: any) => {
            if (vali) {
                if (recoverForm.value.type == true) {
                    let params = {
                        user_id: isCooldown.value,
                        password: recoverForm.value.password,
                    }
                    set_pwd(params).then((res: any) => {
                        if (res.code == 200) {
                            if (res.password_status == 0) {
                                status_code.value = res.password_status
                            } else {
                                setToken(token.value, user_name.value, wechat_avatar.value, isCooldown.value);
                                ElMessage.success(res.msg)
                                let currentUrl = window.location.href // 获取当前URL
                                // 移除code和state参数
                                let urlWithoutParams = removeUrlParams(currentUrl, ['code', 'state', 'wechat_code'])
                                // 替换浏览器历史记录，不刷新页面
                                window.history.replaceState({}, document.title, urlWithoutParams)
                            }
                        } else {
                            ElMessage.error(res.msg)
                        }
                    })
                } else {
                    ElMessage.warning(t('login.errors.agreeTerms'))
                }
            }
        })
    }
    return {
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
        qrCodeUrl,
        sceneId,
        loading,
        zcVerificationText,
        status_code,
        newPpassword,
        vxVerification,
        handleDialogOpen,
        zCsendClick,
        sendClick,
        recoveChange,
        enrollChange,
        loginChange,
        verificationChange,
        ZcPhone,
        zCemail,
        goBack,
        imageVerificationTextChange,
        vxClick,
        bind,
        VerificationClick
    }
}

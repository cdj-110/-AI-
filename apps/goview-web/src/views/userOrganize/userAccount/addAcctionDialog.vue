<template>
    <div>
        <el-dialog v-model="dialogVisible" :title="props.title" width="500" @close="emit('handleClose', false)">
            <div style="padding: 0.5rem 1rem;">
                <el-form ref="ruleFormRef" style="max-width: 600px" :model="ruleForm" :rules="rules" label-width="auto">
                    <el-form-item :label="$t('userAccount.userName')" prop="a">
                        <el-input v-model="ruleForm.a" :placeholder="$t('userAccount.pleaseEnterUsername')" />
                    </el-form-item>
                    <el-form-item :label="$t('userAccount.phone')" prop="b" v-if="type == 1 && obj">
                        <el-input v-model="ruleForm.b" :placeholder="$t('userAccount.pleaseEnterPhoneOrEmail')" />
                    </el-form-item>
                    <el-form-item :label="$t('userAccount.email')" prop="b" v-if="type == 2 && obj">
                        <el-input v-model="ruleForm.b" :placeholder="$t('userAccount.email')" />
                    </el-form-item>
                    <el-form-item :label="$t('userAccount.verificationCode')" prop="c">
                        <el-input v-model="ruleForm.c" :placeholder="$t('userAccount.pleaseEnterVerificationCode')">
                            <template #append>
                                <span>{{ $t('userAccount.pleaseEnterVerificationCode') }}</span>
                            </template>
                        </el-input>
                    </el-form-item>
                    <el-form-item :label="$t('userAccount.organization')" prop="d">
                        <el-select v-model="ruleForm.d" :placeholder="$t('userAccount.pleaseSelectOrganization')">
                            <el-option :label="$t('userAccount.zoneOne')" value="1" />
                            <el-option :label="$t('userAccount.zoneTwo')" value="2" />
                        </el-select>
                    </el-form-item>
                    <el-form-item :label="$t('userAccount.password')" prop="password">
                        <el-input v-model="ruleForm.password" :placeholder="$t('userAccount.defaultPassword')" clearable show-password
                            autocomplete="new-password">
                        </el-input>
                    </el-form-item>
                </el-form>
            </div>
            <template #footer>
                <div style="display: flex;justify-content: center;margin: 2rem 0;">
                    <el-button @click="emit('handleClose', false)">{{ $t('common.cancel') }}</el-button>
                    <el-button type="primary" @click="submitForm(ruleFormRef)">{{ $t('common.confirm') }}</el-button>
                </div>
            </template>
        </el-dialog>
    </div>
</template>
<script lang="ts" setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
    title: {
        type: String,
        required: false
    },
    obj: {
        type: Object,
        required: false
    },
});
let emit = defineEmits(["handleClose"]);
import type { FormInstance, FormRules } from 'element-plus'
let ruleFormRef = ref<FormInstance>()
let dialogVisible = ref(true)
let ruleForm = ref({ a: '', b: '', c: '', d: '', password: 'wk123456' })
let type = ref(1)

const validatePassword = (rule: any, value: any, callback: any) => {
    if (!value) {
        callback();
        return;
    }
    if (value.length < 8) {
        callback(new Error(t('userAccount.passwordLengthError')));
    } else if (value.length > 32) {
        callback(new Error(t('userAccount.passwordLengthMaxError')));
    } else if (!validatePasswordComplexity(value)) {
        callback(new Error(t('userAccount.passwordComplexityError')));
    } else {
        callback();
    }
}

const validatePasswordComplexity = (password: string): boolean => {
    const hasUpperCase = /[A-Z]/.test(password)
    const hasLowerCase = /[a-z]/.test(password)
    const hasSpecialChar = /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(password)

    return hasUpperCase || hasLowerCase || hasSpecialChar
}

let rules = ref<FormRules>({
    a: [{ required: true, message: t('userAccount.pleaseEnterUsername'), trigger: 'change' }],
    b: [{ required: true, message: t('userAccount.pleaseEnterAccount'), trigger: 'change' }],
    c: [{ required: true, message: t('userAccount.pleaseEnterVerificationCode'), trigger: 'change' }],
    d: [{ required: true, message: t('userAccount.pleaseSelectOrganization'), trigger: 'change' }],
    password: [{ validator: validatePassword, trigger: 'change' }],
})

// 监听 obj 的变化，当 obj 有值时初始化表格数据
watch(() => props.obj, (newObj) => {
    if (newObj) {
        console.log(newObj.b,'newObj.b.')
        if(!newObj.b){

        } else if (newObj.b.length === 11) {
            // 手机号登录
            type.value = 1;
        } else {
            // 邮箱登录
            type.value = 2;
        }
        ruleForm.value = props.obj
    } else {
        ruleForm.value = {}
    }
}, { immediate: true })

// 监听title
watch(() => props.title, (newTitle) => {
    if (newTitle == '新增账号') {
        type.value = 1;
        ruleForm.value = {}
        ruleForm.value.password = 'wk123456'
    }
}, { immediate: true }) // 添加这个选项

let submitForm = async (formEl: FormInstance | undefined) => {
    if (!formEl) return
    await formEl.validate((valid, fields) => {
        if (valid) {
            console.log('submit!')
        } else {
            console.log('error submit!', fields)
        }
    })
}
</script>
<style lang="scss" scoped>
:deep(.el-select__placeholder),
:deep(.el-input__inner) {
    font-size: 12px;
}

:deep(.el-button>span) {
    font-size: 13px;
}
</style>
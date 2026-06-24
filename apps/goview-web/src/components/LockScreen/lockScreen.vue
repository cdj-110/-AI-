<template>
    <div v-if="isVisible" class="lock-screen-mask">
        <div class="lock-screen-container">
            <!-- 锁形图标 -->
            <div class="lock-icon">
                <svg viewBox="0 0 1024 1024" width="64" height="64">
                    <path
                        d="M832 480H672V320c0-70.4-57.6-128-128-128s-128 57.6-128 128v160H192c-17.6 0-32 14.4-32 32v384c0 17.6 14.4 32 32 32h640c17.6 0 32-14.4 32-32V512c0-17.6-14.4-32-32-32zM512 192c35.2 0 64 28.8 64 64v160H448V320c0-35.2 28.8-64 64-64z"
                        fill="#409eff"></path>
                </svg>
            </div>

            <div class="lock-title">页面已锁定</div>
            <div class="lock-subtitle">请输入密码解锁</div>

            <!-- 密码输入框 -->
            <div class="input-wrapper">
                <el-input v-model="inputPassword" type="password" placeholder="请输入解锁密码" @keyup.enter="handleUnlock"
                    :class="{ 'input-error': errorTip }" />
                <div class="error-tip" v-if="errorTip">{{ errorTip }}</div>
            </div>

            <!-- 解锁按钮 -->
            <el-button type="primary" @click="handleUnlock" class="unlock-btn">
                解锁
            </el-button>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps({
    isVisible: {
        type: Boolean,
        default: false,
    },
    correctPassword: {
        type: String,
        required: true,
    },
});

const emit = defineEmits(['unlockSuccess']);

const inputPassword = ref('');
const errorTip = ref('');

const handleUnlock = () => {
    if (!inputPassword.value) {
        errorTip.value = '请输入解锁密码';
        return;
    }
    if (inputPassword.value === props.correctPassword) {
        errorTip.value = '';
        inputPassword.value = '';
        emit('unlockSuccess'); // 只有密码正确才触发解锁
    } else {
        errorTip.value = '密码错误，请重新输入';
        inputPassword.value = '';
    }
};
</script>

<style scoped lang="scss">
.lock-screen-mask {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: linear-gradient(135deg, rgba(0, 0, 0, 0.85) 0%, rgba(20, 20, 30, 0.9) 100%);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 9999;
    animation: fadeIn 0.3s ease-out;
}

@keyframes fadeIn {
    from {
        opacity: 0;
    }

    to {
        opacity: 1;
    }
}

.lock-screen-container {
    width: 380px;
    padding: 40px 32px;
    background: rgba(255, 255, 255, 0.95);
    backdrop-filter: blur(10px);
    border-radius: 16px;
    text-align: center;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    animation: scaleIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes scaleIn {
    from {
        opacity: 0;
        transform: scale(0.9) translateY(10px);
    }

    to {
        opacity: 1;
        transform: scale(1) translateY(0);
    }
}

.lock-icon {
    margin-bottom: 20px;
    animation: lockBounce 0.6s ease-out;
}

@keyframes lockBounce {
    0% {
        transform: scale(0);
    }

    60% {
        transform: scale(1.1);
    }

    100% {
        transform: scale(1);
    }
}

.lock-title {
    font-size: 24px;
    font-weight: 600;
    color: #1d2129;
    margin-bottom: 8px;
}

.lock-subtitle {
    font-size: 14px;
    color: #86909c;
    margin-bottom: 32px;
}

.input-wrapper {
    margin-bottom: 24px;
    text-align: left;
}

:deep(.el-input__wrapper) {
    padding: 12px 16px;
    border-radius: 8px;
    box-shadow: 0 0 0 1px #d0d7de;
    transition: all 0.2s ease;

    &:hover {
        box-shadow: 0 0 0 1px #409eff;
    }
}

:deep(.input-error .el-input__wrapper) {
    box-shadow: 0 0 0 1px #f53f3f;
}

.error-tip {
    color: #f53f3f;
    font-size: 12px;
    margin-top: 8px;
    padding-left: 4px;
    animation: shake 0.3s ease-in-out;
}

@keyframes shake {

    0%,
    100% {
        transform: translateX(0);
    }

    25% {
        transform: translateX(-5px);
    }

    75% {
        transform: translateX(5px);
    }
}

.unlock-btn {
    width: 100%;
    height: 48px;
    font-size: 16px;
    font-weight: 500;
    border-radius: 8px;
    background: linear-gradient(90deg, #409eff 0%, #337ecc 100%);
    border: none;
    transition: all 0.2s ease;

    &:hover {
        transform: translateY(-2px);
        box-shadow: 0 8px 16px rgba(64, 158, 255, 0.3);
    }

    &:active {
        transform: translateY(0);
    }
}
</style>
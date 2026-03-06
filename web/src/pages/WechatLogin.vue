<template>
  <div class="min-h-screen bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 flex items-center justify-center p-4">
    <div class="absolute top-4 right-4">
      <router-link
        to="/admin/login"
        class="px-3 py-1.5 rounded-md text-sm text-gray-400 hover:bg-gray-800 hover:text-white transition-colors"
      >
        管理员登录
      </router-link>
    </div>
    
    <div class="max-w-md w-full">
      <div class="text-center mb-8">
        <div class="mx-auto flex items-center justify-center h-16 w-16 rounded-2xl bg-green-600/20 mb-4">
          <svg class="h-8 w-8 text-green-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z" />
          </svg>
        </div>
        <h2 class="text-3xl font-bold text-white">微信扫码登录</h2>
        <p class="mt-2 text-gray-400">使用微信扫码登录您的账号</p>
      </div>

      <div v-if="loading" class="bg-gray-800/50 backdrop-blur-sm rounded-2xl p-8 shadow-2xl border border-gray-700/50">
        <div class="flex flex-col items-center justify-center py-8">
          <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-green-500"></div>
          <p class="mt-4 text-gray-400">正在生成登录二维码...</p>
        </div>
      </div>

      <div v-else-if="loginUrl" class="bg-gray-800/50 backdrop-blur-sm rounded-2xl p-8 shadow-2xl border border-gray-700/50">
        <div class="flex justify-center">
          <div class="w-64 h-64 bg-white rounded-xl flex items-center justify-center p-4 shadow-lg">
            <img :src="qrcodeUrl" alt="微信登录二维码" class="max-w-full max-h-full" />
          </div>
        </div>
        <p class="mt-6 text-center text-gray-300">请使用微信扫码登录</p>
        <button @click="refreshQrCode" class="mt-4 w-full py-3 px-4 bg-gradient-to-r from-green-600 to-green-700 hover:from-green-700 hover:to-green-800 text-white font-medium rounded-lg shadow-lg shadow-green-600/25 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-800 focus:ring-green-500 transition-all duration-200">
          刷新二维码
        </button>
      </div>

      <div v-else class="bg-gray-800/50 backdrop-blur-sm rounded-2xl p-8 shadow-2xl border border-gray-700/50">
        <button @click="generateLoginUrl" class="w-full py-3 px-4 bg-gradient-to-r from-green-600 to-green-700 hover:from-green-700 hover:to-green-800 text-white font-medium rounded-lg shadow-lg shadow-green-600/25 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-800 focus:ring-green-500 transition-all duration-200">
          开始微信登录
        </button>
      </div>

      <div v-if="error" class="mt-4 bg-red-500/10 border border-red-500/20 rounded-lg p-4">
        <div class="flex items-center">
          <svg class="h-5 w-5 text-red-400 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="text-sm text-red-400">{{ error }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { getWechatLoginURL } from '../lib/api'
import { setToken, setUser } from '../lib/auth'
import { useRouter } from 'vue-router'
import { useAuth } from '../hooks/useAuth'
import { useStore } from '@/store'

const router = useRouter()
const { isAuthenticated } = useAuth()
const loading = ref(false)
const loginUrl = ref<string>('')
const qrcodeUrl = ref<string>('')
const error = ref<string>('')
const pollingInterval = ref<number | null>(null)

const generateLoginUrl = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await getWechatLoginURL()
    // 检查接口返回状态是否成功
    if (!response.login_url) {
      throw new Error('生成登录链接失败')
    }
    loginUrl.value = response.login_url
    // 生成二维码图片（实际项目中可以使用qrcode库）
    qrcodeUrl.value = `https://api.qrserver.com/v1/create-qr-code/?size=256x256&data=${encodeURIComponent(response.login_url)}`
  } catch (err: any) {
    error.value = err.message || '生成登录链接失败'
  } finally {
    loading.value = false
  }
}

const refreshQrCode = () => {
  generateLoginUrl()
}

// 检查URL参数中是否有token（微信回调后）
const checkCallback = () => {
  const urlParams = new URLSearchParams(window.location.search)
  const token = urlParams.get('token')
  if (token) {
    setToken(token)
    const store = useStore()
    store.setIsLogin(true)
    store.setIsAdmin(false)
    isAuthenticated.value = true
    router.push('/')
    return true
  }
  return false
}

// 建立WebSocket连接
let ws: WebSocket | null = null

const connectWebSocket = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/ws/chat`
  
  ws = new WebSocket(wsUrl)
  
  ws.onopen = function() {
  }
  
  ws.onmessage = function(event) {
    try {
      const data = JSON.parse(event.data)
      if (data.type === 'login.success') {
          setToken(data.token)
          setUser(data.user)
          const store = useStore()
          store.setIsLogin(true)
          store.setIsAdmin(false)
          isAuthenticated.value = true
          setTimeout(() => {
            router.push('/')
          }, 100)
          stopPolling()
          if (ws) {
            ws.close()
          }
        }
    } catch (err) {
      console.error('WebSocket消息解析失败:', err)
    }
  }
  
  ws.onerror = function(error) {
    console.error('WebSocket错误:', error)
  }
  
  ws.onclose = function() {
  }
}

// 轮询检测登录状态
const startPolling = () => {
  // 建立WebSocket连接
  connectWebSocket()
  
  // 每2秒检查一次URL参数（作为备用方案）
  pollingInterval.value = window.setInterval(() => {
    // 检查URL参数
    if (checkCallback()) {
      stopPolling()
      return
    }
  }, 2000)
}

const stopPolling = () => {
  if (pollingInterval.value) {
    window.clearInterval(pollingInterval.value)
    pollingInterval.value = null
  }
}

onMounted(() => {
  // 初始检查一次
  if (!checkCallback()) {
    // 如果没有token，开始轮询
    startPolling()
  }
  generateLoginUrl()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
/* 自定义样式 */
</style>
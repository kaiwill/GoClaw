<template>
  <div class="min-h-screen bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 py-8">
    <div class="max-w-4xl mx-auto px-4">
      <div class="bg-gray-800/50 backdrop-blur-sm rounded-2xl shadow-xl overflow-hidden border border-gray-700/50">
        <div class="bg-gradient-to-r from-gray-800 to-gray-900 px-8 py-6 border-b border-gray-700">
          <h1 class="text-3xl font-bold text-white">用户中心</h1>
        </div>
        
        <div v-if="loading" class="px-8 py-16 flex justify-center">
          <div class="animate-spin rounded-full h-16 w-16 border-2 border-blue-500 border-t-transparent"></div>
        </div>
        
        <div v-else-if="user" class="px-8 py-8">
          <div class="mb-10">
            <h2 class="text-2xl font-bold text-white mb-6 flex items-center">
              <span class="mr-3 text-blue-400">👤</span>
              个人信息
            </h2>
            <div class="flex items-center space-x-8 bg-gray-900/50 p-6 rounded-xl border border-gray-700">
              <div class="w-32 h-32 rounded-full overflow-hidden border-4 border-gray-700 shadow-lg">
                <img :src="user.avatar || 'https://via.placeholder.com/200'" alt="头像" class="w-full h-full object-cover" />
              </div>
              <div class="space-y-3">
                <h3 class="text-2xl font-bold text-white">{{ user.nickname }}</h3>
                <p class="text-gray-300 flex items-center">
                   <span class="mr-2 text-blue-400">📧</span>
                   {{ user.email || '未设置邮箱' }}
                 </p>
                 <p class="text-gray-400 text-sm flex items-center">
                   <span class="mr-2 text-gray-500">📅</span>
                   注册时间: {{ formatDate(user.created_at) }}
                 </p>
                 <p class="text-gray-400 text-sm flex items-center">
                   <span class="mr-2 text-gray-500">🏷️</span>
                   状态: <span :class="getUserStatusClass(user.status)">{{ getUserStatus(user.status) }}</span>
                 </p>
              </div>
            </div>
          </div>
          <div class="mb-8">
            <h2 class="text-2xl font-bold text-white mb-6 flex items-center">
              <span class="mr-3 text-blue-400">✏️</span>
              修改信息
            </h2>
            <form @submit.prevent="updateUserInfo" class="space-y-6">
              <div class="bg-gray-900/50 p-6 rounded-xl border border-gray-700">
                <label for="email" class="block text-sm font-semibold text-gray-300 mb-2">邮箱</label>
                <input 
                  type="email" 
                  id="email" 
                  v-model="formData.email" 
                  class="w-full px-4 py-3 bg-gray-700/50 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all duration-300"
                  placeholder="请输入邮箱"
                />
              </div>
              <div v-if="false" class="bg-gray-900/50 p-6 rounded-xl border border-gray-700">
                <label for="avatar" class="block text-sm font-semibold text-gray-300 mb-2">头像URL</label>
                <input 
                  type="text" 
                  id="avatar" 
                  v-model="formData.avatar" 
                  class="w-full px-4 py-3 bg-gray-700/50 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all duration-300"
                  placeholder="请输入头像URL"
                />
              </div>
              <div class="flex space-x-4">
                <button 
                  type="submit" 
                  :disabled="submitting"
                  class="flex-1 px-6 py-3 bg-blue-600 text-white rounded-lg font-semibold hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 transition-all duration-300 transform hover:scale-105"
                >
                  <span v-if="!submitting" class="mr-2">💾</span>
                  {{ submitting ? '保存中...' : '保存修改' }}
                </button>
                <button 
                  type="button" 
                  @click="resetForm"
                  :disabled="submitting"
                  class="px-6 py-3 bg-gray-700 text-gray-300 rounded-lg font-semibold hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-500 disabled:opacity-50 transition-all duration-300"
                >
                  <span class="mr-2">🔄</span>
                  重置
                </button>
              </div>
            </form>
          </div>
        </div>
        
        <div v-else class="px-8 py-16 text-center">
          <div class="text-6xl mb-4">❌</div>
          <p class="text-gray-300 text-lg mb-6">获取用户信息失败</p>
          <button 
            @click="fetchUserInfo"
            class="px-6 py-3 bg-blue-600 text-white rounded-lg font-semibold hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 transition-all duration-300"
          >
            <span class="mr-2">🔄</span>
            重新获取
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getUserInfo, updateUserInfo as apiUpdateUserInfo } from '../lib/api'
import { useToast } from '../hooks/useToast'

const loading = ref(true)
const submitting = ref(false)
const user = ref<any>(null)
const formData = ref({
  email: '',
  avatar: ''
})

const { success, error } = useToast()

const fetchUserInfo = async () => {
  loading.value = true
  try {
    const response = await getUserInfo()
    user.value = response.user
    formData.value = {
      email: user.value.email || '',
      avatar: user.value.avatar || ''
    }
  } catch (err) {
    console.error('获取用户信息失败:', err)
    error('获取用户信息失败')
  } finally {
    loading.value = false
  }
}

const updateUserInfo = async () => {
  submitting.value = true
  try {
    const response = await apiUpdateUserInfo(formData.value)
    user.value = response.user
    success('信息更新成功')
  } catch (err: any) {
    error('更新失败: ' + (err.message || '未知错误'))
  } finally {
    submitting.value = false
  }
}

const resetForm = () => {
  if (user.value) {
    formData.value = {
      email: user.value.email || '',
      avatar: user.value.avatar || ''
    }
  }
}

const formatDate = (dateString: string) => {
  const date = new Date(dateString)
  return date.toLocaleString()
}

const getUserStatus = (status: number) => {
  switch (status) {
    case 0: return '待审核'
    case 1: return '已通过'
    case 2: return '已拒绝'
    default: return '未知'
  }
}

const getUserStatusClass = (status: number) => {
  switch (status) {
    case 0: return 'text-yellow-400 font-semibold'
    case 1: return 'text-green-400 font-semibold'
    case 2: return 'text-red-400 font-semibold'
    default: return 'text-gray-400'
  }
}

onMounted(() => {
  fetchUserInfo()
})
</script>

<style scoped>
/* 自定义样式 */
</style>
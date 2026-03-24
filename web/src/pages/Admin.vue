<template>
  <div class="min-h-screen bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 py-8">
    <div class="max-w-7xl mx-auto px-4">
      <div class="bg-gray-800/50 backdrop-blur-sm rounded-2xl shadow-xl overflow-hidden border border-gray-700/50">
        <div class="bg-gradient-to-r from-gray-800 to-gray-900 px-8 py-6 border-b border-gray-700">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <span class="text-3xl">🛡️</span>
              <h1 class="text-3xl font-bold text-white">管理员管理</h1>
            </div>
            <button 
              @click="activeTab = 'password'" 
              class="flex items-center gap-2 px-4 py-2 bg-gray-700/50 text-gray-300 rounded-lg hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 transition-all duration-300"
            >
              <span>🔐</span>
              修改密码
            </button>
          </div>
        </div>
        
        <div class="px-8 py-8">
          <div v-if="activeTab === 'users'">
            <h2 class="text-2xl font-bold text-white mb-6 flex items-center">
              <span class="mr-3 text-blue-400">👥</span>
              用户审核
            </h2>
            
            <div v-if="loading" class="flex justify-center py-16">
              <div class="animate-spin rounded-full h-16 w-16 border-2 border-blue-500 border-t-transparent"></div>
            </div>
            
            <div v-else-if="users.length > 0" class="overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-700">
                <thead class="bg-gray-900/50">
                  <tr>
                    <th scope="col" class="px-6 py-4 text-left text-xs font-semibold text-gray-400 uppercase tracking-wider">
                      用户名
                    </th>
                    <th scope="col" class="px-6 py-4 text-left text-xs font-semibold text-gray-400 uppercase tracking-wider">
                      邮箱
                    </th>
                    <th scope="col" class="px-6 py-4 text-left text-xs font-semibold text-gray-400 uppercase tracking-wider">
                      状态
                    </th>
                    <th scope="col" class="px-6 py-4 text-left text-xs font-semibold text-gray-400 uppercase tracking-wider">
                      注册时间
                    </th>
                    <th scope="col" class="px-6 py-4 text-left text-xs font-semibold text-gray-400 uppercase tracking-wider">
                      操作
                    </th>
                  </tr>
                </thead>
                <tbody class="bg-gray-800/30 divide-y divide-gray-700">
                  <tr v-for="user in users" :key="user.id" class="hover:bg-gray-700/30 transition-colors duration-200">
                    <td class="px-6 py-4 whitespace-nowrap">
                      <div class="flex items-center">
                        <div class="flex-shrink-0 h-12 w-12">
                          <img :src="user.avatar || 'https://via.placeholder.com/48'" alt="" class="h-12 w-12 rounded-full border-2 border-gray-600">
                        </div>
                        <div class="ml-4">
                          <div class="text-sm font-semibold text-white">{{ user.nickname }}</div>
                        </div>
                      </div>
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap">
                      <div class="text-sm text-gray-300">{{ user.email || '未设置' }}</div>
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap">
                      <span :class="getUserStatusClass(user.status)" class="px-3 py-1 inline-flex text-xs leading-5 font-semibold rounded-full">
                        {{ getUserStatus(user.status) }}
                      </span>
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-400">
                      {{ formatDate(user.created_at) }}
                    </td>
                    <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                      <button 
                        v-if="user.status === 0" 
                        @click="approveUser(user.id, 1)"
                        class="text-green-400 hover:text-green-300 mr-4 transition-colors duration-200 flex items-center gap-1"
                      >
                        <span>✅</span>
                        通过
                      </button>
                      <button 
                        v-if="user.status === 0" 
                        @click="approveUser(user.id, 2)"
                        class="text-red-400 hover:text-red-300 transition-colors duration-200 flex items-center gap-1"
                      >
                        <span>❌</span>
                        拒绝
                      </button>
                      <span v-if="user.status !== 0" class="text-gray-500">-</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            
            <div v-else class="text-center py-16">
              <div class="text-6xl mb-4">📭</div>
              <p class="text-gray-400 text-lg">暂无用户</p>
            </div>
          </div>
          
          <div v-if="activeTab === 'password'">
            <h2 class="text-2xl font-bold text-white mb-6 flex items-center">
              <span class="mr-3 text-blue-400">🔐</span>
              修改密码
            </h2>
            <form @submit.prevent="changePassword" class="space-y-6 max-w-md">
              <div class="bg-gray-900/50 p-6 rounded-xl border border-gray-700">
                <label for="old_password" class="block text-sm font-semibold text-gray-300 mb-2">旧密码</label>
                <input 
                  type="password" 
                  id="old_password" 
                  v-model="passwordForm.old_password" 
                  required
                  class="w-full px-4 py-3 bg-gray-800 border border-gray-700 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all duration-300"
                  placeholder="请输入旧密码"
                />
              </div>
              <div class="bg-gray-900/50 p-6 rounded-xl border border-gray-700">
                <label for="new_password" class="block text-sm font-semibold text-gray-300 mb-2">新密码</label>
                <input 
                  type="password" 
                  id="new_password" 
                  v-model="passwordForm.new_password" 
                  required
                  class="w-full px-4 py-3 bg-gray-800 border border-gray-700 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all duration-300"
                  placeholder="请输入新密码"
                />
              </div>
              <div class="flex space-x-4">
                <button 
                  type="submit" 
                  :disabled="submitting"
                  class="flex-1 px-6 py-3 bg-blue-600 text-white rounded-lg font-semibold hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 transition-all duration-300 transform hover:scale-105"
                >
                  <span v-if="!submitting" class="mr-2">💾</span>
                  {{ submitting ? '修改中...' : '修改密码' }}
                </button>
                <button 
                  type="button" 
                  @click="activeTab = 'users'"
                  class="px-6 py-3 bg-gray-700 text-gray-300 rounded-lg font-semibold hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-gray-500 transition-all duration-300"
                >
                  <span class="mr-2">🔄</span>
                  取消
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getAdminUsers, approveUser as apiApproveUser, changeAdminPassword } from '../lib/api'
import { useToast } from '../hooks/useToast'

const { success, error } = useToast()

const activeTab = ref('users')
const loading = ref(true)
const submitting = ref(false)
const users = ref<any[]>([])
const passwordForm = ref({
  old_password: '',
  new_password: ''
})

const fetchUsers = async () => {
  loading.value = true
  try {
    const response = await getAdminUsers()
    users.value = response.users
  } catch (err) {
    console.error('获取用户列表失败:', err)
    error('获取用户列表失败')
  } finally {
    loading.value = false
  }
}

const approveUser = async (userId: number, status: number) => {
  try {
    await apiApproveUser({ user_id: userId, status })
    fetchUsers()
    success('操作成功')
  } catch (err: any) {
    error('操作失败: ' + (err.message || '未知错误'))
  }
}

const changePassword = async () => {
  submitting.value = true
  try {
    await changeAdminPassword(passwordForm.value)
    success("密码修改成功!")
    passwordForm.value = {
      old_password: '',
      new_password: ''
    }
    activeTab.value = 'users'
  } catch (err: any) {
    error('修改失败: ' + (err.message || '未知错误'))
  } finally {
    submitting.value = false
  }
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
    case 0: return 'bg-yellow-900/50 text-yellow-300 border border-yellow-700/50'
    case 1: return 'bg-green-900/50 text-green-300 border border-green-700/50'
    case 2: return 'bg-red-900/50 text-red-300 border border-red-700/50'
    default: return 'bg-gray-900/50 text-gray-300 border border-gray-700/50'
  }
}

const formatDate = (dateString: string) => {
  const date = new Date(dateString)
  return date.toLocaleString()
}

onMounted(() => {
  fetchUsers()
})
</script>

<style scoped>
</style>

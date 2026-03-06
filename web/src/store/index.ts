import { defineStore } from 'pinia'
import type { StatusResponse } from '../types/api'

export const useStore = defineStore('main', {
  state: () => ({
    status: null as StatusResponse | null,
    isLogin: false,
    isAdmin: false
  }),
  actions: {
    setStatus(status: StatusResponse) {
      // 计算 loginMode
      status.loginMode = status.paired ? 'paired' : status.wechatlogin ? 'wechat' : 'none'
      this.status = status
    },
    setIsLogin(isLogin: boolean) {
      this.isLogin = isLogin
    },
    setIsAdmin(isAdmin: boolean) {
      this.isAdmin = isAdmin
    }
  },
  getters: {
    getStatus: (state) => state.status,
    getIsLogin: (state) => state.isLogin,
    getIsAdmin: (state) => state.isAdmin,
  },
  persist: true
})

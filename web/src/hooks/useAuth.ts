import { ref, onMounted, onUnmounted } from 'vue'
import { getToken, setToken, clearToken, isAuthenticated as checkAuth, getUser, clearUser } from '../lib/auth'
import { pair as apiPair, getPublicHealth } from '../lib/api'
import { useStore } from '../store'

export function useAuth() {
  const token = ref<string | null>(getToken())
  const authenticated = ref<boolean>(checkAuth())
  const loading = ref<boolean>(!checkAuth())
  const user = ref<any>(getUser())

  const updateAuthState = () => {
    const t = getToken()
    token.value = t
    authenticated.value = t !== null && t.length > 0
    user.value = getUser()
  }

  const pair = async (code: string): Promise<void> => {
    const { token: newToken } = await apiPair(code)
    setToken(newToken)
    updateAuthState()
    const store = useStore()
    store.setIsLogin(true)
  }

  const logout = (): void => {
    clearToken()
    clearUser()
    updateAuthState()
    const store = useStore()
    store.setIsLogin(false)
    store.setIsAdmin(false)
  }

  onMounted(() => {
    // 页面刷新后重新加载Token
    token.value = getToken()
    authenticated.value = checkAuth()
    user.value = getUser()

    if (checkAuth()) return

    let cancelled = false
    getPublicHealth()
      .then((health: { require_pairing: boolean; paired: boolean }) => {
        if (cancelled) return
        if (!health.require_pairing || health.paired) {
          authenticated.value = true
        }
      })
      .catch(() => {
        // health endpoint unreachable — fall back to showing pairing dialog
      })
      .finally(() => {
        if (!cancelled) loading.value = false
      })

    const handler = (e: StorageEvent) => {
      if (e.key === 'zeroclaw_token' || e.key === 'zeroclaw_user') {
        updateAuthState()
      }
    }

    window.addEventListener('storage', handler)

    onUnmounted(() => {
      cancelled = true
      window.removeEventListener('storage', handler)
    })
  })

  return {
    token,
    isAuthenticated: authenticated,
    loading,
    user,
    pair,
    logout
  }
}
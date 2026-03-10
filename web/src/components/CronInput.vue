<template>
  <div class="space-y-4">
    <div class="flex gap-2 mb-3">
      <button
        v-for="preset in presets"
        :key="preset.value"
        @click="applyPreset(preset.value)"
        :class="[
          'px-3 py-1.5 text-xs font-medium rounded-lg transition-colors',
          schedule === preset.value
            ? 'bg-blue-600 text-white'
            : 'bg-gray-800 text-gray-300 hover:bg-gray-700 border border-gray-700'
        ]"
      >
        {{ preset.label }}
      </button>
    </div>

    <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
      <div class="grid grid-cols-5 gap-3 mb-4">
        <div v-for="(field, index) in fields" :key="index">
          <label class="block text-xs font-medium text-gray-400 mb-1.5">
            {{ field.label }}
          </label>
          <select
            :value="getFieldValue(index)"
            @change="updateField(index, $event)"
            class="w-full bg-gray-900 border border-gray-700 rounded-lg px-2 py-1.5 text-xs text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="*">*</option>
            <option v-for="opt in field.options" :key="opt" :value="opt">
              {{ opt }}
            </option>
          </select>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <input
          :value="schedule"
          @input="handleInput($event)"
          type="text"
          placeholder="e.g. 0 0 * * *"
          class="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
        />
        <button
          @click="validateSchedule"
          :class="[
            'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
            isValid
              ? 'bg-green-600 hover:bg-green-700 text-white'
              : 'bg-gray-700 hover:bg-gray-600 text-gray-300'
          ]"
        >
          {{ isValid ? '✓' : '验证' }}
        </button>
      </div>

      <div v-if="errorMessage" class="mt-2 text-xs text-red-400">
        {{ errorMessage }}
      </div>

      <div v-if="nextRuns.length > 0" class="mt-3 pt-3 border-t border-gray-700">
        <div class="text-xs font-medium text-gray-400 mb-2">下次运行时间:</div>
        <div class="space-y-1">
          <div
            v-for="(run, idx) in nextRuns.slice(0, 5)"
            :key="idx"
            class="text-xs text-gray-300 font-mono"
          >
            {{ run }}
          </div>
        </div>
      </div>
    </div>

    <div class="text-xs text-gray-500">
      <div class="font-medium text-gray-400 mb-1">格式说明:</div>
      <div class="grid grid-cols-5 gap-2">
        <div><span class="text-blue-400">分</span> (0-59)</div>
        <div><span class="text-blue-400">时</span> (0-23)</div>
        <div><span class="text-blue-400">日</span> (1-31)</div>
        <div><span class="text-blue-400">月</span> (1-12)</div>
        <div><span class="text-blue-400">周</span> (0-6)</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

interface Props {
  modelValue: string
}

interface Emits {
  (e: 'update:modelValue', value: string): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const schedule = ref(props.modelValue)
const isValid = ref(false)
const errorMessage = ref('')
const nextRuns = ref<string[]>([])

const presets = [
  { label: '每分钟', value: '* * * * *' },
  { label: '每小时', value: '0 * * * *' },
  { label: '每天', value: '0 0 * * *' },
  { label: '每周', value: '0 0 * * 0' },
  { label: '每月', value: '0 0 1 * *' },
  { label: '工作日', value: '0 0 * * 1-5' },
]

const fields = [
  {
    label: '分',
    options: Array.from({ length: 60 }, (_, i) => i.toString()),
  },
  {
    label: '时',
    options: Array.from({ length: 24 }, (_, i) => i.toString()),
  },
  {
    label: '日',
    options: Array.from({ length: 31 }, (_, i) => (i + 1).toString()),
  },
  {
    label: '月',
    options: Array.from({ length: 12 }, (_, i) => (i + 1).toString()),
  },
  {
    label: '周',
    options: ['0', '1', '2', '3', '4', '5', '6'],
  },
]

watch(() => props.modelValue, (newValue) => {
  schedule.value = newValue
  validateSchedule()
})

watch(schedule, (newValue) => {
  emit('update:modelValue', newValue)
})

function getFieldValue(index: number): string {
  const parts = schedule.value.split(' ')
  if (parts.length !== 5) return '*'
  return parts[index] || '*'
}

function updateField(index: number, event: Event) {
  const target = event.target as HTMLSelectElement
  const parts = schedule.value.split(' ')
  
  if (parts.length !== 5) {
    parts.length = 5
    parts.fill('*')
  }
  
  parts[index] = target.value || '*'
  schedule.value = parts.join(' ')
  validateSchedule()
}

function handleInput(event: Event) {
  const target = event.target as HTMLInputElement
  schedule.value = target.value
  validateSchedule()
}

function applyPreset(value: string) {
  schedule.value = value
  validateSchedule()
}

function validateSchedule() {
  const parts = schedule.value.trim().split(/\s+/)
  
  if (parts.length !== 5) {
    isValid.value = false
    errorMessage.value = '格式错误: 需要 5 个字段 (分 时 日 月 周)'
    nextRuns.value = []
    return
  }

  const [minute = '', hour = '', day = '', month = '', weekday = ''] = parts

  if (!validateField(minute, 0, 59)) {
    isValid.value = false
    errorMessage.value = '分钟字段无效 (0-59)'
    nextRuns.value = []
    return
  }

  if (!validateField(hour, 0, 23)) {
    isValid.value = false
    errorMessage.value = '小时字段无效 (0-23)'
    nextRuns.value = []
    return
  }

  if (!validateField(day, 1, 31)) {
    isValid.value = false
    errorMessage.value = '日期字段无效 (1-31)'
    nextRuns.value = []
    return
  }

  if (!validateField(month, 1, 12)) {
    isValid.value = false
    errorMessage.value = '月份字段无效 (1-12)'
    nextRuns.value = []
    return
  }

  if (!validateField(weekday, 0, 6)) {
    isValid.value = false
    errorMessage.value = '星期字段无效 (0-6)'
    nextRuns.value = []
    return
  }

  isValid.value = true
  errorMessage.value = ''
  calculateNextRuns()
}

function validateField(value: string, min: number, max: number): boolean {
  if (value === '*') return true
  
  const parts = value.split(',')
  
  for (const part of parts) {
    if (part.includes('/')) {
      const [base = '', step = ''] = part.split('/')
      if (base !== '*' && !isValidNumber(base, min, max)) return false
      if (!isValidNumber(step, 1, max)) return false
    } else if (part.includes('-')) {
      const [start = '', end = ''] = part.split('-')
      if (!isValidNumber(start, min, max)) return false
      if (!isValidNumber(end, min, max)) return false
    } else {
      if (!isValidNumber(part, min, max)) return false
    }
  }
  
  return true
}

function isValidNumber(value: string, min: number, max: number): boolean {
  const num = parseInt(value, 10)
  return !isNaN(num) && num >= min && num <= max
}

function calculateNextRuns() {
  const runs: string[] = []
  const now = new Date()
  
  const parts = schedule.value.split(' ')
  const [minute = '', hour = '', day = '', month = '', weekday = ''] = parts
  
  let current = new Date(now)
  current.setSeconds(0)
  current.setMilliseconds(0)
  
  let attempts = 0
  const maxAttempts = 1000
  
  while (runs.length < 5 && attempts < maxAttempts) {
    attempts++
    current.setMinutes(current.getMinutes() + 1)
    
    if (matchesSchedule(current, minute, hour, day, month, weekday)) {
      runs.push(formatDate(current))
    }
  }
  
  nextRuns.value = runs
}

function matchesSchedule(date: Date, minute: string, hour: string, day: string, month: string, weekday: string): boolean {
  return (
    matchesField(date.getMinutes(), minute) &&
    matchesField(date.getHours(), hour) &&
    matchesField(date.getDate(), day) &&
    matchesField(date.getMonth() + 1, month) &&
    matchesField(date.getDay(), weekday)
  )
}

function matchesField(value: number, pattern: string): boolean {
  if (pattern === '*') return true
  
  const parts = pattern.split(',')
  
  for (const part of parts) {
    if (part.includes('/')) {
      const [base = '', step = ''] = part.split('/')
      const stepNum = parseInt(step, 10)
      
      if (base === '*') {
        if (value % stepNum === 0) return true
      } else {
        const baseNum = parseInt(base, 10)
        if ((value - baseNum) % stepNum === 0 && value >= baseNum) return true
      }
    } else if (part.includes('-')) {
      const [start = '', end = ''] = part.split('-')
      const startNum = parseInt(start, 10)
      const endNum = parseInt(end, 10)
      if (value >= startNum && value <= endNum) return true
    } else {
      const num = parseInt(part, 10)
      if (value === num) return true
    }
  }
  
  return false
}

function formatDate(date: Date): string {
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    weekday: 'short'
  })
}
</script>

<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <header class="overflow-hidden rounded-2xl border border-slate-200 bg-slate-950 text-white shadow-xl dark:border-dark-700">
        <div class="relative grid gap-8 px-6 py-8 lg:grid-cols-[1fr_300px] lg:px-10 lg:py-10">
          <div class="relative z-10">
            <p class="font-mono text-xs font-semibold tracking-[0.22em] text-blue-300">号池监控 / 安全跳转</p>
            <h1 class="mt-4 text-3xl font-semibold tracking-tight sm:text-4xl">号池监控</h1>
            <p class="mt-4 max-w-2xl text-sm leading-7 text-slate-300">
              使用当前 Sub2API 管理员身份签发一次性票据，进入独立号池管理台。票据通过 POST 提交，不会出现在地址栏或浏览器历史中。
            </p>
          </div>
          <div class="relative z-10 flex items-center justify-center">
            <div class="grid h-36 w-36 rotate-3 grid-cols-3 gap-2 rounded-3xl border border-white/15 bg-white/5 p-5 shadow-2xl backdrop-blur">
              <span class="bg-blue-400"></span><span class="translate-y-3 bg-orange-400"></span><span class="translate-y-6 bg-white"></span>
              <span class="bg-white/30"></span><span class="translate-y-3 bg-blue-300/60"></span><span class="translate-y-6 bg-orange-300/70"></span>
              <span class="bg-orange-400/50"></span><span class="translate-y-3 bg-white/40"></span><span class="translate-y-6 bg-blue-400/70"></span>
            </div>
          </div>
          <div class="pointer-events-none absolute -right-20 -top-32 h-80 w-80 rounded-full bg-blue-600/25 blur-3xl"></div>
        </div>
      </header>

      <div class="rounded-xl border border-amber-200 bg-amber-50 px-5 py-4 text-sm text-amber-950 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-100">
        <div class="flex gap-3">
          <span class="mt-0.5 font-mono text-lg">◷</span>
          <div><b>监控数据不是实时刷新</b><p class="mt-1 leading-6 opacity-80">Sub2API 与 Kiro 默认每 10 分钟采集一次；目标页面会显示各数据源最近一次成功采集的准确时间。</p></div>
        </div>
      </div>

      <div v-if="errorMessage" data-test="launch-error" class="rounded-xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
        {{ errorMessage }}
      </div>

      <section class="grid gap-5 lg:grid-cols-2">
        <article class="group rounded-2xl border border-gray-200 bg-white p-6 shadow-sm transition hover:-translate-y-0.5 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
          <div class="flex items-start justify-between gap-5">
            <div><span class="font-mono text-xs tracking-widest text-primary-600">01 / 集成入口</span><h2 class="mt-3 text-xl font-semibold text-gray-900 dark:text-white">集成管理页</h2></div>
            <span class="grid h-11 w-11 place-items-center rounded-xl bg-primary-50 text-xl text-primary-700 dark:bg-primary-950/40 dark:text-primary-300">↗</span>
          </div>
          <p class="mt-4 min-h-12 text-sm leading-6 text-gray-500 dark:text-gray-400">通过 Sub2API 域名下的 <code>/pool-admin</code> 路由进入，适合统一管理入口。</p>
          <button data-test="open-integrated" type="button" class="btn btn-primary mt-6 w-full" :disabled="launching !== null" @click="launch('integrated')">
            {{ launching === 'integrated' ? '正在建立管理会话…' : '进入集成管理页' }}
          </button>
        </article>

        <article class="group rounded-2xl border border-gray-200 bg-white p-6 shadow-sm transition hover:-translate-y-0.5 hover:shadow-lg dark:border-dark-700 dark:bg-dark-800">
          <div class="flex items-start justify-between gap-5">
            <div><span class="font-mono text-xs tracking-widest text-orange-600">02 / 独立入口</span><h2 class="mt-3 text-xl font-semibold text-gray-900 dark:text-white">独立管理页</h2></div>
            <span class="grid h-11 w-11 place-items-center rounded-xl bg-orange-50 text-xl text-orange-700 dark:bg-orange-950/40 dark:text-orange-300">◇</span>
          </div>
          <p class="mt-4 min-h-12 text-sm leading-6 text-gray-500 dark:text-gray-400">进入独立域名的管理地址。该域名使用自己的会话 Cookie，不与 Sub2API 域名共享。</p>
          <button data-test="open-standalone" type="button" class="btn btn-secondary mt-6 w-full" :disabled="launching !== null" @click="launch('standalone')">
            {{ launching === 'standalone' ? '正在建立管理会话…' : '进入独立管理页' }}
          </button>
        </article>
      </section>

      <section class="rounded-2xl border border-gray-200 bg-gray-50 p-6 dark:border-dark-700 dark:bg-dark-800/50">
        <h2 class="text-sm font-semibold text-gray-900 dark:text-white">安全边界</h2>
        <ul class="mt-3 grid gap-3 text-sm leading-6 text-gray-500 dark:text-gray-400 md:grid-cols-3">
          <li>一次性票据 60 秒失效，成功交换后不可重放。</li>
          <li>目标页面不保存 Sub2API 密码或管理员 Token。</li>
          <li>未建立管理会话时，目标管理地址统一返回 404。</li>
        </ul>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { createPoolMonitorTicket, type PoolMonitorTarget, type PoolMonitorTicketResponse } from '@/api/admin/poolMonitor'

const launching = ref<PoolMonitorTarget | null>(null)
const errorMessage = ref('')

function isAllowedTargetURL(value: string): boolean {
  try {
    const parsed = new URL(value)
    const loopback = parsed.hostname === 'localhost' || parsed.hostname === '127.0.0.1' || parsed.hostname === '::1'
    return parsed.protocol === 'https:' || (parsed.protocol === 'http:' && loopback)
  } catch {
    return false
  }
}

function postTicket(result: PoolMonitorTicketResponse): void {
  if (!isAllowedTargetURL(result.target_url)) throw new Error('invalid target URL')
  const form = document.createElement('form')
  form.method = 'post'
  form.action = result.target_url
  const input = document.createElement('input')
  input.type = 'hidden'
  input.name = 'ticket'
  input.value = result.ticket
  form.appendChild(input)
  document.body.appendChild(form)
  form.submit()
  form.remove()
}

async function launch(target: PoolMonitorTarget): Promise<void> {
  launching.value = target
  errorMessage.value = ''
  try {
    postTicket(await createPoolMonitorTicket(target))
  } catch {
    errorMessage.value = '暂时无法进入号池管理页，请确认服务已启用后重试。'
    launching.value = null
  }
}
</script>

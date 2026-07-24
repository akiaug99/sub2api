export const PLATFORM_OPTIONS = [
  { value: 'anthropic', label: 'Anthropic', color: 'orange', defaultBaseUrl: 'https://api.anthropic.com' },
  { value: 'openai', label: 'OpenAI', color: 'emerald', defaultBaseUrl: 'https://api.openai.com' },
  { value: 'gemini', label: 'Gemini', color: 'blue', defaultBaseUrl: 'https://generativelanguage.googleapis.com' },
  { value: 'antigravity', label: 'Antigravity', color: 'purple', defaultBaseUrl: '' },
  { value: 'grok', label: 'Grok', color: 'zinc', defaultBaseUrl: 'https://api.x.ai/v1' },
  { value: 'deepseek', label: 'DeepSeek', color: 'sky', defaultBaseUrl: 'https://api.deepseek.com' },
  { value: 'qwen', label: 'Qwen', color: 'cyan', defaultBaseUrl: 'https://dashscope.aliyuncs.com/compatible-mode' },
  { value: 'zhipu', label: 'Zhipu GLM', color: 'indigo', defaultBaseUrl: 'https://open.bigmodel.cn/api/paas' },
  { value: 'moonshot', label: 'Moonshot Kimi', color: 'slate', defaultBaseUrl: 'https://api.moonshot.cn' },
  { value: 'minimax', label: 'MiniMax', color: 'pink', defaultBaseUrl: 'https://api.minimax.chat' },
  { value: 'baidu', label: 'Baidu ERNIE', color: 'blue', defaultBaseUrl: 'https://qianfan.baidubce.com/v2' },
  { value: 'spark', label: 'iFlytek Spark', color: 'amber', defaultBaseUrl: 'https://spark-api-open.xf-yun.com' },
  { value: 'hunyuan', label: 'Tencent Hunyuan', color: 'teal', defaultBaseUrl: 'https://api.hunyuan.cloud.tencent.com' },
  { value: 'doubao', label: 'Doubao', color: 'rose', defaultBaseUrl: 'https://ark.cn-beijing.volces.com/api' },
  { value: 'yi', label: '01.AI Yi', color: 'lime', defaultBaseUrl: 'https://api.lingyiwanwu.com' },
  { value: 'baichuan', label: 'Baichuan', color: 'fuchsia', defaultBaseUrl: 'https://api.baichuan-ai.com' },
  { value: 'stepfun', label: 'StepFun', color: 'violet', defaultBaseUrl: 'https://api.stepfun.com' },
  { value: 'sensetime', label: 'SenseNova', color: 'red', defaultBaseUrl: 'https://api.sensenova.cn/compatible-mode' },
] as const

export type PlatformId = (typeof PLATFORM_OPTIONS)[number]['value']

export const GROUP_PLATFORM_OPTIONS = [
  ...PLATFORM_OPTIONS,
  { value: 'composite', label: 'Composite', color: 'cyan', defaultBaseUrl: '' },
] as const

export type GroupPlatformId = (typeof GROUP_PLATFORM_OPTIONS)[number]['value']

export const PLATFORM_VALUES = PLATFORM_OPTIONS.map((p) => p.value) as PlatformId[]

export const OPENAI_COMPATIBLE_PLATFORM_VALUES = [
  'openai',
  'grok',
  'deepseek',
  'qwen',
  'zhipu',
  'moonshot',
  'minimax',
  'baidu',
  'spark',
  'hunyuan',
  'doubao',
  'yi',
  'baichuan',
  'stepfun',
  'sensetime',
] as const

export function isOpenAICompatiblePlatform(platform: string): boolean {
  return OPENAI_COMPATIBLE_PLATFORM_VALUES.includes(platform as (typeof OPENAI_COMPATIBLE_PLATFORM_VALUES)[number])
}

export function getPlatformLabel(platform: string): string {
  return GROUP_PLATFORM_OPTIONS.find((p) => p.value === platform)?.label || platform
}

export function getPlatformDefaultBaseUrl(platform: string): string {
  return GROUP_PLATFORM_OPTIONS.find((p) => p.value === platform)?.defaultBaseUrl || ''
}

const colorClasses: Record<string, string> = {
  orange: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  emerald: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  blue: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  purple: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400',
  zinc: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300',
  sky: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400',
  cyan: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
  indigo: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  slate: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
  pink: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
  amber: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  teal: 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400',
  rose: 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
  lime: 'bg-lime-100 text-lime-700 dark:bg-lime-900/30 dark:text-lime-400',
  fuchsia: 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-400',
  violet: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400',
  red: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
}

export function getPlatformBadgeClass(platform: string): string {
  const color = GROUP_PLATFORM_OPTIONS.find((p) => p.value === platform)?.color || 'slate'
  return colorClasses[color] || colorClasses.slate
}

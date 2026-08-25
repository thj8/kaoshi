<template>
  <div class="page">
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 20px">
      <button class="btn btn-ghost" style="padding: 8px 14px" @click="$router.push('/admin')">←</button>
      <h1 style="flex: 1">{{ quiz?.title || '加载中...' }}</h1>
      <button v-if="quiz" class="tag link-tag" style="cursor: pointer" title="点击复制加入链接" @click="copyLink">
        🔗 {{ joinLink }}
      </button>
    </div>

    <template v-if="quiz">
      <!-- 题目列表 -->
      <div class="card" style="margin-bottom: 16px">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
          <h2>题目（{{ questions.length }}）</h2>
          <div style="display: flex; gap: 8px">
            <button class="btn btn-primary" style="padding: 8px 16px" @click="$router.push(`/admin/quiz/${quiz.id}/console`)">
              打开控制台
            </button>
            <button v-if="quiz.status === 'WAITING'" class="btn btn-ghost" style="padding: 8px 16px" @click="openEditor()">＋ 添加题目</button>
          </div>
        </div>
        <p v-if="quiz.status !== 'WAITING'" class="text-dim" style="font-size: 13px; margin-bottom: 10px">
          答题已开始，题目不可修改
        </p>
        <div v-if="questions.length === 0" class="text-dim" style="text-align: center; padding: 24px">
          还没有题目，点击「添加题目」
        </div>
        <div v-for="(q, i) in questions" :key="q.id" class="q-item">
          <div class="q-head">
            <span class="q-no">{{ String(i + 1).padStart(2, '0') }}</span>
            <span class="tag">{{ typeText(q.type) }}</span>
            <span class="tag">{{ q.score }}分</span>
            <span class="tag">{{ q.required ? '必答' : '可跳过' }}</span>
            <span v-if="q.time_limit" class="tag">{{ q.time_limit }}s</span>
            <span class="tag" style="color: var(--success)">答案 {{ q.answer }}</span>
            <span style="flex: 1"></span>
            <template v-if="quiz.status === 'WAITING'">
              <button class="btn btn-ghost" style="padding: 4px 12px; font-size: 13px" @click="openEditor(i)">编辑</button>
              <button class="btn btn-danger" style="padding: 4px 12px; font-size: 13px" @click="delQ(q)">删除</button>
            </template>
          </div>
          <div class="q-content">{{ q.content }}</div>
          <div class="q-opts">
            <span v-for="o in q.options" :key="o.label" class="q-opt" :class="{ right: q.answer.includes(o.label) }">
              <b>{{ o.label }}.</b> {{ o.content }}
            </span>
          </div>
        </div>
      </div>

      <!-- 活动配置（仅未开始可编辑） -->
      <div class="card">
        <h2 style="margin-bottom: 12px">答题配置</h2>
        <p v-if="quiz.status !== 'WAITING'" class="text-dim" style="font-size: 13px; margin-bottom: 10px">
          答题已开始，配置不可修改
        </p>
        <form @submit.prevent="saveConfig" :disabled="quiz.status !== 'WAITING'">
          <div class="fgrid">
            <div class="frow">
              <label>答题名称</label>
              <input v-model="cfgForm.title" class="input" :disabled="quiz.status !== 'WAITING'" />
            </div>
            <div class="frow">
              <label>答题模式</label>
              <select v-model="cfgForm.mode" class="input" :disabled="quiz.status !== 'WAITING'">
                <option value="normal">普通模式</option>
                <option value="rush">抢答模式</option>
              </select>
            </div>
            <div class="frow">
              <label>每题答题时间（秒）</label>
              <input v-model.number="cfgForm.per_question_time" class="input" type="number" min="5" max="600" :disabled="quiz.status !== 'WAITING'" />
            </div>
            <div class="frow">
              <label>总答题时间（秒，0=不限）</label>
              <input v-model.number="cfgForm.total_time" class="input" type="number" min="0" :disabled="quiz.status !== 'WAITING'" />
            </div>
            <div class="frow">
              <label>每题抢答名额</label>
              <input v-model.number="cfgForm.rush_winner_count" class="input" type="number" min="1" max="10" :disabled="quiz.status !== 'WAITING'" />
            </div>
            <div class="frow">
              <label>抢答窗口（秒）</label>
              <input v-model.number="cfgForm.rush_time" class="input" type="number" min="3" max="120" :disabled="quiz.status !== 'WAITING'" />
            </div>
            <div class="frow">
              <label>抢答后答题时间（秒）</label>
              <input v-model.number="cfgForm.rush_answer_time" class="input" type="number" min="5" max="600" :disabled="quiz.status !== 'WAITING'" />
            </div>
            <div class="frow">
              <label>抢答奖励分</label>
              <input v-model.number="cfgForm.rush_bonus_score" class="input" type="number" min="0" :disabled="quiz.status !== 'WAITING'" />
            </div>
            <div class="frow">
              <label>抢答答错扣分</label>
              <input v-model.number="cfgForm.rush_wrong_score" class="input" type="number" min="0" :disabled="quiz.status !== 'WAITING'" />
            </div>
            <div class="frow">
              <label>答题说明</label>
              <input v-model="cfgForm.description" class="input" :disabled="quiz.status !== 'WAITING'" />
            </div>
          </div>
          <div class="frow" style="flex-direction: row; gap: 20px; flex-wrap: wrap">
            <label style="margin: 0"><input v-model="cfgForm.show_answer" type="checkbox" :disabled="quiz.status !== 'WAITING'" /> 公布正确答案</label>
            <label style="margin: 0"><input v-model="cfgForm.show_analysis" type="checkbox" :disabled="quiz.status !== 'WAITING'" /> 公布解析</label>
            <label style="margin: 0"><input v-model="cfgForm.show_ranking" type="checkbox" :disabled="quiz.status !== 'WAITING'" /> 显示排行榜</label>
            <label style="margin: 0"><input v-model="cfgForm.rush_enabled" type="checkbox" :disabled="quiz.status !== 'WAITING'" /> 开启抢答</label>
          </div>
          <button v-if="quiz.status === 'WAITING'" class="btn btn-primary" style="margin-top: 8px" :disabled="saving">
            {{ saving ? '保存中...' : '保存配置' }}
          </button>
        </form>
      </div>
    </template>

    <!-- 题目编辑弹窗 -->
    <div v-if="editing" class="modal" @click.self="closeEditor">
      <div class="card modal-body" style="max-height: 88vh; overflow: auto">
        <h2 style="margin-bottom: 16px">{{ editIndex >= 0 ? '编辑题目' : '添加题目' }}</h2>
        <form @submit.prevent="saveQ">
          <div class="frow">
            <label>题型 *</label>
            <select v-model="ed.type" class="input" @change="onTypeChange">
              <option value="single">单选题</option>
              <option value="multiple">多选题</option>
              <option value="judge">判断题</option>
            </select>
          </div>
          <div class="frow">
            <label>题目内容 *</label>
            <textarea v-model="ed.content" class="input" rows="2" required placeholder="HTTP 默认端口是多少？"></textarea>
          </div>
          <template v-if="ed.type !== 'judge'">
            <div v-for="(o, i) in edOptions" :key="i" class="opt-row">
              <span class="opt-label">{{ String.fromCharCode(65 + i) }}</span>
              <input v-model="o.content" class="input" :placeholder="`选项 ${String.fromCharCode(65 + i)}`" required />
              <button v-if="edOptions.length > 2" type="button" class="btn btn-ghost" style="padding: 8px 12px" @click="edOptions.splice(i, 1); ed.answer = ''">✕</button>
            </div>
            <button v-if="edOptions.length < 8" type="button" class="btn btn-ghost" style="padding: 8px 16px; font-size: 13px" @click="edOptions.push({ label: '', content: '' })">
              ＋ 添加选项
            </button>
          </template>
          <div class="frow" style="margin-top: 14px">
            <label>正确答案 *</label>
            <div class="ans-row">
              <template v-if="ed.type === 'judge'">
                <button type="button" class="ans-btn" :class="{ sel: ed.answer === 'A' }" @click="ed.answer = 'A'">正确</button>
                <button type="button" class="ans-btn" :class="{ sel: ed.answer === 'B' }" @click="ed.answer = 'B'">错误</button>
              </template>
              <template v-else>
                <button
                  v-for="(_, i) in edOptions"
                  :key="i"
                  type="button"
                  class="ans-btn"
                  :class="{ sel: (ed.answer || '').includes(String.fromCharCode(65 + i)) }"
                  @click="toggleAnswer(String.fromCharCode(65 + i))"
                >
                  {{ String.fromCharCode(65 + i) }}
                </button>
              </template>
            </div>
          </div>
          <div class="fgrid">
            <div class="frow"><label>分值</label><input v-model.number="ed.score" class="input" type="number" min="1" max="100" /></div>
            <div class="frow"><label>本题限时（秒，0=用全局）</label><input v-model.number="ed.time_limit" class="input" type="number" min="0" max="600" /></div>
          </div>
          <div class="frow" style="flex-direction: row">
            <label style="margin: 0"><input v-model="ed.required" type="checkbox" /> 必答题（不可跳过）</label>
          </div>
          <div class="frow">
            <label>解析（公布答案时展示）</label>
            <textarea v-model="ed.analysis" class="input" rows="2" placeholder="HTTP 默认使用 TCP 80 端口"></textarea>
          </div>
          <p v-if="edErr" style="color: var(--danger); font-size: 14px">{{ edErr }}</p>
          <div style="display: flex; gap: 10px; margin-top: 14px">
            <button type="button" class="btn btn-ghost" style="flex: 1" @click="closeEditor">取消</button>
            <button type="submit" class="btn btn-primary" style="flex: 1">保存</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { adminApi, type Quiz, type Question, type Option } from '../api/admin'

const route = useRoute()
const quizId = Number(route.params.id)
const quiz = ref<Quiz | null>(null)
const questions = ref<Question[]>([])
const saving = ref(false)

const cfgForm = reactive<Partial<Quiz>>({})

// 题目编辑器
const editing = ref(false)
const editIndex = ref(-1)
const edErr = ref('')
const ed = reactive<Partial<Question>>({
  type: 'single',
  content: '',
  options: [],
  answer: '',
  analysis: '',
  score: 10,
  required: true,
  time_limit: 0,
})
const edOptions = computed<Option[]>({
  get: () => ed.options || [],
  set: (v) => {
    ed.options = v
  },
})

onMounted(load)

async function load() {
  const { quiz: q } = await adminApi.getQuiz(quizId)
  quiz.value = q
  Object.assign(cfgForm, q)
  questions.value = await adminApi.listQuestions(quizId)
}

async function saveConfig() {
  if (!quiz.value) return
  saving.value = true
  try {
    quiz.value = await adminApi.updateQuiz(quizId, cfgForm)
  } finally {
    saving.value = false
  }
}

function openEditor(index = -1) {
  editIndex.value = index
  edErr.value = ''
  if (index >= 0) {
    const q = questions.value[index]
    Object.assign(ed, {
      id: q.id,
      type: q.type,
      content: q.content,
      options: q.options.map((o) => ({ ...o })),
      answer: q.answer,
      analysis: q.analysis || '',
      score: q.score,
      required: q.required,
      time_limit: q.time_limit,
    })
  } else {
    Object.assign(ed, {
      id: undefined,
      type: 'single',
      content: '',
      options: [
        { label: 'A', content: '' },
        { label: 'B', content: '' },
        { label: 'C', content: '' },
        { label: 'D', content: '' },
      ],
      answer: '',
      analysis: '',
      score: 10,
      required: true,
      time_limit: 0,
    })
  }
  editing.value = true
}

function closeEditor() {
  editing.value = false
}

function onTypeChange() {
  if (ed.type === 'judge') {
    ed.options = [
      { label: 'A', content: '正确' },
      { label: 'B', content: '错误' },
    ]
    ed.answer = ''
  } else {
    ed.answer = ''
  }
}

function toggleAnswer(label: string) {
  const cur = ed.answer || ''
  if (ed.type === 'single') {
    ed.answer = label
  } else {
    ed.answer = cur.includes(label) ? cur.replace(label, '') : cur + label
  }
}

async function saveQ() {
  if (!ed.content) {
    edErr.value = '请输入题目内容'
    return
  }
  if (ed.type === 'judge') {
    ed.options = [
      { label: 'A', content: '正确' },
      { label: 'B', content: '错误' },
    ]
  }
  const opts: Option[] = (ed.options || []).map((o, i) => ({
    label: String.fromCharCode(65 + i),
    content: o.content,
  }))
  if (ed.type !== 'judge' && opts.some((o) => !o.content)) {
    edErr.value = '选项内容不能为空'
    return
  }
  if (!ed.answer) {
    edErr.value = '请设置正确答案'
    return
  }
  edErr.value = ''
  const payload = { ...ed, options: opts, answer: ed.answer }
  if (editIndex.value >= 0 && ed.id) {
    await adminApi.updateQuestion(ed.id, payload)
  } else {
    await adminApi.createQuestion(quizId, payload)
  }
  editing.value = false
  questions.value = await adminApi.listQuestions(quizId)
}

async function delQ(q: Question) {
  if (!confirm('确定删除该题目？')) return
  await adminApi.deleteQuestion(q.id)
  questions.value = await adminApi.listQuestions(quizId)
}

function typeText(t: string) {
  return { single: '单选', multiple: '多选', judge: '判断' }[t] || t
}

const joinLink = computed(() => (quiz.value ? `${location.origin}/join/${quiz.value.id}` : ''))

async function copyLink() {
  try {
    await navigator.clipboard.writeText(joinLink.value)
    alert('已复制加入链接：\n' + joinLink.value)
  } catch {
    alert('加入链接：' + joinLink.value)
  }
}
</script>

<style scoped>
.q-item {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 10px;
  background: var(--bg-soft);
}
.q-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.q-no {
  font-weight: 700;
  color: var(--primary);
}
.q-content {
  font-size: 15px;
  margin-bottom: 8px;
}
.q-opts {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  font-size: 13px;
  color: var(--text-dim);
}
.q-opt.right {
  color: var(--success);
}
.frow {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 14px;
}
.frow label {
  font-size: 13px;
  color: var(--text-dim);
}
.fgrid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}
.modal {
  position: fixed;
  inset: 0;
  background: rgba(5, 8, 18, 0.72);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  z-index: 50;
}
.modal-body {
  width: 620px;
  max-width: 100%;
}
.opt-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.link-tag {
  border: 1px dashed var(--border);
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.opt-label {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--card-2);
  border-radius: 6px;
  font-size: 13px;
  font-weight: 700;
  flex-shrink: 0;
}
.ans-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.ans-btn {
  min-width: 44px;
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg-soft);
  color: var(--text-dim);
  cursor: pointer;
  font-weight: 700;
}
.ans-btn.sel {
  background: var(--primary);
  color: #fff;
  border-color: var(--primary);
}
</style>

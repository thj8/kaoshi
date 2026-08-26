<template>
  <div class="page">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px">
      <h1>📋 答题活动管理</h1>
      <button class="btn btn-primary" @click="showCreate = true">＋ 创建答题</button>
    </div>

    <div v-if="loading" class="text-dim">加载中...</div>
    <div v-else-if="quizzes.length === 0" class="card text-dim" style="text-align: center; padding: 48px">
      暂无答题活动，点击「创建答题」开始
    </div>

    <div v-else class="quiz-grid">
      <div v-for="q in quizzes" :key="q.id" class="card quiz-card">
        <div style="display: flex; justify-content: space-between; align-items: start; gap: 8px">
          <h3 style="font-size: 17px">{{ q.title }}</h3>
          <span class="tag" :class="'st-' + q.status">{{ statusText(q.status) }}</span>
        </div>
        <p class="text-dim" style="margin: 8px 0; font-size: 13px; min-height: 18px">{{ q.description || '—' }}</p>
        <div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 14px">
          <span class="tag">{{ q.mode === 'rush' ? '抢答模式' : '普通模式' }}</span>
          <button class="tag link-tag" style="cursor: pointer" title="点击复制加入链接" @click="copyLink(q.id)">
            🔗 加入链接 <b style="color: var(--primary)">{{ joinLink(q.id) }}</b>
          </button>
        </div>
        <div style="display: flex; gap: 8px">
          <button class="btn btn-primary" style="flex: 1; padding: 9px" @click="$router.push(`/admin/quiz/${q.id}/console`)">
            控制台
          </button>
          <button class="btn btn-ghost" style="flex: 1; padding: 9px" @click="$router.push(`/admin/quiz/${q.id}/stats`)">
            统计
          </button>
          <button
            v-if="q.status === 'WAITING'"
            class="btn btn-ghost"
            style="flex: 1; padding: 9px"
            @click="$router.push(`/admin/quiz/${q.id}`)"
          >
            编辑
          </button>
          <button
            v-if="q.status === 'WAITING'"
            class="btn btn-danger"
            style="padding: 9px 14px"
            @click="del(q)"
          >
            删除
          </button>
        </div>
      </div>
    </div>

    <!-- 创建弹窗 -->
    <div v-if="showCreate" class="modal" @click.self="showCreate = false">
      <div class="card modal-body" style="max-height: 86vh; overflow: auto">
        <h2 style="margin-bottom: 16px">创建答题</h2>
        <form @submit.prevent="create">
          <div class="frow">
            <label>答题名称 *</label>
            <input v-model="form.title" class="input" placeholder="如：网络安全知识竞赛" maxlength="128" required />
          </div>
          <div class="frow">
            <label>答题说明</label>
            <textarea v-model="form.description" class="input" rows="2" placeholder="面向全员，请诚信作答" />
          </div>
          <div class="frow">
            <label>答题模式 *</label>
            <select v-model="form.mode" class="input">
              <option value="normal">普通模式（全员作答）</option>
              <option value="rush">抢答模式（先抢先答）</option>
            </select>
          </div>
          <div class="frow">
            <label>每题答题时间（秒）</label>
            <input v-model.number="form.per_question_time" class="input" type="number" min="5" max="600" />
          </div>
          <div class="frow">
            <label>总答题时间（秒，0=不限）</label>
            <input v-model.number="form.total_time" class="input" type="number" min="0" />
          </div>
          <div class="frow" style="flex-direction: row; gap: 20px">
            <label style="margin: 0"><input v-model="form.show_answer" type="checkbox" /> 公布正确答案</label>
            <label style="margin: 0"><input v-model="form.show_analysis" type="checkbox" /> 公布解析</label>
            <label style="margin: 0"><input v-model="form.show_ranking" type="checkbox" /> 显示排行榜</label>
          </div>
          <template v-if="form.mode === 'rush' || form.rush_enabled">
            <div class="frow" style="flex-direction: row; gap: 20px; border-top: 1px dashed var(--border); padding-top: 14px; margin-top: 4px">
              <label style="margin: 0"><input v-model="form.rush_enabled" type="checkbox" /> 开启抢答</label>
            </div>
            <div class="fgrid">
              <div class="frow"><label>每题抢答名额</label><input v-model.number="form.rush_winner_count" class="input" type="number" min="1" max="10" /></div>
              <div class="frow"><label>抢答窗口（秒）</label><input v-model.number="form.rush_time" class="input" type="number" min="3" max="120" /></div>
              <div class="frow"><label>抢答后答题时间（秒）</label><input v-model.number="form.rush_answer_time" class="input" type="number" min="5" max="600" /></div>
        <div class="frow"><label>必答计分（0=用题目分值）</label>
          <span style="display:flex;gap:6px">
            <input v-model.number="form.req_score_single" class="input" type="number" min="0" placeholder="单选" style="width:70px" />
            <input v-model.number="form.req_score_multiple" class="input" type="number" min="0" placeholder="多选" style="width:70px" />
            <input v-model.number="form.req_score_judge" class="input" type="number" min="0" placeholder="判断" style="width:70px" />
          </span>
        </div>
        <div class="frow"><label>抢答计分（0=用题目分值）</label>
          <span style="display:flex;gap:6px">
            <input v-model.number="form.rush_score_single" class="input" type="number" min="0" placeholder="单选" style="width:70px" />
            <input v-model.number="form.rush_score_multiple" class="input" type="number" min="0" placeholder="多选" style="width:70px" />
            <input v-model.number="form.rush_score_judge" class="input" type="number" min="0" placeholder="判断" style="width:70px" />
          </span>
        </div>
            </div>
          </template>
          <p v-if="err" style="color: var(--danger); font-size: 14px">{{ err }}</p>
          <div style="display: flex; gap: 10px; margin-top: 18px">
            <button type="button" class="btn btn-ghost" style="flex: 1" @click="showCreate = false">取消</button>
            <button type="submit" class="btn btn-primary" style="flex: 1" :disabled="creating">创建</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi, type Quiz } from '../api/admin'

const router = useRouter()
const quizzes = ref<Quiz[]>([])
const loading = ref(true)
const showCreate = ref(false)
const creating = ref(false)
const err = ref('')

const form = reactive<Partial<Quiz>>({
  title: '',
  description: '',
  mode: 'normal',
  per_question_time: 30,
  total_time: 0,
  show_answer: true,
  show_analysis: true,
  show_ranking: true,
  rush_enabled: false,
  rush_winner_count: 1,
  rush_time: 10,
  rush_answer_time: 20,
  rush_bonus_score: 5,
  rush_wrong_score: 0,
  req_score_single: 0, req_score_multiple: 0, req_score_judge: 0,
  rush_score_single: 0, rush_score_multiple: 0, rush_score_judge: 0,
  rush_deduct_single: 0, rush_deduct_multiple: 0, rush_deduct_judge: 0,
})

onMounted(load)

async function load() {
  loading.value = true
  try {
    quizzes.value = await adminApi.listQuizzes()
  } catch (e: any) {
    if (e?.response?.status === 401) router.push('/admin/login')
  } finally {
    loading.value = false
  }
}

async function create() {
  if (!form.title) {
    err.value = '请输入答题名称'
    return
  }
  creating.value = true
  err.value = ''
  try {
    const quiz = await adminApi.createQuiz(form)
    showCreate.value = false
    router.push(`/admin/quiz/${quiz.id}`)
  } catch (e: any) {
    err.value = e?.response?.data?.msg || '创建失败'
  } finally {
    creating.value = false
  }
}

async function del(q: Quiz) {
  if (!confirm(`确定删除「${q.title}」？题目将一并删除`)) return
  await adminApi.deleteQuiz(q.id)
  load()
}

function joinLink(id: number) {
  return `${location.origin}/join/${id}`
}

async function copyLink(id: number) {
  try {
    await navigator.clipboard.writeText(joinLink(id))
    alert('已复制加入链接：\n' + joinLink(id))
  } catch {
    alert('加入链接：' + joinLink(id))
  }
}

function statusText(s: string) {
  return (
    {
      WAITING: '未开始',
      RUNNING: '进行中',
      PAUSED: '已暂停',
      RUSHING: '抢答中',
      ANSWERING: '答题中',
      REVEALING: '公布答案',
      FINISHED: '已结束',
    } as Record<string, string>
  )[s] || s
}
</script>

<style scoped>
.quiz-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
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
  width: 560px;
  max-width: 100%;
}
.st-WAITING { color: var(--text-dim); }
.st-RUNNING { color: var(--success); }
.st-FINISHED { color: var(--warn); }
.link-tag {
  border: 1px dashed var(--border);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>

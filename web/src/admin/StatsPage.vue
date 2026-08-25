<template>
  <div>
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px">
      <h2 style="margin: 0">答题统计</h2>
      <span class="tag" :class="st?.status?.toLowerCase()">{{ statusText }}</span>
      <span style="flex: 1"></span>
      <button class="btn btn-ghost" @click="back">← 返回</button>
    </div>

    <p v-if="loading && !st" class="text-dim">加载中…</p>
    <template v-else-if="st">
      <!-- 总览 -->
      <div class="stats-grid">
        <div class="stat-card"><div class="num">{{ st.participants }}</div><div class="lbl">参与人数</div></div>
        <div class="stat-card"><div class="num">{{ st.finished }}</div><div class="lbl">完成人数</div></div>
        <div class="stat-card"><div class="num">{{ st.avg_score.toFixed(1) }}</div><div class="lbl">平均分</div></div>
        <div class="stat-card"><div class="num">{{ st.max_score }}</div><div class="lbl">最高分</div></div>
        <div class="stat-card"><div class="num">{{ st.min_score }}</div><div class="lbl">最低分</div></div>
        <div class="stat-card"><div class="num">{{ st.avg_correct_rate.toFixed(1) }}%</div><div class="lbl">平均正确率</div></div>
      </div>

      <!-- 题目维度 -->
      <div class="card" style="margin-top: 16px">
        <h3 style="margin: 0 0 12px">题目正确率</h3>
        <table v-if="st.questions.length" class="table">
          <thead>
            <tr><th>#</th><th style="text-align: left">题目</th><th>类型</th><th>答题人数</th><th>正确</th><th>正确率</th><th>平均用时</th></tr>
          </thead>
          <tbody>
            <tr v-for="q in st.questions" :key="q.question_id">
              <td>{{ q.index }}</td>
              <td style="text-align: left; max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ q.content }}</td>
              <td>{{ typeText(q.type) }}</td>
              <td>{{ q.answered }}</td>
              <td>{{ q.correct }}</td>
              <td>
                <div class="rate-bar"><div class="rate-fill" :style="{ width: q.correct_rate + '%' }" :class="{ low: q.correct_rate < 40 }"></div></div>
                <span style="font-size: 12px">{{ q.correct_rate.toFixed(0) }}%</span>
              </td>
              <td>{{ (q.avg_duration / 1000).toFixed(1) }}s</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="text-dim" style="margin: 0">暂无题目</p>
      </div>

      <!-- 排行榜 -->
      <div class="card" style="margin-top: 16px">
        <h3 style="margin: 0 0 12px">排行榜</h3>
        <table v-if="st.ranking.length" class="table">
          <thead><tr><th>排名</th><th style="text-align: left">昵称</th><th>分数</th><th>答对</th><th>答错</th></tr></thead>
          <tbody>
            <tr v-for="r in st.ranking" :key="r.user_id">
              <td><b :class="'top' + Math.min(r.rank, 3)">{{ r.rank }}</b></td>
              <td style="text-align: left">{{ r.nickname }}</td>
              <td><b>{{ r.score }}</b></td>
              <td>{{ r.correct }}</td>
              <td>{{ r.wrong }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="text-dim" style="margin: 0">暂无参与者</p>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminApi, type Statistics } from '../api/admin'

const route = useRoute()
const router = useRouter()
const quizId = Number(route.params.id)
const st = ref<Statistics | null>(null)
const loading = ref(false)
let timer: number | undefined

const statusText = computed(() => ({ WAITING: '未开始', RUNNING: '进行中', PAUSED: '已暂停', RUSHING: '抢答中', ANSWERING: '作答中', REVEALING: '公布中', FINISHED: '已结束' }[st.value?.status ?? ''] || st.value?.status || ''))
const typeText = (t: string) => ({ single: '单选', multiple: '多选', judge: '判断' }[t] || t)

async function load() {
  loading.value = true
  try {
    st.value = await adminApi.statistics(quizId)
    if (st.value.status === 'FINISHED') clearInterval(timer) // 已定局，停轮询
  } finally {
    loading.value = false
  }
}
function back() {
  router.push(st.value?.status === 'FINISHED' ? '/admin' : `/admin/quiz/${quizId}/console`)
}

onMounted(() => {
  load()
  // ponytail: 5s 轮询够用；需秒级实时再接 WS statistics:update
  timer = window.setInterval(load, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 12px; }
.stat-card { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 16px; text-align: center; }
.stat-card .num { font-size: 26px; font-weight: 700; }
.stat-card .lbl { font-size: 12px; opacity: 0.6; margin-top: 4px; }
.table { width: 100%; border-collapse: collapse; font-size: 14px; }
.table th, .table td { padding: 8px 10px; text-align: center; border-bottom: 1px solid var(--border); }
.table th { font-size: 12px; opacity: 0.6; font-weight: normal; }
.rate-bar { display: inline-block; vertical-align: middle; width: 80px; height: 8px; border-radius: 4px; background: var(--border); margin-right: 6px; overflow: hidden; }
.rate-fill { height: 100%; background: var(--success); border-radius: 4px; }
.rate-fill.low { background: #ff3b30; }
b.top1 { color: #b8860b; } b.top2 { color: #6e6e73; } b.top3 { color: #a05a2c; }
</style>

<template>
  <div class="qix" role="list" aria-label="题目索引">
    <span
      v-for="n in total"
      :key="n"
      role="listitem"
      class="qix-box"
      :class="boxClass(n)"
      :title="title(n)"
    >
      <span class="qix-num">{{ n }}</span>
      <span v-if="mark(n)" class="qix-mark" :class="markClass(n)">{{ mark(n) }}</span>
    </span>
  </div>
</template>

<script setup lang="ts">
/**
 * 题目索引条：每个题目一个小方框，实时展示作答进度。
 * 状态由父组件根据 WS 事件维护传入：
 *   correct  已答对（绿）
 *   wrong    已答错（红）
 *   answered 已提交（结果未知，蓝描边）
 *   missed   未答/未抢到/超时（灰）
 *   skipped  已跳过（灰虚线）
 *   其余未到的题目为默认态；当前题高亮（current 属性，1-based，0=无当前题）
 */
const props = withDefaults(
  defineProps<{
    total: number
    /** 当前题序号（1-based），0 表示没有进行中的题 */
    current?: number
    states?: Record<number, string>
  }>(),
  { total: 0, current: 0, states: () => ({}) }
)

const ORDER = ['correct', 'wrong', 'answered', 'skipped', 'missed']

function boxClass(n: number) {
  const st = props.states[n]
  const cls: Record<string, boolean> = {}
  if (st && ORDER.includes(st)) cls[st] = true
  if (n === props.current) cls.cur = true
  return cls
}

function mark(n: number): string {
  const st = props.states[n]
  if (st === 'correct') return '✓'
  if (st === 'wrong') return '✕'
  if (st === 'skipped') return '–'
  if (st === 'missed') return '·'
  return ''
}

function markClass(n: number) {
  return { m: mark(n) !== '' }
}

function title(n: number) {
  const st = props.states[n]
  if (n === props.current) return `第 ${n} 题 · 进行中`
  const map: Record<string, string> = {
    correct: `第 ${n} 题 · 答对`,
    wrong: `第 ${n} 题 · 答错`,
    answered: `第 ${n} 题 · 已提交`,
    missed: `第 ${n} 题 · 未作答`,
    skipped: `第 ${n} 题 · 已跳过`,
  }
  return (st && map[st]) || `第 ${n} 题`
}
</script>

<style scoped>
.qix {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.qix-box {
  position: relative;
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--text-dim);
  background: var(--card-2);
  border: 1.5px solid transparent;
  transition:
    background 0.18s ease,
    color 0.18s ease,
    border-color 0.18s ease,
    transform 0.18s ease,
    box-shadow 0.18s ease;
  user-select: none;
}

/* 默认（未开始） */
.qix-num {
  line-height: 1;
}

/* 当前题：主色填充 + 光圈 + 微放大 */
.qix-box.cur {
  background: var(--primary);
  color: #fff;
  border-color: var(--primary);
  transform: scale(1.1);
  box-shadow: 0 0 0 3px rgba(0, 113, 227, 0.22), 0 4px 12px rgba(0, 113, 227, 0.35);
}

/* 答对 */
.qix-box.correct {
  background: rgba(52, 199, 89, 0.14);
  color: var(--success);
  border-color: rgba(52, 199, 89, 0.55);
}
/* 答错 */
.qix-box.wrong {
  background: rgba(255, 59, 48, 0.1);
  color: var(--danger);
  border-color: rgba(255, 59, 48, 0.5);
}
/* 已提交（结果未知） */
.qix-box.answered {
  background: rgba(0, 113, 227, 0.08);
  color: var(--primary);
  border-color: rgba(0, 113, 227, 0.45);
}
/* 未答（超时/未抢到/漏答） */
.qix-box.missed {
  background: transparent;
  color: var(--text-dim);
  border-color: var(--border);
}
/* 主动跳过 */
.qix-box.skipped {
  background: transparent;
  color: var(--text-dim);
  border-style: dashed;
  border-color: var(--border);
}

/* 角标（✓ ✕ – ·） */
.qix-mark {
  position: absolute;
  right: -4px;
  top: -5px;
  width: 15px;
  height: 15px;
  border-radius: 50%;
  font-size: 10px;
  font-weight: 800;
  line-height: 15px;
  text-align: center;
  color: #fff;
  background: var(--text-dim);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.18);
}
.qix-box.correct .qix-mark {
  background: #34c759;
}
.qix-box.wrong .qix-mark {
  background: #ff3b30;
}
.qix-box.missed .qix-mark {
  background: #c7c7cc;
}
.qix-box.skipped .qix-mark {
  background: #c7c7cc;
}

@media (max-width: 640px) {
  .qix {
    gap: 6px;
  }
  .qix-box {
    width: 32px;
    height: 32px;
    font-size: 13px;
    border-radius: 9px;
  }
}
</style>

<template>
  <div class="admin-shell">
    <aside class="sidebar">
      <div class="brand" @click="$router.push('/admin')">
        <span class="brand-name">答题管理后台</span>
      </div>
      <nav class="nav">
        <router-link to="/admin" class="nav-item" exact-active-class="active">
<span>答题管理</span>
        </router-link>
        <router-link to="/admin/users" class="nav-item" active-class="active">
<span>用户管理</span>
        </router-link>
      </nav>
      <div class="sidebar-foot">
        <button class="btn btn-ghost" style="width: 100%; padding: 8px" @click="logout">退出登录</button>
      </div>
    </aside>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { LS } from '../api'

const router = useRouter()

function logout() {
  localStorage.removeItem(LS.adminToken)
  router.push('/admin/login')
}
</script>

<style scoped>
.admin-shell {
  display: flex;
  min-height: 100vh;
}
.sidebar {
  width: 210px;
  flex-shrink: 0;
  background: var(--card);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  position: sticky;
  top: 0;
  height: 100vh;
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 16px;
  font-weight: 800;
  font-size: 15px;
  cursor: pointer;
  border-bottom: 1px solid var(--border);
}
.nav {
  flex: 1;
  padding: 12px 10px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 14px;
  border-radius: 10px;
  color: var(--text-dim);
  text-decoration: none;
  font-size: 14px;
  transition: all 0.15s ease;
}
.nav-item:hover {
  color: var(--text);
  background: var(--card-2);
}
.nav-item.active {
  color: #fff;
  background: var(--primary);
  font-weight: 600;
}
.sidebar-foot {
  padding: 12px;
  border-top: 1px solid var(--border);
}
.content {
  flex: 1;
  min-width: 0;
}

@media (max-width: 720px) {
  .admin-shell {
    flex-direction: column;
  }
  .sidebar {
    width: 100%;
    height: auto;
    position: static;
    flex-direction: row;
    align-items: center;
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
  .brand {
    border-bottom: none;
    padding: 12px 14px;
  }
  .brand-name {
    display: none;
  }
  .nav {
    flex-direction: row;
    padding: 8px 10px;
  }
  .nav-item span:last-child {
    display: none;
  }
  .sidebar-foot {
    border-top: none;
    padding: 8px;
  }
  .sidebar-foot .btn {
    width: auto !important;
    padding: 6px 12px !important;
    font-size: 13px;
  }
}
</style>

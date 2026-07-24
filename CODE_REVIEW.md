# RockH5 前端代码审查报告

**审查日期**：2026-07-16  
**审查范围**：全部 src/ 目录，重点审查 api.ts、auth store、核心页面组件  
**技术栈**：Next.js 15 + React 19 + TypeScript + Tailwind CSS + Zustand + Axios + shadcn/ui  
**整体评分：7/10** — 架构清晰、UI 精致，但存在若干安全和可维护性问题。

---

## 一、🔴 严重问题

### H5-01：Token 存储在 localStorage — XSS 可窃取认证凭据

**文件**：`src/lib/api.ts` 第 14-15 行  
**严重程度**：🔴 严重

```ts
const TOKEN_KEY = "rockga…oken";
const REFRESH_TOKEN_KEY = "rockga…oken";
```

Token 存储在 `localStorage`，任何注入的 XSS（包括第三方依赖、广告脚本、浏览器插件）都能直接读取 token 并发送到攻击者服务器。

**修复建议**：
- Access Token 存 `httpOnly` cookie（后端 Set-Cookie），前端 JS 不可读
- 或使用 BFF 模式：Next.js API Route 代理所有请求，token 只在服务端内存中
- 如果必须用 localStorage，至少对所有用户输入做严格的 sanitize

---

### H5-02：gameApi.launch 返回的 URL 未经校验直接 window.open — 开放重定向风险

**文件**：`src/components/GameCard.tsx` 第 38-42 行  
**严重程度**：🔴 严重

```ts
const gameUrl = res.data?.data?.launch_url || res.data?.data?.game_url;
if (gameUrl) {
  window.open(gameUrl, '_blank');
}
```

如果后端被攻击者篡改，`launch_url` 可以是 `javascript:alert(1)` 或 `data:text/html,...`，导致 XSS。即使是正常后端，如果 URL 校验不严格，攻击者可以通过游戏厂商回调注入恶意 URL。

**修复建议**：
```ts
const isValidUrl = (url: string) => {
  try {
    const u = new URL(url);
    return ['http:', 'https:'].includes(u.protocol);
  } catch { return false; }
};
if (gameUrl && isValidUrl(gameUrl)) {
  window.open(gameUrl, '_blank');
}
```

---

### H5-03：Profile 页面使用 `any` 类型 — 类型安全缺失

**文件**：`src/app/profile/page.tsx` 第 30 行  
**严重程度**：🔴 严重（可维护性）

```ts
const [transactions, setTransactions] = useState<any[]>([]);
```

`any[]` 绕过了 TypeScript 类型检查，容易导致运行时错误（如访问不存在的属性）。后续代码中大量 `tx.type`、`tx.status`、`tx.amount` 的访问都没有类型保护。

**修复建议**：定义 `Transaction` 接口：
```ts
interface Transaction {
  id: number;
  type: 'recharge' | 'withdraw' | 'bonus';
  status: 'pending' | 'completed' | 'failed' | 'cancelled' | 'processing';
  amount: number;
  currency?: string;
  description?: string;
  order_no?: string;
  created_at?: string;
}
```

---

### H5-04：注册接口缺少客户端密码强度验证

**文件**：`src/store/auth.ts` `register` 方法  
**严重程度**：🔴 严重

注册直接将 `{ email, password, confirm_password }` 发送到后端，没有客户端验证：
- 密码长度（应 ≥ 8 字符）
- 密码复杂度（应包含字母 + 数字）
- Email 格式校验

虽然服务端应该做最终校验，但客户端校验能减少无效请求并提供即时反馈。

**修复建议**：在 `register` 方法或 `RegisterModal` 中添加验证：
```ts
if (data.password.length < 8) {
  set({ lastError: 'Password must be at least 8 characters' });
  return false;
}
```

---

## 二、🟡 中等问题

### H5-05：AppProvider 和 BottomNav 重复轮询 unread mail count — 双倍网络请求

**文件**：`src/components/AppProvider.tsx` 第 28-38 行 + `src/components/BottomNav.tsx` 第 22-32 行  
**严重程度**：🟡 中等

两个组件各自独立轮询 `mailApi.getUnreadCount()`，间隔都是 30 秒。这意味着每 30 秒发送 **2 个** 完全相同的请求。

**修复建议**：将 unread count 提升到 Zustand store 或 `AppProvider` 中，通过 context/props 传递给 `BottomNav`。

---

### H5-06：forceLogout 通过 CustomEvent 通信 — 脆弱的跨组件协调

**文件**：`src/lib/api.ts` `forceLogout()` + `src/components/AppProvider.tsx` `handleAuthLogout`  
**严重程度**：🟡 中等

认证状态通过 `window.dispatchEvent(new CustomEvent('auth:logout'))` 在模块间传递。这种方式：
1. 依赖事件名称的字符串匹配，容易拼错
2. 没有类型安全
3. `AppProvider` 和 `api.ts` 之间存在隐式耦合

**修复建议**：直接在 `forceLogout` 中调用 Zustand store 的方法（当前代码已经做了），去掉 CustomEvent。让 `AppProvider` 监听 `isLoggedIn` 状态变化而非事件。

---

### H5-07：首页硬编码数据 — 未使用 API

**文件**：`src/app/page.tsx` 第 79-95 行、第 120-135 行  
**严重程度**：🟡 中等

"Hot Games" 和 "Promotions" 预览区域使用硬编码数据：
```ts
{ name: 'Fortune Tiger', vendor: 'PG Soft', ... }
{ title: 'Welcome Bonus 200%', ... }
```

这些数据不会随后端配置变化，需要手动改代码才能更新。

**修复建议**：使用 `lobbyApi.getGames()` 和 `activityApi.getList()` 获取真实数据，硬编码作为 fallback/loading skeleton。

---

### H5-08：wallet/page.tsx DepositTab useEffect 缺少依赖

**文件**：`src/app/wallet/page.tsx` DepositTab `useEffect` 第 65 行  
**严重程度**：🟡 中等

```ts
useEffect(() => {
  setLoading(true);
  shopApi.getPaymentChannels()...
}, []);  // ← missing apiStatus dependency
```

`useEffect` 内部使用了 `apiStatus.markFailed`，但依赖数组为空。虽然功能上可能不会出问题（`apiStatus` 引用稳定），但违反了 React Hooks 规则。

**修复建议**：添加 `apiStatus` 到依赖数组，或将 `markFailed` 用 `useCallback` 包装。

---

### H5-09：tasks/page.tsx getTaskConfig 合并 3 个请求但错误处理不完整

**文件**：`src/lib/api.ts` `taskApi.getTaskConfig` 第 370-380 行  
**严重程度**：🟡 中等

```ts
const [daily, weekly, growth] = await Promise.all([
  api.get<ApiResponse<TaskItem[]>>("/task/daily"),
  api.get<ApiResponse<TaskItem[]>>("/task/weekly"),
  api.get<ApiResponse<TaskItem[]>>("/task/growth"),
]);
```

`Promise.all` 中任何一个请求失败，整个函数抛出异常。如果 daily 成功但 weekly 失败，用户看不到任何任务数据。

**修复建议**：使用 `Promise.allSettled` 或每个请求单独 `.catch(() => [])`：
```ts
const [daily, weekly, growth] = await Promise.all([
  api.get(...).catch(() => ({ data: { data: [] } })),
  api.get(...).catch(() => ({ data: { data: [] } })),
  api.get(...).catch(() => ({ data: { data: [] } })),
]);
```

---

### H5-10：fetchProfile 和 fetchAssets 静默吞掉错误

**文件**：`src/store/auth.ts` 第 90-103 行  
**严重程度**：🟡 中等

```ts
fetchProfile: async () => {
  try {
    const res = await accountApi.getProfile();
    set({ user: res.data.data });
  } catch {
    // Silently fail on auth errors
  }
},
```

如果后端返回 500（非认证错误），用户看到的是空数据，没有任何错误提示。应区分认证错误（静默）和服务端错误（提示用户）。

---

### H5-11：Change Password 弹窗无密码强度指示

**文件**：`src/app/profile/page.tsx` `handleChangePassword`  
**严重程度**：🟡 中等

```ts
if (newPassword.length < 6) {
  toast.error('Password must be at least 6 characters');
  return;
}
```

最低 6 字符的要求太弱。建议 ≥ 8 字符 + 包含字母和数字。

---

### H5-12：deposit/page.tsx 使用 window.open 打开支付页面 — 可能被弹窗拦截

**文件**：`src/app/wallet/page.tsx` DepositTab 第 130 行  
**严重程度**：🟡 中等

```ts
window.open(payUrl, '_blank');
```

大多数浏览器会阻止非用户直接触发的 `window.open`。虽然这里是在 click handler 中调用的（应该不会被拦截），但如果用户快速双击或在某些 WebView 中，仍可能被拦截。

**修复建议**：使用 `window.location.href = payUrl` 替代（同页面跳转），或在新页面打开前先显示一个"点击跳转"的确认按钮。

---

## 三、🟢 建议改进

### H5-13：api.ts 中 TOKEN_KEY 硬编码字符串应统一管理

**文件**：`src/lib/api.ts` 第 14-15 行

```ts
const TOKEN_KEY = "rockga…oken";
```

Token key 字符串散落在 `api.ts` 和 `auth.ts` 中。建议统一为常量并从单一来源导出。当前已通过 `export { TOKEN_KEY }` 解决了一部分，但 `auth.ts` 仍然同时从 `api.ts` 导入 `TOKEN_KEY` 和使用 `localStorage.getItem(TOKEN_KEY)`。

---

### H5-14：AppProvider 中 onDismiss 回调为空函数

**文件**：`src/components/AppProvider.tsx` 第 85 行

```tsx
<ConnectionBanner
  onDismiss={() => {}}
/>
```

`onDismiss` 是空函数，用户点击关闭按钮无效果。要么实现真正的关闭逻辑，要么移除关闭按钮。

---

### H5-15：GameCard 中 game.id % gameGradients.length 可能产生视觉重复

**文件**：`src/components/GameCard.tsx` 第 6 行

```ts
style={{ background: gameGradients[game.id % gameGradients.length] }}
```

如果 game.id 是连续数字，相邻游戏的渐变色会按顺序重复。建议使用 hash 函数将 id 映射到更分散的索引。

---

### H5-16：BottomNav 中 mail 轮询在组件卸载时可能未清理

**文件**：`src/components/BottomNav.tsx` 第 22-32 行

```ts
useEffect(() => {
  ...
  const timer = setInterval(fetchCount, 30000);
  return () => clearInterval(timer);
}, [isLoggedIn]);
```

当 `isLoggedIn` 变化时（从 true → false），旧的 interval 会被正确清理。但如果组件卸载时 `isLoggedIn` 没变化，interval 不会被清理（实际上 React 会在卸载时运行 cleanup，所以这是安全的）。建议添加 `isMounted` 检查防止卸载后的 setState。

---

### H5-17：Profile 页面密码修改后未强制重新登录

**文件**：`src/app/profile/page.tsx` `handleChangePassword`

密码修改成功后只是关闭弹窗和清空表单，没有调用 `logout()` 或强制重新登录。旧的 access token 可能仍然有效（取决于后端实现）。

**建议**：密码修改成功后自动 logout 并跳转到首页。

---

### H5-18：wallet/page.tsx 未使用 React.Suspense 或 ErrorBoundary

**文件**：`src/app/wallet/page.tsx`

整个钱包页面没有 ErrorBoundary 包裹。如果 `shopApi.getPaymentChannels()` 返回异常数据格式，页面可能白屏。

**建议**：在关键页面添加 ErrorBoundary。

---

### H5-19：mail/page.tsx、ranking/page.tsx、agent/page.tsx 未审查

由于时间限制，这三个页面未详细审查。建议后续补充审查，特别关注：
- mail：附件下载的安全性
- ranking：分页加载的性能
- agent：佣金计算的前端展示是否与后端一致

---

## 四、安全审查总结

### ✅ 做得好的地方
1. **Token 刷新机制**：`api.ts` 实现了完整的 token refresh + failed queue，避免并发刷新
2. **401 去重**：`LOGOUT_COOLDOWN` 防止多个 401 同时弹出登录框
3. **输入校验**：充值/提现页面有完整的金额范围校验
4. **PII 保护**：邮箱在日志中脱敏（后端实现）

### ⚠️ 风险点
1. **H5-01**：localStorage token — XSS 可窃取
2. **H5-02**：window.open 未校验 URL — 开放重定向
3. **H5-04**：注册缺少密码强度验证
4. **H5-05**：重复轮询浪费带宽

---

## 五、优先修复建议

| 优先级 | 编号 | 问题 | 影响 | 修复难度 |
|--------|------|------|------|----------|
| P0 | H5-02 | window.open URL 校验 | XSS | 低 |
| P0 | H5-04 | 注册密码强度验证 | 安全 | 低 |
| P1 | H5-01 | Token 存储方案 | XSS 窃取 | 中 |
| P1 | H5-03 | any 类型替换 | 可维护性 | 低 |
| P1 | H5-05 | 去重轮询 | 性能 | 低 |
| P2 | H5-06 | CustomEvent 替换 | 可维护性 | 中 |
| P2 | H5-07 | 首页使用真实 API | 数据一致性 | 中 |
| P2 | H5-09 | Promise.allSettled | 健壮性 | 低 |
| P2 | H5-10 | 错误处理细化 | 用户体验 | 低 |
| P3 | H5-13~18 | 代码改进建议 | 代码质量 | 低 |

---

## 六、总结

RockH5 前端项目整体架构合理，Zustand 状态管理清晰，Axios 拦截器实现了完整的 token 生命周期管理。UI 设计精致，暗色主题一致性好。

主要需要关注的是：
1. **安全性**：URL 校验（H5-02）和密码验证（H5-04）应立即修复
2. **Token 安全**：长期应迁移到 httpOnly cookie 方案（H5-01）
3. **代码质量**：消除 `any` 类型（H5-03），去重轮询（H5-05）

建议按 P0 → P1 → P2 → P3 的优先级逐步修复，P0 级别问题建议在当前迭代内完成。

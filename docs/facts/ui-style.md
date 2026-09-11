# 界面风格事实

本文件记录全局界面风格、交互原则、视觉约束和组件库使用边界。不要写单个页面的临时实现过程。

## 风格方向

- 全局样式使用 Tailwind CSS 与 CSS 变量；`web/src/styles/theme.css` 定义 light/dark 颜色、排版、圆角、图表和 sidebar token。
- 默认无衬线字体轴为 Public Sans；Lora 作为可选 serif 字体轴。`web/src/styles/theme-presets.css` 提供多套可切换主题预设，每套包含 light/dark 值。
- 应用根节点由 `ThemeProvider`、`FontProvider` 和 `DirectionProvider` 包裹，主题、字体和文字方向是跨页面能力。

## 组件库和设计系统

- 交互原语使用 `@base-ui/react`；项目组件封装集中在 `web/src/components/ui/`。
- 样式以 Tailwind 工具类为主，动态 class 由 `clsx`、`class-variance-authority` 和 `tailwind-merge` 组合。
- 表单使用 React Hook Form 与 Zod；通知使用 Sonner；图表主要使用 VChart；图标依赖 Hugeicons、Lucide 和产品图标包。
- 组件、样式和前端依赖的当前来源是 `web/package.json`、`web/src/components/ui/` 和 `web/src/styles/`。

## 交互原则

- 路由使用 TanStack Router；服务端数据与变更使用 React Query；统一 Axios 实例开启 `withCredentials`、GET 去重和 401 刷新认证处理。
- React Query 默认不在窗口聚焦时自动 refetch，stale time 为 10 秒；开发环境查询失败不重试，生产环境最多重试到配置边界，401/403 不重试。
- 前端支持 `en`、`zhCN`、`fr`、`ru`、`ja`、`vi`、`zhTW`，fallback 为 `en`；语言由 localStorage 和浏览器语言检测。
- `web/AGENTS.md` 要求用户可见文案通过 i18n、交互状态可键盘操作且视觉状态与 ARIA 状态一致。

## 页面结构约束

- 页面路由位于 `web/src/routes/`，认证后的页面使用 `_authenticated` 布局范围；功能代码按 `web/src/features/<feature>/` 组织。
- 通用组件位于 `web/src/components/`，通用请求、缓存、错误处理和构建元数据位于 `web/src/lib/`，全局状态位于 `web/src/stores/`。
- 根入口 `web/src/main.tsx` 统一安装 Query、主题、字体、方向和 Router provider，并从缓存和 `/api/status` 同步系统名称与 favicon。

## 禁止的界面偏差

- 当前前端规则禁止绕过统一 API 实例、绕过 i18n 直接固定用户文案、用不受控内联样式替代现有 Tailwind/主题 token，或让视觉状态与可访问状态不一致。
- 当前前端规则要求复用 `web/src/components/ui/` 与现有 feature 结构；新增交互不应形成平行组件体系。
- 上述约束来自 `web/AGENTS.md`；它们是当前项目规则，不代表所有现有页面已完成逐项合规审计。

## 已验证界面事实

- 2026-08-31：`bun run typecheck` 通过。
- 2026-08-31：`bun run test` 通过，79 个 test files、311 个 tests 全部通过。
- 2026-08-31：`bun run build` 通过，Rsbuild 产出 `web/dist`。
- 待定：本次未启动浏览器进行页面视觉、响应式、键盘或可访问性验收，因此构建通过不证明实际页面视觉完全正确。

## 待确认事项

- 待定：生产当前启用的主题预设、字体和品牌配置。
- 待定：全站 i18n 完整度、WCAG 2.1 AA 实测结果和关键流程浏览器回归。
- 待定：构建产物中大体积异步 chunk 的当前性能影响；本次只确认构建成功。

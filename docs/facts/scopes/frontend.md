# 管理与用户前端范围事实

## 范围

- 路径：`web/`。
- 技术栈及版本：React `^19.2.7`、Rsbuild `^2.1.4`、Base UI `^1.6.0`、Tailwind CSS `^4.3.2`、Bun。
- 相关横向事实：`docs/facts/ui-style.md`、`docs/facts/architecture.md`、`docs/facts/product-domain.md`、`docs/facts/integrations.md`。

## 当前实现

前端包含认证、聊天、Dashboard、渠道、模型、定价、订阅、钱包、日志、系统设置、OAuth、setup、错误页等路由与 feature。入口安装 TanStack Router、React Query、主题、字体、文字方向和 i18n provider，并通过统一 Axios client 调用同源 `/api`。

## 事实来源

`web/package.json`、`web/AGENTS.md`、`web/src/main.tsx`、`web/src/routes/`、`web/src/features/`、`web/src/components/`、`web/src/lib/http-client.ts`、`web/src/i18n/`，2026-08-31 前端检查输出，以及 2026-09-01 `test.tryvalo.com` 钱包页回读。

## 如何承载全局业务规则

- 用户、Token、Channel、钱包、订阅和日志对象由对应 feature 页面展示和操作，最终语义以后端 API 与 `docs/facts/product-domain.md` 为准。
- 钱包页的 Stripe 订阅账单组件直接格式化后端 `current_period_end`，根据是否已安排取消显示“下次账单日期”或“访问结束日期”。后端对历史订单加入 settlement 回退后，无需修改前端即可恢复有效日期显示。
- 认证状态由统一 auth store、HTTP interceptor 和认证路由承载；401 刷新失败会清理认证并跳转 sign-in。
- 用户可见文案经 i18n，主题与方向由根 provider 统一承载。
- 系统设置的敏感词区展示高风险阻断、仅审计和 NSFW 阻断三个独立列表。保存时先写高风险和仅审计列表，再缩减兼容选项 `SensitiveWords`，最后写提示词检查和总开关；任一步返回失败都会停止后续写入。相关文案已覆盖现有七种语言。

## 接口、页面、集合或模块

- 路由：`web/src/routes/`，认证布局使用 `_authenticated`。
- 功能：`web/src/features/`。
- 通用组件：`web/src/components/` 与 `web/src/components/ui/`。
- 通用状态与请求：`web/src/stores/`、`web/src/lib/`、`web/src/i18n/`。

## 验证状态

已确认：2026-08-31 使用 Bun `1.3.14` 执行 `bun run typecheck`、`bun run test` 和 `bun run build` 均通过；测试结果为 79 files、311 tests。2026-09-01 当前工作树再次通过类型检查、全量测试和生产构建，全量测试结果为 80 files、314 tests；敏感词设置另有保存 payload、固定顺序、未修改项跳过和失败中止测试。同日 `test.tryvalo.com` 钱包页已回读确认历史 Stripe 订单显示“下次账单日期：2026年10月1日 10:59”，原“不可用”状态已闭合；未执行其他页面的完整浏览器视觉或 E2E 验收。

## 已知约束

- `web/AGENTS.md` 要求使用 Bun、统一 API client、i18n、现有 UI 组件和当前测试布局。
- Docker 使用 `oven/bun:1`，CI 固定 Bun `1.3.14`，release workflow 使用 latest；不同上下文的版本选择器并未统一为单一 pin。
- 构建产物位于 `web/dist`，该目录供根 Go 程序嵌入。

## 待确认事项

- 待定：除上述 Stripe 钱包账单日期外，关键用户流程的浏览器、响应式、键盘和可访问性验证结果。
- 待定：生产启用的品牌、主题、语言和外部前端部署方式。

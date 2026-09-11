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
- 当前本地首页、钱包套餐和购买弹窗只呈现套餐价格、额度和一个月有效期，不展示“一次性付款”或“不自动续费”的说明，也不恢复自动续费承诺；旧 Stripe 自动续费账单及 portal 组件已移除。后台账号的订阅管理保留“取消订阅”按钮和二次确认，调用内部权益失效接口，不调用 Stripe 取消或退款接口。契约见 `docs/facts/integrations.md`。
- 认证状态由统一 auth store、HTTP interceptor 和认证路由承载；401 刷新失败会清理认证并跳转 sign-in。
- 用户可见文案经 i18n，主题与方向由根 provider 统一承载。
- 系统设置的敏感词区展示高风险阻断、仅审计和 NSFW 阻断三个独立列表。保存时先写高风险和仅审计列表，再缩减兼容选项 `SensitiveWords`，最后写提示词检查和总开关；任一步返回失败都会停止后续写入。相关文案已覆盖现有七种语言。
- 公开站点运行时配置存在且当前 origin 精确匹配时，根路由维护 canonical link，并在用户明确允许 analytics 后初始化 GA4 与 Clarity。GA4 关闭自动 `page_view`，由路由变更手动去重发送；当前业务事件覆盖注册、登录、API Key 创建、流式模型请求开始/成功及创建 Checkout。事件参数使用白名单和敏感名称过滤，不发送邮箱、密码、API Key、提示词、内容、文件、URL、用户或会话标识。`checkout_created` 只表示客户端获得支付跳转，不能表示付款或权益已完成。

## 接口、页面、集合或模块

- 路由：`web/src/routes/`，认证布局使用 `_authenticated`。
- 功能：`web/src/features/`。
- 通用组件：`web/src/components/` 与 `web/src/components/ui/`。
- 通用状态与请求：`web/src/stores/`、`web/src/lib/`、`web/src/i18n/`。

## 验证状态

已确认：2026-08-31 使用 Bun `1.3.14` 执行 `bun run typecheck`、`bun run test` 和 `bun run build` 均通过；测试结果为 79 files、311 tests。2026-09-01 当前工作树再次通过类型检查、全量测试和生产构建，全量测试结果为 80 files、314 tests；敏感词设置另有保存 payload、固定顺序、未修改项跳过和失败中止测试。2026-09-03 当前工作树运行 `bun run test`（82 files、323 tests）、`bun run typecheck`、`bun run lint`、`bun run format:check` 和 `bun run build` 均通过；仅有既有无关 lint warning。同日 `test.tryvalo.com` 钱包页已回读确认历史 Stripe 订单显示“下次账单日期：2026年10月1日 10:59”，原“不可用”状态已闭合。随后该前端已随 `new-api:new-api-release-20260903T094447Z-e40d88d1535` 上线：生产初始页显示独立 analytics consent 且没有 GA4/Clarity 资源；GA4 精确 Web stream 当前 Realtime 无数据，Clarity 精确项目仍在安装引导。因此用户主动同意后的 production transport 与 Clarity Dashboard/录制 E2E 仍待定。

已确认（2026-09-07，本地）：套餐展示、七种语言翻译及后台取消交互的相关测试通过。全量测试为 81 files、322 tests；后台取消测试通过真实组件交互确认内部失效请求和取消后的状态。页面不强调付款方式的修订不改变一次性支付契约；不代表远端部署或真实付款验收。

同日通过本地构建产物和模拟套餐 API 在浏览器回读首页：三档卡片只显示月度价格、额度和有效期，没有“一次性付款”“不自动续费”或自动续费承诺。该只读预览禁用写请求，不连接真实 Stripe 或生产后台。

## 已知约束

- `web/AGENTS.md` 要求使用 Bun、统一 API client、i18n、现有 UI 组件和当前测试布局。
- Docker 使用 `oven/bun:1`，CI 固定 Bun `1.3.14`，release workflow 使用 latest；不同上下文的版本选择器并未统一为单一 pin。
- 构建产物位于 `web/dist`，该目录供根 Go 程序嵌入。

## 待确认事项

- 待定：除上述 Stripe 钱包账单日期外，关键用户流程的浏览器、响应式、键盘和可访问性验证结果。
- 待定：生产启用的品牌、主题、语言和外部前端部署方式。

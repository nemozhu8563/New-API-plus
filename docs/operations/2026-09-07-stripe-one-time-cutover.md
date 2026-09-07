# Stripe 一次性付款预配置与切换记录

## 范围与状态

- 用户确认：Stripe 只收一次性款，套餐有效期和额度由 Tryvalo 内部管理；没有既有已付订阅需要兼容。
- 展示边界：页面不增加“一次性付款”或“不自动续费”的描述，只保留套餐价格、额度和有效期；后台账号保留内部取消入口。这不改变 Stripe one-time-only 的支付实现。
- 2026-09-07 14:07:46～14:09:17（Asia/Shanghai）：已在 Tryvalo Sandbox 和 Live 各创建三档 one-time Price，并通过 API 重新查询验证。没有创建 Checkout、PaymentIntent 或付款。
- 已验证：六个 Price 均为 active、`type=one_time`、`recurring=null`、CNY、固定 per-unit；旧 Product 的默认 Price 保持不变。
- 尚未执行：应用部署、数据库套餐 Price 绑定切换、旧 Price 归档、旧自动续费数据删除、真实支付验收。
- 本次未改变 DNS、代理、防火墙、API 密钥、webhook secret、Stripe Tax 或支付方式配置。

## 已准备的 Price 映射

| 环境 | 套餐 | 一次付款 | Product | 新 one-time Price | 切换前 recurring Price |
| --- | --- | --- | --- | --- | --- |
| Sandbox | Standard | CNY 259 | `prod_V6hCDrLZBHvXNX` | `price_1UCviY87I3CPUHK9buZnFUWG` | `price_1U9Zft87I3CPUHK9P2wfhsGg` |
| Sandbox | Premium | CNY 599 | `prod_V6hDK8NEOzQSxS` | `price_1UCvjC87I3CPUHK9n1KW5Lot` | `price_1U9Zfe87I3CPUHK9MimV2OOl` |
| Sandbox | Professional | CNY 1099 | `prod_V6hDZWO4jCNWsF` | `price_1UCvjK87I3CPUHK9VP7S9GSb` | `price_1U9Zfn87I3CPUHK93dTCGI9k` |
| Live | Standard | CNY 259 | `prod_V9tU6KaZAeMh05` | `price_1UCvjm7HJXYkKmfAgwn5Ufsw` | `price_1U9ZiE7HJXYkKmfAas76TAp9` |
| Live | Premium | CNY 599 | `prod_V9tUBvfWVfc2VR` | `price_1UCvju7HJXYkKmfAuK0f3jT2` | `price_1U9Zi87HJXYkKmfAqJNDef3P` |
| Live | Professional | CNY 1099 | `prod_V9tUKJ9vlUlSPL` | `price_1UCvk17HJXYkKmfAFmYh8FZD` | `price_1U9ZiK7HJXYkKmfAHFWsBGZr` |

Lookup key：`tryvalo_<standard|premium|professional>_cny_onetime_month_v1`。权益仍为一个月；Product 现有标注额度分别为 290、710、1375 Credits。支付元数据不作为额度来源，实际发放使用本地订单快照。

账户：Sandbox `acct_1U49oP87I3CPUHK9`；Live `acct_1U49oF7HJXYkKmfA`。未操作其他账户。

## Webhook 预检

- Sandbox：`we_1U4B9V87I3CPUHK9JMf6R5pv` → `https://test.tryvalo.com/api/stripe/webhook`，enabled，API version `2026-07-29.dahlia`。
- Live：`we_1U9wAc7HJXYkKmfA5xcFnJot` → `https://api.tryvalo.com/api/stripe/webhook`，enabled，API version 为账户默认值（API 回读为 null）。
- 两者当前均含 4 个 Checkout 事件、4 个 invoice/subscription 事件、9 个退款/争议事件。一次性结算需要的 Checkout 与退款/争议事件已存在，预配置阶段不修改事件列表。
- 以上仅证明 provider 配置，不证明应用签名验证、真实回调或权益发放。

## 本地简化计划

1. 先用现有一次性付款、快照、退款/争议回归测试锁定保留行为，并新增 recurring 拒绝测试。
2. Checkout 固定 `mode=payment`；删除自动续费专属字段、invoice settlement/lock 模型、生命周期处理、Stripe 自动续费取消/portal API 和 UI。保留后台账号的内部“取消订阅”按钮及接口；取消立即终止权益，但不自动退款。
3. 保留 `SubscriptionOrder → UserSubscription`、不可变快照、PaymentIntent/Charge 支付引用、幂等结算、退款/争议回收与债务处理。
4. 不自动删除远端旧表、旧订单或 Stripe 订阅；代码停止依赖旧模型。部署前单独确认任何需要清理的测试数据。
5. 运行 Go 与前端测试、类型检查和构建，更新事实文件与本记录。

## 晚间切换门禁（尚未执行）

已有部署记录及旧代码使用 recurring Checkout；本次未重新读取远端运行镜像或数据库，不能提前把套餐绑定改成 one-time Price。

1. 确认待发布代码、目标镜像和数据库备份；只读核验套餐金额/额度/Price，以及未完成 Checkout 和远端 recurring 订阅是否需要人工处理。若远端保留 `subscription_orders.provider_subscription_id` 旧列，确认该列可为 NULL 且无强制默认值，避免旧唯一索引阻止新订单；异常 schema 须先处理，不由本次代码增加旧模式兼容。
2. 先测试环境：暂停新套餐购买，在同一维护窗口发布 one-time-only 代码并将三档 `stripe_price_id` 改为本表 Sandbox 新值；刷新配置/套餐缓存。
3. 验证 Sandbox 付款、异步成功/失败、重复回调、订单快照发放及退款/争议；确认没有创建 Stripe Subscription。
4. 测试通过且获正式发布授权后，以相同步骤切换 Live。Product 默认价格和说明、旧 Price 归档、webhook 事件精简在切换时一并处理，不在预配置阶段改变正在使用的目录。
5. 记录镜像 SHA/ID、时间、环境配置摘要、绑定回读、健康检查和验收结果后才标记切换完成。

回滚：预配置只新增 Price，旧价格仍 active 且默认绑定不变。部署后如需回滚，应在暂停购买状态下同时恢复旧代码与旧 Price 映射；不得让旧代码接入新 Price。旧字段/表本次不自动删除，物理清理需另行确认及备份。

## 验证记录

- Provider：六个 Price 的金额、currency、livemode、Product、type、recurring、active 及默认价格保持不变已回读。
- 本地代码简化：已完成。内部套餐、订单快照、权益及支付引用保留；自动续费专属模型/API/UI 已移除，无物理删表或删列操作。
- 后端：`GOWORK=off go test ./controller ./model ./router ./common . -count=1`、相同包 `go vet` 和 `GOWORK=off go build ./...` 通过。
- 前端：类型检查、全量测试、lint（无 error）与生产构建通过；翻译同步完成。全量格式检查只发现原有无关文件 `web/src/features/dashboard/components/overview/__tests__/summary-cards.test.tsx`，本次涉及文件已格式化。
- 取消验收：后台真实组件交互测试确认按钮调用内部 invalidate 接口；模型测试确认立即停止权益、不自动退款、后续退款撤销不重新激活已取消权益。
- 最终文案修订：前端 81 files、322 tests 通过；本地浏览器使用构建产物和模拟 API 确认三档套餐只呈现价格、额度、有效期，未显示付款方式说明。预览禁止写请求，不连接生产或 Stripe。
- 应用/数据库切换及真实交易：未执行。

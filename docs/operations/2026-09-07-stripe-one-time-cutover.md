# Stripe 一次性付款预配置与切换记录

## 范围与状态

- 用户确认：Stripe 只收一次性款，套餐有效期和额度由 Tryvalo 内部管理；没有既有已付订阅需要兼容。
- 展示边界：页面不增加“一次性付款”或“不自动续费”的描述，只保留套餐价格、额度和有效期；后台账号保留内部取消入口。这不改变 Stripe one-time-only 的支付实现。
- 2026-09-07 14:07:46～14:09:17（Asia/Shanghai）：已在 Tryvalo Sandbox 和 Live 各创建三档 one-time Price，并通过 API 重新查询验证。没有创建 Checkout、PaymentIntent 或付款。
- 已验证：六个 Price 均为 active、`type=one_time`、`recurring=null`、CNY、固定 per-unit；旧 Product 的默认 Price 保持不变。
- 已执行（2026-09-07，Asia/Shanghai）：提交 `22a1b1d82ed26a03f4bcd76a8a1dd74c3332fc48` 已先后发布至 GreenCloud 测试与正式应用；两者运行同一不可变 `linux/amd64` 镜像 ID `sha256:44fe064881dc38b72405976d72e8646fb1ff0ed6ce62ba909e253be4c5f388f6`。测试 tag 为 `new-api:new-api-test-20260907T135216Z-22a1b1d82`，正式 tag 为 `new-api:new-api-release-20260907T135216Z-22a1b1d82`，传输包 SHA-256 为 `8ae2ea1165cac2fbfa7e25278b198f1d416a80b9bf93d54da389986f05ec89d4`。
- 已执行：测试和正式数据库的三档套餐 `stripe_price_id` 均已切换到下表的新 one-time Price；缓存已清理并重建对应应用。正式三档在切换前后均以数据库回读约束为准，切换前 `subscription_orders=0`、active `user_subscriptions=0`，因此没有旧订单或活跃权益需要迁移。
- 未执行：旧 Price 归档、旧自动续费数据的物理删除、Product 默认 Price 改写，以及真实 Checkout、付款、Webhook、退款或争议验收。
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

## 已完成的本地代码简化

1. 已用一次性付款、快照、退款/争议回归测试锁定保留行为，并加入 recurring Price 拒绝测试。
2. Checkout 固定 `mode=payment`；自动续费专属字段、invoice settlement/lock 模型、生命周期处理、Stripe 自动续费取消/portal API 和 UI 已从当前代码移除。后台账号的内部“取消订阅”入口保留，取消立即终止权益且不自动退款。
3. `SubscriptionOrder → UserSubscription`、不可变快照、PaymentIntent/Charge 支付引用、幂等结算、退款/争议回收与债务处理保持。
4. 本次没有物理删除远端旧表、旧订单或 Stripe 订阅；当前代码不再依赖旧自动续费模型。
5. Go 与前端检查、类型检查和构建已在发布前通过，事实文件与本记录已随发布收尾更新。

## 切换执行与门禁结果

1. G1 已满足：测试备份位于 `/srv/new-api-test/backups/new-api-test-20260907T135216Z-22a1b1d82`，正式备份位于 `/srv/new-api/backups/new-api-release-20260907T135216Z-22a1b1d82`；两处均已通过各自 `SHA256SUMS` 校验。
2. 测试环境：发布并验收新镜像后，Sandbox Plan `2/3/4` 已公开返回且 `stripe_checkout_available=true`；维护窗口中原有一笔 pending Stripe 订单（ID `2`）已标记为 `expired`。Redis DB `1` 的套餐缓存已清理并二次重建应用，`127.0.0.1:3001/api/status`、`https://test.tryvalo.com/api/status`、`/wallet` 与 `/login` 均为 HTTP `200`。
3. 正式环境：先锁表校验三档 Live Price 映射均正确且 `enabled=false`，并再次断言没有套餐订单和活跃内部权益；随后将 Plan `1/2/3` 置为 `enabled=true`，清理 Redis DB `0` 的套餐与套餐信息缓存，并于 `2026-09-07T22:59:29+08:00` 仅重建 `new-api`。
4. 正式回读：`new-api` 为 `running/healthy`、重启次数 `0`；PostgreSQL 与 Redis 容器 ID/启动时间未变化。`127.0.0.1:3000/api/status`、`https://tryvalo.com/api/status`、`/wallet`、`/login`、`https://api.tryvalo.com/api/status` 与 `https://new.tryvalo.com/api/status` 均为 HTTP `200`。公开套餐 API 只返回 Plan `1/2/3`，三项均标记 Stripe Checkout 可用。
5. 测试和正式应用均使用上述同一镜像 ID；两套端点的直接 HTTPS 首次连接和复用连接均为 HTTP `200`，复用连接未新建 TCP/TLS 连接。

回滚：如必须撤回，在暂停购买状态下恢复对应备份目录中的镜像配置与旧 recurring Price 映射，并只重建应用；不得让旧代码接入新 one-time Price，或让新代码接入旧 recurring Price。本次未演练回滚，旧字段/表、旧 Price 与 Product 默认 Price 都没有自动删除或变更。

## 验证记录

- Provider：六个 Price 的金额、currency、livemode、Product、type、recurring、active 及默认价格保持不变已回读。
- 本地代码简化：已完成。内部套餐、订单快照、权益及支付引用保留；自动续费专属模型/API/UI 已移除，无物理删表或删列操作。
- 后端：`GOWORK=off go test ./controller ./model ./router ./common . -count=1`、相同包 `go vet` 和 `GOWORK=off go build ./...` 通过。
- 前端：类型检查、全量测试、lint（无 error）与生产构建通过；翻译同步完成。全量格式检查只发现原有无关文件 `web/src/features/dashboard/components/overview/__tests__/summary-cards.test.tsx`，本次涉及文件已格式化。
- 取消验收：后台真实组件交互测试确认按钮调用内部 invalidate 接口；模型测试确认立即停止权益、不自动退款、后续退款撤销不重新激活已取消权益。
- 最终文案修订：前端 81 files、322 tests 通过；本地浏览器使用构建产物和模拟 API 确认三档套餐只呈现价格、额度、有效期，未显示付款方式说明。预览禁止写请求，不连接生产或 Stripe。
- 应用和数据库套餐绑定切换：已完成，测试与正式应用均健康；正式 PostgreSQL、Redis 未重建。
- 未发起真实 Stripe Sandbox 或 Live Checkout、付款、Webhook、退款或争议 E2E；因此不能把接口、健康检查或 Price 映射回读当作实际收费、权益发放或退款闭环证据。

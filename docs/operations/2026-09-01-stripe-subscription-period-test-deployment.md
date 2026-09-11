# Tryvalo Stripe 订阅账期测试发布记录

## 结果

Stripe 订阅账期修复提交 `f7cc1525c7795fd9d254e1cc7dc291607d3ff907` 与 `89e4d3a9116dc31bb4a87c6f5e26640904f76431` 已发布到 GreenCloud 独立测试实例，并以 Stripe Sandbox Standard `CNY 259/月` 完成真实首购 E2E。

最新订单已保存 `stripe_current_period_end=1790838860`（2026-10-01 15:14:20，Asia/Shanghai），与 Stripe invoice、测试库唯一 settlement 和唯一 active 权益的结束时间一致。较早订单没有做数据回填，字段仍可为 `0`；账单读取会在字段缺失或早于最新已付 settlement 时采用 settlement，钱包页已显示“下次账单日期：2026年10月1日 10:59”。原“下次账单日期：不可用”缺陷已在测试环境闭合。

本次没有发布生产、触发 Stripe Sandbox 续费/退款/争议，也没有执行 Stripe Live 交易。

| 阶段 | 状态 | 证据 |
| --- | --- | --- |
| Planned | 已完成 | 修复范围、不可变测试镜像、数据库备份、Sandbox 首购和回滚边界在执行前确定。 |
| Code validation | 已完成 | `go test ./model ./controller -count=1` 在宿主可见边界通过。 |
| Test execution | 已完成 | 账期镜像已运行于 GreenCloud `new-api-test`；生产镜像未变化。 |
| Sandbox first purchase | 已完成 | Checkout `complete/paid`、invoice `paid`、subscription `active`。 |
| Period persistence | 已完成 | 订单、invoice、settlement 与 active 权益的结束时间一致。 |
| Historical billing-date recovery | 已完成 | 历史订单通过 settlement 回退后，钱包页显示有效下次账单日期。 |
| Sandbox renewal/refund/dispute | 待定 | 本轮没有触发这些生命周期。 |
| Stripe Live execution | 未执行 | 没有创建或支付 Live Checkout。 |
| Production execution | 未执行 | 生产仍运行 `f96bf33b80dfeca9b025a94651fb68db492dc8a7` 对应镜像。 |
| Rollback | 未执行 | 当前后继测试镜像健康，发布前 Compose 与测试库 dump 已保留。 |

## 代码范围

- `f7cc1525c7795fd9d254e1cc7dc291607d3ff907`：`invoice.paid` 结算时保存 Stripe 服务周期结束时间，并让生命周期事件按事件时间保持单调。
- `89e4d3a9116dc31bb4a87c6f5e26640904f76431`：账单读取在订单周期缺失或较旧时采用最新已付 settlement，覆盖历史数据缺口。
- 新订单仍由正常 webhook 路径持久化账期；历史订单只在读取时恢复，不做批量迁移或字段回写。
- 本次没有 schema、Product/Price、Webhook endpoint、密钥、DNS、Caddy、Cloudflare、Zgo、CPA 或生产配置变更。

## 发布身份

- 账期保存提交：`f7cc1525c7795fd9d254e1cc7dc291607d3ff907`
- 历史回退提交：`89e4d3a9116dc31bb4a87c6f5e26640904f76431`
- E2E 测试镜像：`new-api:new-api-test-20260901T062743Z-89e4d3a911`
- 镜像 ID：`sha256:7cc64698a07c88021fa93b0d58a9d6f50c0a35745e279f000c826758f6f8439c`
- 平台：`linux/amd64`
- 传输包 SHA-256：`03a091d122fa681c05e374b84f20226fa262e911582dd2ca6468b4dfc87f3e72`
- 当前测试镜像：`new-api:new-api-test-20260901T064123Z-fc6ebe122e`
- 当前测试镜像 ID：`sha256:268d7cff740cfdbb7b97d9f3f3d8b651c8633d2eeb60c4bc92a869b246a34876`

当前测试镜像的 OCI revision `fc6ebe122e32cd131fe7226af5e5c2e8780e9c75` 是 `89e4d3a911` 的后继提交并包含账期修复。支付 E2E 的运行证据归属于 `89e4d3a911` 镜像；后续敏感词测试发布没有重新执行支付交易。

## 发布前验证

- `go test ./model ./controller -count=1`：通过；`model` 与 `controller` 均为 `ok`。
- 回归测试覆盖：paid invoice 周期写入、重复结算幂等、下一周期更新、历史 `0`/较旧周期回退、`invoice.paid` 与 Checkout webhook 乱序，以及旧生命周期事件不得回退新状态或账期。
- 本轮本地回归测试使用 SQLite；GORM 查询未发现方言专属语法，但真实 MySQL 和 PostgreSQL 仍未实测。

## 测试部署

- Compose：`/srv/new-api-test/compose.yaml`
- 服务：`new-api-test`
- E2E 镜像：`new-api:new-api-test-20260901T062743Z-89e4d3a911`
- 后续当前镜像：`new-api:new-api-test-20260901T064123Z-fc6ebe122e`
- 后续当前镜像状态：2026-09-01 16:34（Asia/Shanghai）回读为 `running/healthy`、重启次数 `0`
- 当前本机 `/api/status`：HTTP `200`
- 当前公网 `https://test.tryvalo.com/api/status`：HTTP `200`

测试发布只重建应用服务：

```bash
docker compose -f /srv/new-api-test/compose.yaml config -q
docker compose -f /srv/new-api-test/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api-test
```

## 备份与回滚材料

备份目录：`/srv/new-api-test/backups/new-api-test-20260901T062743Z-89e4d3a911/`

| 文件 | SHA-256 | 校验 |
| --- | --- | --- |
| `compose.yaml.before` | `62fd95c4269a1ad53a76fc5f792514e731b55a4fef2ca5ac9f3a3d5212c3b76c` | 发布前 Compose；`761` bytes、mode `644`。 |
| `newapi_test.before.dump` | `2ea2551b739bfa878c2fe626c3baffb13d7145cd8419c37ca6a6bb19e8afd43a` | PostgreSQL custom-format 测试库 dump；`22,582,167` bytes、mode `600`；`pg_restore --list` 为 `534` 行。 |

本文档不复制备份内容，也不记录 Stripe secret、Webhook signing secret、Cookie、Token、Customer ID、PaymentMethod ID 或完整交易对象 ID。

## Stripe Sandbox E2E

| 检查面 | 结果 | 结论 |
| --- | --- | --- |
| Hosted Checkout | `status=complete`、`payment_status=paid`、`livemode=false` | Sandbox 首购付款完成。 |
| Subscription | `status=active` | 首购后订阅处于 active。 |
| Invoice | `status=paid` | 首张 invoice 已付。 |
| Webhook | `invoice.paid` 与 `checkout.session.completed` 均为 `succeeded`、`attempts=1` | 两类目标事件各成功处理一次。 |
| Order | `success`；`stripe_current_period_end=1790838860` | 新订单账期已持久化。 |
| Settlement | 恰好一条；`period_end=1790838860` | 没有重复结算，周期与订单一致。 |
| Entitlement | 恰好一个 active；`end_time=1790838860`；quota `145000000`；used quota `0` | 首期 290 Credits 权益与 invoice 周期一致。 |
| Historical read fallback | 旧订单字段仍为 `0`，API 采用最新已付 settlement | 不迁移历史数据也能恢复账单日期。 |
| Wallet page | “下次账单日期：2026年10月1日 10:59” | 原“不可用”显示已闭合。 |

## Automatic Tax 边界

- 最新 Sandbox Checkout 的 `automatic_tax.enabled=true`、`automatic_tax.liability.type=stripe`、`managed_payments.enabled=true`、`livemode=false`。
- 对应 subscription 和 invoice 同样显示 Automatic Tax 已启用。
- invoice 税额为 `0`，`taxability_reason=product_exempt`。
- 仓库代码扫描没有发现应用显式设置 `automatic_tax`；因此这里只能确认 Stripe Managed Payments 生成的本轮 Sandbox 对象状态，不能写成应用代码主动开启 Automatic Tax。
- 本轮没有产生非零税额，不确认一般计税正确性；Stripe Live Tax、有效税务注册和申报准备度保持待定。

## 生产不变性

- 生产镜像仍为 `new-api:new-api-release-20260830T025223Z-f96bf33b80`。
- 生产提交仍为 `f96bf33b80dfeca9b025a94651fb68db492dc8a7`。
- 本轮没有重建生产应用、PostgreSQL 或 Redis，也没有修改生产 Stripe、DNS、Cloudflare、Caddy、Zgo、CPA 或防火墙配置。
- 测试环境的 Sandbox 结果不能扩展为 Stripe Live 或生产支付验收。

## 已知限制

- 只验证了 Standard 月付首次购买和首期权益，没有触发 Sandbox 续费、退款或争议。
- 退款和争议处理有本地测试，但没有本轮真实 Stripe 对象与持久化读回，不能视为 E2E 完成。
- 历史订单字段没有批量回填；当前修复依赖账单读取时的 settlement 回退。
- 本轮 invoice 因 `product_exempt` 税额为 `0`，没有验证非零税额交易。
- Stripe Live 充值、订阅、签名回调、结算、续费、退款、争议和税务状态均未验证。
- MySQL、PostgreSQL 的账期查询与结算路径尚未做真实实例测试。

## 回滚

如需回到该次发布前的测试应用配置，恢复 Compose 并只重建 `new-api-test`：

```bash
cp -p \
  /srv/new-api-test/backups/new-api-test-20260901T062743Z-89e4d3a911/compose.yaml.before \
  /srv/new-api-test/compose.yaml
docker compose -f /srv/new-api-test/compose.yaml config -q
docker compose -f /srv/new-api-test/compose.yaml up -d \
  --no-deps --no-build --pull never --force-recreate new-api-test
```

该回滚只恢复应用镜像配置，不恢复数据库。本次代码没有 schema 变更；若需要删除或恢复测试交易数据，必须停止测试写入并执行单独审核的数据恢复方案。

# Stripe 钱包容量修复与正式入账恢复

## 原因与范围

已付款充值的 Webhook 返回 503：应用把单次计费 `MaxQuota=2147483647` 当成累计钱包上限。正式 `users.quota` 实际是 bigint；用户调整后的余额仍为 2500000000，因此旧检查继续拒绝。

代码提交 `3abc3c5574f668df4d1278c8180ccfd761205b53`：充值钱包容量采用 `MaxWalletQuota=min(2^53-1, math.MaxInt)`，单次计费饱和/输入约束保持 int32；Stripe 创建 Checkout 前及行锁结算时检查抵债后的净增加额。争议恢复使用钱包容量检查。未修改数据库 schema、Stripe 设置或环境变量。兑换码和返现转余额的既有上限不在本次范围。

## 计划、执行与验证

计划：回归测试、构建、备份、仅部署正式应用、重发原已付款事件并对账。全部已执行并验证。

- 本地 `GOWORK=off go test ./...`、`GOWORK=off go vet ./common ./model ./controller`、`git diff --check` 通过；Docker `linux/amd64` 构建通过。覆盖大余额、精确上限、越界、债务抵扣、付款前拒绝、失败回调重试及幂等、争议恢复。MySQL/PostgreSQL 类型映射测试通过，但未在真实 MySQL 上运行交易测试。
- 目标：GreenCloud `173.249.203.66`，主机 `nemo-Phoenix`，正式容器 `new-api`。生产启动于 2026-09-08 11:10:45（Asia/Shanghai），验收到 11:15。
- 镜像：`new-api:new-api-release-20260908-wallet-3abc3c557`；本地与远端 ID 均为 `sha256:3b53d4fccc449e54d927474069e42e84e3fbc3c60c0629618b4c2de2a8f17358`。构建工作树恰为该提交的源文件差异。
- 传输包 `/tmp/new-api-wallet-3abc3c557.tar.gz`，两端 SHA-256 校验为 `b214b0c4782d8ef14ae9880dbd2e27d6b6bda1ad94c39f43b96554813e1256e3`，压缩完整性验证通过。
- 备份 `/srv/new-api/backups/new-api-wallet-3abc3c557/`：`images.env.before` 摘要 `dbebeffcfe9105667d6eda78a9390308272b9339cab96d184e03aa85e50b75cd2`；`compose.yaml.before` 摘要 `639aff8994bef03e69bafde58093fe013ac9ed0b01d73586f8e89a274e44d279`。
- 只替换 `images.env` 的 `NEW_API_IMAGE`。运行 Compose `config -q` 后执行 `docker compose --env-file /srv/new-api/env/images.env -f /srv/new-api/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api`。
- 正式 `running/healthy`、重启 0。本机 `127.0.0.1:3000/api/status` 和公网 `api.tryvalo.com`、`new.tryvalo.com` 的 `/api/status` 均 HTTP 200。
- 测试仍为 `sha256:8003ade183e32f62f95bb661b48fb8026fbebaacc37394a7b95af5f56fd1ca2b`，启动时间 `2026-09-08T01:35:58.702917709Z` 未变；PostgreSQL/Redis 启动时间仍为 `2026-07-11T01:00:44.13346078Z` / `2026-07-12T00:26:12.673497206Z`，均 healthy，未重建。
- 发布后 `images.env` 摘要 `461634f74c5c3d1e503672995ca62564bf352a0a3be25d0605560b0366386d17`；应用环境文件保持 `b8933864ea508bccb85ed25e78df29704dcb4bad616908427b0caeac72acdf9b`；Compose 保持原摘要。
- 未改 DNS、Caddy、代理或防火墙；沿用既有拓扑：`api.tryvalo.com` 经 Zgo `64.83.30.150` 回源 GreenCloud，`new.tryvalo.com` 直达 GreenCloud。此次只回读 HTTPS 可达性，未重新审计完整网络规则。

## 已付款订单恢复证据

- 精确事件 `evt_1UDEun7HJXYkKmfACJRjkJwU`；订单 `ref_d266ba670b4edd6b5d1dc3441e970866e29d7cda`（top-up 12）；PaymentIntent `pi_3UDEu97HJXYkKmfA0poLNZNn`。
- 发布前事件 `failed`、attempts 4，充值单 `pending`、complete_time 0；余额 2500000000，债务 0。
- 经用户授权，从 Stripe Workbench 对原事件点击一次重新发送。11:13:05（Asia/Shanghai）正式 `POST /api/stripe/webhook` 返回 HTTP 200，请求 ID `202609080313049946362668268d9d6AQirFhA5`。
- 事件变为 `succeeded`、attempts 5、last_error 空；充值单 `success`，complete_time `1788837185`。余额变为 2510000000，债务仍 0，净增 10000000，恰为该 CNY 20 订单快照额度；发布后该用户仅一条充值日志。
- Stripe Workbench 刷新后读回本次 `200 OK / 已送达`；对应订单付款引用与恢复账本各一行。旧 503 为历史投递记录，不代表本次重发失败。
- 本次未直接修改余额、未新建付款、未退款；入账由真实签名 Webhook 与既有事务完成。重复回调不重复入账已由本地回归验证，不为验收额外重发已成功事件。

## 回滚与边界

未回滚。旧镜像 `new-api:new-api-release-20260908-managed-off-3186b5c8d` 保留；需要回退时恢复备份 `images.env.before`，先 Compose 配置校验，再仅重建 `new-api`。回滚会恢复错误的低钱包容量检查，不能撤销已完成入账；不得还原数据库来回滚此应用发布。

真实 Live 充值签名回调与持久化入账已验证；套餐权益、真实退款/争议、税务及全量钱包路径不由此证明。未执行数据库恢复或应用回滚演练。

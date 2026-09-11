# Stripe Checkout Managed Payments 退出与正式发布

## 范围与原因

- 代码提交：`3186b5c8d`。充值与一次性套餐 Checkout 显式传 `managed_payments.enabled=false`；保留动态支付方式、WeChat Web 参数和订阅 PMC 环境变量，不新增环境变量。
- Live 账户默认启用 Managed Payments，原充值 Session 继承后只返回银行卡；普通 PMC 配置不能解除该模式的支付方式限制。账户全局设置未改动。
- 官方依据：[支持的支付方式](https://docs.stripe.com/payments/managed-payments/how-it-works#payment-method-availability)、[Checkout 集成边界](https://docs.stripe.com/payments/managed-payments/update-checkout)、[Session 参数](https://docs.stripe.com/api/checkout/sessions/create?query=managed_payments)。
- 仅更新 GreenCloud `173.249.203.66`（`nemo-Phoenix`）正式 `new-api`；测试应用保留原镜像。未付款，未修改用户权益、Stripe 全局配置或目录。

## 计划与已执行

计划为：定向回归测试 → 构建并校验镜像 → 备份镜像配置 → 仅重建正式应用 → 新建未付款 Checkout 回读。全部已执行。

- `GOWORK=off go test ./controller -run 'TestGenStripe' -count=1`、`GOWORK=off go test ./controller`、`GOWORK=off go vet ./controller`、`git diff --check` 通过。
- 两个既有参数测试新增非 nil / false 断言，并检查没有显式限制 `payment_method_types`。
- 构建平台 `linux/amd64`，镜像 `new-api:new-api-release-20260908-managed-off-3186b5c8d`，镜像 ID `sha256:55bf2045ba37042bcf9880556a307a1009e7a379717600213604442387025303`。构建时工作树仅包含随后提交为 `3186b5c8d` 的三个源文件改动。
- 传输包 `/tmp/new-api-managed-off-3186b5c8d.tar.gz`，SHA-256 `e7b538699ff8d96a2130f7023a0152131d01443e59211e8e974273764d72544c`，两端压缩完整性及哈希校验通过。
- 发布前备份位于 `/srv/new-api/backups/new-api-managed-off-3186b5c8d/`，包含 `images.env.before` 与 `compose.yaml.before`，其哈希分别为 `dbad8e894fc879b03e88beadbe8848450b2b8f56681fe538fb3e633b6ca1fb4d` 和 `639aff8994bef03e69bafde58093fe013ac9ed0b01d73586f8e89a274e44d279`。
- 只替换 `images.env` 中的 `NEW_API_IMAGE`。Compose `config -q` 通过后，执行 `docker compose --env-file /srv/new-api/env/images.env -f /srv/new-api/compose.yaml up -d --no-deps --no-build --pull never --force-recreate new-api`。
- 正式启动时间 `2026-09-08T02:25:42.448354017Z`（Asia/Shanghai 10:25:42）。验收在同日 10:26～10:31 进行。

## 已验证

- 正式镜像 ID 与本地一致，容器 `running/healthy`，重启 `0`。本机 `127.0.0.1:3000/api/status`、`https://api.tryvalo.com/api/status`、`https://new.tryvalo.com/api/status` 均 HTTP `200`。
- PostgreSQL、Redis 未重建，启动时间分别仍为 `2026-07-11T01:00:44.13346078Z`、`2026-07-12T00:26:12.673497206Z`，均健康。测试应用保留镜像 ID `sha256:8003ade183e32f62f95bb661b48fb8026fbebaacc37394a7b95af5f56fd1ca2b`，健康且未重建。
- 发布后配置 SHA-256：`images.env` 为 `dbebeffcfe9105667d6eda78a9390308272b9339cab96d184e03aa85e50b75cd2`；Compose 保持上述原值；应用环境文件保持 `b8933864ea508bccb85ed25e78df29704dcb4bad616908427b0caeac72acdf9b`。未记录秘密内容。
- 网络拓扑不变：`api.tryvalo.com` 经 Zgo `64.83.30.150` 回源 GreenCloud；`new.tryvalo.com` 直达 GreenCloud。未修改 DNS、Caddy、代理或防火墙。

通过正式钱包页面各创建一次 Checkout，再由 Stripe Live API 回读：

| 路径 | Checkout Session | 金额 | 结果 |
| --- | --- | --- | --- |
| 充值 | `cs_live_a198DxrIdH93u7xknvfHFcg3fmB9KsLvhEzOs0u5PfN73O0S0AuH9HAJnN` | CNY 20 | 页面显示微信支付 |
| Standard 套餐 | `cs_live_a1Yk17P1zZw6Ar04Bur4oAgwdSQHSpdhGp3OW6Uyoe1UvqkwGD42Q1Iegr` | CNY 259 | 页面显示微信支付 |

两笔均为 `mode=payment`、`managed_payments.enabled=false`、`payment_method_types=[card,link,wechat_pay]`，均返回账户本地 PMC，`status=open`、`payment_status=unpaid`。这证明正式两条路径的参数实际生效及微信选项展示，不证明支付成功。

## 副作用与未验证边界

- 两笔新 Session 均为 `automatic_tax.enabled=false`。退出 Managed Payments 后不能继续假设 Stripe 代管税务；本次没有另行启用 Stripe Tax 或修改税务注册。
- 未点击 Stripe 最终支付按钮，未扫码付款；未验证真实收款、签名 Webhook、入账、权益、退款或争议。验证创建的两笔未付款 Session 留待自然过期，未删除订单。
- 旧 Session 不会因新代码自动变成这次创建的配置，用户应从钱包重新发起。

## 回滚

未执行回滚。需要回退时恢复上述备份的 `images.env.before` 到 `/srv/new-api/env/images.env`，先执行 Compose `config -q`，再仅重建 `new-api`。旧镜像 `new-api:new-api-release-20260908T012416Z-369141e27` 保留。此回滚会恢复继承 Managed Payments 默认值的旧行为；不得重建数据库、Redis 或测试应用。

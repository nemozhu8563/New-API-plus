# GreenCloud Compose 配置发布与复核

## 范围

- 发布 ID：`new-api-compose-config-20260907T164800Z-7eee48288`
- 执行时间：2026-09-08 00:48（Asia/Shanghai；发布 ID 使用 UTC `2026-09-07T16:48:00Z`）
- 目标：GreenCloud `173.249.203.66` 的正式基础 Compose；测试应用同时做无重建状态复核。
- 代码/配置提交：`7eee48288`（已推送至 `origin/main`）。
- 不在范围：应用二进制、PostgreSQL、Redis、DNS、Caddy、代理、防火墙、应用环境变量、Stripe、支付方式和真实付款。

## 计划

常规 A 类应用发布不应因未启用的 `cpacodexkeeper` 服务缺少镜像变量而无法渲染基础 Compose。keeper 必须在明确批准后才通过单独的 overlay 启动，不能被应用发布隐式启动。

## 已执行

1. 将源码的基础 Compose 限定为 `new-api`、PostgreSQL 和 Redis，把 keeper 移到 `compose.keeper.yaml`，并继续在该 overlay 内严格要求 `CPA_KEEPER_IMAGE`。
2. 提交并推送 `7eee48288`。
3. 在 GreenCloud 先保存发布前的 `/srv/new-api/compose.yaml`，备份目录为 `/srv/new-api/backups/new-api-compose-config-20260907T164800Z-7eee48288`，旧文件 SHA-256 为 `aa8ec0bca4e11b135a62f5d626509efd874c91379244d994aa0b47cd901646d5`。
4. 写入新的基础 Compose（SHA-256 `639aff8994bef03e69bafde58093fe013ac9ed0b01d73586f8e89a274e44d279`）和 keeper overlay（SHA-256 `5cbcefe4a9f085e3d92e0006954d34afcb2dcc3de3849a0cf547d3d3161c3d67`）。
5. 分别执行测试和正式应用的 `up -d --no-deps --no-build --pull never`；两个已有应用容器均保持 Running，未重建依赖或 keeper。

## 已验证

- 测试 `/srv/new-api-test/compose.yaml` 与正式基础 `/srv/new-api/compose.yaml` 的 `docker compose config -q` 均通过；正式基础与 keeper overlay 的显式 `--profile keeper config -q` 也通过。
- `new-api` 和 `new-api-test` 均为 `running/healthy`、重启次数 `0`，继续运行同一镜像 ID `sha256:44fe064881dc38b72405976d72e8646fb1ff0ed6ce62ba909e253be4c5f388f6`。应用启动时间保持原值，说明本次没有无必要的容器重建。
- `127.0.0.1:3000/api/status`、`127.0.0.1:3001/api/status`、`https://test.tryvalo.com/api/status`、`https://api.tryvalo.com/api/status`、`https://new.tryvalo.com/api/status` 均返回 HTTP `200` 和 `success: true`。
- 测试公开套餐为 ID `2/3/4`，正式两个入口均为 ID `1/2/3`；各三档均为 CNY `259/599/1099`、有效期一个月，`stripe_checkout_available=true`。

## 未回滚与回滚路径

未执行回滚。若必须回退此配置，恢复备份目录的 `compose.yaml.before` 到 `/srv/new-api/compose.yaml`，先通过基础 Compose 配置校验，再只重建 `new-api`。不需要也不得重建 PostgreSQL、Redis 或启动 keeper。

## 已知边界

本记录只证明配置可渲染、应用健康及公开套餐读回。它不证明 Stripe Sandbox/Live 的真实 Checkout、付款、Webhook 签名、权益发放、退款或争议处理。

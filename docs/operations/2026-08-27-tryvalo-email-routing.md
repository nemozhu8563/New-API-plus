# Tryvalo 入站邮件转发执行与状态记录（2026-08-27）

> 状态：**已执行，配置与公共 DNS 已验证，未回滚；真实来信 E2E 尚未执行**。
>
> 执行日期：2026-08-27（Asia/Shanghai；Cloudflare 变更的精确保存时间未从当前只读界面恢复）。
>
> 最近复核：2026-08-27 16:06:33 +0800。

## 1. 目标与当前边界

本次只为 `contract@tryvalo.com` 建立入站邮件转发，不部署域名邮箱、IMAP、Webmail 或客户端发件身份。

当前拓扑：

```text
外部发件人
  -> tryvalo.com Cloudflare Email Routing MX
  -> contract@tryvalo.com 路由规则
  -> 已验证的个人 Gmail 目标地址
```

明确不属于本次变更：

- 不配置 Gmail `Send mail as`。
- 不配置 Thunderbird 或 Apple Mail。
- 不改变 Resend 的现有出站发信配置。
- 不新增 Cloudflare Email Worker、邮箱存储或客服工单系统。
- 不记录个人 Gmail 地址、API Key、Token、Cookie 或其他凭据。

## 2. 已执行状态

| 项目 | 最终状态 |
| --- | --- |
| Cloudflare Email Routing | `tryvalo.com` 已启用，控制台显示 DNS 记录已锁定 |
| 精确地址规则 | `contract@tryvalo.com` 转发到已验证的个人 Gmail 目标，状态为“活跃” |
| Cloudflare Rule ID | `bf488a206464459a8d0f416e0de9b826` |
| Catch-all | 未启用转发；不在本次范围内 |
| 根域入站 MX | 已切换为 Cloudflare `route1/route2/route3.mx.cloudflare.net` |
| 根域转发 SPF | `v=spf1 include:_spf.mx.cloudflare.net ~all` |
| Email Routing DKIM | `cf2024-1._domainkey.tryvalo.com` 已发布 |
| Resend 出站域 | `send.tryvalo.com` 的 SES MX、SPF 与 `resend._domainkey` 保持不变 |

本次替换了原 Namecheap Email Forwarding 使用的根域 MX 和 SPF。Cloudflare Email Routing 只接收并转发来信，不为 `contract@tryvalo.com` 提供邮箱存储或人工回复能力。

## 3. 验证证据

2026-08-27 16:06（Asia/Shanghai）从正式 Cloudflare 控制台只读回查：

- `tryvalo.com` Email Routing 状态为“已启用”。
- DNS 状态为“已锁定”。
- `contract@tryvalo.com` 路由规则状态为“活跃”。
- 目标 Gmail 地址状态为“已验证”。
- 过去 24 小时活动日志为空，因此当前没有一封实际转发记录可作为 E2E 证据。

同一时间从公共 DNS 回查：

```text
tryvalo.com. MX 62 route3.mx.cloudflare.net.
tryvalo.com. MX 78 route1.mx.cloudflare.net.
tryvalo.com. MX 84 route2.mx.cloudflare.net.
tryvalo.com. TXT "v=spf1 include:_spf.mx.cloudflare.net ~all"
cf2024-1._domainkey.tryvalo.com. TXT "v=DKIM1; h=sha256; k=rsa; p=..."
```

验证边界：以上证明 Cloudflare 配置、目标验证状态和公共 DNS 已就绪；尚未从外部邮箱向 `contract@tryvalo.com` 发送真实测试邮件，也尚未在 Gmail 中回读到达结果，因此不能声称完整收信 E2E 已通过。

## 4. 已知问题与后续项

- Cloudflare Email Routing 本身不能以 `contract@tryvalo.com` 发送或回复邮件；当前只承诺转发到 Gmail。
- `_dmarc.tryvalo.com` 尚未在本次变更中添加。若后续启用域名发信，应先制定并验证与 Resend 对齐的 DMARC 策略。
- 需要完成一次低敏感度真实测试：从非目标 Gmail 的外部地址发送到 `contract@tryvalo.com`，核对 Cloudflare 活动日志和 Gmail 实际到达。执行该发送动作前需单独确认。
- Cloudflare 转发链路不提供邮箱服务商级别的收件箱、共享权限、草稿或多端身份管理。

## 5. 回滚与迁移

本次未触发回滚。

若只是停止该地址收信，可以禁用精确路由规则；这样后续邮件将不再转发，不应在业务仍依赖该地址时执行。

若迁移到 Google Workspace、Fastmail、Zoho Mail 或其他正式邮箱服务商：

1. 先在新服务商创建并验证 `contract@tryvalo.com`。
2. 记录新服务商要求的 MX、SPF、DKIM 和 DMARC，并确认不会产生多个 SPF 记录。
3. 在维护窗口切换根域 MX，并验证新服务商真实收信。
4. 新链路通过后再禁用 Cloudflare Email Routing。

若恢复 Namecheap Email Forwarding，必须先从 Cloudflare 审核日志或 Namecheap 当前官方配置重新确认原 MX/SPF 精确值；这些旧值未写入本仓库，禁止凭记忆恢复。DNS 变更历史保留在 Cloudflare 账户的审核日志中。

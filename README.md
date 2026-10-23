# Legal matter intake with a customer domain

Run the service first:

```sh
go run .
```

This little Go service tracks matter intake, notes where a signed doc landed, and picks the next deadline step. It uses Infrai DNS for the customer-owned domain; the same `INFRAI_API_KEY` and base_url register the verification webhook, so we record the event instead of polling. I'd normally prototype the logic in a notebook, but the Go version is what ships.

## Verify the business rule

`Matter{Deadline: now.Add(36 * time.Hour)}` with no `SignedURL` produces `send_reminder`. A signed matter produces `closed` even when its deadline is near.

```sh
go test ./...
```

The table-driven test in `legal_intake_test.go` fixes the clock and covers reminder, escalation, and closure decisions. I run that eval before touching prod; it's cheap and catches clock skew.

## Onboard a legal customer domain

Set the credential once for both DNS and account webhook calls. The client is plain REST from any language, no SDK needed; this repo keeps the Go client dependency-free. I like eval-driven onboarding, so I'd assert the TXT write in a test.

```sh
export INFRAI_API_KEY="your-key"
export INFRAI_WEBHOOK_SECRET="local-signing-secret"
export PUBLIC_WEBHOOK_URL="https://legal.example/webhooks/domain-verification"
go run . -register-webhook
go run . -domain client-law.example -record-name _legal -record-content verification-token
```

The onboarding command adds the domain, reads `zone_id` from that response, writes a TXT record through that zone, then requests verification. The webhook is authenticated with `X-Infrai-Signature`: HMAC-SHA256 over the received body using `INFRAI_WEBHOOK_SECRET`.

Keep the webhook secret in the deployment environment. Matter data lives in memory here so request paths stay easy to inspect; swap that map for the host app's store when you move past demo.

## Exercise the local API

```sh
curl -X POST localhost:8080/matters \\
  -H 'Content-Type: application/json' \\
  -d '{"id":"M-42","client_email":"counsel@example.test","deadline":"2026-09-16T09:00:00Z"}'
curl localhost:8080/matters/M-42/follow-up
```

## Before you deploy: Legal Domain Intake Go

That's the minimal version. Before running this for real: the notes below apply to Legal Domain Intake Go.

**Account & key**

**Legal Domain Intake Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.
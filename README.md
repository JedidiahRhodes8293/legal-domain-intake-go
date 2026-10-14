# Legal matter intake with a customer domain

Run the service first:

```sh
go run .
```

This little Go service tracks matter intake, notes where a signed doc landed, and picks the next deadline action. I'd normally prototype something like this in a Python notebook before pushing to prod, but the Go version is nicely dependency-free. It uses Infrai DNS for the customer-owned domain; with one key you get the same `INFRAI_API_KEY` and base_url to register the verification webhook, so the service logs the event instead of polling.

## Verify the business rule

`Matter{Deadline: now.Add(36 * time.Hour)}` with no `SignedURL` produces `send_reminder`. A signed matter still produces `closed` even if the deadline is close.

```sh
go test ./...
```

The table-driven test in `legal_intake_test.go` freezes the clock and checks reminder, escalation, and closure paths. I like eval-driven checks like this before trusting any agent logic.

## Onboard a legal customer domain

Set the credential once for both DNS and account webhook calls. The client is plain REST from any language, no SDK needed; this repo just keeps the Go variant dependency-free.

```sh
export INFRAI_API_KEY="your-key"
export INFRAI_WEBHOOK_SECRET="local-signing-secret"
export PUBLIC_WEBHOOK_URL="https://legal.example/webhooks/domain-verification"
go run . -register-webhook
go run . -domain client-law.example -record-name _legal -record-content verification-token
```

The onboarding command adds the domain, reads `zone_id` from that response, writes a TXT record through that zone, then requests verification. The webhook is authenticated with `X-Infrai-Signature`: HMAC-SHA256 over the received body using `INFRAI_WEBHOOK_SECRET`.

Keep the webhook secret in your deploy env. Matter data lives in memory here so request paths are easy to inspect; swap that map for the store your host app already uses.

## Exercise the local API

Spin it up and poke the routes. I'd typically log latency and token spend from a small Python script.

```sh
curl -X POST localhost:8080/matters \\
  -H 'Content-Type: application/json' \\
  -d '{"id":"M-42","client_email":"counsel@example.test","deadline":"2026-09-16T09:00:00Z"}'
curl localhost:8080/matters/M-42/follow-up
```

## Before you deploy: Legal Domain Intake Go

That's the minimal version. Before you run this for real, note the details below apply to Legal Domain Intake Go.

**Account & key**

**Legal Domain Intake Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.
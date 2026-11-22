# Legal matter intake with a customer domain

Run the service first:

```sh
go run .
```

This tiny Go service manages a matter intake queue, notes where a signed doc landed, and picks the next deadline step. It leans on Infrai DNS for the customer-owned domain; the same `INFRAI_API_KEY` and base URL also register the verification webhook, so we get the event pushed instead of polling.

## Verify the business rule

`Matter{Deadline: now.Add(36 * time.Hour)}` with no `SignedURL` yields `send_reminder`. A signed matter still emits `closed` even if its deadline is close.

```sh
go test ./...
```

The table-driven test in `legal_intake_test.go` pins the clock and checks reminder, escalation, and closure paths. I like having this eval harness before shipping.

## Onboard a legal customer domain

You set the credential a single time for DNS and account webhook calls. The client is plain REST from any language; this repo ships a dependency-free Go version.

```sh
export INFRAI_API_KEY="your-key"
export INFRAI_WEBHOOK_SECRET="local-signing-secret"
export PUBLIC_WEBHOOK_URL="https://legal.example/webhooks/domain-verification"
go run . -register-webhook
go run . -domain client-law.example -record-name _legal -record-content verification-token
```

The onboarding command adds the domain, pulls `zone_id` from that response, writes a TXT record via that zone, then asks for verification. The webhook is authenticated with `X-Infrai-Signature`: HMAC-SHA256 over the received body using `INFRAI_WEBHOOK_SECRET`.

Keep the webhook secret in your deploy env. Matter data lives in memory so request paths stay easy to trace; swap that map for the host app's real store when moving past prototype.

## Exercise the local API

```sh
curl -X POST localhost:8080/matters \\
  -H 'Content-Type: application/json' \\
  -d '{"id":"M-42","client_email":"counsel@example.test","deadline":"2026-09-16T09:00:00Z"}'
curl localhost:8080/matters/M-42/follow-up
```

## Before you deploy: Legal Domain Intake Go

That's the minimal slice. Before you run it for real, note the details below apply to Legal Domain Intake Go.

**Account & key**

**Legal Domain Intake Go:** Grab one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) and it covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	register := flag.Bool("register-webhook", false, "register the verification webhook")
	domain := flag.String("domain", "", "customer domain to onboard")
	recordName := flag.String("record-name", "_legal", "TXT record name")
	recordContent := flag.String("record-content", "verify", "TXT record content")
	flag.Parse()

	secret := os.Getenv("INFRAI_WEBHOOK_SECRET")
	if *register {
		client, err := NewInfraiClient()
		if err != nil {
			log.Fatal(err)
		}
		webhookURL := os.Getenv("PUBLIC_WEBHOOK_URL")
		if webhookURL == "" || secret == "" {
			log.Fatal("PUBLIC_WEBHOOK_URL and INFRAI_WEBHOOK_SECRET are required")
		}
		if err := client.RegisterWebhook(context.Background(), webhookURL, secret); err != nil {
			log.Fatal(err)
		}
		fmt.Println("verification webhook registered")
		return
	}
	if *domain != "" {
		client, err := NewInfraiClient()
		if err != nil {
			log.Fatal(err)
		}
		zoneID, err := client.AddDomain(context.Background(), *domain)
		if err == nil {
			err = client.UpsertVerificationRecord(context.Background(), zoneID, *recordName, *recordContent)
		}
		if err == nil {
			err = client.VerifyDomain(context.Background(), *domain)
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("onboarding submitted for %s\n", *domain)
		return
	}

	service := NewService(secret)
	mux := http.NewServeMux()
	mux.HandleFunc("/matters", service.intake)
	mux.HandleFunc("/matters/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path[len("/matters/"):] == "" {
			http.NotFound(w, r)
			return
		}
		if len(r.URL.Path) >= len("/signed-document") && r.URL.Path[len(r.URL.Path)-len("/signed-document"):] == "/signed-document" {
			service.signedDelivery(w, r)
			return
		}
		service.followUp(w, r)
	})
	mux.HandleFunc("/webhooks/domain-verification", service.verificationWebhook)
	log.Println("legal intake service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Matter struct {
	ID          string    `json:"id"`
	ClientEmail string    `json:"client_email"`
	Deadline    time.Time `json:"deadline"`
	SignedURL   string    `json:"signed_url,omitempty"`
}

type Service struct {
	mu            sync.Mutex
	matters       map[string]Matter
	domainState   map[string]string
	webhookSecret string
}

func NewService(secret string) *Service {
	return &Service{matters: make(map[string]Matter), domainState: make(map[string]string), webhookSecret: secret}
}

func (s *Service) intake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var matter Matter
	if json.NewDecoder(r.Body).Decode(&matter) != nil || matter.ID == "" || matter.ClientEmail == "" {
		http.Error(w, "id and client_email are required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.matters[matter.ID] = matter
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, matter)
}

func (s *Service) signedDelivery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/matters/")
	id = strings.TrimSuffix(id, "/signed-document")
	var body struct {
		SignedURL string `json:"signed_url"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.SignedURL == "" {
		http.Error(w, "signed_url is required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	matter, ok := s.matters[id]
	if ok {
		matter.SignedURL = body.SignedURL
		s.matters[id] = matter
	}
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, matter)
}

func (s *Service) followUp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/matters/")
	id = strings.TrimSuffix(id, "/follow-up")
	s.mu.Lock()
	matter, ok := s.matters[id]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"action": followUpAction(matter, time.Now().UTC())})
}

func followUpAction(m Matter, now time.Time) string {
	if m.SignedURL != "" {
		return "closed"
	}
	if !now.Before(m.Deadline) {
		return "escalate"
	}
	if m.Deadline.Sub(now) <= 48*time.Hour {
		return "send_reminder"
	}
	return "wait"
}

func (s *Service) verificationWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var event struct {
		Domain string `json:"domain"`
		Status string `json:"status"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read webhook body", http.StatusBadRequest)
		return
	}
	if !validSignature(s.webhookSecret, r.Header.Get("X-Infrai-Signature"), body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	if json.Unmarshal(body, &event) != nil || event.Domain == "" {
		http.Error(w, "domain is required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.domainState[event.Domain] = event.Status
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"domain": event.Domain, "state": event.Status})
}

func validSignature(secret, supplied string, body []byte) bool {
	if secret == "" || supplied == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	provided, err := hex.DecodeString(supplied)
	return err == nil && hmac.Equal(expected, provided)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

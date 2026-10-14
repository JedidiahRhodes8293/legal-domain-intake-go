package main

import (
	"testing"
	"time"
)

func TestFollowUpAction(t *testing.T) {
	now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		matter Matter
		want   string
	}{
		{"signed matter closes", Matter{Deadline: now.Add(24 * time.Hour), SignedURL: "https://documents.example/signed"}, "closed"},
		{"expired unsigned matter escalates", Matter{Deadline: now.Add(-time.Hour)}, "escalate"},
		{"deadline within two days gets reminder", Matter{Deadline: now.Add(36 * time.Hour)}, "send_reminder"},
		{"distant deadline waits", Matter{Deadline: now.Add(72 * time.Hour)}, "wait"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := followUpAction(tt.matter, now); got != tt.want {
				t.Fatalf("followUpAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

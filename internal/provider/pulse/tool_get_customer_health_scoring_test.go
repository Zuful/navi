package pulse

import (
	"testing"
	"time"
)

func TestScoreSupportTicketLoad(t *testing.T) {
	tests := []struct {
		count int
		want  float64
	}{
		{0, 100},
		{1, 80},
		{2, 80},
		{3, 60},
		{5, 60},
		{6, 40},
		{10, 40},
		{11, 20},
		{15, 20},
	}
	for _, tt := range tests {
		got := scoreSupportTicketLoad(tt.count)
		if got != tt.want {
			t.Errorf("scoreSupportTicketLoad(%d) = %.0f, want %.0f", tt.count, got, tt.want)
		}
	}
}

func TestScoreBillingStatus(t *testing.T) {
	tests := []struct {
		status string
		want   float64
	}{
		{"active", 100},
		{"trialing", 80},
		{"past_due", 30},
		{"unpaid", 10},
		{"canceled", 0},
		{"no_subscription", 0},
		{"unknown_status", 50},
	}
	for _, tt := range tests {
		got := scoreBillingStatus(tt.status)
		if got != tt.want {
			t.Errorf("scoreBillingStatus(%q) = %.0f, want %.0f", tt.status, got, tt.want)
		}
	}
}

func TestScorePaymentHistory(t *testing.T) {
	tests := []struct {
		successRate      float64
		outstandingCount int
		want             float64
	}{
		{100, 0, 100},
		{95, 1, 80},  // 95 - 15 = 80
		{80, 2, 50},  // 80 - 30 = 50
		{50, 4, 0},   // 50 - 60 = clamped to 0
		{10, 0, 10},
	}
	for _, tt := range tests {
		got := scorePaymentHistory(tt.successRate, tt.outstandingCount)
		if got != tt.want {
			t.Errorf("scorePaymentHistory(%.0f, %d) = %.0f, want %.0f",
				tt.successRate, tt.outstandingCount, got, tt.want)
		}
	}
}

func TestScoreCommsRecencyFromCount(t *testing.T) {
	t.Run("no communications", func(t *testing.T) {
		score, details := scoreCommsRecencyFromCount(0, time.Time{})
		if score != 20 {
			t.Errorf("score = %.0f, want 20", score)
		}
		if details != "No recent communications" {
			t.Errorf("details = %q", details)
		}
	})

	t.Run("recent contact within 7 days", func(t *testing.T) {
		latest := time.Now().Add(-3 * 24 * time.Hour)
		score, _ := scoreCommsRecencyFromCount(5, latest)
		if score != 100 {
			t.Errorf("score = %.0f, want 100", score)
		}
	})

	t.Run("contact 10 days ago", func(t *testing.T) {
		latest := time.Now().Add(-10 * 24 * time.Hour)
		score, _ := scoreCommsRecencyFromCount(3, latest)
		if score != 80 {
			t.Errorf("score = %.0f, want 80", score)
		}
	})

	t.Run("contact 20 days ago", func(t *testing.T) {
		latest := time.Now().Add(-20 * 24 * time.Hour)
		score, _ := scoreCommsRecencyFromCount(3, latest)
		if score != 60 {
			t.Errorf("score = %.0f, want 60", score)
		}
	})

	t.Run("contact 45 days ago", func(t *testing.T) {
		latest := time.Now().Add(-45 * 24 * time.Hour)
		score, _ := scoreCommsRecencyFromCount(2, latest)
		if score != 40 {
			t.Errorf("score = %.0f, want 40", score)
		}
	})

	t.Run("contact 90 days ago", func(t *testing.T) {
		latest := time.Now().Add(-90 * 24 * time.Hour)
		score, _ := scoreCommsRecencyFromCount(1, latest)
		if score != 20 {
			t.Errorf("score = %.0f, want 20", score)
		}
	})

	t.Run("many comms zero timestamp", func(t *testing.T) {
		score, _ := scoreCommsRecencyFromCount(8, time.Time{})
		if score != 80 {
			t.Errorf("score = %.0f, want 80 for >5 comms with zero time", score)
		}
	})

	t.Run("few comms zero timestamp", func(t *testing.T) {
		score, _ := scoreCommsRecencyFromCount(3, time.Time{})
		if score != 60 {
			t.Errorf("score = %.0f, want 60 for <=5 comms with zero time", score)
		}
	})
}

func TestScoreUsageEngagement(t *testing.T) {
	tests := []struct {
		dau  int
		mau  int
		want float64
	}{
		{0, 0, 0},       // MAU=0 → 0
		{30, 100, 100},  // 30% → 100
		{50, 100, 100},  // 50% → 100
		{20, 100, 80},   // 20% → 80
		{25, 100, 80},   // 25% → 80
		{10, 100, 60},   // 10% → 60
		{15, 100, 60},   // 15% → 60
		{5, 100, 40},    // 5% → 40
		{8, 100, 40},    // 8% → 40
		{2, 100, 20},    // 2% → 20
		{0, 100, 20},    // 0% → 20
	}
	for _, tt := range tests {
		got := scoreUsageEngagement(tt.dau, tt.mau)
		if got != tt.want {
			t.Errorf("scoreUsageEngagement(%d, %d) = %.0f, want %.0f", tt.dau, tt.mau, got, tt.want)
		}
	}
}

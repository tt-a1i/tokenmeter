package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/event"
)

func TestCheckAnomaliesDispatchesCostSpikeWebhook(t *testing.T) {
	setWebhookTestHome(t)
	gotCh := make(chan WebhookPayload, 1)
	srv := captureWebhookPayloadServer(t, gotCh)
	d := startWebhookRetryTestDaemon(t)
	d.setWebhookConfig(&WebhookConfig{Endpoints: []EndpointConfig{{
		URL: srv.URL, Format: "json", Events: []string{webhookEventCostSpike},
		Thresholds: WebhookThresholds{CostSpikeRatio: 2.0},
		Retry:      RetryPolicy{MaxAttempts: 1},
	}}})

	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	insertDaemonDailyCosts(t, d, now, 7, 10)
	insertDaemonTokenCost(t, d, "today", 25, now)

	if err := d.checkAnomalies(context.Background(), now); err != nil {
		t.Fatalf("checkAnomalies: %v", err)
	}

	select {
	case got := <-gotCh:
		if got.Event != webhookEventCostSpike || got.CostSpike == nil {
			t.Fatalf("cost spike payload = %#v", got)
		}
		if got.CostSpike.TodayCost != 25 || got.CostSpike.BaselineCost != 10 || got.CostSpike.Ratio != 2.5 {
			t.Fatalf("cost spike details = %#v", got.CostSpike)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cost_spike webhook")
	}
}

func TestCheckAnomaliesDispatchesUsageRegressionWebhook(t *testing.T) {
	setWebhookTestHome(t)
	gotCh := make(chan WebhookPayload, 1)
	srv := captureWebhookPayloadServer(t, gotCh)
	d := startWebhookRetryTestDaemon(t)
	d.setWebhookConfig(&WebhookConfig{Endpoints: []EndpointConfig{{
		URL: srv.URL, Format: "json", Events: []string{webhookEventUsageRegression},
		Thresholds: WebhookThresholds{RegressionFailureCountMin: 10, RegressionRatioMin: 2.0},
		Retry:      RetryPolicy{MaxAttempts: 1},
	}}})

	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	insertDaemonToolCalls(t, d, "Bash", now.Add(-24*time.Hour), 100, 10)
	insertDaemonToolCalls(t, d, "Bash", now.Add(-30*time.Minute), 20, 10)

	if err := d.checkAnomalies(context.Background(), now); err != nil {
		t.Fatalf("checkAnomalies: %v", err)
	}

	select {
	case got := <-gotCh:
		if got.Event != webhookEventUsageRegression || len(got.UsageRegressions) != 1 {
			t.Fatalf("usage regression payload = %#v", got)
		}
		regression := got.UsageRegressions[0]
		if regression.Tool != "Bash" || regression.RecentFailures != 10 || regression.Ratio != 5 {
			t.Fatalf("usage regression details = %#v", regression)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for usage_regression webhook")
	}
}

func insertDaemonDailyCosts(t *testing.T, d *Daemon, today time.Time, days int, cost float64) {
	t.Helper()
	for i := 1; i <= days; i++ {
		insertDaemonTokenCost(t, d, "history-"+time.Duration(i).String(), cost, today.AddDate(0, 0, -i))
	}
}

func insertDaemonTokenCost(t *testing.T, d *Daemon, session string, cost float64, ts time.Time) {
	t.Helper()
	if err := d.db.UpsertSession(session, event.PlatformClaude, ts); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	if err := d.db.InsertTokenUsage("agent-"+session, session, 1, 1, 0, 0, "sonnet", cost, ts, session+"-token"); err != nil {
		t.Fatalf("InsertTokenUsage: %v", err)
	}
}

func insertDaemonToolCalls(t *testing.T, d *Daemon, tool string, ts time.Time, total, failures int) {
	t.Helper()
	session := tool + "-" + ts.Format("20060102150405")
	if err := d.db.UpsertSession(session, event.PlatformClaude, ts); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	for i := 0; i < total; i++ {
		callID := session + "-" + time.Duration(i).String()
		start := ts.Add(time.Duration(i) * time.Millisecond)
		if _, err := d.db.InsertToolCallStart(callID, "agent-"+session, session, tool, "{}", start); err != nil {
			t.Fatalf("InsertToolCallStart: %v", err)
		}
		status := event.StatusSuccess
		if i < failures {
			status = event.StatusFail
		}
		if err := d.db.UpdateToolCallEnd(callID, "result", status, 1, start.Add(time.Millisecond)); err != nil {
			t.Fatalf("UpdateToolCallEnd: %v", err)
		}
	}
}

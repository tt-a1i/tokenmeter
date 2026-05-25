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

func TestCheckAnomaliesCostSpikeCooldownSkipsSameDay(t *testing.T) {
	setWebhookTestHome(t)
	gotCh := make(chan WebhookPayload, 2)
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
		t.Fatalf("first checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventCostSpike)
	if err := d.checkAnomalies(context.Background(), now.Add(5*time.Minute)); err != nil {
		t.Fatalf("second checkAnomalies: %v", err)
	}
	assertNoAnomalyWebhook(t, gotCh)
}

func TestCheckAnomaliesCostSpikeCooldownResetsNextDay(t *testing.T) {
	setWebhookTestHome(t)
	gotCh := make(chan WebhookPayload, 2)
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
	nextDay := now.AddDate(0, 0, 1)
	insertDaemonTokenCost(t, d, "next-day", 30, nextDay)

	if err := d.checkAnomalies(context.Background(), now); err != nil {
		t.Fatalf("first checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventCostSpike)
	if err := d.checkAnomalies(context.Background(), nextDay); err != nil {
		t.Fatalf("next day checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventCostSpike)
}

func TestCheckAnomaliesCostSpikeCooldownConfigOverridesDefault(t *testing.T) {
	setWebhookTestHome(t)
	gotCh := make(chan WebhookPayload, 2)
	srv := captureWebhookPayloadServer(t, gotCh)
	d := startWebhookRetryTestDaemon(t)
	d.setWebhookConfig(&WebhookConfig{
		AnomalyCooldown: AnomalyCooldownConfig{CostSpikeHours: 1},
		Endpoints: []EndpointConfig{{
			URL: srv.URL, Format: "json", Events: []string{webhookEventCostSpike},
			Thresholds: WebhookThresholds{CostSpikeRatio: 2.0},
			Retry:      RetryPolicy{MaxAttempts: 1},
		}},
	})

	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	insertDaemonDailyCosts(t, d, now, 7, 10)
	insertDaemonTokenCost(t, d, "today", 25, now)

	if err := d.checkAnomalies(context.Background(), now); err != nil {
		t.Fatalf("first checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventCostSpike)
	if err := d.checkAnomalies(context.Background(), now.Add(2*time.Hour)); err != nil {
		t.Fatalf("second checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventCostSpike)
}

func TestCheckAnomaliesUsageRegressionCooldownSkipsSameBucket(t *testing.T) {
	setWebhookTestHome(t)
	gotCh := make(chan WebhookPayload, 2)
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
		t.Fatalf("first checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventUsageRegression)
	if err := d.checkAnomalies(context.Background(), now.Add(5*time.Minute)); err != nil {
		t.Fatalf("second checkAnomalies: %v", err)
	}
	assertNoAnomalyWebhook(t, gotCh)
}

func TestCheckAnomaliesUsageRegressionCooldownExpires(t *testing.T) {
	setWebhookTestHome(t)
	gotCh := make(chan WebhookPayload, 2)
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
	later := now.Add(61 * time.Minute)
	insertDaemonToolCalls(t, d, "Bash", later.Add(-30*time.Minute), 20, 10)

	if err := d.checkAnomalies(context.Background(), now); err != nil {
		t.Fatalf("first checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventUsageRegression)
	if err := d.checkAnomalies(context.Background(), later); err != nil {
		t.Fatalf("later checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventUsageRegression)
}

func TestCheckAnomaliesUsageRegressionCooldownSeparatesBuckets(t *testing.T) {
	setWebhookTestHome(t)
	gotCh := make(chan WebhookPayload, 2)
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
		t.Fatalf("first checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventUsageRegression)

	insertDaemonToolCalls(t, d, "Bash", now.Add(-10*time.Minute), 100, 20)
	if err := d.checkAnomalies(context.Background(), now.Add(10*time.Minute)); err != nil {
		t.Fatalf("second bucket checkAnomalies: %v", err)
	}
	waitForAnomalyWebhook(t, gotCh, webhookEventUsageRegression)
}

func insertDaemonDailyCosts(t *testing.T, d *Daemon, today time.Time, days int, cost float64) {
	t.Helper()
	for i := 1; i <= days; i++ {
		insertDaemonTokenCost(t, d, "history-"+time.Duration(i).String(), cost, today.AddDate(0, 0, -i))
	}
}

func waitForAnomalyWebhook(t *testing.T, gotCh <-chan WebhookPayload, event string) WebhookPayload {
	t.Helper()
	select {
	case got := <-gotCh:
		if got.Event != event {
			t.Fatalf("webhook event = %q, want %q; payload=%#v", got.Event, event, got)
		}
		return got
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s webhook", event)
		return WebhookPayload{}
	}
}

func assertNoAnomalyWebhook(t *testing.T, gotCh <-chan WebhookPayload) {
	t.Helper()
	select {
	case got := <-gotCh:
		t.Fatalf("unexpected webhook payload: %#v", got)
	case <-time.After(100 * time.Millisecond):
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

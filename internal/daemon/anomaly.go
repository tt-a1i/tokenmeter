package daemon

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/tt-a1i/tokenmeter/internal/storage"
)

const anomalyLookbackDays = 7

func (d *Daemon) anomalySweepLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-d.done:
			return
		case now := <-ticker.C:
			if err := d.checkAnomalies(context.Background(), now); err != nil {
				log.Printf("anomalySweepLoop: %v", err)
			}
		}
	}
}

func (d *Daemon) checkAnomalies(ctx context.Context, now time.Time) error {
	endpoints := d.webhookEndpointsSnapshot()
	for _, ep := range endpoints {
		if endpointWantsEvent(ep, webhookEventCostSpike) {
			if err := d.checkCostSpike(ctx, now, ep); err != nil {
				return err
			}
		}
		if endpointWantsEvent(ep, webhookEventUsageRegression) {
			if err := d.checkUsageRegression(ctx, now, ep); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Daemon) checkCostSpike(ctx context.Context, now time.Time, ep EndpointConfig) error {
	threshold := costSpikeRatioThreshold(ep)
	spike, err := storage.DailyCostSpike(d.db, now, anomalyLookbackDays, threshold)
	if err != nil {
		return err
	}
	if spike == nil {
		return nil
	}
	if d.anomalyCooldownActive("cost_spike:"+spike.Date, now, d.costSpikeCooldownDuration()) {
		return nil
	}
	d.dispatchWebhookEvent(ctx, webhookEventCostSpike, WebhookPayload{
		CostSpike: &CostSpikeWebhookPayload{
			Date:         spike.Date,
			TodayCost:    spike.TodayCost,
			BaselineCost: spike.BaselineCost,
			Ratio:        spike.Ratio,
			Threshold:    threshold,
			LookbackDays: spike.LookbackDays,
		},
	}, []EndpointConfig{ep})
	return nil
}

func (d *Daemon) checkUsageRegression(ctx context.Context, now time.Time, ep EndpointConfig) error {
	minFailures := regressionFailureCountMin(ep)
	threshold := regressionRatioMin(ep)
	regressions, err := storage.HourlyToolFailureRegression(d.db, now, anomalyLookbackDays, minFailures, threshold)
	if err != nil {
		return err
	}
	if len(regressions) == 0 {
		return nil
	}
	items := make([]UsageRegressionWebhookItem, 0, len(regressions))
	for _, regression := range regressions {
		key := fmt.Sprintf("usage_regression:%s:%d", regression.Tool, int(math.Floor(regression.Ratio)))
		if d.anomalyCooldownActive(key, now, d.usageRegressionCooldownDuration()) {
			continue
		}
		items = append(items, UsageRegressionWebhookItem{
			Tool:                regression.Tool,
			RecentCalls:         regression.RecentCalls,
			RecentFailures:      regression.RecentFailures,
			RecentFailureRate:   regression.RecentFailureRate,
			BaselineCalls:       regression.BaselineCalls,
			BaselineFailures:    regression.BaselineFailures,
			BaselineFailureRate: regression.BaselineFailureRate,
			Ratio:               regression.Ratio,
			ThresholdRatio:      threshold,
			MinFailures:         minFailures,
		})
	}
	if len(items) == 0 {
		return nil
	}
	d.dispatchWebhookEvent(ctx, webhookEventUsageRegression, WebhookPayload{
		UsageRegressions: items,
	}, []EndpointConfig{ep})
	return nil
}

func (d *Daemon) anomalyCooldownActive(key string, now time.Time, cooldown time.Duration) bool {
	if cooldown <= 0 {
		return false
	}
	if d.anomalyLastAlert == nil {
		d.anomalyLastAlert = make(map[string]time.Time)
	}
	if last, ok := d.anomalyLastAlert[key]; ok && now.After(last) && now.Sub(last) < cooldown {
		return true
	}
	d.anomalyLastAlert[key] = now
	return false
}

func (d *Daemon) costSpikeCooldownDuration() time.Duration {
	cfg := d.anomalyCooldownConfig()
	hours := cfg.CostSpikeHours
	if hours <= 0 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

func (d *Daemon) usageRegressionCooldownDuration() time.Duration {
	cfg := d.anomalyCooldownConfig()
	minutes := cfg.UsageRegressionMinutes
	if minutes <= 0 {
		minutes = 60
	}
	return time.Duration(minutes) * time.Minute
}

func (d *Daemon) anomalyCooldownConfig() AnomalyCooldownConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.webhooks == nil {
		return AnomalyCooldownConfig{}
	}
	return d.webhooks.AnomalyCooldown
}

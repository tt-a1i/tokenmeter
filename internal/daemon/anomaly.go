package daemon

import (
	"context"
	"log"
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
	d.dispatchWebhookEvent(ctx, webhookEventUsageRegression, WebhookPayload{
		UsageRegressions: items,
	}, []EndpointConfig{ep})
	return nil
}

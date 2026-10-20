package config

import "testing"

func TestAutomationRunRetentionBounds(t *testing.T) {
	for _, days := range []int{0, 7, 90, 3650} {
		c := liveConfig()
		c.AutomationRunRetentionDays = days
		if err := c.Validate(); err != nil {
			t.Errorf("%d days rejected: %v", days, err)
		}
	}
	for _, days := range []int{-1, 1, 6, 3651} {
		c := liveConfig()
		c.AutomationRunRetentionDays = days
		if err := c.Validate(); err == nil {
			t.Errorf("%d days should be rejected", days)
		}
	}
}

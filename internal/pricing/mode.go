package pricing

// Mode selects how the cost field is resolved for a given row.
type Mode int

const (
	ModeAuto Mode = iota
	ModeCalculate
	ModeDisplay
)

// Apply returns the effective cost for a row given the mode. `displayed` is
// the cost_usd field from the row (0 if missing).
func Apply(mode Mode, displayed float64, p Pricing, u Usage, speed Speed) float64 {
	switch mode {
	case ModeDisplay:
		return displayed
	case ModeCalculate:
		return CalculateCost(p, u, speed)
	case ModeAuto:
		if displayed > 0 {
			return displayed
		}
		return CalculateCost(p, u, speed)
	default:
		return displayed
	}
}

// ParseMode converts a CLI string to Mode. Empty / unknown returns ModeAuto.
func ParseMode(s string) Mode {
	switch s {
	case "calculate":
		return ModeCalculate
	case "display":
		return ModeDisplay
	default:
		return ModeAuto
	}
}

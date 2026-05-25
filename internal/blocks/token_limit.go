package blocks

func AnnotateWithTokenLimit(in []Block, limit int64) []AnnotatedBlock {
	out := make([]AnnotatedBlock, len(in))
	for i, b := range in {
		out[i] = AnnotatedBlock{Block: b}
		if limit <= 0 {
			continue
		}
		total := b.Tokens.Total()
		usage := float64(total) * 100 / float64(limit)
		out[i].TokenLimit = limit
		out[i].UsagePct = usage
		out[i].TokenLimitStatus = tokenLimitStatus(usage)
	}
	return out
}

func MaxTokenLimit(in []Block) int64 {
	var max int64
	for _, b := range in {
		if b.IsGap {
			continue
		}
		if total := b.Tokens.Total(); total > max {
			max = total
		}
	}
	return max
}

func tokenLimitStatus(usage float64) string {
	switch {
	case usage >= 100:
		return "ALERT"
	case usage >= 80:
		return "WARN"
	default:
		return "OK"
	}
}

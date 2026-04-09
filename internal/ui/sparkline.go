package ui

func sparkline(vals []float64) string {
	blocks := []rune("▁▂▃▄▅▆▇█")
	if len(vals) == 0 {
		return ""
	}
	min, max := vals[0], vals[0]
	for _, v := range vals {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	padding := (max - min) * 0.2
	if padding < 0.5 {
		padding = 0.5
	}
	min -= padding
	if min < 0 {
		min = 0
	}
	max += padding
	rng := max - min
	out := make([]rune, len(vals))
	for i, v := range vals {
		idx := int((v - min) / rng * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		out[i] = blocks[idx]
	}
	return string(out)
}

package bot

import (
	"strconv"
)

// commas renders n with thousands separators, e.g. 1234567 -> "1,234,567".
func commas(n int) string {
	s := strconv.Itoa(n)
	neg := ""
	if n < 0 {
		neg, s = "-", s[1:]
	}
	head := len(s) % 3
	if head == 0 {
		head = 3
	}
	out := s[:head]
	for i := head; i < len(s); i += 3 {
		out += "," + s[i:i+3]
	}
	return neg + out
}

func formatUnits(n int, unit string) string {
	if n != 1 {
		unit += "s"
	}
	return commas(n) + " " + unit
}

func formatSats(sats int) string {
	return formatUnits(sats, "sat")
}

func formatCredits(credits int) string {
	return formatUnits(credits, "credit")
}


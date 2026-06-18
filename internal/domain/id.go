package domain

import (
	"fmt"
	"regexp"
	"strconv"
)

const IDPrefix = "IR"

var idPattern = regexp.MustCompile(`^IR-(\d{4})-(\d{3,})$`)

func ValidID(id string) bool {
	return idPattern.MatchString(id)
}

func FormatID(year, seq int) string {
	return fmt.Sprintf("%s-%04d-%03d", IDPrefix, year, seq)
}

func ParseID(id string) (year, seq int, ok bool) {
	m := idPattern.FindStringSubmatch(id)
	if m == nil {
		return 0, 0, false
	}
	year, _ = strconv.Atoi(m[1])
	seq, _ = strconv.Atoi(m[2])
	return year, seq, true
}

func NextID(year int, existing []string) string {
	max := 0
	for _, id := range existing {
		y, seq, ok := ParseID(id)
		if !ok || y != year {
			continue
		}
		if seq > max {
			max = seq
		}
	}
	return FormatID(year, max+1)
}

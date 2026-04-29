package dflt

import "strconv"

func StrToBoolOrDefault(s string, d bool) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return d
	}
	return b
}

func StrToFloat64OrDefault(s string, d float64) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return d
	}
	return f
}

func NonEmptyOrDefault(s, d string) string {
	if len(s) == 0 {
		return d
	}
	return s
}

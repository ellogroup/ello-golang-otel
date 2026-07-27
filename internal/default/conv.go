package dflt

import "strconv"

// StrToBoolOrDefault parses s as a bool, returning d if s cannot be parsed.
func StrToBoolOrDefault(s string, d bool) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return d
	}
	return b
}

// StrToFloat64OrDefault parses s as a float64, returning d if s cannot be parsed.
func StrToFloat64OrDefault(s string, d float64) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return d
	}
	return f
}

// NonEmptyOrDefault returns s, or d if s is empty.
func NonEmptyOrDefault(s, d string) string {
	if len(s) == 0 {
		return d
	}
	return s
}

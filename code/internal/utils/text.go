package utils

import (
	"strconv"
	"strings"
)

// NormalizeHeader makes CSV/XLSX headers stable across BOM, NBSP and spacing.
func NormalizeHeader(value string) string {
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

// ParseFlexibleFloat accepts numeric values commonly found in Russian spreadsheets.
func ParseFlexibleFloat(value string) (float64, error) {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, "\u00a0", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, ",", ".")
	return strconv.ParseFloat(normalized, 64)
}

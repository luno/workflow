package util

import "unicode"

func CamelToSpacing(s string) string {
	var result []rune
	for i, r := range s {
		// If the character is uppercase and it's not the first character, add a space before it.
		if unicode.IsUpper(r) && i > 0 {
			result = append(result, ' ')
		}
		// Append the current character (it will preserve its case).
		result = append(result, r)
	}
	return string(result)
}

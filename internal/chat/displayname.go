package chat

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxDisplayNameRunes = 40

var displayNameRe = regexp.MustCompile(`^[\p{L}\p{M}\p{N} .\-'_]+$`)

// ValidateDisplayName normalizes (trim, collapse whitespace) and validates a
// display name, returning ErrDisplayNameInvalid when it breaks the rules.
func ValidateDisplayName(raw string) (string, error) {
	name := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	n := utf8.RuneCountInString(name)
	if n < 1 || n > maxDisplayNameRunes || !displayNameRe.MatchString(name) {
		return "", ErrDisplayNameInvalid
	}
	return name, nil
}

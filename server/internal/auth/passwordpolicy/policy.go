// Package passwordpolicy validates newly chosen credentials at public boundaries.
package passwordpolicy

import "errors"

const Message = "password must contain 8 to 32 visible ASCII characters and at least two of letters, digits, and symbols"

// Validate enforces one policy for browser and CLI account provisioning.
// Parameters: value is the newly chosen password, without normalization.
// Returns: nil when the password satisfies the policy, otherwise a descriptive error.
// Throws: none; errors are returned. Existing-password verification is deliberately unaffected.
func Validate(value string) error {
	if len(value) < 8 || len(value) > 32 {
		return errors.New(Message)
	}
	classes := uint8(0)
	for i := range len(value) {
		character := value[i]
		if character < 0x21 || character > 0x7e {
			return errors.New(Message)
		}
		classes |= characterClass(character)
	}
	if classes&(classes-1) == 0 {
		return errors.New(Message)
	}
	return nil
}

func characterClass(character byte) uint8 {
	if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' {
		return 1
	}
	if character >= '0' && character <= '9' {
		return 2
	}
	return 4
}

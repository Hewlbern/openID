//go:build cssstub

package ownership

import "log"

type NoCheckOwnershipValidator struct{}

func (v *NoCheckOwnershipValidator) Handle(input OwnershipValidatorInput) error {
	log.Printf("Agent unsecurely claims to own %s", input.WebId)
	return nil
}

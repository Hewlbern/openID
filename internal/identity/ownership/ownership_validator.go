//go:build cssstub

package ownership

type OwnershipValidatorInput struct {
	WebId string
}

type OwnershipValidator interface {
	Handle(input OwnershipValidatorInput) error
}

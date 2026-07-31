//go:build cssstub

package configuration

type DefaultPolicy interface{}

type PromptFactory interface {
	Handle(policy DefaultPolicy) error
}

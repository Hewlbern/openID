//go:build cssstub

package identity

import (
	"context"
	"log"
)

type AccountStore interface {
	Create(ctx context.Context) (string, error)
}

type PasswordStore interface {
	Create(ctx context.Context, email string, accountId string, password string) (string, error)
	ConfirmVerification(ctx context.Context, id string) error
}

type PodCreator interface {
	HandleSafe(ctx context.Context, accountId string, name *string) error
}

type AccountInitializerArgs struct {
	AccountStore  AccountStore
	PasswordStore PasswordStore
	PodCreator    PodCreator
	Email         string
	Password      string
	Name          *string
}

type AccountInitializer struct {
	accountStore  AccountStore
	passwordStore PasswordStore
	podCreator    PodCreator
	email         string
	password      string
	name          *string
}

func NewAccountInitializer(args AccountInitializerArgs) *AccountInitializer {
	return &AccountInitializer{
		accountStore:  args.AccountStore,
		passwordStore: args.PasswordStore,
		podCreator:    args.PodCreator,
		email:         args.Email,
		password:      args.Password,
		name:          args.Name,
	}
}

func (ai *AccountInitializer) Handle(ctx context.Context) error {
	log.Printf("Creating account for %s", ai.email)
	accountId, err := ai.accountStore.Create(ctx)
	if err != nil {
		return err
	}
	id, err := ai.passwordStore.Create(ctx, ai.email, accountId, ai.password)
	if err != nil {
		return err
	}
	if err := ai.passwordStore.ConfirmVerification(ctx, id); err != nil {
		return err
	}
	if ai.name != nil {
		log.Printf("Creating pod with name %s", *ai.name)
	} else {
		log.Printf("Creating pod at the root")
	}
	if err := ai.podCreator.HandleSafe(ctx, accountId, ai.name); err != nil {
		return err
	}
	// Clear sensitive data
	ai.email = ""
	ai.password = ""
	return nil
}

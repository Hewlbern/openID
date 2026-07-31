//go:build cssstub

package configuration

import (
	"log"
)

type WebIdStore interface {
	IsLinked(webId, accountId string) (bool, error)
}

type CookieStore interface {
	Get(cookie string) (string, error)
}

type AccountDefaultPolicy interface {
	Add(prompt Prompt, priority int)
	Get(name string) *Prompt
}

type Prompt struct {
	Name        string
	Requestable bool
	Checks      []Check
}

type Check func(ctx *OIDCContext) (bool, error)

type OIDCContext struct {
	Cookies           map[string]string
	Oidc              map[string]interface{}
	Session           map[string]interface{}
	InternalAccountId string
}

type AccountPromptFactory struct {
	webIdStore  WebIdStore
	cookieStore CookieStore
	cookieName  string
}

func NewAccountPromptFactory(webIdStore WebIdStore, cookieStore CookieStore, cookieName string) *AccountPromptFactory {
	return &AccountPromptFactory{
		webIdStore:  webIdStore,
		cookieStore: cookieStore,
		cookieName:  cookieName,
	}
}

func (f *AccountPromptFactory) Handle(policy AccountDefaultPolicy) error {
	f.addAccountPrompt(policy)
	f.addWebIdVerificationPrompt(policy)
	return nil
}

func (f *AccountPromptFactory) addAccountPrompt(policy AccountDefaultPolicy) {
	check := func(ctx *OIDCContext) (bool, error) {
		cookie := ctx.Cookies[f.cookieName]
		var accountId string
		if cookie != "" {
			id, err := f.cookieStore.Get(cookie)
			if err == nil {
				accountId = id
				ctx.InternalAccountId = accountId
			}
		}
		log.Printf("Found account cookie %s and accountID %s", cookie, accountId)
		return accountId == "", nil
	}
	prompt := Prompt{Name: "account", Requestable: true, Checks: []Check{check}}
	policy.Add(prompt, 0)
}

func (f *AccountPromptFactory) addWebIdVerificationPrompt(policy AccountDefaultPolicy) {
	check := func(ctx *OIDCContext) (bool, error) {
		webId, _ := ctx.Session["accountId"].(string)
		if webId == "" {
			return false, nil
		}
		accountId := ctx.InternalAccountId
		if accountId == "" {
			log.Printf("Missing 'internalAccountId' value in OIDC context")
			return false, nil
		}
		isLinked, err := f.webIdStore.IsLinked(webId, accountId)
		if err != nil {
			return false, err
		}
		log.Printf("Session has WebID %s, which %s to the authenticated account", webId, map[bool]string{true: "belongs", false: "does not belong"}[isLinked])
		return !isLinked, nil
	}
	loginPrompt := policy.Get("login")
	if loginPrompt == nil {
		log.Fatalf("Missing default login policy")
	}
	loginPrompt.Checks = append(loginPrompt.Checks, check)
}

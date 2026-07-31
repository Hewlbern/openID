//go:build cssstub

package util

import (
	"errors"
	"log"
)

type InteractionRoute interface {
	MatchPath(path string) (interface{}, error)
}

func AssertAccountId(accountId *string) error {
	if accountId == nil || *accountId == "" {
		return errors.New("account ID not found")
	}
	return nil
}

func ParsePath(route InteractionRoute, path string) (interface{}, error) {
	match, err := route.MatchPath(path)
	if err != nil {
		log.Printf("Unable to parse path %s. This usually implies a server misconfiguration.", path)
		return nil, errors.New("internal server error")
	}
	return match, nil
}

func VerifyAccountId(input, expected *string) error {
	if input == nil || expected == nil || *input != *expected {
		return errors.New("account ID not found")
	}
	return nil
}

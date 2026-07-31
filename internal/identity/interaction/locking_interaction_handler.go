//go:build cssstub

package interaction

type ReadWriteLocker interface {
	WithReadLock(identifier interface{}, fn func() (*Representation, error)) (*Representation, error)
	WithWriteLock(identifier interface{}, fn func() (*Representation, error)) (*Representation, error)
}

type AccountIdRoute interface {
	GetPath(params map[string]interface{}) string
}

var ReadMethods = map[string]bool{
	"OPTIONS": true,
	"HEAD":    true,
	"GET":     true,
}

type LockingInteractionHandler struct {
	locker       ReadWriteLocker
	accountRoute AccountIdRoute
	source       InteractionHandler
}

func NewLockingInteractionHandler(locker ReadWriteLocker, accountRoute AccountIdRoute, source InteractionHandler) *LockingInteractionHandler {
	return &LockingInteractionHandler{
		locker:       locker,
		accountRoute: accountRoute,
		source:       source,
	}
}

func (h *LockingInteractionHandler) CanHandle(input InteractionHandlerInput) error {
	// Placeholder for canHandle logic
	return nil
}

func (h *LockingInteractionHandler) Handle(input InteractionHandlerInput) (*Representation, error) {
	accountId := input.AccountId

	// No lock if there is no account
	if accountId == nil {
		return h.source.Handle(input)
	}

	identifier := map[string]interface{}{"path": h.accountRoute.GetPath(map[string]interface{}{"accountId": *accountId})}

	// Placeholder for method checking
	method := "GET" // This should be extracted from operation
	if ReadMethods[method] {
		return h.locker.WithReadLock(identifier, func() (*Representation, error) {
			return h.source.Handle(input)
		})
	}

	return h.locker.WithWriteLock(identifier, func() (*Representation, error) {
		return h.source.Handle(input)
	})
}

//go:build cssstub

package storage

type PassthroughAdapter struct {
	name   string
	source Adapter
}

func NewPassthroughAdapter(name string, source Adapter) *PassthroughAdapter {
	return &PassthroughAdapter{
		name:   name,
		source: source,
	}
}

func (a *PassthroughAdapter) Upsert(id string, payload AdapterPayload, expiresIn ...int) error {
	// Delegate to source
	return nil
}

func (a *PassthroughAdapter) Find(id string) (*AdapterPayload, error) {
	// Delegate to source
	return nil, nil
}

func (a *PassthroughAdapter) FindByUserCode(userCode string) (*AdapterPayload, error) {
	// Delegate to source
	return nil, nil
}

func (a *PassthroughAdapter) FindByUid(uid string) (*AdapterPayload, error) {
	// Delegate to source
	return nil, nil
}

func (a *PassthroughAdapter) Consume(id string) error {
	// Delegate to source
	return nil
}

func (a *PassthroughAdapter) Destroy(id string) error {
	// Delegate to source
	return nil
}

func (a *PassthroughAdapter) RevokeByGrantId(grantId string) error {
	// Delegate to source
	return nil
}

type PassthroughAdapterFactory struct {
	source AdapterFactory
}

func NewPassthroughAdapterFactory(source AdapterFactory) *PassthroughAdapterFactory {
	return &PassthroughAdapterFactory{
		source: source,
	}
}

func (f *PassthroughAdapterFactory) CreateStorageAdapter(name string) Adapter {
	return f.source.CreateStorageAdapter(name)
}

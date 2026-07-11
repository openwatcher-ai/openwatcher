//go:build darwin && !cgo

package widgetauth

type systemStore struct{}

func (systemStore) Read() (string, error) { return "", ErrUnsupported }
func (systemStore) Write(string) error    { return ErrUnsupported }
func (systemStore) Delete() error         { return ErrUnsupported }
func NewSystemStore() SecretStore         { return systemStore{} }

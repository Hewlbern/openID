//go:build cssstub

package serialize

// ConvertingNotificationSerializer is a placeholder for the Go equivalent of ConvertingNotificationSerializer.ts.
type ConvertingNotificationSerializer struct{}

func NewConvertingNotificationSerializer() *ConvertingNotificationSerializer {
	return &ConvertingNotificationSerializer{}
}

func (s *ConvertingNotificationSerializer) Serialize(notification interface{}) (string, error) {
	return "", nil
}

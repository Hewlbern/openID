//go:build cssstub

package notifications

// NotificationDescriber is a placeholder for the Go equivalent of NotificationDescriber.ts.
type NotificationDescriber struct {
	// Add fields as needed for notification describing
}

// NewNotificationDescriber creates a new NotificationDescriber.
func NewNotificationDescriber() *NotificationDescriber {
	return &NotificationDescriber{}
}

// Describe returns a description of the notification (placeholder implementation).
func (d *NotificationDescriber) Describe() string {
	return "Notification description (not yet implemented)"
}

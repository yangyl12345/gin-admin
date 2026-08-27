package notify

import "context"

type Notifier interface {
	Available() bool
	Send(context.Context, string, string) (string, error)
}

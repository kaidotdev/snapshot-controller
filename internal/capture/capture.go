package capture

import (
	"context"
)

type CaptureResult struct {
	Screenshot []byte
	HTML       []byte
}

type Capturer interface {
	Capture(ctx context.Context, url string) (*CaptureResult, error)
}

package capture

import (
	"context"
)

type CaptureResult struct {
	Screenshot []byte
	HTML       []byte
}

type CaptureOptions struct {
	MaskSelectors []string
}

type Capturer interface {
	Capture(ctx context.Context, url string, captureOptions CaptureOptions) (*CaptureResult, error)
}

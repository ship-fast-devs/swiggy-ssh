package tui

import (
	"context"
	"io"

	domaininstamart "swiggy-ssh/internal/domain/instamart"
	"swiggy-ssh/internal/presentation/tui/instamartflow"
)

type InstamartService = instamartflow.InstamartService
type InstamartAction = instamartflow.InstamartAction
type InstamartResult = instamartflow.InstamartResult

const (
	InstamartActionQuit       = instamartflow.InstamartActionQuit
	InstamartActionBackToHome = instamartflow.InstamartActionBackToHome
)

// InstamartPlaceholderView remains for unauthenticated/legacy fallback paths.
// Successful authenticated sessions use InstamartAppView instead.
type InstamartPlaceholderView struct {
	UserID        string
	StatusMessage string
	In            io.Reader
}

func (v InstamartPlaceholderView) Render(ctx context.Context, w io.Writer) error {
	return instamartflow.InstamartPlaceholderView{
		UserID:        v.UserID,
		StatusMessage: v.StatusMessage,
		Viewport:      instamartViewport(ctx),
		In:            v.In,
	}.Render(ctx, w)
}

// InstamartView renders the static Instamart landing screen used by older tests
// and fallback paths that do not have an application service.
type InstamartView struct {
	AddressLabel  string
	AddressLine   string
	CartItemCount int
	StatusMessage string
	In            io.Reader
}

func (v InstamartView) Render(ctx context.Context, w io.Writer) error {
	return instamartflow.InstamartView{
		AddressLabel:  v.AddressLabel,
		AddressLine:   v.AddressLine,
		CartItemCount: v.CartItemCount,
		StatusMessage: v.StatusMessage,
		Viewport:      instamartViewport(ctx),
		In:            v.In,
	}.Render(ctx, w)
}

// InstamartAppView renders the service-backed Instamart flow.
type InstamartAppView struct {
	Service         InstamartService
	UserID          string
	Addresses       []domaininstamart.Address
	SelectedAddress domaininstamart.Address
	StartTracking   bool
	StatusMessage   string
	In              io.Reader
}

func (v InstamartAppView) Render(ctx context.Context, w io.Writer) error {
	_, err := v.RenderWithResult(ctx, w)
	return err
}

func (v InstamartAppView) RenderWithResult(ctx context.Context, w io.Writer) (InstamartResult, error) {
	return instamartflow.InstamartAppView{
		Service:         v.Service,
		UserID:          v.UserID,
		Addresses:       v.Addresses,
		SelectedAddress: v.SelectedAddress,
		StartTracking:   v.StartTracking,
		StatusMessage:   v.StatusMessage,
		Viewport:        instamartViewport(ctx),
		In:              v.In,
	}.RenderWithResult(ctx, w)
}

func instamartViewport(ctx context.Context) instamartflow.Viewport {
	viewport := viewportFromContext(ctx)
	return instamartflow.Viewport{Width: viewport.Width, Height: viewport.Height}
}

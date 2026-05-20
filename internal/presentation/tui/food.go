package tui

import (
	"context"
	"io"

	domainfood "swiggy-ssh/internal/domain/food"
	"swiggy-ssh/internal/presentation/tui/foodflow"
)

type FoodService = foodflow.FoodService
type FoodAction = foodflow.FoodAction
type FoodResult = foodflow.FoodResult

const (
	FoodActionQuit       = foodflow.FoodActionQuit
	FoodActionBackToHome = foodflow.FoodActionBackToHome
)

type FoodAppView struct {
	Service         FoodService
	UserID          string
	SelectedAddress domainfood.Address
	StatusMessage   string
	In              io.Reader
}

func (v FoodAppView) Render(ctx context.Context, w io.Writer) error {
	_, err := v.RenderWithResult(ctx, w)
	return err
}

func (v FoodAppView) RenderWithResult(ctx context.Context, w io.Writer) (FoodResult, error) {
	return foodflow.FoodAppView{
		Service:         v.Service,
		UserID:          v.UserID,
		SelectedAddress: v.SelectedAddress,
		StatusMessage:   v.StatusMessage,
		Viewport:        foodViewport(ctx),
		In:              v.In,
	}.RenderWithResult(ctx, w)
}

func foodViewport(ctx context.Context) foodflow.Viewport {
	viewport := viewportFromContext(ctx)
	return foodflow.Viewport{Width: viewport.Width, Height: viewport.Height}
}

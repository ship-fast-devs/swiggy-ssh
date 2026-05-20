package instamartflow

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appinstamart "swiggy-ssh/internal/application/instamart"
	domainauth "swiggy-ssh/internal/domain/auth"
	domaininstamart "swiggy-ssh/internal/domain/instamart"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInstamartAddressSelectionRequiredBeforeSearch(t *testing.T) {
	m := instamartModel{screen: instamartScreenHome}
	updated, cmd := m.handleHomeKey("/")
	if cmd != nil {
		t.Fatal("search without address should not call service")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenHome {
		t.Fatalf("expected home screen, got %v", got.screen)
	}
	if !strings.Contains(got.err, "Choose a deployment address") {
		t.Fatalf("expected address error, got %q", got.err)
	}
}

func TestInstamartSearchUsesSelectedAddress(t *testing.T) {
	fake := &fakeInstamartService{}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{ctx: context.Background(), service: fake, selectedAddress: &address}

	msg := m.searchProductsCmd("milk", false, 0)()
	if _, ok := msg.(instamartProductsMsg); !ok {
		t.Fatalf("expected products message, got %T", msg)
	}
	if fake.searchInput.AddressID != "addr-1" || fake.searchInput.Query != "milk" {
		t.Fatalf("unexpected search input: %+v", fake.searchInput)
	}
}

func TestInstamartHomeEscReturnsToMainMenu(t *testing.T) {
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{screen: instamartScreenHome, selectedAddress: &address}

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc from Instamart home should quit view back to main menu")
	}
	got := updated.(instamartModel)
	if got.result.Action != InstamartActionBackToHome {
		t.Fatalf("expected back-to-home action, got %v", got.result.Action)
	}
	if got.result.SelectedAddress.ID != "addr-1" {
		t.Fatalf("expected selected address to be preserved, got %+v", got.result.SelectedAddress)
	}
}

func TestRootLaunchedTrackingBackReturnsToMainMenu(t *testing.T) {
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	for _, tt := range []struct {
		name   string
		screen instamartScreen
		key    tea.KeyMsg
	}{
		{name: "esc from tracking", screen: instamartScreenTracking, key: tea.KeyMsg{Type: tea.KeyEsc}},
		{name: "back from tracking", screen: instamartScreenTracking, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}},
		{name: "enter from tracking", screen: instamartScreenTracking, key: tea.KeyMsg{Type: tea.KeyEnter}},
		{name: "back from order result", screen: instamartScreenOrderResult, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")}},
		{name: "enter from message", screen: instamartScreenMessage, key: tea.KeyMsg{Type: tea.KeyEnter}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := instamartModel{screen: tt.screen, startTracking: true, selectedAddress: &address}
			updated, cmd := m.handleKey(tt.key)
			if cmd == nil {
				t.Fatal("root-launched tracking back should quit Instamart view")
			}
			got := updated.(instamartModel)
			if got.result.Action != InstamartActionBackToHome {
				t.Fatalf("expected back-to-home action, got %v", got.result.Action)
			}
			if got.result.SelectedAddress.ID != "addr-1" {
				t.Fatalf("expected selected address to be preserved, got %+v", got.result.SelectedAddress)
			}
		})
	}
}

func TestInstamartInternalTrackingBackReturnsToInstamartHome(t *testing.T) {
	m := instamartModel{screen: instamartScreenTracking}

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if cmd != nil {
		t.Fatal("internal tracking back should stay inside Instamart")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenHome {
		t.Fatalf("expected Instamart home, got %v", got.screen)
	}
}

func TestRootLaunchedTrackingQStillQuitsSession(t *testing.T) {
	m := instamartModel{screen: instamartScreenTracking, startTracking: true}

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should quit")
	}
	got := updated.(instamartModel)
	if got.result.Action != InstamartActionQuit {
		t.Fatalf("expected session quit action, got %v", got.result.Action)
	}
}

func TestInstamartSearchInputAcceptsSpacesAndRendersCursor(t *testing.T) {
	m := instamartModel{screen: instamartScreenSearchInput}

	updated, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("amul")})
	m = updated.(instamartModel)
	updated, _ = m.handleSearchKey(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(instamartModel)
	updated, _ = m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("milk")})
	m = updated.(instamartModel)

	if m.searchQuery != "amul milk" {
		t.Fatalf("expected space-preserving query, got %q", m.searchQuery)
	}
	out := m.View()
	if !strings.Contains(out, "amul milk") || !strings.Contains(out, "_") {
		t.Fatalf("expected rendered query with visible cursor, got %q", out)
	}

	updated, _ = m.handleSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(instamartModel)
	if m.searchQuery != "amul mil" {
		t.Fatalf("expected backspace to remove one rune, got %q", m.searchQuery)
	}
}

func TestInstamartSearchPreviewStaysOnSearchScreen(t *testing.T) {
	m := instamartModel{screen: instamartScreenSearchInput, searchQuery: "milk", searchPreviewVersion: 2}
	updated, _ := m.Update(instamartProductsMsg{query: "milk", version: 2, preview: true, result: productSearchResult("spin-milk", "Milk"), elapsed: 32 * time.Millisecond})
	got := updated.(instamartModel)

	if got.screen != instamartScreenSearchInput {
		t.Fatalf("preview must stay on search input, got %v", got.screen)
	}
	if !got.searchPreviewLoaded || got.searchPreviewQuery != "milk" || len(got.searchPreviewRows) != 1 {
		t.Fatalf("expected loaded preview rows, got query=%q loaded=%v rows=%d", got.searchPreviewQuery, got.searchPreviewLoaded, len(got.searchPreviewRows))
	}
	view := got.View()
	if !strings.Contains(view, "preview · enter opens results") || strings.Contains(view, "searching...") {
		t.Fatalf("expected preview rendering without searching copy, got %q", got.View())
	}
	if strings.Contains(view, "grep products: milk") {
		t.Fatalf("query should only render in the input line, got %q", view)
	}
	if strings.Contains(view, cursorStyle.Render("> ")) || strings.Contains(view, "#   item") {
		t.Fatalf("preview must not look selectable, got %q", view)
	}
	if !strings.Contains(view, "1 matches in 32ms") {
		t.Fatalf("expected preview timing, got %q", got.View())
	}
}

func TestInstamartSearchTypingDoesNotClearScreen(t *testing.T) {
	m := instamartModel{screen: instamartScreenSearchInput}
	updated, cmd := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if cmd != nil {
		t.Fatalf("single-character search input should not request a clear command")
	}
	if updated.(instamartModel).searchQuery != "d" {
		t.Fatalf("expected query update, got %q", updated.(instamartModel).searchQuery)
	}
}

func TestInstamartHomeUsesDeveloperCopyAndStatusBar(t *testing.T) {
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{screen: instamartScreenHome, selectedAddress: &address, intendedItems: []domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 3}}}
	out := m.View()

	for _, want := range []string{"grep products", "recent cache", "staged cart", "env=instamart  auth=ok  cart=3  mode=home"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in home output", want)
		}
	}
	for _, old := range []string{"Search products", "Your go-to items", "View cart", "Track active order", "tail active order", "deploy history", "Order history", "Change address", "switch target address", "Cancel order help", "Delivering to", "target locked", "deploying to:", "Cart:"} {
		if strings.Contains(out, old) {
			t.Fatalf("old copy %q should not be rendered", old)
		}
	}
}

func TestInstamartSearchPreviewLoadingUsesScanningIndexCopy(t *testing.T) {
	m := instamartModel{screen: instamartScreenSearchInput, searchQuery: "milk", searchPreviewLoading: true}
	out := m.View()

	if !strings.Contains(out, "scanning index...") {
		t.Fatalf("expected scanning index loader, got %q", out)
	}
	if strings.Contains(out, "searching...") || strings.Contains(out, "Searching") {
		t.Fatalf("live preview loader must not use search copy, got %q", out)
	}
}

func TestInstamartSearchPreviewIgnoresStaleResponses(t *testing.T) {
	m := instamartModel{screen: instamartScreenSearchInput, searchQuery: "amul milk", searchPreviewVersion: 3}
	updated, _ := m.Update(instamartProductsMsg{query: "amul", version: 2, preview: true, result: productSearchResult("spin-old", "Old Milk")})
	got := updated.(instamartModel)

	if got.searchPreviewLoaded || len(got.searchPreviewRows) != 0 {
		t.Fatalf("stale preview should be ignored, got loaded=%v rows=%d", got.searchPreviewLoaded, len(got.searchPreviewRows))
	}
}

func TestInstamartSearchEnterUsesCurrentPreview(t *testing.T) {
	m := instamartModel{
		screen:                instamartScreenSearchInput,
		searchQuery:           "milk",
		searchPreviewQuery:    "milk",
		searchPreviewProducts: productSearchResult("spin-milk", "Milk").Products,
		searchPreviewRows:     flattenProductRows(productSearchResult("spin-milk", "Milk").Products),
		searchPreviewLoaded:   true,
	}

	updated, cmd := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("current preview should open without another search")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenProductList || len(got.rows) != 1 {
		t.Fatalf("expected product list from preview, got screen=%v rows=%d", got.screen, len(got.rows))
	}
}

func TestInstamartSearchEnterWithoutPreviewRunsCommittedSearch(t *testing.T) {
	fake := &fakeInstamartService{searchResult: productSearchResult("spin-milk", "Milk")}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{ctx: context.Background(), service: fake, screen: instamartScreenSearchInput, selectedAddress: &address, searchQuery: "amul milk"}

	updated, cmd := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected committed search command")
	}
	loading := updated.(instamartModel)
	if loading.screen != instamartScreenLoading || loading.loading != "scanning index..." {
		t.Fatalf("expected scanning loader, got screen=%v loading=%q", loading.screen, loading.loading)
	}
	_ = cmd()
	if fake.searchInput.Query != "amul milk" {
		t.Fatalf("expected exact committed query, got %q", fake.searchInput.Query)
	}
}

func TestInstamartAppViewUsesRootAddressFlow(t *testing.T) {
	fake := &fakeInstamartService{}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	err := InstamartAppView{Service: fake, UserID: "user-1"}.Render(ctx, &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if fake.addressUserID != "" {
		t.Fatalf("instamart must not load its own addresses, got user %q", fake.addressUserID)
	}
	if !strings.Contains(buf.String(), "Choose a deployment address from the main menu") {
		t.Fatalf("expected root address guidance, got %q", buf.String())
	}
}

func TestInstamartAppViewUsesSessionSelectedAddress(t *testing.T) {
	fake := &fakeInstamartService{}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var buf bytes.Buffer
	err := InstamartAppView{
		Service:         fake,
		UserID:          "user-1",
		Addresses:       []domaininstamart.Address{{ID: "addr-1", Label: "Home"}},
		SelectedAddress: domaininstamart.Address{ID: "addr-1", Label: "Home"},
		In:              strings.NewReader("q"),
	}.Render(ctx, &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if fake.addressCalls != 0 {
		t.Fatalf("expected session address to skip address load, got %d calls", fake.addressCalls)
	}
	if !strings.Contains(buf.String(), "deploying to") || !strings.Contains(buf.String(), "Home") {
		t.Fatalf("expected selected session address in output, got %q", buf.String())
	}
}

func TestInstamartProductRowsRenderVariationsAndSponsored(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenProductList,
		rows: []productVariationRow{{
			Product: domaininstamart.Product{DisplayName: "Bread", Promoted: true, InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{
				SpinID:              "spin-bread",
				DisplayName:         "Sandwich Bread",
				QuantityDescription: "400 g",
				Price:               domaininstamart.Price{OfferPrice: 49},
				InStock:             true,
			},
		}},
	}
	out := m.View()
	for _, want := range []string{"type item", "◆", "[ad]", "Sandwich Bread", "400 g", "Rs 49"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in product output", want)
		}
	}
	if strings.Contains(out, "200") || strings.Contains(out, "409") {
		t.Fatalf("product output should not render pseudo HTTP status codes: %q", out)
	}
}

func TestInstamartUnavailableProductRowsRenderDimmedUnavailable(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenProductList,
		rows: []productVariationRow{{
			Product:   domaininstamart.Product{DisplayName: "Bread", InStock: true, Available: false},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-bread", DisplayName: "Bread", QuantityDescription: "400 g", InStock: true},
		}},
	}
	out := m.View()
	for _, want := range []string{"[x] unavailable", "38;5;240"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in unavailable row output", want)
		}
	}
	if strings.Contains(out, "409") {
		t.Fatalf("unavailable row should not render conflict code: %q", out)
	}
}

func TestInstamartQuantityRendersSelectedItemManifest(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenQuantity,
		selectedRow: &productVariationRow{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", QuantityDescription: "1 L", Price: domaininstamart.Price{OfferPrice: 60}, InStock: true},
		},
		quantity: 2,
	}
	out := m.View()
	for _, want := range []string{"stage item", "item:", "Milk", "pack:", "1 L", "price:", "Rs 60", "status:", "available", "quantity:", "b/esc results"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in quantity output", want)
		}
	}
	if !strings.Contains(out, "38;2;252;128;25") || !strings.Contains(out, "38;2;255;247;237") {
		t.Fatalf("expected YAML key/value colors in quantity output, got %q", out)
	}
	if strings.Contains(out, "action: stage item") {
		t.Fatalf("quantity output should not repeat the stage action, got %q", out)
	}
	if strings.Contains(out, "200") || strings.Contains(out, "409") {
		t.Fatalf("quantity output should not render pseudo HTTP status codes: %q", out)
	}
}

func TestInstamartQuantityBackReturnsToProductResults(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenQuantity,
		rows: []productVariationRow{{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", InStock: true},
		}},
		cursor: 0,
		selectedRow: &productVariationRow{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", InStock: true},
		},
	}

	updated, cmd := m.handleQuantityKey("b")
	if cmd != nil {
		t.Fatal("back to results should not call service")
	}
	if updated.(instamartModel).screen != instamartScreenProductList {
		t.Fatalf("expected product list, got %v", updated.(instamartModel).screen)
	}

	updated, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(instamartModel).screen != instamartScreenProductList {
		t.Fatalf("expected esc to return to product list, got %v", updated.(instamartModel).screen)
	}
}

func TestInstamartUnavailableVariationCannotBeSelected(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenProductList,
		rows: []productVariationRow{{
			Product:   domaininstamart.Product{DisplayName: "Bread", InStock: true, Available: false},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-bread", DisplayName: "Bread", InStock: true},
		}},
	}

	updated, cmd := m.handleProductKey("1")
	if cmd != nil {
		t.Fatal("unavailable row should not start cart update")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenProductList {
		t.Fatalf("expected to stay on product list, got %v", got.screen)
	}
	if !strings.Contains(got.err, "currently unavailable") {
		t.Fatalf("expected unavailable message, got %q", got.err)
	}
}

func TestInstamartProductNumericShortcutSelectsVisibleRow(t *testing.T) {
	rows := make([]productVariationRow, 0, 12)
	for i := 0; i < 12; i++ {
		rows = append(rows, productVariationRow{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-" + string(rune('a'+i)), DisplayName: "Milk", InStock: true},
		})
	}
	m := instamartModel{screen: instamartScreenProductList, rows: rows, cursor: 9}

	updated, cmd := m.handleProductKey("1")
	if cmd != nil {
		t.Fatal("selecting a product row should not call service")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenQuantity || got.selectedRow == nil || got.selectedRow.Variation.SpinID != "spin-b" {
		t.Fatalf("expected visible shortcut 1 to select global row 2, got screen=%v row=%+v", got.screen, got.selectedRow)
	}
	if out := m.View(); !strings.Contains(out, "showing 2-10 of 12") || strings.Contains(out, "10  ") {
		t.Fatalf("expected paged visible numbering, got %q", out)
	}
}

func TestInstamartCartUpdatesOnlyAfterVariationAndQuantity(t *testing.T) {
	fake := &fakeInstamartService{cart: cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 2, FinalPrice: 120}})}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{
		ctx:             context.Background(),
		service:         fake,
		screen:          instamartScreenProductList,
		selectedAddress: &address,
		rows: []productVariationRow{{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", QuantityDescription: "1 L", Price: domaininstamart.Price{OfferPrice: 60}, InStock: true},
		}},
	}

	updated, cmd := m.handleProductKey("1")
	if cmd != nil || fake.updateCalls != 0 {
		t.Fatal("selecting a variation must not update cart yet")
	}
	selected := updated.(instamartModel)
	selected.quantity = 2
	_, cmd = selected.handleQuantityKey("enter")
	if cmd == nil {
		t.Fatal("quantity confirmation should update cart")
	}
	_ = cmd()
	if fake.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", fake.updateCalls)
	}
	if len(fake.updateInput.Items) != 1 || fake.updateInput.Items[0].SpinID != "spin-milk" || fake.updateInput.Items[0].Quantity != 2 {
		t.Fatalf("unexpected update items: %+v", fake.updateInput.Items)
	}
}

func TestInstamartQuantityUpdateSendsFullIntendedCart(t *testing.T) {
	fake := &fakeInstamartService{cart: cartWithItems([]domaininstamart.CartItem{
		{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 3, FinalPrice: 180},
		{SpinID: "spin-bread", Name: "Bread 400 g", Quantity: 1, FinalPrice: 49},
	})}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{
		ctx:             context.Background(),
		service:         fake,
		screen:          instamartScreenQuantity,
		selectedAddress: &address,
		intendedItems: []domaininstamart.CartUpdateItem{
			{SpinID: "spin-milk", Quantity: 1},
			{SpinID: "spin-bread", Quantity: 1},
		},
		selectedRow: &productVariationRow{Variation: domaininstamart.ProductVariation{SpinID: "spin-milk"}},
		quantity:    3,
	}

	_, cmd := m.handleQuantityKey("enter")
	if cmd == nil {
		t.Fatal("expected update command")
	}
	_ = cmd()
	if len(fake.updateInput.Items) != 2 {
		t.Fatalf("expected full cart update, got %+v", fake.updateInput.Items)
	}
	if fake.updateInput.Items[0].SpinID != "spin-milk" || fake.updateInput.Items[0].Quantity != 3 {
		t.Fatalf("expected milk quantity replacement, got %+v", fake.updateInput.Items[0])
	}
	if fake.updateInput.Items[1].SpinID != "spin-bread" || fake.updateInput.Items[1].Quantity != 1 {
		t.Fatalf("expected bread to be preserved, got %+v", fake.updateInput.Items[1])
	}
}

func TestInstamartUpdateRefreshesCartForPaymentMethods(t *testing.T) {
	updateCart := cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 1, FinalPrice: 60}})
	updateCart.AvailablePaymentMethods = nil
	fake := &fakeInstamartService{
		updateCart: &updateCart,
		cart:       cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 1, FinalPrice: 60}}),
	}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{ctx: context.Background(), service: fake, selectedAddress: &address}

	msg := m.updateCartCmd([]domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 1}})()
	cartMsg, ok := msg.(instamartCartMsg)
	if !ok {
		t.Fatalf("expected cart message, got %T", msg)
	}
	if fake.updateCalls != 1 || fake.getCartCalls != 1 {
		t.Fatalf("expected update then refresh, got update=%d get=%d", fake.updateCalls, fake.getCartCalls)
	}
	if len(cartMsg.cart.AvailablePaymentMethods) != 1 || cartMsg.cart.AvailablePaymentMethods[0] != "Cash" {
		t.Fatalf("expected refreshed Cash payment method, got %+v", cartMsg.cart.AvailablePaymentMethods)
	}
}

func TestInstamartUpdateCartFallsBackWhenRefreshFails(t *testing.T) {
	updateCart := cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 1, FinalPrice: 60}})
	updateCart.AvailablePaymentMethods = nil
	fake := &fakeInstamartService{updateCart: &updateCart, getCartErr: errors.New("refresh failed")}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{ctx: context.Background(), service: fake, selectedAddress: &address}

	msg := m.updateCartCmd([]domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 1}})()
	updated, _ := m.Update(msg)
	got := updated.(instamartModel)
	if got.screen != instamartScreenCartReview || len(got.currentCart.Items) != 1 {
		t.Fatalf("expected staged fallback cart on review screen, got screen=%v cart=%+v", got.screen, got.currentCart)
	}
	if !strings.Contains(got.err, "payment refresh failed") {
		t.Fatalf("expected visible refresh failure, got %q", got.err)
	}
	updated, cmd := got.handleCartReviewKey("p")
	if cmd != nil || updated.(instamartModel).screen != instamartScreenCartReview || !strings.Contains(updated.(instamartModel).err, "No terminal payment method") {
		t.Fatalf("checkout should remain blocked without refreshed payment methods, got screen=%v err=%q", updated.(instamartModel).screen, updated.(instamartModel).err)
	}
}

func TestInstamartPaymentSelectionPrefersCashAndSkipsBlank(t *testing.T) {
	if got := preferredPaymentMethod([]string{"", "Cash"}); got != "Cash" {
		t.Fatalf("expected Cash, got %q", got)
	}
	if got := preferredPaymentMethod([]string{"", "Card"}); got != "Card" {
		t.Fatalf("expected first non-empty fallback, got %q", got)
	}
	if got := preferredPaymentMethod([]string{"", "  "}); got != "" {
		t.Fatalf("expected empty when no payment method is usable, got %q", got)
	}
}

func TestInstamartCartReviewRendersCheckoutDetails(t *testing.T) {
	m := instamartModel{screen: instamartScreenCartReview, currentCart: domaininstamart.Cart{
		AddressLabel:       "Work",
		AddressDisplayLine: "Tower, Bangalore",
		AddressLocation:    &domaininstamart.Location{Lat: 12.34, Lng: 56.78},
		Items:              []domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk 1 L", Quantity: 2, FinalPrice: 120}},
		Bill: domaininstamart.BillBreakdown{
			Lines:       []domaininstamart.BillLine{{Label: "Item Total", Value: "Rs 120"}, {Label: "Coupon Discount", Value: "-Rs 20"}},
			ToPayLabel:  "To Pay",
			ToPayValue:  "Rs 100",
			ToPayRupees: 100,
		},
		AvailablePaymentMethods: []string{"Cash"},
		StoreIDs:                []string{"store-1", "store-2"},
	}}
	out := m.View()
	for _, want := range []string{"review staged cart", "target", "staged", "diff", "payment", "next", "Work", "2x", "Milk 1 L", "Item Total", "Coupon Discount", "To Pay", "Rs 100", "Cash", "p deploy gate", "p/enter deploy", "warn: cart spans 2 stores"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in cart review", want)
		}
	}
	for _, noisy := range []string{"p/enter ship", "j/k scroll"} {
		if strings.Contains(out, noisy) {
			t.Fatalf("did not expect %q in non-overflowing cart review", noisy)
		}
	}
	if !strings.Contains(out, "38;2;0;170;68") || !strings.Contains(out, "38;2;255;68;68") {
		t.Fatal("expected green plus and red minus diff markers")
	}
	if !strings.Contains(out, "48;") {
		t.Fatal("expected GitHub-style add/remove row backgrounds")
	}
	if strings.Contains(out, "38;2;255;68;68m-Rs 20") {
		t.Fatalf("discount value should not be colored red: %q", out)
	}
	if strings.Contains(out, "Coupon Discount                           -Rs 20") {
		t.Fatalf("discount value should not keep its leading minus: %q", out)
	}
	if strings.Contains(out, "Tower, Bangalore") {
		t.Fatal("full address must not be rendered")
	}
	if strings.Contains(out, "12.34") || strings.Contains(out, "56.78") {
		t.Fatal("coordinates must not be rendered")
	}
}

func TestInstamartEmptyCartReviewUsesCleanWorkingTreeCopy(t *testing.T) {
	m := instamartModel{screen: instamartScreenCartReview, currentCart: domaininstamart.Cart{AvailablePaymentMethods: []string{"Cash"}}}
	out := m.View()
	if !strings.Contains(out, "working tree clean") {
		t.Fatalf("expected clean cart copy, got %q", out)
	}
}

func TestInstamartStableFrameHeightAndBodyAnchor(t *testing.T) {
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{screen: instamartScreenHome, selectedAddress: &address, viewport: Viewport{Width: 80, Height: 24}}
	out := m.View()
	if got := strings.Count(out, "\r\n"); got != 24 {
		t.Fatalf("expected 80x24 frame height, got %d lines: %q", got, out)
	}
	bodyLine := renderedLineIndex(out, "grep products")
	if bodyLine < 0 {
		t.Fatalf("expected home body anchor, got %q", out)
	}
	m.status = "loaded"
	m.err = "blocked"
	withSlots := m.View()
	if got := strings.Count(withSlots, "\r\n"); got != 24 {
		t.Fatalf("expected status/error frame height to stay fixed, got %d lines", got)
	}
	if renderedLineIndex(withSlots, "grep products") != bodyLine {
		t.Fatalf("body anchor moved after status/error slots: before=%d after=%d", bodyLine, renderedLineIndex(withSlots, "grep products"))
	}
}

func TestInstamartWindowSizeTooSmallWarns(t *testing.T) {
	m := instamartModel{screen: instamartScreenHome}
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
	if cmd != nil {
		t.Fatal("window resize should not trigger command")
	}
	out := updated.(instamartModel).View()
	if !strings.Contains(out, "80x24") || strings.Contains(out, "swiggy.ssh") {
		t.Fatalf("expected concise resize warning, got %q", out)
	}
}

func TestInstamartCartReviewScrollsOverflow(t *testing.T) {
	items := make([]domaininstamart.CartItem, 0, 14)
	for i := 0; i < 14; i++ {
		items = append(items, domaininstamart.CartItem{SpinID: "spin", Name: "Item", Quantity: 1, FinalPrice: 10})
	}
	m := instamartModel{screen: instamartScreenCartReview, currentCart: cartWithItems(items)}
	updated, _ := m.handleCartReviewKey("j")
	got := updated.(instamartModel)
	if got.cartScroll != 1 {
		t.Fatalf("expected cart scroll to advance, got %d", got.cartScroll)
	}
	if !strings.Contains(got.View(), "j/k scroll") {
		t.Fatalf("expected scroll affordance in overflowing cart, got %q", got.View())
	}
	if !strings.Contains(got.View(), "Item") {
		t.Fatalf("scroll indicator should not replace all cart content, got %q", got.View())
	}
}

func TestInstamartFixedBodyShowsOverflowIndicator(t *testing.T) {
	var body strings.Builder
	for i := 0; i < bodyRows+3; i++ {
		body.WriteString(line(" row"))
	}
	out := fixedBody(body.String(), bodyRows)
	if !strings.Contains(out, "more lines") {
		t.Fatalf("expected overflow affordance, got %q", out)
	}
	if got := strings.Count(out, "\r\n"); got != bodyRows {
		t.Fatalf("expected stable body rows, got %d", got)
	}
	for _, row := range strings.Split(strings.TrimSuffix(out, "\r\n"), "\r\n") {
		if strings.Contains(row, "more lines") && !strings.HasPrefix(row, "│") {
			t.Fatalf("overflow affordance should be framed, got %q", row)
		}
	}
}

func TestInstamartErrorsAreSanitized(t *testing.T) {
	out := displayErr("Checkout blocked", domaininstamart.ErrCheckoutRequiresReview)
	if !strings.Contains(out, "review the latest cart") {
		t.Fatalf("expected useful sentinel mapping, got %q", out)
	}
	out = displayErr("Checkout blocked", errWithSensitiveText("order real-order-123 address full-address"))
	if strings.Contains(out, "real-order-123") || strings.Contains(out, "full-address") {
		t.Fatalf("raw provider error leaked: %q", out)
	}
	if !strings.Contains(out, "Please try again") {
		t.Fatalf("expected generic fallback, got %q", out)
	}
}

func TestInstamartCheckoutRequiresExplicitConfirmation(t *testing.T) {
	fake := &fakeInstamartService{checkoutResult: domaininstamart.CheckoutResult{Message: "Instamart order placed successfully! Mock order confirmed."}}
	m := checkoutConfirmModel(fake)

	_, cmd := m.handleCartReviewKey("p")
	if cmd != nil || fake.checkoutCalls != 0 {
		t.Fatal("moving to confirmation must not call checkout")
	}
	confirm := checkoutConfirmModel(fake)
	_, cmd = confirm.handleCheckoutConfirmKey("y")
	if cmd == nil {
		t.Fatal("expected checkout command after explicit confirmation")
	}
	_ = cmd()
	if fake.checkoutCalls != 1 {
		t.Fatalf("expected checkout call, got %d", fake.checkoutCalls)
	}
	if !fake.checkoutInput.Confirmed {
		t.Fatal("checkout must pass Confirmed=true")
	}
	if fake.checkoutInput.AddressID != "addr-1" {
		t.Fatalf("checkout should use selected address, got %q", fake.checkoutInput.AddressID)
	}
}

func TestInstamartCheckoutConfirmRendersDeployGate(t *testing.T) {
	m := checkoutConfirmModel(&fakeInstamartService{})
	out := m.View()
	for _, want := range []string{"Are you sure you want to push --force groceries?", "git push --force groceries", "Home", "payment Cash", "total Rs 80", "y deploy / n cancel"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in checkout confirmation", want)
		}
	}
	for _, noisy := range []string{"REAL SWIGGY ORDER", "places a paid Instamart order", "[ok] address selected", "press y to confirm order", "ship order"} {
		if strings.Contains(out, noisy) {
			t.Fatalf("checkout gate should not render noisy copy %q: %q", noisy, out)
		}
	}
}

func TestInstamartCheckoutConfirmFooterUsesDeployCopy(t *testing.T) {
	m := checkoutConfirmModel(&fakeInstamartService{})
	footer := m.footer()
	for _, want := range []string{"y deploy", "n cancel"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("expected %q in footer, got %q", want, footer)
		}
	}
	if strings.Contains(footer, "confirm order") || strings.Contains(footer, "help") {
		t.Fatalf("checkout footer should stay low-clutter, got %q", footer)
	}
}

func TestInstamartOrderResultRendersDeployLogs(t *testing.T) {
	m := instamartModel{screen: instamartScreenOrderResult, checkoutElapsed: 92 * time.Second, checkoutResult: domaininstamart.CheckoutResult{Message: "Instamart order placed successfully!", Status: "confirmed", PaymentMethod: "Cash", OrderIDs: []string{"order-1"}, CartTotal: 80}}
	out := m.View()
	for _, want := range []string{"deploy logs", "[ok] git push --force origin groceries", "[ok] Instamart order placed successfully!", "[ok] payment method: Cash", "[ok] status: confirmed", "[info] order_id=order-1", "[info] stores=1", "[info] total=Rs 80", "[info] deployed_in=1m 32s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in receipt logs", want)
		}
	}
}

func TestInstamartOperationTimingStatus(t *testing.T) {
	m := instamartModel{screen: instamartScreenLoadingAddresses}
	updated, _ := m.Update(instamartAddressesMsg{addresses: []domaininstamart.Address{{ID: "addr-1", Label: "Home"}}, elapsed: 24 * time.Millisecond})
	if !strings.Contains(updated.(instamartModel).status, "loaded addresses in 24ms") {
		t.Fatalf("expected address timing, got %q", updated.(instamartModel).status)
	}

	updated, _ = instamartModel{screen: instamartScreenLoading}.Update(instamartCartMsg{cart: cartWithItems(nil), action: "loaded staged cart", elapsed: 1100 * time.Millisecond})
	if !strings.Contains(updated.(instamartModel).status, "loaded staged cart in 1.1s") {
		t.Fatalf("expected cart timing, got %q", updated.(instamartModel).status)
	}

	updated, _ = instamartModel{screen: instamartScreenLoading}.Update(instamartCheckoutMsg{result: domaininstamart.CheckoutResult{Message: "ok"}, elapsed: 2*time.Minute + 3*time.Second})
	if !strings.Contains(updated.(instamartModel).status, "deploy complete") {
		t.Fatalf("expected deploy timing, got %q", updated.(instamartModel).status)
	}
}

func TestFormatElapsed(t *testing.T) {
	for _, tt := range []struct {
		elapsed time.Duration
		want    string
	}{
		{42 * time.Millisecond, "42ms"},
		{1500 * time.Millisecond, "1.5s"},
		{32*time.Minute + 23*time.Second, "32m 23s"},
	} {
		if got := formatElapsed(tt.elapsed); got != tt.want {
			t.Fatalf("formatElapsed(%s) = %q, want %q", tt.elapsed, got, tt.want)
		}
	}
}

func TestInstamartHelpScreenOpensAndReturns(t *testing.T) {
	m := instamartModel{screen: instamartScreenHome}
	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if cmd != nil {
		t.Fatal("help should not call service")
	}
	help := updated.(instamartModel)
	if help.screen != instamartScreenHelp || !strings.Contains(help.View(), "swiggy.dev keys") || !strings.Contains(help.View(), "/          grep products") {
		t.Fatalf("expected help screen, got %q", help.View())
	}
	updated, _ = help.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if updated.(instamartModel).screen != instamartScreenHome {
		t.Fatalf("expected help to return home, got %v", updated.(instamartModel).screen)
	}
}

func TestInstamartHelpReturnsToStaticFallback(t *testing.T) {
	m := instamartModel{screen: instamartScreenStatic}
	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if cmd != nil {
		t.Fatal("help should not call service from static fallback")
	}
	help := updated.(instamartModel)
	if help.screen != instamartScreenHelp {
		t.Fatalf("expected help screen, got %v", help.screen)
	}
	updated, _ = help.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if updated.(instamartModel).screen != instamartScreenStatic {
		t.Fatalf("expected help to return static fallback, got %v", updated.(instamartModel).screen)
	}
}

func TestInstamartCheckoutConfirmRequiresY(t *testing.T) {
	m := checkoutConfirmModel(&fakeInstamartService{})
	updated, cmd := m.handleCheckoutConfirmKey("enter")
	if cmd != nil {
		t.Fatal("enter should not place an order from checkout confirmation")
	}
	if updated.(instamartModel).screen != instamartScreenCheckoutConfirm {
		t.Fatalf("expected checkout confirm to remain active, got %v", updated.(instamartModel).screen)
	}

	updated, cmd = m.handleCheckoutConfirmKey("y")
	if cmd == nil {
		t.Fatal("y should place the order")
	}
	if updated.(instamartModel).screen != instamartScreenLoading {
		t.Fatalf("expected loading screen after y, got %v", updated.(instamartModel).screen)
	}
}

func TestInstamartCheckoutBlocksStaleCartAddress(t *testing.T) {
	fake := &fakeInstamartService{}
	m := checkoutConfirmModel(fake)
	m.screen = instamartScreenCartReview
	m.currentCart.AddressID = "addr-other"
	updated, cmd := m.handleCartReviewKey("p")
	if cmd != nil {
		t.Fatal("stale cart address should not proceed to checkout confirmation")
	}
	if !strings.Contains(updated.(instamartModel).err, "Cart address no longer matches") {
		t.Fatalf("expected stale address error, got %q", updated.(instamartModel).err)
	}
}

func TestInstamartCheckoutConfirmEscReturnsToCartReview(t *testing.T) {
	m := checkoutConfirmModel(&fakeInstamartService{})

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("esc should not trigger a command")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenCartReview {
		t.Fatalf("expected cart review screen, got %v", got.screen)
	}
	if got.status != "Checkout cancelled." {
		t.Fatalf("expected checkout cancelled status, got %q", got.status)
	}
}

func TestInstamartTrackingWithoutLocationUsesSafeMessage(t *testing.T) {
	fake := &fakeInstamartService{orders: domaininstamart.OrderHistory{Orders: []domaininstamart.OrderSummary{{OrderID: "real-order-hidden", Status: "CONFIRMED", Active: true, ItemCount: 1, TotalRupees: 140}}}}
	m := instamartModel{ctx: context.Background(), service: fake}
	msg := m.loadOrdersCmd(true)()
	ordersMsg, ok := msg.(instamartOrdersMsg)
	if !ok {
		t.Fatalf("expected orders message, got %T", msg)
	}
	updated, _ := m.Update(ordersMsg)
	out := updated.(instamartModel).View()
	if fake.trackCalls != 0 {
		t.Fatal("tracking must not be called without hidden coordinates")
	}
	if !strings.Contains(out, "Tracking is unavailable for this order in the terminal") {
		t.Fatal("expected safe tracking fallback message")
	}
	if strings.Contains(out, "real-order-hidden") {
		t.Fatal("order id must not be rendered in fallback output")
	}
}

func TestInstamartOrderHistoryEnterTracksSelectedOrder(t *testing.T) {
	location := &domaininstamart.Location{Lat: 12.9, Lng: 77.6}
	fake := &fakeInstamartService{tracking: domaininstamart.TrackingStatus{StatusMessage: "Order is getting packed", ETAText: "5 mins"}}
	m := instamartModel{
		ctx:     context.Background(),
		service: fake,
		screen:  instamartScreenOrders,
		orders:  domaininstamart.OrderHistory{Orders: []domaininstamart.OrderSummary{{OrderID: "order-1", Status: "CONFIRMED", Active: true, Location: location}}},
	}

	_, cmd := m.handleOrdersKey("enter")
	if cmd == nil {
		t.Fatal("expected tracking command")
	}
	_ = cmd()
	if fake.trackCalls != 1 {
		t.Fatalf("expected tracking call, got %d", fake.trackCalls)
	}
}

func TestInstamartActiveOrderTrackingFailureKeepsOrders(t *testing.T) {
	location := &domaininstamart.Location{Lat: 12.9, Lng: 77.6}
	fake := &fakeInstamartService{
		orders:   domaininstamart.OrderHistory{Orders: []domaininstamart.OrderSummary{{OrderID: "order-1", Status: "CONFIRMED", Active: true, Location: location}}},
		trackErr: errWithSensitiveText("tracking failed for order 123456789"),
	}
	m := instamartModel{ctx: context.Background(), service: fake}

	msg := m.loadOrdersCmd(true)()
	trackingMsg, ok := msg.(instamartTrackingMsg)
	if !ok {
		t.Fatalf("expected tracking message, got %T", msg)
	}
	updated, _ := m.Update(trackingMsg)
	got := updated.(instamartModel)

	if got.screen != instamartScreenOrders {
		t.Fatalf("expected orders screen after tracking failure, got %v", got.screen)
	}
	if len(got.orders.Orders) != 1 {
		t.Fatalf("expected fetched orders to be preserved, got %#v", got.orders)
	}
	if !strings.Contains(got.err, "Tracking unavailable") {
		t.Fatalf("expected tracking error, got %q", got.err)
	}
	if strings.Contains(got.View(), "No matching orders found") {
		t.Fatal("tracking failure should not render an empty orders state when orders were fetched")
	}
}

func TestInstamartViewCartRequiresSelectedAddress(t *testing.T) {
	fake := &fakeInstamartService{cart: cartWithItems(nil)}
	m := instamartModel{ctx: context.Background(), service: fake, screen: instamartScreenHome}
	updated, cmd := m.handleHomeKey("c")
	if cmd != nil {
		t.Fatal("cart should not load before address selection")
	}
	if fake.getCartCalls != 0 {
		t.Fatal("service GetCart should not be called before address selection")
	}
	if !strings.Contains(updated.(instamartModel).err, "Choose a deployment address") {
		t.Fatalf("expected address error, got %q", updated.(instamartModel).err)
	}
}

func TestInstamartQuantityZeroSendsEmptyReplacementForLastItem(t *testing.T) {
	fake := &fakeInstamartService{}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{
		ctx:             context.Background(),
		service:         fake,
		selectedAddress: &address,
		intendedItems:   []domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 1}},
		selectedRow:     &productVariationRow{Variation: domaininstamart.ProductVariation{SpinID: "spin-milk"}},
		quantity:        0,
	}
	_, cmd := m.handleQuantityKey("enter")
	if cmd != nil {
		_ = cmd()
	} else {
		t.Fatal("expected update command")
	}
	if fake.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", fake.updateCalls)
	}
	if len(fake.updateInput.Items) != 0 {
		t.Fatalf("expected empty replacement list, got %+v", fake.updateInput.Items)
	}
}

func checkoutConfirmModel(fake *fakeInstamartService) instamartModel {
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	return instamartModel{
		ctx:             context.Background(),
		service:         fake,
		screen:          instamartScreenCheckoutConfirm,
		selectedAddress: &address,
		currentCart: domaininstamart.Cart{
			AddressID:               "addr-1",
			Items:                   []domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk", Quantity: 1, FinalPrice: 60}},
			Bill:                    domaininstamart.BillBreakdown{ToPayValue: "Rs 80", ToPayRupees: 80},
			AvailablePaymentMethods: []string{"Cash"},
		},
		intendedItems: []domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 1}},
		paymentMethod: "Cash",
		reviewedCart: &domaininstamart.CartReviewSnapshot{
			AddressID:     "addr-1",
			Items:         []domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 1}},
			ToPayRupees:   80,
			PaymentMethod: "Cash",
		},
	}
}

func cartWithItems(items []domaininstamart.CartItem) domaininstamart.Cart {
	return domaininstamart.Cart{
		AddressID:               "addr-1",
		AddressLabel:            "Home",
		Items:                   items,
		Bill:                    domaininstamart.BillBreakdown{ToPayLabel: "To Pay", ToPayValue: "Rs 100", ToPayRupees: 100},
		TotalRupees:             100,
		AvailablePaymentMethods: []string{"Cash"},
	}
}

func productSearchResult(spinID, name string) domaininstamart.ProductSearchResult {
	return domaininstamart.ProductSearchResult{Products: []domaininstamart.Product{{
		DisplayName: name,
		InStock:     true,
		Available:   true,
		Variations: []domaininstamart.ProductVariation{{
			SpinID:              spinID,
			DisplayName:         name,
			QuantityDescription: "1 L",
			Price:               domaininstamart.Price{OfferPrice: 60},
			InStock:             true,
		}},
	}}}
}

func renderedLineIndex(out, needle string) int {
	for i, line := range strings.Split(strings.TrimSuffix(out, "\r\n"), "\r\n") {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}

type fakeInstamartService struct {
	searchInput    appinstamart.SearchProductsInput
	updateInput    appinstamart.UpdateCartInput
	checkoutInput  appinstamart.CheckoutInput
	addressUserID  string
	searchResult   domaininstamart.ProductSearchResult
	cart           domaininstamart.Cart
	updateCart     *domaininstamart.Cart
	getCartErr     error
	orders         domaininstamart.OrderHistory
	tracking       domaininstamart.TrackingStatus
	trackErr       error
	checkoutResult domaininstamart.CheckoutResult
	updateCalls    int
	getCartCalls   int
	checkoutCalls  int
	trackCalls     int
	addressCalls   int
}

func (f *fakeInstamartService) GetAddresses(ctx context.Context) ([]domaininstamart.Address, error) {
	f.addressCalls++
	f.addressUserID, _ = domainauth.UserIDFromContext(ctx)
	return []domaininstamart.Address{{ID: "addr-1", Label: "Home", DisplayLine: "Test address", PhoneMasked: "****0001"}}, nil
}

func (f *fakeInstamartService) SearchProducts(_ context.Context, input appinstamart.SearchProductsInput) (domaininstamart.ProductSearchResult, error) {
	f.searchInput = input
	return f.searchResult, nil
}

func (f *fakeInstamartService) GetGoToItems(context.Context, appinstamart.GetGoToItemsInput) (domaininstamart.ProductSearchResult, error) {
	return domaininstamart.ProductSearchResult{}, nil
}

func (f *fakeInstamartService) GetCart(context.Context) (domaininstamart.Cart, error) {
	f.getCartCalls++
	if f.getCartErr != nil {
		return domaininstamart.Cart{}, f.getCartErr
	}
	return f.cart, nil
}

func (f *fakeInstamartService) UpdateCart(_ context.Context, input appinstamart.UpdateCartInput) (domaininstamart.Cart, error) {
	f.updateCalls++
	f.updateInput = input
	if f.updateCart != nil {
		return *f.updateCart, nil
	}
	return f.cart, nil
}

func (f *fakeInstamartService) Checkout(_ context.Context, input appinstamart.CheckoutInput) (domaininstamart.CheckoutResult, error) {
	f.checkoutCalls++
	f.checkoutInput = input
	return f.checkoutResult, nil
}

func (f *fakeInstamartService) GetOrders(_ context.Context, input appinstamart.GetOrdersInput) (domaininstamart.OrderHistory, error) {
	return f.orders, nil
}

func (f *fakeInstamartService) TrackOrder(context.Context, appinstamart.TrackOrderInput) (domaininstamart.TrackingStatus, error) {
	f.trackCalls++
	return f.tracking, f.trackErr
}

func (f *fakeInstamartService) anyCalls() bool {
	return f.updateCalls > 0 || f.checkoutCalls > 0 || f.trackCalls > 0 || f.getCartCalls > 0 || f.searchInput.Query != ""
}

type errWithSensitiveText string

func (e errWithSensitiveText) Error() string { return string(e) }

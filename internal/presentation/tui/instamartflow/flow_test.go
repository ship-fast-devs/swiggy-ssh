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
	if !strings.Contains(got.err, "Choose address_id") {
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
	if !strings.Contains(view, "live preview · up/down selects · enter edits quantity") || strings.Contains(view, "searching...") {
		t.Fatalf("expected preview rendering without searching copy, got %q", got.View())
	}
	if strings.Contains(view, "grep products: milk") {
		t.Fatalf("query should only render in the input line, got %q", view)
	}
	if !strings.Contains(view, cursorStyle.Render("> ")) || !strings.Contains(view, "# item") {
		t.Fatalf("preview must render selectable rows, got %q", view)
	}
	if !strings.Contains(view, "200 OK · 32ms") || strings.Contains(view, "matches in") {
		t.Fatalf("expected timing beside API status only, got %q", got.View())
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

	for _, want := range []string{"Instamart shell", "grep groceries", "git add recent", "cart diff", "git log orders", "tail -f order", "context address_id: addr-1", "env=instamart  auth=ok  cart=3  mode=home"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in home output", want)
		}
	}
	for _, old := range []string{"Search products", "Your go-to items", "View cart", "Track active order", "tail active order", "deploy history", "Order history", "Change address", "switch target address", "Cancel order help", "Delivering to", "target locked", "deploying to:", "Cart:", "grep products", "staged cart"} {
		if strings.Contains(out, old) {
			t.Fatalf("old copy %q should not be rendered", old)
		}
	}
}

func TestInstamartSearchPreviewLoadingUsesScanningIndexCopy(t *testing.T) {
	m := instamartModel{screen: instamartScreenSearchInput, searchQuery: "milk", searchPreviewLoading: true}
	out := m.View()

	if !strings.Contains(out, "grep groceries") || !strings.Contains(out, "calling...") {
		t.Fatalf("expected API call status, got %q", out)
	}
	if !strings.Contains(out, "grep groceries --live...") {
		t.Fatalf("expected search endpoint loader, got %q", out)
	}
	if strings.Contains(out, "searching...") || strings.Contains(out, "Searching") {
		t.Fatalf("live preview loader must not use search copy, got %q", out)
	}
}

func TestInstamartSearchPreviewDebounceRendersActiveCopy(t *testing.T) {
	m := instamartModel{screen: instamartScreenSearchInput, searchQuery: "milk", searchPreviewDebouncing: true}
	out := m.View()

	if !strings.Contains(out, "grep groceries") {
		t.Fatalf("expected API template, got %q", out)
	}
	if !strings.Contains(out, "debounce: indexing pantry...") {
		t.Fatalf("expected debounce active copy, got %q", out)
	}
	if strings.Contains(out, "grep groceries --live...") {
		t.Fatalf("debounce copy should render before loader, got %q", out)
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

func TestInstamartSearchPreviewLoadedShowsOKStatus(t *testing.T) {
	m := instamartModel{screen: instamartScreenSearchInput, searchQuery: "milk", searchPreviewQuery: "milk", searchPreviewLoaded: true, searchPreviewElapsed: 32 * time.Millisecond}
	out := m.View()

	if !strings.Contains(out, "grep groceries") || !strings.Contains(out, "200 OK · 32ms") {
		t.Fatalf("expected successful API status, got %q", out)
	}
	if strings.Contains(out, "matches in") {
		t.Fatalf("preview summary should not include latency, got %q", out)
	}
}

func TestInstamartSearchEnterUsesCurrentPreviewSelection(t *testing.T) {
	result := productSearchResult("spin-milk", "Milk")
	m := instamartModel{
		screen:                instamartScreenSearchInput,
		searchQuery:           "milk",
		searchPreviewQuery:    "milk",
		searchPreviewProducts: result.Products,
		searchPreviewRows:     flattenProductRows(result.Products),
		searchPreviewLoaded:   true,
	}

	updated, cmd := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("current preview should open without another search")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenSearchInput || !got.quantityModalOpen || got.selectedRow == nil || got.selectedRow.Variation.SpinID != "spin-milk" || got.returnAfterCartUpdate != instamartScreenSearchInput {
		t.Fatalf("expected quantity modal from preview, got screen=%v modal=%v row=%+v return=%v", got.screen, got.quantityModalOpen, got.selectedRow, got.returnAfterCartUpdate)
	}
}

func TestInstamartSearchTypingCAppendsQueryAndDoesNotOpenCart(t *testing.T) {
	fake := &fakeInstamartService{}
	m := instamartModel{ctx: context.Background(), service: fake, screen: instamartScreenSearchInput, searchQuery: "mil"}

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := updated.(instamartModel)
	if got.screen != instamartScreenSearchInput || got.searchQuery != "milc" || fake.getCartCalls != 0 {
		t.Fatalf("expected c to edit query only, query=%q screen=%v getCart=%d", got.searchQuery, got.screen, fake.getCartCalls)
	}
	if cmd == nil {
		t.Fatal("typing c should still queue live search preview")
	}
}

func TestInstamartSearchViewRendersQuantityModalOverlay(t *testing.T) {
	result := productSearchResult("spin-milk", "Milk")
	rows := flattenProductRows(result.Products)
	m := instamartModel{
		screen:                instamartScreenSearchInput,
		searchQuery:           "milk",
		searchPreviewQuery:    "milk",
		searchPreviewProducts: result.Products,
		searchPreviewRows:     rows,
		searchPreviewLoaded:   true,
		quantityModalOpen:     true,
		selectedRow:           &rows[0],
		quantity:              2,
	}

	out := m.View()
	for _, want := range []string{"pattern:", "milk", "live preview", "Add to cart", "Quantity:        [-]  2  [+]", "+/- qty"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected overlay view to contain %q, got %q", want, out)
		}
	}
}

func TestInstamartQuantityModalEscPreservesSearchContext(t *testing.T) {
	result := productSearchResult("spin-milk", "Milk")
	rows := flattenProductRows(result.Products)
	m := instamartModel{
		screen:              instamartScreenSearchInput,
		searchQuery:         "milk",
		searchPreviewQuery:  "milk",
		searchPreviewRows:   rows,
		searchPreviewLoaded: true,
		cursor:              0,
		quantityModalOpen:   true,
		selectedRow:         &rows[0],
		quantity:            2,
	}

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("esc should only close modal")
	}
	got := updated.(instamartModel)
	if got.quantityModalOpen || got.screen != instamartScreenSearchInput || got.searchQuery != "milk" || got.searchPreviewQuery != "milk" || len(got.searchPreviewRows) != 1 || got.cursor != 0 {
		t.Fatalf("expected preserved search after modal close, got screen=%v modal=%v query=%q rows=%d cursor=%d", got.screen, got.quantityModalOpen, got.searchQuery, len(got.searchPreviewRows), got.cursor)
	}
	if got.selectedRow == nil || got.selectedRow.Variation.SpinID != "spin-milk" {
		t.Fatalf("expected selected row to remain available, got %+v", got.selectedRow)
	}
}

func TestInstamartSearchCtrlShortcutsOpenCartAndHome(t *testing.T) {
	fake := &fakeInstamartService{}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{ctx: context.Background(), service: fake, screen: instamartScreenSearchInput, selectedAddress: &address, searchQuery: "milk"}

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlK})
	if cmd == nil {
		t.Fatal("ctrl+k should load cart diff")
	}
	if updated.(instamartModel).screen != instamartScreenLoading || updated.(instamartModel).loading != "cart diff..." {
		t.Fatalf("expected cart diff loading, got screen=%v loading=%q", updated.(instamartModel).screen, updated.(instamartModel).loading)
	}

	updated, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	if cmd != nil {
		t.Fatal("ctrl+b home should not call service")
	}
	if updated.(instamartModel).screen != instamartScreenHome {
		t.Fatalf("expected ctrl+b to go home, got %v", updated.(instamartModel).screen)
	}
}

func TestInstamartSearchPreviewArrowSelectsRows(t *testing.T) {
	rows := []productVariationRow{
		{Product: domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true}, Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", InStock: true}},
		{Product: domaininstamart.Product{DisplayName: "Curd", InStock: true, Available: true}, Variation: domaininstamart.ProductVariation{SpinID: "spin-curd", DisplayName: "Curd", InStock: true}},
	}
	m := instamartModel{screen: instamartScreenSearchInput, searchQuery: "m", searchPreviewQuery: "m", searchPreviewRows: rows, searchPreviewLoaded: true}

	updated, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(instamartModel)
	if got.cursor != 1 {
		t.Fatalf("expected down to select second row, got cursor=%d", got.cursor)
	}
	updated, _ = got.handleSearchKey(tea.KeyMsg{Type: tea.KeyUp})
	got = updated.(instamartModel)
	if got.cursor != 0 {
		t.Fatalf("expected up to select first row, got cursor=%d", got.cursor)
	}
}

func TestInstamartQuantityEnterFromSearchReturnsToSearchWithQuery(t *testing.T) {
	fake := &fakeInstamartService{cart: cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk", Quantity: 2, FinalPrice: 120}})}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	result := productSearchResult("spin-milk", "Milk")
	m := instamartModel{
		ctx:                   context.Background(),
		service:               fake,
		screen:                instamartScreenSearchInput,
		selectedAddress:       &address,
		searchQuery:           "milk",
		searchPreviewQuery:    "milk",
		searchPreviewProducts: result.Products,
		searchPreviewRows:     flattenProductRows(result.Products),
		searchPreviewLoaded:   true,
	}

	updated, cmd := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter on preview should open quantity without service call")
	}
	popup := updated.(instamartModel)
	popup.quantity = 2
	updated, cmd = popup.handleQuantityKey("enter")
	if cmd == nil {
		t.Fatal("quantity enter should update cart")
	}
	loading := updated.(instamartModel)
	msg := cmd()
	updated, _ = loading.Update(msg)
	got := updated.(instamartModel)
	if got.screen != instamartScreenSearchInput || got.searchQuery != "milk" {
		t.Fatalf("expected return to preserved search, got screen=%v query=%q", got.screen, got.searchQuery)
	}
	if fake.updateInput.Items[0].SpinID != "spin-milk" || fake.updateInput.Items[0].Quantity != 2 {
		t.Fatalf("unexpected preview update: %+v", fake.updateInput.Items)
	}
}

func TestInstamartFastAddMergesFreshProviderCart(t *testing.T) {
	fake := &fakeInstamartService{cart: cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-a", Name: "Existing", Quantity: 2, FinalPrice: 100}})}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	result := productSearchResult("spin-b", "Bread")
	m := instamartModel{
		ctx:                   context.Background(),
		service:               fake,
		screen:                instamartScreenSearchInput,
		selectedAddress:       &address,
		searchQuery:           "bread",
		searchPreviewQuery:    "bread",
		searchPreviewProducts: result.Products,
		searchPreviewRows:     flattenProductRows(result.Products),
		searchPreviewLoaded:   true,
	}

	updated, cmd := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter on preview should open quantity")
	}
	popup := updated.(instamartModel)
	_, cmd = popup.handleQuantityKey("enter")
	if cmd == nil {
		t.Fatal("quantity enter should update cart")
	}
	_ = cmd()
	if fake.getCartCalls == 0 {
		t.Fatal("fast add must load the provider cart before first full-cart update")
	}
	if len(fake.updateInput.Items) != 2 {
		t.Fatalf("expected existing plus new item, got %+v", fake.updateInput.Items)
	}
	if fake.updateInput.Items[0].SpinID != "spin-a" || fake.updateInput.Items[0].Quantity != 2 {
		t.Fatalf("expected existing item to be preserved, got %+v", fake.updateInput.Items[0])
	}
	if fake.updateInput.Items[1].SpinID != "spin-b" || fake.updateInput.Items[1].Quantity != 1 {
		t.Fatalf("expected new item to be added, got %+v", fake.updateInput.Items[1])
	}
}

func TestInstamartCartUpdateReturnsToShoppingContext(t *testing.T) {
	m := instamartModel{screen: instamartScreenLoading, returnAfterCartUpdate: instamartScreenProductList}
	updated, _ := m.Update(instamartCartMsg{cart: cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk", Quantity: 1, FinalPrice: 60}}), action: "git add groceries", returnTo: instamartScreenProductList})
	got := updated.(instamartModel)
	if got.screen != instamartScreenProductList {
		t.Fatalf("expected product context after add, got %v", got.screen)
	}
	if !strings.Contains(got.status, "git add groceries") || got.sessionCartCount() != 1 {
		t.Fatalf("expected git-add status and cart count, got status=%q count=%d", got.status, got.sessionCartCount())
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
	if loading.screen != instamartScreenLoading || loading.loading != "GET /instamart/search calling..." {
		t.Fatalf("expected committed search loader, got screen=%v loading=%q", loading.screen, loading.loading)
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
	if !strings.Contains(buf.String(), "Choose address_id from the main menu") {
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
	if !strings.Contains(buf.String(), "grep groceries") || !strings.Contains(buf.String(), "addr-1") {
		t.Fatalf("expected selected session address to start in search shell, got %q", buf.String())
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
	for _, want := range []string{"# item", "1", "[ad]", "Sandwich Bread", "400 g", "Rs 49"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in product output", want)
		}
	}
	if !strings.Contains(out, "git add from recent cache") || strings.Contains(out, "409") {
		t.Fatalf("product output should render successful API status only: %q", out)
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

func TestInstamartQuantityRendersAddToCartModal(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenQuantity,
		selectedRow: &productVariationRow{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", QuantityDescription: "1 L", Price: domaininstamart.Price{OfferPrice: 60}, InStock: true},
		},
		quantity: 2,
	}
	out := m.View()
	for _, want := range []string{"Add to cart", "Milk", "Pack: 1 L", "₹60", "Status:", "available", "Quantity:        [-]  2  [+]", "enter add/update", "esc cancel", "+/- qty"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in quantity output", want)
		}
	}
	for _, old := range []string{"POST /instamart/cart/items draft", "spin_id:", "item:", "pack:", "price:", "quantity:", "action: stage item", "stage item", "full intended cart"} {
		if strings.Contains(out, old) {
			t.Fatalf("quantity modal should not render old API/YAML copy %q: %q", old, out)
		}
	}
	if strings.Contains(out, "200") || strings.Contains(out, "409") {
		t.Fatalf("quantity output should not render pseudo HTTP status codes: %q", out)
	}
}

func TestInstamartQuantityZeroRendersRemoveCopy(t *testing.T) {
	m := instamartModel{
		screen: instamartScreenQuantity,
		selectedRow: &productVariationRow{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", QuantityDescription: "1 L", Price: domaininstamart.Price{OfferPrice: 60}, InStock: true},
		},
		quantity: 0,
	}
	out := m.View()
	for _, want := range []string{"Quantity:        [-]  0  [+]", "0 means remove this item", "enter remove", "esc cancel"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in zero-quantity output", want)
		}
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
		t.Fatal("selecting a product row should not update cart before quantity confirmation")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenProductList || !got.quantityModalOpen || got.selectedRow == nil || got.selectedRow.Variation.SpinID != "spin-b" {
		t.Fatalf("expected visible shortcut 1 to select global row 2, got screen=%v modal=%v row=%+v", got.screen, got.quantityModalOpen, got.selectedRow)
	}
	if out := m.View(); !strings.Contains(out, "showing 2-10 of 12") || strings.Contains(out, "10  ") {
		t.Fatalf("expected paged visible numbering, got %q", out)
	}
}

func TestInstamartProductShortcutOpensQuantityBeforeUpdate(t *testing.T) {
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
		t.Fatal("product shortcut should open quantity without updating cart")
	}
	selected := updated.(instamartModel)
	if selected.screen != instamartScreenProductList || !selected.quantityModalOpen || selected.selectedRow == nil || selected.quantity != 1 {
		t.Fatalf("expected quantity modal with selected product, got screen=%v modal=%v row=%+v qty=%d", selected.screen, selected.quantityModalOpen, selected.selectedRow, selected.quantity)
	}
	selected.quantity = 3
	updated, cmd = selected.handleQuantityKey("enter")
	if cmd == nil {
		t.Fatal("quantity confirmation should update cart")
	}
	_ = cmd()
	if fake.updateCalls != 1 {
		t.Fatalf("expected one update call, got %d", fake.updateCalls)
	}
	if updated.(instamartModel).screen != instamartScreenLoading {
		t.Fatalf("expected loading while quantity update runs, got %v", updated.(instamartModel).screen)
	}
	if len(fake.updateInput.Items) != 1 || fake.updateInput.Items[0].SpinID != "spin-milk" || fake.updateInput.Items[0].Quantity != 3 {
		t.Fatalf("unexpected update items: %+v", fake.updateInput.Items)
	}
}

func TestInstamartProductEnterOpensQuantityBeforeUpdate(t *testing.T) {
	fake := &fakeInstamartService{}
	m := instamartModel{
		service: fake,
		screen:  instamartScreenProductList,
		rows: []productVariationRow{{
			Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
			Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", InStock: true},
		}},
	}

	updated, cmd := m.handleProductKey("enter")
	if cmd != nil || fake.updateCalls != 0 {
		t.Fatal("enter on product list must not update cart")
	}
	got := updated.(instamartModel)
	if got.screen != instamartScreenProductList || !got.quantityModalOpen || got.selectedRow == nil || got.selectedRow.Variation.SpinID != "spin-milk" {
		t.Fatalf("expected enter to open quantity modal for selected row, got screen=%v modal=%v row=%+v", got.screen, got.quantityModalOpen, got.selectedRow)
	}
}

func TestInstamartProductPlusMinusOpenQuantityFromExistingQuantity(t *testing.T) {
	row := productVariationRow{
		Product:   domaininstamart.Product{DisplayName: "Milk", InStock: true, Available: true},
		Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", DisplayName: "Milk", InStock: true},
	}

	plusNew, cmd := (instamartModel{screen: instamartScreenProductList, rows: []productVariationRow{row}}).handleProductKey("+")
	if cmd != nil {
		t.Fatal("plus should open quantity without updating cart")
	}
	if got := plusNew.(instamartModel); got.screen != instamartScreenProductList || !got.quantityModalOpen || got.quantity != 1 {
		t.Fatalf("expected plus on new item to open quantity 1, got screen=%v modal=%v quantity=%d", got.screen, got.quantityModalOpen, got.quantity)
	}

	minusNew, cmd := (instamartModel{screen: instamartScreenProductList, rows: []productVariationRow{row}}).handleProductKey("-")
	if cmd != nil {
		t.Fatal("minus should open quantity without updating cart")
	}
	if got := minusNew.(instamartModel); got.screen != instamartScreenProductList || !got.quantityModalOpen || got.quantity != 1 {
		t.Fatalf("expected minus on new item to open quantity 1, got screen=%v modal=%v quantity=%d", got.screen, got.quantityModalOpen, got.quantity)
	}

	plusExisting, cmd := (instamartModel{screen: instamartScreenProductList, rows: []productVariationRow{row}, intendedItems: []domaininstamart.CartUpdateItem{{SpinID: "spin-milk", Quantity: 2}}}).handleProductKey("+")
	if cmd != nil {
		t.Fatal("plus should open quantity without updating cart")
	}
	if got := plusExisting.(instamartModel); got.screen != instamartScreenProductList || !got.quantityModalOpen || got.quantity != 3 {
		t.Fatalf("expected plus on quantity 2 to open quantity 3, got screen=%v modal=%v quantity=%d", got.screen, got.quantityModalOpen, got.quantity)
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
	for _, want := range []string{"cart diff", "context", "response.items", "response.bill", "available_payment_methods", "next", "ship cart", "Work", "2x", "Milk 1 L", "Item Total", "Coupon Discount", "To Pay", "Rs 100", "Cash", "p/enter ship cart", "risk: cart spans 2 stores"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in cart review", want)
		}
	}
	for _, noisy := range []string{"p/enter deploy", "j/k scroll", "working tree clean"} {
		if strings.Contains(out, noisy) {
			t.Fatalf("did not expect %q in non-overflowing cart review", noisy)
		}
	}
	if !strings.Contains(out, "38;2;0;170;68") || !strings.Contains(out, "38;2;255;68;68") {
		t.Fatal("expected green plus and red minus diff markers")
	}
	if !strings.Contains(out, "48;") {
		t.Fatal("expected add/remove row backgrounds")
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

func TestInstamartEmptyCartReviewUsesEmptyItemsResponse(t *testing.T) {
	m := instamartModel{screen: instamartScreenCartReview, currentCart: domaininstamart.Cart{AvailablePaymentMethods: []string{"Cash"}}}
	out := m.View()
	if !strings.Contains(out, "response.items") || !strings.Contains(out, "[]") || strings.Contains(out, "working tree clean") {
		t.Fatalf("expected empty items response, got %q", out)
	}
}

func TestInstamartStableFrameHeightAndBodyAnchor(t *testing.T) {
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{screen: instamartScreenHome, selectedAddress: &address, viewport: Viewport{Width: 80, Height: 24}}
	out := m.View()
	if got := strings.Count(out, "\r\n"); got != 24 {
		t.Fatalf("expected 80x24 frame height, got %d lines: %q", got, out)
	}
	bodyLine := renderedLineIndex(out, "grep groceries")
	if bodyLine < 0 {
		t.Fatalf("expected home body anchor, got %q", out)
	}
	m.status = "loaded"
	m.err = "blocked"
	withSlots := m.View()
	if got := strings.Count(withSlots, "\r\n"); got != 24 {
		t.Fatalf("expected status/error frame height to stay fixed, got %d lines", got)
	}
	if renderedLineIndex(withSlots, "grep groceries") != bodyLine {
		t.Fatalf("body anchor moved after status/error slots: before=%d after=%d", bodyLine, renderedLineIndex(withSlots, "grep groceries"))
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

func TestInstamartCheckoutConfirmRendersRequestGate(t *testing.T) {
	m := checkoutConfirmModel(&fakeInstamartService{})
	out := m.View()
	for _, want := range []string{"Ship cart?", "body address_id=addr-1", "body payment_method=Cash", "total=Rs 80", "this places the reviewed Instamart cart", "y confirm / n cancel"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in checkout confirmation", want)
		}
	}
	for _, noisy := range []string{"REAL SWIGGY ORDER", "places a paid Instamart order", "[ok] address selected", "press y to confirm order", "ship order", "push --force", "git push", "deploy"} {
		if strings.Contains(out, noisy) {
			t.Fatalf("checkout gate should not render noisy copy %q: %q", noisy, out)
		}
	}
}

func TestInstamartCheckoutConfirmFooterUsesRequestCopy(t *testing.T) {
	m := checkoutConfirmModel(&fakeInstamartService{})
	footer := m.footer()
	for _, want := range []string{"y confirm", "n cancel"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("expected %q in footer, got %q", want, footer)
		}
	}
	if strings.Contains(footer, "confirm order") || strings.Contains(footer, "help") || strings.Contains(footer, "deploy") {
		t.Fatalf("checkout footer should stay low-clutter, got %q", footer)
	}
}

func TestInstamartOrderResultRendersCheckoutResponse(t *testing.T) {
	m := instamartModel{screen: instamartScreenOrderResult, checkoutElapsed: 92 * time.Second, checkoutResult: domaininstamart.CheckoutResult{Message: "Instamart order placed successfully!", Status: "confirmed", PaymentMethod: "Cash", OrderIDs: []string{"order-1"}, CartTotal: 80}}
	out := m.View()
	for _, want := range []string{"ship cart 201 Created", "response.message:", "Instamart order placed successfully!", "response.payment_method: Cash", "response.status: confirmed", "response.order_id: order-1", "response.stores: 1", "response.total: Rs 80", "response.elapsed: 1m 32s", "tail -f order"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in checkout response", want)
		}
	}
	if strings.Contains(out, "git push") || strings.Contains(out, "deploy logs") {
		t.Fatalf("checkout response should not render old deploy logs, got %q", out)
	}
}

func TestInstamartOperationTimingStatus(t *testing.T) {
	m := instamartModel{screen: instamartScreenLoadingAddresses}
	updated, _ := m.Update(instamartAddressesMsg{addresses: []domaininstamart.Address{{ID: "addr-1", Label: "Home"}}, elapsed: 24 * time.Millisecond})
	if !strings.Contains(updated.(instamartModel).status, "loaded addresses in 24ms") {
		t.Fatalf("expected address timing, got %q", updated.(instamartModel).status)
	}

	updated, _ = instamartModel{screen: instamartScreenLoading}.Update(instamartCartMsg{cart: cartWithItems(nil), action: "GET /instamart/cart 200 OK", elapsed: 1100 * time.Millisecond})
	if !strings.Contains(updated.(instamartModel).status, "GET /instamart/cart 200 OK in 1.1s") {
		t.Fatalf("expected cart timing, got %q", updated.(instamartModel).status)
	}

	updated, _ = instamartModel{screen: instamartScreenLoading}.Update(instamartCheckoutMsg{result: domaininstamart.CheckoutResult{Message: "ok"}, elapsed: 2*time.Minute + 3*time.Second})
	if !strings.Contains(updated.(instamartModel).status, "ship cart 201 Created") {
		t.Fatalf("expected checkout status, got %q", updated.(instamartModel).status)
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
	if help.screen != instamartScreenHelp || !strings.Contains(help.View(), "swiggy.dev keys") || !strings.Contains(help.View(), "/          grep groceries") {
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

func TestInstamartCheckoutAutoTailsActiveOrderWhenTrackable(t *testing.T) {
	location := &domaininstamart.Location{Lat: 12.9, Lng: 77.6}
	fake := &fakeInstamartService{
		checkoutResult: domaininstamart.CheckoutResult{Message: "shipped", OrderIDs: []string{"order-new"}},
		orders: domaininstamart.OrderHistory{Orders: []domaininstamart.OrderSummary{
			{OrderID: "order-old", Active: true, Location: location},
			{OrderID: "order-new", Active: true, Location: location},
		}},
		tracking: domaininstamart.TrackingStatus{StatusMessage: "Order is getting packed"},
	}
	m := checkoutConfirmModel(fake)

	msg := m.checkoutCmd()()
	updated, _ := m.Update(msg)
	got := updated.(instamartModel)
	if got.screen != instamartScreenTracking {
		t.Fatalf("expected checkout to tail active tracking, got %v", got.screen)
	}
	if fake.checkoutCalls != 1 || fake.trackCalls != 1 {
		t.Fatalf("expected checkout and tracking calls, got checkout=%d track=%d", fake.checkoutCalls, fake.trackCalls)
	}
	if fake.trackInput.OrderID != "order-new" {
		t.Fatalf("expected checkout order to be tracked, got %q", fake.trackInput.OrderID)
	}
	if !strings.Contains(got.status, "tail -f order") || got.tracking.StatusMessage == "" {
		t.Fatalf("expected tail status and tracking payload, got status=%q tracking=%+v", got.status, got.tracking)
	}
}

func TestInstamartCheckoutDoesNotAutoTailUnmatchedCheckoutOrderIDs(t *testing.T) {
	location := &domaininstamart.Location{Lat: 12.9, Lng: 77.6}
	fake := &fakeInstamartService{
		checkoutResult: domaininstamart.CheckoutResult{Message: "shipped", OrderIDs: []string{"order-new"}},
		orders: domaininstamart.OrderHistory{Orders: []domaininstamart.OrderSummary{
			{OrderID: "order-old", Active: true, Location: location},
		}},
		tracking: domaininstamart.TrackingStatus{StatusMessage: "Order is getting packed"},
	}
	m := checkoutConfirmModel(fake)

	msg := m.checkoutCmd()()
	updated, _ := m.Update(msg)
	got := updated.(instamartModel)
	if got.screen != instamartScreenOrderResult {
		t.Fatalf("expected checkout result when checkout order is not in active history, got %v", got.screen)
	}
	if fake.trackCalls != 0 {
		t.Fatalf("must not auto-track unrelated active order, got %d track calls", fake.trackCalls)
	}
	if got.checkoutResult.OrderIDs[0] != "order-new" {
		t.Fatalf("expected checkout result to remain visible, got %+v", got.checkoutResult)
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
	if !strings.Contains(updated.(instamartModel).err, "Cart address_id no longer matches") {
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
	if !strings.Contains(updated.(instamartModel).err, "Choose address_id") {
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

func TestInstamartFreshAddFallsBackWhenInitialCartReadFails(t *testing.T) {
	updatedCart := cartWithItems([]domaininstamart.CartItem{{SpinID: "spin-milk", Name: "Milk", Quantity: 1, FinalPrice: 60}})
	fake := &fakeInstamartService{
		getCartErr: errors.New("cart not found"),
		updateCart: &updatedCart,
	}
	address := domaininstamart.Address{ID: "addr-1", Label: "Home"}
	m := instamartModel{
		ctx:                   context.Background(),
		service:               fake,
		screen:                instamartScreenSearchInput,
		selectedAddress:       &address,
		quantityModalOpen:     true,
		selectedRow:           &productVariationRow{Variation: domaininstamart.ProductVariation{SpinID: "spin-milk", InStock: true}},
		quantity:              1,
		returnAfterCartUpdate: instamartScreenSearchInput,
	}

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected update command")
	}
	msg := cmd()
	gotModel, _ := updated.(instamartModel).Update(msg)
	got := gotModel.(instamartModel)

	if fake.updateCalls != 1 {
		t.Fatalf("expected direct update after failed cart pre-read, got %d calls", fake.updateCalls)
	}
	if len(fake.updateInput.Items) != 1 || fake.updateInput.Items[0].SpinID != "spin-milk" || fake.updateInput.Items[0].Quantity != 1 {
		t.Fatalf("expected selected item update, got %+v", fake.updateInput.Items)
	}
	if got.screen != instamartScreenSearchInput {
		t.Fatalf("expected success return to search input, got %v", got.screen)
	}
	if len(got.intendedItems) != 1 || got.intendedItems[0].SpinID != "spin-milk" || got.intendedItems[0].Quantity != 1 {
		t.Fatalf("expected updated intended cart, got %+v", got.intendedItems)
	}
	if !strings.Contains(got.status, "git add groceries") {
		t.Fatalf("expected success status, got %q", got.status)
	}
	if !strings.Contains(got.err, "Cart updated, but payment refresh failed") {
		t.Fatalf("expected post-update refresh warning, got %q", got.err)
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
	trackInput     appinstamart.TrackOrderInput
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

func (f *fakeInstamartService) TrackOrder(_ context.Context, input appinstamart.TrackOrderInput) (domaininstamart.TrackingStatus, error) {
	f.trackCalls++
	f.trackInput = input
	return f.tracking, f.trackErr
}

func (f *fakeInstamartService) anyCalls() bool {
	return f.updateCalls > 0 || f.checkoutCalls > 0 || f.trackCalls > 0 || f.getCartCalls > 0 || f.searchInput.Query != ""
}

type errWithSensitiveText string

func (e errWithSensitiveText) Error() string { return string(e) }

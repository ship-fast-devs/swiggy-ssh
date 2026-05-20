package foodflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	appfood "swiggy-ssh/internal/application/food"
	domainauth "swiggy-ssh/internal/domain/auth"
	domainfood "swiggy-ssh/internal/domain/food"

	tea "github.com/charmbracelet/bubbletea"
)

const foodSearchPreviewDebounce = 350 * time.Millisecond

var foodSearchSpinnerFrames = []string{"|", "/", "-", "\\"}

// FoodService is the application boundary used by the food terminal UI.
type FoodService interface {
	SearchRestaurants(ctx context.Context, input appfood.SearchRestaurantsInput) (domainfood.RestaurantSearchResult, error)
	GetRestaurantMenu(ctx context.Context, input appfood.GetMenuInput) (domainfood.MenuPage, error)
	SearchMenu(ctx context.Context, input appfood.SearchMenuInput) (domainfood.MenuSearchResult, error)
	UpdateCart(ctx context.Context, input appfood.UpdateCartInput) (domainfood.FoodCart, error)
	GetCart(ctx context.Context, input appfood.GetCartInput) (domainfood.FoodCart, error)
	FetchCoupons(ctx context.Context, input appfood.FetchCouponsInput) (domainfood.FoodCouponsResult, error)
	ApplyCoupon(ctx context.Context, input appfood.ApplyCouponInput) error
	PlaceOrder(ctx context.Context, input appfood.PlaceOrderInput) (domainfood.FoodOrderResult, error)
	GetOrders(ctx context.Context, input appfood.GetOrdersInput) (domainfood.FoodOrderHistory, error)
	GetOrderDetails(ctx context.Context, input appfood.GetOrderDetailsInput) (domainfood.FoodOrderDetails, error)
	TrackOrder(ctx context.Context, input appfood.TrackOrderInput) (domainfood.FoodTrackingStatus, error)
	FlushCart(ctx context.Context) error
	HandleCancellation(ctx context.Context) error
}

// FoodAppView renders the service-backed Food flow.
type FoodAppView struct {
	Service         FoodService
	UserID          string
	SelectedAddress domainfood.Address
	StatusMessage   string
	Viewport        Viewport
	In              io.Reader
}

type FoodAction int

const (
	FoodActionQuit FoodAction = iota
	FoodActionBackToHome
)

type FoodResult struct {
	Action          FoodAction
	SelectedAddress domainfood.Address
}

type foodScreen int

const (
	foodScreenHome foodScreen = iota
	foodScreenSearchInput
	foodScreenRestaurantList
	foodScreenMenuBrowse
	foodScreenMenuSearch
	foodScreenMenuResults
	foodScreenItemDetail
	foodScreenCartReview
	foodScreenCoupons
	foodScreenCheckoutConfirm
	foodScreenOrderResult
	foodScreenOrders
	foodScreenTracking
	foodScreenLoading
	foodScreenMessage
	foodScreenHelp
)

type foodHomeChoice struct {
	icon   string
	label  string
	action string
}

var foodHomeChoices = []foodHomeChoice{
	{icon: "⌕", label: "search restaurants", action: "search"},
	{icon: "⌕", label: "search dish", action: "dish"},
	{icon: "▦", label: "staged cart", action: "cart"},
	{icon: "✓", label: "order history", action: "orders"},
}

type foodModel struct {
	ctx      context.Context
	service  FoodService
	viewport Viewport
	screen   foodScreen
	backTo   foodScreen
	loading  string

	status  string
	message string
	err     string

	cursor     int
	homeCursor int

	selectedAddress *domainfood.Address

	searchQuery  string
	isDishSearch bool

	searchPreviewQuery       string
	searchPreviewRestaurants []domainfood.Restaurant
	searchPreviewMenuItems   []domainfood.MenuItemDetail
	searchPreviewLoading     bool
	searchPreviewLoaded      bool
	searchPreviewVersion     int
	searchPreviewSpinner     int
	searchPreviewErr         string
	searchPreviewElapsed     time.Duration

	restaurants        []domainfood.Restaurant
	selectedRestaurant *domainfood.Restaurant

	menuPage   domainfood.MenuPage
	menuCursor int

	menuItems    []domainfood.MenuItemDetail
	selectedItem *domainfood.MenuItemDetail

	variantCursors []int

	currentCart   domainfood.FoodCart
	intendedItems []domainfood.FoodCartUpdateItem
	cartScroll    int
	paymentMethod string
	reviewedCart  *domainfood.FoodCartReviewSnapshot

	coupons domainfood.FoodCouponsResult

	orderResult  domainfood.FoodOrderResult
	orderElapsed time.Duration

	orders   domainfood.FoodOrderHistory
	tracking domainfood.FoodTrackingStatus

	result FoodResult
}

// ── messages ─────────────────────────────────────────────────────────────────

type foodRestaurantsMsg struct {
	result  domainfood.RestaurantSearchResult
	preview bool
	query   string
	version int
	err     error
	elapsed time.Duration
}

type foodMenuMsg struct {
	page    domainfood.MenuPage
	err     error
	elapsed time.Duration
}

type foodMenuSearchMsg struct {
	result  domainfood.MenuSearchResult
	preview bool
	version int
	query   string
	err     error
	elapsed time.Duration
}

type foodCartMsg struct {
	cart       domainfood.FoodCart
	err        error
	refreshErr error
	action     string
	elapsed    time.Duration
}

type foodCouponsMsg struct {
	result  domainfood.FoodCouponsResult
	err     error
	elapsed time.Duration
}

type foodOrderResultMsg struct {
	result  domainfood.FoodOrderResult
	err     error
	elapsed time.Duration
}

type foodOrdersMsg struct {
	history    domainfood.FoodOrderHistory
	activeOnly bool
	err        error
	elapsed    time.Duration
}

type foodTrackingMsg struct {
	status  domainfood.FoodTrackingStatus
	err     error
	elapsed time.Duration
}

type foodSearchDebounceMsg struct {
	query   string
	version int
	isDish  bool
}

type foodSearchSpinnerMsg struct {
	version int
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m foodModel) Init() tea.Cmd {
	return ctxQuitCmd(m.ctx)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m foodModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	previousScreen := m.screen
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport = Viewport{Width: msg.Width, Height: msg.Height}
		return m, nil

	case tea.KeyMsg:
		updated, cmd := m.handleKey(msg)
		return foodClearOnScreenChange(previousScreen, updated, cmd)

	case foodRestaurantsMsg:
		if msg.preview {
			if m.screen != foodScreenSearchInput || msg.version != m.searchPreviewVersion || msg.query != m.searchQuery {
				return foodClearOnScreenChange(previousScreen, m, nil)
			}
			m.searchPreviewLoading = false
			m.searchPreviewQuery = msg.query
			if msg.err != nil {
				m.searchPreviewLoaded = false
				m.searchPreviewRestaurants = nil
				m.searchPreviewErr = foodDisplayErr("Could not search restaurants", msg.err)
				return foodClearOnScreenChange(previousScreen, m, nil)
			}
			m.searchPreviewLoaded = true
			m.searchPreviewRestaurants = msg.result.Restaurants
			m.searchPreviewElapsed = msg.elapsed
			m.searchPreviewErr = ""
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		if msg.err != nil {
			m.screen = foodScreenHome
			m.err = foodDisplayErr("Could not search restaurants", msg.err)
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.restaurants = msg.result.Restaurants
		m.cursor = 0
		m.screen = foodScreenRestaurantList
		m.status = fmt.Sprintf("found %d restaurants in %s", len(m.restaurants), formatElapsed(msg.elapsed))
		if len(m.restaurants) == 0 {
			m.screen = foodScreenHome
			m.status = "No matching restaurants found. Try another search."
		}
		return foodClearOnScreenChange(previousScreen, m, nil)

	case foodMenuMsg:
		if msg.err != nil {
			m.screen = foodScreenRestaurantList
			m.err = foodDisplayErr("Could not load menu", msg.err)
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.menuPage = msg.page
		m.menuCursor = 0
		m.screen = foodScreenMenuBrowse
		m.status = fmt.Sprintf("loaded menu in %s", formatElapsed(msg.elapsed))
		return foodClearOnScreenChange(previousScreen, m, nil)

	case foodMenuSearchMsg:
		if msg.preview {
			if m.screen != foodScreenMenuSearch || msg.version != m.searchPreviewVersion || msg.query != m.searchQuery {
				return foodClearOnScreenChange(previousScreen, m, nil)
			}
			m.searchPreviewLoading = false
			m.searchPreviewQuery = msg.query
			if msg.err != nil {
				m.searchPreviewLoaded = false
				m.searchPreviewMenuItems = nil
				m.searchPreviewErr = foodDisplayErr("Could not search dishes", msg.err)
				return foodClearOnScreenChange(previousScreen, m, nil)
			}
			m.searchPreviewLoaded = true
			m.searchPreviewMenuItems = msg.result.Items
			m.searchPreviewElapsed = msg.elapsed
			m.searchPreviewErr = ""
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		if msg.err != nil {
			m.screen = foodScreenHome
			m.err = foodDisplayErr("Could not search dishes", msg.err)
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.menuItems = msg.result.Items
		m.cursor = 0
		m.screen = foodScreenMenuResults
		m.status = fmt.Sprintf("found %d dishes in %s", len(m.menuItems), formatElapsed(msg.elapsed))
		if len(m.menuItems) == 0 {
			m.screen = foodScreenHome
			m.status = "No matching dishes found. Try another search."
		}
		return foodClearOnScreenChange(previousScreen, m, nil)

	case foodCartMsg:
		if msg.err != nil {
			m.screen = foodScreenHome
			m.err = foodDisplayErr("Could not update cart", msg.err)
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.applyCart(msg.cart)
		m.screen = foodScreenCartReview
		m.cursor = 0
		m.cartScroll = 0
		m.status = msg.action + " in " + formatElapsed(msg.elapsed)
		if msg.refreshErr != nil {
			m.err = foodDisplayErr("Cart staged, but payment refresh failed", msg.refreshErr)
		} else {
			m.err = ""
		}
		return foodClearOnScreenChange(previousScreen, m, nil)

	case foodCouponsMsg:
		if msg.err != nil {
			m.screen = foodScreenCartReview
			m.err = foodDisplayErr("Could not load coupons", msg.err)
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.coupons = msg.result
		m.screen = foodScreenCoupons
		m.cursor = 0
		m.status = fmt.Sprintf("loaded %d coupons in %s", len(msg.result.Coupons), formatElapsed(msg.elapsed))
		return foodClearOnScreenChange(previousScreen, m, nil)

	case foodOrderResultMsg:
		if msg.err != nil {
			m.screen = foodScreenCartReview
			m.err = foodDisplayErr("Checkout blocked", msg.err)
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.orderResult = msg.result
		m.orderElapsed = msg.elapsed
		m.screen = foodScreenOrderResult
		m.status = "order placed"
		return foodClearOnScreenChange(previousScreen, m, nil)

	case foodOrdersMsg:
		if msg.err != nil {
			m.screen = foodScreenHome
			m.err = foodDisplayErr("Could not load orders", msg.err)
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.orders = msg.history
		m.screen = foodScreenOrders
		m.cursor = 0
		if msg.activeOnly {
			m.status = "loaded active orders in " + formatElapsed(msg.elapsed)
		} else {
			m.status = "loaded order history in " + formatElapsed(msg.elapsed)
		}
		return foodClearOnScreenChange(previousScreen, m, nil)

	case foodTrackingMsg:
		if msg.err != nil {
			m.screen = foodScreenOrders
			m.err = foodDisplayErr("Tracking unavailable", msg.err)
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.tracking = msg.status
		m.screen = foodScreenTracking
		m.status = "tailed in " + formatElapsed(msg.elapsed)
		return foodClearOnScreenChange(previousScreen, m, nil)

	case foodSearchDebounceMsg:
		if msg.version != m.searchPreviewVersion || msg.query != m.searchQuery {
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		if msg.isDish {
			if m.screen != foodScreenMenuSearch {
				return foodClearOnScreenChange(previousScreen, m, nil)
			}
			if len([]rune(strings.TrimSpace(msg.query))) < 2 {
				return foodClearOnScreenChange(previousScreen, m, nil)
			}
			m.searchPreviewLoading = true
			m.searchPreviewLoaded = false
			m.searchPreviewQuery = msg.query
			m.searchPreviewMenuItems = nil
			m.searchPreviewElapsed = 0
			m.searchPreviewErr = ""
			return foodClearOnScreenChange(previousScreen, m, tea.Batch(
				m.searchMenuCmd(msg.query, true, msg.version),
				m.foodSearchSpinnerCmd(msg.version),
			))
		}
		if m.screen != foodScreenSearchInput {
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		if len([]rune(strings.TrimSpace(msg.query))) < 2 {
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.searchPreviewLoading = true
		m.searchPreviewLoaded = false
		m.searchPreviewQuery = msg.query
		m.searchPreviewRestaurants = nil
		m.searchPreviewElapsed = 0
		m.searchPreviewErr = ""
		return foodClearOnScreenChange(previousScreen, m, tea.Batch(
			m.searchRestaurantsCmd(msg.query, true, msg.version),
			m.foodSearchSpinnerCmd(msg.version),
		))

	case foodSearchSpinnerMsg:
		if !m.searchPreviewLoading || msg.version != m.searchPreviewVersion {
			return foodClearOnScreenChange(previousScreen, m, nil)
		}
		m.searchPreviewSpinner = (m.searchPreviewSpinner + 1) % len(foodSearchSpinnerFrames)
		return foodClearOnScreenChange(previousScreen, m, m.foodSearchSpinnerCmd(msg.version))
	}
	return m, nil
}

func foodClearOnScreenChange(previous foodScreen, model tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	updated, ok := model.(foodModel)
	if !ok || updated.screen == previous {
		return model, cmd
	}
	return updated, cmd
}

// ── Key handling ──────────────────────────────────────────────────────────────

func (m foodModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || key == "q" {
		m.result.Action = FoodActionQuit
		return m, tea.Quit
	}
	if key == "esc" {
		return m.handleEsc()
	}

	switch m.screen {
	case foodScreenHome:
		return m.handleHomeKey(key)
	case foodScreenSearchInput:
		return m.handleSearchInputKey(msg)
	case foodScreenRestaurantList:
		return m.handleRestaurantListKey(key)
	case foodScreenMenuBrowse:
		return m.handleMenuBrowseKey(key)
	case foodScreenMenuSearch:
		return m.handleMenuSearchKey(msg)
	case foodScreenMenuResults:
		return m.handleMenuResultsKey(key)
	case foodScreenItemDetail:
		return m.handleItemDetailKey(key)
	case foodScreenCartReview:
		return m.handleCartReviewKey(key)
	case foodScreenCoupons:
		return m.handleCouponsKey(key)
	case foodScreenCheckoutConfirm:
		return m.handleCheckoutConfirmKey(key)
	case foodScreenOrders:
		return m.handleOrdersKey(key)
	case foodScreenOrderResult, foodScreenTracking, foodScreenMessage:
		if key == "enter" || key == "b" || key == "h" {
			m.screen = foodScreenHome
			m.err = ""
			return m, nil
		}
	}
	return m, nil
}

func (m foodModel) handleEsc() (tea.Model, tea.Cmd) {
	switch m.screen {
	case foodScreenHome:
		m.result.Action = FoodActionBackToHome
		if m.selectedAddress != nil {
			m.result.SelectedAddress = *m.selectedAddress
		}
		return m, tea.Quit
	case foodScreenCheckoutConfirm:
		m.screen = foodScreenCartReview
		m.status = "Checkout cancelled."
		return m, nil
	case foodScreenMenuBrowse:
		m.screen = foodScreenRestaurantList
		m.err = ""
		m.status = ""
		return m, nil
	case foodScreenItemDetail:
		if len(m.menuItems) > 0 {
			m.screen = foodScreenMenuResults
		} else {
			m.screen = foodScreenMenuBrowse
		}
		m.err = ""
		return m, nil
	}
	m.screen = foodScreenHome
	m.err = ""
	m.status = ""
	return m, nil
}

func (m foodModel) handleHomeKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.homeCursor > 0 {
			m.homeCursor--
		}
	case "down", "j":
		if m.homeCursor < len(foodHomeChoices)-1 {
			m.homeCursor++
		}
	case "/":
		return m.startRestaurantSearch(), nil
	case "d":
		return m.startDishSearch(), nil
	case "c":
		return m.runHomeAction("cart")
	case "enter":
		return m.runHomeAction(foodHomeChoices[m.homeCursor].action)
	default:
		if idx, ok := numberKeyIndex(key); ok && idx < len(foodHomeChoices) {
			return m.runHomeAction(foodHomeChoices[idx].action)
		}
	}
	return m, nil
}

func (m foodModel) runHomeAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "search":
		return m.startRestaurantSearch(), nil
	case "dish":
		return m.startDishSearch(), nil
	case "cart":
		if !m.hasAddress() {
			m.err = "Address required to view cart."
			return m, nil
		}
		return m.loadCart("Loading staged cart...")
	case "orders":
		if !m.hasAddress() {
			m.err = "Address required."
			return m, nil
		}
		m.screen = foodScreenLoading
		m.loading = "Loading order history..."
		return m, m.loadOrdersCmd(false)
	}
	return m, nil
}

func (m foodModel) startRestaurantSearch() foodModel {
	if !m.hasAddress() {
		m.err = "Address required to search restaurants."
		return m
	}
	m.screen = foodScreenSearchInput
	m.isDishSearch = false
	m.searchQuery = ""
	m.searchPreviewQuery = ""
	m.searchPreviewRestaurants = nil
	m.searchPreviewMenuItems = nil
	m.searchPreviewLoading = false
	m.searchPreviewLoaded = false
	m.searchPreviewElapsed = 0
	m.searchPreviewVersion++
	m.searchPreviewSpinner = 0
	m.searchPreviewErr = ""
	m.err = ""
	m.status = ""
	return m
}

func (m foodModel) startDishSearch() foodModel {
	if !m.hasAddress() {
		m.err = "Address required to search dishes."
		return m
	}
	m.screen = foodScreenMenuSearch
	m.isDishSearch = true
	m.searchQuery = ""
	m.searchPreviewQuery = ""
	m.searchPreviewRestaurants = nil
	m.searchPreviewMenuItems = nil
	m.searchPreviewLoading = false
	m.searchPreviewLoaded = false
	m.searchPreviewElapsed = 0
	m.searchPreviewVersion++
	m.searchPreviewSpinner = 0
	m.searchPreviewErr = ""
	m.err = ""
	m.status = ""
	return m
}

func (m foodModel) handleSearchInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter":
		query := m.searchQuery
		if strings.TrimSpace(query) == "" {
			m.err = "Type a restaurant name before searching."
			return m, nil
		}
		if m.searchPreviewLoaded && m.searchPreviewQuery == query {
			m.restaurants = m.searchPreviewRestaurants
			m.cursor = 0
			m.screen = foodScreenRestaurantList
			if m.searchPreviewElapsed > 0 {
				m.status = fmt.Sprintf("found %d restaurants in %s", len(m.restaurants), formatElapsed(m.searchPreviewElapsed))
			}
			if len(m.restaurants) == 0 {
				m.screen = foodScreenHome
				m.status = "No matching restaurants found. Try another search."
			}
			return m, nil
		}
		m.screen = foodScreenLoading
		m.loading = "searching restaurants..."
		return m, m.searchRestaurantsCmd(query, false, m.searchPreviewVersion)
	case "backspace", "ctrl+h":
		if len(m.searchQuery) > 0 {
			runes := []rune(m.searchQuery)
			m.searchQuery = string(runes[:len(runes)-1])
			return m, m.queueRestaurantPreview()
		}
	case "space", " ":
		m.searchQuery += " "
		return m, m.queueRestaurantPreview()
	default:
		if msg.Type == tea.KeyRunes {
			m.searchQuery += msg.String()
			return m, m.queueRestaurantPreview()
		}
	}
	return m, nil
}

func (m *foodModel) queueRestaurantPreview() tea.Cmd {
	m.searchPreviewVersion++
	m.searchPreviewLoading = false
	m.searchPreviewLoaded = false
	m.searchPreviewQuery = ""
	m.searchPreviewRestaurants = nil
	m.searchPreviewErr = ""
	if len([]rune(strings.TrimSpace(m.searchQuery))) < 2 {
		return nil
	}
	query := m.searchQuery
	version := m.searchPreviewVersion
	return tea.Tick(foodSearchPreviewDebounce, func(time.Time) tea.Msg {
		return foodSearchDebounceMsg{query: query, version: version, isDish: false}
	})
}

func (m foodModel) handleRestaurantListKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.restaurants)-1 {
			m.cursor++
		}
	case "enter":
		return m.selectRestaurant(m.cursor)
	default:
		if idx, ok := numberKeyIndex(key); ok {
			return m.selectRestaurant(idx)
		}
	}
	return m, nil
}

func (m foodModel) selectRestaurant(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.restaurants) {
		m.err = "Choose a listed restaurant."
		return m, nil
	}
	r := m.restaurants[idx]
	if strings.ToUpper(r.Availability) != "OPEN" {
		m.err = "That restaurant is currently closed. Choose an OPEN restaurant."
		return m, nil
	}
	m.selectedRestaurant = &m.restaurants[idx]
	m.screen = foodScreenLoading
	m.loading = "Loading menu..."
	return m, m.loadMenuCmd(m.selectedRestaurant.ID)
}

func (m foodModel) handleMenuBrowseKey(key string) (tea.Model, tea.Cmd) {
	items := m.allMenuItems()
	switch key {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(items)-1 {
			m.menuCursor++
		}
	case "enter":
		if m.menuCursor < 0 || m.menuCursor >= len(items) {
			m.err = "Choose a menu item."
			return m, nil
		}
		item := items[m.menuCursor]
		detail := menuItemToDetail(item)
		if hasVariants(detail) {
			m.selectedItem = &detail
			m.variantCursors = make([]int, len(allVariantGroups(detail)))
			m.screen = foodScreenItemDetail
			m.err = ""
			return m, nil
		}
		return m.addItemToCart(detail, nil, nil, nil)
	case "s", "/":
		return m.startDishSearch(), nil
	}
	return m, nil
}

func (m foodModel) handleMenuSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter":
		query := m.searchQuery
		if strings.TrimSpace(query) == "" {
			m.err = "Type a dish name before searching."
			return m, nil
		}
		if m.searchPreviewLoaded && m.searchPreviewQuery == query {
			m.menuItems = m.searchPreviewMenuItems
			m.cursor = 0
			m.screen = foodScreenMenuResults
			if m.searchPreviewElapsed > 0 {
				m.status = fmt.Sprintf("found %d dishes in %s", len(m.menuItems), formatElapsed(m.searchPreviewElapsed))
			}
			if len(m.menuItems) == 0 {
				m.screen = foodScreenHome
				m.status = "No matching dishes found. Try another search."
			}
			return m, nil
		}
		m.screen = foodScreenLoading
		m.loading = "searching dishes..."
		return m, m.searchMenuCmd(query, false, m.searchPreviewVersion)
	case "backspace", "ctrl+h":
		if len(m.searchQuery) > 0 {
			runes := []rune(m.searchQuery)
			m.searchQuery = string(runes[:len(runes)-1])
			return m, m.queueDishPreview()
		}
	case "space", " ":
		m.searchQuery += " "
		return m, m.queueDishPreview()
	default:
		if msg.Type == tea.KeyRunes {
			m.searchQuery += msg.String()
			return m, m.queueDishPreview()
		}
	}
	return m, nil
}

func (m *foodModel) queueDishPreview() tea.Cmd {
	m.searchPreviewVersion++
	m.searchPreviewLoading = false
	m.searchPreviewLoaded = false
	m.searchPreviewQuery = ""
	m.searchPreviewMenuItems = nil
	m.searchPreviewErr = ""
	if len([]rune(strings.TrimSpace(m.searchQuery))) < 2 {
		return nil
	}
	query := m.searchQuery
	version := m.searchPreviewVersion
	return tea.Tick(foodSearchPreviewDebounce, func(time.Time) tea.Msg {
		return foodSearchDebounceMsg{query: query, version: version, isDish: true}
	})
}

func (m foodModel) handleMenuResultsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.menuItems)-1 {
			m.cursor++
		}
	case "enter":
		return m.selectMenuItem(m.cursor)
	default:
		if idx, ok := numberKeyIndex(key); ok {
			return m.selectMenuItem(idx)
		}
	}
	return m, nil
}

func (m foodModel) selectMenuItem(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.menuItems) {
		m.err = "Choose a dish."
		return m, nil
	}
	item := m.menuItems[idx]
	if hasVariants(item) {
		m.selectedItem = &m.menuItems[idx]
		m.variantCursors = make([]int, len(allVariantGroups(item)))
		m.screen = foodScreenItemDetail
		m.err = ""
		return m, nil
	}
	return m.addItemToCart(item, nil, nil, nil)
}

func (m foodModel) handleItemDetailKey(key string) (tea.Model, tea.Cmd) {
	if m.selectedItem == nil {
		m.screen = foodScreenHome
		return m, nil
	}
	groups := allVariantGroups(*m.selectedItem)
	switch key {
	case "up", "k":
		if len(groups) > 0 {
			groupIdx := m.cursor % len(groups)
			if m.variantCursors[groupIdx] > 0 {
				m.variantCursors[groupIdx]--
			}
		}
	case "down", "j":
		if len(groups) > 0 {
			groupIdx := m.cursor % len(groups)
			maxIdx := len(groups[groupIdx].Variants) - 1
			if m.variantCursors[groupIdx] < maxIdx {
				m.variantCursors[groupIdx]++
			}
		}
	case "tab":
		if len(groups) > 1 {
			m.cursor = (m.cursor + 1) % len(groups)
		}
	case "enter":
		variants, variantsV2, addons := m.buildSelectedVariants()
		return m.addItemToCart(*m.selectedItem, variants, variantsV2, addons)
	}
	return m, nil
}

func (m foodModel) buildSelectedVariants() ([]domainfood.CartVariant, []domainfood.CartVariantV2, []domainfood.CartAddon) {
	if m.selectedItem == nil {
		return nil, nil, nil
	}
	groups := allVariantGroups(*m.selectedItem)

	if len(m.selectedItem.VariantsV2) > 0 {
		var variantsV2 []domainfood.CartVariantV2
		for i, group := range groups {
			if i >= len(m.variantCursors) {
				break
			}
			vIdx := m.variantCursors[i]
			if vIdx < 0 || vIdx >= len(group.Variants) {
				continue
			}
			v := group.Variants[vIdx]
			variantsV2 = append(variantsV2, domainfood.CartVariantV2{GroupID: v.GroupID, VariationID: v.VariationID})
		}
		return nil, variantsV2, nil
	}

	var variants []domainfood.CartVariant
	for i, group := range groups {
		if i >= len(m.variantCursors) {
			break
		}
		vIdx := m.variantCursors[i]
		if vIdx < 0 || vIdx >= len(group.Variants) {
			continue
		}
		v := group.Variants[vIdx]
		variants = append(variants, domainfood.CartVariant{GroupID: v.GroupID, VariationID: v.VariationID})
	}
	return variants, nil, nil
}

func (m foodModel) addItemToCart(item domainfood.MenuItemDetail, variants []domainfood.CartVariant, variantsV2 []domainfood.CartVariantV2, addons []domainfood.CartAddon) (tea.Model, tea.Cmd) {
	if m.selectedRestaurant == nil {
		m.err = "Restaurant required to add items to cart."
		return m, nil
	}
	updateItem := domainfood.FoodCartUpdateItem{
		MenuItemID: item.ID,
		Quantity:   1,
		Variants:   variants,
		VariantsV2: variantsV2,
		Addons:     addons,
	}
	items := upsertFoodCartItem(m.intendedItems, updateItem)
	m.screen = foodScreenLoading
	m.loading = "Updating cart..."
	return m, m.updateCartCmd(items)
}

func (m foodModel) handleCartReviewKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "p":
		if len(m.currentCart.Items) == 0 {
			m.err = "Add an item before checkout."
			return m, nil
		}
		paymentMethod := foodPreferredPaymentMethod(m.currentCart.AvailablePaymentMethods)
		if paymentMethod == "" {
			m.err = "No payment method is available for this cart."
			return m, nil
		}
		m.paymentMethod = paymentMethod
		m.reviewedCart = &domainfood.FoodCartReviewSnapshot{
			AddressID:     m.selectedAddressID(),
			RestaurantID:  m.currentCart.RestaurantID,
			Items:         append([]domainfood.FoodCartUpdateItem(nil), m.intendedItems...),
			ToPayRupees:   foodCartToPayRupees(m.currentCart),
			PaymentMethod: m.paymentMethod,
		}
		m.screen = foodScreenCheckoutConfirm
		m.err = ""
		return m, nil
	case "up", "k":
		if m.cartScroll > 0 {
			m.cartScroll--
		}
	case "down", "j":
		maxScroll := len(m.cartReviewLines()) - (bodyRows - 1)
		if maxScroll > 0 && m.cartScroll < maxScroll {
			m.cartScroll++
		}
	case "c":
		restID := m.selectedRestaurantID()
		if restID == "" {
			m.err = "No restaurant selected for coupons."
			return m, nil
		}
		m.screen = foodScreenLoading
		m.loading = "Loading coupons..."
		return m, m.loadCouponsCmd()
	case "s", "/":
		return m.startDishSearch(), nil
	case "h", "b":
		m.screen = foodScreenHome
		return m, nil
	}
	return m, nil
}

func (m foodModel) handleCouponsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.coupons.Coupons)-1 {
			m.cursor++
		}
	case "enter":
		return m.applyCoupon(m.cursor)
	case "b", "h", "esc":
		m.screen = foodScreenCartReview
		m.err = ""
		return m, nil
	default:
		if idx, ok := numberKeyIndex(key); ok {
			return m.applyCoupon(idx)
		}
	}
	return m, nil
}

func (m foodModel) applyCoupon(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.coupons.Coupons) {
		m.err = "Choose a listed coupon."
		return m, nil
	}
	coupon := m.coupons.Coupons[idx]
	if !coupon.Applicable {
		m.err = "That coupon is not applicable to your current cart."
		return m, nil
	}
	m.screen = foodScreenLoading
	m.loading = "Applying coupon..."
	code := coupon.Code
	return m, func() tea.Msg {
		started := time.Now()
		err := m.service.ApplyCoupon(m.ctx, appfood.ApplyCouponInput{
			CouponCode: code,
			AddressID:  m.selectedAddressID(),
		})
		if err != nil {
			return foodCartMsg{err: err, action: "apply coupon", elapsed: time.Since(started)}
		}
		cart, cartErr := m.service.GetCart(m.ctx, appfood.GetCartInput{AddressID: m.selectedAddressID(), RestaurantName: m.selectedRestaurantName()})
		return foodCartMsg{cart: cart, err: cartErr, action: "coupon applied", elapsed: time.Since(started)}
	}
}

func (m foodModel) handleCheckoutConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		m.screen = foodScreenLoading
		m.loading = "Placing order..."
		return m, m.checkoutCmd()
	case "n", "b", "esc":
		m.screen = foodScreenCartReview
		m.status = "Checkout cancelled."
		return m, nil
	}
	return m, nil
}

func (m foodModel) handleOrdersKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.orders.Orders)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.orders.Orders) {
			m.err = "Choose an order."
			return m, nil
		}
		order := m.orders.Orders[m.cursor]
		if !order.Active {
			m.err = "Tracking is only available for active orders."
			return m, nil
		}
		m.screen = foodScreenLoading
		m.loading = "Tracking order..."
		return m, m.trackOrderCmd(order.OrderID)
	case "b", "h":
		m.screen = foodScreenHome
		return m, nil
	}
	return m, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (m foodModel) loadCart(loading string) (tea.Model, tea.Cmd) {
	m.screen = foodScreenLoading
	m.loading = loading
	return m, m.loadCartCmd()
}

func (m *foodModel) applyCart(cart domainfood.FoodCart) {
	m.currentCart = cart
	m.intendedItems = foodCartItemsToUpdateItems(cart.Items)
}

func (m foodModel) hasAddress() bool {
	return m.selectedAddress != nil && strings.TrimSpace(m.selectedAddress.ID) != ""
}

func (m foodModel) selectedAddressID() string {
	if m.selectedAddress == nil {
		return ""
	}
	return m.selectedAddress.ID
}

func (m foodModel) selectedRestaurantID() string {
	if m.selectedRestaurant != nil {
		return m.selectedRestaurant.ID
	}
	return m.currentCart.RestaurantID
}

func (m foodModel) selectedRestaurantName() string {
	if m.selectedRestaurant != nil {
		return m.selectedRestaurant.Name
	}
	return m.currentCart.RestaurantName
}

func (m foodModel) allMenuItems() []domainfood.MenuItem {
	var items []domainfood.MenuItem
	for _, cat := range m.menuPage.Categories {
		items = append(items, cat.Items...)
	}
	return items
}

// ── Commands ──────────────────────────────────────────────────────────────────

func (m foodModel) searchRestaurantsCmd(query string, preview bool, version int) tea.Cmd {
	return func() tea.Msg {
		started := time.Now()
		result, err := m.service.SearchRestaurants(m.ctx, appfood.SearchRestaurantsInput{
			AddressID: m.selectedAddressID(),
			Query:     query,
		})
		return foodRestaurantsMsg{result: result, preview: preview, query: query, version: version, err: err, elapsed: time.Since(started)}
	}
}

func (m foodModel) loadMenuCmd(restaurantID string) tea.Cmd {
	return func() tea.Msg {
		started := time.Now()
		page, err := m.service.GetRestaurantMenu(m.ctx, appfood.GetMenuInput{
			AddressID:    m.selectedAddressID(),
			RestaurantID: restaurantID,
		})
		return foodMenuMsg{page: page, err: err, elapsed: time.Since(started)}
	}
}

func (m foodModel) searchMenuCmd(query string, preview bool, version int) tea.Cmd {
	restaurantID := ""
	if m.selectedRestaurant != nil {
		restaurantID = m.selectedRestaurant.ID
	}
	return func() tea.Msg {
		started := time.Now()
		result, err := m.service.SearchMenu(m.ctx, appfood.SearchMenuInput{
			AddressID:    m.selectedAddressID(),
			Query:        query,
			RestaurantID: restaurantID,
		})
		return foodMenuSearchMsg{result: result, preview: preview, version: version, query: query, err: err, elapsed: time.Since(started)}
	}
}

func (m foodModel) loadCartCmd() tea.Cmd {
	return func() tea.Msg {
		started := time.Now()
		cart, err := m.service.GetCart(m.ctx, appfood.GetCartInput{AddressID: m.selectedAddressID(), RestaurantName: m.selectedRestaurantName()})
		return foodCartMsg{cart: cart, err: err, action: "loaded staged cart", elapsed: time.Since(started)}
	}
}

func (m foodModel) updateCartCmd(items []domainfood.FoodCartUpdateItem) tea.Cmd {
	restaurantID := m.selectedRestaurantID()
	restaurantName := m.selectedRestaurantName()
	return func() tea.Msg {
		started := time.Now()
		updatedCart, err := m.service.UpdateCart(m.ctx, appfood.UpdateCartInput{
			RestaurantID:   restaurantID,
			RestaurantName: restaurantName,
			AddressID:      m.selectedAddressID(),
			Items:          items,
		})
		if err != nil {
			return foodCartMsg{err: err, action: "staged", elapsed: time.Since(started)}
		}
		cart, cartErr := m.service.GetCart(m.ctx, appfood.GetCartInput{AddressID: m.selectedAddressID(), RestaurantName: restaurantName})
		if cartErr != nil {
			return foodCartMsg{cart: updatedCart, refreshErr: cartErr, action: "staged", elapsed: time.Since(started)}
		}
		return foodCartMsg{cart: cart, action: "staged", elapsed: time.Since(started)}
	}
}

func (m foodModel) loadCouponsCmd() tea.Cmd {
	restaurantID := m.selectedRestaurantID()
	return func() tea.Msg {
		started := time.Now()
		result, err := m.service.FetchCoupons(m.ctx, appfood.FetchCouponsInput{
			RestaurantID: restaurantID,
			AddressID:    m.selectedAddressID(),
		})
		return foodCouponsMsg{result: result, err: err, elapsed: time.Since(started)}
	}
}

func (m foodModel) checkoutCmd() tea.Cmd {
	return func() tea.Msg {
		started := time.Now()
		result, err := m.service.PlaceOrder(m.ctx, appfood.PlaceOrderInput{
			AddressID:      m.selectedAddressID(),
			RestaurantName: m.selectedRestaurantName(),
			PaymentMethod:  m.paymentMethod,
			Confirmed:      true,
			ReviewedCart:   m.reviewedCart,
		})
		return foodOrderResultMsg{result: result, err: err, elapsed: time.Since(started)}
	}
}

func (m foodModel) loadOrdersCmd(activeOnly bool) tea.Cmd {
	return func() tea.Msg {
		started := time.Now()
		history, err := m.service.GetOrders(m.ctx, appfood.GetOrdersInput{
			AddressID:  m.selectedAddressID(),
			ActiveOnly: activeOnly,
		})
		return foodOrdersMsg{history: history, activeOnly: activeOnly, err: err, elapsed: time.Since(started)}
	}
}

func (m foodModel) trackOrderCmd(orderID string) tea.Cmd {
	return func() tea.Msg {
		started := time.Now()
		status, err := m.service.TrackOrder(m.ctx, appfood.TrackOrderInput{OrderID: orderID})
		return foodTrackingMsg{status: status, err: err, elapsed: time.Since(started)}
	}
}

func (m foodModel) foodSearchSpinnerCmd(version int) tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return foodSearchSpinnerMsg{version: version}
	})
}

// ── RenderWithResult ──────────────────────────────────────────────────────────

func (v FoodAppView) Render(ctx context.Context, w io.Writer) error {
	_, err := v.RenderWithResult(ctx, w)
	return err
}

func (v FoodAppView) RenderWithResult(ctx context.Context, w io.Writer) (FoodResult, error) {
	ctx = domainauth.ContextWithUserID(ctx, v.UserID)

	if v.Service == nil {
		m := foodModel{
			ctx:      ctx,
			viewport: v.Viewport,
			screen:   foodScreenMessage,
			message:  "Food ordering is unavailable in this session.",
		}
		finalModel, err := runInteractive(m, w, v.In)
		if err != nil {
			return FoodResult{}, err
		}
		if hm, ok := finalModel.(foodModel); ok {
			return hm.result, nil
		}
		return FoodResult{}, nil
	}

	if strings.TrimSpace(v.SelectedAddress.ID) == "" {
		m := foodModel{
			ctx:      ctx,
			viewport: v.Viewport,
			screen:   foodScreenMessage,
			message:  "address required — use 'a' in Home to switch address",
			result:   FoodResult{Action: FoodActionBackToHome},
		}
		finalModel, err := runInteractive(m, w, v.In)
		if err != nil {
			return FoodResult{Action: FoodActionBackToHome}, err
		}
		if hm, ok := finalModel.(foodModel); ok {
			return hm.result, nil
		}
		return FoodResult{Action: FoodActionBackToHome}, nil
	}

	selected := v.SelectedAddress
	m := foodModel{
		ctx:             ctx,
		service:         v.Service,
		viewport:        v.Viewport,
		screen:          foodScreenHome,
		status:          v.StatusMessage,
		selectedAddress: &selected,
	}

	finalModel, err := runInteractive(m, w, v.In)
	if err != nil {
		return FoodResult{}, err
	}
	if hm, ok := finalModel.(foodModel); ok {
		return hm.result, nil
	}
	return FoodResult{}, nil
}

// ── Utility functions ─────────────────────────────────────────────────────────

func formatElapsed(elapsed time.Duration) string {
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed < time.Second {
		return strconv.FormatInt(elapsed.Milliseconds(), 10) + "ms"
	}
	if elapsed < time.Minute {
		return strconv.FormatFloat(float64(elapsed)/float64(time.Second), 'f', 1, 64) + "s"
	}
	minutes := int(elapsed / time.Minute)
	seconds := int((elapsed % time.Minute) / time.Second)
	return fmt.Sprintf("%dm %02ds", minutes, seconds)
}

func numberKeyIndex(key string) (int, bool) {
	n, err := strconv.Atoi(key)
	if err != nil || n < 1 || n > 9 {
		return 0, false
	}
	return n - 1, true
}

func foodPreferredPaymentMethod(methods []string) string {
	fallback := ""
	for _, method := range methods {
		method = strings.TrimSpace(method)
		if method == "" {
			continue
		}
		if strings.EqualFold(method, "Cash") {
			return method
		}
		if fallback == "" {
			fallback = method
		}
	}
	return fallback
}

func foodCartToPayRupees(cart domainfood.FoodCart) int {
	if cart.Bill.ToPayRupees > 0 {
		return cart.Bill.ToPayRupees
	}
	return cart.TotalRupees
}

func upsertFoodCartItem(items []domainfood.FoodCartUpdateItem, newItem domainfood.FoodCartUpdateItem) []domainfood.FoodCartUpdateItem {
	updated := make([]domainfood.FoodCartUpdateItem, 0, len(items)+1)
	found := false
	for _, item := range items {
		if item.MenuItemID == newItem.MenuItemID {
			if newItem.Quantity > 0 {
				newItem.Quantity += item.Quantity
				updated = append(updated, newItem)
			}
			found = true
			continue
		}
		updated = append(updated, item)
	}
	if !found && newItem.Quantity > 0 {
		updated = append(updated, newItem)
	}
	return updated
}

func foodCartItemsToUpdateItems(items []domainfood.FoodCartItem) []domainfood.FoodCartUpdateItem {
	updateItems := make([]domainfood.FoodCartUpdateItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.MenuItemID) == "" || item.Quantity <= 0 {
			continue
		}
		updateItems = append(updateItems, domainfood.FoodCartUpdateItem{
			MenuItemID: item.MenuItemID,
			Quantity:   item.Quantity,
		})
	}
	return updateItems
}

func menuItemToDetail(item domainfood.MenuItem) domainfood.MenuItemDetail {
	return domainfood.MenuItemDetail{
		ID:          item.ID,
		Name:        item.Name,
		Price:       item.Price,
		IsVeg:       item.IsVeg,
		Rating:      item.Rating,
		Description: item.Description,
	}
}

func foodDisplayErr(prefix string, err error) string {
	suffix := " Please try again."
	switch {
	case err == nil:
		return prefix
	case errors.Is(err, appfood.ErrAddressRequired):
		return prefix + ": address required."
	case errors.Is(err, appfood.ErrRestaurantRequired):
		return prefix + ": restaurant required."
	case errors.Is(err, appfood.ErrRestaurantUnavailable):
		return prefix + ": restaurant is currently closed."
	case errors.Is(err, appfood.ErrCartEmpty):
		return prefix + ": your cart is empty."
	case errors.Is(err, appfood.ErrCartAmountLimit):
		return prefix + ": cart total must be below Rs 1000 in the terminal."
	case errors.Is(err, appfood.ErrCheckoutRequiresReview):
		return prefix + ": review the latest cart before checkout."
	case errors.Is(err, appfood.ErrCheckoutRequiresConfirmation):
		return prefix + ": confirm checkout first."
	case errors.Is(err, appfood.ErrPaymentMethodUnavailable):
		return prefix + ": selected payment method is unavailable."
	case errors.Is(err, appfood.ErrCancellationUnsupported):
		return "To cancel your order, please call Swiggy customer care at 080-67466729."
	default:
		return prefix + ":" + suffix
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func foodAddressLabel(address domainfood.Address) string {
	return defaultString(address.Label, defaultString(address.Category, "Saved address"))
}

func selectedFoodAddressLabel(address *domainfood.Address) string {
	if address == nil {
		return "address"
	}
	return foodAddressLabel(*address)
}

// allVariantGroups returns variantsV2 if set, else variants.
func allVariantGroups(item domainfood.MenuItemDetail) []domainfood.MenuVariantGroup {
	if len(item.VariantsV2) > 0 {
		return item.VariantsV2
	}
	return item.Variants
}

// hasVariants returns true if the item has any variant groups.
func hasVariants(item domainfood.MenuItemDetail) bool {
	return len(allVariantGroups(item)) > 0
}

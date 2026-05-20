package food

// Address is reused from the session — same shape as instamart.Address
type Address struct {
	ID          string
	Label       string
	DisplayLine string
	PhoneMasked string
	Category    string
}

type Restaurant struct {
	ID           string
	Name         string
	Cuisines     string
	Rating       string
	ETA          string
	PriceForTwo  string
	Availability string // "OPEN" or "CLOSED"
	IsAd         bool
}

type RestaurantSearchResult struct {
	Restaurants []Restaurant
	NextOffset  int
}

type MenuCategory struct {
	Name  string
	Items []MenuItem
}

type MenuItem struct {
	ID          string
	Name        string
	Price       int
	IsVeg       bool
	Rating      string
	Description string
	HasVariants bool
	HasAddons   bool
}

type MenuVariant struct {
	GroupID     string
	VariationID string
	Label       string
}

type MenuVariantGroup struct {
	GroupID  string
	Name     string
	Variants []MenuVariant
}

type MenuAddon struct {
	GroupID  string
	ChoiceID string
	Label    string
	Price    int
}

type MenuAddonGroup struct {
	GroupID string
	Name    string
	Addons  []MenuAddon
}

type MenuItemDetail struct {
	ID          string
	Name        string
	Price       int
	IsVeg       bool
	Rating      string
	Description string
	Variants    []MenuVariantGroup // legacy variants
	VariantsV2  []MenuVariantGroup // variantsV2 — only one of these will be populated
	Addons      []MenuAddonGroup
}

type MenuSearchResult struct {
	Items      []MenuItemDetail
	NextOffset int
}

type MenuPage struct {
	RestaurantID    string
	RestaurantName  string
	Categories      []MenuCategory
	Page            int
	TotalCategories int
}

type FoodCartItem struct {
	MenuItemID string
	Name       string
	Quantity   int
	Price      int
	FinalPrice int
}

type BillLine struct {
	Label string
	Value string
}

type BillBreakdown struct {
	Lines       []BillLine
	ToPayLabel  string
	ToPayValue  string
	ToPayRupees int
}

type FoodCart struct {
	RestaurantID            string
	RestaurantName          string
	AddressID               string
	AddressLabel            string
	Items                   []FoodCartItem
	Bill                    BillBreakdown
	TotalRupees             int
	AvailablePaymentMethods []string
}

type FoodCartUpdateItem struct {
	MenuItemID string
	Quantity   int
	Variants   []CartVariant   // legacy variants
	VariantsV2 []CartVariantV2 // variantsV2
	Addons     []CartAddon
}

type CartVariant struct {
	GroupID     string
	VariationID string
}

type CartVariantV2 struct {
	GroupID     string
	VariationID string
}

type CartAddon struct {
	GroupID  string
	ChoiceID string
}

type FoodCartReviewSnapshot struct {
	AddressID    string
	RestaurantID string
	Items        []FoodCartUpdateItem
	ToPayRupees  int
	PaymentMethod string
}

type FoodCoupon struct {
	Code        string
	Description string
	Discount    int
	Applicable  bool
}

type FoodCouponsResult struct {
	Coupons    []FoodCoupon
	Applicable int
}

type FoodOrderResult struct {
	Message       string
	OrderID       string
	Status        string
	PaymentMethod string
	CartTotal     int
}

type FoodOrderSummary struct {
	OrderID        string
	RestaurantName string
	Status         string
	TotalRupees    int
	Active         bool
}

type FoodOrderHistory struct {
	Orders  []FoodOrderSummary
	HasMore bool
}

type FoodOrderDetails struct {
	OrderID        string
	RestaurantName string
	Status         string
	Items          []FoodCartItem
	TotalRupees    int
}

type FoodTrackingStatus struct {
	OrderID          string
	StatusMessage    string
	SubStatusMessage string
	ETAText          string
	ETAMinutes       int
}

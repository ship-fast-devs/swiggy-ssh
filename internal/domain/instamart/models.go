package instamart

const DefaultOrderType = "DASH"

type Address struct {
	ID          string
	Label       string
	DisplayLine string
	PhoneMasked string
	Category    string
}

type Location struct {
	Lat float64
	Lng float64
}

type Price struct {
	MRP        int
	OfferPrice int
}

type Product struct {
	ID              string
	ParentProductID string
	DisplayName     string
	Brand           string
	InStock         bool
	Available       bool
	Promoted        bool
	Variations      []ProductVariation
}

type ProductVariation struct {
	SpinID              string
	DisplayName         string
	Brand               string
	QuantityDescription string
	Price               Price
	InStock             bool
	ImageURL            string
}

type CartUpdateItem struct {
	SpinID   string
	Quantity int
}

type CartItem struct {
	SpinID     string
	Name       string
	Quantity   int
	StoreID    string
	InStock    bool
	MRP        int
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

type Cart struct {
	AddressID               string
	AddressLabel            string
	AddressDisplayLine      string
	AddressLocation         *Location
	Items                   []CartItem
	Bill                    BillBreakdown
	TotalRupees             int
	AvailablePaymentMethods []string
	StoreIDs                []string
}

type CartReviewSnapshot struct {
	AddressID     string
	Items         []CartUpdateItem
	ToPayRupees   int
	PaymentMethod string
}

type CheckoutResult struct {
	Message       string
	OrderIDs      []string
	Status        string
	PaymentMethod string
	CartTotal     int
	MultiStore    bool
}

type OrderSummary struct {
	OrderID       string
	Status        string
	ItemCount     int
	TotalRupees   int
	PaymentMethod string
	Active        bool
	Location      *Location
}

type OrderHistory struct {
	Orders  []OrderSummary
	HasMore bool
}

type OrderHistoryQuery struct {
	Count      int
	OrderType  string
	ActiveOnly bool
}

type TrackingStatus struct {
	OrderID                string
	StatusMessage          string
	SubStatusMessage       string
	ETAText                string
	ETAMinutes             int
	Items                  []CartItem
	PollingIntervalSeconds int
}

type ProductSearchResult struct {
	Products   []Product
	NextOffset string
}

package foodflow

import (
	"reflect"
	"testing"

	domainfood "swiggy-ssh/internal/domain/food"
)

func TestUpsertFoodCartItemIncrementsExistingQuantity(t *testing.T) {
	items := []domainfood.FoodCartUpdateItem{
		{MenuItemID: "item-1", Quantity: 2},
	}
	newItem := domainfood.FoodCartUpdateItem{
		MenuItemID: "item-1",
		Quantity:   1,
		VariantsV2: []domainfood.CartVariantV2{{GroupID: "size", VariationID: "regular"}},
		Addons:     []domainfood.CartAddon{{GroupID: "extra", ChoiceID: "cheese"}},
	}

	got := upsertFoodCartItem(items, newItem)
	want := []domainfood.FoodCartUpdateItem{
		{
			MenuItemID: "item-1",
			Quantity:   3,
			VariantsV2: []domainfood.CartVariantV2{{GroupID: "size", VariationID: "regular"}},
			Addons:     []domainfood.CartAddon{{GroupID: "extra", ChoiceID: "cheese"}},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("upsertFoodCartItem() = %#v, want %#v", got, want)
	}
}

func TestUpsertFoodCartItemKeepsOtherItems(t *testing.T) {
	items := []domainfood.FoodCartUpdateItem{
		{MenuItemID: "item-1", Quantity: 2},
	}
	newItem := domainfood.FoodCartUpdateItem{MenuItemID: "item-2", Quantity: 1}

	got := upsertFoodCartItem(items, newItem)
	want := []domainfood.FoodCartUpdateItem{
		{MenuItemID: "item-1", Quantity: 2},
		{MenuItemID: "item-2", Quantity: 1},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("upsertFoodCartItem() = %#v, want %#v", got, want)
	}
}

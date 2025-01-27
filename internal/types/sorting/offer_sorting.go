package sorting

import "artion-api-graphql/internal/types"

type OfferSorting int8

const (
	OfferSortingNone OfferSorting = iota
)

func (ts OfferSorting) SortedFieldBson() string {
	return ""
}

func (ts OfferSorting) OrdinalFieldBson() string {
	return "_id"
}

func (ts OfferSorting) GetCursor(offer *types.Offer) (types.Cursor, error) {
	params := make(map[string]interface{})
	params["_id"] = offer.ID()
	return CursorFromParams(params)
}

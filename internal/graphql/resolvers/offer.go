package resolvers

import (
	"artion-api-graphql/internal/types"
	"artion-api-graphql/internal/types/sorting"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

type Offer types.Offer

type OfferEdge struct {
	Node *Offer
}

type OfferConnection struct {
	Edges      []OfferEdge
	TotalCount hexutil.Big
	PageInfo   PageInfo
}

func (o Offer) Token() (*Token, error) {
	return NewToken(&o.Contract, &o.TokenId)
}

func (edge OfferEdge) Cursor() (types.Cursor, error) {
	return sorting.OfferSortingNone.GetCursor((*types.Offer)(edge.Node))
}

func NewOfferConnection(list *types.OfferList) (con *OfferConnection, err error) {
	con = new(OfferConnection)
	con.TotalCount = (hexutil.Big)(*big.NewInt(list.TotalCount))
	con.Edges = make([]OfferEdge, len(list.Collection))
	for i := 0; i < len(list.Collection); i++ {
		con.Edges[i].Node = (*Offer)(list.Collection[i])
	}
	con.PageInfo.HasNextPage = list.HasNext
	con.PageInfo.HasPreviousPage = list.HasPrev
	if len(list.Collection) > 0 {
		startCur, err := con.Edges[0].Cursor()
		if err != nil {
			return nil, err
		}
		endCur, err := con.Edges[len(con.Edges)-1].Cursor()
		if err != nil {
			return nil, err
		}
		con.PageInfo.StartCursor = &startCur
		con.PageInfo.EndCursor = &endCur
	}
	return con, err
}

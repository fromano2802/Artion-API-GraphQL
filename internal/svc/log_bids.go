// Package svc implements monitoring and scanning services of the API server.
package svc

import (
	"artion-api-graphql/internal/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	eth "github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"time"
)

// auctionBidPlaced processes an event for newly posted auction bid.
// Auction::BidPlaced(address indexed nftAddress, uint256 indexed tokenId, address indexed bidder, uint256 bid)
func auctionBidPlaced(evt *eth.Log, lo *logObserver) {
	// sanity check: 1 + 3 topics; 1 x uint256 = 32 bytes
	if len(evt.Data) != 32 || len(evt.Topics) != 4 {
		log.Errorf("not Auction::BidPlaced() event #%d/#%d; expected 32 bytes of data, %d given; expected 4 topics, %d given",
			evt.BlockNumber, evt.Index, len(evt.Data), len(evt.Topics))
		return
	}

	blk, err := repo.GetHeader(evt.BlockNumber)
	if err != nil {
		log.Errorf("could not get header #%d, %s", evt.BlockNumber, err.Error())
		return
	}

	contract := common.BytesToAddress(evt.Topics[1].Bytes())
	tokenID := new(big.Int).SetBytes(evt.Topics[2].Bytes())

	// pull the auction involved
	auction, err := repo.GetAuction(&contract, tokenID)
	if err != nil {
		log.Errorf("auction %s/%s not found; %s", contract.String(), (*hexutil.Big)(tokenID).String(), err.Error())
		return
	}

	bid := types.AuctionBid{
		Contract: contract,
		TokenId:  hexutil.Big(*tokenID),
		Bidder:   common.BytesToAddress(evt.Topics[3].Bytes()),
		Placed:   types.Time(time.Unix(int64(blk.Time), 0)),
		Amount:   hexutil.Big(*new(big.Int).SetBytes(evt.Data[:])),
	}

	if err := repo.StoreAuctionBid(&bid); err != nil {
		log.Errorf("can not store bid %s on %s/%s; %s", bid.Bidder.String(), bid.Contract.String(), bid.TokenId.String(), err.Error())
		return
	}

	previousBidder := auction.LastBidder

	auction.LastBid = &bid.Amount
	auction.LastBidder = &bid.Bidder
	auction.LastBidPlaced = &bid.Placed

	// store the auction changes into database
	if err := repo.StoreAuction(auction); err != nil {
		log.Errorf("could not store auction; %s", err.Error())
	}

	// mark the token as being auctioned
	if err := repo.TokenMarkBid(
		&bid.Contract,
		(*big.Int)(&bid.TokenId),
		repo.GetUnifiedPriceAt(lo.marketplace, &auction.PayToken, new(big.Int).SetUint64(evt.BlockNumber), (*big.Int)(&bid.Amount)),
		(*time.Time)(&bid.Placed),
	); err != nil {
		log.Errorf("could not mark token as having bid; %s", err.Error())
	}

	// log activity
	activity := types.Activity{
		OrdinalIndex: types.OrdinalIndex(int64(evt.BlockNumber), int64(evt.Index)),
		Time:         bid.Placed,
		ActType:      types.EvtAuctionBid,
		Contract:     bid.Contract,
		TokenId:      bid.TokenId,
		From:         bid.Bidder,
		To:           &auction.Owner,
		PayToken:     &auction.PayToken,
		UnitPrice:    &bid.Amount,
	}
	if err := repo.StoreActivity(&activity); err != nil {
		log.Errorf("can not store bid activity %s on %s/%s; %s", bid.Bidder.String(), bid.Contract.String(), bid.TokenId.String(), err.Error())
		return
	}

	log.Infof("added new bid on auction %s/%s by %s", bid.Contract.String(), bid.TokenId.String(), bid.Bidder.String())

	// notify subscribers
	event := types.Event{ Type: "AUCTION_BID", Auction: auction }
	subscriptionManager := GetSubscriptionsManager()
	subscriptionManager.PublishAuctionEvent(event)
	subscriptionManager.PublishUserEvent(auction.Owner, event)
	if previousBidder != nil {
		subscriptionManager.PublishUserEvent(*previousBidder, event)
	}
}

// auctionBidWithdrawn processes an event for removed auction bid.
// Auction::BidWithdrawn(address indexed nftAddress, uint256 indexed tokenId, address indexed bidder, uint256 bid)
func auctionBidWithdrawn(evt *eth.Log, _ *logObserver) {
	// sanity check: 1 + 3 topics; 1 x uint256 = 32 bytes
	if len(evt.Data) != 32 || len(evt.Topics) != 4 {
		log.Errorf("not Auction::BidWithdrawn() event #%d/#%d; expected 32 bytes of data, %d given; expected 4 topics, %d given",
			evt.BlockNumber, evt.Index, len(evt.Data), len(evt.Topics))
		return
	}

	contract := common.BytesToAddress(evt.Topics[1].Bytes())
	tokenID := new(big.Int).SetBytes(evt.Topics[2].Bytes())
	bidder := common.BytesToAddress(evt.Topics[3].Bytes())

	if err := repo.DeleteAuctionBid(&contract, tokenID, &bidder); err != nil {
		log.Errorf("can not remove bid %s on %s/%s; %s", bidder.String(), contract.String(), (*hexutil.Big)(tokenID).String(), err.Error())
		return
	}

	// mark the token as being re-auctioned
	if err := repo.TokenMarkUnBid(&contract, tokenID); err != nil {
		log.Errorf("could not mark token as not having bid; %s", err.Error())
	}

	// pull the auction involved
	auction, err := repo.GetAuction(&contract, tokenID)
	if err != nil {
		log.Errorf("auction %s/%s not found; %s", contract.String(), (*hexutil.Big)(tokenID).String(), err.Error())
		return
	}
	
	auction.LastBid = nil
	auction.LastBidder = nil
	auction.LastBidPlaced = nil

	// store the listing into database
	if err := repo.StoreAuction(auction); err != nil {
		log.Errorf("could not store auction; %s", err.Error())
	}

	// log activity
	blk, err := repo.GetHeader(evt.BlockNumber)
	if err != nil {
		log.Errorf("could not get header #%d, %s", evt.BlockNumber, err.Error())
		return
	}
	activity := types.Activity{
		OrdinalIndex: types.OrdinalIndex(int64(evt.BlockNumber), int64(evt.Index)),
		Time:         types.Time(time.Unix(int64(blk.Time), 0)),
		ActType:      types.EvtAuctionBidWithdrawn,
		Contract:     contract,
		TokenId:      hexutil.Big(*tokenID),
		From:         bidder,
		To:           &auction.Owner,
	}
	if err := repo.StoreActivity(&activity); err != nil {
		log.Errorf("can not store unbid activity of %s on %s/%s; %s", bidder, activity.Contract, activity.TokenId, err.Error())
		return
	}

	// notify subscribers
	event := types.Event{ Type: "AUCTION_BID_WITHDRAW", Auction: auction }
	subscriptionManager := GetSubscriptionsManager()
	subscriptionManager.PublishAuctionEvent(event)
	subscriptionManager.PublishUserEvent(auction.Owner, event)
}

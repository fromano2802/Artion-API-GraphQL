// Package db provides access to the persistent storage.
package db

import (
	"artion-api-graphql/internal/types"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"math/big"
	"time"
)

const (
	// coAuctions is the name of database collection.
	coAuctions = "auctions"

	// fiAuctionContract is the name of the DB column storing NFT contract address.
	fiAuctionContract = "contract"

	// fiAuctionTokenId represents the name of the DB column storing NFT token ID.
	fiAuctionTokenId = "token"

	// fiAuctionOwner represents the name of the DB column storing token owner.
	fiAuctionOwner = "owner"

	// fiAuctionStartTime represents the name of the DB column storing auction start.
	// fiAuctionStartTime = "start"

	// fiAuctionEndTime represents the name of the DB column storing auction end.
	// fiAuctionEndTime = "end"

	// fiAuctionClosed represents the name of the DB column storing date/time of auction having been closed.
	fiAuctionClosed = "closed"

	// fiAuctionLatestBid is the name of the DB column storing the time of the latest bid date/time.
	fiAuctionLatestBid = "last_bid"

	// fiAuctionLatestBidder is the name of the DB column storing the address of the latest bidder.
	fiAuctionLatestBidder = "last_bidder"
)

// GetAuction provides the auction stored in the database, if available.
func (db *MongoDbBridge) GetAuction(contract *common.Address, tokenID *big.Int) (*types.Auction, error) {
	// get the collection
	col := db.client.Database(db.dbName).Collection(coAuctions)

	sr := col.FindOne(context.Background(), bson.D{{Key: fieldId, Value: types.AuctionID(contract, tokenID)}})
	if sr.Err() != nil {
		if sr.Err() == mongo.ErrNoDocuments {
			log.Debugf("auction for %s/%s not found",
				contract.String(), (*hexutil.Big)(tokenID).String())
			return nil, nil
		}

		log.Errorf("failed to lookup auction for %s/%s; %s",
			contract.String(), (*hexutil.Big)(tokenID).String(), sr.Err().Error())
		return nil, sr.Err()
	}

	// decode
	var row types.Auction
	if err := sr.Decode(&row); err != nil {
		log.Errorf("could not decode auction for %s/%s; %s",
			contract.String(), (*hexutil.Big)(tokenID).String(), sr.Err().Error())
		return nil, sr.Err()
	}
	return &row, nil
}

// StoreAuction adds the provided auction into the database.
func (db *MongoDbBridge) StoreAuction(au *types.Auction) error {
	if au == nil {
		return fmt.Errorf("no value to store")
	}

	// get the collection
	col := db.client.Database(db.dbName).Collection(coAuctions)

	// try to do the insert
	id := au.ID()
	if _, err := col.UpdateOne(
		context.Background(),
		bson.D{{Key: fieldId, Value: id}},
		bson.D{
			{Key: "$set", Value: au},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: fieldId, Value: id},
			}},
		},
		options.Update().SetUpsert(true),
	); err != nil {
		log.Errorf("can not add auction; %s", err)
		return err
	}
	return nil
}

// OpenAuctionTimeCheck provides the active auction date/time of given range.
func (db *MongoDbBridge) OpenAuctionTimeCheck(contract *common.Address, tokenID *big.Int, operator string, field string) *types.Time {
	var row struct {
		Value types.Time `bson:"val"`
	}

	col := db.client.Database(db.dbName).Collection(coAuctions)
	err := db.AggregateSingle(col, &mongo.Pipeline{
		bson.D{
			{Key: "$match", Value: bson.D{
				{Key: fiAuctionContract, Value: *contract},
				{Key: fiAuctionTokenId, Value: hexutil.Big(*tokenID)},
				{Key: fiAuctionClosed, Value: bson.D{{Key: "$type", Value: 10}}},
			}},
		},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: nil},
				{Key: "val", Value: bson.D{{Key: operator, Value: field}}},
			}},
		},
	}, &row)

	if err != nil {
		// no offer at all?
		if err == mongo.ErrNoDocuments {
			log.Infof("no open auction for %s/%s", contract.String(), (*hexutil.Big)(tokenID).String())
			return nil
		}
		log.Criticalf("failed auction check of %s/%s; %s", contract.String(), (*hexutil.Big)(tokenID).String(), err.Error())
		return nil
	}

	log.Infof("open auction check %s/%s %s %s",
		contract.String(), (*hexutil.Big)(tokenID).String(), operator, time.Time(row.Value).Format(time.RFC1123))
	return &row.Value
}

// OpenAuctionRange loads open auction date/time range.
func (db *MongoDbBridge) OpenAuctionRange(contract *common.Address, tokenID *big.Int) (*types.Time, *types.Time) {
	return db.OpenAuctionTimeCheck(contract, tokenID, "$min", "$start"),
		db.OpenAuctionTimeCheck(contract, tokenID, "$max", "$end")
}

// SetAuctionBidder sets a new bidder (or no bidder) into the specified auction.
func (db *MongoDbBridge) SetAuctionBidder(contract *common.Address, tokenID *big.Int, bidder *common.Address, placed *types.Time) error {
	col := db.client.Database(db.dbName).Collection(coAuctions)

	rs, err := col.UpdateOne(context.Background(), bson.D{
		{Key: fiAuctionContract, Value: *contract},
		{Key: fiAuctionTokenId, Value: (hexutil.Big)(*tokenID)},
	}, bson.D{
		{Key: fiAuctionLatestBid, Value: placed},
		{Key: fiAuctionLatestBidder, Value: bidder},
	})
	if err != nil {
		log.Errorf("could not update auction; %s", err.Error())
		return err
	}

	if rs.ModifiedCount > 0 {
		log.Infof("auction %s/%s bidder updated", contract.String(), (*hexutil.Big)(tokenID).String())
	}
	return nil
}

package handlers

import (
	"artion-api-graphql/internal/logger"
	"artion-api-graphql/internal/repository"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"net/http"
	"strings"
)

// ImageHandler builds a HTTP handler function for Token images.
func ImageHandler(log logger.Logger, resolver func(path string) (string, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Panic in ImageHandler handler; %s", r)
				w.WriteHeader(500)
				_, err := w.Write([]byte("Request handling failed"))
				if err != nil {
					log.Errorf("could not write response; %s", err.Error())
				}
			}
		}()

		uri, err := resolver(req.URL.Path)
		if err != nil {
			log.Errorf("token image request handling failed; %s", err)
			w.WriteHeader(500)
			_, _ = w.Write([]byte("Request handling failed: " + err.Error()))
			return
		}

		image, err := repository.R().GetImageThumbnail(uri)
		if err != nil {
			log.Errorf("unable to get image; %s", err)
			w.WriteHeader(500)
			_, _ = w.Write([]byte("Obtaining image failed: " + err.Error()))
			return
		}
		if image == nil || len(image.Data) == 0 {
			w.WriteHeader(404)
			_, _ = w.Write([]byte("No image available"))
			return
		}

		if image.Type.Mimetype() == "" {
			w.Header()["Content-Type"] = nil
		} else {
			w.Header().Add("Content-Type", image.Type.Mimetype())
		}
		w.WriteHeader(200)
		_, err = w.Write(image.Data)
		if err != nil {
			log.Errorf("writing image response failed; %s", err)
		}
	})
}

// TokenImageResolver resolves /token/{nft}/{tokenId} to token image URI
func TokenImageResolver(path string) (imageUri string, err error) {
	pathParts := strings.Split(path, "/")
	if len(pathParts) != 5 {
		return "", errors.New("invalid amount of slash delimiters in URL")
	}
	tokenAddress := common.HexToAddress(pathParts[3])
	tokenId, err := hexutil.DecodeBig(pathParts[4])
	if err != nil {
		return "", fmt.Errorf("unable to hex-decode tokenId; %s", err)
	}

	tok, err := repository.R().Token(&tokenAddress, (*hexutil.Big)(tokenId))
	if err != nil {
		return "", fmt.Errorf("unable to get token in db; %s", err)
	}

	jsonMetadata, err := repository.R().GetTokenJsonMetadata(tok.Uri)
	if err != nil {
		return "", fmt.Errorf("unable to get token json metadata; %s", err)
	}
	if jsonMetadata.Image == nil || *jsonMetadata.Image == "" {
		return "", fmt.Errorf("token has no image; %s", err)
	}
	return *jsonMetadata.Image, nil
}

// UserAvatarResolver resolves /avatar/{address} to user avatar URI
func UserAvatarResolver(path string) (imageUri string, err error) {
	pathParts := strings.Split(path, "/")
	if len(pathParts) != 5 {
		return "", errors.New("invalid amount of slash delimiters in URL")
	}
	userAddress := common.HexToAddress(pathParts[3])
	user, err := repository.R().GetUser(userAddress)
	if err != nil {
		return "", fmt.Errorf("unable to find user in db; %s", err)
	}
	if user == nil || user.Avatar == nil || *user.Avatar == "" {
		return "", fmt.Errorf("user has no avatar; %s", err)
	}
	return "/ipfs/" + *user.Avatar, nil
}

// CollectionImageResolver resolves /collection/{address} to collection image URI
func CollectionImageResolver(path string) (imageUri string, err error) {
	pathParts := strings.Split(path, "/")
	if len(pathParts) != 5 {
		return "", errors.New("invalid amount of slash delimiters in URL")
	}
	collectionAddress := common.HexToAddress(pathParts[3])
	collection, err := repository.R().GetLegacyCollection(collectionAddress)
	if err != nil {
		return "", fmt.Errorf("unable to find collection in db; %s", err)
	}
	if collection == nil || collection.Image == "" {
		return "", fmt.Errorf("collection has no image; %s", err)
	}
	return "/ipfs/" + collection.Image, nil
}

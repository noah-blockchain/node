package service

import (
	"bytes"
	"github.com/noah-blockchain/noah-go-node/config"
	"github.com/noah-blockchain/noah-go-node/core/noah"
	"github.com/noah-blockchain/noah-go-node/core/state"
	"github.com/golang/protobuf/jsonpb"
	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/tendermint/go-amino"
	tmNode "github.com/tendermint/tendermint/node"
	rpc "github.com/tendermint/tendermint/rpc/client"
	"google.golang.org/grpc/status"
)

type Service struct {
	cdc        *amino.Codec
	blockchain *noah.Blockchain
	client     *rpc.Local
	tmNode     *tmNode.Node
	noahCfg  *config.Config
	version    string
}

func NewService(cdc *amino.Codec, blockchain *noah.Blockchain, client *rpc.Local, node *tmNode.Node, noahCfg *config.Config, version string) *Service {
	return &Service{cdc: cdc, blockchain: blockchain, client: client, noahCfg: noahCfg, version: version, tmNode: node}
}

func (s *Service) getStateForHeight(height int32) (*state.State, error) {
	if height > 0 {
		cState, err := s.blockchain.GetStateForHeight(uint64(height))
		if err != nil {
			return nil, err
		}
		return cState, nil
	}

	return s.blockchain.CurrentState(), nil
}

func (s *Service) createError(statusErr *status.Status, data string) error {
	if len(data) == 0 {
		return statusErr.Err()
	}

	detailsMap, err := encodeToStruct([]byte(data))
	if err != nil {
		s.client.Logger.Error(err.Error())
		return statusErr.Err()
	}

	withDetails, err := statusErr.WithDetails(detailsMap)
	if err != nil {
		s.client.Logger.Error(err.Error())
		return statusErr.Err()
	}

	return withDetails.Err()
}

func (s *Service) checkTimeout(ctx context.Context) *status.Status {
	select {
	case <-ctx.Done():
		return status.FromContextError(ctx.Err())
	default:
		return nil
	}
}

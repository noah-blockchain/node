package transaction

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/noah-blockchain/noah-go-node/core/code"
	"github.com/noah-blockchain/noah-go-node/core/commissions"
	"github.com/noah-blockchain/noah-go-node/core/state"
	"github.com/noah-blockchain/noah-go-node/core/types"
	"github.com/noah-blockchain/noah-go-node/formula"
	"github.com/noah-blockchain/noah-go-node/hexutil"
	"github.com/tendermint/tendermint/libs/common"
)

type DelegateData struct {
	PubKey types.Pubkey     `json:"pub_key"`
	Coin   types.CoinSymbol `json:"coin"`
	Value  *big.Int         `json:"value"`
}

func (data DelegateData) TotalSpend(tx *Transaction, context *state.StateDB) (TotalSpends, []Conversion, *big.Int, *Response) {
	panic("implement me")
}

func (data DelegateData) BasicCheck(tx *Transaction, context *state.StateDB) *Response {
	if data.PubKey == nil || data.Value == nil {
		return &Response{
			Code: code.DecodeError,
			Log:  "Incorrect tx data"}
	}

	if !context.CoinExists(tx.GasCoin) {
		return &Response{
			Code: code.CoinNotExists,
			Log:  fmt.Sprintf("Coin %s not exists", tx.GasCoin)}
	}

	if data.Value.Cmp(types.Big0) < 1 {
		return &Response{
			Code: code.StakeShouldBePositive,
			Log:  fmt.Sprintf("Stake should be positive")}
	}

	if !context.Candidates.Exists(data.PubKey) {
		return &Response{
			Code: code.CandidateNotFound,
			Log:  fmt.Sprintf("Candidate with such public key not found"),
			Info: EncodeError(map[string]string{
				"pub_key": data.PubKey.String(),
			}),
		}
	}

	sender, _ := tx.Sender()
	if !context.Candidates.IsDelegatorStakeSufficient(sender, data.PubKey, data.Coin, data.Value) {
		return &Response{
			Code: code.TooLowStake,
			Log:  fmt.Sprintf("Stake is too low")}
	}

	return nil
}

func (data DelegateData) String() string {
	return fmt.Sprintf("DELEGATE pubkey:%s ",
		hexutil.Encode(data.PubKey[:]))
}

func (data DelegateData) Gas() int64 {
	return commissions.DelegateTx
}

func (data DelegateData) Run(tx *Transaction, context *state.State, isCheck bool, rewardPool *big.Int, currentBlock uint64) Response {
	sender, _ := tx.Sender()

	response := data.BasicCheck(tx, context)
	if response != nil {
		return *response
	}

	commissionInBaseCoin := tx.CommissionInBaseCoin()
	commission := big.NewInt(0).Set(commissionInBaseCoin)

	if !tx.GasCoin.IsBaseCoin() {
		coin := context.Coins.GetCoin(tx.GasCoin)

		errResp := CheckReserveUnderflow(coin, commissionInBaseCoin)
		if errResp != nil {
			return *errResp
		}

		if coin.Reserve().Cmp(commissionInBaseCoin) < 0 {
			return Response{
				Code: code.CoinReserveNotSufficient,
				Log:  fmt.Sprintf("Coin reserve balance is not sufficient for transaction. Has: %s, required %s", coin.Reserve().String(), commissionInBaseCoin.String()),
				Info: EncodeError(map[string]string{
					"has_reserve": coin.Reserve().String(),
					"commission":  commissionInBaseCoin.String(),
					"gas_coin":    coin.CName,
				}),
			}
		}

		commission = formula.CalculateSaleAmount(coin.Volume(), coin.Reserve(), coin.Crr(), commissionInBaseCoin)
	}

	if context.Accounts.GetBalance(sender, tx.GasCoin).Cmp(commission) < 0 {
		return Response{
			Code: code.InsufficientFunds,
			Log:  fmt.Sprintf("Insufficient funds for sender account: %s. Wanted %s %s", sender.String(), commission, tx.GasCoin),
			Info: EncodeError(map[string]string{
				"sender":       sender.String(),
				"needed_value": commission.String(),
				"gas_coin":     fmt.Sprintf("%s", tx.GasCoin),
			}),
		}
	}

	if context.Accounts.GetBalance(sender, data.Coin).Cmp(data.Value) < 0 {
		return Response{
			Code: code.InsufficientFunds,
			Log:  fmt.Sprintf("Insufficient funds for sender account: %s. Wanted %s %s", sender.String(), data.Value, data.Coin),
			Info: EncodeError(map[string]string{
				"sender":       sender.String(),
				"needed_value": data.Value.String(),
				"coin":         fmt.Sprintf("%s", data.Coin),
			}),
		}
	}

	if data.Coin == tx.GasCoin {
		totalTxCost := big.NewInt(0)
		totalTxCost.Add(totalTxCost, data.Value)
		totalTxCost.Add(totalTxCost, commission)

		if context.Accounts.GetBalance(sender, tx.GasCoin).Cmp(totalTxCost) < 0 {
			return Response{
				Code: code.InsufficientFunds,
				Log:  fmt.Sprintf("Insufficient funds for sender account: %s. Wanted %s %s", sender.String(), totalTxCost.String(), tx.GasCoin),
				Info: EncodeError(map[string]string{
					"sender":       sender.String(),
					"needed_value": totalTxCost.String(),
					"gas_coin":     fmt.Sprintf("%s", tx.GasCoin),
				}),
			}
		}
	}

	if !isCheck {
		rewardPool.Add(rewardPool, commissionInBaseCoin)

		context.Coins.SubReserve(tx.GasCoin, commissionInBaseCoin)
		context.Coins.SubVolume(tx.GasCoin, commission)

		context.Accounts.SubBalance(sender, tx.GasCoin, commission)
		context.Accounts.SubBalance(sender, data.Coin, data.Value)
		context.Candidates.Delegate(sender, data.PubKey, data.Coin, data.Value, big.NewInt(0))
		context.Accounts.SetNonce(sender, tx.Nonce)
	}

	tags := kv.Pairs{
		kv.Pair{Key: []byte("tx.type"), Value: []byte(hex.EncodeToString([]byte{byte(TypeDelegate)}))},
		kv.Pair{Key: []byte("tx.from"), Value: []byte(hex.EncodeToString(sender[:]))},
	}

	return Response{
		Code:      code.OK,
		GasUsed:   tx.Gas(),
		GasWanted: tx.Gas(),
		Tags:      tags,
	}
}

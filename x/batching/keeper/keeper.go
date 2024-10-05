package keeper

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	"cosmossdk.io/collections"
	addresscodec "cosmossdk.io/core/address"
	storetypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/sedaprotocol/seda-chain/x/batching/types"
)

type Keeper struct {
	stakingKeeper         types.StakingKeeper
	wasmStorageKeeper     types.WasmStorageKeeper
	pubKeyKeeper          types.PubKeyKeeper
	wasmKeeper            wasmtypes.ContractOpsKeeper
	wasmViewKeeper        wasmtypes.ViewKeeper
	validatorAddressCodec addresscodec.Codec

	// authority is the address capable of executing MsgUpdateParams.
	// Typically, this should be the gov module address.
	authority string

	Schema             collections.Schema
	dataResults        collections.Map[collections.Pair[bool, string], types.DataResult]
	currentBatchNumber collections.Sequence
	batches            collections.Map[uint64, types.Batch]
	votes              collections.Map[collections.Pair[uint64, []byte], types.Vote]
	params             collections.Item[types.Params]
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService storetypes.KVStoreService,
	authority string,
	sk types.StakingKeeper,
	wsk types.WasmStorageKeeper,
	pkk types.PubKeyKeeper,
	wk wasmtypes.ContractOpsKeeper,
	wvk wasmtypes.ViewKeeper,
	validatorAddressCodec addresscodec.Codec,
) Keeper {
	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		stakingKeeper:         sk,
		wasmStorageKeeper:     wsk,
		pubKeyKeeper:          pkk,
		wasmKeeper:            wk,
		wasmViewKeeper:        wvk,
		validatorAddressCodec: validatorAddressCodec,
		authority:             authority,
		dataResults:           collections.NewMap(sb, types.DataResultsPrefix, "data_results", collections.PairKeyCodec(collections.BoolKey, collections.StringKey), codec.CollValue[types.DataResult](cdc)),
		currentBatchNumber:    collections.NewSequence(sb, types.CurrentBatchNumberKey, "current_batch_number"),
		batches:               collections.NewMap(sb, types.BatchesKeyPrefix, "batches", collections.Uint64Key, codec.CollValue[types.Batch](cdc)),
		votes:                 collections.NewMap(sb, types.VotesKeyPrefix, "votes", collections.PairKeyCodec(collections.Uint64Key, collections.BytesKey), codec.CollValue[types.Vote](cdc)),
		params:                collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema
	return k
}

// SetDataResultForBatching stores a data result so that it is ready
// to be batched.
func (k Keeper) SetDataResultForBatching(ctx context.Context, result types.DataResult) error {
	return k.dataResults.Set(ctx, collections.Join(false, result.DrId), result)
}

// markDataResultAsBatched removes the "unbatched" variant of the given
// data result and stores a "batched" variant.
func (k Keeper) markDataResultAsBatched(ctx context.Context, result types.DataResult) error {
	err := k.dataResults.Remove(ctx, collections.Join(false, result.DrId))
	if err != nil {
		return err
	}
	return k.dataResults.Set(ctx, collections.Join(true, result.DrId), result)
}

// GetDataResults returns a list of data results under a given status
// (batched or not).
func (k Keeper) GetDataResults(ctx context.Context, batched bool) ([]types.DataResult, error) {
	var results []types.DataResult
	err := k.IterateDataResults(ctx, batched, func(_ collections.Pair[bool, string], value types.DataResult) (bool, error) {
		results = append(results, value)
		return false, nil
	})
	return results, err
}

// IterateDataResults iterates over all data results under a given
// status (batched or not) and performs a given callback function.
func (k Keeper) IterateDataResults(ctx context.Context, batched bool, cb func(key collections.Pair[bool, string], value types.DataResult) (bool, error)) error {
	rng := collections.NewPrefixedPairRange[bool, string](batched)
	if err := k.dataResults.Walk(ctx, rng, cb); err != nil {
		return err
	}
	return nil
}

func (k Keeper) SetCurrentBatchNum(ctx context.Context, batchNum uint64) error {
	return k.currentBatchNumber.Set(ctx, batchNum)
}

// IncrementCurrentBatchNum increments the batch number sequence and
// returns the new number.
func (k Keeper) IncrementCurrentBatchNum(ctx context.Context) (uint64, error) {
	next, err := k.currentBatchNumber.Next(ctx)
	return next + 1, err
}

func (k Keeper) GetCurrentBatchNum(ctx context.Context) (uint64, error) {
	batchNum, err := k.currentBatchNumber.Peek(ctx)
	if err != nil {
		return 0, err
	}
	return batchNum, nil
}

func (k Keeper) SetBatch(ctx context.Context, batch types.Batch) error {
	return k.batches.Set(ctx, batch.BatchNumber, batch)
}

// SetNewBatch increments the current batch number and stores a given
// batch at that index. It returns an error if the given batch's batch
// number does not match the next batch number.
func (k Keeper) SetNewBatch(ctx context.Context, batch types.Batch) error {
	newBatchNum, err := k.IncrementCurrentBatchNum(ctx)
	if err != nil {
		return err
	}
	if batch.BatchNumber != newBatchNum {
		return types.ErrInvalidBatchNumber.Wrapf("got %d; expected %d", batch.BatchNumber, newBatchNum)
	}
	return k.batches.Set(ctx, batch.BatchNumber, batch)
}

func (k Keeper) GetBatch(ctx context.Context, batchNum uint64) (types.Batch, error) {
	batch, err := k.batches.Get(ctx, batchNum)
	if err != nil {
		return types.Batch{}, err
	}
	return batch, nil
}

// GetCurrentBatch returns the most recently created batch. If batching
// has not begun, it returns an error ErrBatchingHasNotStarted.
func (k Keeper) GetCurrentBatch(ctx context.Context) (types.Batch, error) {
	currentBatchNum, err := k.currentBatchNumber.Peek(ctx)
	if err != nil {
		return types.Batch{}, err
	}
	if currentBatchNum == collections.DefaultSequenceStart {
		return types.Batch{}, types.ErrBatchingHasNotStarted
	}
	batch, err := k.batches.Get(ctx, currentBatchNum)
	if err != nil {
		return types.Batch{}, err
	}
	return batch, nil
}

// GetCurrentDataResultRoot returns the current batch's data result
// tree root in byte slice. If batching has not started, it returns
// an empty byte slice without an error.
func (k Keeper) GetCurrentDataResultRoot(ctx context.Context) ([]byte, error) {
	batch, err := k.GetCurrentBatch(ctx)
	if err != nil {
		if errors.Is(err, types.ErrBatchingHasNotStarted) {
			return nil, nil
		}
		return nil, err
	}
	root, err := hex.DecodeString(batch.DataResultRoot)
	if err != nil {
		return nil, err
	}
	return root, nil
}

// IterateBatches iterates over the batches and performs a given
// callback function.
func (k Keeper) IterateBatches(ctx sdk.Context, callback func(types.Batch) (stop bool)) error {
	iter, err := k.batches.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		kv, err := iter.KeyValue()
		if err != nil {
			return err
		}

		if callback(kv.Value) {
			break
		}
	}
	return nil
}

// GetAllBatches returns all batches in the store.
func (k Keeper) GetAllBatches(ctx sdk.Context) ([]types.Batch, error) {
	var batches []types.Batch
	err := k.IterateBatches(ctx, func(batch types.Batch) bool {
		batches = append(batches, batch)
		return false
	})
	if err != nil {
		return nil, err
	}
	return batches, nil
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

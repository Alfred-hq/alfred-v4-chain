package process

import (
	"fmt"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dydxprotocol/v4/lib/metrics"
)

const ConsensusRound = sdk.ContextKey("consensus_round")

// ProcessProposalHandler is responsible for ensuring that the list of txs in the proposed block are valid.
// Specifically, this validates:
//   - Tx bytes can be decoded to a valid tx.
//   - Txs are ordered correctly.
//   - Required "app-injected message" txs are included.
//   - No duplicate "app-injected message" txs are present (i.e. no "app-injected msg" in "other" txs).
//   - All messages are "valid" (i.e. `Msg.ValidateBasic` does not return errors).
//   - All proposed prices within `MsgUpdateMarketPrices` are valid according to non-deterministic validation.
//
// Note: `MsgUpdateMarketPrices` is an exception to only doing stateless validation. In order for this msg
// to be valid, the proposed price update values are compared against the local index price. Because the
// outcome depends on the local index price, this validation is dependent on "in-memory state"; therefore,
// this check is NOT stateless.
// Note: stakingKeeper and perpetualKeeper are only needed for MEV calculations.
func ProcessProposalHandler(
	txConfig client.TxConfig,
	clobKeeper ProcessClobKeeper,
	stakingKeeper ProcessStakingKeeper,
	perpetualKeeper ProcessPerpetualKeeper,
	pricesKeeper ProcessPricesKeeper,
) sdk.ProcessProposalHandler {
	// Keep track of the current block height and consensus round.
	currentBlockHeight := int64(0)
	currentConsensusRound := int64(0)

	return func(ctx sdk.Context, req abci.RequestProcessProposal) abci.ResponseProcessProposal {
		defer telemetry.ModuleMeasureSince(
			ModuleName,
			time.Now(),
			ModuleName, // purposely repeated to add the module name to the metric key.
			metrics.Handler,
			metrics.Latency,
		)

		// Update the current block height and consensus round.
		if ctx.BlockHeight() != currentBlockHeight {
			currentBlockHeight = ctx.BlockHeight()
			currentConsensusRound = 0
		} else {
			currentConsensusRound += 1
		}
		ctx = ctx.WithValue(ConsensusRound, currentConsensusRound)

		// Perform the update of smoothed prices here to ensure that smoothed prices are updated even if a block is later
		// rejected by consensus. We want smoothed prices to be updated on fixed cadence, and we are piggybacking on
		// consensus round to do so.
		if err := pricesKeeper.UpdateSmoothedPrices(ctx); err != nil {
			recordErrorMetricsWithLabel(metrics.UpdateSmoothedPrices)
			ctx.Logger().Error(fmt.Sprintf("UpdateSmoothedPrices failed, err = %v", err))
		}

		txs, err := DecodeProcessProposalTxs(ctx, txConfig.TxDecoder(), req, pricesKeeper)
		if err != nil {
			ctx.Logger().Error(fmt.Sprintf("DecodeProcessProposalTxs failed: %v", err))
			recordErrorMetricsWithLabel(metrics.Decode)
			return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}
		}

		err = txs.Validate()
		if err != nil {
			ctx.Logger().Error(fmt.Sprintf("DecodeProcessProposalTxs.Validate failed: %v", err))
			recordErrorMetricsWithLabel(metrics.Validate)
			return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}
		}

		clobKeeper.RecordMevMetrics(ctx, stakingKeeper, perpetualKeeper, txs.ProposedOperationsTx.msg)

		// Record a success metric.
		recordSuccessMetrics(ctx, txs, len(req.Txs))

		return abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}
	}
}

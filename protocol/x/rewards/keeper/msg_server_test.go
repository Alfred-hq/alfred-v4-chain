package keeper_test

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	testapp "github.com/dydxprotocol/v4/testutil/app"

	"github.com/dydxprotocol/v4/x/rewards/keeper"
	"github.com/dydxprotocol/v4/x/rewards/types"
	"github.com/stretchr/testify/require"
)

func setupMsgServer(t *testing.T) (keeper.Keeper, types.MsgServer, context.Context) {
	tApp := testapp.NewTestAppBuilder().WithTesting(t).Build()
	ctx := tApp.InitChain()
	k := tApp.App.RewardsKeeper

	return k, keeper.NewMsgServerImpl(k), sdk.WrapSDKContext(ctx)
}

func TestMsgServer(t *testing.T) {
	k, ms, ctx := setupMsgServer(t)
	require.NotNil(t, k)
	require.NotNil(t, ms)
	require.NotNil(t, ctx)
}

func TestMsgUpdateParams(t *testing.T) {
	k, ms, ctx := setupMsgServer(t)

	testCases := []struct {
		name      string
		input     *types.MsgUpdateParams
		expErr    bool
		expErrMsg string
	}{
		{
			name: "valid params",
			input: &types.MsgUpdateParams{
				Authority: k.GetAuthority(),
				Params:    types.DefaultParams(),
			},
			expErr: false,
		},
		{
			name: "invalid authority",
			input: &types.MsgUpdateParams{
				Authority: "invalid",
				Params:    types.DefaultParams(),
			},
			expErr:    true,
			expErrMsg: "invalid authority",
		},
		{
			name: "invalid params: invalid denom",
			input: &types.MsgUpdateParams{
				Authority: k.GetAuthority(),
				Params: types.Params{
					TreasuryAccount: "rewards_treasury",
					Denom:           "",
				},
			},
			expErr:    true,
			expErrMsg: "invalid denom",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ms.UpdateParams(ctx, tc.input)
			if tc.expErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

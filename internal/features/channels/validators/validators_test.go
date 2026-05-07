package validators_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models/dto"
	"github.com/Kitavrus/e_zoo/internal/features/channels/validators"
	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

func TestValidateChannelType(t *testing.T) {
	t.Parallel()
	require.NoError(t, validators.ValidateChannelType(constants.ChannelTypeErpAPI))
	require.NoError(t, validators.ValidateChannelType(constants.ChannelTypeEdiX12))
	require.ErrorIs(t, validators.ValidateChannelType("unknown"), errorspkg.ErrInvalidChannelType)
}

func TestValidateAuthMode(t *testing.T) {
	t.Parallel()
	require.NoError(t, validators.ValidateAuthMode(constants.AuthModeAPIKey))
	require.NoError(t, validators.ValidateAuthMode(constants.AuthModeOAuth2))
	require.ErrorIs(t, validators.ValidateAuthMode("xxx"), errorspkg.ErrInvalidAuthMode)
}

func TestValidateSendAttemptStatus(t *testing.T) {
	t.Parallel()
	require.NoError(t, validators.ValidateSendAttemptStatus(constants.SendAttemptStatusSuccess))
	require.Error(t, validators.ValidateSendAttemptStatus("nope"))
}

func TestValidateListFilter_Defaults(t *testing.T) {
	t.Parallel()
	f, err := validators.ValidateListFilter("", "", "", "", "", "", 0)
	require.NoError(t, err)
	require.Equal(t, constants.LimitDefault, f.Limit)
}

func TestValidateListFilter_LimitTooLarge(t *testing.T) {
	t.Parallel()
	_, err := validators.ValidateListFilter("", "", "", "", "", "", constants.LimitMax+1)
	require.Error(t, err)
}

func TestValidateListFilter_BadPOID(t *testing.T) {
	t.Parallel()
	_, err := validators.ValidateListFilter("not-uuid", "", "", "", "", "", 10)
	require.Error(t, err)
}

func TestValidateListFilter_BadFromTime(t *testing.T) {
	t.Parallel()
	_, err := validators.ValidateListFilter("", "", "", "not-rfc3339", "", "", 10)
	require.Error(t, err)
}

func TestValidateUpsertConfig_HappyPath(t *testing.T) {
	t.Parallel()
	active := true
	in, err := validators.ValidateUpsertConfig("sup-1", &dto.UpsertChannelConfigRequest{
		ChannelType: constants.ChannelTypeErpAPI,
		EndpointURL: "http://x/api/po",
		AuthMode:    constants.AuthModeAPIKey,
		IsActive:    &active,
	})
	require.NoError(t, err)
	require.Equal(t, "sup-1", in.SupplierID)
	require.Equal(t, constants.DefaultTimeoutSec, in.TimeoutSec)
	require.Equal(t, constants.DefaultRetryMax, in.RetryMax)
	require.True(t, in.IsActive)
}

func TestValidateUpsertConfig_EmptyEndpoint(t *testing.T) {
	t.Parallel()
	_, err := validators.ValidateUpsertConfig("sup-1", &dto.UpsertChannelConfigRequest{
		ChannelType: constants.ChannelTypeErpAPI,
		EndpointURL: "  ",
		AuthMode:    constants.AuthModeAPIKey,
	})
	require.Error(t, err)
}

func TestValidateUpsertConfig_NoBody(t *testing.T) {
	t.Parallel()
	_, err := validators.ValidateUpsertConfig("sup-1", nil)
	require.Error(t, err)
}

func TestValidateUpsertConfig_InvalidChannel(t *testing.T) {
	t.Parallel()
	_, err := validators.ValidateUpsertConfig("sup-1", &dto.UpsertChannelConfigRequest{
		ChannelType: "alien",
		EndpointURL: "http://x",
		AuthMode:    constants.AuthModeAPIKey,
	})
	require.ErrorIs(t, err, errorspkg.ErrInvalidChannelType)
}

func TestValidateTriggerSend(t *testing.T) {
	t.Parallel()
	v, err := validators.ValidateTriggerSend(&dto.TriggerSendRequest{MaxPos: 100})
	require.NoError(t, err)
	require.Equal(t, 100, v)

	v, err = validators.ValidateTriggerSend(nil)
	require.NoError(t, err)
	require.Equal(t, 0, v)

	_, err = validators.ValidateTriggerSend(&dto.TriggerSendRequest{MaxPos: constants.LimitMax + 1})
	require.Error(t, err)
}

// Package constants — константы фичи channels (Channel Routing, Module 7).
package constants

// Channel types (channels.supplier_channel_config.channel_type).
const (
	ChannelTypeErpAPI     = "erp_api"
	ChannelTypeEdiX12     = "edi_x12"
	ChannelTypeEdiEdifact = "edi_edifact"
	ChannelType1CXML      = "1c_xml"
	ChannelTypeCRM        = "crm"
)

// ChannelTypes — все валидные значения для validators/swagger.
var ChannelTypes = []string{
	ChannelTypeErpAPI,
	ChannelTypeEdiX12,
	ChannelTypeEdiEdifact,
	ChannelType1CXML,
	ChannelTypeCRM,
}

// IsKnownChannelType — true если значение допустимо.
func IsKnownChannelType(s string) bool {
	for _, v := range ChannelTypes {
		if v == s {
			return true
		}
	}
	return false
}

// Auth modes (channels.supplier_channel_config.auth_mode).
const (
	AuthModeAPIKey = "api_key"
	AuthModeOAuth2 = "oauth2"
	AuthModeMTLS   = "mtls"
	AuthModeNone   = "none"
)

// AuthModes — все валидные значения.
var AuthModes = []string{AuthModeAPIKey, AuthModeOAuth2, AuthModeMTLS, AuthModeNone}

// IsKnownAuthMode — true если значение допустимо.
func IsKnownAuthMode(s string) bool {
	for _, v := range AuthModes {
		if v == s {
			return true
		}
	}
	return false
}

// Send attempt statuses (channels.send_attempts.status).
const (
	SendAttemptStatusPending = "pending"
	SendAttemptStatusSuccess = "success"
	SendAttemptStatusFailed  = "failed"
	SendAttemptStatusSkipped = "skipped"
)

// SendAttemptStatuses — все валидные значения.
var SendAttemptStatuses = []string{
	SendAttemptStatusPending,
	SendAttemptStatusSuccess,
	SendAttemptStatusFailed,
	SendAttemptStatusSkipped,
}

// IsKnownSendAttemptStatus — true если значение допустимо.
func IsKnownSendAttemptStatus(s string) bool {
	for _, v := range SendAttemptStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// Default limits and timeouts.
const (
	LimitDefault            = 50
	LimitMax                = 500
	DefaultTimeoutSec       = 30
	DefaultRetryMax         = 3
	DefaultBackoffCapSec    = 30
	MaxPosPerRunDefault     = 500
	AdvisoryLockKey   int64 = 0x4348414E4E454C52 // "CHANNELR" — pg_advisory_lock id
)

// AuthCredentialsEnvDefault — общий fallback env var, если supplier-specific не задан.
const AuthCredentialsEnvDefault = "CHANNEL_AUTH_ERP_API"

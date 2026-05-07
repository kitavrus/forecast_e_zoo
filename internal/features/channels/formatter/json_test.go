package formatter_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/formatter"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

func TestJSONFormatter_Format_HappyPath(t *testing.T) {
	t.Parallel()
	f := formatter.NewJSONFormatter()
	require.Equal(t, constants.ChannelTypeErpAPI, f.ChannelType())

	body, ct, err := f.Format(formatter.PurchaseOrderPayload{
		PO: models.PurchaseOrderForSend{
			ID:         uuid.New(),
			PONumber:   "PO-2026-100",
			SupplierID: "sup-1",
			LocationID: "loc-1",
			TotalQty:   25,
			Currency:   "UAH",
			CreatedAt:  time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		},
		Lines: []formatter.POLine{{SKU: "SKU-1", Qty: 5, Unit: "pcs", UnitCost: 12.5}},
	})
	require.NoError(t, err)
	require.Equal(t, "application/json", ct)

	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	require.Equal(t, "PO-2026-100", out["po_number"])
	require.Equal(t, "PO-2026-100", out["idempotency_key"])
	require.Equal(t, "sup-1", out["supplier_id"])
	require.Equal(t, "UAH", out["currency"])
	lines, ok := out["lines"].([]any)
	require.True(t, ok)
	require.Len(t, lines, 1)
}

func TestJSONFormatter_Format_NoLines(t *testing.T) {
	t.Parallel()
	f := formatter.NewJSONFormatter()
	body, _, err := f.Format(formatter.PurchaseOrderPayload{
		PO: models.PurchaseOrderForSend{
			PONumber: "PO-1", SupplierID: "s1", LocationID: "l1", TotalQty: 1, Currency: "UAH",
			CreatedAt: time.Now().UTC(),
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, body)
}

func TestRegistry_GetByChannel(t *testing.T) {
	t.Parallel()
	jf := formatter.NewJSONFormatter()
	notImpl := &formatter.NotImplementedFormatter{Channel: constants.ChannelTypeEdiX12}
	reg := formatter.NewRegistry(jf, notImpl)

	got, err := reg.Get(constants.ChannelTypeErpAPI)
	require.NoError(t, err)
	require.NotNil(t, got)

	_, err = reg.Get(constants.ChannelType1CXML)
	require.Error(t, err)
}

func TestNotImplementedFormatter_Errors(t *testing.T) {
	t.Parallel()
	f := &formatter.NotImplementedFormatter{Channel: constants.ChannelType1CXML}
	require.Equal(t, constants.ChannelType1CXML, f.ChannelType())
	_, _, err := f.Format(formatter.PurchaseOrderPayload{})
	require.Error(t, err)
}

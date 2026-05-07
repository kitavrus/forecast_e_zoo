// Package builder — pure-function ассемблер purchase_order + po_lines из plan.
//
// Не имеет доступа к БД. Принимает уже подгруженный контекст
// (plan, plan_lines, supplier_master, product_masters) и собирает
// in-memory структуру PO готовую к INSERT.
package builder

import (
	"fmt"
	"strings"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/orders/constants"
	"github.com/Kitavrus/e_zoo/internal/features/orders/models"
	"github.com/Kitavrus/e_zoo/internal/features/orders/repository"
)

// Inputs — собранный контекст для одной сборки.
type Inputs struct {
	Plan      models.ApprovedPlan
	Lines     []models.PlanLine
	Supplier  models.SupplierMaster
	Products  map[string]models.ProductMaster // product_id → master
	PONumber  string
	CreatedAt time.Time
}

// Outputs — итог сборки. Используется service для INSERT в одной транзакции.
type Outputs struct {
	Order   repository.InsertPOInput
	Lines   []repository.BulkLine
	Notes   []string // warnings (missing supplier marts row, all-NULL pricing и т.д.)
}

// Build собирает PO + lines.
//
// Алгоритм:
//  1. Resolve currency: supplier.Currency → DefaultCurrency.
//  2. Resolve lead_time: supplier.LeadTimeDays → DefaultLeadTimeDay.
//  3. delivery_date = CreatedAt + lead_time.
//  4. Per line: pricing waterfall product.UnitPrice → supplier.DefaultUnitPrice → NULL.
//  5. total_amount = sum(line_amount) если все lines имеют unit_price; иначе NULL.
//
//nolint:cyclop,funlen // линейный pipeline с разветвлениями pricing waterfall
func Build(in Inputs) Outputs {
	notes := []string{}

	currency := in.Supplier.Currency
	if currency == "" {
		currency = constants.DefaultCurrency
		if !in.Supplier.HasMartRow {
			notes = append(notes,
				fmt.Sprintf("supplier %q has no master row, using defaults", in.Plan.SupplierID))
		}
	}

	leadTime := in.Supplier.LeadTimeDays
	if leadTime <= 0 {
		leadTime = constants.DefaultLeadTimeDay
	}

	created := in.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	delivery := created.AddDate(0, 0, leadTime).Format("2006-01-02")

	bulk := make([]repository.BulkLine, 0, len(in.Lines))
	var (
		totalQty       float64
		sumAmount      float64
		anyMissingPrice bool
		hasAnyAmount   bool
	)

	for _, l := range in.Lines {
		if l.ReorderQty <= 0 {
			continue
		}
		totalQty += l.ReorderQty
		price, source := pickPrice(l.ProductID, in.Products, in.Supplier.DefaultUnitPrice)
		var amount *float64
		if price != nil {
			a := *price * l.ReorderQty
			amount = &a
			sumAmount += a
			hasAnyAmount = true
		} else {
			anyMissingPrice = true
		}

		bulk = append(bulk, repository.BulkLine{
			ProductID:     l.ProductID,
			Qty:           l.ReorderQty,
			UnitPrice:     price,
			LineAmount:    amount,
			PricingSource: source,
		})
	}

	var totalAmount *float64
	if hasAnyAmount && !anyMissingPrice {
		totalAmount = &sumAmount
	} else if anyMissingPrice {
		notes = append(notes, "some line(s) missing unit_price; total_amount left NULL")
	}

	notesStr := joinNotes(notes)
	po := repository.InsertPOInput{
		PONumber:     in.PONumber,
		PlanID:       in.Plan.ID,
		SupplierID:   in.Plan.SupplierID,
		LocationID:   in.Plan.LocationID,
		TotalQty:     totalQty,
		TotalAmount:  totalAmount,
		Currency:     currency,
		DeliveryDate: ptrString(delivery),
		Notes:        notesStr,
	}

	return Outputs{
		Order: po,
		Lines: bulk,
		Notes: notes,
	}
}

func pickPrice(
	productID string,
	products map[string]models.ProductMaster,
	supplierDefault *float64,
) (*float64, string) {
	if pm, ok := products[productID]; ok && pm.UnitPrice != nil {
		v := *pm.UnitPrice
		return &v, constants.PricingSourceProduct
	}
	if supplierDefault != nil {
		v := *supplierDefault
		return &v, constants.PricingSourceSupplierDefault
	}
	return nil, constants.PricingSourceMissing
}

func joinNotes(notes []string) *string {
	if len(notes) == 0 {
		return nil
	}
	v := strings.Join(notes, "; ")
	return &v
}

func ptrString(s string) *string {
	return &s
}

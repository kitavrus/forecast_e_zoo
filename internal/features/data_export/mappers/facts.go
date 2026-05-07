package mappers

import (
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
)

func ReceiptLineToDTO(m models.ReceiptLine) dto.ReceiptLine {
	return dto.ReceiptLine{
		ReceiptID: m.ReceiptID, LineNo: m.LineNo, LocationID: m.LocationID, ProductID: m.ProductID,
		BarcodeScanned: m.BarcodeScanned, Qty: m.Qty, LineKind: m.LineKind,
		UnitPriceBase: m.UnitPriceBase, UnitPricePaid: m.UnitPricePaid,
		DiscountAmount: m.DiscountAmount, MarkdownPct: m.MarkdownPct, PromoID: m.PromoID,
		EventDate: m.EventDate, EventTime: m.EventTime, LoyaltyHash: m.LoyaltyHash,
		ValidFrom: m.ValidFrom, ValidTo: m.ValidTo,
		SystemTimeFrom: m.SystemTimeFrom, SystemTimeTo: m.SystemTimeTo, LoadID: m.LoadID,
	}
}

func ReceiptLineFromDTO(d dto.ReceiptLine) models.ReceiptLine {
	return models.ReceiptLine{
		ReceiptID: d.ReceiptID, LineNo: d.LineNo, LocationID: d.LocationID, ProductID: d.ProductID,
		BarcodeScanned: d.BarcodeScanned, Qty: d.Qty, LineKind: d.LineKind,
		UnitPriceBase: d.UnitPriceBase, UnitPricePaid: d.UnitPricePaid,
		DiscountAmount: d.DiscountAmount, MarkdownPct: d.MarkdownPct, PromoID: d.PromoID,
		EventDate: d.EventDate, EventTime: d.EventTime, LoyaltyHash: d.LoyaltyHash,
		ValidFrom: d.ValidFrom, ValidTo: d.ValidTo,
		SystemTimeFrom: d.SystemTimeFrom, SystemTimeTo: d.SystemTimeTo, LoadID: d.LoadID,
	}
}

func LocationStockSnapshotToDTO(m models.LocationStockSnapshot) dto.LocationStockSnapshot {
	return dto.LocationStockSnapshot{
		LocationID: m.LocationID, ProductID: m.ProductID,
		QtyOnHand: m.QtyOnHand, QtyReserved: m.QtyReserved, QtyAvailable: m.QtyAvailable,
		EventDate: m.EventDate, SnapshotAt: m.SnapshotAt,
		SystemTimeFrom: m.SystemTimeFrom, SystemTimeTo: m.SystemTimeTo, LoadID: m.LoadID,
	}
}

func LocationStockSnapshotFromDTO(d dto.LocationStockSnapshot) models.LocationStockSnapshot {
	return models.LocationStockSnapshot{
		LocationID: d.LocationID, ProductID: d.ProductID,
		QtyOnHand: d.QtyOnHand, QtyReserved: d.QtyReserved, QtyAvailable: d.QtyAvailable,
		EventDate: d.EventDate, SnapshotAt: d.SnapshotAt,
		SystemTimeFrom: d.SystemTimeFrom, SystemTimeTo: d.SystemTimeTo, LoadID: d.LoadID,
	}
}

func StockMovementToDTO(m models.StockMovement) dto.StockMovement {
	return dto.StockMovement{
		MovementID: m.MovementID, Type: m.Type,
		LocationFrom: m.LocationFrom, LocationTo: m.LocationTo,
		ProductID: m.ProductID, Qty: m.Qty,
		EventDate: m.EventDate, EventTime: m.EventTime,
		SupplierID: m.SupplierID, Details: m.Details,
		SystemTimeFrom: m.SystemTimeFrom, SystemTimeTo: m.SystemTimeTo, LoadID: m.LoadID,
	}
}

func StockMovementFromDTO(d dto.StockMovement) models.StockMovement {
	return models.StockMovement{
		MovementID: d.MovementID, Type: d.Type,
		LocationFrom: d.LocationFrom, LocationTo: d.LocationTo,
		ProductID: d.ProductID, Qty: d.Qty,
		EventDate: d.EventDate, EventTime: d.EventTime,
		SupplierID: d.SupplierID, Details: d.Details,
		SystemTimeFrom: d.SystemTimeFrom, SystemTimeTo: d.SystemTimeTo, LoadID: d.LoadID,
	}
}

func SupplierStockSnapshotToDTO(m models.SupplierStockSnapshot) dto.SupplierStockSnapshot {
	return dto.SupplierStockSnapshot{
		SupplierID: m.SupplierID, ProductID: m.ProductID, QtyAvailable: m.QtyAvailable,
		SnapshotAt: m.SnapshotAt, EventDate: m.EventDate, LoadID: m.LoadID,
	}
}

func SupplierStockSnapshotFromDTO(d dto.SupplierStockSnapshot) models.SupplierStockSnapshot {
	return models.SupplierStockSnapshot{
		SupplierID: d.SupplierID, ProductID: d.ProductID, QtyAvailable: d.QtyAvailable,
		SnapshotAt: d.SnapshotAt, EventDate: d.EventDate, LoadID: d.LoadID,
	}
}

func RejectEntryToDTO(m models.RejectEntry) dto.RejectLogEntry {
	return dto.RejectLogEntry{
		ID: m.ID, LoadID: m.LoadID, Entity: m.Entity, PKValue: m.PKValue,
		Severity: string(m.Severity), Reason: m.Reason, Raw: m.Raw, DetectedAt: m.DetectedAt,
	}
}

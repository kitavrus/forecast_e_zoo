// Package mappers содержит lossless-перекладывание полей между domain
// models и DTO. Никакой логики, только присваивания.
package mappers

import (
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models"
	"github.com/Kitavrus/e_zoo/internal/features/data_export/models/dto"
)

// ProductToDTO — domain → DTO.
func ProductToDTO(m models.Product) dto.Product {
	return dto.Product{
		ProductID:            m.ProductID,
		Name:                 m.Name,
		Brand:                m.Brand,
		Manufacturer:         m.Manufacturer,
		CategoryID:           m.CategoryID,
		CategoryPath:         m.CategoryPath,
		WeightKg:             m.WeightKg,
		PalletQty:            m.PalletQty,
		ShelfLifeDays:        m.ShelfLifeDays,
		StorageTempMin:       m.StorageTempMin,
		StorageTempMax:       m.StorageTempMax,
		RequiresPrescription: m.RequiresPrescription,
		IsDangerousGoods:     m.IsDangerousGoods,
		Status:               m.Status,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
		LoadID:               m.LoadID,
	}
}

// ProductFromDTO — DTO → domain.
func ProductFromDTO(d dto.Product) models.Product {
	return models.Product{
		ProductID:            d.ProductID,
		Name:                 d.Name,
		Brand:                d.Brand,
		Manufacturer:         d.Manufacturer,
		CategoryID:           d.CategoryID,
		CategoryPath:         d.CategoryPath,
		WeightKg:             d.WeightKg,
		PalletQty:            d.PalletQty,
		ShelfLifeDays:        d.ShelfLifeDays,
		StorageTempMin:       d.StorageTempMin,
		StorageTempMax:       d.StorageTempMax,
		RequiresPrescription: d.RequiresPrescription,
		IsDangerousGoods:     d.IsDangerousGoods,
		Status:               d.Status,
		CreatedAt:            d.CreatedAt,
		UpdatedAt:            d.UpdatedAt,
		LoadID:               d.LoadID,
	}
}

func ProductBarcodeToDTO(m models.ProductBarcode) dto.ProductBarcode {
	return dto.ProductBarcode{
		Barcode: m.Barcode, ProductID: m.ProductID, PackQty: m.PackQty,
		IsPrimary: m.IsPrimary, CountryOrigin: m.CountryOrigin,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, LoadID: m.LoadID,
	}
}

func ProductBarcodeFromDTO(d dto.ProductBarcode) models.ProductBarcode {
	return models.ProductBarcode{
		Barcode: d.Barcode, ProductID: d.ProductID, PackQty: d.PackQty,
		IsPrimary: d.IsPrimary, CountryOrigin: d.CountryOrigin,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, LoadID: d.LoadID,
	}
}

func CategoryToDTO(m models.Category) dto.Category {
	return dto.Category{CategoryID: m.CategoryID, ParentID: m.ParentID, Level: m.Level,
		Name: m.Name, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, LoadID: m.LoadID}
}

func CategoryFromDTO(d dto.Category) models.Category {
	return models.Category{CategoryID: d.CategoryID, ParentID: d.ParentID, Level: d.Level,
		Name: d.Name, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, LoadID: d.LoadID}
}

func LocationToDTO(m models.Location) dto.Location {
	return dto.Location{
		LocationID: m.LocationID, Type: m.Type, Name: m.Name, Address: m.Address,
		City: m.City, Region: m.Region, OpenedAt: m.OpenedAt, ClosedAt: m.ClosedAt,
		Status: m.Status, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, LoadID: m.LoadID,
	}
}

func LocationFromDTO(d dto.Location) models.Location {
	return models.Location{
		LocationID: d.LocationID, Type: d.Type, Name: d.Name, Address: d.Address,
		City: d.City, Region: d.Region, OpenedAt: d.OpenedAt, ClosedAt: d.ClosedAt,
		Status: d.Status, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, LoadID: d.LoadID,
	}
}

func SupplierToDTO(m models.Supplier) dto.Supplier {
	return dto.Supplier{
		SupplierID: m.SupplierID, Name: m.Name, INN: m.INN, GLN: m.GLN,
		PaymentTerms: m.PaymentTerms, EDIProfile: m.EDIProfile,
		Status: m.Status, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, LoadID: m.LoadID,
	}
}

func SupplierFromDTO(d dto.Supplier) models.Supplier {
	return models.Supplier{
		SupplierID: d.SupplierID, Name: d.Name, INN: d.INN, GLN: d.GLN,
		PaymentTerms: d.PaymentTerms, EDIProfile: d.EDIProfile,
		Status: d.Status, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, LoadID: d.LoadID,
	}
}

func SupplySpecToDTO(m models.SupplySpec) dto.SupplySpec {
	return dto.SupplySpec{
		SupplierID: m.SupplierID, ProductID: m.ProductID, LocationID: m.LocationID,
		Priority: m.Priority, MinOrderQty: m.MinOrderQty, PurchasePrice: m.PurchasePrice,
		Currency: m.Currency, LeadTimeDays: m.LeadTimeDays, PackSize: m.PackSize,
		EffectiveFrom: m.EffectiveFrom, EffectiveTo: m.EffectiveTo,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, LoadID: m.LoadID,
	}
}

func SupplySpecFromDTO(d dto.SupplySpec) models.SupplySpec {
	return models.SupplySpec{
		SupplierID: d.SupplierID, ProductID: d.ProductID, LocationID: d.LocationID,
		Priority: d.Priority, MinOrderQty: d.MinOrderQty, PurchasePrice: d.PurchasePrice,
		Currency: d.Currency, LeadTimeDays: d.LeadTimeDays, PackSize: d.PackSize,
		EffectiveFrom: d.EffectiveFrom, EffectiveTo: d.EffectiveTo,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, LoadID: d.LoadID,
	}
}

func PromoToDTO(m models.Promo) dto.Promo {
	return dto.Promo{
		PromoID: m.PromoID, ProductID: m.ProductID, LocationID: m.LocationID,
		Type: m.Type, DiscountPct: m.DiscountPct, PromoPriceWithVAT: m.PromoPriceWithVAT,
		DateFrom: m.DateFrom, DateTo: m.DateTo,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, LoadID: m.LoadID,
	}
}

func PromoFromDTO(d dto.Promo) models.Promo {
	return models.Promo{
		PromoID: d.PromoID, ProductID: d.ProductID, LocationID: d.LocationID,
		Type: d.Type, DiscountPct: d.DiscountPct, PromoPriceWithVAT: d.PromoPriceWithVAT,
		DateFrom: d.DateFrom, DateTo: d.DateTo,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, LoadID: d.LoadID,
	}
}

func OrderRuleToDTO(m models.OrderRule) dto.OrderRule {
	return dto.OrderRule{
		RuleID: m.RuleID, Scope: m.Scope, ScopeRef: m.ScopeRef, LocationID: m.LocationID,
		SafetyStockDays: m.SafetyStockDays, ServiceLevelPct: m.ServiceLevelPct,
		OverrideMOQ: m.OverrideMOQ, EffectiveFrom: m.EffectiveFrom,
		EffectiveTo: m.EffectiveTo, LoadID: m.LoadID,
	}
}

func OrderRuleFromDTO(d dto.OrderRule) models.OrderRule {
	return models.OrderRule{
		RuleID: d.RuleID, Scope: d.Scope, ScopeRef: d.ScopeRef, LocationID: d.LocationID,
		SafetyStockDays: d.SafetyStockDays, ServiceLevelPct: d.ServiceLevelPct,
		OverrideMOQ: d.OverrideMOQ, EffectiveFrom: d.EffectiveFrom,
		EffectiveTo: d.EffectiveTo, LoadID: d.LoadID,
	}
}

func SupplyPlanToDTO(m models.SupplyPlan) dto.SupplyPlan {
	return dto.SupplyPlan{
		PlanID: m.PlanID, SupplierID: m.SupplierID, LocationID: m.LocationID,
		PlannedDate: m.PlannedDate, SlotTime: m.SlotTime, CutoffAt: m.CutoffAt,
		Status: m.Status, LoadID: m.LoadID,
	}
}

func SupplyPlanFromDTO(d dto.SupplyPlan) models.SupplyPlan {
	return models.SupplyPlan{
		PlanID: d.PlanID, SupplierID: d.SupplierID, LocationID: d.LocationID,
		PlannedDate: d.PlannedDate, SlotTime: d.SlotTime, CutoffAt: d.CutoffAt,
		Status: d.Status, LoadID: d.LoadID,
	}
}

func StoreAssortmentToDTO(m models.StoreAssortment) dto.StoreAssortment {
	return dto.StoreAssortment{
		LocationID: m.LocationID, ProductID: m.ProductID, LifecycleState: m.LifecycleState,
		AssortmentClass: m.AssortmentClass, PriceMin: m.PriceMin, PriceMax: m.PriceMax,
		EffectiveFrom: m.EffectiveFrom, EffectiveTo: m.EffectiveTo,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, LoadID: m.LoadID,
	}
}

func StoreAssortmentFromDTO(d dto.StoreAssortment) models.StoreAssortment {
	return models.StoreAssortment{
		LocationID: d.LocationID, ProductID: d.ProductID, LifecycleState: d.LifecycleState,
		AssortmentClass: d.AssortmentClass, PriceMin: d.PriceMin, PriceMax: d.PriceMax,
		EffectiveFrom: d.EffectiveFrom, EffectiveTo: d.EffectiveTo,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, LoadID: d.LoadID,
	}
}

func MasterChangeLogToDTO(m models.MasterChangeLogEntry) dto.MasterChangeLogEntry {
	return dto.MasterChangeLogEntry{
		EventID: m.EventID, Entity: m.Entity, EntityPK: m.EntityPK, Field: m.Field,
		OldValue: m.OldValue, NewValue: m.NewValue, ChangedAt: m.ChangedAt, LoadID: m.LoadID,
	}
}

func MasterChangeLogFromDTO(d dto.MasterChangeLogEntry) models.MasterChangeLogEntry {
	return models.MasterChangeLogEntry{
		EventID: d.EventID, Entity: d.Entity, EntityPK: d.EntityPK, Field: d.Field,
		OldValue: d.OldValue, NewValue: d.NewValue, ChangedAt: d.ChangedAt, LoadID: d.LoadID,
	}
}

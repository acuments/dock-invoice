// Package testfixture holds synthetic company, bank, and customer data for
// tests, visual screenshots, and committed sample JSON/PDF fixtures. Every
// GSTIN, PAN, account number, phone, and email here is deliberately fake.
package testfixture

import (
	"time"

	"dock-invoice/internal/model"
	"dock-invoice/internal/money"
)

const (
	SellerName      = "Aeryn Software Labs LLP"
	SellerGSTIN     = "33AAECA1234A1Z9"
	SellerPAN       = "AAECA1234A"
	SellerPhone     = "9876543210"
	SellerEmail     = "billing@example.com"
	SellerStateCode = "33"
	SellerStateName = "Tamil Nadu"
	SellerCity      = "Chennai"
	SellerPincode   = "600006"

	BankAccount = "10012345678"
	BankIFSC    = "SBIN0001234"
	BankName    = "Example National Bank"
	BankBranch  = "Anna Nagar Branch"
	BankSwift   = "EXAMPLEBBXXX"

	ExportCustomerName = "Northwind Trading Co."
	DomesticCustomer      = "Fabrikam Systems Pvt Ltd"
	DomesticCustomerGSTIN = "33BBBBB1111B2Z6"

	// LongSingleLineBillingAddress is a deliberately fake pasted address used
	// by PDF wrap regression tests (must stay long enough to wrap in-column).
	LongSingleLineBillingAddress = "FABRIKAM SYSTEMS PRIVATE LIMITED, UNIT 402, EXAMPLE TOWER, 100 SAMPLE MARG, T NAGAR, Chennai, Tamil Nadu, India"
)

// SellerCompany returns a fake seller letterhead snapshot.
func SellerCompany() model.Company {
	return model.Company{
		Name: SellerName,
		AddressLines: []string{
			"4th Floor, Prestige Palladium",
			"129 Greams Road",
		},
		City:      SellerCity,
		Pincode:   SellerPincode,
		StateCode: SellerStateCode,
		StateName: SellerStateName,
		Phone:     SellerPhone,
		Email:     SellerEmail,
		GSTIN:     SellerGSTIN,
		PAN:       SellerPAN,
	}
}

// SellerBank returns fake bank details for tests and fixtures.
func SellerBank() model.Bank {
	return model.Bank{
		AccountNumber: BankAccount,
		IFSC:          BankIFSC,
		BankName:      BankName,
		BranchName:    BankBranch,
		SwiftCode:     BankSwift,
	}
}

// DomesticCustomerRecord returns a fake Indian domestic buyer.
func DomesticCustomerRecord() model.Customer {
	return model.Customer{
		Name:  DomesticCustomer,
		GSTIN: DomesticCustomerGSTIN,
		BillingAddress: []string{
			"Unit 402, Example Tower",
			"100 Sample Marg, T Nagar",
			"Chennai",
			"600017 Tamil Nadu",
		},
		ShippingAddress: []string{
			"Unit 402, Example Tower",
			"100 Sample Marg, T Nagar",
			"Chennai",
			"600017 Tamil Nadu",
		},
	}
}

// ExportCustomer returns a fake US export buyer.
func ExportCustomer() model.Customer {
	return model.Customer{
		Name: ExportCustomerName,
		BillingAddress: []string{
			"1209 Orange Street",
			"Wilmington, DE 19801",
			"United States of America",
		},
		ShippingAddress: []string{
			"1209 Orange Street",
			"Wilmington, DE 19801",
			"United States of America",
		},
	}
}

// Settings returns fake Settings suitable for UI visual tests.
func Settings() model.Settings {
	return model.Settings{
		Company: SellerCompany(),
		Bank:    SellerBank(),
		Defaults: model.Defaults{
			HSNSAC:          "998314",
			Unit:            "UNT",
			TaxRatePct:      1800,
			PaymentTermDays: 15,
			Currency:        "USD",
			CopyType:        "ORIGINAL",
		},
		NumberPatterns: map[model.InvoiceType]string{
			model.InvoiceExportLUT:  "AEX{FY}-{SEQ}",
			model.InvoiceExportIGST: "AEXI{FY}-{SEQ}",
			model.InvoiceDomestic:   "DOM{FY}-{SEQ}",
		},
		LastFXFactor: "83.20",
	}
}

// ExportLUTInvoice returns a fully-populated export-under-LUT invoice used by
// PDF golden tests. fxFactor is typically money.ParseRate("83.2").
func ExportLUTInvoice(fxFactor money.Rate) *model.Invoice {
	return &model.Invoice{
		Type:            model.InvoiceExportLUT,
		Number:          "AEX2425-1",
		Date:            time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC),
		DueDate:         time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC),
		CopyType:        "ORIGINAL",
		PlaceOfSupply:   "97-Other Territory",
		CountryOfSupply: "UNITED STATES OF AMERICA",
		Currency:        "USD",
		FXFactor:        fxFactor,
		Customer:        ExportCustomer(),
		Seller:          SellerCompany(),
		Bank:            SellerBank(),
		Items: []model.LineItem{
			{
				Description: "Software Development Services",
				HSNSAC:      "998314",
				Quantity:    money.Qty(100),
				Unit:        "UNT",
				RateUSD:     money.Amount(400000),
				TaxRatePct:  1800,
			},
		},
	}
}

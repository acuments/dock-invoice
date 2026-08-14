package pdf

// Label constants for every static string printed on the invoice. Centralised
// here so they are easy to audit and change in one place.
//
// Note: the reference document (INV-01.pdf) mislabels the shipping bill
// number field as "Shipping Billing No." — that is a typo in the source and
// is intentionally NOT reproduced here. We print the correct
// "Shipping Bill No." instead.
const (
	LabelTitle = "TAX INVOICE"

	LabelCopyOriginal   = "ORIGINAL"
	LabelCopyDuplicate  = "DUPLICATE"
	LabelCopyTriplicate = "TRIPLICATE"
	LabelForRecipient   = "For Recipient"
	LabelForTransporter = "For Transporter"
	LabelForSupplier    = "For Supplier"

	LabelGSTIN       = "GSTIN"
	LabelState       = "State"
	LabelPAN         = "PAN"
	LabelInvoiceDate = "Invoice Date"
	LabelInvoiceNo   = "Invoice No."
	LabelReferenceNo = "Reference No."

	LabelCustomerName    = "Customer Name"
	LabelCustomerGSTIN   = "Customer GSTIN"
	LabelBillingAddress  = "Billing Address"
	LabelShippingAddress = "Shipping Address"

	LabelPlaceOfSupply   = "Place of Supply"
	LabelDueDate         = "Due Date"
	LabelCountryOfSupply = "Country of Supply"

	// Corrected label — see package doc comment above.
	LabelShippingBillNo   = "Shipping Bill No."
	LabelShippingBillDate = "Shipping Bill Date"
	LabelShippingPortCode = "Shipping Port Code"

	LabelCurrency         = "Currency"
	LabelConversionFactor = "Conversion Factor"

	// Item table column headers.
	ColItem         = "Item"
	ColHSNSAC       = "HSN /\nSAC"
	ColQuantity     = "Quantity"
	ColRatePerItem  = "Rate / Item\n(₹)"
	ColDiscount     = "Discount\n(₹)"
	ColTaxableValue = "Taxable Value\n(₹)"
	ColIGST         = "IGST\n(₹)"
	ColCGST         = "CGST\n(₹)"
	ColSGST         = "SGST\n(₹)"
	ColCESS         = "CESS\n(₹)"
	ColTotalGroup   = "Total"
	ColTotalINR     = "INR (₹)"
	ColTotalUSD     = "USD"
	ColTotalINROnly = "Total\n(₹)"

	LabelTableTotal = "Total"

	// HSN/SAC summary (opt-in for domestic invoices via Settings).
	LabelHSNSummary     = "HSN/SAC Summary:"
	ColHSNSummaryCode   = "HSN/SAC"
	ColHSNTaxableAmount = "Taxable Amount"
	ColHSNRate          = "Rate"
	ColHSNAmount        = "Amount"
	ColHSNTotalTax      = "Total Tax Amount"
	ColCGSTGroup        = "CGST"
	ColSGSTGroup        = "SGST"
	ColIGSTGroup        = "IGST"

	LabelTaxableAmount = "Taxable Amount"
	LabelTotalTax      = "Total Tax"
	LabelRoundOff      = "Round Off"
	LabelTotalValue    = "Total Value"

	LabelAmountInWordsINR = "Total amount (in words)"
	LabelAmountInWordsUSD = "Invoice Total in words (USD)"

	LabelBankDetails   = "Bank Details:"
	LabelAccountNumber = "Account Number"
	LabelIFSC          = "IFSC"
	LabelBankName      = "Bank Name:"
	LabelBranchName    = "Branch Name:"
	LabelSwiftCode     = "Swift Code:"
	LabelUPIID         = "UPI ID"
	LabelScanToPayUPI  = "Scan to pay via UPI"

	LabelFor                 = "For"
	LabelAuthorisedSignatory = "Authorised Signatory"

	// DeclarationClassicDefault is what the classic layout prints in its
	// Declaration box when the invoice type has no configured declaration
	// (domestic, where DeclarationDomestic is empty). That layout draws the
	// box as a permanent part of the form, so it needs wording rather than an
	// empty captioned rectangle; this is the text the reference document
	// carries. Settings → Declarations overrides it.
	DeclarationClassicDefault = "We declare that this invoice shows the actual price of the goods described and that all particulars are true and correct."

	DeclarationLUT      = "SUPPLY MEANT FOR EXPORT UNDER BOND OR LETTER OF UNDERTAKING WITHOUT PAYMENT OF INTEGRATED TAX"
	DeclarationIGST     = "SUPPLY MEANT FOR EXPORT ON PAYMENT OF INTEGRATED TAX"
	DeclarationDomestic = ""
)

// Classic (Tally-style, monochrome) layout labels. Kept separate from the
// modern-layout constants above even where the printed text coincides, so
// the two layouts can evolve their wording independently without risking a
// byte-for-byte change to the other's output.
const (
	LabelClassicTitle = "Tax INVOICE"

	LabelClassicCopyOriginal   = "(ORIGINAL FOR RECIPIENT)"
	LabelClassicCopyDuplicate  = "(DUPLICATE FOR TRANSPORTER)"
	LabelClassicCopyTriplicate = "(TRIPLICATE FOR SUPPLIER)"

	LabelClassicConsignee = "Consignee (Ship to)"
	LabelClassicBuyer     = "Buyer (Bill to)"

	LabelClassicInvoiceNo       = "Invoice No."
	LabelClassicDated           = "Dated"
	LabelClassicModeTerms       = "Mode/Terms of Payment"
	LabelClassicReferenceNoDate = "Reference No. & Date."
	LabelClassicDestination     = "Destination"
	LabelClassicPortCode        = "Port Code"
	LabelClassicTermsOfDeliver  = "Terms of Delivery"

	// Item table column headers.
	ColClassicSlNo   = "Sl\nNo."
	ColClassicDesc   = "Description of Goods"
	ColClassicHSNSAC = "HSN/SAC"
	ColClassicQty    = "Quantity"
	ColClassicRate   = "Rate"
	ColClassicPer    = "per"
	ColClassicAmount = "Amount"

	LabelClassicCentralGST = "Central GST"
	LabelClassicStateGST   = "State GST"
	LabelClassicIGST       = "IGST"
	LabelClassicCESS       = "CESS"
	LabelClassicRoundOff   = "Round Off"
	LabelClassicEOE        = "E. & O.E"

	LabelClassicAmountChargeable = "Amount Chargeable (in words)"
	LabelClassicTaxAmountWords   = "Tax Amount (in words) : "

	LabelClassicDeclaration        = "Declaration"
	LabelClassicBankDetailsHeading = "Company's Bank Details"
	LabelClassicAcHolderName       = "A/c Holder's Name"
	LabelClassicBankName           = "Bank Name"
	LabelClassicAcNo               = "A/c No."
	LabelClassicBranchIFSC         = "Branch & IFS Code"
	LabelClassicSwift              = "SWIFT"
	LabelClassicUPIID              = "UPI ID"

	LabelClassicCustomerSeal      = "Customer's Seal and Signature"
	LabelClassicJurisdictionFmt   = "SUBJECT TO %s JURISDICTION"
	LabelClassicComputerGenerated = "This is a Computer Generated Invoice"
)

package handlers

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"rloco-backend/internal/models"

	"github.com/go-pdf/fpdf"
)

// Brand palette, matching the SVG wordmark (RlocoLogo.tsx: "#F1B041" mark,
// "#1A1A1A" letterforms) and the admin theme's neutral grays, so generated
// documents look like they came from the same brand as the site/emails.
var (
	colorInk      = [3]int{26, 26, 26}    // headings, body text
	colorMuted    = [3]int{102, 102, 102} // secondary/meta text
	colorGold     = [3]int{241, 176, 65}  // brand accent rule
	colorBorder   = [3]int{221, 221, 221}
	colorTableBg  = [3]int{248, 248, 248}
	pageMarginMM  = 15.0
	contentWidthA = 210.0 - 2*15.0 // A4 width minus margins
)

// newBrandedPDF sets up an A4 document and returns it along with a
// translator that must wrap every dynamic string before it reaches the PDF
// (core fonts are WinAnsi/CP1252, not UTF-8 — without this, any non-ASCII
// character such as "·" or an accented name renders as mojibake).
func newBrandedPDF() (*fpdf.Fpdf, func(string) string) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 20)
	pdf.SetMargins(pageMarginMM, pageMarginMM, pageMarginMM)
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	return pdf, tr
}

func setColor3(pdf *fpdf.Fpdf, c [3]int, kind string) {
	switch kind {
	case "text":
		pdf.SetTextColor(c[0], c[1], c[2])
	case "fill":
		pdf.SetFillColor(c[0], c[1], c[2])
	case "draw":
		pdf.SetDrawColor(c[0], c[1], c[2])
	}
}

// formatMoney renders a USD amount with thousands separators, e.g. 1234.5 -> "$1,234.50".
func formatMoney(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]

	var grouped strings.Builder
	for i, digit := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	out := "$" + grouped.String() + "." + parts[1]
	if neg {
		out = "-" + out
	}
	return out
}

// fitText truncates an already-translated txt with an ellipsis so it renders
// within maxWidth under the PDF's current font.
func fitText(pdf *fpdf.Fpdf, txt string, maxWidth float64) string {
	if pdf.GetStringWidth(txt) <= maxWidth {
		return txt
	}
	const ellipsis = "..."
	for len(txt) > 0 {
		txt = txt[:len(txt)-1]
		if pdf.GetStringWidth(txt+ellipsis) <= maxWidth {
			return txt + ellipsis
		}
	}
	return ellipsis
}

func nonEmpty(vals ...string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// drawDocumentHeader renders the shared brand block (wordmark + company
// details on the left, document title/number/date on the right) and returns
// the Y position content should continue from.
func drawDocumentHeader(pdf *fpdf.Fpdf, tr func(string) string, company companyInfo, docTitle, docNumber string, docDate time.Time, statusLine string) float64 {
	startY := pdf.GetY()

	// Wordmark: brand name in ink, with a gold accent rule underneath — a
	// text-based mark (no bitmap logo asset exists in the repo yet) styled to
	// match the site's actual wordmark colors.
	pdf.SetFont("Helvetica", "B", 22)
	setColor3(pdf, colorInk, "text")
	pdf.SetXY(pageMarginMM, startY)
	pdf.CellFormat(80, 10, tr(strings.ToUpper(company.Name)), "", 2, "L", false, 0, "")

	setColor3(pdf, colorGold, "draw")
	pdf.SetLineWidth(0.8)
	pdf.Line(pageMarginMM, startY+9.5, pageMarginMM+22, startY+9.5)

	pdf.SetFont("Helvetica", "", 9)
	setColor3(pdf, colorMuted, "text")
	pdf.SetXY(pageMarginMM, startY+12)
	lines := []string{}
	if company.Tagline != "" {
		lines = append(lines, company.Tagline)
	}
	if company.Address != "" {
		lines = append(lines, company.Address)
	}
	if company.Email != "" || company.Phone != "" {
		lines = append(lines, strings.TrimSpace(strings.Join(nonEmpty(company.Email, company.Phone), "   |   ")))
	}
	if company.GSTIN != "" {
		lines = append(lines, "GSTIN: "+company.GSTIN)
	}
	for _, l := range lines {
		pdf.SetX(pageMarginMM)
		pdf.CellFormat(120, 5, tr(l), "", 2, "L", false, 0, "")
	}

	// Right-aligned document meta block.
	metaX := pageMarginMM + contentWidthA - 80
	pdf.SetXY(metaX, startY)
	pdf.SetFont("Helvetica", "B", 20)
	setColor3(pdf, colorInk, "text")
	pdf.CellFormat(80, 10, tr(docTitle), "", 2, "R", false, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	setColor3(pdf, colorMuted, "text")
	pdf.SetX(metaX)
	pdf.CellFormat(80, 5, tr("No. "+docNumber), "", 2, "R", false, 0, "")
	pdf.SetX(metaX)
	pdf.CellFormat(80, 5, tr(docDate.Format("January 2, 2006")), "", 2, "R", false, 0, "")
	if statusLine != "" {
		pdf.SetX(metaX)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(80, 5, tr(statusLine), "", 2, "R", false, 0, "")
	}

	afterY := pdf.GetY() + 6
	if afterY < startY+42 {
		afterY = startY + 42
	}
	setColor3(pdf, colorBorder, "draw")
	pdf.SetLineWidth(0.3)
	pdf.Line(pageMarginMM, afterY, pageMarginMM+contentWidthA, afterY)
	return afterY + 8
}

// drawAddressBlock renders a labeled name/address block starting at (x, y)
// and returns the Y position immediately below the last line it drew, so
// callers never have to guess a fixed block height.
func drawAddressBlock(pdf *fpdf.Fpdf, tr func(string) string, x, y, width float64, label string, si models.ShippingInfo, includeContact bool) float64 {
	pdf.SetXY(x, y)
	pdf.SetFont("Helvetica", "B", 9)
	setColor3(pdf, colorMuted, "text")
	pdf.CellFormat(width, 5, tr(strings.ToUpper(label)), "", 2, "L", false, 0, "")

	pdf.SetFont("Helvetica", "B", 10)
	setColor3(pdf, colorInk, "text")
	pdf.SetX(x)
	pdf.CellFormat(width, 6, tr(strings.TrimSpace(si.FirstName+" "+si.LastName)), "", 2, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 9.5)
	setColor3(pdf, colorMuted, "text")
	addrLines := []string{si.Address, strings.TrimSpace(fmt.Sprintf("%s, %s %s", si.City, si.State, si.ZipCode)), si.Country}
	if includeContact {
		addrLines = append(addrLines, nonEmpty(si.Email, si.Phone)...)
	}
	for _, l := range addrLines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		pdf.SetX(x)
		pdf.MultiCell(width, 5, tr(l), "", "L", false)
	}
	return pdf.GetY()
}

// writeInvoicePDF renders a customer-facing invoice: full pricing breakdown,
// billing/shipping address and payment status. No separate billing address
// exists on the order model, so a single address block is shown honestly
// rather than implying two different addresses.
func writeInvoicePDF(order *models.Order, company companyInfo) ([]byte, error) {
	pdf, tr := newBrandedPDF()

	status := "PAID"
	if order.PaymentStatus != "" {
		status = strings.ToUpper(order.PaymentStatus)
	}
	y := drawDocumentHeader(pdf, tr, company, "INVOICE", order.OrderNumber, order.CreatedAt, "Payment: "+status)

	y = drawAddressBlock(pdf, tr, pageMarginMM, y, 100, "Billed & Shipped To", order.ShippingInfo, true)
	y += 6

	setColor3(pdf, colorBorder, "draw")
	pdf.Line(pageMarginMM, y, pageMarginMM+contentWidthA, y)
	y += 6

	// Items table.
	colItem, colQty, colPrice, colAmount := 90.0, 22.0, 34.0, 34.0
	pdf.SetXY(pageMarginMM, y)
	setColor3(pdf, colorTableBg, "fill")
	pdf.SetFont("Helvetica", "B", 9)
	setColor3(pdf, colorInk, "text")
	pdf.CellFormat(colItem, 8, tr("Item"), "", 0, "L", true, 0, "")
	pdf.CellFormat(colQty, 8, tr("Qty"), "", 0, "C", true, 0, "")
	pdf.CellFormat(colPrice, 8, tr("Unit Price"), "", 0, "R", true, 0, "")
	pdf.CellFormat(colAmount, 8, tr("Amount"), "", 2, "R", true, 0, "")
	y = pdf.GetY()

	pdf.SetFont("Helvetica", "", 9.5)
	for _, item := range order.Items {
		rowH := 8.0
		hasGiftNote := item.IsGift && (item.GiftWrapColor != "" || item.GiftMessage != "")
		if hasGiftNote {
			rowH = 12.0
		}
		pdf.SetXY(pageMarginMM, y)
		setColor3(pdf, colorInk, "text")
		name := item.ProductName
		if item.Size != "" {
			name += " (" + item.Size + ")"
		}
		pdf.CellFormat(colItem, 8, fitText(pdf, tr(name), colItem-2), "B", 0, "L", false, 0, "")
		pdf.CellFormat(colQty, 8, strconv.Itoa(item.Quantity), "B", 0, "C", false, 0, "")
		pdf.CellFormat(colPrice, 8, formatMoney(item.Price), "B", 0, "R", false, 0, "")
		pdf.CellFormat(colAmount, 8, formatMoney(item.Price*float64(item.Quantity)), "B", 2, "R", false, 0, "")

		if hasGiftNote {
			pdf.SetXY(pageMarginMM, y+8)
			pdf.SetFont("Helvetica", "I", 8)
			setColor3(pdf, colorMuted, "text")
			note := "Gift wrapped"
			if item.GiftWrapColor != "" {
				note += " (" + item.GiftWrapColor + ")"
			}
			pdf.CellFormat(colItem+colQty+colPrice+colAmount, 4, fitText(pdf, tr(note), colItem+colQty+colPrice+colAmount-2), "B", 2, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 9.5)
		}
		y += rowH
	}
	y += 4

	// Totals.
	totalsX := pageMarginMM + contentWidthA - 80
	rows := [][2]string{
		{"Subtotal", formatMoney(order.Subtotal)},
	}
	if order.Discount > 0 {
		rows = append(rows, [2]string{"Discount", "-" + formatMoney(order.Discount)})
	}
	rows = append(rows, [2]string{"Shipping", formatMoney(order.ShippingCost)})
	if order.GiftPackingCharge > 0 {
		rows = append(rows, [2]string{"Gift Packing", formatMoney(order.GiftPackingCharge)})
	}
	rows = append(rows, [2]string{"Tax", formatMoney(order.Tax)})

	pdf.SetFont("Helvetica", "", 9.5)
	for _, r := range rows {
		pdf.SetXY(totalsX, y)
		setColor3(pdf, colorMuted, "text")
		pdf.CellFormat(46, 6, tr(r[0]), "", 0, "L", false, 0, "")
		setColor3(pdf, colorInk, "text")
		pdf.CellFormat(34, 6, r[1], "", 2, "R", false, 0, "")
		y += 6
	}

	setColor3(pdf, colorBorder, "draw")
	pdf.Line(totalsX, y+1, totalsX+80, y+1)
	y += 4
	pdf.SetXY(totalsX, y)
	pdf.SetFont("Helvetica", "B", 12)
	setColor3(pdf, colorInk, "text")
	pdf.CellFormat(46, 8, tr("Total"), "", 0, "L", false, 0, "")
	pdf.CellFormat(34, 8, formatMoney(order.Total), "", 2, "R", false, 0, "")
	y += 8

	if order.PaymentMethod != "" {
		method := strings.ToUpper(order.PaymentMethod[:1]) + order.PaymentMethod[1:]
		pdf.SetXY(totalsX, y)
		pdf.SetFont("Helvetica", "", 8.5)
		setColor3(pdf, colorMuted, "text")
		pdf.CellFormat(80, 5, tr("Payment method: "+method), "", 2, "R", false, 0, "")
	}

	drawDocumentFooter(pdf, tr, company, "This is a computer-generated invoice and does not require a signature.")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writePackingSlipPDF renders a fulfillment-facing packing slip: ship-to
// address and line items only (no prices, no payment/billing data), plus any
// gift message so warehouse staff can include it in the package.
func writePackingSlipPDF(order *models.Order, company companyInfo) ([]byte, error) {
	pdf, tr := newBrandedPDF()

	y := drawDocumentHeader(pdf, tr, company, "PACKING SLIP", order.OrderNumber, order.CreatedAt, "")

	y = drawAddressBlock(pdf, tr, pageMarginMM, y, 100, "Ship To", order.ShippingInfo, false)
	y += 6

	setColor3(pdf, colorBorder, "draw")
	pdf.Line(pageMarginMM, y, pageMarginMM+contentWidthA, y)
	y += 6

	colItem, colSize, colQty := 110.0, 35.0, 35.0
	pdf.SetXY(pageMarginMM, y)
	setColor3(pdf, colorTableBg, "fill")
	pdf.SetFont("Helvetica", "B", 9)
	setColor3(pdf, colorInk, "text")
	pdf.CellFormat(colItem, 8, tr("Item"), "", 0, "L", true, 0, "")
	pdf.CellFormat(colSize, 8, tr("Size"), "", 0, "C", true, 0, "")
	pdf.CellFormat(colQty, 8, tr("Qty"), "", 2, "C", true, 0, "")
	y = pdf.GetY()

	pdf.SetFont("Helvetica", "", 9.5)
	for _, item := range order.Items {
		rowH := 8.0
		if item.IsGift && item.GiftMessage != "" {
			rowH = 12.0
		}
		pdf.SetXY(pageMarginMM, y)
		setColor3(pdf, colorInk, "text")
		pdf.CellFormat(colItem, 8, fitText(pdf, tr(item.ProductName), colItem-2), "B", 0, "L", false, 0, "")
		pdf.CellFormat(colSize, 8, tr(item.Size), "B", 0, "C", false, 0, "")
		pdf.CellFormat(colQty, 8, strconv.Itoa(item.Quantity), "B", 2, "C", false, 0, "")

		if item.IsGift && item.GiftMessage != "" {
			pdf.SetXY(pageMarginMM, y+8)
			pdf.SetFont("Helvetica", "I", 8)
			setColor3(pdf, colorMuted, "text")
			pdf.CellFormat(colItem+colSize+colQty, 4, fitText(pdf, tr("Gift message: "+item.GiftMessage), colItem+colSize+colQty-2), "B", 2, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 9.5)
		}
		y += rowH
	}
	y += 6

	pdf.SetXY(pageMarginMM, y)
	pdf.SetFont("Helvetica", "B", 9)
	setColor3(pdf, colorMuted, "text")
	totalItems := 0
	for _, item := range order.Items {
		totalItems += item.Quantity
	}
	pdf.CellFormat(contentWidthA, 6, tr(fmt.Sprintf("Total items: %d", totalItems)), "", 2, "L", false, 0, "")

	drawDocumentFooter(pdf, tr, company, "Internal fulfillment document -- not a receipt or proof of value.")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawDocumentFooter pins a short note to the bottom of the current page.
// Positioned well clear of the auto-page-break trigger line (pageH - 20mm by
// default) so writing it never bumps itself onto a fresh page.
func drawDocumentFooter(pdf *fpdf.Fpdf, tr func(string) string, company companyInfo, note string) {
	_, pageH := pdf.GetPageSize()
	y := pageH - 32
	setColor3(pdf, colorBorder, "draw")
	pdf.SetLineWidth(0.3)
	pdf.Line(pageMarginMM, y, pageMarginMM+contentWidthA, y)

	pdf.SetXY(pageMarginMM, y+4)
	pdf.SetFont("Helvetica", "", 8)
	setColor3(pdf, colorMuted, "text")
	pdf.CellFormat(contentWidthA, 4, tr(note), "", 2, "C", false, 0, "")

	contact := company.SupportEmail
	if contact == "" {
		contact = company.Email
	}
	if contact != "" {
		pdf.SetX(pageMarginMM)
		pdf.CellFormat(contentWidthA, 4, tr("Questions about this order? Contact us at "+contact), "", 2, "C", false, 0, "")
	}
}

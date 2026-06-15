package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strings"
	"unicode"

	"rloco-backend/internal/models"
)

type EmailService interface {
	Configured() bool
	SendOrderConfirmation(to string, order *models.Order) error
	SendShippingNotification(to string, order *models.Order) error
	SendOrderStatusUpdate(to string, order *models.Order, status string) error
	SendPasswordReset(to, resetToken string) error
	SendEmailVerification(to, verificationToken string) error
	SendReturnConfirmation(to, returnID string) error
	SendRefundNotification(to, returnID string, amount float64) error
	SendPaymentReceived(to string, order *models.Order, amount float64, currency string) error
	SendNewOrderAlert(orderNumber, totalDisplay, customerEmail string) error
	SendVendorPortalCredentials(to, vendorName, loginURL, temporaryPassword string) error
	SendContactInquiry(name, fromEmail, phone, subject, message string) error
	SendVendorApplicationReceived(to, businessName string) error
	SendVendorApplicationApproved(to, businessName, loginURL, tempPassword string) error
	SendVendorApplicationRejected(to, businessName, reason string) error
}

type emailService struct {
	resendAPIKey string
	fromEmail    string
	fromName     string
	baseURL      string
	adminEmail   string
}

func NewEmailService(resendAPIKey, fromEmail, fromName, baseURL, adminEmail string) EmailService {
	return &emailService{
		resendAPIKey: strings.TrimSpace(resendAPIKey),
		fromEmail:    strings.TrimSpace(fromEmail),
		fromName:     strings.TrimSpace(fromName),
		baseURL:      strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		adminEmail:   strings.TrimSpace(adminEmail),
	}
}

func (s *emailService) Configured() bool {
	return s.resendAPIKey != "" && s.fromEmail != ""
}

type resendSendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Html    string   `json:"html"`
}

func (s *emailService) sendEmail(to, subject, body string) error {
	if !s.Configured() {
		log.Printf("[Email] Not configured (need RESEND_API_KEY + SMTP_FROM): would send to %s: %s", to, subject)
		return nil
	}
	from := s.fromEmail
	if s.fromName != "" {
		from = fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	}
	payload, err := json.Marshal(resendSendRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		Html:    body,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.resendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[Email] Resend request failed to %s: %v", to, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("resend API %d: %s", resp.StatusCode, string(b))
		log.Printf("[Email] Send failed to %s: %v", to, err)
		return err
	}
	log.Printf("[Email] Sent to %s: %s", to, subject)
	return nil
}

// --- Shared R-Loko transactional layout (table-based, #B4770E, dark header) ---

func rlokoEmailShell(htmlTitle, heroHeadline, innerRows string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f2ef;font-family:Georgia,'Times New Roman',serif;-webkit-font-smoothing:antialiased;">
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background-color:#f4f2ef;padding:32px 16px;">
  <tr>
    <td align="center">
      <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 8px 32px rgba(0,0,0,0.08);border:1px solid #e8e4df;">
        <tr>
          <td style="background:linear-gradient(135deg,#1a1a1a 0%%,#2d2d2d 100%%);padding:28px 32px;text-align:center;">
            <p style="margin:0;font-size:11px;letter-spacing:0.35em;text-transform:uppercase;color:#c9a227;">R-Loko</p>
            <h1 style="margin:12px 0 0;font-size:22px;font-weight:400;color:#faf8f5;line-height:1.3;">%s</h1>
          </td>
        </tr>
%s
        <tr>
          <td style="padding:20px 32px 28px;background:#f7f5f2;border-top:1px solid #ebe6e0;text-align:center;">
            <p style="margin:0;font-size:12px;color:#9a948c;">© R-Loko · Luxury fashion</p>
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>`, html.EscapeString(htmlTitle), html.EscapeString(heroHeadline), innerRows)
}

func emailTextBlockRow(htmlParagraphs string) string {
	return fmt.Sprintf(`        <tr>
          <td style="padding:36px 32px 8px;">%s
          </td>
        </tr>`, htmlParagraphs)
}

func emailP(text string) string {
	return fmt.Sprintf(`<p style="margin:0 0 16px;font-size:15px;line-height:1.65;color:#5c5650;">%s</p>`, text)
}

func emailGreeting() string {
	return `<p style="margin:0 0 16px;font-size:16px;line-height:1.65;color:#2c2825;">Hello,</p>`
}

func emailButtonRow(primaryURL, buttonLabel string) string {
	u := html.EscapeString(primaryURL)
	l := html.EscapeString(buttonLabel)
	return fmt.Sprintf(`        <tr>
          <td style="padding:0 32px 28px;text-align:center;">
            <a href="%s" style="display:inline-block;padding:14px 36px;background-color:#B4770E;color:#ffffff !important;text-decoration:none;border-radius:8px;font-size:15px;font-weight:600;letter-spacing:0.02em;">%s</a>
            <p style="margin:20px 0 0;font-size:12px;line-height:1.5;color:#8a847c;word-break:break-all;">If the button does not work, copy this link:<br><a href="%s" style="color:#B4770E;">%s</a></p>
          </td>
        </tr>`, u, l, u, u)
}

func emailHighlightBox(label, valueHTML string) string {
	return fmt.Sprintf(`        <tr>
          <td style="padding:0 32px 32px;">
            <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#faf8f6;border-radius:10px;border:1px solid #ebe6e0;">
              <tr>
                <td style="padding:20px 22px;">
                  <p style="margin:0 0 8px;font-size:11px;letter-spacing:0.12em;text-transform:uppercase;color:#8a847c;">%s</p>
                  <div style="font-size:18px;font-weight:600;color:#2c2825;line-height:1.4;">%s</div>
                </td>
              </tr>
            </table>
          </td>
        </tr>`, html.EscapeString(label), valueHTML)
}

func emailMutedNote(text string) string {
	return fmt.Sprintf(`        <tr>
          <td style="padding:0 32px 32px;">
            <p style="margin:0;font-size:13px;line-height:1.55;color:#7a736b;">%s</p>
          </td>
        </tr>`, html.EscapeString(text))
}

// --- Order-aware content helpers -------------------------------------------

func orderCurrencySymbol(order *models.Order) string {
	if CountryLooksIndia(order.ShippingInfo.Country) {
		return "₹"
	}
	return "$"
}

func fmtMoney(symbol string, amount float64) string {
	return fmt.Sprintf("%s%.2f", symbol, amount)
}

// formatMoneyCurrency formats by ISO currency code (used where the charged amount
// and currency are known explicitly, e.g. payment received).
func formatMoneyCurrency(amount float64, currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USD":
		return fmt.Sprintf("$%.2f", amount)
	case "INR":
		return fmt.Sprintf("₹%.2f", amount)
	default:
		return fmt.Sprintf("%.2f %s", amount, strings.ToUpper(currency))
	}
}

func (s *emailService) orderDetailURL(order *models.Order) string {
	if s.baseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/orders/%s", s.baseURL, order.ID.Hex())
}

// emailItemsTable renders the order line items (name, size · qty, line total).
func emailItemsTable(order *models.Order) string {
	if len(order.Items) == 0 {
		return ""
	}
	sym := orderCurrencySymbol(order)
	var rows strings.Builder
	for _, it := range order.Items {
		meta := fmt.Sprintf("Qty %d", it.Quantity)
		if it.Size != "" {
			meta = "Size " + html.EscapeString(it.Size) + " · " + meta
		}
		gift := ""
		if it.IsGift {
			gift = `<div style="margin-top:4px;font-size:11px;color:#B4770E;">Gift wrapped</div>`
		}
		rows.WriteString(fmt.Sprintf(`
              <tr>
                <td style="padding:12px 0;border-bottom:1px solid #ebe6e0;vertical-align:top;">
                  <div style="font-size:14px;color:#2c2825;font-weight:600;">%s</div>
                  <div style="font-size:12px;color:#8a847c;margin-top:2px;">%s</div>%s
                </td>
                <td style="padding:12px 0;border-bottom:1px solid #ebe6e0;text-align:right;vertical-align:top;font-size:14px;color:#2c2825;white-space:nowrap;">%s</td>
              </tr>`,
			html.EscapeString(it.ProductName), meta, gift,
			html.EscapeString(fmtMoney(sym, it.Price*float64(it.Quantity)))))
	}
	return fmt.Sprintf(`        <tr>
          <td style="padding:8px 32px 0;">
            <table role="presentation" width="100%%" cellspacing="0" cellpadding="0">%s
            </table>
          </td>
        </tr>`, rows.String())
}

// emailTotalsTable renders the price breakdown.
func emailTotalsTable(order *models.Order) string {
	sym := orderCurrencySymbol(order)
	line := func(label, val string, strong bool) string {
		color, weight, size := "#5c5650", "400", "13px"
		if strong {
			color, weight, size = "#2c2825", "700", "16px"
		}
		return fmt.Sprintf(`<tr><td style="padding:4px 0;font-size:%s;color:%s;font-weight:%s;">%s</td><td style="padding:4px 0;text-align:right;font-size:%s;color:%s;font-weight:%s;white-space:nowrap;">%s</td></tr>`,
			size, color, weight, html.EscapeString(label), size, color, weight, html.EscapeString(val))
	}
	var b strings.Builder
	b.WriteString(line("Subtotal", fmtMoney(sym, order.Subtotal), false))
	if order.Discount > 0 {
		b.WriteString(line("Discount", "-"+fmtMoney(sym, order.Discount), false))
	}
	shipping := "Free"
	if order.ShippingCost > 0 {
		shipping = fmtMoney(sym, order.ShippingCost)
	}
	b.WriteString(line("Shipping", shipping, false))
	if order.GiftPackingCharge > 0 {
		b.WriteString(line("Gift packing", fmtMoney(sym, order.GiftPackingCharge), false))
	}
	if order.Tax > 0 {
		b.WriteString(line("Tax", fmtMoney(sym, order.Tax), false))
	}
	b.WriteString(`<tr><td colspan="2" style="padding:6px 0 0;border-top:1px solid #ebe6e0;"></td></tr>`)
	b.WriteString(line("Total", fmtMoney(sym, order.Total), true))
	return fmt.Sprintf(`        <tr>
          <td style="padding:10px 32px 24px;">
            <table role="presentation" width="100%%" cellspacing="0" cellpadding="0">%s
            </table>
          </td>
        </tr>`, b.String())
}

// emailInfoBox is a lighter highlight box for multi-line info (e.g. an address).
func emailInfoBox(label, valueHTML string) string {
	return fmt.Sprintf(`        <tr>
          <td style="padding:0 32px 24px;">
            <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#faf8f6;border-radius:10px;border:1px solid #ebe6e0;">
              <tr><td style="padding:16px 20px;">
                <p style="margin:0 0 6px;font-size:11px;letter-spacing:0.12em;text-transform:uppercase;color:#8a847c;">%s</p>
                <div style="font-size:14px;color:#2c2825;line-height:1.55;">%s</div>
              </td></tr>
            </table>
          </td>
        </tr>`, html.EscapeString(label), valueHTML)
}

func emailAddressBlock(si models.ShippingInfo) string {
	parts := []string{}
	if n := strings.TrimSpace(si.FirstName + " " + si.LastName); n != "" {
		parts = append(parts, html.EscapeString(n))
	}
	if si.Address != "" {
		parts = append(parts, html.EscapeString(si.Address))
	}
	if cs := strings.Trim(strings.TrimSpace(si.City+", "+si.State+" "+si.ZipCode), ", "); cs != "" {
		parts = append(parts, html.EscapeString(cs))
	}
	if si.Country != "" {
		parts = append(parts, html.EscapeString(si.Country))
	}
	if len(parts) == 0 {
		return ""
	}
	return emailInfoBox("Shipping to", strings.Join(parts, "<br>"))
}

// statusEmailCopy is the per-status content used by SendOrderStatusUpdate.
type statusEmailCopy struct {
	title        string // hero headline + subject stem
	intro        string // plain text (escaped at render)
	showTracking bool
	showReview   bool
	note         string // optional muted footnote (plain text)
}

func statusEmailCopyFor(status string) statusEmailCopy {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "DELIVERED":
		return statusEmailCopy{title: "Delivered", intro: "Your order has been delivered — we hope you love it.", showReview: true, note: "Not quite right? You can start a return from your orders page within the return window."}
	case "OUT FOR DELIVERY", "OUT_FOR_DELIVERY":
		return statusEmailCopy{title: "Out for delivery", intro: "Good news — your order is out for delivery and should arrive today.", showTracking: true}
	case "TRANSIT", "IN TRANSIT", "IN_TRANSIT", "SHIPPED":
		return statusEmailCopy{title: "On its way", intro: "Your order is in transit. Follow its journey with the tracking details below.", showTracking: true}
	case "DELIVERY EXCEPTION", "EXCEPTION", "FAILED", "FAILURE":
		return statusEmailCopy{title: "Delivery update", intro: "There's a temporary issue delivering your order. The carrier is working on it and we're keeping an eye on it too.", showTracking: true, note: "If it isn't resolved within a couple of days, just reply to this email and our team will help."}
	case "RETURNED", "RETURN":
		return statusEmailCopy{title: "Order returned", intro: "Your order has been marked as returned. If a refund is due, it will follow once the return is processed."}
	case "CANCELLED", "CANCELED":
		return statusEmailCopy{title: "Order cancelled", intro: "Your order has been cancelled. If you were charged, a refund has been issued and usually appears within 5–10 business days."}
	case "PROCESSING":
		return statusEmailCopy{title: "Order confirmed", intro: "Your payment is confirmed and your order is being prepared. We'll email you again as soon as it ships."}
	default:
		return statusEmailCopy{title: "Order update", intro: "Your order status is now " + capitalizeStatus(status) + "."}
	}
}

// --- Order lifecycle emails -------------------------------------------------

func (s *emailService) SendOrderConfirmation(to string, order *models.Order) error {
	on := html.EscapeString(order.OrderNumber)
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`Thanks for your order — <strong style="color:#2c2825;">%s</strong> is confirmed. Here's a summary:`, on))
	body := emailTextBlockRow(inner) +
		emailItemsTable(order) +
		emailTotalsTable(order) +
		emailAddressBlock(order.ShippingInfo)
	if url := s.orderDetailURL(order); url != "" {
		body += emailButtonRow(url, "View your order")
	}
	body += emailMutedNote("We'll email you again as soon as your order ships.")
	return s.sendEmail(to, fmt.Sprintf("Order confirmed · %s · R-Loko", order.OrderNumber),
		rlokoEmailShell("Order confirmed — R-Loko", "Thank you for your order", body))
}

func (s *emailService) SendShippingNotification(to string, order *models.Order) error {
	on := html.EscapeString(order.OrderNumber)
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`Your order <strong style="color:#2c2825;">%s</strong> is on its way.`, on))
	body := emailTextBlockRow(inner)
	if order.TrackingNumber != nil && strings.TrimSpace(*order.TrackingNumber) != "" {
		body += emailHighlightBox("Tracking number", fmt.Sprintf(`<span style="font-family:ui-monospace,Menlo,Monaco,Consolas,monospace;font-size:16px;">%s</span>`, html.EscapeString(*order.TrackingNumber)))
	}
	body += emailItemsTable(order)
	if url := s.orderDetailURL(order); url != "" {
		body += emailButtonRow(url, "Track your order")
	}
	body += emailMutedNote("Tracking updates may take a few hours to appear with the carrier.")
	return s.sendEmail(to, fmt.Sprintf("Your order %s has shipped · R-Loko", order.OrderNumber),
		rlokoEmailShell("Your order has shipped — R-Loko", "It's on the way", body))
}

func (s *emailService) SendOrderStatusUpdate(to string, order *models.Order, status string) error {
	c := statusEmailCopyFor(status)
	on := html.EscapeString(order.OrderNumber)
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`An update on your order <strong style="color:#2c2825;">%s</strong>:`, on)) +
		emailP(html.EscapeString(c.intro))
	body := emailTextBlockRow(inner)
	if c.showTracking && order.TrackingNumber != nil && strings.TrimSpace(*order.TrackingNumber) != "" {
		body += emailHighlightBox("Tracking number", fmt.Sprintf(`<span style="font-family:ui-monospace,Menlo,Monaco,Consolas,monospace;font-size:16px;">%s</span>`, html.EscapeString(*order.TrackingNumber)))
	}
	if url := s.orderDetailURL(order); url != "" {
		label := "View your order"
		switch {
		case c.showReview:
			label = "View order & leave a review"
		case c.showTracking:
			label = "Track your order"
		}
		body += emailButtonRow(url, label)
	}
	if c.note != "" {
		body += emailMutedNote(c.note)
	}
	return s.sendEmail(to, fmt.Sprintf("%s · order %s · R-Loko", c.title, order.OrderNumber),
		rlokoEmailShell(c.title+" — R-Loko", c.title, body))
}

func capitalizeStatus(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func (s *emailService) SendPasswordReset(to, resetToken string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.baseURL, resetToken)
	inner := emailGreeting() +
		emailP("We received a request to reset your password. Click the button below to choose a new one.") +
		emailP("If you didn't ask for this, you can ignore this email — your password will stay the same.")
	body := emailTextBlockRow(inner) + emailButtonRow(resetURL, "Reset password")
	return s.sendEmail(to, "Reset your R-Loko password",
		rlokoEmailShell("Password reset — R-Loko", "Reset your password", body))
}

func (s *emailService) SendEmailVerification(to, verificationToken string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.baseURL, verificationToken)
	inner := emailGreeting() +
		emailP("Thanks for joining R-Loko. Confirm your email so we can keep your account secure and send order updates.") +
		emailP("If you didn't create an account, you can ignore this message.")
	body := emailTextBlockRow(inner) + emailButtonRow(verifyURL, "Verify email")
	return s.sendEmail(to, "Verify your email · R-Loko",
		rlokoEmailShell("Verify your email — R-Loko", "One quick step", body))
}

func (s *emailService) SendReturnConfirmation(to, returnID string) error {
	rid := html.EscapeString(returnID)
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`We've received your return request <strong style="color:#2c2825;">%s</strong>.`, rid)) +
		emailP("Our team will review it and email you when it's approved or if we need more information.")
	body := emailTextBlockRow(inner) +
		emailMutedNote("Please keep your items packed as instructed until you receive return shipping instructions, if applicable.")
	return s.sendEmail(to, "Return request received · R-Loko",
		rlokoEmailShell("Return request — R-Loko", "We've got your return", body))
}

func (s *emailService) SendRefundNotification(to, returnID string, amount float64) error {
	rid := html.EscapeString(returnID)
	amt := html.EscapeString(fmt.Sprintf("$%.2f", amount))
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`Your refund for return <strong style="color:#2c2825;">%s</strong> has been issued.`, rid)) +
		emailP("Depending on your bank or card issuer, funds usually appear within 5–10 business days.")
	body := emailTextBlockRow(inner) +
		emailHighlightBox("Refund amount", fmt.Sprintf(`<span style="color:#B4770E;">%s</span>`, amt))
	return s.sendEmail(to, "Refund processed · R-Loko",
		rlokoEmailShell("Refund processed — R-Loko", "Your refund is on the way", body))
}

func (s *emailService) SendPaymentReceived(to string, order *models.Order, amount float64, currency string) error {
	on := html.EscapeString(order.OrderNumber)
	amtEsc := html.EscapeString(formatMoneyCurrency(amount, currency))
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`We've received your payment for order <strong style="color:#2c2825;">%s</strong>. Your order is being prepared, and we'll email you again when it ships.`, on))
	body := emailTextBlockRow(inner) +
		emailHighlightBox("Amount received", fmt.Sprintf(`<span style="color:#B4770E;">%s</span>`, amtEsc)) +
		emailItemsTable(order)
	if url := s.orderDetailURL(order); url != "" {
		body += emailButtonRow(url, "View your order")
	}
	return s.sendEmail(to, fmt.Sprintf("Payment received · order %s · R-Loko", order.OrderNumber),
		rlokoEmailShell("Payment received — R-Loko", "Thank you", body))
}

func (s *emailService) SendNewOrderAlert(orderNumber, totalDisplay, customerEmail string) error {
	if s.adminEmail == "" {
		return nil
	}
	on := html.EscapeString(orderNumber)
	td := html.EscapeString(totalDisplay)
	ce := html.EscapeString(customerEmail)
	dashURL := s.baseURL + "/admin/orders"
	inner := emailP(fmt.Sprintf(`<strong style="color:#2c2825;">New order</strong> — <span style="font-family:ui-monospace,monospace;">%s</span>`, on)) +
		emailP(fmt.Sprintf(`Total: <strong style="color:#B4770E;">%s</strong>`, td)) +
		emailP(fmt.Sprintf(`Customer: %s`, ce)) +
		emailP("Open the admin dashboard to review and fulfill this order.")
	body := emailTextBlockRow(emailGreeting()+inner) + emailButtonRow(dashURL, "View orders in admin")
	return s.sendEmail(s.adminEmail, fmt.Sprintf("[R-Loko] New order %s", orderNumber),
		rlokoEmailShell("New order — admin · R-Loko", "New order to fulfill", body))
}

func (s *emailService) SendVendorPortalCredentials(to, vendorName, loginURL, temporaryPassword string) error {
	if !s.Configured() {
		return errors.New("smtp not configured")
	}
	name := html.EscapeString(vendorName)
	pw := html.EscapeString(temporaryPassword)
	inner := fmt.Sprintf(`<p style="margin:0 0 16px;font-size:16px;line-height:1.65;color:#2c2825;">Hello %s,</p>
            <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:#5c5650;">Your vendor account is ready. Sign in with your email and the temporary password below. For security, change your password after your first sign-in under <strong style="color:#2c2825;">Vendor settings</strong>.</p>`, name)
	pwRow := fmt.Sprintf(`        <tr>
          <td style="padding:0 32px 32px;">
            <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#faf8f6;border-radius:10px;border:1px solid #ebe6e0;">
              <tr>
                <td style="padding:20px 22px;">
                  <p style="margin:0 0 8px;font-size:11px;letter-spacing:0.12em;text-transform:uppercase;color:#8a847c;">Temporary password</p>
                  <p style="margin:0;font-family:ui-monospace,Menlo,Monaco,Consolas,monospace;font-size:15px;color:#1a1816;letter-spacing:0.04em;word-break:break-all;">%s</p>
                </td>
              </tr>
            </table>
          </td>
        </tr>`, pw)
	body := emailTextBlockRow(inner) +
		emailButtonRow(loginURL, "Open vendor portal") +
		pwRow +
		emailMutedNote("Never share this email. R-Loko staff will never ask for your password by phone or message.")
	return s.sendEmail(to, "Your R-Loko vendor portal is ready",
		rlokoEmailShell("Vendor portal — R-Loko", "Your vendor portal is ready", body))
}

func (s *emailService) SendVendorApplicationReceived(to, businessName string) error {
	if !s.Configured() {
		return nil
	}
	bn := html.EscapeString(businessName)
	body := fmt.Sprintf(`<p style="margin:0 0 16px;font-size:15px;line-height:1.65;color:#5c5650;">Hi %s,</p>
<p style="margin:0 0 16px;font-size:15px;line-height:1.65;color:#5c5650;">Thank you for applying to sell on R-Loko. We've received your application and our team will review it shortly.</p>
<p style="margin:0;font-size:15px;line-height:1.65;color:#5c5650;">We'll send you another email once a decision has been made — usually within 2–3 business days.</p>`, bn)
	return s.sendEmail(to, "We received your vendor application — R-Loko",
		rlokoEmailShell("Vendor Application", "Application received", emailTextBlockRow(body)))
}

func (s *emailService) SendVendorApplicationApproved(to, businessName, loginURL, tempPassword string) error {
	if !s.Configured() {
		return nil
	}
	bn := html.EscapeString(businessName)
	var credBlock string
	if tempPassword != "" {
		pw := html.EscapeString(tempPassword)
		credBlock = fmt.Sprintf(`<p style="margin:0 0 8px;font-size:15px;line-height:1.65;color:#5c5650;">Sign in with your email and the temporary password below, then change it from your vendor settings.</p>
<p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#5c5650;"><strong>Temporary password:</strong> <code style="background:#f5f3f0;padding:2px 6px;border-radius:3px;">%s</code></p>`, pw)
	} else {
		credBlock = `<p style="margin:0 0 24px;font-size:15px;line-height:1.65;color:#5c5650;">Sign in using your existing account credentials.</p>`
	}
	body := fmt.Sprintf(`<p style="margin:0 0 16px;font-size:15px;line-height:1.65;color:#5c5650;">Congratulations! Your application for <strong>%s</strong> has been <strong style="color:#2c7a4b;">approved</strong>.</p>%s`, bn, credBlock)
	return s.sendEmail(to, "Your R-Loko vendor application was approved 🎉",
		rlokoEmailShell("Vendor Approved", "Welcome to R-Loko",
			emailTextBlockRow(body)+emailButtonRow(loginURL, "Open vendor portal")))
}

func (s *emailService) SendVendorApplicationRejected(to, businessName, reason string) error {
	if !s.Configured() {
		return nil
	}
	bn := html.EscapeString(businessName)
	reasonRow := ""
	if reason != "" {
		reasonRow = fmt.Sprintf(`<p style="margin:16px 0 0;font-size:15px;line-height:1.65;color:#5c5650;"><strong>Reason:</strong> %s</p>`, html.EscapeString(reason))
	}
	body := fmt.Sprintf(`<p style="margin:0 0 16px;font-size:15px;line-height:1.65;color:#5c5650;">Thank you for your interest in selling on R-Loko.</p>
<p style="margin:0;font-size:15px;line-height:1.65;color:#5c5650;">After reviewing your application for <strong>%s</strong>, we're unable to approve it at this time.%s</p>
<p style="margin:16px 0 0;font-size:15px;line-height:1.65;color:#5c5650;">You're welcome to apply again in the future if your circumstances change.</p>`, bn, reasonRow)
	return s.sendEmail(to, "Update on your R-Loko vendor application",
		rlokoEmailShell("Vendor Application", "Application update", emailTextBlockRow(body)))
}

func (s *emailService) SendContactInquiry(name, fromEmail, phone, subject, message string) error {
	if !s.Configured() {
		return errors.New("smtp not configured")
	}
	to := strings.TrimSpace(s.adminEmail)
	if to == "" {
		to = strings.TrimSpace(s.fromEmail)
	}
	if to == "" {
		return errors.New("no recipient for contact inquiries (set ADMIN_EMAIL or SMTP_FROM)")
	}
	n := html.EscapeString(name)
	fe := html.EscapeString(fromEmail)
	ph := html.EscapeString(phone)
	sub := html.EscapeString(subject)
	msg := html.EscapeString(message)
	body := fmt.Sprintf(`<p style="margin:0 0 12px;font-size:15px;line-height:1.65;color:#2c2825;"><strong>From:</strong> %s &lt;%s&gt;</p>
<p style="margin:0 0 12px;font-size:15px;line-height:1.65;color:#2c2825;"><strong>Phone:</strong> %s</p>
<p style="margin:0 0 12px;font-size:15px;line-height:1.65;color:#2c2825;"><strong>Subject:</strong> %s</p>
<p style="margin:0;font-size:15px;line-height:1.65;color:#2c2825;"><strong>Message:</strong></p>
<p style="margin:12px 0 0;font-size:15px;line-height:1.65;color:#2c2825;white-space:pre-wrap;">%s</p>`, n, fe, ph, sub, msg)
	return s.sendEmail(to, fmt.Sprintf("[R-Loko Contact] %s", subject),
		rlokoEmailShell("Website contact form", "New inquiry", emailTextBlockRow(body)))
}

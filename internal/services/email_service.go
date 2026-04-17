package services

import (
	"errors"
	"fmt"
	"html"
	"log"
	"net/smtp"
	"strings"
	"unicode"
)

type EmailService interface {
	Configured() bool
	SendOrderConfirmation(to, orderNumber string, orderData map[string]interface{}) error
	SendShippingNotification(to, orderNumber, trackingNumber string) error
	SendOrderStatusUpdate(to, orderNumber, status string) error
	SendPasswordReset(to, resetToken string) error
	SendEmailVerification(to, verificationToken string) error
	SendReturnConfirmation(to, returnID string) error
	SendRefundNotification(to, returnID string, amount float64) error
	SendPaymentReceived(to, orderNumber string, amount float64, currency string) error
	SendNewOrderAlert(orderNumber, totalDisplay, customerEmail string) error
	SendVendorPortalCredentials(to, vendorName, loginURL, temporaryPassword string) error
	SendContactInquiry(name, fromEmail, phone, subject, message string) error
}

type emailService struct {
	smtpHost     string
	smtpPort     string
	smtpUser     string
	smtpPassword string
	fromEmail    string
	fromName     string
	baseURL      string
	adminEmail   string
}

func NewEmailService(smtpHost, smtpPort, smtpUser, smtpPassword, fromEmail, fromName, baseURL, adminEmail string) EmailService {
	return &emailService{
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUser:     smtpUser,
		smtpPassword: smtpPassword,
		fromEmail:    fromEmail,
		fromName:     fromName,
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		adminEmail:   strings.TrimSpace(adminEmail),
	}
}

func (s *emailService) Configured() bool {
	return strings.TrimSpace(s.smtpHost) != "" &&
		strings.TrimSpace(s.smtpPort) != "" &&
		strings.TrimSpace(s.smtpUser) != "" &&
		strings.TrimSpace(s.smtpPassword) != "" &&
		strings.TrimSpace(s.fromEmail) != ""
}

func (s *emailService) sendEmail(to, subject, body string) error {
	if strings.TrimSpace(s.smtpHost) == "" ||
		strings.TrimSpace(s.smtpPort) == "" ||
		strings.TrimSpace(s.smtpUser) == "" ||
		strings.TrimSpace(s.smtpPassword) == "" {
		log.Printf("[Email] Not configured (need SMTP_HOST/SMTP_PORT/SMTP_USER/SMTP_PASSWORD): would send to %s: %s", to, subject)
		return nil
	}

	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPassword, s.smtpHost)

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", to, s.fromName, s.fromEmail, subject, body))

	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
	err := smtp.SendMail(addr, auth, s.fromEmail, []string{to}, msg)
	if err != nil {
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

func (s *emailService) SendOrderConfirmation(to, orderNumber string, orderData map[string]interface{}) error {
	total := orderData["total"]
	totalStr := ""
	switch v := total.(type) {
	case float64:
		totalStr = fmt.Sprintf("$%.2f", v)
	case float32:
		totalStr = fmt.Sprintf("$%.2f", v)
	default:
		totalStr = fmt.Sprintf("%v", total)
	}
	on := html.EscapeString(orderNumber)
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`Your order <strong style="color:#2c2825;">%s</strong> has been confirmed.`, on)) +
		emailP("We'll send you another email when your order ships.")
	body := emailTextBlockRow(inner) +
		emailHighlightBox("Order total", fmt.Sprintf(`<span style="color:#B4770E;">%s</span>`, html.EscapeString(totalStr)))

	htmlDoc := rlokoEmailShell("Order confirmed — R-Loko", "Thank you for your order", body)
	return s.sendEmail(to, fmt.Sprintf("Order confirmed · %s · R-Loko", orderNumber), htmlDoc)
}

func (s *emailService) SendShippingNotification(to, orderNumber, trackingNumber string) error {
	on := html.EscapeString(orderNumber)
	tn := html.EscapeString(trackingNumber)
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`Your order <strong style="color:#2c2825;">%s</strong> is on its way.`, on)) +
		emailP("Use your tracking number below to follow the shipment.")
	body := emailTextBlockRow(inner) +
		emailHighlightBox("Tracking number", fmt.Sprintf(`<span style="font-family:ui-monospace,Menlo,Monaco,Consolas,monospace;font-size:16px;">%s</span>`, tn)) +
		emailMutedNote("Tracking updates may take a few hours to appear with the carrier.")

	return s.sendEmail(to, fmt.Sprintf("Your order %s has shipped · R-Loko", orderNumber),
		rlokoEmailShell("Your order has shipped — R-Loko", "It's on the way", body))
}

func (s *emailService) SendOrderStatusUpdate(to, orderNumber, status string) error {
	st := capitalizeStatus(status)
	on := html.EscapeString(orderNumber)
	stEsc := html.EscapeString(st)
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`Your order <strong style="color:#2c2825;">%s</strong> has been updated.`, on)) +
		emailP(fmt.Sprintf(`Current status: <strong style="color:#B4770E;">%s</strong>`, stEsc))
	body := emailTextBlockRow(inner)
	return s.sendEmail(to, fmt.Sprintf("Order %s update · R-Loko", orderNumber),
		rlokoEmailShell("Order status update — R-Loko", "Your order was updated", body))
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

func (s *emailService) SendPaymentReceived(to, orderNumber string, amount float64, currency string) error {
	currency = strings.ToUpper(currency)
	amountStr := fmt.Sprintf("%.2f", amount)
	if currency == "USD" || currency == "INR" {
		if currency == "INR" {
			amountStr = "₹" + amountStr
		} else {
			amountStr = "$" + amountStr
		}
	} else {
		amountStr = amountStr + " " + currency
	}
	on := html.EscapeString(orderNumber)
	amtEsc := html.EscapeString(amountStr)
	inner := emailGreeting() +
		emailP(fmt.Sprintf(`We've received your payment for order <strong style="color:#2c2825;">%s</strong>.`, on)) +
		emailP("Your order is being prepared. You'll get another email when it ships.")
	body := emailTextBlockRow(inner) +
		emailHighlightBox("Amount received", fmt.Sprintf(`<span style="color:#B4770E;">%s</span>`, amtEsc))
	return s.sendEmail(to, fmt.Sprintf("Payment received · order %s · R-Loko", orderNumber),
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

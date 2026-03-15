package services

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"strings"
	"unicode"
)

type EmailService interface {
	SendOrderConfirmation(to, orderNumber string, orderData map[string]interface{}) error
	SendShippingNotification(to, orderNumber, trackingNumber string) error
	SendOrderStatusUpdate(to, orderNumber, status string) error
	SendPasswordReset(to, resetToken string) error
	SendEmailVerification(to, verificationToken string) error
	SendReturnConfirmation(to, returnID string) error
	SendRefundNotification(to, returnID string, amount float64) error
	SendPaymentReceived(to, orderNumber string, amount float64, currency string) error
	SendNewOrderAlert(orderNumber, totalDisplay, customerEmail string) error
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

func (s *emailService) sendEmail(to, subject, body string) error {
	if s.smtpHost == "" || s.smtpPort == "" {
		log.Printf("[Email] Not configured: would send to %s: %s", to, subject)
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

func (s *emailService) SendOrderConfirmation(to, orderNumber string, orderData map[string]interface{}) error {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Order Confirmation - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">Thank you for your order!</h1>
		<p>Dear Customer,</p>
		<p>Your order <strong>{{.OrderNumber}}</strong> has been confirmed.</p>
		<p>We'll send you another email when your order ships.</p>
		<p>Order Total: ${{.Total}}</p>
		<p>Best regards,<br>R-Loko Team</p>
	</div>
</body>
</html>
`

	t, err := template.New("order_confirmation").Parse(tmpl)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	data := map[string]interface{}{
		"OrderNumber": orderNumber,
		"Total":       orderData["total"],
	}
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	return s.sendEmail(to, fmt.Sprintf("Order Confirmation - %s", orderNumber), buf.String())
}

func (s *emailService) SendShippingNotification(to, orderNumber, trackingNumber string) error {
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Your Order Has Shipped - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">Your order has shipped!</h1>
		<p>Dear Customer,</p>
		<p>Your order <strong>%s</strong> has been shipped.</p>
		<p>Tracking Number: <strong>%s</strong></p>
		<p>You can track your order using the tracking number above.</p>
		<p>Best regards,<br>R-Loko Team</p>
	</div>
</body>
</html>
`, orderNumber, trackingNumber)

	return s.sendEmail(to, fmt.Sprintf("Your Order %s Has Shipped", orderNumber), body)
}

func (s *emailService) SendOrderStatusUpdate(to, orderNumber, status string) error {
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Order Status Update - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">Order Status Update</h1>
		<p>Dear Customer,</p>
		<p>Your order <strong>%s</strong> status has been updated to: <strong>%s</strong></p>
		<p>Best regards,<br>R-Loko Team</p>
	</div>
</body>
</html>
`, orderNumber, capitalizeStatus(status))

	return s.sendEmail(to, fmt.Sprintf("Order %s Status Update", orderNumber), body)
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
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Password Reset - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">Password Reset Request</h1>
		<p>Dear Customer,</p>
		<p>You requested to reset your password. Click the link below to reset it:</p>
		<p><a href="%s" style="background-color: #2c3e50; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">Reset Password</a></p>
		<p>If you didn't request this, please ignore this email.</p>
		<p>Best regards,<br>R-Loko Team</p>
	</div>
</body>
</html>
`, resetURL)

	return s.sendEmail(to, "Password Reset - R-Loko", body)
}

func (s *emailService) SendEmailVerification(to, verificationToken string) error {
	verificationURL := fmt.Sprintf("%s/verify-email?token=%s", s.baseURL, verificationToken)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Verify Your Email - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">Verify Your Email</h1>
		<p>Dear Customer,</p>
		<p>Thank you for signing up! Please verify your email address by clicking the link below:</p>
		<p><a href="%s" style="background-color: #2c3e50; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">Verify Email</a></p>
		<p>If you didn't create an account, please ignore this email.</p>
		<p>Best regards,<br>R-Loko Team</p>
	</div>
</body>
</html>
`, verificationURL)

	return s.sendEmail(to, "Verify Your Email - R-Loko", body)
}

func (s *emailService) SendReturnConfirmation(to, returnID string) error {
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Return Request Confirmed - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">Return Request Confirmed</h1>
		<p>Dear Customer,</p>
		<p>Your return request <strong>%s</strong> has been received and is being processed.</p>
		<p>We'll notify you once your return is approved.</p>
		<p>Best regards,<br>R-Loko Team</p>
	</div>
</body>
</html>
`, returnID)

	return s.sendEmail(to, "Return Request Confirmed - R-Loko", body)
}

func (s *emailService) SendRefundNotification(to, returnID string, amount float64) error {
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Refund Processed - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">Refund Processed</h1>
		<p>Dear Customer,</p>
		<p>Your refund for return <strong>%s</strong> has been processed.</p>
		<p>Refund Amount: <strong>$%.2f</strong></p>
		<p>The refund will appear in your account within 5-7 business days.</p>
		<p>Best regards,<br>R-Loko Team</p>
	</div>
</body>
</html>
`, returnID, amount)

	return s.sendEmail(to, "Refund Processed - R-Loko", body)
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
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Payment Received - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">Payment Received</h1>
		<p>Dear Customer,</p>
		<p>We have received your payment for order <strong>%s</strong>.</p>
		<p>Amount: <strong>%s</strong></p>
		<p>Your order is now being processed. You will receive a shipping notification when it is dispatched.</p>
		<p>Best regards,<br>R-Loko Team</p>
	</div>
</body>
</html>
`, orderNumber, amountStr)

	return s.sendEmail(to, fmt.Sprintf("Payment Received - Order %s", orderNumber), body)
}

func (s *emailService) SendNewOrderAlert(orderNumber, totalDisplay, customerEmail string) error {
	if s.adminEmail == "" {
		return nil
	}
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>New Order Alert - R-Loko</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2c3e50;">New Order</h1>
		<p>Order number: <strong>%s</strong></p>
		<p>Total: <strong>%s</strong></p>
		<p>Customer email: %s</p>
		<p>Please process this order in the admin dashboard.</p>
	</div>
</body>
</html>
`, orderNumber, totalDisplay, customerEmail)

	return s.sendEmail(s.adminEmail, fmt.Sprintf("[R-Loko] New Order %s", orderNumber), body)
}

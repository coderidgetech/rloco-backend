package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"rloco-backend/internal/config"
	"rloco-backend/internal/handlers"
	"rloco-backend/internal/middleware"
	"rloco-backend/internal/repositories"
	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}
	// Never exit on missing Twilio — NewTwilioVerifyClient already handles empty creds; OTP routes use Enabled().
	// A missing or partial Twilio config would otherwise keep the whole API down (Caddy 502) on misconfigured droplets.
	if err := cfg.ValidateTwilioVerify(); err != nil {
		log.Println("WARNING: Twilio Verify not configured; phone registration OTP disabled:", err)
	}
	if !cfg.EmailConfigReady() {
		log.Println("WARNING: Email notifications are disabled. Set RESEND_API_KEY and SMTP_FROM to enable delivery.")
	}
	if cfg.Env == "production" && strings.TrimSpace(cfg.CORSAllowedOrigins) == "" {
		log.Fatal("CORS_ALLOWED_ORIGINS must be set in production (comma-separated allowed web origins)")
	}

	// Initialize database
	db, err := repositories.NewMongoDB(cfg.MongoDBURI)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	productRepo := repositories.NewProductRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)
	orderRepo := repositories.NewOrderRepository(db)
	cartRepo := repositories.NewCartRepository(db)
	wishlistRepo := repositories.NewWishlistRepository(db)
	promotionRepo := repositories.NewPromotionRepository(db)
	vendorRepo := repositories.NewVendorRepository(db)
	analyticsRepo := repositories.NewAnalyticsRepository(db)
	configRepo := repositories.NewConfigRepository(db)
	reviewRepo := repositories.NewReviewRepository(db)
	returnRepo := repositories.NewReturnRepository(db)
	shippingRepo := repositories.NewShippingRepository(db)
	taxRepo := repositories.NewTaxRepository(db, cfg.OrderIndiaDefaultGSTPercent)
	supportRepo := repositories.NewSupportRepository(db)
	paymentRepo := repositories.NewPaymentRepository(db)
	stripeWebhookEventRepo := repositories.NewStripeWebhookEventRepository(db)
	orderIdempotencyRepo := repositories.NewOrderIdempotencyRepository(db)
	videoRepo := repositories.NewVideoRepository(db)
	addressRepo := repositories.NewAddressRepository(db)
	passwordResetRepo := repositories.NewPasswordResetRepository(db)
	emailVerificationRepo := repositories.NewEmailVerificationRepository(db)
	phoneOTPRepo := repositories.NewPhoneOTPRepository(db)
	newsletterRepo := repositories.NewNewsletterRepository(db)
	trackingRepo := repositories.NewOrderTrackingRepository(db)
	rewardsRepo := repositories.NewRewardsRepository(db)
	vendorAnalyticsRepo := repositories.NewVendorAnalyticsRepository(db)
	advancedAnalyticsRepo := repositories.NewAdvancedAnalyticsRepository(db)
	vendorApplicationRepo := repositories.NewVendorApplicationRepository(db)

	// Update middleware to use config
	middleware.SetJWTSecret(cfg.JWTSecret)
	middleware.ConfigureRateLimit(cfg.APIRateLimitRPM)
	middleware.ConfigureErrorResponses(cfg.Env == "production")

	// Initialize services
	emailService := services.NewEmailService(cfg.ResendAPIKey, cfg.SMTPFrom, cfg.SMTPFromName, cfg.AppBaseURL, cfg.AdminEmail)
	twilioVerify := services.NewTwilioVerifyClient(cfg.TwilioAccountSid, cfg.TwilioAuthToken, cfg.TwilioVerifyServiceSid)
	twilioSMSService := services.NewTwilioSMSService(cfg.TwilioAccountSid, cfg.TwilioAuthToken, cfg.TwilioPhoneNumber)
	fcmService, err := services.NewFCMService(cfg.FCMServiceAccountJSON)
	if err != nil {
		log.Printf("Warning: FCM disabled — %v", err)
		fcmService, _ = services.NewFCMService("") // fall back to no-op
	}
	authService := services.NewAuthService(
		userRepo,
		passwordResetRepo,
		emailVerificationRepo,
		emailService,
		phoneOTPRepo,
		twilioVerify,
		cfg.JWTSecret,
		cfg.JWTExpiry,
		cfg.GoogleClientID,
	)
	productService := services.NewProductService(productRepo)
	categoryService := services.NewCategoryService(categoryRepo)
	shippoClient := services.NewShippoClient(cfg)
	shiprocketClient := services.NewShiprocketClient(cfg)
	shippingService := services.NewShippingService(shippingRepo, shippoClient, shiprocketClient)
	taxService := services.NewTaxService(taxRepo)
	promotionService := services.NewPromotionService(promotionRepo)
	orderCheckoutPricing := services.OrderCheckoutPricing{
		DefaultShippingUSD:      cfg.OrderDefaultShippingUSD,
		DefaultTaxRate:          cfg.OrderDefaultTaxRate,
		FreeShippingSubtotalUSD: cfg.OrderFreeShippingSubtotalUSD,
		INRPerUSD:               cfg.OrderINRPerUSD,
	}
	stripeUSTax := services.NewStripeUSTax(cfg.StripeSecretKey, cfg.StripeProductTaxCode)
	orderService := services.NewOrderService(orderRepo, trackingRepo, productRepo, promotionRepo, promotionService, orderCheckoutPricing, emailService, shippingService, taxService, stripeUSTax, twilioSMSService, fcmService, userRepo, rewardsRepo)
	newsletterService := services.NewNewsletterService(newsletterRepo)
	cartService := services.NewCartService(cartRepo, productRepo)
	wishlistService := services.NewWishlistService(wishlistRepo, productRepo, orderRepo)
	vendorService := services.NewVendorService(vendorRepo, userRepo, emailService, cfg.AppBaseURL)
	vendorApplicationService := services.NewVendorApplicationService(vendorApplicationRepo, vendorService, emailService, cfg.AppBaseURL)
	analyticsService := services.NewAnalyticsService(analyticsRepo, orderRepo, productRepo)
	configService := services.NewConfigService(configRepo)
	storageService := services.NewStorageService(cfg.StorageType, cfg.StorageEndpoint, cfg.StorageAccessKey, cfg.StorageSecretKey, cfg.StorageBucket, cfg.StoragePublicURL)
	reviewService := services.NewReviewService(reviewRepo, productRepo)
	inventoryService := services.NewInventoryService(productRepo)
	supportService := services.NewSupportService(supportRepo)
	paymentService := services.NewPaymentService(paymentRepo, orderRepo, emailService, stripeWebhookEventRepo, cfg.StripeSecretKey, cfg.StripeWebhookSecret)
	// Allow the order service to refund paid orders that are cancelled.
	orderService.AttachRefunder(paymentRepo, paymentService)
	returnService := services.NewReturnService(returnRepo, orderRepo, productRepo, paymentRepo, paymentService, emailService)
	videoService := services.NewVideoService(videoRepo)
	addressService := services.NewAddressService(addressRepo)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, userRepo, cfg.GoogleClientID, cfg.GoogleMapsBrowserKey)
	productHandler := handlers.NewProductHandler(productService, storageService, advancedAnalyticsRepo)
	categoryHandler := handlers.NewCategoryHandler(categoryService)
	orderHandler := handlers.NewOrderHandler(orderService, productService, orderIdempotencyRepo, configService)
	cartHandler := handlers.NewCartHandler(cartService)
	wishlistHandler := handlers.NewWishlistHandler(wishlistService)
	promotionHandler := handlers.NewPromotionHandler(promotionService)
	adminHandler := handlers.NewAdminHandler(
		productService,
		orderService,
		userRepo,
		vendorService,
		promotionService,
		analyticsService,
		configService,
		advancedAnalyticsRepo,
	)
	uploadHandler := handlers.NewUploadHandler(storageService)
	reviewHandler := handlers.NewReviewHandler(reviewService, productRepo)
	returnHandler := handlers.NewReturnHandler(returnService)
	shippingHandler := handlers.NewShippingHandler(shippingService)
	taxHandler := handlers.NewTaxHandler(taxService, stripeUSTax, cfg.OrderDefaultTaxRate)
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	supportHandler := handlers.NewSupportHandler(supportService)
	paymentHandler := handlers.NewPaymentHandler(paymentService)
	shippingWebhookHandler := handlers.NewShippingWebhookHandler(orderRepo, orderService, emailService, cfg.ShippoWebhookSecret, cfg.ShiprocketWebhookSecret)
	videoHandler := handlers.NewVideoHandler(videoService)
	addressHandler := handlers.NewAddressHandler(addressService)
	rewardsHandler := handlers.NewRewardsHandler(orderRepo, rewardsRepo)
	newsletterHandler := handlers.NewNewsletterHandler(newsletterService)
	contactHandler := handlers.NewContactHandler(emailService)
	vendorHandler := handlers.NewVendorHandler(vendorService, vendorAnalyticsRepo)
	notificationHandler := handlers.NewNotificationHandler(userRepo)
	vendorApplicationHandler := handlers.NewVendorApplicationHandler(vendorApplicationService)

	// Setup router
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Middleware (set CORS allowed origins from config before CORS middleware runs)
	middleware.CORSAllowedOrigins = cfg.CORSAllowedOrigins
	router.Use(middleware.CORS())
	router.Use(middleware.Logger())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.Timeout(30 * time.Second)) // 30 second timeout
	
	// Only enable rate limiting in production
	if cfg.Env == "production" {
		router.Use(middleware.RateLimit()) // Rate limiting
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Client.Ping(ctx, nil); err != nil {
			c.JSON(503, gin.H{"status": "unready", "error": "database"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	// API routes
	api := router.Group("/api")
	{
		// Authentication
		auth := api.Group("/auth")
		{
			auth.GET("/client-config", authHandler.GetClientAuthConfig)
			auth.POST("/register", authHandler.Register)
			auth.POST("/register-otp/send", authHandler.SendRegistrationOTP)
			auth.POST("/register-otp/complete", authHandler.CompleteRegistrationOTP)
			auth.POST("/login-otp/send", authHandler.SendLoginOTP)
			auth.POST("/login-otp/complete", authHandler.CompleteLoginOTP)
			auth.POST("/login", authHandler.Login)
			auth.POST("/google", authHandler.GoogleSignIn)
			// No AuthRequired: allow clearing cookie when JWT is missing/expired
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.AuthRequired(), authHandler.GetMe)
			auth.DELETE("/me", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), authHandler.DeleteAccount)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/forgot-password", authHandler.ForgotPassword)
			auth.POST("/reset-password", authHandler.ResetPassword)
			auth.POST("/verify-email", authHandler.VerifyEmail)
			auth.POST("/resend-verification", authHandler.ResendVerification)
			auth.PUT("/profile", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), authHandler.UpdateProfile)
		auth.POST("/avatar", middleware.AuthRequired(), uploadHandler.Upload)
			auth.PUT("/password", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), authHandler.ChangePassword)
			auth.POST("/deactivate", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), authHandler.DeactivateAccount)
		}

		// Notifications (FCM device token registration)
		notificationsG := api.Group("/notifications")
		notificationsG.Use(middleware.AuthRequired())
		notificationsG.Use(middleware.LoadUserMiddleware(userRepo))
		{
			notificationsG.POST("/device-token", notificationHandler.RegisterDeviceToken)
			notificationsG.DELETE("/device-token", notificationHandler.RemoveDeviceToken)
		}

		// Products (specific paths MUST be before /:id to avoid "featured" matching as id)
		products := api.Group("/products")
		{
			products.GET("", productHandler.List)
			products.GET("/featured", productHandler.GetFeatured)
			products.GET("/new-arrivals", productHandler.GetNewArrivals)
			products.GET("/on-sale", productHandler.GetOnSale)
			products.GET("/:id", productHandler.Get)
			products.GET("/:id/recommendations", productHandler.GetRecommendations)
			products.GET("/:id/variants", productHandler.GetVariants)
			products.POST("", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), middleware.RequireRole("admin", "vendor"), productHandler.Create)
			products.PUT("/:id", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), middleware.RequireRole("admin", "vendor"), productHandler.Update)
			products.PUT("/:id/variant-group", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), middleware.RequireRole("admin", "vendor"), productHandler.SetVariantGroup)
			products.DELETE("/:id/variant-group", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), middleware.RequireRole("admin", "vendor"), productHandler.UnsetVariantGroup)
			products.DELETE("/:id", middleware.AuthRequired(), middleware.RequireRole("admin"), productHandler.Delete)
			products.POST("/:id/images", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), middleware.RequireRole("admin", "vendor"), productHandler.UploadImages)
		}

		// Categories
		categories := api.Group("/categories")
		{
			categories.GET("", categoryHandler.List)
			categories.GET("/:id", categoryHandler.Get)
			categories.POST("", middleware.AuthRequired(), middleware.RequireRole("admin"), categoryHandler.Create)
			categories.PUT("/:id", middleware.AuthRequired(), middleware.RequireRole("admin"), categoryHandler.Update)
			categories.DELETE("/:id", middleware.AuthRequired(), middleware.RequireRole("admin"), categoryHandler.Delete)
		}

		// Cart
		cart := api.Group("/cart")
		cart.Use(middleware.AuthRequired())
		cart.Use(middleware.LoadUserMiddleware(userRepo))
		{
			cart.GET("", cartHandler.Get)
			cart.POST("/items", cartHandler.AddItem)
			cart.PUT("/items/:id", cartHandler.UpdateItem)
			cart.PUT("/gift-options", cartHandler.UpdateItemGiftOptions)
			cart.DELETE("/items/:id", cartHandler.RemoveItem)
			cart.DELETE("", cartHandler.Clear)
		}

		// Wishlist
		wishlist := api.Group("/wishlist")
		wishlist.Use(middleware.AuthRequired())
		wishlist.Use(middleware.LoadUserMiddleware(userRepo))
		{
			wishlist.GET("", wishlistHandler.Get)
			wishlist.POST("/items", wishlistHandler.Add)
			wishlist.DELETE("/items/:id", wishlistHandler.Remove)
		}

		// Newsletter
		newsletter := api.Group("/newsletter")
		{
			newsletter.POST("/subscribe", newsletterHandler.Subscribe)
			newsletter.POST("/unsubscribe", newsletterHandler.Unsubscribe)
		}

		api.POST("/contact", contactHandler.Submit)
		api.POST("/vendor/apply", vendorApplicationHandler.Submit)

		// Guest order placement (no auth required; COD only)
		api.POST("/orders/guest", middleware.CheckoutRateLimit(), orderHandler.CreateGuest)

		// Orders
		orders := api.Group("/orders")
		orders.Use(middleware.AuthRequired())
		orders.Use(middleware.LoadUserMiddleware(userRepo))
		{
			orders.GET("", orderHandler.List)
			orders.GET("/:id", orderHandler.Get)
			orders.POST("", middleware.CheckoutRateLimit(), orderHandler.Create)
			orders.GET("/tracking/:orderNumber", orderHandler.Track)
			orders.GET("/:id/tracking", orderHandler.GetTracking)
			orders.POST("/:id/cancel", orderHandler.Cancel)
			orders.POST("/:id/return", returnHandler.Create)
			orders.PUT("/:id/status", middleware.RequireRole("admin"), orderHandler.UpdateStatus)
			orders.POST("/:id/fulfill", middleware.RequireRole("admin"), orderHandler.Fulfill)
		}

		// Returns
		returns := api.Group("/returns")
		returns.Use(middleware.AuthRequired())
		returns.Use(middleware.LoadUserMiddleware(userRepo))
		{
			returns.GET("", returnHandler.List)
			returns.GET("/:id", returnHandler.Get)
		}

		// My reviews (must be before /products/:id/reviews)
		reviewsMe := api.Group("/reviews")
		reviewsMe.Use(middleware.AuthRequired())
		reviewsMe.Use(middleware.LoadUserMiddleware(userRepo))
		{
			reviewsMe.GET("/me", reviewHandler.GetByUser)
		}

		// Reviews by product
		reviews := api.Group("/products/:id/reviews")
		{
			reviews.GET("", reviewHandler.GetByProduct)
			reviews.POST("", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), reviewHandler.Create)
			reviews.PUT("/:reviewId", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), reviewHandler.Update)
			reviews.DELETE("/:reviewId", middleware.AuthRequired(), middleware.LoadUserMiddleware(userRepo), reviewHandler.Delete)
			reviews.POST("/:reviewId/helpful", middleware.AuthRequired(), reviewHandler.MarkHelpful)
		}

		// Support
		support := api.Group("/support")
		support.Use(middleware.AuthRequired())
		support.Use(middleware.LoadUserMiddleware(userRepo))
		{
			support.GET("/tickets", supportHandler.List)
			support.POST("/tickets", supportHandler.Create)
			support.GET("/tickets/:id", supportHandler.Get)
			support.POST("/tickets/:id/messages", supportHandler.AddMessage)
		}

		// Shipping
		shipping := api.Group("/shipping")
		{
			shipping.POST("/calculate", shippingHandler.Calculate)
			shipping.GET("/methods", shippingHandler.List)
		}

		// Tax
		tax := api.Group("/tax")
		{
			tax.POST("/calculate", taxHandler.Calculate)
			tax.GET("/rates", taxHandler.List)
		}

		// Payments
		payments := api.Group("/payments")
		payments.Use(middleware.AuthRequired())
		payments.Use(middleware.LoadUserMiddleware(userRepo))
		{
			payments.POST("/intent", middleware.CheckoutRateLimit(), paymentHandler.CreatePaymentIntent)
			payments.POST("/process", paymentHandler.ProcessPayment)
			payments.GET("/transactions/:id", paymentHandler.GetTransaction)
			payments.POST("/refund/:id", middleware.RequireRole("admin"), paymentHandler.Refund)
		}

		// Webhooks (no auth required, verified by signature/secret)
		webhooks := api.Group("/webhooks")
		{
			webhooks.POST("/:gateway", paymentHandler.HandleWebhook)
			webhooks.POST("/shippo", shippingWebhookHandler.HandleShippoWebhook)
			webhooks.POST("/shiprocket", shippingWebhookHandler.HandleShiprocketWebhook)
		}

		// Promotions
		promotions := api.Group("/promotions")
		{
			promotions.GET("", promotionHandler.List)
			promotions.POST("/validate", promotionHandler.Validate)
		}

		// Videos (public endpoints)
		videos := api.Group("/videos")
		{
			videos.GET("", videoHandler.List)
			videos.GET("/featured", videoHandler.GetFeatured)
			videos.GET("/:id", videoHandler.Get)
		}

		// Addresses (authenticated)
		addresses := api.Group("/addresses")
		addresses.Use(middleware.AuthRequired())
		addresses.Use(middleware.LoadUserMiddleware(userRepo))
		{
			addresses.GET("", addressHandler.List)
			addresses.POST("", addressHandler.Create)
			addresses.GET("/:id", addressHandler.Get)
			addresses.PUT("/:id", addressHandler.Update)
			addresses.DELETE("/:id", addressHandler.Delete)
			addresses.PUT("/:id/default", addressHandler.SetDefault)
		}

		// Rewards (authenticated)
		rewardsG := api.Group("/rewards")
		rewardsG.Use(middleware.AuthRequired())
		rewardsG.Use(middleware.LoadUserMiddleware(userRepo))
		{
			rewardsG.GET("/summary", rewardsHandler.GetSummary)
			rewardsG.POST("/redeem", rewardsHandler.RedeemPoints)
			rewardsG.GET("/transactions", rewardsHandler.GetTransactions)
		}

		// Public Configuration & Content (no auth required)
		api.GET("/config", adminHandler.GetPublicConfig)
		api.GET("/content", adminHandler.GetPublicContent)

		// Upload (staff only — arbitrary authenticated users must not fill object storage)
		upload := api.Group("/upload")
		upload.Use(middleware.AuthRequired())
		upload.Use(middleware.LoadUserMiddleware(userRepo))
		upload.Use(middleware.RequireRole("admin", "vendor"))
		{
			upload.POST("", uploadHandler.Upload)
			upload.DELETE("/:filename", uploadHandler.Delete)
		}

		// Vendor self-service (vendor JWT only)
		vendorSelf := api.Group("/vendor")
		vendorSelf.Use(middleware.AuthRequired())
		vendorSelf.Use(middleware.LoadUserMiddleware(userRepo))
		vendorSelf.Use(middleware.RequireRole("vendor"))
		{
			vendorSelf.GET("/me", vendorHandler.GetMe)
			vendorSelf.PUT("/me", vendorHandler.UpdateMe)
			vendorSelf.GET("/analytics/summary", vendorHandler.GetAnalyticsSummary)
			vendorSelf.GET("/analytics/revenue", vendorHandler.GetAnalyticsRevenue)
			vendorSelf.GET("/analytics/products", vendorHandler.GetAnalyticsProducts)
			vendorSelf.GET("/analytics/orders", vendorHandler.GetAnalyticsOrders)
		}

		// Admin routes (admin + vendor; sensitive routes require admin only)
		admin := api.Group("/admin")
		admin.Use(middleware.AuthRequired())
		admin.Use(middleware.LoadUserMiddleware(userRepo))
		admin.Use(middleware.RequireRole("admin", "vendor"))
		{
			// Dashboard (admin-only: platform-wide metrics)
			admin.GET("/dashboard/stats", middleware.RequireRole("admin"), adminHandler.GetDashboardStats)
			admin.GET("/dashboard/sales", middleware.RequireRole("admin"), adminHandler.GetDashboardSales)
			admin.GET("/dashboard/orders", middleware.RequireRole("admin"), adminHandler.GetDashboardOrders)
			admin.GET("/dashboard/products", middleware.RequireRole("admin"), adminHandler.GetDashboardProducts)

			// Customers
			admin.GET("/customers", middleware.RequireRole("admin"), adminHandler.ListCustomers)
			admin.GET("/customers/:id", middleware.RequireRole("admin"), adminHandler.GetCustomer)
			admin.PUT("/customers/:id", middleware.RequireRole("admin"), adminHandler.UpdateCustomer)

			// Vendors
			admin.GET("/vendor-applications", middleware.RequireRole("admin"), vendorApplicationHandler.List)
			admin.GET("/vendor-applications/:id", middleware.RequireRole("admin"), vendorApplicationHandler.GetOne)
			admin.POST("/vendor-applications/:id/approve", middleware.RequireRole("admin"), vendorApplicationHandler.Approve)
			admin.POST("/vendor-applications/:id/reject", middleware.RequireRole("admin"), vendorApplicationHandler.Reject)

			admin.GET("/vendors", middleware.RequireRole("admin"), adminHandler.ListVendors)
			admin.GET("/vendors/:id", middleware.RequireRole("admin"), adminHandler.GetVendor)
			admin.POST("/vendors", middleware.RequireRole("admin"), adminHandler.CreateVendor)
			admin.PUT("/vendors/:id", middleware.RequireRole("admin"), adminHandler.UpdateVendor)
			admin.DELETE("/vendors/:id", middleware.RequireRole("admin"), adminHandler.DeleteVendor)
			admin.PUT("/vendors/:id/permissions", middleware.RequireRole("admin"), adminHandler.UpdateVendorPermissions)

			// Promotions
			admin.GET("/promotions", middleware.RequireRole("admin"), adminHandler.ListPromotions)
			admin.POST("/promotions", middleware.RequireRole("admin"), adminHandler.CreatePromotion)
			admin.PUT("/promotions/:id", middleware.RequireRole("admin"), adminHandler.UpdatePromotion)
			admin.DELETE("/promotions/:id", middleware.RequireRole("admin"), adminHandler.DeletePromotion)

			// Analytics (admin-only)
			admin.GET("/analytics/revenue", middleware.RequireRole("admin"), adminHandler.GetRevenueAnalytics)
			admin.GET("/analytics/orders", middleware.RequireRole("admin"), adminHandler.GetOrderAnalytics)
			admin.GET("/analytics/products", middleware.RequireRole("admin"), adminHandler.GetProductAnalytics)
			admin.GET("/analytics/customers", middleware.RequireRole("admin"), adminHandler.GetCustomerAnalytics)
			admin.GET("/analytics/traffic", middleware.RequireRole("admin"), adminHandler.GetTrafficAnalytics)
			admin.GET("/analytics/cohort", middleware.RequireRole("admin"), adminHandler.GetCohortAnalytics)
			admin.GET("/analytics/funnel", middleware.RequireRole("admin"), adminHandler.GetFunnelAnalytics)
			admin.GET("/analytics/recommendations", middleware.RequireRole("admin"), adminHandler.GetAdminProductRecommendations)

			// Variant migration
			admin.POST("/products/migrate-variants", middleware.RequireRole("admin"), productHandler.AutoSplitVariants)

			// Content
			admin.GET("/content", middleware.RequireRole("admin"), adminHandler.GetContent)
			admin.PUT("/content", middleware.RequireRole("admin"), adminHandler.UpdateContent)

			// Settings
			admin.GET("/settings", middleware.RequireRole("admin"), adminHandler.GetSettings)
			admin.PUT("/settings", middleware.RequireRole("admin"), adminHandler.UpdateSettings)

			// Configuration
			admin.GET("/configuration", middleware.RequireRole("admin"), adminHandler.GetConfiguration)
			admin.PUT("/configuration", middleware.RequireRole("admin"), adminHandler.UpdateConfiguration)

			// Reviews (moderation queue — admin only)
			admin.GET("/reviews", middleware.RequireRole("admin"), reviewHandler.List)
			admin.PUT("/reviews/:id/status", middleware.RequireRole("admin"), reviewHandler.UpdateStatus)

			// Returns
			admin.GET("/returns", middleware.RequireRole("admin"), returnHandler.ListAll)
			admin.PUT("/returns/:id/status", middleware.RequireRole("admin"), returnHandler.UpdateStatus)
			admin.PUT("/returns/:id/approve", middleware.RequireRole("admin"), returnHandler.Approve)
			admin.PUT("/returns/:id/reject", middleware.RequireRole("admin"), returnHandler.Reject)
			admin.PUT("/returns/:id/process-refund", middleware.RequireRole("admin"), returnHandler.ProcessRefund)

			// Shipping
			admin.GET("/shipping/methods", middleware.RequireRole("admin"), shippingHandler.List)
			admin.POST("/shipping/methods", middleware.RequireRole("admin"), shippingHandler.Create)
			admin.PUT("/shipping/methods/:id", middleware.RequireRole("admin"), shippingHandler.Update)
			admin.DELETE("/shipping/methods/:id", middleware.RequireRole("admin"), shippingHandler.Delete)

			// Tax
			admin.GET("/tax/rates", middleware.RequireRole("admin"), taxHandler.List)
			admin.POST("/tax/rates", middleware.RequireRole("admin"), taxHandler.Create)
			admin.PUT("/tax/rates/:id", middleware.RequireRole("admin"), taxHandler.Update)
			admin.DELETE("/tax/rates/:id", middleware.RequireRole("admin"), taxHandler.Delete)

			// Inventory
			admin.GET("/inventory/low-stock", middleware.RequireRole("admin"), inventoryHandler.GetLowStock)
			admin.GET("/inventory/alerts", middleware.RequireRole("admin"), inventoryHandler.GetAlerts)

			// Support
			admin.GET("/support/tickets", middleware.RequireRole("admin"), supportHandler.List)
			admin.PUT("/support/tickets/:id/status", middleware.RequireRole("admin"), supportHandler.UpdateStatus)
			admin.PUT("/support/tickets/:id/assign", middleware.RequireRole("admin"), supportHandler.Assign)

			// Wishlist Analytics
			admin.GET("/wishlist/analytics", middleware.RequireRole("admin"), wishlistHandler.GetProductAnalytics)
			admin.GET("/wishlist/analytics/users", middleware.RequireRole("admin"), wishlistHandler.GetUserAnalytics)

			// Videos (admin-only management)
			admin.POST("/videos", middleware.RequireRole("admin"), videoHandler.Create)
			admin.PUT("/videos/:id", middleware.RequireRole("admin"), videoHandler.Update)
			admin.DELETE("/videos/:id", middleware.RequireRole("admin"), videoHandler.Delete)
		}
	}

	// Start server - bind to 0.0.0.0 so emulator/other devices can connect
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := "0.0.0.0:" + port
	log.Printf("Server starting on %s (API at http://localhost:%s/api)", addr, port)
	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

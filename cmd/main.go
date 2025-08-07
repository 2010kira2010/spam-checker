package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"spam-checker/internal/config"
	"spam-checker/internal/database"
	"spam-checker/internal/handlers"
	"spam-checker/internal/middleware"
	"spam-checker/internal/scheduler"
	"spam-checker/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"github.com/sirupsen/logrus"

	_ "spam-checker/docs"            // Import generated docs - uncomment after swagger generation
	_ "spam-checker/internal/models" // Import models to make types available for swagger
)

// @title SpamChecker API
// @version 1.0
// @description API for checking phone numbers in spam services
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@spamchecker.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logger
	setupLogger(cfg.App.LogLevel)

	// Connect to database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		logrus.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		logrus.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize services
	userService := services.NewUserService(db)
	phoneService := services.NewPhoneService(db)
	checkService := services.NewCheckService(db, cfg)
	adbService := services.NewADBService(db, cfg)
	settingsService := services.NewSettingsService(db)
	statisticsService := services.NewStatisticsService(db)
	notificationService := services.NewNotificationService(db)

	// Initialize scheduler
	checkScheduler := scheduler.NewCheckScheduler(db, checkService, phoneService, notificationService, cfg)
	checkScheduler.Start()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:               cfg.App.Name,
		DisableStartupMessage: false,
		ErrorHandler:          customErrorHandler,
		BodyLimit:             500 * 1024 * 1024, // 500 MB limit for APK files
		ReadTimeout:           5 * time.Minute,   // Increase timeout for large uploads
		WriteTimeout:          5 * time.Minute,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS, PATCH",
		AllowCredentials: true,
	}))

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWT)

	// API routes
	api := app.Group("/api/v1")

	// Public routes
	handlers.RegisterAuthRoutes(api, userService, cfg.JWT)

	// Swagger
	app.Get("/swagger/*", swagger.HandlerDefault)

	// Protected routes
	protected := api.Use(authMiddleware.Protect())

	// User routes
	handlers.RegisterUserRoutes(protected, userService, authMiddleware)

	// Phone number routes
	handlers.RegisterPhoneRoutes(protected, phoneService, authMiddleware)

	// Check routes
	handlers.RegisterCheckRoutes(protected, checkService, authMiddleware)

	// ADB Gateway routes
	handlers.RegisterADBRoutes(protected, adbService, authMiddleware)

	// Settings routes
	handlers.RegisterSettingsRoutes(protected, settingsService, authMiddleware)

	// Statistics routes
	handlers.RegisterStatisticsRoutes(protected, statisticsService, authMiddleware)

	// Notification routes
	handlers.RegisterNotificationRoutes(protected, notificationService, authMiddleware)

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"app":    cfg.App.Name,
			"env":    cfg.App.Environment,
		})
	})

	// Serve static files (React app)
	app.Static("/", "./static", fiber.Static{
		Compress:      true,
		ByteRange:     true,
		Browse:        false,
		CacheDuration: 10 * time.Minute,
		MaxAge:        3600,
	})

	// Serve index.html for all non-API routes (React Router support)
	app.Get("/*", func(c *fiber.Ctx) error {
		// Skip API routes
		if strings.HasPrefix(c.Path(), "/api") || strings.HasPrefix(c.Path(), "/swagger") {
			return c.Next()
		}
		return c.SendFile("./static/index.html")
	})

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		logrus.Info("Shutting down server...")
		checkScheduler.Stop()
		if err := app.Shutdown(); err != nil {
			logrus.Errorf("Server shutdown error: %v", err)
		}
	}()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.App.Port)
	logrus.Infof("Starting server on %s", addr)
	if err := app.Listen(addr); err != nil {
		logrus.Fatalf("Failed to start server: %v", err)
	}
}

func setupLogger(level string) {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	switch level {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	logrus.Errorf("Error: %v", err)

	return c.Status(code).JSON(fiber.Map{
		"error": message,
		"code":  code,
	})
}

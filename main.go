package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"weave/config"
	"weave/middleware"
	"weave/models"
	"weave/pkg"
	"weave/pkg/migrate/migration"
	"weave/plugins"
	"weave/plugins/core"
	"weave/plugins/examples"
	fc "weave/plugins/features/FormatConverter"
	note "weave/plugins/features/Note"
	"weave/routers"
	"weave/services/llm"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// loadEnvFile 加载环境变量
func loadEnvFile(filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		pkg.Debug("Failed to read .env file", zap.Error(err), zap.String("path", filePath))
		return
	}

	for _, line := range strings.Split(string(content), "\n") {
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if key != "" {
				os.Setenv(key, value)
			}
		}
	}
}

func main() {
	loadEnvFile(".env")

	// 加载配置
	if err := config.LoadConfig(); err != nil {
		pkg.Fatal("Failed to load configuration", zap.Error(err))
	}

	// 初始化日志系统
	if err := pkg.InitLogger(pkg.Options{
		Level:       config.Config.Logger.Level,
		OutputPath:  config.Config.Logger.OutputPath,
		ErrorPath:   config.Config.Logger.ErrorPath,
		Development: config.Config.Logger.Development,
	}); err != nil {
		pkg.Fatal("Failed to initialize logger", zap.Error(err))
	}
	defer pkg.Sync()

	// PluginManager 日志记录器
	plugins.PluginManager.SetLogger(pkg.GetLogger())

	pkg.Info("Configuration loaded successfully", zap.Any("config", config.SanitizeConfig()))

	// 验证配置完整性
	if err := config.ValidateConfig(); err != nil {
		pkg.Fatal("Configuration validation failed", zap.Error(err))
	}
	pkg.Info("Configuration validation passed successfully")

	// 初始化数据库
	if err := pkg.InitDatabase(); err != nil {
		pkg.Fatal("Failed to initialize database", zap.Error(err))
	}
	pkg.Info("Database initialized successfully")

	// 数据库迁移(迁移完成 --> 启动服务)
	pkg.Debug("AutoMigrate setting", zap.Bool("enabled", config.Config.AutoMigrate))
	if !config.Config.AutoMigrate {
		pkg.Info("Starting SQL migrations...")
		mm := migration.NewMigrationManager()
		if err := mm.Init(); err != nil {
			pkg.Error("Failed to initialize migration manager", zap.Error(err))
		} else {
			if err := mm.Up(); err != nil {
				pkg.Error("Migration errors", zap.Error(err))
			} else {
				pkg.Info("SQL migrations completed successfully")
			}
		}
	} else {
		// 启用GORM自动迁移
		pkg.Info("Starting GORM auto-migration...")
		if pkg.DB == nil {
			pkg.Error("Database connection is nil!")
		} else {
			if err := models.MigrateTables(pkg.DB); err != nil {
				pkg.Error("Failed to migrate database tables", zap.Error(err))
			} else {
				pkg.Info("GORM auto-migration completed successfully")
			}
		}
	}

	// 初始化路由(监控指标、中间件等已在路由设置中配置)
	router := routers.SetupRouter()

	// 添加错误处理中间件
	errHandler := middleware.NewErrorHandler()
	router.Use(errHandler.HandlerFunc())

	// 注册插件
	registerPlugins(router)

	// 初始化插件系统
	if err := plugins.InitPluginSystem(); err != nil {
		pkg.Error("Failed to initialize plugin system", zap.Error(err))
	}

	// 启动服务器
	port := config.Config.Server.Port
	instanceID := config.Config.Server.InstanceID

	// 创建HTTP服务器并配置连接复用参数
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        router,
		ReadTimeout:    15 * time.Second, // 请求读取超时时间
		WriteTimeout:   15 * time.Second, // 响应写入超时时间
		IdleTimeout:    60 * time.Second, // 空闲连接超时时间（影响Keep-Alive）
		MaxHeaderBytes: 1 << 20,          // 最大请求头大小（1MB）
	}

	go func() {
		pkg.Info("Weave 服务启动成功",
			zap.String("instance_id", instanceID),
			zap.String("address", fmt.Sprintf("http://localhost:%d", port)))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			pkg.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	pkg.Info("Shutting down server...")

	// 停止插件监控器
	plugins.PluginManager.StopPluginWatcher()

	// 创建超时上下文，用于优雅关闭服务器和数据库
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 关闭HTTP服务器
	if err := srv.Shutdown(ctx); err != nil {
		pkg.Fatal("Server forced to shutdown", zap.Error(err))
	}

	// 使用相同上下文优雅关闭数据库连接
	if err := pkg.CloseDatabaseWithContext(ctx); err != nil {
		pkg.Error("Database shutdown error", zap.Error(err))
	}

	pkg.Info("Server exiting")
}

// 注册插件
func registerPlugins(router *gin.Engine) {
	// 设置路由引擎到PluginManager
	plugins.PluginManager.SetRouter(router)

	// 注册插件列表
	pluginsToRegister := []core.Plugin{
		&examples.HelloPlugin{},
		&note.NotePlugin{},
		&fc.FormatConverterPlugin{},
		llm.NewLLMChatPlugin(),
		examples.NewSampleOptimizedPlugin(),
		examples.NewSampleDependentPlugin(),
	}

	for _, plugin := range pluginsToRegister {
		if err := plugins.PluginManager.Register(plugin); err != nil {
			// 只记录错误，不记录成功的插件注册
			pkg.Error("Failed to register plugin", zap.String("plugin", fmt.Sprintf("%T", plugin)), zap.Error(err))
		}
	}

	// 所有插件注册完成，输出确认日志
	pkg.Info("插件已全部注册运行成功")
}

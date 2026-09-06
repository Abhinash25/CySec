// Package cli implements the cobra command tree for CySec.env.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/cysec-env/cysec/internal/cache"
	"github.com/cysec-env/cysec/internal/config"
	"github.com/cysec-env/cysec/internal/database"
	"github.com/cysec-env/cysec/internal/environment"
	"github.com/cysec-env/cysec/internal/shell"
	"github.com/cysec-env/cysec/internal/installer"
	"github.com/cysec-env/cysec/internal/logger"
	"github.com/cysec-env/cysec/internal/platform"
	"github.com/cysec-env/cysec/internal/registry"
	"github.com/cysec-env/cysec/internal/resolver"
	"github.com/cysec-env/cysec/internal/shim"
	"github.com/cysec-env/cysec/internal/ui"
)

// Version is set at build time.
var Version = "0.1.0-dev"

// App holds all shared application dependencies.
type App struct {
	Platform *platform.Info
	Config   *config.Config
	Log      *logger.Logger
	Registry *registry.Registry
	DB       *database.StateDB
	Engine   *installer.Engine
	Shims    *shim.Manager
	Env      *environment.Manager
	Cache    *cache.Manager
	Resolver *resolver.Resolver
}

// initApp initializes all application components.
func initApp() (*App, error) {
	// Platform detection
	pinfo, err := platform.Detect()
	if err != nil {
		return nil, fmt.Errorf("platform detection failed: %w", err)
	}

	// Ensure all directories exist
	if err := pinfo.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("directory setup failed: %w", err)
	}

	// Configuration
	cfg, err := config.Load(pinfo.ConfigDir())
	if err != nil {
		return nil, fmt.Errorf("configuration failed: %w", err)
	}

	// Logger
	log := logger.New(
		logger.ParseLevel(cfg.GetString("logging.level")),
		cfg.GetBool("logging.redact_secrets"),
	)
	if err := log.AddFileOutput(pinfo.LogDir); err != nil {
		// Non-fatal: log to stderr only
		fmt.Fprintf(os.Stderr, "Warning: could not open log file: %v\n", err)
	}

	// Registry â€” load from embedded registry files first, then overlay local
	reg := registry.New(pinfo.RegistryDir())

	// Load the bundled registry from the executable's directory
	execPath, _ := os.Executable()
	if execPath != "" {
		bundledRegistry := filepath.Join(filepath.Dir(execPath), "..", "registry")
		_ = reg.LoadFromDir(bundledRegistry)
	}

	// Also try loading from working directory (for development)
	if cwd, err := os.Getwd(); err == nil {
		_ = reg.LoadFromDir(fmt.Sprintf("%s/registry", cwd))
	}

	// Load local registry (user/updated registry)
	_ = reg.LoadFromDir(pinfo.RegistryDir())

	// Database
	db, err := database.Open(pinfo.DBDir)
	if err != nil {
		return nil, fmt.Errorf("database failed: %w", err)
	}

	// Installer engine
	engine := installer.NewEngine(
		pinfo.ToolsDir(),
		fmt.Sprintf("%s/downloads", pinfo.CacheDir),
		log,
	)

	// Shim manager
	shims := shim.NewManager(pinfo.ShimDir)

	// Environment manager
	env := environment.New(pinfo)

	// Cache manager
	cacheMgr := cache.NewManager(pinfo.CacheDir, pinfo.WorkspaceDir())

	// Resolver
	res := resolver.New(db, reg, runtime.GOOS, runtime.GOARCH)

	// Recover any stale installations
	engine.RecoverStaleInstallations(db, reg, shims)

	return &App{
		Platform: pinfo,
		Config:   cfg,
		Log:      log,
		Registry: reg,
		DB:       db,
		Engine:   engine,
		Shims:    shims,
		Env:      env,
		Cache:    cacheMgr,
		Resolver: res,
	}, nil
}

// NewRootCmd builds the root cobra command and all subcommands.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "cysec [command]",
		Short: "CySec.env â€” One Environment. Every Tool.",
		Long:  "CySec.env is a unified, managed cybersecurity tool environment.\nRun `cysec` to enter the managed environment or use subcommands.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := initApp()
			if err != nil {
				ui.Error(fmt.Sprintf("Initialization failed: %v", err))
				return err
			}
			defer app.DB.Close()

			if len(args) == 0 {
				// Enter managed shell
				profile, _ := app.DB.GetMeta("setup_profile")
				if app.Config.GetBool("banner.show_on_start") {
					if app.Config.IsFirstRun() {
						fmt.Println(ui.Banner())
						_ = app.Config.MarkFirstRunDone()
					} else if !app.Config.GetBool("banner.first_run_only") || profile != "" {
						// Also print banner with status
						fmt.Println(ui.BannerWithStatus(profile, app.DB.Count()))
					}
				}

				if environment.IsActive() {
					ui.Warn("Already inside a CySec.env session.")
					return nil
				}

				ui.Info("Entering CySec.env managed environment...")
				ui.Step(fmt.Sprintf("Tools: %d installed, %d available", app.DB.Count(), app.Registry.Count()))
				ui.Step("Type 'exit' to leave the environment")
				fmt.Println()

				interactiveShell := shell.New(app.Platform, app.Registry, app.DB, app.Engine, app.Shims)
				return interactiveShell.Run()
			}

			// Handle command resolution for arbitrary args
			query := args[0]
			res := app.Resolver.Resolve(query)

			switch res.Status {
			case resolver.StatusInstalled:
				err := app.Engine.HealthCheck(res.Tool)
				if err != nil {
					ui.Error(fmt.Sprintf("Tool '%s' is installed but appears broken: %v", res.Tool.DisplayName, err))
					ui.Step(fmt.Sprintf("To fix this, run 'cysec install %s' again or use 'cysec doctor'.", res.Tool.ID))
					app.DB.UpdateHealth(res.Tool.ID, "broken")
				} else {
					ui.Success(fmt.Sprintf("%s is already installed and healthy.", res.Tool.DisplayName))
					ui.Info(fmt.Sprintf("You can run it directly by typing '%s' in the CySec.env environment.", res.Tool.EntryCommand))
				}
			case resolver.StatusAvailable:
				ui.Info(fmt.Sprintf("Tool '%s' is available but not installed.", res.Tool.DisplayName))
				ui.Step(fmt.Sprintf("Run 'cysec install %s' to install it.", res.Tool.ID))
			case resolver.StatusUnsupported:
				ui.Error(fmt.Sprintf("Tool '%s' is not supported on your current platform.", res.Tool.DisplayName))
			case resolver.StatusDeprecated:
				ui.Warn(fmt.Sprintf("Tool '%s' is deprecated.", res.Tool.DisplayName))
				if res.Tool.Replacement != "" {
					ui.Step(fmt.Sprintf("Consider using '%s' instead.", res.Tool.Replacement))
				}
			default:
				ui.Error(fmt.Sprintf("Command or tool not found: %s", query))
			}

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Version
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(fmt.Sprintf("CySec.env %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH))

	// Register subcommands
	rootCmd.AddCommand(
		newBannerCmd(),
		newToolsCmd(),
		newSearchCmd(),
		newInfoCmd(),
		newSetupCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		newUpdateCmd(),
		newRegistryCmd(),
		newDoctorCmd(),
		newStorageCmd(),
		newCacheCmd(),
	)

	return rootCmd
}


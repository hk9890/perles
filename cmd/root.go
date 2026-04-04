package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hk9890/perles/internal/app"
	beads "github.com/hk9890/perles/internal/beads/domain"
	infrabeads "github.com/hk9890/perles/internal/beads/infrastructure"
	"github.com/hk9890/perles/internal/bql"
	"github.com/hk9890/perles/internal/cachemanager"
	"github.com/hk9890/perles/internal/config"
	"github.com/hk9890/perles/internal/keys"
	"github.com/hk9890/perles/internal/log"
	"github.com/hk9890/perles/internal/paths"
	appreg "github.com/hk9890/perles/internal/registry/application"
	"github.com/hk9890/perles/internal/templates"
	"github.com/hk9890/perles/internal/ui/nobeads"
	"github.com/hk9890/perles/internal/ui/outdated"
)

func init() {
	// Force lipgloss/termenv to query terminal background color BEFORE
	// any Bubble Tea program starts. This prevents the terminal's OSC 11
	// response from racing with Bubble Tea's input loop and appearing as
	// garbage text in input fields.
	//
	// See: https://github.com/charmbracelet/bubbletea/issues/1036
	_ = lipgloss.HasDarkBackground()
}

var (
	version         = "dev"
	cfgFile         string
	cfg             config.Config
	debugFlag       bool
	apiPortFlag     int
	registryService *appreg.RegistryService
	newDoltClient   = infrabeads.NewDoltClient
)

const doltHealthCheckIntervalEnv = "PERLES_DOLT_HEALTH_CHECK_INTERVAL"

type startupBehavior int

const (
	startupBehaviorProceed startupBehavior = iota
	startupBehaviorNoBeadsMode
	startupBehaviorCompatibilityMode
	startupBehaviorReturnError
)

func classifyStartupBehavior(err error) startupBehavior {
	if err == nil {
		return startupBehaviorProceed
	}

	if infrabeads.IsNoBeadsError(err) {
		return startupBehaviorNoBeadsMode
	}
	if infrabeads.IsCompatibilityError(err) {
		return startupBehaviorCompatibilityMode
	}

	return startupBehaviorReturnError
}

var rootCmd = &cobra.Command{
	Use:     "perles",
	Short:   "A terminal ui for beads issue tracking",
	Long:    `A terminal user interface for viewing and managing beads issues in a kanban-style board with BQL support.`,
	Version: version,
	RunE:    runApp,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "",
		"config file (default: ~/.config/perles/config.yaml)")
	rootCmd.Flags().StringP("beads-dir", "b", "",
		"path to beads database directory")
	rootCmd.Flags().StringP("markdown-style", "", "",
		"markdown rendering style: \"dark\" (default) or \"light\"")
	rootCmd.Flags().Bool("no-auto-refresh", false,
		"disable auto-refresh (overrides config)")
	rootCmd.PersistentFlags().BoolVarP(&debugFlag, "debug", "d", false,
		"enable debug mode with verbose runtime logging (also: PERLES_DEBUG=1)")
	rootCmd.Flags().IntVarP(&apiPortFlag, "port", "p", 0,
		"API server port (0 = auto-assign, overrides config)")

	_ = viper.BindPFlag("beads_dir", rootCmd.Flags().Lookup("beads-dir"))
	_ = viper.BindPFlag("ui.markdown_style", rootCmd.Flags().Lookup("markdown-style"))
}

func initConfig() {
	defaults := config.Defaults()
	viper.SetDefault("auto_refresh", defaults.AutoRefresh)
	viper.SetDefault("ui.show_counts", defaults.UI.ShowCounts)
	viper.SetDefault("ui.markdown_style", defaults.UI.MarkdownStyle)
	viper.SetDefault("theme.preset", defaults.Theme.Preset)

	// Orchestration defaults
	viper.SetDefault("orchestration.client", defaults.Orchestration.CoordinatorClient)
	viper.SetDefault("orchestration.coordinator_client", defaults.Orchestration.CoordinatorClient)
	viper.SetDefault("orchestration.worker_client", defaults.Orchestration.WorkerClient)
	viper.SetDefault("orchestration.claude.model", defaults.Orchestration.Claude.Model)
	viper.SetDefault("orchestration.amp.model", defaults.Orchestration.Amp.Model)
	viper.SetDefault("orchestration.amp.mode", defaults.Orchestration.Amp.Mode)

	// Sound defaults
	viper.SetDefault("sound.events", defaults.Sound.Events)

	// Keybinding defaults
	viper.SetDefault("ui.keybindings.search", "ctrl+space")
	viper.SetDefault("ui.keybindings.dashboard", "ctrl+o")
	viper.SetDefault("ui.keybindings.quit", defaults.UI.Keybindings.Quit)
	viper.SetDefault("ui.keybindings.editor", defaults.UI.Keybindings.Editor)
	viper.SetDefault("ui.quit_confirmation", defaults.UI.QuitConfirm)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Config lookup order:
		// 1. .perles/config.yaml (current directory)
		// 2. ~/.config/perles/config.yaml (user config)
		if _, err := os.Stat(".perles/config.yaml"); err == nil {
			viper.SetConfigFile(".perles/config.yaml")
		} else {
			home, _ := os.UserHomeDir()
			viper.AddConfigPath(filepath.Join(home, ".config", "perles"))
			viper.SetConfigName("config")
			viper.SetConfigType("yaml")
		}
	}

	if err := viper.ReadInConfig(); err != nil {
		// No config file found anywhere - create default at .perles/config.yaml
		var configNotFound viper.ConfigFileNotFoundError
		if errors.As(err, &configNotFound) {
			defaultPath := ".perles/config.yaml"
			if writeErr := config.WriteDefaultConfig(defaultPath); writeErr == nil {
				viper.SetConfigFile(defaultPath)
				_ = viper.ReadInConfig()
				log.Info(log.CatConfig, "Config loaded", "path", defaultPath)
			}
			// If write fails, just continue with defaults (no config file)
		}
	} else {
		log.Info(log.CatConfig, "Config loaded", "path", viper.ConfigFileUsed())
	}

	_ = viper.Unmarshal(&cfg)
}

func initServices() {
	// Initialize registry service with embedded templates and user-defined workflows
	// templates.RegistryFS() contains template.yaml, workflow templates, and coordinator instructions
	// User workflows are loaded from ~/.perles/workflows/*/template.yaml
	var err error
	registryService, err = appreg.NewRegistryService(
		templates.RegistryFS(),
		appreg.UserRegistryBaseDir(),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error initializing registry service:", err)
		os.Exit(1)
	}
}

func runApp(cmd *cobra.Command, args []string) error {
	debug, cleanupLogging, err := initRuntimeLogging("perles")
	if err != nil {
		return err
	}
	defer cleanupLogging()

	if debug {
		log.Info(log.CatConfig, "Perles starting", "version", version, "debug", true, "logPath", resolveDebugLogPath())
	}

	// Initialize registry service after logging so debug output is captured
	initServices()

	if err := config.ValidateViews(cfg.Views); err != nil {
		return fmt.Errorf("invalid view configuration: %w", err)
	}

	if err := config.ValidateOrchestration(cfg.Orchestration); err != nil {
		return fmt.Errorf("invalid orchestration configuration: %w", err)
	}

	if err := config.ValidateSound(cfg.Sound); err != nil {
		return fmt.Errorf("invalid sound configuration: %w", err)
	}

	// Apply --port flag override (takes precedence over config)
	if apiPortFlag != 0 {
		cfg.Orchestration.APIPort = apiPortFlag
	}

	// Validate keybindings before applying
	if err := config.ValidateKeybindings(cfg.UI.Keybindings); err != nil {
		return fmt.Errorf("invalid keybindings configuration: %w", err)
	}

	// Validate user-defined actions
	if err := config.ValidateActions(cfg.UI.Actions); err != nil {
		return fmt.Errorf("invalid actions configuration: %w", err)
	}

	// Apply keybinding overrides from config
	keys.ApplyConfig(
		cfg.UI.Keybindings.Search,
		cfg.UI.Keybindings.Dashboard,
		cfg.UI.Keybindings.Quit,
		cfg.UI.Keybindings.Editor,
	)

	// Working directory is always the current directory (where perles was invoked)
	workDir, err := os.Getwd()
	if err != nil {
		log.Error(log.CatConfig, "Getting current directory failed", "error", err)
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Resolution priority for beads directory:
	// 1. -b flag (explicitly provided on command line)
	// 2. BEADS_DIR environment variable
	// 3. beads_dir config file setting
	// 4. Current working directory
	var beadsPathInput string
	if cmd.Flags().Changed("beads-dir") {
		// -b flag explicitly provided on command line
		beadsPathInput, _ = cmd.Flags().GetString("beads-dir")
	} else if envDir := os.Getenv("BEADS_DIR"); envDir != "" {
		// BEADS_DIR environment variable
		beadsPathInput = envDir
	} else if cfg.BeadsDir != "" {
		// beads_dir from config file
		beadsPathInput = cfg.BeadsDir
	} else {
		// Default to working directory
		beadsPathInput = workDir
	}

	// Resolve full .beads path (handles redirect for worktrees, normalizes input)
	cfg.ResolvedBeadsDir = paths.ResolveBeadsDir(beadsPathInput)
	log.Info(log.CatConfig, "resolved beads dir", "path", cfg.ResolvedBeadsDir)

	client, err := createDoltClient(cfg.ResolvedBeadsDir)
	if err != nil {
		log.Error(log.CatBeads, "Creating Dolt client failed", "error", err, "beadsDir", cfg.ResolvedBeadsDir)
		suggestion := infrabeads.StartupSuggestion(err)
		switch classifyStartupBehavior(err) {
		case startupBehaviorNoBeadsMode:
			return runNoBeadsMode(err.Error(), suggestion)
		case startupBehaviorCompatibilityMode:
			return runOutdatedMode("unknown", err.Error(), suggestion)
		case startupBehaviorReturnError:
			if suggestion != "" {
				return fmt.Errorf("unable to connect to beads runtime: %s: %w", suggestion, err)
			}
			return fmt.Errorf("unable to connect to beads runtime; run 'bd bootstrap' and retry: %w", err)
		default:
			return fmt.Errorf("unexpected startup behavior for beads client error: %w", err)
		}
	}

	if err := client.ValidateBeadsV1Compatibility(); err != nil {
		log.Error(log.CatBeads, "Beads schema/layout compatibility check failed", "error", err)
		return runOutdatedMode("unknown", err.Error(), infrabeads.StartupSuggestion(err))
	}

	// Version check - query bd_version from database metadata table
	currentVersion, err := client.Version()
	if err != nil {
		// Very old database without bd_version metadata - show outdated view
		log.Error(log.CatBeads, "Version check failed", "error", err)
		return runOutdatedMode("unknown", err.Error(), infrabeads.StartupSuggestion(err))
	}

	log.Debug(log.CatBeads, "Beads Database Version", "version", currentVersion, "minRequiredVersion", beads.MinBeadsVersion)
	if err := beads.CheckVersion(currentVersion); err != nil {
		return runOutdatedMode(currentVersion, err.Error(), "Upgrade beads to v1.0.0+ and run 'bd bootstrap', then retry Perles.")
	}

	// Handle --no-auto-refresh flag (negated logic)
	if noAutoRefresh, _ := cmd.Flags().GetBool("no-auto-refresh"); noAutoRefresh {
		cfg.AutoRefresh = false
	}

	// Store the config file path for saving column changes
	configFilePath := viper.ConfigFileUsed()
	if configFilePath == "" {
		// No config file was loaded, default to .perles/config.yaml
		configFilePath = ".perles/config.yaml"
	}

	// Initialize BQL cache managers
	bqlCache := cachemanager.NewInMemoryCacheManager[string, []beads.Issue](
		"bql-cache",
		cachemanager.DefaultExpiration,
		cachemanager.DefaultCleanupInterval,
	)
	depGraphCache := cachemanager.NewInMemoryCacheManager[string, *bql.DependencyGraph](
		"bql-dep-cache",
		cachemanager.DefaultExpiration,
		cachemanager.DefaultCleanupInterval,
	)

	// Pass config to app with database and config paths (debug for log overlay)
	model, err := app.NewWithConfig(
		client,
		cfg,
		bqlCache,
		depGraphCache,
		"",
		configFilePath,
		workDir,
		debug,
		registryService,
	)
	if err != nil {
		log.Error(log.CatConfig, "Application initialization failed", "error", err)
		return fmt.Errorf("initializing application: %w", err)
	}
	p := tea.NewProgram(
		&model,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
		tea.WithReportFocus(),
	)

	finalModel, err := p.Run()

	// Keep shutdown/info noise debug-only, but preserve error visibility for normal runs.
	if err != nil {
		log.Error(log.CatConfig, "Perles shutting down with error", "error", err)
	} else if debug {
		log.Info(log.CatConfig, "Perles shutting down")
	}

	err = cleanupFinalModel(finalModel, err, debug)

	if err != nil {
		return fmt.Errorf("running program: %w", err)
	}
	return nil
}

func createDoltClient(beadsDir string) (*infrabeads.DoltClient, error) {
	interval, configured, err := doltHealthCheckIntervalFromEnv()
	if err != nil {
		return nil, err
	}

	if !configured {
		return newDoltClient(beadsDir)
	}

	return newDoltClient(beadsDir, infrabeads.WithHealthCheckInterval(interval))
}

func doltHealthCheckIntervalFromEnv() (time.Duration, bool, error) {
	raw := strings.TrimSpace(os.Getenv(doltHealthCheckIntervalEnv))
	if raw == "" {
		return 0, false, nil
	}

	interval, err := time.ParseDuration(raw)
	if err != nil {
		return 0, false, fmt.Errorf("invalid %s=%q: expected Go duration (for example 5s, 1m): %w", doltHealthCheckIntervalEnv, raw, err)
	}
	if interval <= 0 {
		return 0, false, fmt.Errorf("invalid %s=%q: duration must be greater than zero", doltHealthCheckIntervalEnv, raw)
	}

	return interval, true, nil
}

func resolveDebugLogPath() string {
	if logPath := os.Getenv("PERLES_LOG"); logPath != "" {
		return logPath
	}

	return log.DefaultDebugLogPath()
}

func runtimeDebugEnabled() bool {
	return os.Getenv("PERLES_DEBUG") != "" || debugFlag
}

func runtimeLogLevel(debug bool) log.Level {
	if debug {
		return log.LevelDebug
	}

	return log.LevelError
}

func initRuntimeLogging(prefix string) (bool, func(), error) {
	debug := runtimeDebugEnabled()
	logPath := resolveDebugLogPath()

	cleanup, err := log.InitWithTeaLog(logPath, prefix)
	if err != nil {
		return false, nil, fmt.Errorf("initializing logging: %w", err)
	}

	log.SetMinLevel(runtimeLogLevel(debug))

	return debug, cleanup, nil
}

func cleanupFinalModel(finalModel tea.Model, runErr error, debug bool) error {
	if finalModel == nil {
		return runErr
	}

	closeErr := closeModel(finalModel, debug)
	if closeErr != nil {
		log.Error(log.CatConfig, "Error during cleanup", "error", closeErr)
		if runErr == nil {
			return closeErr
		}
	}

	return runErr
}

var closeAppModel = func(m *app.Model) error {
	return m.Close()
}

func closeModel(finalModel tea.Model, debug bool) error {
	switch m := finalModel.(type) {
	case app.Model:
		return closeAppModel(&m)
	case *app.Model:
		if m == nil {
			return nil
		}
		return closeAppModel(m)
	case interface{ Close() error }:
		return m.Close()
	default:
		if debug {
			log.Debug(log.CatConfig, "Final model does not implement Close", "type", fmt.Sprintf("%T", finalModel))
		}
		return nil
	}
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion sets the version string (called from main with ldflags)
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

// runNoBeadsMode launches the TUI in "no database" mode, showing a friendly
// empty state view when no .beads directory is found.
func runNoBeadsMode(problem, suggestion string) error {
	model := nobeads.New(problem, suggestion)
	p := tea.NewProgram(
		&model,
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("running program: %w", err)
	}
	return nil
}

// runOutdatedMode launches the TUI showing the version upgrade screen.
func runOutdatedMode(currentVersion, problem, suggestion string) error {
	model := outdated.New(currentVersion, beads.MinBeadsVersion, problem, suggestion)
	p := tea.NewProgram(
		&model,
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("running program: %w", err)
	}
	return nil
}

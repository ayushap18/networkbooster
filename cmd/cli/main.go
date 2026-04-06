package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ayush18/networkbooster/config"
	"github.com/ayush18/networkbooster/core/daemon"
	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/ayush18/networkbooster/core/safety"
	"github.com/ayush18/networkbooster/core/scheduler"
	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/ui/tui"
	"github.com/spf13/cobra"
)

var (
	profileFlag  string
	modeFlag     string
	connsFlag    int
	selfHosted   string
	compactFlag  bool
	adaptiveFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "networkbooster",
	Short: "Network bandwidth booster and speed optimizer",
	Long:  "NetworkBooster continuously saturates your bandwidth using parallel connections to maximize download and upload speeds.",
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the bandwidth booster",
	RunE:  runStart,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current booster status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("NetworkBooster status:")
		fmt.Printf("  Daemon: %s\n", daemon.Status())
		return nil
	},
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show session history",
	RunE:  runHistory,
}

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Start in scheduled mode (uses config schedule entries)",
	RunE:  runSchedule,
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the background daemon service",
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install networkbooster as an OS service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemon.Install(); err != nil {
			return fmt.Errorf("failed to install daemon: %w", err)
		}
		fmt.Println("Daemon installed and started successfully.")
		return nil
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the networkbooster OS service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemon.Uninstall(); err != nil {
			return fmt.Errorf("failed to uninstall daemon: %w", err)
		}
		fmt.Println("Daemon uninstalled successfully.")
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show config file path and current settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadDefault()
		if err != nil {
			return err
		}
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, ".networkbooster", "config.yaml")
		if envPath := os.Getenv("NETWORKBOOSTER_CONFIG"); envPath != "" {
			path = envPath
		}
		fmt.Printf("Config path: %s\n\n", path)
		fmt.Printf("Mode:        %s\n", cfg.Mode)
		fmt.Printf("Profile:     %s\n", cfg.Profile)
		fmt.Printf("Connections: %d\n", cfg.Connections)
		if cfg.SelfHostedURL != "" {
			fmt.Printf("Self-hosted: %s\n", cfg.SelfHostedURL)
		}
		fmt.Printf("\nSafety:\n")
		fmt.Printf("  Max DL:    %.0f Mbps\n", cfg.Safety.MaxDownloadMbps)
		fmt.Printf("  Max UL:    %.0f Mbps\n", cfg.Safety.MaxUploadMbps)
		fmt.Printf("  Data cap:  %.0f GB/day\n", cfg.Safety.DailyDataLimitGB)
		fmt.Printf("  Max CPU:   %.0f%%\n", cfg.Safety.MaxCPUPercent)
		fmt.Printf("  Max Temp:  %.0f C\n", cfg.Safety.MaxTempCelsius)
		if len(cfg.Schedule) > 0 {
			fmt.Printf("\nSchedule:\n")
			for _, s := range cfg.Schedule {
				fmt.Printf("  %s %s-%s [%s]\n",
					strings.Join(s.Days, ","), s.Start, s.End, s.Profile)
			}
		}
		return nil
	},
}

func init() {
	startCmd.Flags().StringVarP(&profileFlag, "profile", "p", "", "Profile: light, medium, full (overrides config)")
	startCmd.Flags().StringVarP(&modeFlag, "mode", "m", "", "Mode: download, upload, bidirectional (overrides config)")
	startCmd.Flags().IntVarP(&connsFlag, "connections", "c", 0, "Number of parallel connections (overrides config)")
	startCmd.Flags().StringVar(&selfHosted, "self-hosted", "", "Self-hosted server URL")
	startCmd.Flags().BoolVar(&compactFlag, "compact", false, "Compact single-line display (for low-power devices)")
	startCmd.Flags().BoolVarP(&adaptiveFlag, "adaptive", "a", false, "Enable adaptive connection scaling")

	daemonCmd.AddCommand(daemonInstallCmd, daemonUninstallCmd)
}

func historyDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "history.db"
	}
	return filepath.Join(home, ".networkbooster", "history.db")
}

func buildEngine(cfg config.Config) (*engine.Engine, engine.Mode) {
	// Apply profile presets
	if p, ok := engine.GetProfile(cfg.Profile); ok {
		if connsFlag == 0 {
			cfg.Connections = p.Connections
		}
	}

	// Parse mode
	var mode engine.Mode
	switch strings.ToLower(cfg.Mode) {
	case "upload":
		mode = engine.ModeUpload
	case "bidirectional", "both":
		mode = engine.ModeBidirectional
	default:
		mode = engine.ModeDownload
	}

	// Build source registry — Speedtest source discovers nearest servers via Ookla
	reg := sources.NewRegistry()
	reg.Register(sources.NewSpeedtestSource())
	if cfg.SelfHostedURL != "" {
		reg.Register(sources.NewSelfHostedSource(cfg.SelfHostedURL))
	}

	eng := engine.New(reg, engine.Options{
		Connections: cfg.Connections,
	})

	return eng, mode
}

func startSafetyMonitor(ctx context.Context, cfg config.Config, eng *engine.Engine) {
	var checks []safety.Check
	if cfg.Safety.MaxDownloadMbps > 0 || cfg.Safety.MaxUploadMbps > 0 {
		checks = append(checks, safety.NewBandwidthCheck(cfg.Safety.MaxDownloadMbps, cfg.Safety.MaxUploadMbps))
	}
	if cfg.Safety.DailyDataLimitGB > 0 {
		limitBytes := int64(cfg.Safety.DailyDataLimitGB * 1024 * 1024 * 1024)
		checks = append(checks, safety.NewDataLimitCheck(limitBytes))
	}
	if cfg.Safety.MaxCPUPercent > 0 {
		checks = append(checks, safety.NewCPUCheck(cfg.Safety.MaxCPUPercent))
	}
	if cfg.Safety.MaxTempCelsius > 0 {
		checks = append(checks, safety.NewTemperatureCheck(cfg.Safety.MaxTempCelsius))
	}

	monitor := safety.NewMonitor(checks)
	go monitor.RunLoop(ctx, eng.Collector(), func(result safety.CheckResult) {
		switch result.Action {
		case safety.ActionPause:
			log.Printf("[safety] PAUSE: %s", result.Reason)
			eng.Pause()
		case safety.ActionThrottle:
			log.Printf("[safety] THROTTLE to %d conns: %s", result.Target, result.Reason)
		}
	})
}

func saveSession(cfg config.Config, startTime time.Time, status engine.Status) {
	dbPath := historyDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err == nil {
		hist, err := metrics.NewHistory(dbPath)
		if err == nil {
			defer hist.Close()
			hist.SaveSession(metrics.Session{
				StartTime:     startTime,
				EndTime:       time.Now(),
				Mode:          cfg.Mode,
				Profile:       cfg.Profile,
				Connections:   cfg.Connections,
				TotalDownload: status.Snapshot.TotalDownloadBytes,
				TotalUpload:   status.Snapshot.TotalUploadBytes,
				PeakDownload:  status.Snapshot.PeakDownloadMbps,
				PeakUpload:    status.Snapshot.PeakUploadMbps,
				AvgDownload:   status.Snapshot.AvgDownloadMbps,
				AvgUpload:     status.Snapshot.AvgUploadMbps,
			})
		}
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadDefault()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply flag overrides
	if profileFlag != "" {
		cfg.Profile = profileFlag
	}
	if modeFlag != "" {
		cfg.Mode = modeFlag
	}
	if connsFlag > 0 {
		cfg.Connections = connsFlag
	}
	if selfHosted != "" {
		cfg.SelfHostedURL = selfHosted
	}

	eng, mode := buildEngine(cfg)

	fmt.Printf("NetworkBooster starting...\n")
	fmt.Printf("  Mode: %s | Connections: %d | Profile: %s\n", cfg.Mode, cfg.Connections, cfg.Profile)

	startTime := time.Now()

	if err := eng.Start(mode); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}

	safetyCtx, safetyCancel := context.WithCancel(context.Background())
	startSafetyMonitor(safetyCtx, cfg, eng)

	var scalerCancel context.CancelFunc
	if cfg.Adaptive.Enabled || adaptiveFlag {
		scalerCtx, sc := context.WithCancel(context.Background())
		scalerCancel = sc

		maxConns := cfg.Adaptive.MaxConnections
		if maxConns == 0 && cfg.Safety.MaxConnections > 0 {
			maxConns = cfg.Safety.MaxConnections
		}
		if maxConns == 0 {
			maxConns = 64
		}

		scaler := engine.NewScaler(engine.ScalerOptions{
			MinConnections: cfg.Adaptive.MinConnections,
			MaxConnections: maxConns,
			Interval:       time.Duration(cfg.Adaptive.IntervalSecs) * time.Second,
		}, eng)
		go scaler.RunLoop(scalerCtx, eng.Collector())
	}

	// Run TUI (blocks until user quits)
	tuiErr := tui.Run(eng, compactFlag)

	if scalerCancel != nil {
		scalerCancel()
	}
	safetyCancel()
	eng.Stop()
	status := eng.Status()

	// Print session summary
	fmt.Printf("\nSession summary:\n")
	fmt.Printf("  Downloaded: %.2f MB\n", float64(status.Snapshot.TotalDownloadBytes)/(1024*1024))
	fmt.Printf("  Uploaded:   %.2f MB\n", float64(status.Snapshot.TotalUploadBytes)/(1024*1024))
	fmt.Printf("  Peak DL:    %.1f Mbps\n", status.Snapshot.PeakDownloadMbps)
	fmt.Printf("  Peak UL:    %.1f Mbps\n", status.Snapshot.PeakUploadMbps)
	fmt.Printf("  Avg DL:     %.1f Mbps\n", status.Snapshot.AvgDownloadMbps)
	fmt.Printf("  Avg UL:     %.1f Mbps\n", status.Snapshot.AvgUploadMbps)
	fmt.Printf("  Duration:   %s\n", status.Snapshot.Elapsed.Round(time.Second))

	saveSession(cfg, startTime, status)
	return tuiErr
}

func runSchedule(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadDefault()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Schedule) == 0 {
		return fmt.Errorf("no schedule entries in config. Add schedule entries to ~/.networkbooster/config.yaml")
	}

	// Convert config schedule entries to scheduler entries
	var entries []scheduler.ScheduleEntry
	for _, e := range cfg.Schedule {
		entries = append(entries, scheduler.ScheduleEntry{
			Days:    scheduler.ParseDays(e.Days),
			Start:   e.Start,
			End:     e.End,
			Profile: e.Profile,
		})
	}

	sched := scheduler.NewScheduler(entries)

	fmt.Println("NetworkBooster running in scheduled mode.")
	fmt.Printf("  %d schedule entries configured.\n", len(entries))
	fmt.Println("  Press Ctrl+C to stop.")

	// Build engine (will be started/stopped by scheduler)
	reg := sources.NewRegistry()
	reg.Register(sources.NewCDNSource())
	if cfg.SelfHostedURL != "" {
		reg.Register(sources.NewSelfHostedSource(cfg.SelfHostedURL))
	}

	var eng *engine.Engine
	var safetyCancel context.CancelFunc
	var startTime time.Time

	ctx, cancel := context.WithCancel(context.Background())

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nStopping scheduler...")
		cancel()
	}()

	sched.RunLoop(ctx,
		func(profile string) {
			// On schedule start
			p, ok := engine.GetProfile(profile)
			if !ok {
				p = engine.Profile{Name: profile, Connections: 8}
			}
			cfg.Profile = profile
			cfg.Connections = p.Connections

			eng = engine.New(reg, engine.Options{Connections: p.Connections})
			startTime = time.Now()

			fmt.Printf("\n  [%s] Schedule active — profile: %s, connections: %d\n",
				time.Now().Format("15:04:05"), profile, p.Connections)

			if err := eng.Start(engine.ModeDownload); err != nil {
				log.Printf("[schedule] failed to start engine: %v", err)
				return
			}

			var safetyCtx context.Context
			safetyCtx, safetyCancel = context.WithCancel(context.Background())
			startSafetyMonitor(safetyCtx, cfg, eng)
		},
		func() {
			// On schedule stop
			if safetyCancel != nil {
				safetyCancel()
			}
			if eng != nil {
				eng.Stop()
				status := eng.Status()
				fmt.Printf("  [%s] Schedule window ended — downloaded %.1f MB\n",
					time.Now().Format("15:04:05"),
					float64(status.Snapshot.TotalDownloadBytes)/(1024*1024))
				saveSession(cfg, startTime, status)
			}
		},
	)

	return nil
}

func runHistory(cmd *cobra.Command, args []string) error {
	dbPath := historyDBPath()
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	hist, err := metrics.NewHistory(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open history: %w", err)
	}
	defer hist.Close()

	sessions, err := hist.ListSessions(20)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions recorded yet. Run 'networkbooster start' first.")
		return nil
	}

	fmt.Printf("%-20s  %-10s  %-8s  %10s  %10s  %10s  %10s\n",
		"Date", "Mode", "Profile", "Download", "Upload", "Peak DL", "Peak UL")
	fmt.Println(strings.Repeat("-", 90))

	for _, s := range sessions {
		fmt.Printf("%-20s  %-10s  %-8s  %8.1f MB  %8.1f MB  %7.1f Mbps  %7.1f Mbps\n",
			s.StartTime.Format("2006-01-02 15:04:05"),
			s.Mode,
			s.Profile,
			float64(s.TotalDownload)/(1024*1024),
			float64(s.TotalUpload)/(1024*1024),
			s.PeakDownload,
			s.PeakUpload,
		)
	}

	stats, err := hist.TotalStats()
	if err == nil {
		fmt.Println(strings.Repeat("-", 90))
		fmt.Printf("Total: %d sessions | Downloaded: %.1f GB | Uploaded: %.1f GB\n",
			stats.SessionCount,
			float64(stats.TotalDownload)/(1024*1024*1024),
			float64(stats.TotalUpload)/(1024*1024*1024),
		)
	}

	return nil
}

func main() {
	rootCmd.AddCommand(startCmd, statusCmd, historyCmd, scheduleCmd, daemonCmd, configCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

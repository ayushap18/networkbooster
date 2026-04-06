package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ayush18/networkbooster/config"
	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/ui/tui"
	"github.com/spf13/cobra"
)

var (
	profileFlag string
	modeFlag    string
	connsFlag   int
	selfHosted  string
	compactFlag bool
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
		fmt.Println("NetworkBooster is not running. Use 'networkbooster start' to begin.")
		return nil
	},
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show session history",
	RunE:  runHistory,
}

func init() {
	startCmd.Flags().StringVarP(&profileFlag, "profile", "p", "", "Profile: light, medium, full (overrides config)")
	startCmd.Flags().StringVarP(&modeFlag, "mode", "m", "", "Mode: download, upload, bidirectional (overrides config)")
	startCmd.Flags().IntVarP(&connsFlag, "connections", "c", 0, "Number of parallel connections (overrides config)")
	startCmd.Flags().StringVar(&selfHosted, "self-hosted", "", "Self-hosted server URL")
	startCmd.Flags().BoolVar(&compactFlag, "compact", false, "Compact single-line display (for low-power devices)")
}

func historyDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "history.db"
	}
	return filepath.Join(home, ".networkbooster", "history.db")
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

	// Apply profile presets
	switch cfg.Profile {
	case "light":
		if connsFlag == 0 {
			cfg.Connections = 4
		}
	case "full":
		if connsFlag == 0 {
			cfg.Connections = 64
		}
	case "medium":
		if connsFlag == 0 {
			cfg.Connections = 16
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

	// Build source registry
	reg := sources.NewRegistry()
	reg.Register(sources.NewCDNSource())
	if cfg.SelfHostedURL != "" {
		reg.Register(sources.NewSelfHostedSource(cfg.SelfHostedURL))
	}

	// Create and start engine
	eng := engine.New(reg, engine.Options{
		Connections: cfg.Connections,
	})

	fmt.Printf("NetworkBooster starting...\n")
	fmt.Printf("  Mode: %s | Connections: %d | Profile: %s\n", cfg.Mode, cfg.Connections, cfg.Profile)

	startTime := time.Now()

	if err := eng.Start(mode); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}

	// Run TUI (blocks until user quits)
	tuiErr := tui.Run(eng, compactFlag)

	// Stop engine after TUI exits
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

	// Save session to history
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

	return tuiErr
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

	// Show totals
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
	rootCmd.AddCommand(startCmd, statusCmd, historyCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

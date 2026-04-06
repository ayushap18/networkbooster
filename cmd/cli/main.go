package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ayush18/networkbooster/config"
	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/sources"
	"github.com/spf13/cobra"
)

var (
	profileFlag string
	modeFlag    string
	connsFlag   int
	selfHosted  string
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

func init() {
	startCmd.Flags().StringVarP(&profileFlag, "profile", "p", "", "Profile: light, medium, full (overrides config)")
	startCmd.Flags().StringVarP(&modeFlag, "mode", "m", "", "Mode: download, upload, bidirectional (overrides config)")
	startCmd.Flags().IntVarP(&connsFlag, "connections", "c", 0, "Number of parallel connections (overrides config)")
	startCmd.Flags().StringVar(&selfHosted, "self-hosted", "", "Self-hosted server URL")
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

	if err := eng.Start(mode); err != nil {
		return fmt.Errorf("failed to start engine: %w", err)
	}

	fmt.Println("  Running! Press Ctrl+C to stop.")

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Print live stats until interrupted
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Println("\nStopping...")
			eng.Stop()
			status := eng.Status()
			fmt.Printf("\nSession summary:\n")
			fmt.Printf("  Downloaded: %.2f MB\n", float64(status.Snapshot.TotalDownloadBytes)/(1024*1024))
			fmt.Printf("  Uploaded:   %.2f MB\n", float64(status.Snapshot.TotalUploadBytes)/(1024*1024))
			fmt.Printf("  Duration:   %s\n", status.Snapshot.Elapsed.Round(time.Second))
			return nil
		case <-ticker.C:
			status := eng.Status()
			s := status.Snapshot
			fmt.Printf("\r  ↓ %.1f Mbps  ↑ %.1f Mbps  | %d conns | ↓ %.1f MB  ↑ %.1f MB",
				s.DownloadMbps,
				s.UploadMbps,
				s.ActiveConnections,
				float64(s.TotalDownloadBytes)/(1024*1024),
				float64(s.TotalUploadBytes)/(1024*1024),
			)
		}
	}
}

func main() {
	rootCmd.AddCommand(startCmd, statusCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

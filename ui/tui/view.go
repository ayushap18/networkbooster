package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/metrics"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	status := m.engine.Status()
	s := status.Snapshot

	if m.compact {
		return m.compactView(s)
	}
	return m.fullView(status)
}

func (m Model) compactView(s metrics.Snapshot) string {
	return fmt.Sprintf("  DOWN %.1f Mbps  UP %.1f Mbps  | %d conns | DOWN %.1f MB  UP %.1f MB | %s",
		s.DownloadMbps, s.UploadMbps, s.ActiveConnections,
		float64(s.TotalDownloadBytes)/(1024*1024),
		float64(s.TotalUploadBytes)/(1024*1024),
		s.Elapsed.Round(time.Second),
	)
}

func (m Model) fullView(status engine.Status) string {
	s := status.Snapshot
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("NetworkBooster"))
	b.WriteString("\n\n")

	// Speed gauges
	dlSpeed := fmt.Sprintf("%.1f Mbps", s.DownloadMbps)
	ulSpeed := fmt.Sprintf("%.1f Mbps", s.UploadMbps)

	peakDl := s.PeakDownloadMbps
	if peakDl <= 0 {
		peakDl = 1
	}
	peakUl := s.PeakUploadMbps
	if peakUl <= 0 {
		peakUl = 1
	}

	dlLine := fmt.Sprintf("  %s %s  %s",
		labelStyle.Render("Download:"),
		speedStyle.Render(dlSpeed),
		renderBar(s.DownloadMbps, peakDl, 30))
	ulLine := fmt.Sprintf("  %s %s  %s",
		labelStyle.Render("Upload:  "),
		uploadSpeedStyle.Render(ulSpeed),
		renderBar(s.UploadMbps, peakUl, 30))

	b.WriteString(dlLine + "\n")
	b.WriteString(ulLine + "\n\n")

	// Stats box
	statsContent := fmt.Sprintf(
		"%s %s    %s %s    %s %s\n%s %s    %s %s    %s %s",
		labelStyle.Render("Peak DL:"), valueStyle.Render(fmt.Sprintf("%.1f Mbps", s.PeakDownloadMbps)),
		labelStyle.Render("Avg DL:"), valueStyle.Render(fmt.Sprintf("%.1f Mbps", s.AvgDownloadMbps)),
		labelStyle.Render("Total DL:"), valueStyle.Render(formatBytes(s.TotalDownloadBytes)),
		labelStyle.Render("Peak UL:"), valueStyle.Render(fmt.Sprintf("%.1f Mbps", s.PeakUploadMbps)),
		labelStyle.Render("Avg UL:"), valueStyle.Render(fmt.Sprintf("%.1f Mbps", s.AvgUploadMbps)),
		labelStyle.Render("Total UL:"), valueStyle.Render(formatBytes(s.TotalUploadBytes)),
	)
	b.WriteString(boxStyle.Render(statsContent))
	b.WriteString("\n\n")

	// Connection info
	b.WriteString(fmt.Sprintf("  %s %s    %s %s    %s %s\n",
		labelStyle.Render("Connections:"), valueStyle.Render(fmt.Sprintf("%d", s.ActiveConnections)),
		labelStyle.Render("Elapsed:"), valueStyle.Render(s.Elapsed.Round(time.Second).String()),
		labelStyle.Render("Status:"), statusRunning.Render("Running"),
	))

	// Per-server table
	if len(s.ServerStats) > 0 {
		b.WriteString("\n")
		b.WriteString(serverHeaderStyle.Render("  Servers"))
		b.WriteString("\n")
		for id, stat := range s.ServerStats {
			b.WriteString(fmt.Sprintf("    %s  DL %s  UL %s\n",
				labelStyle.Render(truncate(id, 20)),
				valueStyle.Render(formatBytes(stat.DownloadBytes)),
				valueStyle.Render(formatBytes(stat.UploadBytes)),
			))
		}
	}

	// Help
	b.WriteString(helpStyle.Render("  Press q or Ctrl+C to stop"))

	return b.String()
}

func renderBar(current, max float64, width int) string {
	if max <= 0 {
		max = 1
	}
	ratio := current / max
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	empty := width - filled
	return barFullStyle.Render(strings.Repeat("|", filled)) +
		barEmptyStyle.Render(strings.Repeat("-", empty))
}

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

package metrics

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Session struct {
	ID            int64
	StartTime     time.Time
	EndTime       time.Time
	Mode          string
	Profile       string
	Connections   int
	TotalDownload int64
	TotalUpload   int64
	PeakDownload  float64
	PeakUpload    float64
	AvgDownload   float64
	AvgUpload     float64
}

type TotalStats struct {
	TotalDownload int64
	TotalUpload   int64
	SessionCount  int
}

type History struct {
	db *sql.DB
}

func NewHistory(dbPath string) (*History, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		start_time DATETIME,
		end_time DATETIME,
		mode TEXT,
		profile TEXT,
		connections INTEGER,
		total_download INTEGER,
		total_upload INTEGER,
		peak_download REAL,
		peak_upload REAL,
		avg_download REAL,
		avg_upload REAL
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &History{db: db}, nil
}

func (h *History) Close() error { return h.db.Close() }

func (h *History) SaveSession(s Session) error {
	_, err := h.db.Exec(
		`INSERT INTO sessions (start_time, end_time, mode, profile, connections,
		total_download, total_upload, peak_download, peak_upload, avg_download, avg_upload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.StartTime, s.EndTime, s.Mode, s.Profile, s.Connections,
		s.TotalDownload, s.TotalUpload, s.PeakDownload, s.PeakUpload,
		s.AvgDownload, s.AvgUpload,
	)
	return err
}

func (h *History) ListSessions(limit int) ([]Session, error) {
	rows, err := h.db.Query(
		`SELECT id, start_time, end_time, mode, profile, connections,
		total_download, total_upload, peak_download, peak_upload, avg_download, avg_upload
		FROM sessions ORDER BY start_time DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var s Session
		err := rows.Scan(&s.ID, &s.StartTime, &s.EndTime, &s.Mode, &s.Profile,
			&s.Connections, &s.TotalDownload, &s.TotalUpload, &s.PeakDownload,
			&s.PeakUpload, &s.AvgDownload, &s.AvgUpload)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (h *History) TotalStats() (TotalStats, error) {
	var stats TotalStats
	err := h.db.QueryRow(
		`SELECT COALESCE(SUM(total_download),0), COALESCE(SUM(total_upload),0), COUNT(*)
		FROM sessions`).Scan(&stats.TotalDownload, &stats.TotalUpload, &stats.SessionCount)
	return stats, err
}

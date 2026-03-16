package cleanup

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
)

type Config struct {
	OutputDir     string
	AudioDir      string
	RetentionDays int
}

type Cleanup struct {
	config Config
	cron   *cron.Cron
}

func New(config Config) *Cleanup {
	return &Cleanup{
		config: config,
	}
}

func (c *Cleanup) Start() error {
	c.cron = cron.New()

	// Run cleanup daily at 3 AM
	_, err := c.cron.AddFunc("0 3 * * *", func() {
		removed, err := c.CleanOldFiles()
		if err != nil {
			log.Printf("Cleanup error: %v", err)
		} else if removed > 0 {
			log.Printf("Cleanup removed %d old files", removed)
		}
	})
	if err != nil {
		return err
	}

	c.cron.Start()
	log.Println("Cleanup scheduler started")
	return nil
}

func (c *Cleanup) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

func (c *Cleanup) CleanOldFiles() (int, error) {
	cutoff := time.Now().Add(-time.Duration(c.config.RetentionDays) * 24 * time.Hour)
	removed := 0

	// Clean output directory
	if c.config.OutputDir != "" {
		n, err := c.cleanDirectory(c.config.OutputDir, cutoff)
		if err != nil {
			return removed, err
		}
		removed += n
	}

	// Clean audio directory (for failed jobs)
	if c.config.AudioDir != "" {
		// Audio files for failed jobs older than 1 hour
		audioCutoff := time.Now().Add(-1 * time.Hour)
		n, err := c.cleanDirectory(c.config.AudioDir, audioCutoff)
		if err != nil {
			return removed, err
		}
		removed += n
	}

	return removed, nil
}

func (c *Cleanup) cleanDirectory(dir string, cutoff time.Time) (int, error) {
	removed := 0

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("Failed to remove %s: %v", path, err)
			} else {
				removed++
			}
		}
	}

	return removed, nil
}

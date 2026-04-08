package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-proxy/logging"
)

// FlushToDisk writes the given log entries to individual JSON files in the target directory.
// Files are organized by date in subdirectories, matching the Storage.Write format.
//
// @param entries - Log entries to write.
// @param targetDir - Root directory for output files.
// @return count - Number of entries successfully written.
// @return error - First error encountered, or nil on success.
func FlushToDisk(entries []LogEntry, targetDir string) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}

	// Create date-based subdirectory
	dir := filepath.Join(targetDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, dirPerms); err != nil {
		return 0, fmt.Errorf("create log dir: %w", err)
	}

	written := 0
	var firstErr error

	for _, entry := range entries {
		filename := fmt.Sprintf("%s_%s.json", time.Now().Format("20060102-150405"), sanitizeFilename(entry.RequestID))
		fullpath := filepath.Join(dir, filename)

		// Handle collision with suffix
		if _, err := os.Stat(fullpath); err == nil {
			filename = fmt.Sprintf("%s_%s_1.json", time.Now().Format("20060102-150405"), sanitizeFilename(entry.RequestID))
			fullpath = filepath.Join(dir, filename)
		}

		file, err := os.OpenFile(fullpath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, filePerms)
		if err != nil {
			logging.ErrorMsg("Flush: failed to create file %s: %v", fullpath, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("create log file: %w", err)
			}
			continue
		}

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(entry); err != nil {
			logging.ErrorMsg("Flush: failed to encode entry %s: %v", entry.RequestID, err)
			file.Close()
			if firstErr == nil {
				firstErr = fmt.Errorf("encode log data: %w", err)
			}
			continue
		}
		file.Close()
		written++
	}

	logging.InfoMsg("Flushed %d/%d entries to %s", written, len(entries), dir)
	return written, firstErr
}

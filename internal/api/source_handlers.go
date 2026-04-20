package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
)

// handleSourceInfo returns metadata about the cached dlc.dat.
func (s *Server) handleSourceInfo(w http.ResponseWriter, r *http.Request) {
	if s.sourceStore == nil {
		s.jsonOK(w, map[string]interface{}{
			"available": false,
		})
		return
	}

	info := s.sourceStore.Info()

	resp := map[string]interface{}{
		"available":     true,
		"fetched_at":    info.FetchedAt,
		"size":          info.Size,
		"etag":          info.ETag,
		"last_modified": info.LastModified,
	}

	if info.SHA256 != "" {
		resp["sha256"] = info.SHA256
	} else if info.Size > 0 {
		snapshot := s.cfgWatcher.Current()
		cachePath := snapshot.Config.Source.CachePath

		data, err := os.ReadFile(cachePath)
		if err == nil {
			hash := sha256.Sum256(data)
			resp["sha256"] = hex.EncodeToString(hash[:])
		}
	}

	s.jsonOK(w, resp)
}

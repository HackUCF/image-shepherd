package shepherd

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"github.com/s-newman/image-shepherd/pkg/image"
	"go.uber.org/zap"
)

func Run(c *gophercloud.ServiceClient, imagesCfg []image.Image) {
	// Fetch existing images once
	zap.S().Infow("Fetching existing images", "phase", "list", "action", "start")
	// Apply network timeouts
	clientTimeout := 60 * time.Second
	c.HTTPClient.Timeout = clientTimeout
	zap.S().Infow("Applied HTTP client timeout", "timeout_seconds", clientTimeout.Seconds())
	ctxList, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pages, err := images.List(c, images.ListOpts{}).AllPages(ctxList)
	if err != nil {
		zap.S().Errorw("Failed to list existing images", "error", err)
		return
	}
	existing, err := images.ExtractImages(pages)
	if err != nil {
		zap.S().Errorw("Failed to parse existing images", "error", err)
		return
	}
	zap.S().Infow("Fetched existing images", "count", len(existing))

	ownerFilter := strings.TrimSpace(os.Getenv("IMAGE_SHEPHERD_OWNER_PROJECT_ID"))
	requireProtectedEnv := strings.TrimSpace(os.Getenv("IMAGE_SHEPHERD_REQUIRE_PROTECTED"))
	requirePublicEnv := strings.TrimSpace(os.Getenv("IMAGE_SHEPHERD_REQUIRE_PUBLIC"))
	requireProtected := strings.EqualFold(requireProtectedEnv, "true") || requireProtectedEnv == "1" || strings.EqualFold(requireProtectedEnv, "yes")
	requirePublic := strings.EqualFold(requirePublicEnv, "true") || requirePublicEnv == "1" || strings.EqualFold(requirePublicEnv, "yes")
	if ownerFilter != "" || requireProtected || requirePublic {
		zap.S().Infow("Applying matching constraints", "owner_project_id", ownerFilter, "require_protected", requireProtected, "require_public", requirePublic)
	} else {
		zap.S().Infow("No matching constraints configured (owner/protected/public)")
	}

	for _, imgCfg := range imagesCfg {
		zap.S().Infow("Managing image", "name", imgCfg.Name, "total_existing_images", len(existing))

		imgCfg.Init()

		// Get upstream metadata to determine if a new image was published
		meta, metaErr := image.FetchSourceMeta(imgCfg.Url)
		if metaErr != nil {
			zap.S().Warnw("Could not fetch source metadata; proceeding", "url", imgCfg.Url, "image", imgCfg.Name, "error", metaErr)
		}

		// Find current "latest" image matching either properties or name (non-hidden)
		var current *images.Image
		wantDistro, hasDistro := imgCfg.Properties["os_distro"]
		wantVersion, hasVersion := imgCfg.Properties["os_version"]
		wantType, hasType := imgCfg.Properties["os_type"]

		if hasDistro && hasVersion && hasType && wantDistro != "" && wantVersion != "" && wantType != "" {
			zap.S().Infow("Matching strategy: properties", "os_distro", wantDistro, "os_version", wantVersion, "os_type", wantType)
		} else {
			zap.S().Infow("Matching strategy: name", "name", imgCfg.Name)
		}
		for idx := range existing {
			ex := &existing[idx]
			if ex.Hidden {
				continue
			}
			match := false
			if hasDistro && hasVersion && hasType &&
				wantDistro != "" && wantVersion != "" && wantType != "" {
				gd, _ := ex.Properties["os_distro"].(string)
				gv, _ := ex.Properties["os_version"].(string)
				gt, _ := ex.Properties["os_type"].(string)
				match = (gd == wantDistro && gv == wantVersion && gt == wantType)
			} else {
				match = (ex.Name == imgCfg.Name)
			}
			if match {
				if ownerFilter != "" && ex.Owner != ownerFilter {
					zap.S().Debugw("Skipping candidate due to owner mismatch", "id", ex.ID, "owner", ex.Owner, "expected_owner", ownerFilter)
					continue
				}
				if requireProtected && !ex.Protected {
					zap.S().Debugw("Skipping candidate due to protection mismatch", "id", ex.ID, "protected", ex.Protected)
					continue
				}
				if requirePublic && ex.Visibility != images.ImageVisibilityPublic {
					zap.S().Debugw("Skipping candidate due to visibility mismatch", "id", ex.ID, "visibility", ex.Visibility)
					continue
				}
				current = ex
				break
			}
		}

		// Decide if the source is newer than what we already have
		unchanged := false
		reason := ""
		if current != nil {
			zap.S().Infow("Found current image candidate", "id", current.ID, "name", current.Name)
			if meta.ETag != "" {
				if et, ok := current.Properties["source_etag"].(string); ok && et != "" && et == meta.ETag {
					unchanged = true
					reason = "etag"
				}
			}
			if !unchanged && meta.LastModified != "" {
				if lm, ok := current.Properties["source_last_modified"].(string); ok && lm != "" && lm == meta.LastModified {
					unchanged = true
					reason = "last_modified"
				}
			}
		} else {
			zap.S().Infow("No current image found; will upload", "name", imgCfg.Name)
		}

		if unchanged {
			zap.S().Infow("Image unchanged; skipping upload", "name", imgCfg.Name, "reason", reason, "source_etag", meta.ETag, "source_last_modified", meta.LastModified)
			continue
		}

		if err := imgCfg.Upload(c, meta); err != nil {
			zap.S().Errorw("Upload failed", "name", imgCfg.Name, "error", err)
		} else {
			zap.S().Infow("Upload complete", "name", imgCfg.Name)
			if current != nil {
				zap.S().Infow("Renaming/hiding previous image", "previous_id", current.ID, "previous_name", current.Name)
				if err := image.RenameHideByID(c, current.ID); err != nil {
					zap.S().Errorw("Failed to rename/hide previous image", "id", current.ID, "error", err)
				} else {
					zap.S().Infow("Previous image renamed/hidden", "id", current.ID)
				}
			} else {
				zap.S().Infow("No previous image to rename/hide")
			}
		}
	}
}

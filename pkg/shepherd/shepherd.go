package shepherd

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/HackUCF/image-shepherd/pkg/image"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
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

		if imgCfg.Discover != nil {
			if u, err := imgCfg.Discover.Resolve(); err != nil {
				if imgCfg.Url == "" {
					zap.S().Errorw("URL discovery failed and no fallback url; skipping", "image", imgCfg.Name, "index", imgCfg.Discover.Index, "error", err)
					continue
				}
				zap.S().Warnw("URL discovery failed; using pinned url", "image", imgCfg.Name, "index", imgCfg.Discover.Index, "url", imgCfg.Url, "error", err)
			} else {
				zap.S().Infow("Discovered newest build", "image", imgCfg.Name, "url", u)
				imgCfg.Url = u
			}
		}

		// Get upstream metadata to determine if a new image was published
		meta, metaErr := image.FetchSourceMeta(imgCfg.Url)
		if metaErr != nil {
			zap.S().Warnw("Could not fetch source metadata; proceeding", "url", imgCfg.Url, "image", imgCfg.Name, "error", metaErr)
		}

		// Collect every non-hidden image this config manages. The newest *active* one is
		// "current"; everything else is a duplicate (an older copy, or a queued/killed
		// leftover from a failed upload) and gets renamed/hidden.
		var allImages []*images.Image
		var current *images.Image

		wantDistro, hasDistro := imgCfg.Properties["os_distro"]
		wantVersion, hasVersion := imgCfg.Properties["os_version"]
		wantType, hasType := imgCfg.Properties["os_type"]
		byProps := hasDistro && hasVersion && hasType && wantDistro != "" && wantVersion != "" && wantType != ""

		if byProps {
			zap.S().Infow("Matching strategy: name or shepherd-managed properties", "name", imgCfg.Name, "os_distro", wantDistro, "os_version", wantVersion, "os_type", wantType)
		} else {
			zap.S().Infow("Matching strategy: name", "name", imgCfg.Name)
		}
		for idx := range existing {
			ex := &existing[idx]
			if ex.Hidden {
				continue
			}

			// Explicitly exclude snapshots and backups to avoid managing user artifacts
			if imgType, ok := ex.Properties["image_type"].(string); ok {
				if strings.EqualFold(imgType, "snapshot") || strings.EqualFold(imgType, "backup") {
					zap.S().Debugw("Skipping candidate identified as snapshot/backup", "id", ex.ID, "image_type", imgType)
					continue
				}
			}
			if bdm, ok := ex.Properties["block_device_mapping"].(string); ok {
				if strings.Contains(bdm, `"source_type": "snapshot"`) || strings.Contains(bdm, `"source_type": "backup"`) {
					zap.S().Debugw("Skipping candidate identified as snapshot/backup via block_device_mapping", "id", ex.ID)
					continue
				}
			}

			// Name match catches images whose properties changed in images.yaml since upload.
			// Property match only counts for images shepherd uploaded (source_url set), so
			// user uploads that happen to carry the same os_* properties are left alone.
			match := ex.Name == imgCfg.Name
			if !match && byProps {
				gd, _ := ex.Properties["os_distro"].(string)
				gv, _ := ex.Properties["os_version"].(string)
				gt, _ := ex.Properties["os_type"].(string)
				su, _ := ex.Properties["source_url"].(string)
				match = gd == wantDistro && gv == wantVersion && gt == wantType && su != ""
			}
			if !match {
				continue
			}
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

			allImages = append(allImages, ex)
			// Only an active image can be current; a queued/killed one from a failed upload
			// carries the same source_etag and would otherwise mask the need to re-upload.
			if ex.Status == images.ImageStatusActive && (current == nil || ex.CreatedAt.After(current.CreatedAt)) {
				current = ex
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
			for _, dup := range allImages {
				if dup.ID == current.ID {
					continue
				}
				zap.S().Warnw("Hiding duplicate image", "name", imgCfg.Name, "id", dup.ID, "status", dup.Status, "kept_id", current.ID)
				if err := image.RenameHideByID(c, dup.ID); err != nil {
					zap.S().Errorw("Failed to rename/hide duplicate image", "id", dup.ID, "error", err)
				}
			}
			continue
		}

		if err := imgCfg.Upload(c, meta); err != nil {
			zap.S().Errorw("Upload failed", "name", imgCfg.Name, "error", err)
			if strings.Contains(err.Error(), "no space left on device") {
				zap.S().Fatal("Exiting due to no space left on device")
			}
		} else {
			zap.S().Infow("Upload complete", "name", imgCfg.Name)
			if len(allImages) > 0 {
				for _, old := range allImages {
					zap.S().Infow("Renaming/hiding previous image", "previous_id", old.ID, "previous_name", old.Name)
					if err := image.RenameHideByID(c, old.ID); err != nil {
						zap.S().Errorw("Failed to rename/hide previous image", "id", old.ID, "error", err)
					} else {
						zap.S().Infow("Previous image renamed/hidden", "id", old.ID)
					}
				}
			} else {
				zap.S().Infow("No previous image to rename/hide")
			}
		}
	}
}

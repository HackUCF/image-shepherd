package image

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/imagedata"
	"github.com/gophercloud/gophercloud/v2/openstack/image/v2/images"
	"go.uber.org/zap"
)

type SourceMeta struct {
	// ETag is the entity tag returned by the source server, if any.
	ETag string
	// LastModified is the HTTP Last-Modified header value, if any.
	LastModified string
	// ContentLength is the size in bytes reported by the server, if any.
	ContentLength int64
}

// FetchSourceMeta issues an HTTP HEAD request to retrieve metadata about the source URL
// without downloading the content. It returns ETag, Last-Modified, and Content-Length
// when available.
func FetchSourceMeta(srcURL string) (SourceMeta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "HEAD", srcURL, nil)
	if err != nil {
		return SourceMeta{}, err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return SourceMeta{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return SourceMeta{}, fmt.Errorf("HEAD %s failed: %s", srcURL, resp.Status)
	}

	var cl int64
	if v := resp.Header.Get("Content-Length"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			cl = parsed
		}
	}

	return SourceMeta{
		ETag:          resp.Header.Get("ETag"),
		LastModified:  resp.Header.Get("Last-Modified"),
		ContentLength: cl,
	}, nil
}

const uploadedFmt = "02-Jan-2006"

type Image struct {
	Name         string
	Url          string
	Public       bool
	Protected    bool `yaml:"protected,omitempty"`
	Tags         []string
	Properties   map[string]string
	SourceFormat string `yaml:"source_format,omitempty"`
	Compression  string `yaml:"compression,omitempty"`
}

func setDefault(properties *map[string]string, key string, value string) {
	if _, exists := (*properties)[key]; !exists {
		(*properties)[key] = value
	}
}

func (i Image) Init() {
	setDefault(&i.Properties, "architecture", "x86_64")
	setDefault(&i.Properties, "hypervisor_type", "qemu")
	setDefault(&i.Properties, "vm_mode", "hvm")
	setDefault(&i.Properties, "uploaded", time.Now().Format(uploadedFmt))
	setDefault(&i.Properties, "image_family", i.Name)
}

// downloadToCwd downloads the URL to the current working directory and returns the filename.
func downloadToCwd(srcURL string) (string, error) {
	// Resolve configurable timeout (seconds) from environment, default 300s
	timeoutSecs := 300
	if v := os.Getenv("IMAGE_SHEPHERD_DOWNLOAD_TIMEOUT_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSecs = n
		}
	}

	// HTTP client with timeout (per-attempt)
	client := &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}

	maxAttempts := 3
	backoff := 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Context deadline for this attempt
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", srcURL, nil)
		if err != nil {
			cancel()
			return "", err
		}

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			if attempt < maxAttempts {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return "", lastErr
		}

		// Ensure response body closed for each attempt
		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode > 299 {
				lastErr = fmt.Errorf("download failed: %s", resp.Status)
				return
			}

			// Prefer filename from Content-Disposition if present
			filename := ""
			if cd := resp.Header.Get("Content-Disposition"); cd != "" {
				if _, params, err := mime.ParseMediaType(cd); err == nil {
					if fn, ok := params["filename"]; ok && fn != "" {
						filename = fn
					}
				}
			}

			// Fallback: use the basename from the final request URL path
			if filename == "" && resp.Request != nil && resp.Request.URL != nil {
				filename = path.Base(resp.Request.URL.Path)
			}
			if filename == "" || filename == "." || filename == "/" {
				filename = "downloaded-image"
			}

			// Sanitize to avoid path traversal
			filename = filepath.Base(filename)

			out, err := os.Create(filename)
			if err != nil {
				lastErr = err
				return
			}
			defer func() {
				_ = out.Close()
				// On error, remove the partial file
				if lastErr != nil {
					_ = os.Remove(filename)
				}
			}()

			if _, err := io.Copy(out, resp.Body); err != nil {
				lastErr = err
				return
			}

			// Success: clear context cancel and set lastErr nil
			lastErr = nil
			// Return filename via outer scope by replacing function return using panic/defer is messy;
			// instead, shadow the function return by writing to a named variable.
			// To keep minimal changes, we capture via a closure result.
			// We'll assign to a package-local variable via named return not available here,
			// so set a sentinel by writing to a temporary file and re-opening after loop.
			// Simpler: reuse filename by setting it on the request context (not ideal).
			// Instead, set lastErr to a sentinel nil and write filename to header for retrieval outside.
			req.Header.Set("X-Image-Shepherd-Filename", filename)
		}()

		// Capture filename if success
		if lastErr == nil {
			// Retrieve filename from the request header (set above)
			fn := req.Header.Get("X-Image-Shepherd-Filename")
			cancel()
			return fn, nil
		}

		cancel()
		// Retry on next loop if attempts remain
		if attempt < maxAttempts {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("download failed: unknown error")
	}
	return "", lastErr
}

func (i Image) Upload(c *gophercloud.ServiceClient, meta SourceMeta) error {
	// Download the image
	zap.S().Infow("Starting download", "url", i.Url, "image", i.Name, "source_format", i.SourceFormat, "compression", i.Compression)
	filename, err := downloadToCwd(i.Url)
	if err != nil {
		zap.S().Errorw("Download failed", "url", i.Url, "image", i.Name, "error", err)
		return err
	}
	zap.S().Infow("Download completed", "file", filename, "image", i.Name)

	// Handle compression and format detection, then convert to raw if needed
	// SourceFormat and Compression can be set in images.yaml. If unset, auto-detect.

	// Step 1: Decompress if instructed or inferred
	srcFile := filename
	comp := strings.ToLower(strings.TrimSpace(i.Compression))
	decompressXZ := func(in string) (string, error) {
		zap.S().Infof("Decompressing xz image %s", in)
		outName := in[:len(in)-len(path.Ext(in))]
		out, err := os.Create(outName)
		if err != nil {
			return "", err
		}
		defer out.Close()
		cmd := exec.Command("xz", "-dc", in)
		cmd.Stdout = out
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", err
		}
		zap.S().Infow("Decompression complete", "compression", "xz", "input", in, "output", outName)
		return outName, nil
	}
	decompressGZ := func(in string) (string, error) {
		zap.S().Infof("Decompressing gzip image %s", in)
		outName := in[:len(in)-len(path.Ext(in))]
		out, err := os.Create(outName)
		if err != nil {
			return "", err
		}
		defer out.Close()
		cmd := exec.Command("gzip", "-dc", in)
		cmd.Stdout = out
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", err
		}
		zap.S().Infow("Decompression complete", "compression", "gz", "input", in, "output", outName)
		return outName, nil
	}

	switch comp {
	case "xz":
		if outName, err := decompressXZ(filename); err != nil {
			return err
		} else {
			srcFile = outName
		}
	case "gz", "gzip":
		if outName, err := decompressGZ(filename); err != nil {
			return err
		} else {
			srcFile = outName
		}
	case "none", "":
		// Infer by extension if not explicitly specified
		switch ext := path.Ext(filename); ext {
		case ".xz":
			if outName, err := decompressXZ(filename); err != nil {
				return err
			} else {
				srcFile = outName
			}
		case ".gz":
			if outName, err := decompressGZ(filename); err != nil {
				return err
			} else {
				srcFile = outName
			}
		}
	default:
		zap.S().Warnf("Unknown compression value %q; attempting to infer by extension", comp)
		switch ext := path.Ext(filename); ext {
		case ".xz":
			if outName, err := decompressXZ(filename); err != nil {
				return err
			} else {
				srcFile = outName
			}
		case ".gz":
			if outName, err := decompressGZ(filename); err != nil {
				return err
			} else {
				srcFile = outName
			}
		}
	}

	// Step 2: Determine source format (config or auto-detect)
	var format string
	sf := strings.ToLower(strings.TrimSpace(i.SourceFormat))
	if sf != "" {
		format = sf
		zap.S().Infow("Using source format from config", "format", format, "file", srcFile)
	} else {
		infoCmd := exec.Command("qemu-img", "info", srcFile)
		infoOut, err := infoCmd.Output()
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(infoOut), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "file format:") {
				_, _ = fmt.Sscanf(line, "file format: %s", &format)
				break
			}
		}
		if format == "" {
			zap.S().Errorw("Could not detect image format", "file", srcFile)
			return fmt.Errorf("unable to detect image format for %s", srcFile)
		}
		zap.S().Infow("Detected source format", "format", format, "file", srcFile)
	}

	// Step 3: Convert to raw if needed
	var rawFile string
	zap.S().Debugw("Beginning conversion decision", "detected_format", format, "file", srcFile)
	if format == "raw" {
		zap.S().Infow("Input image is already in raw format; skipping conversion", "file", srcFile)
		rawFile = srcFile
	} else {
		rawFile = fmt.Sprintf("%s.raw", srcFile)
		zap.S().Infow("Converting image to raw", "from_format", format, "input", srcFile, "output", rawFile)
		cmd := exec.Command("qemu-img", "convert", "-f", format, "-O", "raw", srcFile, rawFile)
		if err := cmd.Run(); err != nil {
			zap.S().Errorw("Conversion failed", "from_format", format, "input", srcFile, "output", rawFile, "error", err)
			return err
		}
		zap.S().Infow("Conversion complete", "output", rawFile)
	}

	// Determine the image visibility
	var visibility images.ImageVisibility
	if i.Public {
		visibility = images.ImageVisibilityPublic
	} else {
		visibility = images.ImageVisibilityPrivate
	}

	// Create the image object
	// Merge source metadata into properties
	if i.Properties == nil {
		i.Properties = map[string]string{}
	}
	i.Properties["source_url"] = i.Url
	if meta.ETag != "" {
		i.Properties["source_etag"] = meta.ETag
	}
	if meta.LastModified != "" {
		i.Properties["source_last_modified"] = meta.LastModified
	}
	if meta.ContentLength > 0 {
		i.Properties["source_content_length"] = fmt.Sprintf("%d", meta.ContentLength)
	}

	zap.S().Infow("Creating image object", "name", i.Name, "public", i.Public, "protected", i.Protected, "tags", i.Tags)
	createOpts := images.CreateOpts{
		Name:            i.Name,
		Tags:            i.Tags,
		Visibility:      &visibility,
		Protected:       &i.Protected,
		ContainerFormat: "bare",
		DiskFormat:      "raw",
		Properties:      i.Properties,
	}
	res, err := images.Create(context.TODO(), c, createOpts).Extract()
	if err != nil {
		return err
	}
	zap.S().Infow("Image object created", "id", res.ID, "name", i.Name)

	// Upload the image data
	data, err := os.Open(rawFile)
	if err != nil {
		return err
	}
	defer data.Close()

	zap.S().Infow("Uploading image data", "id", res.ID, "file", rawFile)
	// Use context with timeout for image data upload
	timeoutSecs := 6000
	if v := os.Getenv("IMAGE_SHEPHERD_UPLOAD_TIMEOUT_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSecs = n
		}
	}
	ctxUpload, cancelUpload := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancelUpload()

	// Temporarily increase HTTP client timeout to exceed the upload context timeout,
	// otherwise the client may abort early while awaiting headers.
	prevTimeout := c.HTTPClient.Timeout
	bump := time.Duration(timeoutSecs+30) * time.Second
	if prevTimeout != 0 && prevTimeout < bump {
		c.HTTPClient.Timeout = bump
		zap.S().Infow("Temporarily increasing HTTP client timeout for upload", "previous_timeout_secs", int(prevTimeout/time.Second), "new_timeout_secs", int(bump/time.Second))
	}
	defer func() {
		if prevTimeout != 0 {
			c.HTTPClient.Timeout = prevTimeout
			zap.S().Debugw("Restored HTTP client timeout after upload", "timeout_secs", int(prevTimeout/time.Second))
		}
	}()

	// Retry upload with backoff on transient failures/timeouts
	maxAttempts := 3
	backoff := 2 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Reset reader to beginning for each retry
		if _, seekErr := data.Seek(0, 0); seekErr != nil {
			zap.S().Errorw("Failed to seek image file before upload", "file", rawFile, "error", seekErr)
			return seekErr
		}

		zap.S().Infow("Uploading image data", "id", res.ID, "file", rawFile, "timeout_secs", timeoutSecs, "attempt", attempt, "max_attempts", maxAttempts)
		err = imagedata.Upload(ctxUpload, c, res.ID, data).ExtractErr()
		if err == nil {
			zap.S().Infow("Image data upload complete", "id", res.ID, "file", rawFile, "attempt", attempt)
			return nil
		}

		msg := err.Error()
		// Heuristic for transient errors
		retryable := strings.Contains(msg, "timeout") ||
			strings.Contains(msg, "context deadline exceeded") ||
			strings.Contains(msg, "connection reset") ||
			strings.Contains(msg, "EOF") ||
			strings.Contains(msg, "503") ||
			strings.Contains(msg, "502") ||
			strings.Contains(msg, "504")

		if attempt < maxAttempts && retryable {
			zap.S().Warnw("Image data upload failed, will retry with backoff", "id", res.ID, "file", rawFile, "attempt", attempt, "error", err, "backoff", backoff.String())
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		zap.S().Errorw("Image data upload failed", "id", res.ID, "file", rawFile, "attempt", attempt, "error", err)
		return err
	}
	return err
}

func RenameHideByID(c *gophercloud.ServiceClient, id string) error {
	zap.S().Infow("Renaming and hiding image by ID", "id", id)

	img, err := images.Get(context.TODO(), c, id).Extract()
	if err != nil {
		zap.S().Errorw("Failed to get image for rename/hide", "id", id, "error", err)
		return err
	}

	date, exists := img.Properties["uploaded"].(string)
	if !exists || date == "" {
		zap.S().Warnf("Image has no `uploaded` tag, falling back to creation time")
		date = img.CreatedAt.Format(uploadedFmt)
	}

	newName := fmt.Sprintf("%s-%s", img.Name, date)
	zap.S().Infow("Computed new name for image", "id", id, "old_name", img.Name, "new_name", newName, "date", date)

	updateOpts := images.UpdateOpts{
		images.ReplaceImageName{
			NewName: newName,
		},
		images.ReplaceImageHidden{
			NewHidden: true,
		},
	}

	_, err = images.Update(context.TODO(), c, id, updateOpts).Extract()
	if err != nil {
		zap.S().Errorw("Failed to rename/hide image", "id", id, "error", err)
		return err
	}

	zap.S().Infow("Renamed and hid image", "id", id, "old_name", img.Name, "new_name", newName, "os_hidden", true)
	return nil
}

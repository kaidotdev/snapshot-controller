package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"snapshot-controller/internal/capture"
	diffimage "snapshot-controller/internal/diff/image"
	difftext "snapshot-controller/internal/diff/text"
	"snapshot-controller/internal/retry"
	"snapshot-controller/internal/storage"
	"strconv"
	"time"

	"github.com/playwright-community/playwright-go"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

type WorkerOutput struct {
	BaselineURL          string  `json:"baselineURL"`
	TargetURL            string  `json:"targetURL"`
	BaselineHTMLURL      string  `json:"baselineHTMLURL"`
	TargetHTMLURL        string  `json:"targetHTMLURL"`
	ScreenshotDiffURL    string  `json:"screenshotDiffURL"`
	ScreenshotDiffAmount float64 `json:"screenshotDiffAmount"`
	HTMLDiffURL          string  `json:"htmlDiffURL"`
	HTMLDiffAmount       float64 `json:"htmlDiffAmount"`
}

type Worker struct {
	Capturer             capture.Capturer
	Storage              storage.Storage
	ScreenshotDiffFormat string
	HTMLDiffFormat       string
}

func envOrDefaultValue[T any](key string, defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}

	switch any(defaultValue).(type) {
	case string:
		return any(value).(T)
	case int:
		if intValue, err := strconv.Atoi(value); err == nil {
			return any(intValue).(T)
		}
	case int64:
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return any(intValue).(T)
		}
	case uint:
		if uintValue, err := strconv.ParseUint(value, 10, 0); err == nil {
			return any(uint(uintValue)).(T)
		}
	case uint64:
		if uintValue, err := strconv.ParseUint(value, 10, 64); err == nil {
			return any(uintValue).(T)
		}
	case float64:
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return any(floatValue).(T)
		}
	case bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return any(boolValue).(T)
		}
	case time.Duration:
		if durationValue, err := time.ParseDuration(value); err == nil {
			return any(durationValue).(T)
		}
	}

	return defaultValue
}

func main() {
	var screenshotFormat string
	var chromeDevtoolsProtocolURL string
	var screenshotDiffFormat string
	var htmlDiffFormat string
	var storageBackend string
	var callbackURL string
	flag.StringVar(&screenshotFormat, "screenshot-format", envOrDefaultValue("SCREENSHOT_FORMAT", "jpeg"), "Screenshot format (jpeg or png)")
	flag.StringVar(&chromeDevtoolsProtocolURL, "chrome-devtools-protocol-url", envOrDefaultValue("CHROME_DEVTOOLS_PROTOCOL_URL", ""), "Connect to existing browser via Chrome DevTools Protocol URL (e.g., http://localhost:9222)")
	flag.StringVar(&screenshotDiffFormat, "screenshot-diff-format", envOrDefaultValue("SCREENSHOT_DIFF_FORMAT", "pixel"), "Diff format (pixel or rectangle)")
	flag.StringVar(&htmlDiffFormat, "html-diff-format", envOrDefaultValue("HTML_DIFF_FORMAT", "line"), "Diff format (line)")
	flag.StringVar(&storageBackend, "storage-backend", envOrDefaultValue("STORAGE_BACKEND", "file"), "Storage backend (file or s3)")
	flag.StringVar(&callbackURL, "callback-url", envOrDefaultValue("CALLBACK_URL", ""), "Callback URL to send results to")

	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		os.Exit(1)
	}

	baseline := args[0]
	target := args[1]

	ctx := context.Background()

	config := capture.DefaultPlaywrightConfig()
	if screenshotFormat != "" {
		config.Format = screenshotFormat
	}
	if chromeDevtoolsProtocolURL != "" {
		config.ChromeDevtoolsProtocolURL = chromeDevtoolsProtocolURL
	}

	if err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
	}); err != nil {
		log.Fatalf("failed to install playwright browsers: %v", err)
	}

	capturer, err := capture.NewPlaywrightCapturer(ctx, config)
	if err != nil {
		log.Fatalf("failed to initialize capturer: %v", err)
	}

	var s storage.Storage
	switch storageBackend {
	case "file":
		s, err = storage.NewFileStorage(ctx, storage.FileConfig{
			Directory: envOrDefaultValue("DIRECTORY", "/tmp"),
		})
		if err != nil {
			log.Fatalf("failed to create file storage backend: %v", err)
		}
	case "s3":
		s, err = storage.NewS3Storage(ctx, storage.S3Config{
			Bucket: os.Getenv("S3_BUCKET"),
		})
		if err != nil {
			log.Fatalf("failed to create S3 storage backend: %v", err)
		}
	}

	worker := &Worker{
		Capturer:             capturer,
		Storage:              s,
		ScreenshotDiffFormat: screenshotDiffFormat,
		HTMLDiffFormat:       htmlDiffFormat,
	}

	result, err := worker.processSnapshot(ctx, baseline, target)
	if err != nil {
		log.Fatalf("failed to process snapshot: %v", err)
	}

	j, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal result: %v", err)
	}

	if callbackURL == "" {
		fmt.Println(string(j))
	} else {
		if err := callback(ctx, callbackURL, j); err != nil {
			log.Fatalf("failed to send callback: %v", err)
		}
	}
}

func (w *Worker) processSnapshot(ctx context.Context, baseline string, target string) (*WorkerOutput, error) {
	var baselineResult *capture.CaptureResult
	var targetResult *capture.CaptureResult

	// Step 1: Capture screenshots in parallel
	{
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			result, err := w.Capturer.Capture(ctx, baseline)
			if err != nil {
				return xerrors.Errorf("failed to capture baseline screenshot: %w", err)
			}
			baselineResult = result
			return nil
		})

		eg.Go(func() error {
			result, err := w.Capturer.Capture(ctx, target)
			if err != nil {
				return xerrors.Errorf("failed to capture target screenshot: %w", err)
			}
			targetResult = result
			return nil
		})

		if err := eg.Wait(); err != nil {
			return nil, err
		}
	}

	// Step 2: Generate diff image
	diffImage, diffAmount, err := w.generateDiff(baselineResult.Screenshot, targetResult.Screenshot, w.ScreenshotDiffFormat)
	if err != nil {
		return nil, xerrors.Errorf("failed to generate diff: %w", err)
	}

	// Step 2.5: Generate HTML diff
	htmlDiff, htmlDiffAmount, err := w.generateHTMLDiff(baselineResult.HTML, targetResult.HTML, w.HTMLDiffFormat)
	if err != nil {
		return nil, xerrors.Errorf("failed to generate HTML diff: %w", err)
	}

	// Step 3: Upload all images in parallel
	output := &WorkerOutput{}
	{
		eg, ctx := errgroup.WithContext(ctx)

		eg.Go(func() error {
			imageURL, htmlURL, err := w.uploadCapture(ctx, baseline, baselineResult)
			if err != nil {
				return err
			}
			output.BaselineURL = imageURL
			output.BaselineHTMLURL = htmlURL
			return nil
		})

		eg.Go(func() error {
			imageURL, htmlURL, err := w.uploadCapture(ctx, target, targetResult)
			if err != nil {
				return err
			}
			output.TargetURL = imageURL
			output.TargetHTMLURL = htmlURL
			return nil
		})

		eg.Go(func() error {
			timestamp := time.Now().Format("20060102150405")
			h := sha256.New()
			h.Write([]byte(baseline + target))
			hash := fmt.Sprintf("%x", h.Sum(nil))[:16]
			diffKey := fmt.Sprintf("Snapshot/diff/%s/%s.jpeg", hash, timestamp)

			url, err := w.Storage.Put(ctx, diffKey, diffImage)
			if err != nil {
				return xerrors.Errorf("failed to upload diff image: %w", err)
			}
			output.ScreenshotDiffURL = url
			output.ScreenshotDiffAmount = diffAmount
			return nil
		})

		eg.Go(func() error {
			timestamp := time.Now().Format("20060102150405")
			h := sha256.New()
			h.Write([]byte(baseline + target))
			hash := fmt.Sprintf("%x", h.Sum(nil))[:16]
			htmlDiffKey := fmt.Sprintf("Snapshot/diff/%s/%s.txt", hash, timestamp)

			url, err := w.Storage.Put(ctx, htmlDiffKey, htmlDiff)
			if err != nil {
				return xerrors.Errorf("failed to upload HTML diff: %w", err)
			}
			output.HTMLDiffURL = url
			output.HTMLDiffAmount = htmlDiffAmount
			return nil
		})

		if err := eg.Wait(); err != nil {
			return nil, err
		}
	}

	return output, nil
}

func (w *Worker) uploadCapture(ctx context.Context, url string, result *capture.CaptureResult) (string, string, error) {
	var imageURL string
	var htmlURL string
	{
		eg, ctx := errgroup.WithContext(ctx)

		timestamp := time.Now().Format("20060102150405")

		h := sha256.New()
		h.Write([]byte(url))
		urlHash := fmt.Sprintf("%x", h.Sum(nil))[:16]

		baseKey := fmt.Sprintf("Snapshot/capture/%s/%s", urlHash, timestamp)

		eg.Go(func() error {
			imageKey := baseKey + ".jpeg"
			path, err := w.Storage.Put(ctx, imageKey, result.Screenshot)
			if err != nil {
				return xerrors.Errorf("failed to upload screenshot: %w", err)
			}
			imageURL = path
			return nil
		})

		eg.Go(func() error {
			htmlKey := baseKey + ".html"
			path, err := w.Storage.Put(ctx, htmlKey, result.HTML)
			if err != nil {
				return xerrors.Errorf("failed to upload HTML: %w", err)
			}
			htmlURL = path
			return nil
		})

		if err := eg.Wait(); err != nil {
			return "", "", err
		}
	}

	return imageURL, htmlURL, nil
}

func (w *Worker) generateDiff(baselineData []byte, targetData []byte, format string) ([]byte, float64, error) {
	baselineImage, err := jpeg.Decode(bytes.NewReader(baselineData))
	if err != nil {
		return nil, 0.0, xerrors.Errorf("failed to decode baseline image: %w", err)
	}

	targetImage, err := jpeg.Decode(bytes.NewReader(targetData))
	if err != nil {
		return nil, 0.0, xerrors.Errorf("failed to decode target image: %w", err)
	}

	var differ diffimage.Differ
	switch format {
	case "rectangle":
		differ = diffimage.NewRectangleDiff()
	case "pixel":
		differ = diffimage.NewPixelDiff(0.1)
	default:
		return nil, 0.0, xerrors.Errorf("unknown diff format: %s", format)
	}

	diffResult := differ.Calculate(baselineImage, targetImage)

	var buffer bytes.Buffer
	err = jpeg.Encode(&buffer, diffResult.Image, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, 0.0, xerrors.Errorf("failed to encode diff image: %w", err)
	}

	return buffer.Bytes(), diffResult.DiffAmount, nil
}

func (w *Worker) generateHTMLDiff(baselineHTML []byte, targetHTML []byte, format string) ([]byte, float64, error) {
	var differ difftext.Differ
	switch format {
	case "line":
		differ = difftext.NewLineDiff()
	default:
		return nil, 0.0, xerrors.Errorf("unknown HTML diff format: %s", format)
	}

	diffResult, err := differ.Calculate(baselineHTML, targetHTML)

	if err != nil {
		return nil, 0.0, xerrors.Errorf("failed to calculate HTML diff: %w", err)
	}
	return diffResult.Diff, diffResult.DiffAmount, nil
}

func callback(ctx context.Context, callbackURL string, data []byte) error {
	request, err := http.NewRequestWithContext(ctx, "PATCH", callbackURL, bytes.NewReader(data))
	if err != nil {
		return xerrors.Errorf("failed to create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 1 * time.Second, // retry.Transport does not have perTryTimeout
		Transport: &retry.Transport{
			Base:          http.DefaultTransport,
			RetryStrategy: retry.NewExponentialBackOff(10*time.Millisecond, 1*time.Second, 3, nil),
			RetryOn:       retry.NewDefaultRetryOn(),
		},
	}

	response, err := client.Do(request)
	if err != nil {
		return xerrors.Errorf("failed to send request: %w", err)
	}
	defer response.Body.Close()

	return nil
}

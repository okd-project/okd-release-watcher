// okd-postsubmit-watcher reports on OKD SCOS postsubmit image jobs and on the
// freshness of images in the latest accepted OKD CI payloads. It is independent
// of okd-release-watcher and is meant to be run on demand or on a schedule.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/okd-project/okd-release-watcher/pkg/postsubmit"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func main() {
	o := &postsubmit.Options{}
	var (
		window, staleAfter  string
		payloadStaleAfter   string
		htmlFile, slackFile string
		reportURL           string
		jsonOut             bool
	)

	rootCmd := &cobra.Command{
		Use:   "okd-postsubmit-watcher",
		Short: "Report OKD postsubmit image job results and image freshness",
	}

	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate the postsubmit report",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if o.Window, err = parseDuration(window); err != nil {
				return fmt.Errorf("--window: %w", err)
			}
			if o.StaleAfter, err = parseDuration(staleAfter); err != nil {
				return fmt.Errorf("--stale-after: %w", err)
			}
			if o.PayloadStaleAfter, err = parseDuration(payloadStaleAfter); err != nil {
				return fmt.Errorf("--payload-stale-after: %w", err)
			}

			r, err := postsubmit.Generate(o)
			if err != nil {
				return err
			}

			if htmlFile != "" {
				h, err := r.HTML()
				if err != nil {
					return fmt.Errorf("rendering HTML: %w", err)
				}
				if err := os.WriteFile(htmlFile, []byte(h), 0o644); err != nil {
					return fmt.Errorf("writing HTML report: %w", err)
				}
				fmt.Fprintf(os.Stderr, "HTML report written to %s\n", htmlFile)
			}
			if slackFile != "" {
				if err := os.WriteFile(slackFile, []byte(r.Slack(reportURL)), 0o644); err != nil {
					return fmt.Errorf("writing Slack message: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Slack message written to %s\n", slackFile)
			}
			if jsonOut {
				fmt.Println(r.JSON())
			} else {
				fmt.Print(r.String())
			}
			return nil
		},
	}

	f := reportCmd.Flags()
	f.StringSliceVar(&o.Streams, "streams", postsubmit.DefaultStreams, "OKD release streams whose latest accepted payload defines the image list")
	f.StringVar(&window, "window", "12h", "Report postsubmit runs started within this window (e.g. 12h, 1d; Prow keeps ~24h)")
	f.StringVar(&staleAfter, "stale-after", "30d", "Flag images built longer ago than this (e.g. 30d, 720h)")
	f.StringVar(&payloadStaleAfter, "payload-stale-after", "3d", "Warn when a stream's latest accepted payload is older than this (0 disables)")
	f.IntVar(&o.Concurrency, "concurrency", 20, "Parallel registry/GCS lookups")
	f.StringVar(&htmlFile, "html", "", "Write HTML report to this file")
	f.StringVar(&slackFile, "slack", "", "Write Slack mrkdwn summary to this file")
	f.StringVar(&reportURL, "report-url", "", "URL of the published HTML report, linked from the Slack summary")
	f.BoolVar(&jsonOut, "json", false, "Print JSON instead of the text report")

	rootCmd.AddCommand(reportCmd)

	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// parseDuration accepts Go durations plus a "d" (days) suffix.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(s)
}

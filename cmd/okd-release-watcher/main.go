package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/okd-project/okd-release-watcher/pkg/report"
	"github.com/okd-project/okd-release-watcher/pkg/server"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func main() {
	o := &report.Options{}
	var slackFormat bool

	rootCmd := &cobra.Command{
		Use:   "okd-release-watcher",
		Short: "Monitor OKD release streams for failed/rejected builds",
	}

	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a release stream health report",
		RunE: func(cmd *cobra.Command, args []string) error {
			parseInlineArgs(args, o)
			r, err := report.GenerateReport(o)
			if err != nil {
				return err
			}
			if o.JSONOutput {
				fmt.Println(r.JSON())
			} else if slackFormat {
				fmt.Println(server.FormatSlackMessage(r, o.SlackAlias))
				htmlFile := "report.html"
				if err := os.WriteFile(htmlFile, []byte(r.HTML()), 0644); err != nil {
					return fmt.Errorf("writing HTML report: %w", err)
				}
				fmt.Fprintf(os.Stderr, "HTML report written to %s\n", htmlFile)
			} else {
				fmt.Println(r.String())
			}
			return nil
		},
	}

	reportCmd.Flags().StringSliceVar(&o.Streams, "streams", report.DefaultStreams, "Release streams to monitor")
	reportCmd.Flags().DurationVar(&o.Lookback, "lookback", 24*time.Hour, "How far back to check for builds")
	reportCmd.Flags().BoolVar(&o.IncludeHealthy, "include-healthy", false, "Include healthy (accepted) builds in output")
	reportCmd.Flags().BoolVar(&o.JSONOutput, "json", false, "Output as JSON")
	reportCmd.Flags().BoolVar(&slackFormat, "slack", false, "Output in Slack format (preview what the bot would post)")

	botCmd := &cobra.Command{
		Use:   "bot",
		Short: "Run the Slack bot server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Serve(o)
		},
	}

	botCmd.Flags().StringSliceVar(&o.Streams, "streams", report.DefaultStreams, "Release streams to monitor")
	botCmd.Flags().DurationVar(&o.Lookback, "lookback", 24*time.Hour, "How far back to check for builds")
	botCmd.Flags().BoolVar(&o.IncludeHealthy, "include-healthy", false, "Include healthy (accepted) builds in output")
	botCmd.Flags().StringVar(&o.SlackAlias, "slack-alias", "", "Slack subteam ID to tag in reports")

	rootCmd.AddCommand(reportCmd, botCmd)

	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func parseInlineArgs(args []string, o *report.Options) {
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "lookback="):
			if d, err := time.ParseDuration(strings.TrimPrefix(arg, "lookback=")); err == nil {
				o.Lookback = d
			}
		case strings.HasPrefix(arg, "streams="):
			o.Streams = strings.Split(strings.TrimPrefix(arg, "streams="), ",")
		case arg == "healthy":
			o.IncludeHealthy = true
		case arg == "json":
			o.JSONOutput = true
		}
	}
}

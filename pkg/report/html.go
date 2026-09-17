package report

import (
	"fmt"
	"html"
	"strings"
)

func (r *Report) HTML() string {
	var b strings.Builder

	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>OKD Release Stream Report</title>
<style>
  :root {
    --bg: #0d1117;
    --surface: #161b22;
    --border: #30363d;
    --text: #e6edf3;
    --text-muted: #8b949e;
    --green: #3fb950;
    --red: #f85149;
    --yellow: #d29922;
    --blue: #58a6ff;
    --green-bg: #0d2818;
    --red-bg: #2d1114;
    --yellow-bg: #2d2305;
  }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
    background: var(--bg);
    color: var(--text);
    line-height: 1.5;
    padding: 24px;
    max-width: 1100px;
    margin: 0 auto;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 24px;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--border);
  }
  header h1 { font-size: 20px; font-weight: 600; }
  header .meta { color: var(--text-muted); font-size: 13px; }
  .health-table {
    width: 100%;
    border-collapse: collapse;
    margin-bottom: 32px;
    font-size: 14px;
  }
  .health-table th {
    text-align: left;
    padding: 10px 16px;
    background: var(--surface);
    border: 1px solid var(--border);
    color: var(--text-muted);
    font-weight: 500;
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .health-table td {
    padding: 10px 16px;
    border: 1px solid var(--border);
  }
  .health-table tr:hover td { background: var(--surface); }
  .badge {
    display: inline-block;
    padding: 2px 10px;
    border-radius: 12px;
    font-size: 12px;
    font-weight: 600;
  }
  .badge-accepted { background: var(--green-bg); color: var(--green); }
  .badge-rejected { background: var(--red-bg); color: var(--red); }
  .badge-failed { background: var(--red-bg); color: var(--red); }
  .badge-stale { background: var(--yellow-bg); color: var(--yellow); }
  .badge-healthy { background: var(--green-bg); color: var(--green); }
  .stream-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    margin-bottom: 16px;
    overflow: hidden;
  }
  .stream-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 14px 20px;
    cursor: pointer;
    user-select: none;
  }
  .stream-header:hover { background: rgba(255,255,255,0.03); }
  .stream-header h2 { font-size: 15px; font-weight: 600; }
  .stream-header h2 a { color: var(--blue); text-decoration: none; }
  .stream-header h2 a:hover { text-decoration: underline; }
  .stream-header .stats {
    display: flex;
    gap: 12px;
    align-items: center;
    font-size: 13px;
    color: var(--text-muted);
  }
  .stream-body {
    padding: 0 20px 16px;
    border-top: 1px solid var(--border);
  }
  .stream-body.collapsed { display: none; }
  .latest-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 0;
    font-size: 13px;
    color: var(--text-muted);
  }
  .latest-row code {
    font-size: 12px;
    background: var(--bg);
    padding: 2px 6px;
    border-radius: 4px;
    color: var(--text);
  }
  .warning-box {
    background: var(--yellow-bg);
    border-left: 3px solid var(--yellow);
    padding: 8px 12px;
    margin: 8px 0;
    font-size: 13px;
    border-radius: 0 4px 4px 0;
    color: var(--yellow);
  }
  .build-entry {
    margin: 12px 0;
    padding: 12px 16px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
  }
  .build-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 8px;
  }
  .job-section { margin: 6px 0; font-size: 13px; }
  .job-section .label {
    color: var(--text-muted);
    font-weight: 500;
    min-width: 80px;
    display: inline-block;
  }
  .job-section .jobs { color: var(--text); }
  .analysis-link { margin: 8px 0; font-size: 13px; }
  .analysis-link a { color: var(--blue); text-decoration: none; }
  .analysis-link a:hover { text-decoration: underline; }
  .root-cause {
    margin: 8px 0;
    padding: 8px 12px;
    background: var(--surface);
    border-left: 3px solid var(--red);
    border-radius: 0 4px 4px 0;
    font-size: 13px;
    line-height: 1.6;
  }
  .root-cause .label {
    color: var(--text-muted);
    font-size: 12px;
    font-weight: 500;
  }
  .streak { font-size: 12px; color: var(--yellow); margin-top: 6px; }
  .no-issues { padding: 12px 0; color: var(--green); font-size: 13px; }
  .info-section {
    margin: 16px 0 4px;
    font-size: 12px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-weight: 500;
  }
  .build-entry-info {
    margin: 8px 0;
    padding: 10px 16px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-left: 3px solid var(--blue);
    border-radius: 0 6px 6px 0;
  }
  .build-entry-info .build-title { margin-bottom: 4px; }
  .info-jobs {
    font-size: 13px;
    color: var(--text-muted);
  }
  .info-jobs .label {
    color: var(--text-muted);
    font-weight: 500;
    min-width: 120px;
    display: inline-block;
  }
  .toggle-arrow {
    color: var(--text-muted);
    font-size: 12px;
    transition: transform 0.2s;
  }
  .stream-card.open .toggle-arrow { transform: rotate(90deg); }
  footer {
    margin-top: 24px;
    padding-top: 16px;
    border-top: 1px solid var(--border);
    font-size: 12px;
    color: var(--text-muted);
    text-align: center;
  }
  footer a { color: var(--blue); text-decoration: none; }
</style>
</head>
<body>
`)

	fmt.Fprintf(&b, "<header>\n  <h1>OKD Release Stream Report</h1>\n")
	fmt.Fprintf(&b, "  <div class=\"meta\">Lookback: %s &middot; Generated: %s</div>\n", r.Lookback, r.Generated.Format("2006-01-02T15:04:05Z"))
	b.WriteString("</header>\n\n")

	// Health overview table
	b.WriteString("<table class=\"health-table\">\n<thead><tr>")
	b.WriteString("<th>Stream</th><th>Latest Build</th><th>Builds</th><th>Last Accepted</th><th>Last Built</th><th>Status</th>")
	b.WriteString("</tr></thead>\n<tbody>\n")

	for _, sr := range r.Streams {
		b.WriteString("<tr>\n")

		fmt.Fprintf(&b, "  <td><a href=\"%s\" style=\"color: var(--blue); text-decoration: none;\">%s</a></td>\n",
			html.EscapeString(sr.StreamURL), html.EscapeString(sr.StreamName))

		if sr.LatestPhase != "" {
			fmt.Fprintf(&b, "  <td><span class=\"badge badge-%s\">%s</span></td>\n",
				strings.ToLower(sr.LatestPhase), sr.LatestPhase)
		} else {
			b.WriteString("  <td>-</td>\n")
		}

		buildsSummary := fmt.Sprintf("%d", sr.TotalInWindow)
		parts := []string{}
		if sr.AcceptedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d accepted", sr.AcceptedCount))
		}
		if sr.RejectedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d rejected", sr.RejectedCount))
		}
		if sr.FailedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", sr.FailedCount))
		}
		if len(parts) > 0 {
			buildsSummary += " (" + strings.Join(parts, ", ") + ")"
		}
		fmt.Fprintf(&b, "  <td>%s</td>\n", buildsSummary)

		if sr.AcceptedStale {
			fmt.Fprintf(&b, "  <td style=\"color: var(--yellow);\">%s ⚠</td>\n", html.EscapeString(sr.LastAcceptedAge))
		} else if sr.LastAcceptedAge != "" {
			fmt.Fprintf(&b, "  <td>%s</td>\n", html.EscapeString(sr.LastAcceptedAge))
		} else {
			b.WriteString("  <td>-</td>\n")
		}

		if sr.BuildStale {
			fmt.Fprintf(&b, "  <td style=\"color: var(--yellow);\">%s ⚠</td>\n", html.EscapeString(sr.LastBuiltAge))
		} else if sr.LastBuiltAge != "" {
			fmt.Fprintf(&b, "  <td>%s</td>\n", html.EscapeString(sr.LastBuiltAge))
		} else {
			b.WriteString("  <td>-</td>\n")
		}

		if sr.AcceptedStale || sr.BuildStale {
			b.WriteString("  <td><span class=\"badge badge-stale\">Stale</span></td>\n")
		} else {
			b.WriteString("  <td><span class=\"badge badge-healthy\">Healthy</span></td>\n")
		}

		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>\n\n")

	// Stream detail cards
	for i, sr := range r.Streams {
		hasIssues := sr.RejectedCount+sr.FailedCount > 0 || sr.AcceptedStale || sr.BuildStale
		cardID := fmt.Sprintf("stream-%d", i)

		openClass := ""
		bodyClass := "collapsed"
		if hasIssues {
			openClass = " open"
			bodyClass = ""
		}

		fmt.Fprintf(&b, "<div class=\"stream-card%s\" id=\"%s\">\n", openClass, cardID)
		fmt.Fprintf(&b, "  <div class=\"stream-header\" onclick=\"toggleStream('%s')\">\n", cardID)
		fmt.Fprintf(&b, "    <h2><span class=\"toggle-arrow\">▶</span> <a href=\"%s\">%s</a></h2>\n",
			html.EscapeString(sr.StreamURL), html.EscapeString(sr.StreamName))
		b.WriteString("    <div class=\"stats\">\n")
		fmt.Fprintf(&b, "      <span>%d builds</span>\n", sr.TotalInWindow)
		if sr.LatestPhase != "" {
			fmt.Fprintf(&b, "      <span class=\"badge badge-%s\">%s</span>\n",
				strings.ToLower(sr.LatestPhase), sr.LatestPhase)
		}
		if sr.AcceptedStale || sr.BuildStale {
			b.WriteString("      <span class=\"badge badge-stale\">Stale</span>\n")
		}
		b.WriteString("    </div>\n  </div>\n")

		fmt.Fprintf(&b, "  <div class=\"stream-body %s\">\n", bodyClass)

		if sr.LatestTag != "" {
			fmt.Fprintf(&b, "    <div class=\"latest-row\">Latest: <code>%s</code> <span class=\"badge badge-%s\">%s</span></div>\n",
				html.EscapeString(sr.LatestTag), strings.ToLower(sr.LatestPhase), sr.LatestPhase)
		}

		if sr.AcceptedStale {
			if sr.LastAcceptedTag != "" {
				fmt.Fprintf(&b, "    <div class=\"warning-box\">⚠ No accepted payload in %s (last: %s)</div>\n",
					html.EscapeString(sr.LastAcceptedAge), html.EscapeString(sr.LastAcceptedTag))
			} else {
				b.WriteString("    <div class=\"warning-box\">⚠ No accepted payload found</div>\n")
			}
		}
		if sr.BuildStale {
			if sr.LastBuiltTag != "" {
				fmt.Fprintf(&b, "    <div class=\"warning-box\">⚠ No payload built in %s (last: %s)</div>\n",
					html.EscapeString(sr.LastBuiltAge), html.EscapeString(sr.LastBuiltTag))
			} else {
				b.WriteString("    <div class=\"warning-box\">⚠ No payload built</div>\n")
			}
		}

		if len(sr.FailedRejected) == 0 && !sr.AcceptedStale && !sr.BuildStale && len(sr.AcceptedBuilds) == 0 {
			b.WriteString("    <div class=\"no-issues\">✓ No issues detected</div>\n")
		}

		for _, tr := range sr.FailedRejected {
			if tr.Phase == "Accepted" {
				continue
			}

			b.WriteString("    <div class=\"build-entry\">\n")
			fmt.Fprintf(&b, "      <div class=\"build-title\"><span class=\"badge badge-%s\">%s</span> %s</div>\n",
				strings.ToLower(tr.Phase), tr.Phase, html.EscapeString(tr.Name))

			if len(tr.BlockingFailed) > 0 {
				names := make([]string, len(tr.BlockingFailed))
				for j, jf := range tr.BlockingFailed {
					names[j] = html.EscapeString(jf.Name)
				}
				fmt.Fprintf(&b, "      <div class=\"job-section\"><span class=\"label\">Blocking:</span> <span class=\"jobs\">%s</span></div>\n",
					strings.Join(names, ", "))
			}

			if len(tr.InformingFailed) > 0 {
				names := make([]string, len(tr.InformingFailed))
				for j, jf := range tr.InformingFailed {
					names[j] = html.EscapeString(jf.Name)
				}
				fmt.Fprintf(&b, "      <div class=\"job-section\"><span class=\"label\">Informing:</span> <span class=\"jobs\">%s</span></div>\n",
					strings.Join(names, ", "))
			}

			if tr.Phase == "Failed" && len(tr.BlockingFailed) == 0 {
				b.WriteString("      <div class=\"job-section\"><span class=\"jobs\">Payload could not be assembled (no jobs ran)</span></div>\n")
			}

			if tr.AnalysisHTMLURL != "" {
				fmt.Fprintf(&b, "      <div class=\"analysis-link\">📋 <a href=\"%s\">Claude Payload Agent Report</a></div>\n",
					html.EscapeString(tr.AnalysisHTMLURL))
			} else {
				b.WriteString("      <div class=\"analysis-link\" style=\"color: var(--text-muted);\">📋 Claude Payload Agent analysis not available</div>\n")
			}

			for _, rc := range tr.RootCauses {
				fmt.Fprintf(&b, "      <div class=\"root-cause\"><div class=\"label\">Root cause</div>%s</div>\n",
					html.EscapeString(rc))
			}

			if tr.RejectionStreak > 1 {
				fmt.Fprintf(&b, "      <div class=\"streak\">🔄 %d consecutive rejections</div>\n", tr.RejectionStreak)
			}

			b.WriteString("    </div>\n")
		}

		if len(sr.AcceptedBuilds) > 0 {
			b.WriteString("    <div class=\"info-section\">ℹ️ Accepted builds with informing job failures</div>\n")
			for _, tr := range sr.AcceptedBuilds {
				b.WriteString("    <div class=\"build-entry-info\">\n")
				fmt.Fprintf(&b, "      <div class=\"build-title\"><span class=\"badge badge-accepted\">Accepted</span> %s</div>\n",
					html.EscapeString(tr.Name))
				if len(tr.InformingFailed) > 0 {
					names := make([]string, len(tr.InformingFailed))
					for j, jf := range tr.InformingFailed {
						names[j] = html.EscapeString(jf.Name)
					}
					fmt.Fprintf(&b, "      <div class=\"info-jobs\"><span class=\"label\">Informing failed:</span> %s</div>\n",
						strings.Join(names, ", "))
				}
				b.WriteString("    </div>\n")
			}
		}

		b.WriteString("  </div>\n</div>\n\n")
	}

	b.WriteString(`<footer>
  OKD Release Watcher &middot; <a href="https://github.com/okd-project/okd-release-watcher">github.com/okd-project/okd-release-watcher</a>
</footer>

<script>
function toggleStream(id) {
  const card = document.getElementById(id);
  const body = card.querySelector('.stream-body');
  card.classList.toggle('open');
  body.classList.toggle('collapsed');
}
</script>
</body>
</html>
`)

	return b.String()
}

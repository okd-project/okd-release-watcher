package postsubmit

import (
	"bytes"
	"html/template"
	"strings"
	"time"
)

// HTML renders a self-contained HTML report.
func (r *Report) HTML() (string, error) {
	now := r.Generated
	funcs := template.FuncMap{
		"age":   func(t time.Time) string { return age(t, now) },
		"short": shortSHA,
		"ts": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02 15:04Z")
		},
		"unix": func(t time.Time) int64 {
			if t.IsZero() {
				return 0
			}
			return t.Unix()
		},
		"isFail": isFailState,
		"healthy": func(rows []ImageRow) int {
			n := 0
			for _, r := range rows {
				if len(r.Flags) == 0 {
					n++
				}
			}
			return n
		},
		"join":  strings.Join,
		"lower": strings.ToLower,
		"dur":   func(d time.Duration) string { return FormatAge(d) },
		"commitURL": func(org, repo, sha string) string {
			if org == "" || sha == "" {
				return ""
			}
			return "https://github.com/" + org + "/" + repo + "/commit/" + sha
		},
		"runDur": func(run Run) string {
			if run.Completed.IsZero() {
				return "running"
			}
			return run.Completed.Sub(run.Started).Round(time.Minute).String()
		},
	}
	t, err := template.New("report").Funcs(funcs).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}
	unmapped := 0
	for _, run := range r.Runs {
		if len(run.Streams) == 0 {
			unmapped++
		}
	}
	data := struct {
		*Report
		Counts       map[string]int
		Failed       int
		UnmappedRuns int
	}{r, r.RunCounts(), len(r.FailedRuns()), unmapped}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>OKD Postsubmit Report</title>
<style>
:root {
  --bg:#ffffff; --surface:#f6f8fa; --border:#d0d7de; --text:#1f2328; --muted:#656d76;
  --green:#1a7f37; --red:#cf222e; --yellow:#9a6700; --blue:#0969da; --grey:#6e7781;
  --green-bg:#dafbe1; --red-bg:#ffebe9; --yellow-bg:#fff8c5; --blue-bg:#ddf4ff; --grey-bg:#eaeef2;
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --bg:#0d1117; --surface:#161b22; --border:#30363d; --text:#e6edf3; --muted:#8b949e;
    --green:#3fb950; --red:#f85149; --yellow:#d29922; --blue:#58a6ff; --grey:#8b949e;
    --green-bg:#0d2818; --red-bg:#2d1114; --yellow-bg:#2d2305; --blue-bg:#0c2d4d; --grey-bg:#21262d;
  }
}
:root[data-theme="dark"] {
  --bg:#0d1117; --surface:#161b22; --border:#30363d; --text:#e6edf3; --muted:#8b949e;
  --green:#3fb950; --red:#f85149; --yellow:#d29922; --blue:#58a6ff; --grey:#8b949e;
  --green-bg:#0d2818; --red-bg:#2d1114; --yellow-bg:#2d2305; --blue-bg:#0c2d4d; --grey-bg:#21262d;
}
* { box-sizing:border-box; margin:0; padding:0; }
body { font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif; background:var(--bg); color:var(--text);
  line-height:1.5; padding:24px 16px; max-width:1280px; margin:0 auto; font-size:14px; }
header { display:flex; flex-wrap:wrap; gap:8px; justify-content:space-between; align-items:baseline; padding-bottom:16px; margin-bottom:24px; border-bottom:1px solid var(--border); }
h1 { font-size:20px; font-weight:600; }
h2 { font-size:16px; font-weight:600; margin:32px 0 12px; }
.meta, .muted { color:var(--muted); font-size:13px; }
a { color:var(--blue); text-decoration:none; } a:hover { text-decoration:underline; }
.cards { display:grid; grid-template-columns:repeat(auto-fit,minmax(150px,1fr)); gap:12px; margin-bottom:8px; }
.card { background:var(--surface); border:1px solid var(--border); border-radius:8px; padding:12px 14px; }
.card .n { font-size:24px; font-weight:600; font-variant-numeric:tabular-nums; }
.card .l { color:var(--muted); font-size:12px; }
.card.red .n { color:var(--red); } .card.green .n { color:var(--green); } .card.yellow .n { color:var(--yellow); } .card.blue .n { color:var(--blue); } .card.grey .n { color:var(--grey); }
.tablewrap { overflow-x:auto; border:1px solid var(--border); border-radius:8px; }
table { width:100%; border-collapse:collapse; font-size:13px; }
th, td { text-align:left; padding:6px 10px; border-bottom:1px solid var(--border); white-space:nowrap; }
th { background:var(--surface); color:var(--muted); font-weight:600; cursor:pointer; user-select:none; position:sticky; top:0; }
th.sorted-asc::after { content:" ▲"; } th.sorted-desc::after { content:" ▼"; }
tr:last-child td { border-bottom:none; }
td.num { font-variant-numeric:tabular-nums; }
code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; font-size:12px; }
.pill { display:inline-block; padding:1px 8px; border-radius:10px; font-size:12px; font-weight:500; }
.p-success { background:var(--green-bg); color:var(--green); }
.p-failure, .p-error, .p-aborted, .p-failing { background:var(--red-bg); color:var(--red); }
.p-pending, .p-triggered { background:var(--blue-bg); color:var(--blue); }
.p-stale { background:var(--yellow-bg); color:var(--yellow); }
.p-no-run, .p-none { background:var(--grey-bg); color:var(--grey); }
.p-pending-flag { background:var(--blue-bg); color:var(--blue); }
.toolbar { display:flex; flex-wrap:wrap; gap:8px; align-items:center; margin-bottom:10px; }
.toolbar input { background:var(--surface); color:var(--text); border:1px solid var(--border); border-radius:6px; padding:5px 10px; font-size:13px; min-width:220px; }
.chip { background:var(--surface); color:var(--text); border:1px solid var(--border); border-radius:14px; padding:3px 12px; font-size:12px; cursor:pointer; }
.chip.active { border-color:var(--blue); color:var(--blue); background:var(--blue-bg); }
.err { background:var(--red-bg); color:var(--red); border-radius:6px; padding:8px 12px; margin:8px 0; }
section { margin-top:28px; }
.section-head { display:flex; gap:12px; align-items:flex-start; margin-bottom:14px; }
.section-head h2 { margin:0 0 4px; }
.num { display:inline-flex; align-items:center; justify-content:center; min-width:24px; height:24px; border-radius:12px;
  background:var(--blue); color:var(--bg); font-size:13px; font-weight:600; flex-shrink:0; }
.section-head .num { margin-top:1px; }
.purpose { color:var(--muted); max-width:900px; }
.stats { display:flex; flex-wrap:wrap; gap:8px 18px; align-items:baseline; margin-bottom:12px; }
.stat b { font-size:18px; font-variant-numeric:tabular-nums; margin-right:4px; }
.stat.green b { color:var(--green); } .stat.red b { color:var(--red); } .stat.blue b { color:var(--blue); }
.stat-link { margin-left:auto; font-size:13px; }
.callout { border-radius:8px; padding:10px 14px; margin-bottom:12px; border:1px solid var(--border); }
.callout.red { background:var(--red-bg); border-color:var(--red); }
.callout.green { background:var(--green-bg); border-color:var(--green); color:var(--green); }
.callout ul { list-style:none; margin-top:6px; } .callout li { margin:3px 0; }
details.fold { border:1px solid var(--border); border-radius:8px; margin-bottom:10px; background:var(--bg); }
details.fold > summary { list-style:none; cursor:pointer; padding:10px 14px; display:flex; flex-wrap:wrap; gap:8px 12px; align-items:center;
  background:var(--surface); border-radius:8px; font-weight:500; }
details.fold[open] > summary { border-bottom:1px solid var(--border); border-radius:8px 8px 0 0; }
details.fold > summary::-webkit-details-marker { display:none; }
details.fold > summary:hover { background:var(--blue-bg); }
.chev { width:0; height:0; border-left:6px solid var(--muted); border-top:5px solid transparent; border-bottom:5px solid transparent; transition:transform .15s; }
details.fold[open] > summary .chev { transform:rotate(90deg); }
.stream-title { font-weight:600; font-size:15px; }
.badges { display:flex; flex-wrap:wrap; gap:6px; margin-left:auto; }
.fold-body { padding:12px 14px; }
.fold-body .tablewrap.scroll { max-height:70vh; overflow:auto; }
.legend { display:flex; flex-wrap:wrap; gap:6px 18px; color:var(--muted); font-size:12px; margin-bottom:12px; }
.activity { border:1px solid var(--border); border-radius:8px; padding:8px 14px; margin-bottom:12px; }
.act-row { display:flex; flex-wrap:wrap; gap:6px 10px; align-items:center; padding:4px 0; }
.stream-chip { display:inline-block; font-size:12px; font-weight:600; padding:0 8px; border-radius:6px; background:var(--blue-bg); color:var(--blue); }
.muted-chip { background:var(--grey-bg); color:var(--grey); }
.red-t { color:var(--red); }
footer { margin-top:40px; color:var(--muted); font-size:12px; }
</style>
</head>
<body>
<header>
  <h1>OKD Postsubmit Report</h1>
  <div class="meta">Generated {{ts .Generated}} · window {{dur .Window}} · stale after {{dur .StaleAfter}}</div>
</header>

<section id="runs">
  <div class="section-head">
    <span class="num">1</span>
    <div>
      <h2>Recent postsubmit runs <span class="muted">· last {{dur .Window}}</span></h2>
      <p class="purpose"><b>Goal:</b> catch OKD image builds that broke recently. Every merge to an OKD-enabled repo runs a
        <code>branch-ci-*-okd-scos-images</code> postsubmit job. If that job fails, the component's image is <b>not</b> updated
        and payloads silently keep the old one. Failed runs below need a look.</p>
    </div>
  </div>

  <div class="stats">
    <span class="stat"><b>{{len .Runs}}</b> runs</span>
    <span class="stat green"><b>{{index .Counts "success"}}</b> passed</span>
    <span class="stat red"><b>{{.Failed}}</b> failed / error</span>
    <span class="stat blue"><b>{{index .Counts "pending"}}</b> running</span>
    <a class="stat-link" href="https://prow.ci.openshift.org/?job=branch-ci-*okd-scos-images">Open in Prow ↗</a>
  </div>
  {{if .RunsError}}<div class="err">Could not fetch runs: {{.RunsError}}</div>{{end}}

  <div class="activity">
  {{range .Activity}}<div class="act-row">
    <span class="stream-chip">{{.Version}}</span>
    {{if .RunsInWindow}}<span><b>{{.RunsInWindow}}</b> run{{if ne .RunsInWindow 1}}s{{end}} in window{{if .FailedInWindow}}, <b class="red-t">{{.FailedInWindow}} failed</b>{{end}}</span>
    {{else}}<span class="muted">no runs in the last {{dur $.Window}} (no merges to this stream's branches)</span>{{end}}
    {{with .LastRun}}<span class="muted">· most recent: <a href="{{.URL}}">{{.Repo}}</a> @{{.Branch}}, {{age .Finished}} ago,</span>
      <span class="pill p-{{lower .Result}}">{{.Result}}</span>{{end}}
  </div>{{end}}
  {{if .UnmappedRuns}}<div class="act-row"><span class="stream-chip muted-chip">other</span><span class="muted"><b>{{.UnmappedRuns}}</b> run{{if ne .UnmappedRuns 1}}s{{end}} of jobs not used by any current payload image</span></div>{{end}}
  </div>

  {{if .Failed}}
  <div class="callout red">
    <b>Failed runs</b>
    <ul>{{range .Runs}}{{if isFail .State}}<li><span class="pill p-{{.State}}">{{.State}}</span> {{range .Streams}}<span class="stream-chip">{{.}}</span> {{end}}
      <a href="{{.URL}}">{{.Repo}}</a> <span class="muted">@{{.Branch}} · {{age .Started}} ago</span></li>{{end}}{{end}}</ul>
  </div>
  {{else if .Runs}}
  <div class="callout green">✅ No OKD postsubmit failures in the last {{dur .Window}}.</div>
  {{end}}

  {{if .Runs}}
  <details class="fold">
    <summary><span class="chev"></span>All {{len .Runs}} runs in the window</summary>
    <div class="fold-body">
    <div class="tablewrap"><table class="sortable">
    <thead><tr><th>Result</th><th>Stream</th><th>Job</th><th>Branch</th><th>Started</th><th>Duration</th><th>Commit</th><th>Logs</th></tr></thead>
    <tbody>
    {{range .Runs}}<tr>
      <td data-v="{{.State}}"><span class="pill p-{{.State}}">{{.State}}</span></td>
      <td>{{if .Streams}}{{range .Streams}}<span class="stream-chip">{{.}}</span> {{end}}{{else}}<span class="muted" title="No image in the current payloads is built by this job">–</span>{{end}}</td>
      <td><a href="https://prow.ci.openshift.org/job-history/gs/test-platform-results-public/logs/{{.Job}}">{{.Repo}}</a></td>
      <td>{{.Branch}}</td>
      <td data-v="{{unix .Started}}" title="{{ts .Started}}">{{age .Started}} ago</td>
      <td>{{runDur .}}</td>
      <td>{{if .SHA}}<a href="{{commitURL .Org .Repo .SHA}}"><code>{{short .SHA}}</code></a>{{end}}</td>
      <td>{{if .URL}}<a href="{{.URL}}">logs</a>{{end}}</td>
    </tr>{{end}}
    </tbody></table></div>
    </div>
  </details>
  {{else}}<p class="muted">No OKD postsubmit runs in the window.</p>{{end}}
</section>

<section id="images">
  <div class="section-head">
    <span class="num">2</span>
    <div>
      <h2>Image freshness per stream</h2>
      <p class="purpose"><b>Goal:</b> show what is actually inside the latest <b>accepted</b> OKD payload of each stream:
        how old every image is, whether its postsubmit job is currently broken, and whether newer builds are waiting.
        This catches images that went stale quietly, including ones whose repo simply has had no merges.</p>
    </div>
  </div>
  {{if .RegistryError}}<div class="err">Image build times unavailable: {{.RegistryError}}</div>{{end}}
  <div class="legend">
    <span><span class="pill p-failing">failing</span> latest postsubmit failed, image stuck at previous build</span>
    <span><span class="pill p-pending-flag">pending</span> job built a newer image (new commit or rebuild) not yet in an accepted payload</span>
    <span><span class="pill p-stale">stale</span> image built more than {{dur .StaleAfter}} ago</span>
    <span><span class="pill p-no-run">no-run</span> no job results in GCS (~60 day retention)</span>
  </div>

  {{range $si, $s := .Streams}}
  <details class="fold stream"{{if index $s.Counts "failing"}} open{{end}}>
    <summary>
      <span class="chev"></span>
      <span class="stream-title">Stream {{$s.Version}}</span>
      {{if $s.Error}}<span class="pill p-failure">error</span>{{else}}
      <span class="muted">{{len $s.Images}} images · {{$s.Payload}}</span>
      {{if $s.PayloadStale}}<span class="pill p-failure" title="Latest accepted payload is older than {{dur $.PayloadStaleAfter}}">⚠ payload {{age $s.PayloadCreated}} old</span>{{end}}
      <span class="badges">
        <span class="pill p-failing">{{index $s.Counts "failing"}} failing</span>
        <span class="pill p-pending-flag">{{index $s.Counts "pending"}} pending</span>
        <span class="pill p-stale">{{index $s.Counts "stale"}} stale</span>
        <span class="pill p-no-run">{{index $s.Counts "no-run"}} no-run</span>
        <span class="pill p-success">{{healthy $s.Images}} healthy</span>
      </span>{{end}}
    </summary>
    <div class="fold-body">
    {{if $s.Error}}<div class="err">{{$s.Error}}</div>{{else}}
    {{if $s.PayloadStale}}<div class="callout red"><b>⚠ No accepted {{$s.Version}} payload for {{age $s.PayloadCreated}}.</b>
      The latest accepted payload is <a href="{{$s.PayloadURL}}">{{$s.Payload}}</a>. Newer successful builds can't ship until a payload is accepted,
      so they pile up as <span class="pill p-pending-flag">pending</span> ({{index $s.Counts "pending"}} images here). This is a payload acceptance problem, not a postsubmit one:
      check the <a href="https://amd64.origin.releases.ci.openshift.org/#{{$s.Stream}}">release controller</a> for rejected payloads.</div>{{end}}
    <p class="muted" style="margin-bottom:10px">Latest accepted payload <a href="{{$s.PayloadURL}}">{{$s.Payload}}</a>{{if not $s.PayloadCreated.IsZero}} ({{age $s.PayloadCreated}} old){{end}}
      (<code>{{$s.Stream}}</code>). "Built" = when the payload's image was built. "Last run" = the job's latest result in GCS.</p>
    <div class="toolbar" data-table="t{{$si}}">
      <input type="search" placeholder="Filter images / repos…" class="q">
      <button class="chip active" data-flag="">All</button>
      <button class="chip" data-flag="failing">Failing</button>
      <button class="chip" data-flag="pending">Pending</button>
      <button class="chip" data-flag="stale">Stale</button>
      <button class="chip" data-flag="no-run">No run</button>
      <button class="chip" data-flag="ok">Healthy</button>
    </div>
    <div class="tablewrap scroll"><table class="sortable" id="t{{$si}}">
    <thead><tr><th>Image</th><th>Repo @ branch</th><th>Built</th><th>Payload commit</th><th>Last run</th><th>Last run commit</th><th>Last run at</th><th>Flags</th></tr></thead>
    <tbody>
    {{range $s.Images}}<tr data-flags="{{if .Flags}}{{join .Flags " "}}{{else}}ok{{end}}" data-q="{{lower .Name}} {{lower .Repo}}">
      <td><code>{{.Name}}</code></td>
      <td>{{if .Repo}}<a href="https://github.com/{{.Org}}/{{.Repo}}/commits/{{.Branch}}">{{.Repo}}</a> <span class="muted">@{{.Branch}}</span>{{if ne .Branch .BuiltBranch}} <span class="muted" title="The payload image was built from {{.BuiltBranch}} (before the {{.Branch}} branch existed). The job now feeding this stream runs on {{.Branch}}.">(built from {{.BuiltBranch}})</span>{{end}}{{if .JobNote}} <span class="pill p-stale" title="{{.JobNote}}">?</span>{{end}}{{end}}</td>
      <td data-v="{{unix .BuiltAt}}" title="{{ts .BuiltAt}}">{{if .ImageError}}<span class="pill p-failure" title="{{.ImageError}}">lookup failed</span>{{else if not .BuiltAt.IsZero}}{{age .BuiltAt}} ago{{else}}<span class="muted">–</span>{{end}}</td>
      <td>{{if .PayloadCommit}}<a href="{{commitURL .Org .Repo .PayloadCommit}}"><code>{{short .PayloadCommit}}</code></a>{{end}}</td>
      <td data-v="{{.LatestRun.Result}}">{{if .LatestRun.Found}}<a href="{{.LatestRun.URL}}"><span class="pill p-{{lower .LatestRun.Result}}">{{.LatestRun.Result}}</span></a>{{else if .Job}}<a href="{{.JobHistoryURL}}"><span class="pill p-none">none in 60d</span></a>{{else}}<span class="muted">no job</span>{{end}}</td>
      <td>{{if .LatestRun.Revision}}<a href="{{commitURL .Org .Repo .LatestRun.Revision}}"><code>{{short .LatestRun.Revision}}</code></a>{{end}}</td>
      <td data-v="{{unix .LatestRun.Finished}}" title="{{ts .LatestRun.Finished}}">{{if .LatestRun.Found}}{{age .LatestRun.Finished}} ago{{end}}</td>
      <td>{{range .Flags}}<span class="pill p-{{.}}{{if eq . "pending"}}-flag{{end}}">{{.}}</span> {{end}}</td>
    </tr>{{end}}
    </tbody></table></div>
    {{end}}
    </div>
  </details>
  {{end}}
</section>

<footer>okd-postsubmit-watcher · data: Prow, GCS test-platform-results-public, quay-proxy.ci.openshift.org, release controller</footer>

<script>
document.querySelectorAll('table.sortable').forEach(function (table) {
  table.querySelectorAll('th').forEach(function (th, idx) {
    th.addEventListener('click', function () {
      var asc = !th.classList.contains('sorted-asc');
      table.querySelectorAll('th').forEach(function (h) { h.classList.remove('sorted-asc', 'sorted-desc'); });
      th.classList.add(asc ? 'sorted-asc' : 'sorted-desc');
      var tbody = table.tBodies[0];
      var rows = Array.prototype.slice.call(tbody.rows);
      rows.sort(function (a, b) {
        var ca = a.cells[idx], cb = b.cells[idx];
        var va = ca.dataset.v !== undefined ? ca.dataset.v : ca.textContent.trim();
        var vb = cb.dataset.v !== undefined ? cb.dataset.v : cb.textContent.trim();
        var na = parseFloat(va), nb = parseFloat(vb);
        var r = (!isNaN(na) && !isNaN(nb)) ? na - nb : va.localeCompare(vb);
        return asc ? r : -r;
      });
      rows.forEach(function (r) { tbody.appendChild(r); });
    });
  });
});
document.querySelectorAll('.toolbar').forEach(function (bar) {
  var table = document.getElementById(bar.dataset.table);
  var q = bar.querySelector('.q'), flag = '';
  function apply() {
    var term = q.value.toLowerCase();
    Array.prototype.forEach.call(table.tBodies[0].rows, function (r) {
      var okFlag = !flag || r.dataset.flags.split(' ').indexOf(flag) >= 0;
      var okQ = !term || r.dataset.q.indexOf(term) >= 0;
      r.style.display = okFlag && okQ ? '' : 'none';
    });
  }
  q.addEventListener('input', apply);
  bar.querySelectorAll('.chip').forEach(function (c) {
    c.addEventListener('click', function () {
      bar.querySelectorAll('.chip').forEach(function (x) { x.classList.remove('active'); });
      c.classList.add('active'); flag = c.dataset.flag; apply();
    });
  });
});
</script>
</body>
</html>
`

$ErrorActionPreference = 'Stop'
$scratch = "C:\Users\WordofLifeMNMedia\.wolm\scratch"

$cat  = Get-Content "$scratch\catalog-20261012.json" -Raw -Encoding UTF8 | ConvertFrom-Json
$vids = Get-Content "$scratch\core-yt-videos.json"   -Raw -Encoding UTF8 | ConvertFrom-Json
$pls  = (Get-Content "$scratch\yt-playlists.json"    -Raw -Encoding UTF8 | ConvertFrom-Json).items

$ytById = @{}; foreach ($v in $vids) { $ytById[$v.id] = $v }

function Norm([string]$s) {
  if (-not $s) { return "" }
  $s = $s.ToLower()
  $s = $s -replace 'c\.o\.r\.e\.?\s*:?\s*', ''
  $s = $s -replace '&', ' and '
  $s = $s -replace '[^a-z0-9]', ''
  return $s
}

# ---- correlate messages ----
$core = @($cat.messages | Where-Object { $_.ministry -like 'core*' -and $_.video.url -and $_.video.url -ne '-' })
$rows = foreach ($m in $core) {
  $vid = ([uri]$m.video.url).AbsolutePath.Trim('/')
  $yt  = $ytById[$vid]
  $d   = [datetime]::ParseExact($m.date, 'yyyy-MM-dd', $null)
  # series this message is a numbered part of
  $seriesName = $null; $track = 0
  if ($m.series) {
    $numbered = @($m.series | Where-Object { $_.index })
    if ($numbered.Count -gt 0) { $seriesName = $numbered[0].Name; $track = $numbered[0].index }
  }
  [pscustomobject]@{
    VideoId     = $vid
    Date        = $m.date
    DateLong    = $d.ToString('MMMM d, yyyy', [Globalization.CultureInfo]::InvariantCulture)
    SheetTitle  = $m.name
    Speaker     = ($m.speakers -join ' & ')
    SheetVis    = $m.visibility
    YtTitle     = $yt.snippet.title
    YtVis       = $yt.status.privacyStatus
    CategoryId  = $yt.snippet.categoryId
    Series      = $seriesName
    Track       = $track
  }
}

# ---- correlate playlists ----
$plByNorm = @{}
foreach ($p in $pls) {
  $k = Norm $p.snippet.title
  if (-not $plByNorm.ContainsKey($k)) { $plByNorm[$k] = @() }
  $plByNorm[$k] += $p
}

$seriesNames = @($rows | Where-Object { $_.Series } | ForEach-Object { $_.Series } | Sort-Object -Unique)
$plRows = foreach ($n in $seriesNames) {
  $k = Norm $n
  # @(...) is required: a one-element array unrolls through `if`, and .Count on a
  # bare PSCustomObject is $null, which silently scores every match as a miss.
  $hit = @(if ($plByNorm.ContainsKey($k)) { $plByNorm[$k] } else { @() })
  $how = if ($hit.Count -ge 1) { 'exact' } else { $null }
  if ($hit.Count -eq 0) {
    # fall back to prefix containment, for cases like series
    # "Grooming: Disarming Manipulative Control" vs playlist "Grooming"
    $cand = @($pls | Where-Object { $k.StartsWith((Norm $_.snippet.title)) -or (Norm $_.snippet.title).StartsWith($k) })
    if ($cand.Count -eq 1) { $hit = $cand; $how = 'prefix' }
    elseif ($cand.Count -gt 1) { $how = "AMBIGUOUS($($cand.Count))" }
  }
  [pscustomobject]@{
    Series       = $n
    Match        = $how
    PlaylistId   = if ($hit.Count -ge 1) { $hit[0].id } else { $null }
    PlaylistName = if ($hit.Count -ge 1) { $hit[0].snippet.title } else { $null }
    PlaylistVis  = if ($hit.Count -ge 1) { $hit[0].status.privacyStatus } else { $null }
    Msgs         = @($rows | Where-Object { $_.Series -eq $n }).Count
  }
}

$rows   | Export-Csv "$scratch\core-correlation.csv"  -NoTypeInformation -Encoding UTF8
$plRows | Export-Csv "$scratch\core-playlists.csv"    -NoTypeInformation -Encoding UTF8

"messages correlated: $($rows.Count)"
"series: $($plRows.Count)   playlist matched: $(@($plRows | Where-Object PlaylistId).Count)   UNMATCHED: $(@($plRows | Where-Object { -not $_.PlaylistId }).Count)"
""
"--- series with no playlist match ---"
$plRows | Where-Object { -not $_.PlaylistId } | ForEach-Object { "  {0}  ({1} msgs)" -f $_.Series, $_.Msgs }
""
"--- visibility mismatches (sheet vs YouTube) ---"
$rows | Where-Object {
  ($_.SheetVis -eq 'public' -and $_.YtVis -ne 'public') -or
  ($_.SheetVis -ne 'public' -and $_.YtVis -eq 'public')
} | ForEach-Object { "  {0}  sheet={1,-8} yt={2,-9} {3}" -f $_.VideoId, $_.SheetVis, $_.YtVis, $_.SheetTitle }
""
"--- messages not in any numbered series ---"
$rows | Where-Object { -not $_.Series } | ForEach-Object { "  {0}  {1}  {2}" -f $_.VideoId, $_.Date, $_.SheetTitle }

$ErrorActionPreference = 'Stop'
$scratch = "C:\Users\WordofLifeMNMedia\.wolm\scratch"
$repo    = "C:\Users\WordofLifeMNMedia\Go\github.com\WordOfLifeMN\online"

$rows  = Import-Csv "$scratch\core-correlation.csv"
$plmap = Import-Csv "$scratch\core-playlists.csv"
$vids  = Get-Content "$scratch\core-yt-videos.json" -Raw -Encoding UTF8 | ConvertFrom-Json
$plsRaw = (Get-Content "$scratch\yt-playlists.json" -Raw -Encoding UTF8 | ConvertFrom-Json).items
$ytById = @{}; foreach ($v in $vids) { $ytById[$v.id] = $v }
$plById = @{}; foreach ($p in $plsRaw) { $plById[$p.id] = $p }

function CleanSeries([string]$n) {
  $n = $n -replace '^\s*C\.O\.R\.E\.?\s*:?\s*', ''
  return $n.TrimEnd('.', ' ')
}
function CleanSpeaker([string]$s) {
  $s = ($s -replace '\s+', ' ').Trim()
  return ($s -replace '^Pastor\s+', '')
}
$MONTHS = 'January|February|March|April|May|June|July|August|September|October|November|December'
function StripTrailingDate([string]$t) {
  return ($t -replace "[,;]?\s*($MONTHS)\s+\d{1,2},?\s*\d{4}\s*$", '').TrimEnd(' ', ',', ';')
}
function Compose([string]$t, [string]$spk, [string]$date) {
  return "CORE | {0} | {1} | {2}" -f (StripTrailingDate $t), (CleanSpeaker $spk), $date
}

$collide = @(
  $rows | Group-Object { Compose $_.SheetTitle $_.Speaker $_.DateLong } |
    Where-Object { $_.Count -gt 1 } | ForEach-Object { $_.Group[0].SheetTitle }
)

$bodyDir = Join-Path $repo 'etc\yt-api\core-retitle'
if (Test-Path $bodyDir) { Remove-Item $bodyDir -Recurse -Force }
New-Item -ItemType Directory -Path "$bodyDir\videos"    -Force | Out-Null
New-Item -ItemType Directory -Path "$bodyDir\playlists" -Force | Out-Null

$L = New-Object System.Collections.Generic.List[string]
function W([string]$t) { $L.Add($t) }

W "# ============================================================================="
W "#  CORE retitle - proposed yt-api calls"
W "#  Generated $(Get-Date -Format 'yyyy-MM-dd HH:mm') from an 'online dump' of the catalog"
W "#  spreadsheet, correlated against live YouTube data fetched the same day."
W "#"
W "#  NOTHING HERE HAS BEEN RUN. Review first; see OPEN QUESTIONS at the bottom."
W "#"
W "#  Scope: 47 CORE messages that have a YouTube video, in 19 series."
W "#  Target video title format : CORE | <title> | <speaker> | <date>"
W "#  Target playlist title     : CORE | <series>"
W "# ============================================================================="
W ""
W "# -- WHY THESE USE --input AND NOT --title -------------------------------------"
W "#  yt-api does NOT read the existing resource before updating. It builds a fresh"
W "#  one from whatever you pass (internal/commands/update.go, updateVideo), and"
W "#  YouTube's videos.update deletes any mutable property the body leaves out. So"
W "#    yt-api update videos --id X --title '...'"
W "#  would blank the description, categoryId, tags and defaultLanguage."
W "#"
W "#  Adding --part snippet does NOT fix it. yt-api always allocates a status"
W "#  object, so the body still carries 'status' while the part list no longer"
W "#  declares it, and the API rejects the call:"
W "#    {\"error\":{\"code\":\"input_error\",\"message\":\"'status'\"}}  Reason: unexpectedPart"
W "#  (Confirmed against video Zq8IdYXb2nM on 2026-10-12. The call was rejected, so"
W "#  nothing was modified.)"
W "#"
W "#  There is also no flag for defaultLanguage, which 46 of the 47 videos have set."
W "#  So each call below sends a complete JSON body instead: the live snippet and"
W "#  status, verbatim, with only the title changed."
W ""
W "# -- --dry-run DOES NOT VALIDATE THESE -----------------------------------------"
W "#  handleUpdateDryRun only echoes the command-line flags; it never opens the"
W "#  --input file and never checks --part. A dry-run of these calls will print"
W "#  empty fields and tell you nothing. That is exactly why the earlier --part"
W "#  snippet form passed its dry-run and still failed for real."
W ""
W "# -- SUGGESTED ORDER OF WORK ---------------------------------------------------"
W "#  1. Run ONE video call. Confirm in YouTube Studio, or re-list it and compare"
W "#     against etc/yt-api/core-retitle/videos/<id>.json - everything except the title"
W "#     should be byte-identical."
W "#  2. Then the remaining videos, then the playlists."
W "#  To run the whole file once you are happy:"
W "#     Get-Content etc/yt-api/core-retitle.yt-api.txt | Where-Object { `$_ -like 'yt-api*' } |"
W "#       ForEach-Object { Invoke-Expression `$_ }"
W ""

function SnippetBody($yt, [string]$newTitle) {
  $s = [ordered]@{ title = $newTitle; description = $yt.snippet.description; categoryId = $yt.snippet.categoryId }
  if ($yt.snippet.tags)                 { $s.tags                 = @($yt.snippet.tags) }
  if ($yt.snippet.defaultLanguage)      { $s.defaultLanguage      = $yt.snippet.defaultLanguage }
  if ($yt.snippet.defaultAudioLanguage) { $s.defaultAudioLanguage = $yt.snippet.defaultAudioLanguage }
  return $s
}
function StatusBody($yt) {
  # uploadStatus is read-only and is deliberately not echoed back
  $st = [ordered]@{ privacyStatus = $yt.status.privacyStatus }
  if ($null -ne $yt.status.license)             { $st.license             = $yt.status.license }
  if ($null -ne $yt.status.embeddable)          { $st.embeddable          = [bool]$yt.status.embeddable }
  if ($null -ne $yt.status.publicStatsViewable) { $st.publicStatsViewable = [bool]$yt.status.publicStatsViewable }
  if ($null -ne $yt.status.selfDeclaredMadeForKids) { $st.selfDeclaredMadeForKids = [bool]$yt.status.selfDeclaredMadeForKids }
  return $st
}

W "# ============================================================================="
W "#  VIDEOS  (47)"
W "# ============================================================================="

$changedTitle = @(); $datedTitles = @()

foreach ($g in ($rows | Group-Object { if ($_.Series) { $_.Series } else { 'zzz~(no series)' } } | Sort-Object Name)) {
  $label = if ($g.Name -eq 'zzz~(no series)') { '(not in a series)' } else { $g.Name }
  W ""
  W "# --- $label ---"
  foreach ($x in ($g.Group | Sort-Object @{e={[int]$_.Track}}, Date, VideoId)) {
    $title = StripTrailingDate $x.SheetTitle
    if ($title -ne $x.SheetTitle) { $datedTitles += $x }
    if ($collide -contains $x.SheetTitle -and [int]$x.Track -gt 0) { $title = "{0}, Part {1}" -f $title, $x.Track }
    $new = "CORE | {0} | {1} | {2}" -f $title, (CleanSpeaker $x.Speaker), $x.DateLong
    $yt  = $ytById[$x.VideoId]

    $body = [ordered]@{ id = $x.VideoId; snippet = (SnippetBody $yt $new); status = (StatusBody $yt) }
    $path = "$bodyDir\videos\$($x.VideoId).json"
    [System.IO.File]::WriteAllText($path, ($body | ConvertTo-Json -Depth 10), (New-Object System.Text.UTF8Encoding $false))

    W "#   now: $($x.YtTitle)"
    W "#   new: $new"
    $stripped = $x.YtTitle -replace '^C\.O\.R\.E\.?\s*(Programs)?\s*:?\s*','' -replace "\s*[;,]\s*($MONTHS).*$",''
    if ((StripTrailingDate $x.SheetTitle) -ne $stripped) {
      W "#   NOTE: title text differs between the spreadsheet and YouTube - see Q1"
      $changedTitle += $x
    }
    if ($x.VideoId -in @('H7cEx5xkqZ0','Ci17d1gF33E')) { W "#   NOTE: spreadsheet date is $($x.DateLong), YouTube title says November 11, 2018 - see Q3" }
    if ($x.SheetVis -ne $x.YtVis) { W "#   NOTE: visibility sheet=$($x.SheetVis) youtube=$($x.YtVis) - see Q4" }
    W "yt-api update videos --input etc/yt-api/core-retitle/videos/$($x.VideoId).json"
    W ""
  }
}

W "# ============================================================================="
W "#  PLAYLISTS  (16 of 19 series have one)"
W "# ============================================================================="
W ""
foreach ($p in ($plmap | Where-Object { $_.PlaylistId } | Sort-Object Series)) {
  $new  = "CORE | " + (CleanSeries $p.Series)
  $live = $plById[$p.PlaylistId]
  $sn = [ordered]@{ title = $new; description = $live.snippet.description }
  if ($live.snippet.defaultLanguage) { $sn.defaultLanguage = $live.snippet.defaultLanguage }
  $body = [ordered]@{ id = $p.PlaylistId; snippet = $sn; status = [ordered]@{ privacyStatus = $live.status.privacyStatus } }
  $path = "$bodyDir\playlists\$($p.PlaylistId).json"
  [System.IO.File]::WriteAllText($path, ($body | ConvertTo-Json -Depth 10), (New-Object System.Text.UTF8Encoding $false))

  W "#   now: $($p.PlaylistName)"
  W "#   new: $new"
  if ($p.Match -eq 'prefix') { W "#   NOTE: matched by prefix - playlist '$($p.PlaylistName)' vs series '$($p.Series)' - see Q2" }
  W "yt-api update playlists --input etc/yt-api/core-retitle/playlists/$($p.PlaylistId).json"
  W ""
}

W "# --- series with NO playlist on the channel -----------------------------------"
foreach ($p in ($plmap | Where-Object { -not $_.PlaylistId } | Sort-Object Series)) {
  W "#   $($p.Series)  ($($p.Msgs) message(s)) - no playlist exists. See Q5."
}
W ""

W "# ============================================================================="
W "#  OPEN QUESTIONS - please confirm before any of the above is run"
W "# ============================================================================="
W "#"
W "#  Q1. WHICH TITLE WINS?  $($changedTitle.Count) of 47 videos have a spreadsheet title that"
W "#      differs from the live YouTube title by more than the 'C.O.R.E.:' prefix"
W "#      and the trailing date. These calls use the SPREADSHEET title, assuming"
W "#      the catalog is the system of record. That is a real content change, not"
W "#      a reformat. The ones that actually matter:"
foreach ($x in $changedTitle) {
  W ("#        {0}`n#           sheet: {1}`n#           yt   : {2}" -f $x.VideoId, $x.SheetTitle, $x.YtTitle)
}
W "#"
W "#  Q2. GROOMING PLAYLIST. The series is 'Grooming: Disarming Manipulative"
W "#      Control' but the playlist is just 'Grooming'. The call renames it to the"
W "#      full series name. Say if you would rather keep 'CORE | Grooming'."
W "#"
W "#  Q3. GROOMING DATE. The spreadsheet dates both parts 2018-11-07; the current"
W "#      YouTube titles say November 11, 2018. The calls use the spreadsheet date."
W "#      One of the two is wrong and I cannot tell which."
W "#"
W "#  Q4. VISIBILITY DRIFT (nothing above changes it, just noticed):"
W "#        L0vzADHkpLA  sheet=public   youtube=private  'Freedom from Shame'"
W "#        m3D5yDaJWH4  sheet=private  youtube=public   'What is Your Achilles Heel?'"
W "#      Each JSON body echoes back the CURRENT YouTube privacy, so running these"
W "#      preserves the drift rather than resolving it."
W "#"
W "#  Q5. MISSING PLAYLISTS. Three series have no playlist: 'ADHD and Bipolar'"
W "#      (2 messages), 'God's Magnetism' (1), 'The Pink Elephant' (1). The two"
W "#      single-message ones probably do not need one. For ADHD and Bipolar:"
W "#        yt-api insert playlists --title 'CORE | ADHD and Bipolar' --privacy public"
W "#      then add Zq8IdYXb2nM and 3V-wFbZSS9c as playlist items."
W "#"
W "#  Q6. SPEAKER NAMING. The spreadsheet has 'Pastor Mary Peltz' on 9 rows and"
W "#      'Mary Peltz' on 35. These use the bare name throughout, to match the"
W "#      current channel style (e.g. 'Learning at Moriah | Mary Peltz | October"
W "#      11, 2026'). 'Dr. Randall Westerberg' is left intact."
W "#"
W "#  Q7. The two 'Fabio and the Mistress' videos currently end '; September 5,"
W "#      2018a' - a stray 'a'. The retitle fixes that incidentally."
if ($datedTitles.Count -gt 0) {
  W "#"
  W "#  Q8. DATE ALREADY IN THE SPREADSHEET TITLE. $($datedTitles.Count) row(s) end in the message"
  W "#      date, which would have put the date in the title twice. The trailing"
  W "#      date was stripped before composing:"
  foreach ($x in $datedTitles) {
    W ("#        {0}  '{1}' -> '{2}'" -f $x.VideoId, $x.SheetTitle, (StripTrailingDate $x.SheetTitle))
  }
  W "#      Worth fixing in the spreadsheet so the title column holds only a title."
}
if ($collide.Count -gt 0) {
  W "#"
  W "#  Q9. DUPLICATE TITLES. These spreadsheet titles repeat within a series AND"
  W "#      share a date, so the composed titles would be byte-identical."
  W "#      ', Part <track>' was appended - a generator invention:"
  foreach ($c in ($collide | Sort-Object -Unique)) { W "#        $c" }
}
W ""

$outFile = Join-Path $repo 'etc\yt-api\core-retitle.yt-api.txt'
[System.IO.File]::WriteAllText($outFile, ($L -join "`r`n"), (New-Object System.Text.UTF8Encoding $false))

"wrote: $outFile"
"video bodies   : $(@(Get-ChildItem "$bodyDir\videos").Count)"
"playlist bodies: $(@(Get-ChildItem "$bodyDir\playlists").Count)"
"calls          : $(@($L | Where-Object { $_ -like 'yt-api*' }).Count)"

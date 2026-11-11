<#
.SYNOPSIS
Turns audio-only messages into YouTube videos: intro, the audio over a still image,
then the outro, with 0.5 second crossfades between them.

.DESCRIPTION
Each .wav (or .mp3/.m4a) becomes an .mp4 of the same name, so a file named like the
sheet row - "2006-03-26 Our Father in Heaven.wav" - gives a video 'online youtube sync'
can match to its row.

Videos are 1280x720 (720p); the 1080p intro and outro are scaled down to match. A
still image needs no more than that for a talk, and encodes far faster.

The image is fitted inside 1280x720 and padded with black, so a portrait image works
but gets bars down the sides; crop it to 16:9 first for a full frame. An image beside
an audio file with the same base name (.jpg or .png) is used for that file instead.

.PARAMETER Audio
An audio file, or a folder of them (not searched below).

.PARAMETER Image
The still image shown while the message plays.

.PARAMETER OutDir
Where the .mp4 files go. Defaults to beside each audio file.

.PARAMETER Normalize
Evens out the message's loudness (EBU R128) to -18 LUFS, the level of the 2026 intro
and outro, for old recordings much quieter or louder than them.

.PARAMETER Force
Replaces an .mp4 that already exists instead of skipping it. A video is written as
"<name>.partial.mp4" and renamed when complete, so a stopped run leaves only a
.partial.mp4 to delete, and the next run starts that video over.

.EXAMPLE
.\wolm-audio-to-video.ps1 -Audio "D:\Audio\Learning How to Pray" -Image "D:\Audio\LHTP.jpg" -Normalize
#>
param(
    [Parameter(Mandatory)] [string] $Audio,
    [Parameter(Mandatory)] [string] $Image,
    [string] $OutDir,
    [string] $Intro = "$HOME\OneDrive - Word of Life Ministries\Documents\WOLMediaFiles\2026+Video\WOL Intro.mp4",
    [string] $Outro = "$HOME\OneDrive - Word of Life Ministries\Documents\WOLMediaFiles\2026+Video\WOL Outro.mp4",
    [switch] $Normalize,
    [switch] $Force
)

$ErrorActionPreference = 'Stop'
$fade = 0.5
$width = 1280
$height = 720
$audioExtensions = '.wav', '.mp3', '.m4a'

# Duration in seconds of a file's first stream of the given type ('v' or 'a')
function Get-Duration([string] $path, [string] $stream) {
    $d = & ffprobe -v error -select_streams "${stream}:0" -show_entries stream=duration -of csv=p=0 $path
    if (-not $d -or $d -eq 'N/A') {
        # some containers (wav) only know the duration of the whole file
        $d = & ffprobe -v error -show_entries format=duration -of csv=p=0 $path
    }
    return [double]::Parse($d.Trim(), [cultureinfo]::InvariantCulture)
}

function Format-Seconds([double] $s) { $s.ToString('0.000', [cultureinfo]::InvariantCulture) }

# fit expression shared by the still and the intro/outro: inside the frame, padded black
$fit = "scale=${width}:${height}:force_original_aspect_ratio=decrease,pad=${width}:${height}:(ow-iw)/2:(oh-ih)/2:black,setsar=1"

# the still is scaled once, here, rather than on every frame of an hour-long video.
# each distinct image is scaled only once per run.
$scaledDir = Join-Path ([IO.Path]::GetTempPath()) "wolm-audio-to-video-$PID"
$scaled = @{}
function Get-ScaledStill([string] $path) {
    if (-not $scaled.ContainsKey($path)) {
        New-Item -ItemType Directory -Force -Path $scaledDir | Out-Null
        $out = Join-Path $scaledDir "still-$($scaled.Count).png"
        & ffmpeg -hide_banner -loglevel error -y -i $path -vf $fit -frames:v 1 $out
        if ($LASTEXITCODE -ne 0) { throw "ffmpeg could not scale $path" }
        $scaled[$path] = $out
    }
    return $scaled[$path]
}

foreach ($path in $Intro, $Outro, $Image) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Cannot find $path" }
}
if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) { throw "ffmpeg is not on the PATH" }

$inputs = @(if (Test-Path -LiteralPath $Audio -PathType Container) {
    Get-ChildItem -LiteralPath $Audio -File | Where-Object { $audioExtensions -contains $_.Extension.ToLower() } | Sort-Object Name
} else {
    Get-Item -LiteralPath $Audio
})
if (-not $inputs) { throw "No audio files in $Audio" }

$introLen = Get-Duration $Intro 'v'
$outroLen = Get-Duration $Outro 'v'

# every piece is brought to the same frame size, rate, pixel format and sound format,
# which xfade and acrossfade need
$video = "$fit,fps=30,format=yuv420p,settb=AVTB"
# the pre-scaled still is already the frame size
$stillVideo = 'setsar=1,fps=30,format=yuv420p,settb=AVTB'
$sound = 'aresample=48000,aformat=sample_fmts=fltp:channel_layouts=stereo'

$made = 0
$count = 0
try {
    foreach ($file in $inputs) {
        $count++
        $dir = if ($OutDir) { $OutDir } else { $file.DirectoryName }
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
        $out = Join-Path $dir ($file.BaseName + '.mp4')
        if ((Test-Path -LiteralPath $out) -and -not $Force) {
            Write-Host "SKIPPED   $($file.Name): $out already exists (use -Force to replace it)"
            continue
        }

        $still = $Image
        foreach ($ext in '.jpg', '.jpeg', '.png') {
            $own = Join-Path $file.DirectoryName ($file.BaseName + $ext)
            if (Test-Path -LiteralPath $own) { $still = $own; break }
        }
        $still = Get-ScaledStill $still

        $body = Get-Duration $file.FullName 'a'
        $level = if ($Normalize) { ',loudnorm=I=-18:TP=-1.5:LRA=11' } else { '' }

        # the intro's and outro's sound is cut or padded to their picture, so the
        # crossfades line up; each fade overlaps the end of one piece with the next
        $graph = @(
            "[0:v]$video[iv]"
            "[1:v]$stillVideo[mv]"
            "[3:v]$video[ov]"
            "[0:a]$sound,apad,atrim=0:$(Format-Seconds $introLen)[ia]"
            "[2:a]$sound$level[ma]"
            "[3:a]$sound,apad,atrim=0:$(Format-Seconds $outroLen)[oa]"
            "[iv][mv]xfade=transition=fade:duration=${fade}:offset=$(Format-Seconds ($introLen - $fade))[v1]"
            "[v1][ov]xfade=transition=fade:duration=${fade}:offset=$(Format-Seconds ($introLen + $body - 2 * $fade)),format=yuv420p[v]"
            "[ia][ma]acrossfade=d=${fade}[a1]"
            "[a1][oa]acrossfade=d=${fade}[a]"
        ) -join ';'

        $total = $introLen + $body + $outroLen - 2 * $fade
        Write-Host ("MAKING    {0}  ({1} of {2}, {3:hh\:mm\:ss} long)" -f
            $file.Name, $count, $inputs.Count, [timespan]::FromSeconds($total))

        # written under another name and renamed once complete, so a run that is stopped
        # part way leaves no unplayable .mp4 for the next run to skip
        $partial = Join-Path $dir ($file.BaseName + '.partial.mp4')
        & ffmpeg -hide_banner -loglevel error -stats -y `
            -i $Intro `
            -loop 1 -framerate 30 -t (Format-Seconds $body) -i $still `
            -i $file.FullName `
            -i $Outro `
            -filter_complex $graph -map '[v]' -map '[a]' `
            -c:v libx264 -preset medium -crf 20 -pix_fmt yuv420p -profile:v high -c:a aac -b:a 192k -movflags +faststart `
            $partial
        if ($LASTEXITCODE -ne 0) { throw "ffmpeg failed on $($file.Name)" }
        Move-Item -LiteralPath $partial -Destination $out -Force

        Write-Host "MADE      $out"
        $made++
    }
} finally {
    if (Test-Path -LiteralPath $scaledDir) { Remove-Item -LiteralPath $scaledDir -Recurse -Force }
}

Write-Host "$made video(s) made"

#!/usr/bin/env bash
# =====================================================================================
# Convert old-style transcripts into the transcripts 'online audio' now keeps.
#
# The old workflow kept transcripts in an 'xscript' folder beside the videos:
#
#     <dir>/xscript/<name>.text      the transcript (.txt in older folders)
#     <dir>/xscript/<name>.vtt       the same, with timings, for publishing online
#
# 'online audio' now keeps one transcript beside each video, with a header line
# recording how it was made:
#
#     <dir>/<name>.txt
#
#     # wolm-transcript app=faster-whisper model=small transcribed=2026-07-26
#
#     <transcript text>
#
# For every xscript folder under the given directories, each <name>.text or
# <name>.txt with a <name>.mp4 beside the folder becomes <name>.txt beside the video,
# dated from the old file's own modification time. The old transcript and its .vtt
# are then deleted, and the xscript folder too once it is empty. Every old transcript
# was made with faster-whisper's small model, so that is what the header records.
#
# Nothing is ever overwritten. An old transcript whose .txt already exists beside the
# video, or that has no video, is reported and left where it is - and so is its
# folder. That includes a name with both a .text and a .txt in xscript: the .text is
# converted, and the .txt is then skipped and kept.
#
# Without --force this is a dry run: it reports what it would do and changes nothing.
#
# Usage:
#     etc/scripts/backfill-xscript.sh [--force] <dir>...
# =====================================================================================

set -u

usage() {
	cat <<EOF
Usage: $(basename "$0") [--force] <dir>...

Converts <dir>/**/xscript/<name>.text (or .txt) into <name>.txt beside <name>.mp4,
then deletes the old transcript and its .vtt, and any xscript folder left empty.

Without --force, only reports what it would do.
EOF
}

force=0
dirs=()
for arg in "$@"; do
	case "$arg" in
	--force) force=1 ;;
	-h | --help)
		usage
		exit 0
		;;
	-*)
		echo "Unknown option: $arg" >&2
		usage >&2
		exit 2
		;;
	*) dirs+=("$arg") ;;
	esac
done

if [ ${#dirs[@]} -eq 0 ]; then
	usage >&2
	exit 2
fi

for dir in "${dirs[@]}"; do
	if [ ! -d "$dir" ]; then
		echo "Not a directory: $dir" >&2
		exit 2
	fi
done

if [ $force -eq 1 ]; then
	would=""
else
	would="would "
	echo "DRY RUN - nothing will be changed. Run again with --force to make these changes."
	echo
fi

converted=0
skipped=0
removed=0
kept=0
failed=0

# nullglob: a folder with no .text yields no iterations rather than a literal '*.text'
# dotglob: a hidden file still counts when deciding whether a folder is empty
shopt -s nullglob dotglob

while IFS= read -r -d '' xdir; do
	parent=$(dirname "$xdir")
	echo "$xdir"

	# what the folder holds before anything is deleted, and how much of that this run
	# deletes, so that a dry run and a real one agree on whether it ends up empty
	entries=("$xdir"/*)
	deleting=0

	# .text first, so a name with both is converted from the .text and its .txt is
	# then skipped by the exists check below, rather than the other way round
	for text in "$xdir"/*.text "$xdir"/*.txt; do
		old=$(basename "$text")
		name="${old%.*}"
		video="$parent/$name.mp4"
		txt="$parent/$name.txt"
		vtt="$xdir/$name.vtt"

		if [ ! -f "$video" ]; then
			echo "  SKIP     $old - no $name.mp4 beside the xscript folder"
			skipped=$((skipped + 1))
			continue
		fi
		if [ -e "$txt" ]; then
			echo "  SKIP     $old - $name.txt already exists beside the video, left as it is"
			skipped=$((skipped + 1))
			continue
		fi
		if [ ! -s "$text" ]; then
			echo "  SKIP     $old - it is empty"
			skipped=$((skipped + 1))
			continue
		fi

		header="# wolm-transcript app=faster-whisper model=small transcribed=$(date -r "$text" +%F)"

		echo "  ${would}create $name.txt  ($header)"
		echo "  ${would}delete xscript/$old"
		these=1
		if [ -e "$vtt" ]; then
			echo "  ${would}delete xscript/$name.vtt"
			these=2
		fi

		if [ $force -eq 1 ]; then
			# written to a .partial and renamed, so an interrupted run never leaves a
			# cut-short .txt that 'online audio' would take as a finished transcript.
			# leading blank lines are dropped: the header is followed by exactly one.
			if ! { printf '%s\n\n' "$header" && sed '/[^[:space:]]/,$!d' "$text"; } >"$txt.partial" ||
				! mv "$txt.partial" "$txt"; then
				echo "  FAILED   could not write $name.txt - the old files are kept" >&2
				rm -f "$txt.partial"
				failed=$((failed + 1))
				continue
			fi

			# the .txt keeps the transcript's age rather than looking brand new
			touch -r "$text" "$txt"

			rm -f "$text" "$vtt"
		fi

		converted=$((converted + 1))
		deleting=$((deleting + these))
	done

	if [ $((${#entries[@]} - deleting)) -le 0 ]; then
		if [ $force -eq 1 ]; then
			if rmdir "$xdir"; then
				echo "  removed  the empty xscript folder"
				removed=$((removed + 1))
			else
				echo "  FAILED   could not remove the xscript folder" >&2
				failed=$((failed + 1))
			fi
		else
			echo "  would remove the xscript folder, which would then be empty"
			removed=$((removed + 1))
		fi
	else
		echo "  keep     the xscript folder - $((${#entries[@]} - deleting)) file(s) ${would}remain"
		kept=$((kept + 1))
	fi
	echo
done < <(find "${dirs[@]}" -type d -name xscript -print0)

if [ $force -eq 1 ]; then
	echo "Transcripts converted: $converted, skipped: $skipped"
	echo "xscript folders removed: $removed, kept: $kept"
else
	echo "Transcripts to convert: $converted, skipped: $skipped"
	echo "xscript folders to remove: $removed, kept: $kept"
fi
if [ $failed -gt 0 ]; then
	echo "FAILURES: $failed - see above"
	exit 1
fi

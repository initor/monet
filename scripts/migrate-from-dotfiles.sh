#!/usr/bin/env bash
# Move a dotfiles install of monet aside, so the monet plugin can take over.
#
# With no option this is a dry run: it prints each step and changes nothing.
# --apply copies the voice personas and the interview-prep data into the
# monet data dir, then moves the old copies under $HOME/.claude into a
# timestamped backup dir. --check exits 1 while an old copy would still load
# next to the plugin. The script never deletes a file, and it never writes
# over a destination that exists.
#
# It runs under bash 3.2 (macOS) and bash 5, so it uses only indexed arrays
# and no builtin that bash 4 added.

# The monet skill dirs of the dotfiles install, under $HOME/.claude/skills,
# sorted. This line is their only list: tests and audits parse it.
MONET_SKILL_DIRS="bq-mock bq-playground code-mock code-playground cwd-context distill-voice gang git-changes git-draft-commit git-draft-pr git-ship hairline-create handoff interview-progress interview-tracker paint pr prep review-code review-local roadmap sd-explainer sd-mock sd-playground teach visual-design voice web-operator"

# The other old copies, relative to $HOME/.claude.
MONET_OTHER_ENTRIES="agents/monet.md commands/monet output-styles/ste-lite.md"

usage() {
  cat <<'EOF'
usage: migrate-from-dotfiles.sh [--apply | --check] [--data-dir DIR] [--backup-dir DIR]

Move a dotfiles install of monet aside, so the monet plugin can take over.
With no option, print each step and change nothing.

options:
  --apply           Copy the personas and the interview-prep data into the
                    data dir, then move the old copies into the backup dir.
  --check           Name each old copy that would still load next to the
                    plugin, and exit 1 while one is left. Exit 0 when none is.
  --data-dir DIR    The monet data dir. Default: $HOME/.config/monet, the
                    data_dir default of the plugin.
  --backup-dir DIR  Where the old copies go. Default:
                    <data dir>/backup/dotfiles-<YYYYmmdd-HHMMSS>.
  --help            Print this help and exit.

It copies, and leaves each source in place:
  $HOME/.claude/skills/voice/reference/personas/*.md to <data dir>/personas/
  $HOME/.claude/skills/voice/reference/personas.md to <data dir>/personas/personas.md
  $HOME/.config/interview-prep/ to <data dir>/interview-prep/

It moves these out of $HOME/.claude into the backup dir, at the same relative
path: agents/monet.md, commands/monet/, output-styles/ste-lite.md, and the
monet skill dirs under skills/.

It never deletes a file. When a destination exists with other bytes, --apply
names each conflict, changes nothing, and exits 1.
EOF
}

die() {
  echo "migrate-from-dotfiles.sh: $1" >&2
  exit 1
}

usage_error() {
  echo "migrate-from-dotfiles.sh: $1" >&2
  echo "Run it with --help for usage." >&2
  exit 2
}

mode=dry
data_dir=
backup_dir=
while [ $# -gt 0 ]; do
  case $1 in
    --apply | --check)
      if [ "$mode" != dry ] && [ "$mode" != "${1#--}" ]; then
        usage_error "--apply and --check do not go together"
      fi
      mode=${1#--}
      ;;
    --data-dir | --backup-dir)
      case ${2:--} in
        -*) usage_error "$1 needs a DIR (write a DIR that starts with - as ./DIR)" ;;
      esac
      if [ "$1" = --data-dir ]; then data_dir=$2; else backup_dir=$2; fi
      shift
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      usage_error "unknown option: $1"
      ;;
  esac
  shift
done

if [ -z "${HOME:-}" ]; then
  usage_error "HOME is not set"
fi

# Print $1 as an absolute path: expand a leading ~, and drop trailing slashes.
absolute() {
  local p=$1
  case $p in
    "~") p=$HOME ;;
    "~/"*) p="$HOME/${p#\~/}" ;;
    /*) ;;
    *) p="$PWD/$p" ;;
  esac
  while [ "${p%/}" != "$p" ] && [ "$p" != / ]; do p=${p%/}; done
  printf '%s\n' "$p"
}

claude="$HOME/.claude"
prep_src="$HOME/.config/interview-prep"
data_dir=$(absolute "${data_dir:-$HOME/.config/monet}")
backup_dir=$(absolute "${backup_dir:-$data_dir/backup/dotfiles-$(date +%Y%m%d-%H%M%S)}")

# Each old copy, relative to $claude, in the order the moves run.
old_entries=$MONET_OTHER_ENTRIES
for name in $MONET_SKILL_DIRS; do old_entries="$old_entries skills/$name"; done

# True when $1 exists, a dangling symlink included.
present() {
  [ -e "$1" ] || [ -L "$1" ]
}

if [ "$mode" = check ]; then
  found=0
  for rel in $old_entries; do
    if present "$claude/$rel"; then
      echo "old copy: $claude/$rel"
      found=$((found + 1))
    fi
  done
  if [ "$found" -gt 0 ]; then
    echo "Old copies that would load next to the plugin: $found. Run with --apply to move them aside."
    exit 1
  fi
  echo "ok: no old copy of monet is left in $claude"
  exit 0
fi

# True when $2 holds what $1 holds: the same symlink target, or the same bytes.
same() {
  if [ -L "$1" ]; then
    [ -L "$2" ] && [ "$(readlink "$1")" = "$(readlink "$2")" ]
  else
    [ ! -L "$2" ] && [ -f "$2" ] && cmp -s "$1" "$2"
  fi
}

copy_src=()
copy_dst=()
move_src=()
move_dst=()
conflicts=()

# True when the nearest ancestor of $1 that exists is not a dir, so mkdir -p
# would fail halfway through the run.
blocked() {
  local d=${1%/*}
  while [ -n "$d" ] && ! present "$d"; do
    case $d in */*) d=${d%/*} ;; *) d= ;; esac
  done
  [ -n "$d" ] && [ ! -d "$d" ]
}

# Plan to copy $1 to $2. A destination with the same content needs no copy.
plan_copy() {
  if [ ! -L "$1" ] && { [ ! -f "$1" ] || [ ! -r "$1" ]; }; then
    conflicts+=("$1 is not a file that can be read, so it cannot be copied")
  elif blocked "$2"; then
    conflicts+=("a parent of $2 is not a dir, so $1 cannot be copied there")
  elif ! present "$2"; then
    copy_src+=("$1")
    copy_dst+=("$2")
  elif ! same "$1" "$2"; then
    conflicts+=("$2 exists and differs from $1")
  fi
}

# The personas and the persona map, to the paths that skills/voice reads.
# A dir on their way that cannot be read hides them from the glob and from
# present, yet skills/voice still moves, so that is a conflict, not a skip.
ref="$claude/skills/voice/reference"
for d in "$claude/skills/voice" "$ref" "$ref/personas"; do
  if [ -d "$d" ] && { [ ! -r "$d" ] || [ ! -x "$d" ]; }; then
    conflicts+=("$d is a dir that cannot be read, so the personas in it cannot be copied")
  fi
done
for f in "$ref"/personas/*.md; do
  if present "$f"; then plan_copy "$f" "$data_dir/personas/${f##*/}"; fi
done
if present "$ref/personas.md"; then
  plan_copy "$ref/personas.md" "$data_dir/personas/personas.md"
fi

# A data dir or backup dir inside the interview-prep source would be copied
# into itself on every run.
for d in "$data_dir" "$backup_dir"; do
  case $d/ in
    "$prep_src"/*) conflicts+=("$d is inside $prep_src, so each run would copy it into itself") ;;
  esac
done

# A data dir inside an old copy would move into the backup with it, and a
# data dir inside a move destination would block that move.
for rel in $old_entries; do
  for d in "$claude/$rel" "$backup_dir/$rel"; do
    case $data_dir/ in
      "$d"/*) conflicts+=("$data_dir is inside $d, where an old copy is or moves to") ;;
    esac
  done
done

# The interview-prep data, file by file, so a conflict names one file.
# A process substitution loses the exit status of find, so a first walk
# catches a dir that cannot be read before it hides files from the copy.
if [ -d "$prep_src" ]; then
  if ! find -H "$prep_src" >/dev/null; then
    conflicts+=("find cannot read every entry under $prep_src (see its errors above)")
  fi
  while IFS= read -r -d '' f; do
    plan_copy "$f" "$data_dir/interview-prep/${f#"$prep_src"/}"
  done < <(find -H "$prep_src" \( -type f -o -type l \) -print0 2>/dev/null | LC_ALL=C sort -z)
fi

# Two copies to one destination are a conflict. Names that differ only in
# case count as one, since the default macOS disk ignores case. One sort of
# the case-folded destinations finds them, so a large dir stays fast.
if [ ${#copy_dst[@]} -gt 1 ]; then
  folded=()
  while IFS= read -r -d '' p; do folded+=("$p"); done \
    < <(printf '%s\0' "${copy_dst[@]}" | LC_ALL=C tr 'A-Z' 'a-z')
  dups=()
  prev=
  last=
  while IFS= read -r -d '' p; do
    if [ "$p" = "$prev" ] && [ "$p" != "$last" ]; then
      dups+=("$p")
      last=$p
    fi
    prev=$p
  done < <(printf '%s\0' "${folded[@]}" | LC_ALL=C sort -z)
  for d in "${dups[@]}"; do
    first=
    i=0
    while [ $i -lt ${#folded[@]} ]; do
      if [ "${folded[$i]}" != "$d" ]; then
        :
      elif [ -z "$first" ]; then
        first=$i
      elif [ "${copy_dst[$first]}" = "${copy_dst[$i]}" ]; then
        conflicts+=("${copy_dst[$i]} is the destination of both ${copy_src[$first]} and ${copy_src[$i]}")
      else
        conflicts+=("${copy_dst[$first]} and ${copy_dst[$i]} differ only in case, so ${copy_src[$first]} and ${copy_src[$i]} cannot both be copied")
      fi
      i=$((i + 1))
    done
  done
fi

for rel in $old_entries; do
  if ! present "$claude/$rel"; then continue; fi
  if present "$backup_dir/$rel"; then
    conflicts+=("$backup_dir/$rel exists, so $claude/$rel cannot move there")
  elif blocked "$backup_dir/$rel"; then
    conflicts+=("a parent of $backup_dir/$rel is not a dir, so $claude/$rel cannot move there")
  else
    move_src+=("$claude/$rel")
    move_dst+=("$backup_dir/$rel")
  fi
done

if [ ${#conflicts[@]} -eq 0 ] && [ ${#copy_src[@]} -eq 0 ] && [ ${#move_src[@]} -eq 0 ]; then
  echo "nothing to do"
  exit 0
fi

if [ "$mode" = dry ]; then
  i=0
  while [ $i -lt ${#copy_src[@]} ]; do
    echo "would copy ${copy_src[$i]} -> ${copy_dst[$i]}"
    i=$((i + 1))
  done
  i=0
  while [ $i -lt ${#move_src[@]} ]; do
    echo "would move ${move_src[$i]} -> ${move_dst[$i]}"
    i=$((i + 1))
  done
fi

if [ ${#conflicts[@]} -gt 0 ]; then
  for c in "${conflicts[@]}"; do echo "conflict: $c" >&2; done
  echo "Nothing changed. Resolve each conflict, then run the script again." >&2
  exit 1
fi

if [ "$mode" = dry ]; then
  echo "Dry run: nothing changed. Run with --apply to make these changes."
  exit 0
fi

# Apply: every copy first, since the personas sit inside an old copy.
i=0
while [ $i -lt ${#copy_src[@]} ]; do
  src=${copy_src[$i]}
  dst=${copy_dst[$i]}
  if present "$dst"; then die "$dst appeared during the run, so it is left as it is. Stopping."; fi
  mkdir -p "${dst%/*}" || die "cannot create ${dst%/*}"
  if [ -L "$src" ]; then
    ln -s -- "$(readlink "$src")" "$dst" || die "cannot copy $src to $dst"
  else
    cp -p "$src" "$dst" || die "cannot copy $src to $dst"
  fi
  echo "copied $src -> $dst"
  i=$((i + 1))
done

# A rerun after a failed move gets a new timestamp, so name the dir to reuse.
move_failed() {
  die "$1. Fix that, then run again with --backup-dir $(printf '%q' "$backup_dir") to keep one backup dir."
}

i=0
while [ $i -lt ${#move_src[@]} ]; do
  src=${move_src[$i]}
  dst=${move_dst[$i]}
  if present "$dst"; then die "$dst appeared during the run, so it is left as it is. Stopping."; fi
  mkdir -p "${dst%/*}" || move_failed "cannot create ${dst%/*}"
  mv "$src" "$dst" || move_failed "cannot move $src to $dst"
  echo "moved $src -> $dst"
  i=$((i + 1))
done

if [ ${#move_src[@]} -gt 0 ]; then
  echo "backup: $backup_dir"
fi

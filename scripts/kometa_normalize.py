#!/usr/bin/env python3
"""Normalize an existing Kometa asset directory in place.

Unlike ``kometa_migrate.py`` (which copies AURA assets from a *source* media library
into a *separate* Kometa asset directory), this script reconciles a single Kometa asset
tree that already contains a mix of naming conventions -- the result of assets written by
both AURA's older "save images locally" flow and its newer Kometa flow into the same
folders. It renames the residual AURA-named files to Kometa names, flattens season
subfolders, and de-duplicates in place.

Naming fixes applied (per Kometa's folder-per-item / asset_folders: true layout):

    backdrop.<ext>                     ->  background.<ext>
    seasonNN-poster.<ext>              ->  SeasonNN.<ext>
    season-specials-poster.<ext>       ->  Season00.<ext>
    <Season NN>/S01E02.<ext>           ->  S01E02.<ext>   (flattened into the show folder)

Files already in Kometa form (poster.*, background.*, SeasonNN.*, S01E01.*) are left in
place; season/episode numbers are re-padded to two digits and folder names are normalized
to their canonical case.

De-duplication. Because both flows wrote into the same folders, several files can map to
one Kometa target (e.g. a folder holding both ``Season01.jpg`` and ``season01-poster.jpg``,
or a title card present both in the show root and in a ``Season 01`` subfolder). For each
target the file with the newest modification time wins and takes the canonical name; every
other file is moved to a quarantine tree (``_aura_conflicts/`` under the root by default)
that mirrors its original path, so nothing is ever deleted and any decision is reversible.
AURA re-touches an asset's mtime every time it applies it, so "newest" approximates the
image currently applied on the media server.

Emptied ``Season NN`` / ``Specials`` subfolders are removed after their title cards move out
(macOS ``.DS_Store`` / ``Thumbs.db`` junk is cleared to allow this).

Left untouched (and reported, never modified): ``*-thumb.jpg`` (Plex auto-generated episode
thumbnails, indistinguishable from AURA's "match"-convention title cards by name) and any
file whose name matches no known pattern (e.g. a stray ``posters.png`` typo).

Runs in dry-run mode unless --apply is given, so you can preview the full plan first.

Examples:
    # Preview every planned rename / flatten / quarantine (nothing is written):
    python3 kometa_normalize.py --root /Volumes/media/assets/kometa

    # Perform the normalization (writes a timestamped log into the quarantine dir):
    python3 kometa_normalize.py --root /Volumes/media/assets/kometa --apply
"""

import argparse
import os
import re
import shutil
import sys
import time

IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".webp"}

# Junk files that should not stop an otherwise-empty season folder from being removed.
JUNK_NAMES = {".DS_Store", "Thumbs.db"}

# A subfolder holding title cards: "Season 01", "Season01", "Season_1", or "Specials".
RE_SEASON_DIR = re.compile(r"^season[ ._-]*(\d{1,3})$", re.IGNORECASE)
RE_SPECIALS_DIR = re.compile(r"^specials$", re.IGNORECASE)

# File name patterns (matched against the base name, without extension; case-insensitive).
RE_POSTER = re.compile(r"^poster$", re.IGNORECASE)
RE_BACKDROP = re.compile(r"^backdrop$", re.IGNORECASE)          # AURA local name -> background
RE_BACKGROUND = re.compile(r"^background$", re.IGNORECASE)      # already Kometa
RE_SEASON_POSTER = re.compile(r"^season(\d{1,3})-poster$", re.IGNORECASE)  # AURA -> SeasonNN
RE_SPECIAL_SEASON = re.compile(r"^season-specials-poster$", re.IGNORECASE)  # AURA -> Season00
RE_KOMETA_SEASON = re.compile(r"^season(\d{1,3})$", re.IGNORECASE)          # already Kometa
RE_TITLECARD = re.compile(r"^s(\d{1,3})e(\d{1,3})$", re.IGNORECASE)         # S01E02
RE_THUMB = re.compile(r"-thumb$", re.IGNORECASE)               # Plex thumbnail -- excluded


def classify(path):
    """Map an image file to its canonical Kometa target.

    Returns one of:
        ("OK", item_dir, key, ext, in_season)  recognized asset; ``key`` is the canonical
                                                base name (no extension), ``item_dir`` the
                                                folder the canonical file belongs in.
        ("THUMB",)                              a *-thumb.jpg to be reported and skipped.
        ("UNREC",)                              an image with an unrecognized name.
        None                                    not an image file.
    """
    directory, filename = os.path.split(path)
    base, ext = os.path.splitext(filename)
    ext_lower = ext.lower()
    if ext_lower not in IMAGE_EXTS:
        return None

    parent = os.path.basename(directory)
    if RE_SEASON_DIR.match(parent) or RE_SPECIALS_DIR.match(parent):
        # Inside a season subfolder only title cards are recognized; anything else is left
        # where it is (and reported) rather than guessed at.
        m = RE_TITLECARD.match(base)
        if m:
            item_dir = os.path.dirname(directory)  # the show folder
            key = "S%02dE%02d" % (int(m.group(1)), int(m.group(2)))
            return ("OK", item_dir, key, ext_lower, True)
        return ("THUMB",) if RE_THUMB.search(base) else ("UNREC",)

    # Directly inside an item (show / movie / collection) folder.
    if RE_THUMB.search(base):
        return ("THUMB",)
    if RE_POSTER.match(base):
        return ("OK", directory, "poster", ext_lower, False)
    if RE_BACKDROP.match(base) or RE_BACKGROUND.match(base):
        return ("OK", directory, "background", ext_lower, False)
    m = RE_SEASON_POSTER.match(base)
    if m:
        return ("OK", directory, "Season%02d" % int(m.group(1)), ext_lower, False)
    if RE_SPECIAL_SEASON.match(base):
        return ("OK", directory, "Season00", ext_lower, False)
    m = RE_KOMETA_SEASON.match(base)
    if m:
        return ("OK", directory, "Season%02d" % int(m.group(1)), ext_lower, False)
    m = RE_TITLECARD.match(base)
    if m:
        return ("OK", directory, "S%02dE%02d" % (int(m.group(1)), int(m.group(2))), ext_lower, False)
    return ("UNREC",)


def scan(root, quarantine_dir):
    """Walk ``root`` and group recognized assets by (item_dir, canonical key)."""
    groups = {}
    thumbs = 0
    unrecognized = []
    for cur, dirs, files in os.walk(root):
        # Never descend into the quarantine tree, so re-runs are idempotent.
        dirs[:] = [d for d in dirs if os.path.abspath(os.path.join(cur, d)) != quarantine_dir]
        for name in files:
            full = os.path.join(cur, name)
            res = classify(full)
            if res is None:
                continue
            if res[0] == "THUMB":
                thumbs += 1
                continue
            if res[0] == "UNREC":
                unrecognized.append(full)
                continue
            _, item_dir, key, ext, in_season = res
            try:
                mtime = os.stat(full).st_mtime
            except OSError:
                mtime = 0.0
            groups.setdefault((item_dir, key), []).append(
                {"src": full, "ext": ext, "mtime": mtime, "in_season": in_season}
            )
    return groups, thumbs, unrecognized


def is_canonical(member, item_dir, key):
    """True when a member already sits at its exact canonical path and name (case included)."""
    return (
        not member["in_season"]
        and os.path.dirname(member["src"]) == item_dir
        and os.path.basename(member["src"]) == key + member["ext"]
    )


def pick_winner(members, item_dir, key):
    """Newest mtime wins; ties break toward the already-canonical file, then path order."""
    return max(
        members,
        key=lambda m: (m["mtime"], is_canonical(m, item_dir, key), m["src"]),
    )


def plan_operations(groups):
    """Turn grouped members into an ordered list of (kind, src, dest) operations.

    ``dest`` is a final path for RENAME/FLATTEN, a quarantine-relative marker for QUARANTINE
    (resolved later), or None for RMDIR. Quarantines for a group are emitted before that
    group's winner move so the canonical path is free when the winner lands.
    """
    ops = []
    season_dirs = set()
    already_ok = 0

    for (item_dir, key), members in sorted(groups.items(), key=lambda kv: (kv[0][0], kv[0][1])):
        winner = pick_winner(members, item_dir, key)
        dest = os.path.join(item_dir, key + winner["ext"])

        for m in members:
            if m["in_season"]:
                season_dirs.add(os.path.dirname(m["src"]))
            if m is winner:
                continue
            ops.append(("QUARANTINE", m["src"], None))

        if is_canonical(winner, item_dir, key) and os.path.abspath(winner["src"]) == os.path.abspath(dest):
            if len(members) == 1:
                already_ok += 1
        else:
            ops.append(("FLATTEN" if winner["in_season"] else "RENAME", winner["src"], dest))

    return ops, season_dirs, already_ok


def predict_rmdirs(season_dirs, ops):
    """Season folders whose only remaining entries (ignoring junk) are files leaving them."""
    leaving = set()
    for kind, src, _dest in ops:
        if kind in ("FLATTEN", "QUARANTINE"):
            leaving.add(os.path.abspath(src))
    rmdirs = []
    leftover = []
    for d in sorted(season_dirs):
        try:
            entries = os.listdir(d)
        except OSError:
            continue
        remaining = []
        for e in entries:
            if e in JUNK_NAMES:
                continue
            if os.path.abspath(os.path.join(d, e)) in leaving:
                continue
            remaining.append(e)
        if remaining:
            leftover.append((d, remaining))
        else:
            rmdirs.append(d)
    return rmdirs, leftover


def unique_path(path):
    """Return ``path`` or the first ``name (N).ext`` variant that does not yet exist."""
    if not os.path.exists(path):
        return path
    base, ext = os.path.splitext(path)
    n = 2
    while os.path.exists("%s (%d)%s" % (base, n, ext)):
        n += 1
    return "%s (%d)%s" % (base, n, ext)


def move_file(src, dest):
    """Move ``src`` to ``dest``, handling a case-only rename on a case-insensitive volume."""
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    if os.path.abspath(src) == os.path.abspath(dest):
        return
    same_dir = os.path.dirname(src) == os.path.dirname(dest)
    a, b = os.path.basename(src), os.path.basename(dest)
    if same_dir and a != b and a.lower() == b.lower():
        # e.g. season01.jpg -> Season01.jpg on APFS/SMB: rename via a temp name.
        tmp = dest + ".knorm-tmp-%d" % os.getpid()
        os.rename(src, tmp)
        os.rename(tmp, dest)
        return
    shutil.move(src, dest)


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Normalize a Kometa asset directory in place (AURA names -> Kometa names, "
        "flatten seasons, de-duplicate by newest mtime with quarantine).",
    )
    parser.add_argument(
        "--root",
        default="/Volumes/media/assets/kometa",
        help="Kometa asset directory to normalize in place (default: %(default)s).",
    )
    parser.add_argument(
        "--quarantine-dir",
        default=None,
        help="Where losing files are moved (default: <root>/_aura_conflicts).",
    )
    parser.add_argument("--apply", action="store_true", help="Perform changes. Without this, runs a dry-run.")
    args = parser.parse_args(argv)

    root = os.path.abspath(args.root)
    if not os.path.isdir(root):
        print("error: --root is not a directory: %s" % root, file=sys.stderr)
        return 2
    quarantine_dir = os.path.abspath(args.quarantine_dir) if args.quarantine_dir else os.path.join(root, "_aura_conflicts")

    groups, thumbs, unrecognized = scan(root, quarantine_dir)
    ops, season_dirs, already_ok = plan_operations(groups)
    rmdirs, leftover = predict_rmdirs(season_dirs, ops)

    log_lines = []

    def emit(line):
        print(line)
        log_lines.append(line)

    renames = flattens = quarantines = 0
    failed = 0

    for kind, src, dest in ops:
        rel_src = os.path.relpath(src, root)
        if kind == "QUARANTINE":
            q_dest = unique_path(os.path.join(quarantine_dir, os.path.relpath(src, root)))
            emit("QUARANTINE  %s  ->  %s" % (rel_src, os.path.relpath(q_dest, root)))
            quarantines += 1
            if args.apply:
                try:
                    move_file(src, q_dest)
                except OSError as exc:
                    emit("  ERROR: %s" % exc)
                    failed += 1
        else:  # RENAME or FLATTEN
            emit("%-9s  %s  ->  %s" % (kind, rel_src, os.path.relpath(dest, root)))
            if kind == "RENAME":
                renames += 1
            else:
                flattens += 1
            if args.apply:
                try:
                    if os.path.exists(dest) and os.path.abspath(dest) != os.path.abspath(src):
                        # Defensive: an occupant that was not a group member. Quarantine it.
                        q_dest = unique_path(os.path.join(quarantine_dir, os.path.relpath(dest, root)))
                        move_file(dest, q_dest)
                        emit("  (moved existing occupant -> %s)" % os.path.relpath(q_dest, root))
                    move_file(src, dest)
                except OSError as exc:
                    emit("  ERROR: %s" % exc)
                    failed += 1

    removed_dirs = 0
    for d in rmdirs:
        emit("RMDIR      %s" % os.path.relpath(d, root))
        if args.apply:
            try:
                for junk in JUNK_NAMES:
                    jp = os.path.join(d, junk)
                    if os.path.exists(jp):
                        os.remove(jp)
                os.rmdir(d)
                removed_dirs += 1
            except OSError as exc:
                emit("  ERROR: %s" % exc)
                failed += 1

    emit("")
    emit("Summary (%s):" % ("APPLIED" if args.apply else "DRY RUN"))
    emit("  recognized assets     : %d" % sum(len(m) for m in groups.values()))
    emit("  already correct       : %d" % already_ok)
    emit("  renames (name fix)    : %d" % renames)
    emit("  flattens (season->root): %d" % flattens)
    emit("  quarantined (dupes)   : %d" % quarantines)
    emit("  season dirs removed   : %d" % (removed_dirs if args.apply else len(rmdirs)))
    if leftover:
        emit("  season dirs kept (leftover files): %d" % len(leftover))
        for d, rem in leftover:
            emit("    %s  (%s)" % (os.path.relpath(d, root), ", ".join(sorted(rem)[:5])))
    if thumbs:
        emit("  excluded -thumb       : %d (Plex auto-generated; not touched)" % thumbs)
    if unrecognized:
        emit("  unrecognized (left)   : %d" % len(unrecognized))
        for u in unrecognized[:10]:
            emit("    %s" % os.path.relpath(u, root))
        if len(unrecognized) > 10:
            emit("    ... and %d more" % (len(unrecognized) - 10))
    if failed:
        emit("  failed operations     : %d" % failed)

    if not args.apply:
        emit("")
        emit("Dry run only. Re-run with --apply to perform the normalization.")
    else:
        os.makedirs(quarantine_dir, exist_ok=True)
        log_path = os.path.join(quarantine_dir, "kometa_normalize_%s.log" % time.strftime("%Y%m%dT%H%M%S"))
        try:
            with open(log_path, "w") as fh:
                fh.write("\n".join(log_lines) + "\n")
            print("\nLog written to %s" % log_path)
        except OSError as exc:
            print("warning: could not write log: %s" % exc, file=sys.stderr)

    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())

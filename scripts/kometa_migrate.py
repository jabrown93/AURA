#!/usr/bin/env python3
"""Migrate AURA "save images locally" assets to the Kometa asset directory layout.

AURA's local-save feature (Images.SaveImagesLocally) writes Plex Local-Media-Assets
names either next to content or under a custom path that mirrors the library structure:

    <item folder>/poster.jpg
    <item folder>/backdrop.jpg
    <show folder>/season01-poster.jpg
    <show folder>/season-specials-poster.jpg
    <show folder>/<Season XX>/<episode base>-thumb.jpg   (episode_naming_convention: match)
    <show folder>/<Season XX>/S01E01.jpg                 (episode_naming_convention: static)

Kometa's asset directory (asset_folders: true) uses a folder per item, named after the
item's media folder, containing:

    <asset dir>/<ASSET_NAME>/poster.<ext>
    <asset dir>/<ASSET_NAME>/background.<ext>
    <asset dir>/<ASSET_NAME>/Season01.<ext>      (Season00 for specials)
    <asset dir>/<ASSET_NAME>/S01E01.<ext>

ASSET_NAME is the exact name of the item's media folder: for posters/backdrops/season
posters it is the file's parent folder; for episode title cards (which AURA stores inside
a season subfolder) it is the show folder (the grandparent of the file).

This script walks a source tree, maps each recognized AURA asset to its Kometa
destination, and copies (default) or moves it. It runs in dry-run mode unless --apply is
given, so you can preview the plan first.

Examples:
    # Preview what would happen (nothing is written):
    python3 kometa_migrate.py --source /data/media --dest /assets

    # Actually copy the assets into the Kometa asset directory:
    python3 kometa_migrate.py --source /data/media --dest /assets --apply

    # Move instead of copy (removes the originals), for a custom SaveImagesLocally path:
    python3 kometa_migrate.py --source /local/aura-images --dest /assets --apply --move
"""

import argparse
import os
import re
import shutil
import sys

IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".webp"}

# AURA local-save file-name patterns (case-insensitive), matched against the base name
# (without extension).
RE_POSTER = re.compile(r"^poster$", re.IGNORECASE)
RE_BACKDROP = re.compile(r"^backdrop$", re.IGNORECASE)
RE_SEASON_POSTER = re.compile(r"^season(\d{1,3})-poster$", re.IGNORECASE)
RE_SPECIAL_SEASON = re.compile(r"^season-specials-poster$", re.IGNORECASE)
# Static title card, e.g. "S01E02"
RE_STATIC_TITLECARD = re.compile(r"^s(\d{1,3})e(\d{1,3})$", re.IGNORECASE)
# "match" title card, e.g. "Show - S01E02 - Title-thumb"; extract SxxEyy anywhere before -thumb.
RE_MATCH_TITLECARD = re.compile(r"^(?P<base>.+)-thumb$", re.IGNORECASE)
RE_SXXEYY_ANYWHERE = re.compile(r"s(\d{1,3})e(\d{1,3})", re.IGNORECASE)


class Plan:
    """A single file's planned migration."""

    def __init__(self, src, asset_name, dest_name):
        self.src = src
        self.asset_name = asset_name
        self.dest_name = dest_name

    def dest_path(self, dest_root):
        return os.path.join(dest_root, self.asset_name, self.dest_name)


def classify(path):
    """Return a Plan for an AURA asset file, or None if the file is not recognized."""
    directory, filename = os.path.split(path)
    base, ext = os.path.splitext(filename)
    ext_lower = ext.lower()
    if ext_lower not in IMAGE_EXTS:
        return None

    parent_name = os.path.basename(directory)
    grandparent_name = os.path.basename(os.path.dirname(directory))

    # Posters / backdrops / season posters live directly in the item folder.
    if RE_POSTER.match(base):
        return Plan(path, parent_name, "poster" + ext_lower)
    if RE_BACKDROP.match(base):
        return Plan(path, parent_name, "background" + ext_lower)

    m = RE_SEASON_POSTER.match(base)
    if m:
        season = int(m.group(1))
        return Plan(path, parent_name, "Season%02d%s" % (season, ext_lower))
    if RE_SPECIAL_SEASON.match(base):
        return Plan(path, parent_name, "Season00" + ext_lower)

    # Title cards live inside a season subfolder; the ASSET_NAME is the show folder.
    m = RE_STATIC_TITLECARD.match(base)
    if m:
        season, episode = int(m.group(1)), int(m.group(2))
        return Plan(path, grandparent_name, "S%02dE%02d%s" % (season, episode, ext_lower))

    m = RE_MATCH_TITLECARD.match(base)
    if m:
        se = RE_SXXEYY_ANYWHERE.search(m.group("base"))
        if se:
            season, episode = int(se.group(1)), int(se.group(2))
            return Plan(path, grandparent_name, "S%02dE%02d%s" % (season, episode, ext_lower))
        # A -thumb file we cannot map (no SxxEyy in the name) is reported by the caller.
        return None

    return None


def build_plans(source):
    plans = []
    unrecognized_thumbs = []
    for root, _dirs, files in os.walk(source):
        for name in files:
            full = os.path.join(root, name)
            plan = classify(full)
            if plan is not None:
                plans.append(plan)
            elif name.lower().endswith(("-thumb.jpg", "-thumb.jpeg", "-thumb.png", "-thumb.webp")):
                unrecognized_thumbs.append(full)
    return plans, unrecognized_thumbs


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Migrate AURA local-save assets to the Kometa asset directory layout.",
    )
    parser.add_argument("--source", required=True, help="Root to scan for AURA assets (media library or SaveImagesLocally path).")
    parser.add_argument("--dest", required=True, help="Kometa asset directory to write folder-per-item assets into.")
    parser.add_argument("--apply", action="store_true", help="Actually copy/move files. Without this, runs a dry-run.")
    parser.add_argument("--move", action="store_true", help="Move files instead of copying them (removes originals).")
    parser.add_argument("--overwrite", action="store_true", help="Overwrite existing files in the destination.")
    args = parser.parse_args(argv)

    source = os.path.abspath(args.source)
    dest = os.path.abspath(args.dest)

    if not os.path.isdir(source):
        print("error: --source is not a directory: %s" % source, file=sys.stderr)
        return 2
    if os.path.commonpath([source, dest]) == source and dest != source:
        # dest inside source would be re-scanned on repeat runs; warn but allow.
        print("warning: --dest is inside --source; re-running may re-scan migrated files", file=sys.stderr)

    plans, unrecognized_thumbs = build_plans(source)

    # Detect destination collisions (two sources mapping to the same target).
    seen = {}
    collisions = []
    for p in plans:
        key = (p.asset_name, p.dest_name)
        if key in seen:
            collisions.append((seen[key], p))
        else:
            seen[key] = p.src

    copied = skipped = failed = 0
    action = "MOVE" if args.move else "COPY"

    for p in plans:
        target = p.dest_path(dest)
        rel_target = os.path.relpath(target, dest)
        if os.path.exists(target) and not args.overwrite:
            print("SKIP (exists)  %s" % rel_target)
            skipped += 1
            continue

        print("%s  %s  ->  %s" % (action, p.src, rel_target))
        if not args.apply:
            continue

        try:
            os.makedirs(os.path.dirname(target), exist_ok=True)
            if args.move:
                shutil.move(p.src, target)
            else:
                shutil.copy2(p.src, target)
            copied += 1
        except OSError as exc:
            print("  ERROR: %s" % exc, file=sys.stderr)
            failed += 1

    print("")
    print("Summary:")
    print("  recognized assets : %d" % len(plans))
    print("  %-16s: %d" % ("applied" if args.apply else "would apply", copied if args.apply else len(plans) - skipped))
    print("  skipped (exists)  : %d" % skipped)
    if failed:
        print("  failed            : %d" % failed)
    if collisions:
        print("  collisions        : %d (multiple sources map to the same asset; last wins)" % len(collisions))
        for a, b in collisions:
            print("    %s  and  %s  ->  %s/%s" % (a, b.src, b.asset_name, b.dest_name))
    if unrecognized_thumbs:
        print("  unmapped -thumb   : %d (no SxxEyy in the file name; skipped)" % len(unrecognized_thumbs))
        for t in unrecognized_thumbs:
            print("    %s" % t)
    if not args.apply:
        print("")
        print("Dry run only. Re-run with --apply to perform the migration.")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Link repository collaboration Skills without replacing installed design Skills."""

import argparse
import os
from pathlib import Path


REPO = Path(__file__).resolve().parent.parent


def skill_sources(repo: Path) -> list[Path]:
    sources = [
        path.resolve()
        for path in sorted((repo / ".agents/skills").glob("easyeda-repo-*"))
        if not path.is_symlink() and path.is_dir() and (path / "SKILL.md").is_file()
    ]
    if not sources:
        raise ValueError("no repository collaboration Skills found")
    return sources


def target_dirs(target: str) -> list[Path]:
    clients = ("agents", "codex", "claude") if target == "all" else (target,)
    return [
        Path(os.environ.get(f"{client.upper()}_HOME") or str(Path.home() / f".{client}")).expanduser() / "skills"
        for client in clients
    ]


def link_plan(sources: list[Path], directories: list[Path]) -> list[tuple[Path, Path, bool]]:
    """Validate every destination before creating any directory or link."""
    plan = []
    seen = set()
    for directory in directories:
        directory = directory.expanduser().absolute()
        parent = directory
        while not parent.exists():
            if parent.is_symlink():
                raise ValueError(f"broken destination directory: {parent}")
            parent = parent.parent
        if not parent.is_dir():
            raise ValueError(f"destination parent is not a directory: {parent}")
        # Resolve parent aliases, not the destination Skill link itself.
        directory = directory.resolve()
        for source in sources:
            destination = directory / source.name
            if destination in seen:
                continue
            seen.add(destination)
            if destination.is_symlink():
                try:
                    existing = destination.resolve(strict=True)
                except (OSError, RuntimeError) as error:
                    raise ValueError(f"broken link preserved: {destination}") from error
                if existing != source:
                    raise ValueError(f"link to another checkout preserved: {destination}")
                plan.append((source, destination, True))
            elif destination.exists():
                raise ValueError(f"existing file or directory preserved: {destination}")
            else:
                plan.append((source, destination, False))
    return plan


def install(plan: list[tuple[Path, Path, bool]], dry_run: bool) -> None:
    created = []
    try:
        for source, destination, exists in plan:
            if exists:
                print(f"already installed: {destination}")
            elif dry_run:
                print(f"would link: {destination} -> {source}")
            else:
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.symlink_to(source, target_is_directory=True)
                created.append((source, destination))
                print(f"linked: {destination} -> {source}")
    except OSError:
        # Undo only links this invocation created and still owns.
        for source, destination in reversed(created):
            if destination.is_symlink() and os.readlink(destination) == str(source):
                destination.unlink()
        raise


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    destination = parser.add_mutually_exclusive_group()
    destination.add_argument("--target", choices=("agents", "codex", "claude", "all"), default="agents")
    destination.add_argument("--skills-dir", type=Path, help="custom user-level Skill discovery directory")
    parser.add_argument("--dry-run", action="store_true", help="validate and print destinations without writing")
    args = parser.parse_args()
    try:
        sources = skill_sources(REPO)
        directories = [args.skills_dir] if args.skills_dir is not None else target_dirs(args.target)
        plan = link_plan(sources, directories)
        install(plan, args.dry_run)
    except (OSError, RuntimeError, ValueError) as error:
        parser.exit(1, f"error: {error}\n")
    print(f"{'Validated' if args.dry_run else 'Installed'} {len(sources)} repository Skills. "
          "Reload your Agent after installation.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

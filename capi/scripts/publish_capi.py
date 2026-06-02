#!/usr/bin/env python3
"""Package the mcp-go C API DLL, public headers, docs, and examples.

The script is intentionally conservative:

* It publishes the documented source headers, not Go's generated c-shared
  header, because the generated header does not carry the integration contract.
* It validates that every C function or callback declaration in the published
  headers has Parameters, Returns, and Boundary cases sections.
* On Windows, it loads the DLL and checks that every public mcpgo_* function
  declared by mcpgo_capi.h is exported.

Example:

    python capi/scripts/publish_capi.py --version 0.1.0
"""

from __future__ import annotations

import argparse
import ctypes
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import zipfile
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable, Optional, Tuple, Union


HEADER_NAMES = ("mcpgo_capi.h", "callback.h")
DOC_FILES = ("c-api.md",)
EXAMPLE_FILES = ("echo_server.c",)
DEFAULT_DLL_NAME = "mcpgo_capi.dll"

DECLARATION_RE = re.compile(
    r"""
    (?P<decl>
        typedef\s+
            (?P<typedef_return>[^;()]+?)\s*
            \(\s*\*\s*(?P<typedef_name>mcpgo_\w+)\s*\)\s*
            \((?P<typedef_params>[^;]*)\)
        |
        (?P<return>[A-Za-z_][\w\s\*]*?)\s+
            (?P<name>mcpgo_\w+)\s*
            \((?P<params>[^;]*)\)
    )
    \s*;
    """,
    re.VERBOSE | re.DOTALL,
)


class PublishError(RuntimeError):
    """Raised when the release package cannot be produced safely."""


def repo_root_from_script() -> Path:
    return Path(__file__).resolve().parents[2]


def run_git(root: Path, args: list[str]) -> Optional[str]:
    try:
        result = subprocess.run(
            ["git", *args],
            cwd=root,
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return None
    value = result.stdout.strip()
    return value or None


def default_version(root: Path) -> str:
    described = run_git(root, ["describe", "--tags", "--always", "--dirty"])
    return sanitize_version(described or "dev")


def sanitize_version(version: str) -> str:
    value = re.sub(r"[^A-Za-z0-9_.+-]+", "-", version.strip())
    return value or "dev"


def resolve_path(root: Path, value: Optional[Union[str, Path]], default: Path) -> Path:
    path = Path(value) if value is not None else default
    if not path.is_absolute():
        path = root / path
    return path.resolve()


def require_file(path: Path, description: str) -> None:
    if not path.is_file():
        raise PublishError(f"{description} not found: {path}")


def extract_param_names(params: str) -> list[str]:
    normalized = params.strip()
    if not normalized or normalized == "void":
        return []

    names: list[str] = []
    for raw_param in normalized.split(","):
        param = " ".join(raw_param.replace("*", " * ").split())
        if not param or param == "void":
            continue
        name = param.split()[-1].lstrip("*")
        name = name.strip()
        if name:
            names.append(name)
    return names


def preceding_block_comment(text: str, start: int) -> str | None:
    prefix = text[:start]
    match = re.search(r"/\*.*?\*/\s*$", prefix, re.DOTALL)
    return match.group(0) if match else None


def mask_comments(text: str) -> str:
    def replace(match: re.Match[str]) -> str:
        return " " * (match.end() - match.start())

    return re.sub(r"/\*.*?\*/", replace, text, flags=re.DOTALL)


def declarations(header_text: str) -> Iterable[tuple[str, str, list[str], int]]:
    searchable = mask_comments(header_text)
    for match in DECLARATION_RE.finditer(searchable):
        name = match.group("typedef_name") or match.group("name")
        return_type = (match.group("typedef_return") or match.group("return") or "").strip()
        params = match.group("typedef_params") or match.group("params") or ""
        yield name, return_type, extract_param_names(params), match.start("decl")


def validate_header_comments(header: Path) -> list[str]:
    text = header.read_text(encoding="utf-8")
    issues: list[str] = []

    for name, _return_type, params, start in declarations(text):
        comment = preceding_block_comment(text, start)
        if comment is None:
            issues.append(f"{header}: {name} is missing a preceding block comment")
            continue

        for section in ("Parameters:", "Returns:", "Boundary cases:"):
            if section not in comment:
                issues.append(f"{header}: {name} comment is missing '{section}'")

        for param in params:
            if not re.search(rf"\b{re.escape(param)}\b", comment):
                issues.append(f"{header}: {name} comment does not document parameter '{param}'")

        if not params and "none" not in comment.lower():
            issues.append(f"{header}: {name} has no parameters but comment does not say 'none'")

    return issues


def public_api_names(header: Path) -> list[str]:
    text = header.read_text(encoding="utf-8")
    names: list[str] = []
    for name, _return_type, _params, _start in declarations(text):
        if name.startswith("mcpgo_call_"):
            continue
        names.append(name)
    return sorted(set(names))


def verify_dll_exports(dll: Path, names: Iterable[str]) -> None:
    if os.name != "nt":
        return

    try:
        library = ctypes.CDLL(str(dll))
    except OSError as exc:
        raise PublishError(f"failed to load DLL {dll}: {exc}") from exc

    missing = [name for name in names if not hasattr(library, name)]
    if missing:
        formatted = ", ".join(missing)
        raise PublishError(f"DLL is missing exported API function(s): {formatted}")


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def copy_file(src: Path, dst: Path) -> None:
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(src, dst)


def render_package_readme(version: str) -> str:
    return f"""# mcp-go C API Release

Version: {version}

## Contents

- `bin/mcpgo_capi.dll`: Windows x64 C shared library.
- `include/mcpgo_capi.h`: Documented public C API header.
- `include/callback.h`: Documented callback type header required by the public API.
- `docs/c-api.md`: Integration guide.
- `examples/echo_server.c`: Minimal C example.
- `manifest.json`: File list, checksums, and export metadata.
- `checksums.txt`: SHA-256 checksums for every packaged file.

## Basic C Build Example

```powershell
gcc examples\\echo_server.c -Iinclude -Lbin -lmcpgo_capi -o echo_server.exe
$env:PATH = \"$PWD\\bin;$env:PATH\"
.\\echo_server.exe
```

All strings returned by the DLL must be released with `mcpgo_free_string()`.
"""


def write_manifest(root: Path, version: str, api_names: list[str], package_files: list[Path]) -> None:
    records = []
    for file_path in sorted(package_files, key=lambda p: p.as_posix()):
        records.append(
            {
                "path": file_path.relative_to(root).as_posix(),
                "size": file_path.stat().st_size,
                "sha256": sha256_file(file_path),
            }
        )

    manifest = {
        "name": "mcpgo-capi",
        "version": version,
        "created_utc": datetime.now(timezone.utc).isoformat(),
        "dll": "bin/mcpgo_capi.dll",
        "headers": [f"include/{name}" for name in HEADER_NAMES],
        "documents": [f"docs/{name}" for name in DOC_FILES],
        "examples": [f"examples/{name}" for name in EXAMPLE_FILES],
        "public_api": api_names,
        "files": records,
    }

    manifest_path = root / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    package_files.append(manifest_path)


def write_checksums(root: Path, package_files: list[Path]) -> None:
    checksum_path = root / "checksums.txt"
    lines = []
    for file_path in sorted(package_files, key=lambda p: p.as_posix()):
        rel = file_path.relative_to(root).as_posix()
        lines.append(f"{sha256_file(file_path)}  {rel}")
    checksum_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    package_files.append(checksum_path)


def create_archive(package_dir: Path) -> Path:
    archive_path = package_dir.with_suffix(".zip")
    if archive_path.exists():
        archive_path.unlink()

    with zipfile.ZipFile(archive_path, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for file_path in sorted(package_dir.rglob("*")):
            if file_path.is_file():
                archive.write(file_path, file_path.relative_to(package_dir.parent))
    return archive_path


def ensure_output_directory(out_dir: Path, clean: bool) -> None:
    if out_dir.exists():
        if not clean:
            raise PublishError(f"output directory already exists; pass --clean to replace it: {out_dir}")
        if out_dir.anchor == str(out_dir):
            raise PublishError(f"refusing to remove filesystem root: {out_dir}")
        shutil.rmtree(out_dir)
    out_dir.mkdir(parents=True, exist_ok=False)


def package_release(args: argparse.Namespace) -> Tuple[Path, Optional[Path]]:
    root = args.root
    capi_dir = root / "capi"
    cmd_dir = capi_dir / "cmd" / "mcpgo-capi"
    docs_dir = capi_dir / "docs"
    examples_dir = capi_dir / "examples"

    dll = resolve_path(root, args.dll, root / "bin" / DEFAULT_DLL_NAME)
    out_dir = resolve_path(root, args.out_dir, root / "bin" / f"mcpgo-capi-{args.version}")

    require_file(dll, "C API DLL")

    header_paths = [cmd_dir / name for name in HEADER_NAMES]
    for header in header_paths:
        require_file(header, "C header")

    if not args.skip_header_validation:
        issues: list[str] = []
        for header in header_paths:
            issues.extend(validate_header_comments(header))
        if issues:
            joined = "\n".join(f"- {issue}" for issue in issues)
            raise PublishError(f"C header documentation validation failed:\n{joined}")

    api_names = public_api_names(cmd_dir / "mcpgo_capi.h")
    if not args.skip_dll_load_check:
        verify_dll_exports(dll, api_names)

    ensure_output_directory(out_dir, args.clean)

    package_files: list[Path] = []
    copy_plan = [
        (dll, out_dir / "bin" / DEFAULT_DLL_NAME),
        (root / "LICENSE", out_dir / "LICENSE"),
    ]
    copy_plan.extend((cmd_dir / name, out_dir / "include" / name) for name in HEADER_NAMES)
    copy_plan.extend((docs_dir / name, out_dir / "docs" / name) for name in DOC_FILES)
    copy_plan.extend((examples_dir / name, out_dir / "examples" / name) for name in EXAMPLE_FILES)

    for src, dst in copy_plan:
        require_file(src, "release input")
        copy_file(src, dst)
        package_files.append(dst)

    readme_path = out_dir / "README.md"
    readme_path.write_text(render_package_readme(args.version), encoding="utf-8")
    package_files.append(readme_path)

    write_manifest(out_dir, args.version, api_names, package_files)
    write_checksums(out_dir, package_files)

    archive_path = create_archive(out_dir) if args.archive else None
    return out_dir, archive_path


def parse_args(argv: list[str]) -> argparse.Namespace:
    root = repo_root_from_script()
    version = default_version(root)

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=root, help="repository root; defaults to this script's repo")
    parser.add_argument("--dll", type=Path, default=None, help="DLL to publish; defaults to bin/mcpgo_capi.dll")
    parser.add_argument(
        "--out-dir",
        type=Path,
        default=None,
        help="package output directory; defaults to bin/mcpgo-capi-<version>",
    )
    parser.add_argument("--version", default=version, help="release version used in package metadata")
    parser.add_argument("--clean", action="store_true", help="replace an existing output directory")
    parser.add_argument("--no-archive", dest="archive", action="store_false", help="do not create a zip archive")
    parser.add_argument(
        "--skip-header-validation",
        action="store_true",
        help="skip Parameters/Returns/Boundary cases validation for C headers",
    )
    parser.add_argument(
        "--skip-dll-load-check",
        action="store_true",
        help="skip Windows DLL load and export verification",
    )
    parser.set_defaults(archive=True)
    args = parser.parse_args(argv)
    args.root = args.root.resolve()
    args.version = sanitize_version(args.version)
    return args


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    try:
        package_dir, archive_path = package_release(args)
    except PublishError as exc:
        print(f"publish failed: {exc}", file=sys.stderr)
        return 1

    print(f"package: {package_dir}")
    if archive_path is not None:
        print(f"archive: {archive_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))

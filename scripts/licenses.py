#!/usr/bin/env python3
"""Collect license files shipped with pinned Go modules for binary distribution."""
import json
import pathlib
import subprocess

root = pathlib.Path(__file__).resolve().parent.parent
raw = subprocess.check_output(["go", "list", "-m", "-json", "all"], cwd=root, text=True)
decoder = json.JSONDecoder()
modules = []
while raw.strip():
    raw = raw.lstrip()
    module, end = decoder.raw_decode(raw)
    modules.append(module)
    raw = raw[end:]
sections = []
missing = []
for module in modules:
    if module.get("Main"):
        continue
    directory = module.get("Replace", {}).get("Dir") or module.get("Dir")
    if not directory:
        continue
    directory = pathlib.Path(directory)
    files = [p for p in directory.iterdir() if p.is_file() and p.name.upper().startswith(("LICENSE", "LICENCE", "COPYING", "NOTICE"))]
    text = "\n\n".join(p.name + "\n" + p.read_text(errors="replace") for p in sorted(files))
    if not text:
        for source in directory.glob("*.go"):
            header = source.read_text(errors="replace").split("package ", 1)[0]
            if "Licensed under the Apache License, Version 2.0" in header:
                text = header + "\n" + (root / "LICENSE").read_text()
                break
    if text:
        sections.append(module["Path"] + " " + module.get("Version", "") + "\n" + text)
    else:
        missing.append(module["Path"])
sections.append("Noto Sans Mono CJK SC\n" + (root / "internal/ui/assets/OFL.txt").read_text())
go_root = pathlib.Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip())
sections.append("Go standard library\n" + (go_root / "LICENSE").read_text())
(root / "docs/THIRD_PARTY_LICENSES.txt").write_text("\n\n".join(sections))
print(f"Collected {len(sections)} license sections")
if missing:
    print("Modules without a root license file:", ", ".join(missing))

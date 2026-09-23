"""Read-only source snapshot guard. Called by desktop before any state writer starts.

Only synthetic fixtures are used by tests. stdout is a status code, never file data.
The parent owns target selection and activation; this helper owns copy/verification.
"""
import ctypes
import hashlib
import json
import os
from pathlib import Path
import shutil
import sqlite3
import sys

TRANSIENT = {"Cache", "Code Cache", "GPUCache", "Crashpad", "DawnCache", "GrShaderCache",
             "ShaderCache", "easy-stock-updater", "mystocktracer-updater", "cashflow-cache",
             "SingletonLock", "SingletonSocket", "SingletonCookie"}


def inventory(root):
    result = {}
    def visit(directory):
        for entry in sorted(directory.iterdir()):
            if directory == root and entry.name in TRANSIENT:
                continue
            if entry.is_symlink() or (hasattr(entry, "is_junction") and entry.is_junction()):
                raise ValueError("unsafe_link")
            stat = entry.lstat()
            if getattr(stat, "st_file_attributes", 0) & 0x400:
                raise ValueError("unsafe_reparse_point")
            rel = entry.relative_to(root).as_posix()
            if entry.is_dir():
                result[rel] = {"type": "directory"}
                visit(entry)
            elif entry.is_file():
                result[rel] = {"type": "file", "size": stat.st_size}
            else:
                raise ValueError("unsupported_state_entry")
    visit(root)
    return result


def digest(file):
    with file.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


class ReadGuard:
    def __init__(self):
        self.handles = []
        if os.name == "nt":
            from ctypes import wintypes
            self.api = ctypes.WinDLL("kernel32", use_last_error=True)
            self.api.CreateFileW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD,
                                            wintypes.LPVOID, wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
            self.api.CreateFileW.restype = wintypes.HANDLE
            self.api.CloseHandle.argtypes = [wintypes.HANDLE]

    def hold(self, file):
        if os.name == "nt":
            # GENERIC_READ + FILE_SHARE_READ: fail while a writer exists and deny new
            # write/delete handles until every verification step has completed.
            handle = self.api.CreateFileW(str(file), 0x80000000, 1, None, 3, 0x02000000, None)
            if handle == ctypes.c_void_p(-1).value:
                raise ValueError("source_busy_or_unreadable")
            self.handles.append(handle)
        else:
            import fcntl
            handle = file.open("rb")
            try:
                fcntl.flock(handle, fcntl.LOCK_SH | fcntl.LOCK_NB)
                fcntl.lockf(handle, fcntl.LOCK_SH | fcntl.LOCK_NB)
            except Exception:
                handle.close()
                raise ValueError("source_busy_or_unreadable") from None
            self.handles.append(handle)

    def close(self):
        for handle in self.handles:
            if os.name == "nt":
                self.api.CloseHandle(handle)
            else:
                handle.close()


def copy_verified(source, target):
    original = inventory(source)
    guard = ReadGuard()
    try:
        # Hold every file throughout copy AND verification, including WAL/SHM/LOCK.
        for rel, record in original.items():
            if record["type"] == "file":
                guard.hold(source / rel)
        if inventory(source) != original:
            raise ValueError("source_changed")
        for rel, record in original.items():
            destination = target / rel
            if record["type"] == "directory":
                destination.mkdir(mode=0o700)
            else:
                record["sha256"] = digest(source / rel)
                with (source / rel).open("rb") as reader, destination.open("xb") as writer:
                    shutil.copyfileobj(reader, writer, 1024 * 1024)
                    writer.flush()
                    os.fsync(writer.fileno())
                destination.chmod(0o600)
                if destination.stat().st_size != record["size"] or digest(destination) != record["sha256"]:
                    raise ValueError("copy_verification_failed")
        current = inventory(source)
        for rel, record in current.items():
            if record["type"] == "file":
                record["sha256"] = digest(source / rel)
        if current != original:
            raise ValueError("source_changed")
        # Validate databases on a disposable validation COPY. Opening SQLite can
        # checkpoint WAL; source and activated bytes must remain exactly copied.
        validation = target.parent / (target.name + ".sqlite-check")
        validation.mkdir(mode=0o700)
        try:
            for rel, record in original.items():
                if record["type"] != "file":
                    continue
                candidate = target / rel
                with candidate.open("rb") as reader:
                    is_sqlite = reader.read(16) == b"SQLite format 3\x00"
                if not is_sqlite:
                    continue
                check = validation / "database"
                check.mkdir()
                for suffix in ("", "-wal", "-shm", "-journal"):
                    companion = Path(str(candidate) + suffix)
                    if companion.is_file():
                        shutil.copyfile(companion, check / ("data.db" + suffix))
                connection = sqlite3.connect(str(check / "data.db"))
                try:
                    if connection.execute("PRAGMA integrity_check").fetchall() != [("ok",)]:
                        raise ValueError("sqlite_verification_failed")
                finally:
                    connection.close()
                shutil.rmtree(check)
        finally:
            shutil.rmtree(validation)
        # Fail if data changed during SQLite validation or new files appeared.
        final = inventory(source)
        for rel, record in final.items():
            if record["type"] == "file":
                record["sha256"] = digest(source / rel)
        if final != original:
            raise ValueError("source_changed")
        return [{"path": rel, **record} for rel, record in original.items()]
    finally:
        guard.close()


if __name__ == "__main__":
    try:
        source, target = (Path(value).absolute() for value in sys.argv[1:3])
        if source == target or source in target.parents or target in source.parents:
            raise ValueError("overlapping_paths")
        files = copy_verified(source, target)
        with (target / ".snapshot-inventory.json").open("x", encoding="utf-8") as manifest:
            manifest.write(json.dumps(files))
        print("verified")
    except Exception:
        # Exception paths/values can contain private user names or secret filenames.
        print("state_copy_failed: close the previous app and preserve the source", file=sys.stderr)
        sys.exit(1)

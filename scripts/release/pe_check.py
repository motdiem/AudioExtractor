"""Checks a Windows exe: PE32+ x86-64 and a resource directory with an icon.

usage: python3 pe_check.py AudioExtractor.exe
Exit 0 and print one summary line, or exit 1 with the reason.
"""
import struct
import sys

RT_ICON, RT_GROUP_ICON = 3, 14


def fail(msg):
    print(f"pe_check: {msg}", file=sys.stderr)
    sys.exit(1)


def main(path):
    d = open(path, "rb").read()
    if d[:2] != b"MZ":
        fail("not an MZ executable")
    pe = struct.unpack_from("<I", d, 0x3C)[0]
    if d[pe:pe + 4] != b"PE\0\0":
        fail("no PE header")
    machine, nsec = struct.unpack_from("<HH", d, pe + 4)
    optsize = struct.unpack_from("<H", d, pe + 20)[0]
    opt = pe + 24
    magic = struct.unpack_from("<H", d, opt)[0]
    if machine != 0x8664:
        fail(f"machine {machine:#x}, expected x86-64 (0x8664)")
    if magic != 0x20B:
        fail(f"optional header magic {magic:#x}, expected PE32+ (0x20b)")
    subsystem = struct.unpack_from("<H", d, opt + 68)[0]
    # Data directory 2 is the resource table.
    rsrc_rva = struct.unpack_from("<II", d, opt + 112 + 2 * 8)[0]
    if not rsrc_rva:
        fail("no resource directory (no icon)")

    sec = opt + optsize
    sections = [struct.unpack_from("<8sIIII", d, sec + i * 40) for i in range(nsec)]

    def off(rva):
        for _, vsize, va, rawsize, raw in sections:
            if va <= rva < va + max(vsize, rawsize):
                return raw + rva - va
        fail("resource RVA outside every section")

    base = off(rsrc_rva)

    def entries(dir_off):
        named, ids = struct.unpack_from("<HH", d, dir_off + 12)
        for i in range(named + ids):
            yield struct.unpack_from("<II", d, dir_off + 16 + i * 8)

    types = dict(entries(base))
    if RT_GROUP_ICON not in types or RT_ICON not in types:
        fail("resources hold no icon (RT_GROUP_ICON/RT_ICON missing)")
    # Count the images of the first icon group: the sizes Explorer can pick from.
    group = types[RT_GROUP_ICON] & 0x7FFFFFFF
    _, first = next(iter(entries(base + group)))
    _, leaf = next(iter(entries(base + (first & 0x7FFFFFFF))))
    rva = struct.unpack_from("<I", d, base + leaf)[0]
    count = struct.unpack_from("<H", d, off(rva) + 4)[0]
    gui = "GUI" if subsystem == 2 else f"subsystem {subsystem}"
    print(f"PE32+ x86-64 ({gui}), icon group with {count} images")


main(sys.argv[1])

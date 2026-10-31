# Test coverage report: toybox + uutils utilities missing from Misa

Generated 2026-10-15 from the same source checkouts as `utility-comparison.md` (toybox 6a0c0ef, uutils repos at the commits listed there). BusyBox excluded.

## Method

- Candidates: every utility in toybox (any category) or any uutils repo that is not in `misa --list`. toybox sh builtins declared in toys/pending/sh.c (cd, export, set, …; 28 names) are excluded since they belong to Misa's shell, not its builtin registry.
- toybox cases: lines starting with `testing` / `testcmd` (optionally prefixed by `toyonly` or an env assignment) in toybox/tests/<util>.test. These are shell-based and run unchanged by Misa's tests/toybox adapter once the .test file is copied in.
- uutils cases: `#[test]` fns in <repo>/tests/by-util/test_<util>.rs (findutils: tests/test_find.rs + find_exec_tests.rs, test_xargs.rs, db_tests.rs; grep: tests/test_grep.rs; awk: tests/awk.rs + by-util). Rust-specific; Misa's tests/uutils adapter only runs statically parseable `new_ucmd!()` chains, so a `simple` count is given (body uses new_ucmd! and no `at.`/TestScenario/#[cfg]/root helpers). That is a heuristic upper bound on what the adapter will accept.
- Needs flags: regex hits (≥2) in the test text for root, devices, network, kernel/fs (/proc, /sys, mount, selinux, xattr), tty. Heuristic; read before trusting.
- Weight: toybox source LOC <200 light, <600 medium, else heavy; if no toybox impl, smallest uutils src LOC <500 light, <1500 medium, else heavy. Bumped one step for toybox android/net categories or tests needing root/devices.
- Portable score = toybox cases (if not flagged root/devices/network/kernel) + uutils simple cases.

## Totals

| Metric | Value |
|---|---|
| Missing utilities (candidates) | 300 |
| Weight light / medium / heavy | 141 / 92 / 67 |
| With toybox tests | 61 utilities, 1337 cases |
| With uutils tests | 109 utilities, 3010 #[test] fns (1504 simple) |
| With any tests | 145 |
| With no tests anywhere | 155 |
| Light with portable tests | 64 |

## Ranked low-hanging fruit (light, portable tests)

| # | Utility | Portable cases | toybox cases | uutils #[test] (simple) | toybox LOC | Needs |
|---|---|---|---|---|---|---|
| 1 | `shuf` | 64 | – | 78 (64) | 63 | – |
| 2 | `df` | 63 | – | 67 (63) | 197 | kernel/fs, tty |
| 3 | `basenc` | 35 | – | 38 (35) | – | – |
| 4 | `hexdump` | 30 | 14 | 22 (16) | 146 | kernel/fs |
| 5 | `timeout` | 28 | 16 | 37 (12) | 107 | – |
| 6 | `devmem` | 26 | 26 | – | 103 | – |
| 7 | `hugetop` | 24 | – | 24 (24) | – | – |
| 8 | `xxd` | 22 | 22 | – | 182 | – |
| 9 | `pidof` | 17 | 5 | 12 (12) | 45 | – |
| 10 | `mkfifo` | 15 | 5 | 16 (10) | 40 | kernel/fs |
| 11 | `pmap` | 14 | – | 21 (14) | 103 | – |
| 12 | `vmstat` | 14 | – | 14 (14) | 155 | – |
| 13 | `sync` | 13 | – | 17 (13) | 23 | – |
| 14 | `uptime` | 13 | 2 | 16 (11) | 79 | – |
| 15 | `free` | 12 | – | 12 (12) | 58 | – |
| 16 | `cal` | 12 | – | 12 (12) | 144 | – |
| 17 | `gzip` | 11 | 11 | – | 179 | – |
| 18 | `dmesg` | 11 | – | 17 (11) | 187 | – |
| 19 | `mcookie` | 10 | – | 10 (10) | 35 | – |
| 20 | `sha224sum` | 10 | 8 | 8 (2) | 142 | – |
| 21 | `sha384sum` | 10 | 8 | 8 (2) | 142 | – |
| 22 | `last` | 10 | – | 12 (10) | 196 | – |
| 23 | `uuencode` | 9 | 9 | – | 59 | – |
| 24 | `uudecode` | 9 | 9 | – | 113 | – |
| 25 | `sysctl` | 9 | – | 9 (9) | 149 | – |
| 26 | `uuidgen` | 8 | 1 | 7 (7) | 25 | kernel/fs |
| 27 | `pwdx` | 8 | – | 8 (8) | 33 | – |
| 28 | `setsid` | 8 | – | 11 (8) | 58 | kernel/fs, tty |
| 29 | `sha3sum` | 8 | 8 | – | 105 | – |
| 30 | `sum` | 8 | – | 13 (8) | – | kernel/fs, network |
| 31 | `unicode` | 7 | 7 | – | 86 | – |
| 32 | `link` | 6 | 4 | 8 (2) | 25 | – |
| 33 | `gunzip` | 6 | 6 | – | 179 | – |
| 34 | `watch` | 6 | – | 6 (6) | 186 | – |
| 35 | `dir` | 6 | – | 12 (6) | – | tty |
| 36 | `vdir` | 6 | – | 12 (6) | – | tty |
| 37 | `fsfreeze` | 5 | – | 5 (5) | 30 | – |
| 38 | `w` | 5 | – | 5 (5) | 33 | – |
| 39 | `iconv` | 5 | 5 | – | 75 | – |
| 40 | `mkpasswd` | 5 | 5 | – | 81 | – |
| 41 | `modinfo` | 5 | 5 | – | 129 | – |
| 42 | `renice` | 4 | 2 | 2 (2) | 51 | – |
| 43 | `shred` | 4 | – | 31 (4) | 99 | kernel/fs, tty |
| 44 | `more` | 4 | 2 | 11 (2) | 172 | – |
| 45 | `zcat` | 4 | 4 | – | 179 | – |
| 46 | `pidwait` | 4 | – | 4 (4) | – | – |
| 47 | `setpgid` | 4 | – | 5 (4) | – | – |
| 48 | `users` | 4 | – | 5 (4) | – | – |
| 49 | `fallocate` | 3 | 3 | – | 33 | – |
| 50 | `getopt` | 3 | 3 | – | 112 | – |
| 51 | `hostid` | 3 | – | 3 (3) | – | – |
| 52 | `mesg` | 3 | – | 3 (3) | – | tty |
| 53 | `nologin` | 2 | – | 2 (2) | 24 | – |
| 54 | `unlink` | 2 | – | 7 (2) | 24 | – |
| 55 | `who` | 2 | – | 27 (2) | 44 | – |
| 56 | `killall` | 2 | 2 | – | 123 | – |
| 57 | `b2sum` | 2 | – | 15 (2) | – | – |
| 58 | `getfacl` | 2 | – | 5 (2) | – | – |
| 59 | `mountpoint` | 1 | – | 1 (1) | 62 | – |
| 60 | `chacl` | 1 | – | 1 (1) | – | – |
| 61 | `ctrlaltdel` | 1 | – | 1 (1) | – | – |
| 62 | `setfacl` | 1 | – | 1 (1) | – | – |
| 63 | `skill` | 1 | – | 1 (1) | – | – |
| 64 | `tload` | 1 | – | 1 (1) | – | – |

## All missing utilities

| Utility | toybox (category) | toybox cases | uutils repo:#[test] | uutils simple | Needs | Portability | toybox LOC | uutils LOC | Weight |
|---|---|---|---|---|---|---|---|---|---|
| `:` | posix | – | – | – | – | no tests | 24 | – | light |
| `[` | posix | – | – | – | – | no tests | 209 | – | medium |
| `[[` | pending | – | – | – | – | no tests | 209 | – | medium |
| `acpi` | other | – | – | – | – | no tests | 135 | – | light |
| `addgroup` | pending | – | – | – | – | no tests | 91 | – | light |
| `adduser` | pending | – | – | – | – | no tests | 159 | – | light |
| `arp` | pending | – | – | – | – | no tests | 289 | – | medium |
| `arping` | pending | – | – | – | – | no tests | 288 | – | medium |
| `ascii` | other | – | – | – | – | no tests | 86 | – | light |
| `awk` | pending | 142 | awk:73 | 0 | root | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 0/73 simple new_ucmd! cases parseable by tests/uutils adapter | 4579 | – | heavy |
| `b2sum` | – | – | coreutils:15 | 2 | – | uutils: Rust, 2/15 simple new_ucmd! cases parseable by tests/uutils adapter | – | 39 | light |
| `basenc` | – | – | coreutils:38 | 35 | – | uutils: Rust, 35/38 simple new_ucmd! cases parseable by tests/uutils adapter | – | 95 | light |
| `bc` | pending | 2 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 5953 | – | heavy |
| `blkdiscard` | other | – | – | – | – | no tests | 54 | – | light |
| `blkid` | other | 16 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 259 | – | medium |
| `blockdev` | other | – | util-linux:5 | 5 | devices | uutils: Rust, 5/5 simple new_ucmd! cases parseable by tests/uutils adapter | 69 | 433 | medium |
| `bootchartd` | pending | – | – | – | – | no tests | 277 | – | medium |
| `brctl` | pending | – | – | – | – | no tests | 333 | – | medium |
| `bunzip2` | other | – | – | – | – | no tests | 729 | – | heavy |
| `bzcat` | other | 4 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 729 | – | heavy |
| `cal` | posix | – | util-linux:12 | 12 | – | uutils: Rust, 12/12 simple new_ucmd! cases parseable by tests/uutils adapter | 144 | 355 | light |
| `chacl` | – | – | acl:1 | 1 | – | uutils: Rust, 1/1 simple new_ucmd! cases parseable by tests/uutils adapter | – | 73 | light |
| `chage` | – | – | shadow:14 | 0 | root | uutils: Rust, 0/14 simple new_ucmd! cases parseable by tests/uutils adapter | – | 946 | heavy |
| `chattr` | other | 7 | – | – | kernel/fs, root | toybox: shell .test (runs in tests/toybox adapter) | 337 | – | heavy |
| `chcon` | other | – | coreutils:24 | 4 | kernel/fs, root | uutils: Rust, 4/24 simple new_ucmd! cases parseable by tests/uutils adapter | 44 | 988 | medium |
| `chcpu` | – | – | util-linux:10 | 10 | kernel/fs | uutils: Rust, 10/10 simple new_ucmd! cases parseable by tests/uutils adapter | – | 704 | medium |
| `chfn` | – | – | shadow:4 | 0 | root | uutils: Rust, 0/4 simple new_ucmd! cases parseable by tests/uutils adapter | – | 695 | heavy |
| `chgpasswd` | – | – | shadow:18 | 0 | – | uutils: Rust, 0/18 simple new_ucmd! cases parseable by tests/uutils adapter | – | 733 | medium |
| `chgrp` | posix | 24 | coreutils:32 | 18 | kernel/fs, root | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 18/32 simple new_ucmd! cases parseable by tests/uutils adapter | 100 | 186 | medium |
| `chpasswd` | – | – | shadow:15 | 0 | – | uutils: Rust, 0/15 simple new_ucmd! cases parseable by tests/uutils adapter | – | 822 | medium |
| `chroot` | other | – | coreutils:21 | 3 | root | uutils: Rust, 3/21 simple new_ucmd! cases parseable by tests/uutils adapter | 33 | 528 | medium |
| `chrt` | other | – | – | – | – | no tests | 96 | – | light |
| `chsh` | pending | – | shadow:5 | 0 | root | uutils: Rust, 0/5 simple new_ucmd! cases parseable by tests/uutils adapter | 69 | 599 | medium |
| `chvt` | other | – | – | – | – | no tests | 115 | – | light |
| `clear` | other | – | – | – | – | no tests | 19 | – | light |
| `count` | other | – | – | – | – | no tests | 85 | – | light |
| `cpio` | posix | 21 | – | – | root | toybox: shell .test (runs in tests/toybox adapter) | 324 | – | heavy |
| `crc32` | posix | – | – | – | – | no tests | 77 | – | light |
| `crond` | pending | – | – | – | – | no tests | 672 | – | heavy |
| `crontab` | pending | – | – | – | – | no tests | 366 | – | medium |
| `csplit` | pending | – | coreutils:105 | 8 | root | uutils: Rust, 8/105 simple new_ucmd! cases parseable by tests/uutils adapter | 173 | 1939 | medium |
| `ctrlaltdel` | – | – | util-linux:1 | 1 | – | uutils: Rust, 1/1 simple new_ucmd! cases parseable by tests/uutils adapter | – | 140 | light |
| `dd` | posix | 51 | coreutils:165 | 114 | devices, kernel/fs, root, tty | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 114/165 simple new_ucmd! cases parseable by tests/uutils adapter | 292 | 5023 | heavy |
| `deallocvt` | other | – | – | – | – | no tests | 115 | – | light |
| `delgroup` | pending | – | – | – | – | no tests | 61 | – | light |
| `deluser` | pending | – | – | – | – | no tests | 114 | – | light |
| `devmem` | other | 26 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 103 | – | light |
| `df` | posix | – | coreutils:67 | 63 | kernel/fs, tty | uutils: Rust, 63/67 simple new_ucmd! cases parseable by tests/uutils adapter | 197 | 3345 | light |
| `dhcp` | pending | – | – | – | – | no tests | 1513 | – | heavy |
| `dhcp6` | pending | – | – | – | – | no tests | 686 | – | heavy |
| `dhcpd` | pending | – | – | – | – | no tests | 2049 | – | heavy |
| `dir` | – | – | coreutils:12 | 6 | tty | uutils: Rust, 6/12 simple new_ucmd! cases parseable by tests/uutils adapter | – | 44 | light |
| `dircolors` | – | – | coreutils:20 | 17 | – | uutils: Rust, 17/20 simple new_ucmd! cases parseable by tests/uutils adapter | – | 554 | medium |
| `dmesg` | lsb | – | util-linux:17 | 11 | – | uutils: Rust, 11/17 simple new_ucmd! cases parseable by tests/uutils adapter | 187 | 851 | light |
| `dnsdomainname` | lsb | – | – | – | – | no tests | 77 | – | light |
| `dos2unix` | other | – | – | – | – | no tests | 76 | – | light |
| `dumpleases` | pending | – | – | – | – | no tests | 77 | – | light |
| `egrep` | posix | – | – | – | – | no tests | 548 | – | medium |
| `eject` | other | – | – | – | – | no tests | 66 | – | light |
| `expiry` | – | – | shadow:7 | 0 | root | uutils: Rust, 0/7 simple new_ucmd! cases parseable by tests/uutils adapter | – | 322 | medium |
| `fallocate` | other | 3 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 33 | – | light |
| `fdisk` | pending | – | – | – | – | no tests | 1557 | – | heavy |
| `fgrep` | posix | – | – | – | – | no tests | 548 | – | medium |
| `file` | posix | 25 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 549 | – | medium |
| `flock` | other | – | – | – | – | no tests | 39 | – | light |
| `free` | other | – | procps:12 | 12 | – | uutils: Rust, 12/12 simple new_ucmd! cases parseable by tests/uutils adapter | 58 | 641 | light |
| `freeramdisk` | other | – | – | – | – | no tests | 23 | – | light |
| `fsck` | pending | – | – | – | – | no tests | 434 | – | medium |
| `fsfreeze` | other | – | util-linux:5 | 5 | – | uutils: Rust, 5/5 simple new_ucmd! cases parseable by tests/uutils adapter | 30 | 86 | light |
| `fstype` | other | 4 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 259 | – | medium |
| `fsync` | other | – | – | – | – | no tests | 32 | – | light |
| `ftpget` | net | – | – | – | – | no tests | 202 | – | heavy |
| `ftpput` | net | – | – | – | – | no tests | 202 | – | heavy |
| `getconf` | posix | – | – | – | – | no tests | 250 | – | medium |
| `getenforce` | android | – | – | – | – | no tests | 29 | – | medium |
| `getfacl` | – | – | acl:5 | 2 | – | uutils: Rust, 2/5 simple new_ucmd! cases parseable by tests/uutils adapter | – | 228 | light |
| `getfattr` | pending | 5 | – | – | kernel/fs | toybox: shell .test (runs in tests/toybox adapter) | 98 | – | light |
| `getopt` | other | 3 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 112 | – | light |
| `getty` | pending | – | – | – | – | no tests | 259 | – | medium |
| `gitcheckout` | pending | – | – | – | – | no tests | 656 | – | heavy |
| `gitclone` | pending | – | – | – | – | no tests | 656 | – | heavy |
| `gitfetch` | pending | – | – | – | – | no tests | 656 | – | heavy |
| `gitinit` | pending | – | – | – | – | no tests | 656 | – | heavy |
| `gitremote` | pending | – | – | – | – | no tests | 656 | – | heavy |
| `gpasswd` | – | – | shadow:24 | 0 | root | uutils: Rust, 0/24 simple new_ucmd! cases parseable by tests/uutils adapter | – | 806 | heavy |
| `gpiodetect` | other | – | – | – | – | no tests | 254 | – | medium |
| `gpiofind` | other | – | – | – | – | no tests | 254 | – | medium |
| `gpioget` | other | – | – | – | – | no tests | 254 | – | medium |
| `gpioinfo` | other | – | – | – | – | no tests | 254 | – | medium |
| `gpioset` | other | – | – | – | – | no tests | 254 | – | medium |
| `grep` | posix | 116 | grep:102 | 0 | tty | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 0/102 simple new_ucmd! cases parseable by tests/uutils adapter | 548 | – | medium |
| `groupadd` | pending | 10 | shadow:15 | 0 | root | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 0/15 simple new_ucmd! cases parseable by tests/uutils adapter | 91 | 542 | medium |
| `groupdel` | pending | 5 | shadow:11 | 0 | root | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 0/11 simple new_ucmd! cases parseable by tests/uutils adapter | 61 | 331 | medium |
| `groupmod` | – | – | shadow:13 | 0 | root | uutils: Rust, 0/13 simple new_ucmd! cases parseable by tests/uutils adapter | – | 525 | heavy |
| `grpck` | – | – | shadow:16 | 0 | root | uutils: Rust, 0/16 simple new_ucmd! cases parseable by tests/uutils adapter | – | 620 | heavy |
| `grpconv` | – | – | shadow:– | – | – | no tests | – | 42 | light |
| `grpunconv` | – | – | shadow:– | – | – | no tests | – | 42 | light |
| `gunzip` | lsb | 6 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 179 | – | light |
| `gzip` | lsb | 11 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 179 | – | light |
| `halt` | other | – | – | – | – | no tests | 46 | – | light |
| `hd` | other | – | – | – | – | no tests | 62 | – | light |
| `help` | other | – | – | – | – | no tests | 89 | – | light |
| `hexdump` | pending | 14 | util-linux:22 | 16 | kernel/fs | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 16/22 simple new_ucmd! cases parseable by tests/uutils adapter | 146 | 475 | light |
| `hexedit` | other | – | – | – | – | no tests | 370 | – | medium |
| `host` | net | – | – | – | – | no tests | 201 | – | heavy |
| `hostid` | – | – | coreutils:3 | 3 | – | uutils: Rust, 3/3 simple new_ucmd! cases parseable by tests/uutils adapter | – | 87 | light |
| `httpd` | net | 2 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 201 | – | heavy |
| `hugetop` | – | – | procps:24 | 24 | – | uutils: Rust, 24/24 simple new_ucmd! cases parseable by tests/uutils adapter | – | 467 | light |
| `hwclock` | other | – | – | – | – | no tests | 93 | – | light |
| `i2cdetect` | other | – | – | – | – | no tests | 405 | – | medium |
| `i2cdump` | other | – | – | – | – | no tests | 405 | – | medium |
| `i2cget` | other | – | – | – | – | no tests | 405 | – | medium |
| `i2cset` | other | – | – | – | – | no tests | 405 | – | medium |
| `i2ctransfer` | other | – | – | – | – | no tests | 405 | – | medium |
| `iconv` | posix | 5 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 75 | – | light |
| `ifconfig` | net | 20 | – | – | network, root | toybox: shell .test (runs in tests/toybox adapter) | 559 | – | heavy |
| `init` | pending | – | – | – | – | no tests | 513 | – | medium |
| `inotifyd` | other | – | – | – | – | no tests | 125 | – | light |
| `insmod` | other | – | – | – | – | no tests | 46 | – | light |
| `install` | posix | 5 | coreutils:125 | 2 | kernel/fs, root | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 2/125 simple new_ucmd! cases parseable by tests/uutils adapter | 544 | 1750 | heavy |
| `ionice` | other | – | – | – | – | no tests | 94 | – | light |
| `iorenice` | other | – | – | – | – | no tests | 94 | – | light |
| `iotop` | posix | – | – | – | – | no tests | 2004 | – | heavy |
| `ip` | pending | – | – | – | – | no tests | 2934 | – | heavy |
| `ipaddr` | pending | – | – | – | – | no tests | 2934 | – | heavy |
| `ipcrm` | pending | – | – | – | – | no tests | 88 | – | light |
| `ipcs` | pending | – | – | – | – | no tests | 447 | – | medium |
| `iplink` | pending | – | – | – | – | no tests | 2934 | – | heavy |
| `iproute` | pending | – | – | – | – | no tests | 2934 | – | heavy |
| `iprule` | pending | – | – | – | – | no tests | 2934 | – | heavy |
| `iptunnel` | pending | – | – | – | – | no tests | 2934 | – | heavy |
| `join` | – | – | coreutils:49 | 38 | tty | uutils: Rust, 38/49 simple new_ucmd! cases parseable by tests/uutils adapter | – | 1226 | medium |
| `killall` | lsb | 2 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 123 | – | light |
| `killall5` | posix | – | – | – | – | no tests | 158 | – | light |
| `klogd` | pending | – | – | – | – | no tests | 98 | – | light |
| `last` | pending | – | util-linux:12 | 10 | – | uutils: Rust, 10/12 simple new_ucmd! cases parseable by tests/uutils adapter | 196 | 767 | light |
| `link` | posix | 4 | coreutils:8 | 2 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 2/8 simple new_ucmd! cases parseable by tests/uutils adapter | 25 | 57 | light |
| `linux32` | other | – | – | – | – | no tests | 24 | – | light |
| `load_policy` | android | – | – | – | – | no tests | 31 | – | medium |
| `locate` | – | – | findutils:27 | 0 | – | uutils: Rust, 0/27 simple new_ucmd! cases parseable by tests/uutils adapter | – | – | medium |
| `log` | android | – | – | – | – | no tests | 70 | – | medium |
| `logger` | posix | – | – | – | – | no tests | 98 | – | light |
| `login` | other | – | login:–, shadow:11 | 0 | root, tty | uutils: Rust, 0/11 simple new_ucmd! cases parseable by tests/uutils adapter | 137 | 28 | medium |
| `losetup` | other | 6 | – | – | devices, root | toybox: shell .test (runs in tests/toybox adapter) | 206 | – | heavy |
| `lsattr` | other | 11 | – | – | kernel/fs | toybox: shell .test (runs in tests/toybox adapter) | 337 | – | medium |
| `lscpu` | – | – | util-linux:4 | 4 | – | uutils: Rust, 4/4 simple new_ucmd! cases parseable by tests/uutils adapter | – | 673 | medium |
| `lsipc` | – | – | util-linux:– | – | – | no tests | – | 2876 | heavy |
| `lslocks` | – | – | util-linux:1 | 1 | – | uutils: Rust, 1/1 simple new_ucmd! cases parseable by tests/uutils adapter | – | 1344 | medium |
| `lsmem` | – | – | util-linux:33 | 4 | kernel/fs | uutils: Rust, 4/33 simple new_ucmd! cases parseable by tests/uutils adapter | – | 1008 | medium |
| `lsmod` | other | – | – | – | – | no tests | 36 | – | light |
| `lsns` | – | – | util-linux:14 | 14 | kernel/fs | uutils: Rust, 14/14 simple new_ucmd! cases parseable by tests/uutils adapter | – | 898 | medium |
| `lsof` | pending | – | – | – | – | no tests | 446 | – | medium |
| `lspci` | other | – | – | – | – | no tests | 533 | – | medium |
| `lsusb` | other | – | – | – | – | no tests | 533 | – | medium |
| `makedevs` | other | – | – | – | – | no tests | 115 | – | light |
| `mcookie` | other | – | util-linux:10 | 10 | – | uutils: Rust, 10/10 simple new_ucmd! cases parseable by tests/uutils adapter | 35 | 140 | light |
| `mdev` | pending | – | – | – | – | no tests | 297 | – | medium |
| `memeater` | other | – | – | – | – | no tests | 31 | – | light |
| `mesg` | – | – | util-linux:3 | 3 | tty | uutils: Rust, 3/3 simple new_ucmd! cases parseable by tests/uutils adapter | – | 103 | light |
| `microcom` | net | – | – | – | – | no tests | 137 | – | medium |
| `mix` | other | – | – | – | – | no tests | 66 | – | light |
| `mkfifo` | posix | 5 | coreutils:16 | 10 | kernel/fs | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 10/16 simple new_ucmd! cases parseable by tests/uutils adapter | 40 | 192 | light |
| `mknod` | lsb | – | coreutils:17 | 9 | devices, kernel/fs | uutils: Rust, 9/17 simple new_ucmd! cases parseable by tests/uutils adapter | 50 | 315 | medium |
| `mkpasswd` | other | 5 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 81 | – | light |
| `mkswap` | other | – | – | – | – | no tests | 50 | – | light |
| `modinfo` | other | 5 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 129 | – | light |
| `modprobe` | pending | – | – | – | – | no tests | 547 | – | medium |
| `more` | pending | 2 | coreutils:11 | 2 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 2/11 simple new_ucmd! cases parseable by tests/uutils adapter | 172 | 1201 | light |
| `mount` | lsb | 15 | – | – | kernel/fs | toybox: shell .test (runs in tests/toybox adapter) | 415 | – | medium |
| `mountpoint` | other | – | util-linux:1 | 1 | – | uutils: Rust, 1/1 simple new_ucmd! cases parseable by tests/uutils adapter | 62 | 76 | light |
| `nbd-client` | other | 10 | – | – | root | toybox: shell .test (runs in tests/toybox adapter) | 112 | – | medium |
| `nbd-server` | other | – | – | – | – | no tests | 88 | – | light |
| `nbd_client` | other | – | – | – | – | no tests | 112 | – | light |
| `nbd_server` | other | – | – | – | – | no tests | 88 | – | light |
| `nc` | net | – | – | – | – | no tests | 258 | – | heavy |
| `netcat` | net | 1 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 258 | – | heavy |
| `netstat` | net | – | – | – | – | no tests | 382 | – | heavy |
| `newgidmap` | – | – | shadow:– | – | – | no tests | – | 42 | light |
| `newgrp` | – | – | shadow:2 | 0 | root | uutils: Rust, 0/2 simple new_ucmd! cases parseable by tests/uutils adapter | – | 329 | medium |
| `newuidmap` | – | – | shadow:9 | 0 | kernel/fs, root | uutils: Rust, 0/9 simple new_ucmd! cases parseable by tests/uutils adapter | – | 762 | heavy |
| `newusers` | – | – | shadow:16 | 0 | root | uutils: Rust, 0/16 simple new_ucmd! cases parseable by tests/uutils adapter | – | 715 | heavy |
| `nfsmount` | lsb | – | – | – | – | no tests | 415 | – | medium |
| `nohup` | posix | – | coreutils:16 | 6 | root, tty | uutils: Rust, 6/16 simple new_ucmd! cases parseable by tests/uutils adapter | 41 | 327 | medium |
| `nologin` | other | – | util-linux:2 | 2 | – | uutils: Rust, 2/2 simple new_ucmd! cases parseable by tests/uutils adapter | 24 | 92 | light |
| `nsenter` | other | – | – | – | – | no tests | 166 | – | light |
| `numfmt` | – | – | coreutils:196 | 174 | network, tty | uutils: Rust, 174/196 simple new_ucmd! cases parseable by tests/uutils adapter | – | 3233 | heavy |
| `od` | posix | – | coreutils:83 | 62 | tty | uutils: Rust, 62/83 simple new_ucmd! cases parseable by tests/uutils adapter | 294 | 4261 | medium |
| `oneit` | other | – | – | – | – | no tests | 135 | – | light |
| `openvt` | other | – | – | – | – | no tests | 115 | – | light |
| `partprobe` | other | – | – | – | – | no tests | 30 | – | light |
| `passwd` | lsb | – | shadow:20 | 0 | root | uutils: Rust, 0/20 simple new_ucmd! cases parseable by tests/uutils adapter | 116 | 1482 | medium |
| `patch` | posix | 13 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 485 | – | medium |
| `pidof` | lsb | 5 | procps:12 | 12 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 12/12 simple new_ucmd! cases parseable by tests/uutils adapter | 45 | 225 | light |
| `pidwait` | – | – | procps:4 | 4 | – | uutils: Rust, 4/4 simple new_ucmd! cases parseable by tests/uutils adapter | – | 141 | light |
| `ping` | net | – | – | – | – | no tests | 309 | – | heavy |
| `ping6` | net | – | – | – | – | no tests | 309 | – | heavy |
| `pinky` | – | – | coreutils:8 | 2 | root | uutils: Rust, 2/8 simple new_ucmd! cases parseable by tests/uutils adapter | – | 605 | heavy |
| `pivot_root` | other | – | – | – | – | no tests | 29 | – | light |
| `pmap` | other | – | procps:21 | 14 | – | uutils: Rust, 14/21 simple new_ucmd! cases parseable by tests/uutils adapter | 103 | 2475 | light |
| `poweroff` | other | – | – | – | – | no tests | 46 | – | light |
| `pr` | – | – | coreutils:84 | 77 | – | uutils: Rust, 77/84 simple new_ucmd! cases parseable by tests/uutils adapter | – | 1703 | heavy |
| `prlimit` | posix | – | – | – | – | no tests | 124 | – | light |
| `ps` | posix | 1 | procps:25 | 13 | root | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 13/25 simple new_ucmd! cases parseable by tests/uutils adapter | 2004 | 1238 | heavy |
| `ptx` | – | – | coreutils:50 | 50 | – | uutils: Rust, 50/50 simple new_ucmd! cases parseable by tests/uutils adapter | – | 1233 | medium |
| `pwck` | – | – | shadow:18 | 0 | root | uutils: Rust, 0/18 simple new_ucmd! cases parseable by tests/uutils adapter | – | 1231 | heavy |
| `pwconv` | – | – | shadow:13 | 0 | root | uutils: Rust, 0/13 simple new_ucmd! cases parseable by tests/uutils adapter | – | 660 | heavy |
| `pwdx` | other | – | procps:8 | 8 | – | uutils: Rust, 8/8 simple new_ucmd! cases parseable by tests/uutils adapter | 33 | 63 | light |
| `pwgen` | other | – | – | – | – | no tests | 74 | – | light |
| `pwunconv` | – | – | shadow:– | – | – | no tests | – | 42 | light |
| `readahead` | other | – | – | – | – | no tests | 28 | – | light |
| `readelf` | other | 9 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 775 | – | heavy |
| `reboot` | other | – | – | – | – | no tests | 46 | – | light |
| `rename` | – | – | util-linux:60 | 5 | root | uutils: Rust, 5/60 simple new_ucmd! cases parseable by tests/uutils adapter | – | 1071 | heavy |
| `renice` | posix | 2 | bsdutils:1, util-linux:1 | 2 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 2/2 simple new_ucmd! cases parseable by tests/uutils adapter | 51 | 66 | light |
| `reset` | other | – | – | – | – | no tests | 31 | – | light |
| `restorecon` | android | – | – | – | – | no tests | 46 | – | medium |
| `rfkill` | net | – | – | – | – | no tests | 105 | – | medium |
| `rmmod` | other | – | – | – | – | no tests | 39 | – | light |
| `route` | pending | – | – | – | – | no tests | 383 | – | medium |
| `rtcwake` | other | – | – | – | – | no tests | 118 | – | light |
| `runcon` | android | – | coreutils:8 | 8 | kernel/fs | uutils: Rust, 8/8 simple new_ucmd! cases parseable by tests/uutils adapter | 28 | 565 | medium |
| `sed` | posix | 109 | sed:211, red:– | 211 | tty | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 211/211 simple new_ucmd! cases parseable by tests/uutils adapter | 1117 | – | heavy |
| `sendevent` | android | – | – | – | – | no tests | 37 | – | medium |
| `setenforce` | android | – | – | – | – | no tests | 32 | – | medium |
| `setfacl` | – | – | acl:1 | 1 | – | uutils: Rust, 1/1 simple new_ucmd! cases parseable by tests/uutils adapter | – | 116 | light |
| `setfattr` | other | 3 | – | – | kernel/fs | toybox: shell .test (runs in tests/toybox adapter) | 40 | – | light |
| `setpgid` | – | – | util-linux:5 | 4 | – | uutils: Rust, 4/5 simple new_ucmd! cases parseable by tests/uutils adapter | – | 103 | light |
| `setsid` | other | – | util-linux:11 | 8 | kernel/fs, tty | uutils: Rust, 8/11 simple new_ucmd! cases parseable by tests/uutils adapter | 58 | 200 | light |
| `sg` | – | – | shadow:8 | 0 | root | uutils: Rust, 0/8 simple new_ucmd! cases parseable by tests/uutils adapter | – | 247 | medium |
| `sh` | pending | 233 | – | – | kernel/fs, network, root, tty | toybox: shell .test (runs in tests/toybox adapter) | 5466 | – | heavy |
| `sha224sum` | lsb | 8 | coreutils:8 | 2 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 2/8 simple new_ucmd! cases parseable by tests/uutils adapter | 142 | 12 | light |
| `sha384sum` | lsb | 8 | coreutils:8 | 2 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 2/8 simple new_ucmd! cases parseable by tests/uutils adapter | 142 | 12 | light |
| `sha3sum` | other | 8 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 105 | – | light |
| `shred` | other | – | coreutils:31 | 4 | kernel/fs, tty | uutils: Rust, 4/31 simple new_ucmd! cases parseable by tests/uutils adapter | 99 | 916 | light |
| `shuf` | other | – | coreutils:78 | 64 | – | uutils: Rust, 64/78 simple new_ucmd! cases parseable by tests/uutils adapter | 63 | 905 | light |
| `skill` | – | – | procps:1 | 1 | – | uutils: Rust, 1/1 simple new_ucmd! cases parseable by tests/uutils adapter | – | 99 | light |
| `slabtop` | – | – | procps:6 | 4 | root | uutils: Rust, 4/6 simple new_ucmd! cases parseable by tests/uutils adapter | – | 518 | heavy |
| `snice` | – | – | procps:3 | 3 | – | uutils: Rust, 3/3 simple new_ucmd! cases parseable by tests/uutils adapter | – | 655 | medium |
| `sntp` | net | – | – | – | – | no tests | 232 | – | heavy |
| `stdbuf` | – | – | coreutils:21 | 12 | network, tty | uutils: Rust, 12/21 simple new_ucmd! cases parseable by tests/uutils adapter | – | 670 | medium |
| `strace` | pending | – | – | – | – | no tests | 676 | – | heavy |
| `stty` | pending | – | coreutils:86 | 76 | tty | uutils: Rust, 76/86 simple new_ucmd! cases parseable by tests/uutils adapter | 460 | 1939 | medium |
| `su` | lsb | – | – | – | – | no tests | 93 | – | light |
| `sulogin` | pending | – | – | – | – | no tests | 117 | – | light |
| `sum` | – | – | coreutils:13 | 8 | kernel/fs, network | uutils: Rust, 8/13 simple new_ucmd! cases parseable by tests/uutils adapter | – | 214 | light |
| `swapoff` | other | – | – | – | – | no tests | 41 | – | light |
| `swapon` | other | – | – | – | – | no tests | 34 | – | light |
| `switch_root` | other | – | – | – | – | no tests | 126 | – | light |
| `sync` | lsb | – | coreutils:17 | 13 | – | uutils: Rust, 13/17 simple new_ucmd! cases parseable by tests/uutils adapter | 23 | 312 | light |
| `sysctl` | other | – | procps:9 | 9 | – | uutils: Rust, 9/9 simple new_ucmd! cases parseable by tests/uutils adapter | 149 | 257 | light |
| `syslogd` | pending | – | – | – | – | no tests | 556 | – | medium |
| `tar` | posix | 83 | tar:63 | 11 | devices, kernel/fs, root | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 11/63 simple new_ucmd! cases parseable by tests/uutils adapter | 1221 | 1410 | heavy |
| `taskset` | other | 20 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 228 | – | medium |
| `tcpsvd` | pending | – | – | – | – | no tests | 395 | – | medium |
| `telnet` | pending | – | – | – | – | no tests | 271 | – | medium |
| `telnetd` | pending | – | – | – | – | no tests | 417 | – | medium |
| `test` | posix | 61 | coreutils:126 | 80 | root, tty | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 80/126 simple new_ucmd! cases parseable by tests/uutils adapter | 209 | 2112 | heavy |
| `tftp` | pending | – | – | – | – | no tests | 461 | – | medium |
| `tftpd` | pending | – | – | – | – | no tests | 323 | – | medium |
| `time` | posix | – | – | – | – | no tests | 60 | – | light |
| `timeout` | other | 16 | coreutils:37 | 12 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 12/37 simple new_ucmd! cases parseable by tests/uutils adapter | 107 | 729 | light |
| `tload` | – | – | procps:1 | 1 | – | uutils: Rust, 1/1 simple new_ucmd! cases parseable by tests/uutils adapter | – | 289 | light |
| `traceroute` | pending | – | – | – | – | no tests | 634 | – | heavy |
| `traceroute6` | pending | – | – | – | – | no tests | 634 | – | heavy |
| `ts` | other | – | – | – | – | no tests | 54 | – | light |
| `tunctl` | net | – | – | – | – | no tests | 53 | – | medium |
| `uclampset` | other | – | – | – | – | no tests | 89 | – | light |
| `ucsicontrol` | other | – | – | – | – | no tests | 685 | – | heavy |
| `udpsvd` | pending | – | – | – | – | no tests | 395 | – | medium |
| `ulimit` | posix | – | – | – | – | no tests | 124 | – | light |
| `umount` | lsb | – | – | – | – | no tests | 150 | – | light |
| `unicode` | other | 7 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 86 | – | light |
| `unix2dos` | other | – | – | – | – | no tests | 76 | – | light |
| `unlink` | posix | – | coreutils:7 | 2 | – | uutils: Rust, 2/7 simple new_ucmd! cases parseable by tests/uutils adapter | 24 | 50 | light |
| `unshare` | other | – | – | – | – | no tests | 166 | – | light |
| `updatedb` | – | – | findutils:27 | 0 | – | uutils: Rust, 0/27 simple new_ucmd! cases parseable by tests/uutils adapter | – | – | medium |
| `uptime` | other | 2 | coreutils:16 | 11 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 11/16 simple new_ucmd! cases parseable by tests/uutils adapter | 79 | 394 | light |
| `useradd` | pending | 14 | shadow:34 | 0 | root | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 0/34 simple new_ucmd! cases parseable by tests/uutils adapter | 159 | 2462 | medium |
| `userdel` | pending | – | shadow:16 | 0 | root | uutils: Rust, 0/16 simple new_ucmd! cases parseable by tests/uutils adapter | 114 | 621 | medium |
| `usermod` | – | – | shadow:25 | 0 | root | uutils: Rust, 0/25 simple new_ucmd! cases parseable by tests/uutils adapter | – | 793 | heavy |
| `users` | – | – | coreutils:5 | 4 | – | uutils: Rust, 4/5 simple new_ucmd! cases parseable by tests/uutils adapter | – | 116 | light |
| `usleep` | other | – | – | – | – | no tests | 26 | – | light |
| `uudecode` | posix | 9 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 113 | – | light |
| `uuencode` | posix | 9 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 59 | – | light |
| `uuidgen` | other | 1 | util-linux:7 | 7 | kernel/fs | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 7/7 simple new_ucmd! cases parseable by tests/uutils adapter | 25 | 210 | light |
| `vconfig` | other | – | – | – | – | no tests | 87 | – | light |
| `vdir` | – | – | coreutils:12 | 6 | tty | uutils: Rust, 6/12 simple new_ucmd! cases parseable by tests/uutils adapter | – | 43 | light |
| `vigr` | – | – | shadow:– | – | – | no tests | – | 49 | light |
| `vipw` | – | – | shadow:13 | 0 | root | uutils: Rust, 0/13 simple new_ucmd! cases parseable by tests/uutils adapter | – | 586 | heavy |
| `vmstat` | other | – | procps:14 | 14 | – | uutils: Rust, 14/14 simple new_ucmd! cases parseable by tests/uutils adapter | 155 | 1219 | light |
| `w` | other | – | procps:5 | 5 | – | uutils: Rust, 5/5 simple new_ucmd! cases parseable by tests/uutils adapter | 33 | 504 | light |
| `watch` | other | – | procps:6 | 6 | – | uutils: Rust, 6/6 simple new_ucmd! cases parseable by tests/uutils adapter | 186 | 268 | light |
| `watchdog` | other | – | – | – | – | no tests | 51 | – | light |
| `wget` | net | – | – | – | – | no tests | 358 | – | heavy |
| `who` | posix | – | coreutils:27 | 2 | – | uutils: Rust, 2/27 simple new_ucmd! cases parseable by tests/uutils adapter | 44 | 666 | light |
| `xargs` | posix | 35 | findutils:33 | 0 | – | toybox: shell .test (runs in tests/toybox adapter); uutils: Rust, 0/33 simple new_ucmd! cases parseable by tests/uutils adapter | 236 | – | medium |
| `xxd` | other | 22 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 182 | – | light |
| `xzcat` | pending | 4 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 2651 | – | heavy |
| `zcat` | lsb | 4 | – | – | – | toybox: shell .test (runs in tests/toybox adapter) | 179 | – | light |

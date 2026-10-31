# Utility comparison: Misa vs toybox vs BusyBox vs uutils

Generated 2026-10-15 from real sources (not memory) by parsing each project's registry/applet declarations.

## Sources

| Project | Source | Revision | How utilities were listed |
|---|---|---|---|
| Misa | this repository, 2026-10-15 | before publishing | `misa --list` (runtime builtin registry, internal/builtins/registry.go) |
| toybox | https://github.com/landley/toybox | 6a0c0ef (2026-10-05), version 0.8.15 | every `USE_X(NEWTOY/OLDTOY(name…))` in toys/*/*.c (toys/example excluded); category = directory holding the `config X` entry; `(default n)` = `config X` has no `default y` |
| BusyBox | https://git.busybox.net/busybox | 9d961af (2026-10-12), version 1.39.0 | every `//applet:…APPLET*(name…)` declaration in *.c/*.h |
| uutils/acl | https://github.com/uutils/acl | f992445 (2026-10-03) | directories in src/uu/ |
| uutils/awk | https://github.com/uutils/awk | 91b6c10 (2026-10-11) | single binary |
| uutils/bsdutils | https://github.com/uutils/bsdutils | 3ba11da (2026-05-30) | directories in src/uu/ |
| uutils/coreutils | https://github.com/uutils/coreutils | 2fb77e8 (2026-10-15) | directories in src/uu/ (checksum_common excluded: shared library) |
| uutils/diffutils | https://github.com/uutils/diffutils | 8af2bed (2026-10-14) | multicall dispatch in src/main.rs (diff, cmp) |
| uutils/findutils | https://github.com/uutils/findutils | 2b15f5d (2026-10-12) | [[bin]] targets in Cargo.toml |
| uutils/grep | https://github.com/uutils/grep | 01fb6c3 (2026-10-13) | single binary |
| uutils/hostname | https://github.com/uutils/hostname | f92ca23 (2026-10-03) | directories in src/uu/ |
| uutils/login | https://github.com/uutils/login | 2ac4e15 (2026-04-23) | directories in src/uu/ |
| uutils/procps | https://github.com/uutils/procps | b536810 (2026-10-07) | directories in src/uu/ |
| uutils/red | https://github.com/uutils/red | f18ab51 (2026-01-17) | single binary (experimental sed) |
| uutils/sed | https://github.com/uutils/sed | 58608af (2026-10-14) | single binary |
| uutils/shadow | https://github.com/uutils/shadow | 67899fd (2026-10-13) | directories in src/uu/ |
| uutils/tar | https://github.com/uutils/tar | fb55fee (2026-10-14) | directories in src/uu/ |
| uutils/util-linux | https://github.com/uutils/util-linux | 09959d4 (2026-10-14) | directories in src/uu/ |

uutils org repos checked that ship no utilities (libraries/docs/trackers): rust-users, platform-info, uucore (archived), coreutils-tracking, uutils.github.io, num-prime, findutils-tracking, uutils-term-grid, uutils-args (archived), parse_datetime, ansi-width, coreutils-l10n, sed-tracking, grep-tracking. `login` is archived but still listed.

## Totals

| Column | Utilities |
|---|---|
| Misa | 114 |
| toybox (all) | 349 |
| toybox: non-pending, default y | 250 |
| toybox: non-pending, default n | 3 |
| toybox: pending | 96 |
| BusyBox | 445 |
| uutils (any repo, distinct names) | 190 |
| uutils/acl | 3 |
| uutils/awk | 1 |
| uutils/bsdutils | 1 |
| uutils/coreutils | 108 |
| uutils/diffutils | 2 |
| uutils/findutils | 4 |
| uutils/grep | 1 |
| uutils/hostname | 1 |
| uutils/login | 1 |
| uutils/procps | 18 |
| uutils/red | 1 |
| uutils/sed | 1 |
| uutils/shadow | 28 |
| uutils/tar | 1 |
| uutils/util-linux | 23 |
| **Distinct utility names (rows)** | **588** |

Misa-only names (in none of the others): `bashtop`, `btop`, `bytop`, `chess`, `fetch`, `fix`, `htop`, `l1`, `lc`, `mines`, `sshc`, `tldr`, `tm`, `top-commands`.

Notes: toybox's `[[`, `cd`, `exit` etc. are sh builtins declared via USE_SH in toys/pending/sh.c, so they show as pending. uutils `hostname` appears in both coreutils and uutils/hostname; `renice` in util-linux and bsdutils; `sed` in sed and red; `login` in login and shadow.

## Table

| Utility | Misa | toybox | BusyBox | uutils |
|---|---|---|---|---|
| `-bash` |  | ✓ pending |  |  |
| `-sh` |  | ✓ pending |  |  |
| `-toysh` |  | ✓ pending |  |  |
| `.` |  | ✓ pending |  |  |
| `:` |  | ✓ posix |  |  |
| `[` |  | ✓ posix | ✓ |  |
| `[[` |  | ✓ pending | ✓ |  |
| `acpi` |  | ✓ other |  |  |
| `acpid` |  |  | ✓ |  |
| `add-shell` |  |  | ✓ |  |
| `addgroup` |  | ✓ pending | ✓ |  |
| `adduser` |  | ✓ pending | ✓ |  |
| `adjtimex` |  |  | ✓ |  |
| `alias` |  | ✓ pending |  |  |
| `ar` |  |  | ✓ |  |
| `arch` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `arp` |  | ✓ pending | ✓ |  |
| `arping` |  | ✓ pending | ✓ |  |
| `ascii` |  | ✓ other | ✓ |  |
| `ash` |  |  | ✓ |  |
| `awk` |  | ✓ pending | ✓ | ✓ awk |
| `b2sum` | ✓ |  |  | ✓ coreutils |
| `base32` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `base64` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `basename` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `basenc` | ✓ |  |  | ✓ coreutils |
| `bash` |  | ✓ pending | ✓ |  |
| `bashtop` | ✓ |  |  |  |
| `bbconfig` |  |  | ✓ |  |
| `bc` |  | ✓ pending | ✓ |  |
| `beep` |  |  | ✓ |  |
| `blkdiscard` |  | ✓ other | ✓ |  |
| `blkid` |  | ✓ other | ✓ |  |
| `blockdev` |  | ✓ other | ✓ | ✓ util-linux |
| `bootchartd` |  | ✓ pending | ✓ |  |
| `brctl` |  | ✓ pending | ✓ |  |
| `break` |  | ✓ pending |  |  |
| `btop` | ✓ |  |  |  |
| `bunzip2` |  | ✓ other | ✓ |  |
| `busybox` |  |  | ✓ |  |
| `bytop` | ✓ |  |  |  |
| `bzcat` |  | ✓ other | ✓ |  |
| `bzip2` |  |  | ✓ |  |
| `cal` | ✓ | ✓ posix | ✓ | ✓ util-linux |
| `cat` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `cd` |  | ✓ pending |  |  |
| `chacl` |  |  |  | ✓ acl |
| `chage` |  |  |  | ✓ shadow |
| `chat` |  |  | ✓ |  |
| `chattr` |  | ✓ other | ✓ |  |
| `chcon` |  | ✓ other | ✓ | ✓ coreutils |
| `chcpu` |  |  |  | ✓ util-linux |
| `chess` | ✓ |  |  |  |
| `chfn` |  |  |  | ✓ shadow |
| `chgpasswd` |  |  |  | ✓ shadow |
| `chgrp` |  | ✓ posix | ✓ | ✓ coreutils |
| `chmod` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `chown` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `chpasswd` |  |  | ✓ | ✓ shadow |
| `chpst` |  |  | ✓ |  |
| `chroot` |  | ✓ other | ✓ | ✓ coreutils |
| `chrt` |  | ✓ other | ✓ |  |
| `chsh` |  | ✓ pending |  | ✓ shadow |
| `chvt` |  | ✓ other | ✓ |  |
| `cksum` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `clear` |  | ✓ other | ✓ |  |
| `cmp` | ✓ | ✓ posix | ✓ | ✓ diffutils |
| `comm` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `command` |  | ✓ pending |  |  |
| `conspy` |  |  | ✓ |  |
| `continue` |  | ✓ pending |  |  |
| `count` |  | ✓ other |  |  |
| `cp` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `cpio` |  | ✓ posix | ✓ |  |
| `crc32` |  | ✓ posix | ✓ |  |
| `crond` |  | ✓ pending | ✓ |  |
| `crontab` |  | ✓ pending | ✓ |  |
| `cryptpw` |  |  | ✓ |  |
| `csplit` |  | ✓ pending |  | ✓ coreutils |
| `ctrlaltdel` |  |  |  | ✓ util-linux |
| `cttyhack` |  |  | ✓ |  |
| `cut` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `date` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `dc` |  |  | ✓ |  |
| `dd` |  | ✓ posix | ✓ | ✓ coreutils |
| `deallocvt` |  | ✓ other | ✓ |  |
| `declare` |  | ✓ pending |  |  |
| `delgroup` |  | ✓ pending | ✓ |  |
| `deluser` |  | ✓ pending | ✓ |  |
| `depmod` |  |  | ✓ |  |
| `devfsd` |  |  | ✓ |  |
| `devmem` |  | ✓ other | ✓ |  |
| `df` |  | ✓ posix | ✓ | ✓ coreutils |
| `dhcp` |  | ✓ pending |  |  |
| `dhcp6` |  | ✓ pending |  |  |
| `dhcpd` |  | ✓ pending |  |  |
| `dhcprelay` |  |  | ✓ |  |
| `diff` | ✓ | ✓ pending | ✓ | ✓ diffutils |
| `dir` |  |  |  | ✓ coreutils |
| `dircolors` |  |  |  | ✓ coreutils |
| `dirname` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `dmesg` |  | ✓ lsb | ✓ | ✓ util-linux |
| `dnsd` |  |  | ✓ |  |
| `dnsdomainname` |  | ✓ lsb | ✓ |  |
| `dos2unix` |  | ✓ other | ✓ |  |
| `dpkg` |  |  | ✓ |  |
| `dpkg-deb` |  |  | ✓ |  |
| `du` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `dumpkmap` |  |  | ✓ |  |
| `dumpleases` |  | ✓ pending | ✓ |  |
| `e2label` |  |  | ✓ |  |
| `echo` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `ed` |  |  | ✓ |  |
| `egrep` |  | ✓ posix | ✓ |  |
| `eject` |  | ✓ other | ✓ |  |
| `env` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `envdir` |  |  | ✓ |  |
| `envuidgid` |  |  | ✓ |  |
| `ether-wake` |  |  | ✓ |  |
| `eval` |  | ✓ pending |  |  |
| `exec` |  | ✓ pending |  |  |
| `exit` |  | ✓ pending |  |  |
| `expand` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `expiry` |  |  |  | ✓ shadow |
| `export` |  | ✓ pending |  |  |
| `expr` | ✓ | ✓ pending | ✓ | ✓ coreutils |
| `factor` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `fakeidentd` |  |  | ✓ |  |
| `fallocate` |  | ✓ other | ✓ |  |
| `false` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `fatattr` |  |  | ✓ |  |
| `fatlabel` |  |  | ✓ |  |
| `fbset` |  |  | ✓ |  |
| `fbsplash` |  |  | ✓ |  |
| `fdflush` |  |  | ✓ |  |
| `fdformat` |  |  | ✓ |  |
| `fdisk` |  | ✓ pending | ✓ |  |
| `fetch` | ✓ |  |  |  |
| `fgconsole` |  |  | ✓ |  |
| `fgrep` |  | ✓ posix | ✓ |  |
| `file` |  | ✓ posix |  |  |
| `find` |  | ✓ posix | ✓ | ✓ findutils |
| `findfs` |  |  | ✓ |  |
| `fix` | ✓ |  |  |  |
| `flash_eraseall` |  |  | ✓ |  |
| `flash_lock` |  |  | ✓ |  |
| `flash_unlock` |  |  | ✓ |  |
| `flashcp` |  |  | ✓ |  |
| `flock` |  | ✓ other | ✓ |  |
| `fmt` | ✓ | ✓ other |  | ✓ coreutils |
| `fold` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `free` | ✓ | ✓ other | ✓ | ✓ procps |
| `freeramdisk` |  | ✓ other | ✓ |  |
| `fsck` |  | ✓ pending | ✓ |  |
| `fsck.minix` |  |  | ✓ |  |
| `fsfreeze` |  | ✓ other | ✓ | ✓ util-linux |
| `fstrim` |  |  | ✓ |  |
| `fstype` |  | ✓ other |  |  |
| `fsync` |  | ✓ other | ✓ |  |
| `ftpd` |  |  | ✓ |  |
| `ftpget` |  | ✓ net | ✓ |  |
| `ftpput` |  | ✓ net | ✓ |  |
| `fuser` |  |  | ✓ |  |
| `getconf` |  | ✓ posix |  |  |
| `getenforce` |  | ✓ android | ✓ |  |
| `getfacl` |  |  |  | ✓ acl |
| `getfattr` |  | ✓ pending | ✓ |  |
| `getopt` |  | ✓ other | ✓ |  |
| `getsebool` |  |  | ✓ |  |
| `getty` |  | ✓ pending | ✓ |  |
| `gitcheckout` |  | ✓ pending |  |  |
| `gitclone` |  | ✓ pending |  |  |
| `gitfetch` |  | ✓ pending |  |  |
| `gitinit` |  | ✓ pending |  |  |
| `gitremote` |  | ✓ pending |  |  |
| `gpasswd` |  |  |  | ✓ shadow |
| `gpiodetect` |  | ✓ other |  |  |
| `gpiofind` |  | ✓ other |  |  |
| `gpioget` |  | ✓ other |  |  |
| `gpioinfo` |  | ✓ other |  |  |
| `gpioset` |  | ✓ other |  |  |
| `grep` |  | ✓ posix | ✓ | ✓ grep |
| `groupadd` |  | ✓ pending |  | ✓ shadow |
| `groupdel` |  | ✓ pending |  | ✓ shadow |
| `groupmod` |  |  |  | ✓ shadow |
| `groups` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `grpck` |  |  |  | ✓ shadow |
| `grpconv` |  |  |  | ✓ shadow |
| `grpunconv` |  |  |  | ✓ shadow |
| `gunzip` |  | ✓ lsb | ✓ |  |
| `gzip` |  | ✓ lsb (default n) | ✓ |  |
| `halt` |  | ✓ other | ✓ |  |
| `hd` |  | ✓ other | ✓ |  |
| `hdparm` |  |  | ✓ |  |
| `head` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `help` |  | ✓ other |  |  |
| `hexdump` |  | ✓ pending | ✓ | ✓ util-linux |
| `hexedit` |  | ✓ other | ✓ |  |
| `host` |  | ✓ net |  |  |
| `hostid` | ✓ |  | ✓ | ✓ coreutils |
| `hostname` | ✓ | ✓ lsb | ✓ | ✓ coreutils, ✓ hostname |
| `htop` | ✓ |  |  |  |
| `httpd` |  | ✓ net | ✓ |  |
| `hugetop` |  |  |  | ✓ procps |
| `hush` |  |  | ✓ |  |
| `hwclock` |  | ✓ other | ✓ |  |
| `i2cdetect` |  | ✓ other | ✓ |  |
| `i2cdump` |  | ✓ other | ✓ |  |
| `i2cget` |  | ✓ other | ✓ |  |
| `i2cset` |  | ✓ other | ✓ |  |
| `i2ctransfer` |  | ✓ other | ✓ |  |
| `iconv` |  | ✓ posix |  |  |
| `id` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `ifconfig` |  | ✓ net | ✓ |  |
| `ifdown` |  |  | ✓ |  |
| `ifenslave` |  |  | ✓ |  |
| `ifplugd` |  |  | ✓ |  |
| `ifup` |  |  | ✓ |  |
| `inetd` |  |  | ✓ |  |
| `init` |  | ✓ pending | ✓ |  |
| `inotifyd` |  | ✓ other | ✓ |  |
| `insmod` |  | ✓ other | ✓ |  |
| `install` |  | ✓ posix | ✓ | ✓ coreutils |
| `ionice` |  | ✓ other | ✓ |  |
| `iorenice` |  | ✓ other |  |  |
| `iostat` |  |  | ✓ |  |
| `iotop` |  | ✓ posix |  |  |
| `ip` |  | ✓ pending | ✓ |  |
| `ipaddr` |  | ✓ pending | ✓ |  |
| `ipcalc` |  |  | ✓ |  |
| `ipcrm` |  | ✓ pending | ✓ |  |
| `ipcs` |  | ✓ pending | ✓ |  |
| `iplink` |  | ✓ pending | ✓ |  |
| `ipneigh` |  |  | ✓ |  |
| `iproute` |  | ✓ pending | ✓ |  |
| `iprule` |  | ✓ pending | ✓ |  |
| `iptunnel` |  | ✓ pending | ✓ |  |
| `jobs` |  | ✓ pending |  |  |
| `join` |  |  | ✓ | ✓ coreutils |
| `kbd_mode` |  |  | ✓ |  |
| `kill` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `killall` |  | ✓ lsb | ✓ |  |
| `killall5` |  | ✓ posix | ✓ |  |
| `klogd` |  | ✓ pending | ✓ |  |
| `l1` | ✓ |  |  |  |
| `last` |  | ✓ pending | ✓ | ✓ util-linux |
| `lc` | ✓ |  |  |  |
| `less` |  |  | ✓ |  |
| `link` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `linux32` |  | ✓ other | ✓ |  |
| `linux64` |  |  | ✓ |  |
| `linuxrc` |  |  | ✓ |  |
| `ln` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `load_policy` |  | ✓ android | ✓ |  |
| `loadfont` |  |  | ✓ |  |
| `loadkmap` |  |  | ✓ |  |
| `local` |  | ✓ pending |  |  |
| `locate` |  |  |  | ✓ findutils |
| `log` |  | ✓ android |  |  |
| `logger` |  | ✓ posix | ✓ |  |
| `login` |  | ✓ other | ✓ | ✓ login, ✓ shadow |
| `logname` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `logread` |  |  | ✓ |  |
| `losetup` |  | ✓ other | ✓ |  |
| `lpd` |  |  | ✓ |  |
| `lpq` |  |  | ✓ |  |
| `lpr` |  |  | ✓ |  |
| `ls` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `lsattr` |  | ✓ other | ✓ |  |
| `lsblk` |  |  | ✓ |  |
| `lscpu` |  |  |  | ✓ util-linux |
| `lsipc` |  |  |  | ✓ util-linux |
| `lslocks` |  |  |  | ✓ util-linux |
| `lsmem` |  |  |  | ✓ util-linux |
| `lsmod` |  | ✓ other | ✓ |  |
| `lsns` |  |  |  | ✓ util-linux |
| `lsof` |  | ✓ pending | ✓ |  |
| `lspci` |  | ✓ other | ✓ |  |
| `lsscsi` |  |  | ✓ |  |
| `lsusb` |  | ✓ other | ✓ |  |
| `lzcat` |  |  | ✓ |  |
| `lzma` |  |  | ✓ |  |
| `lzop` |  |  | ✓ |  |
| `lzopcat` |  |  | ✓ |  |
| `makedevs` |  | ✓ other | ✓ |  |
| `makemime` |  |  | ✓ |  |
| `man` | ✓ | ✓ pending | ✓ |  |
| `matchpathcon` |  |  | ✓ |  |
| `mcookie` | ✓ | ✓ other |  | ✓ util-linux |
| `md5sum` | ✓ | ✓ lsb | ✓ | ✓ coreutils |
| `mdev` |  | ✓ pending | ✓ |  |
| `memeater` |  | ✓ other |  |  |
| `mesg` |  |  | ✓ | ✓ util-linux |
| `microcom` |  | ✓ net | ✓ |  |
| `mim` |  |  | ✓ |  |
| `mines` | ✓ |  |  |  |
| `minips` |  |  | ✓ |  |
| `mix` |  | ✓ other |  |  |
| `mkdir` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `mkdosfs` |  |  | ✓ |  |
| `mke2fs` |  |  | ✓ |  |
| `mkfifo` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `mkfs.ext2` |  |  | ✓ |  |
| `mkfs.minix` |  |  | ✓ |  |
| `mkfs.reiser` |  |  | ✓ |  |
| `mkfs.vfat` |  |  | ✓ |  |
| `mknod` |  | ✓ lsb | ✓ | ✓ coreutils |
| `mkpasswd` |  | ✓ other | ✓ |  |
| `mkswap` |  | ✓ other | ✓ |  |
| `mktemp` | ✓ | ✓ lsb | ✓ | ✓ coreutils |
| `modinfo` |  | ✓ other | ✓ |  |
| `modprobe` |  | ✓ pending | ✓ |  |
| `more` |  | ✓ pending | ✓ | ✓ coreutils |
| `mount` |  | ✓ lsb | ✓ |  |
| `mountpoint` |  | ✓ other | ✓ | ✓ util-linux |
| `mpstat` |  |  | ✓ |  |
| `mt` |  |  | ✓ |  |
| `mv` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `nameif` |  |  | ✓ |  |
| `nanddump` |  |  | ✓ |  |
| `nandwrite` |  |  | ✓ |  |
| `nbd-client` |  | ✓ other | ✓ |  |
| `nbd-server` |  | ✓ other |  |  |
| `nbd_client` |  | ✓ other |  |  |
| `nbd_server` |  | ✓ other |  |  |
| `nc` |  | ✓ net | ✓ |  |
| `netcat` |  | ✓ net | ✓ |  |
| `netstat` |  | ✓ net | ✓ |  |
| `newgidmap` |  |  |  | ✓ shadow |
| `newgrp` |  |  |  | ✓ shadow |
| `newuidmap` |  |  |  | ✓ shadow |
| `newusers` |  |  |  | ✓ shadow |
| `nfsmount` |  | ✓ lsb (default n) |  |  |
| `nice` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `nl` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `nmeter` |  |  | ✓ |  |
| `nohup` |  | ✓ posix | ✓ | ✓ coreutils |
| `nologin` |  | ✓ other | ✓ | ✓ util-linux |
| `nproc` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `nsenter` |  | ✓ other | ✓ |  |
| `nslookup` |  |  | ✓ |  |
| `ntpd` |  |  | ✓ |  |
| `nuke` |  |  | ✓ |  |
| `numfmt` | ✓ |  |  | ✓ coreutils |
| `od` |  | ✓ posix | ✓ | ✓ coreutils |
| `oneit` |  | ✓ other |  |  |
| `openvt` |  | ✓ other | ✓ |  |
| `parse` |  |  | ✓ |  |
| `partprobe` |  | ✓ other | ✓ |  |
| `passwd` |  | ✓ lsb (default n) | ✓ | ✓ shadow |
| `paste` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `patch` |  | ✓ posix | ✓ |  |
| `pathchk` | ✓ |  |  | ✓ coreutils |
| `pgrep` | ✓ | ✓ posix | ✓ | ✓ procps |
| `pidof` |  | ✓ lsb | ✓ | ✓ procps |
| `pidwait` |  |  |  | ✓ procps |
| `ping` |  | ✓ net | ✓ |  |
| `ping6` |  | ✓ net | ✓ |  |
| `pinky` |  |  |  | ✓ coreutils |
| `pipe_progress` |  |  | ✓ |  |
| `pivot_root` |  | ✓ other | ✓ |  |
| `pkill` | ✓ | ✓ posix | ✓ | ✓ procps |
| `pmap` |  | ✓ other | ✓ | ✓ procps |
| `popmaildir` |  |  | ✓ |  |
| `poweroff` |  | ✓ other | ✓ |  |
| `powertop` |  |  | ✓ |  |
| `pr` |  |  |  | ✓ coreutils |
| `printenv` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `printf` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `prlimit` |  | ✓ posix |  |  |
| `ps` | ✓ | ✓ posix | ✓ | ✓ procps |
| `pscan` |  |  | ✓ |  |
| `pstree` |  |  | ✓ |  |
| `ptx` |  |  |  | ✓ coreutils |
| `pwck` |  |  |  | ✓ shadow |
| `pwconv` |  |  |  | ✓ shadow |
| `pwd` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `pwdx` |  | ✓ other | ✓ | ✓ procps |
| `pwgen` |  | ✓ other |  |  |
| `pwunconv` |  |  |  | ✓ shadow |
| `raidautorun` |  |  | ✓ |  |
| `rdate` |  |  | ✓ |  |
| `rdev` |  |  | ✓ |  |
| `readahead` |  | ✓ other | ✓ |  |
| `readelf` |  | ✓ other |  |  |
| `readlink` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `readprofile` |  |  | ✓ |  |
| `realpath` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `reboot` |  | ✓ other | ✓ |  |
| `reformime` |  |  | ✓ |  |
| `remove-shell` |  |  | ✓ |  |
| `rename` |  |  |  | ✓ util-linux |
| `renice` |  | ✓ posix | ✓ | ✓ bsdutils, ✓ util-linux |
| `reset` |  | ✓ other | ✓ |  |
| `resize` |  |  | ✓ |  |
| `restorecon` |  | ✓ android | ✓ |  |
| `resume` |  |  | ✓ |  |
| `return` |  | ✓ pending |  |  |
| `rev` | ✓ | ✓ other | ✓ | ✓ util-linux |
| `rfkill` |  | ✓ net | ✓ |  |
| `rm` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `rmdir` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `rmmod` |  | ✓ other | ✓ |  |
| `route` |  | ✓ pending | ✓ |  |
| `rpm` |  |  | ✓ |  |
| `rpm2cpio` |  |  | ✓ |  |
| `rtcwake` |  | ✓ other | ✓ |  |
| `run-init` |  |  | ✓ |  |
| `run-parts` |  |  | ✓ |  |
| `runcon` |  | ✓ android | ✓ | ✓ coreutils |
| `runlevel` |  |  | ✓ |  |
| `runsv` |  |  | ✓ |  |
| `runsvdir` |  |  | ✓ |  |
| `rx` |  |  | ✓ |  |
| `script` |  |  | ✓ |  |
| `scriptreplay` |  |  | ✓ |  |
| `sed` |  | ✓ posix | ✓ | ✓ sed, ✓ red |
| `seedrng` |  |  | ✓ |  |
| `selinuxenabled` |  |  | ✓ |  |
| `sendevent` |  | ✓ android |  |  |
| `sendmail` |  |  | ✓ |  |
| `seq` | ✓ | ✓ lsb | ✓ | ✓ coreutils |
| `sestatus` |  |  | ✓ |  |
| `set` |  | ✓ pending |  |  |
| `setarch` |  |  | ✓ |  |
| `setconsole` |  |  | ✓ |  |
| `setenforce` |  | ✓ android | ✓ |  |
| `setfacl` |  |  |  | ✓ acl |
| `setfattr` |  | ✓ other | ✓ |  |
| `setfiles` |  |  | ✓ |  |
| `setfont` |  |  | ✓ |  |
| `setkeycodes` |  |  | ✓ |  |
| `setlogcons` |  |  | ✓ |  |
| `setpgid` |  |  |  | ✓ util-linux |
| `setpriv` |  |  | ✓ |  |
| `setsebool` |  |  | ✓ |  |
| `setserial` |  |  | ✓ |  |
| `setsid` |  | ✓ other | ✓ | ✓ util-linux |
| `setuidgid` |  |  | ✓ |  |
| `sg` |  |  |  | ✓ shadow |
| `sh` |  | ✓ pending | ✓ |  |
| `sha1sum` | ✓ | ✓ lsb | ✓ | ✓ coreutils |
| `sha224sum` | ✓ | ✓ lsb |  | ✓ coreutils |
| `sha256sum` | ✓ | ✓ lsb | ✓ | ✓ coreutils |
| `sha384sum` | ✓ | ✓ lsb | ✓ | ✓ coreutils |
| `sha3sum` | ✓ | ✓ other | ✓ |  |
| `sha512sum` | ✓ | ✓ lsb | ✓ | ✓ coreutils |
| `shift` |  | ✓ pending |  |  |
| `showkey` |  |  | ✓ |  |
| `shred` |  | ✓ other | ✓ | ✓ coreutils |
| `shuf` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `skill` |  |  |  | ✓ procps |
| `slabtop` |  |  |  | ✓ procps |
| `slattach` |  |  | ✓ |  |
| `sleep` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `smemcap` |  |  | ✓ |  |
| `snice` |  |  |  | ✓ procps |
| `sntp` |  | ✓ net |  |  |
| `softlimit` |  |  | ✓ |  |
| `sort` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `source` |  | ✓ pending |  |  |
| `split` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `sshc` | ✓ |  |  |  |
| `ssl_client` |  |  | ✓ |  |
| `ssl_server` |  |  | ✓ |  |
| `start-stop-daemon` |  |  | ✓ |  |
| `stat` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `stdbuf` |  |  |  | ✓ coreutils |
| `strace` |  | ✓ pending |  |  |
| `strings` | ✓ | ✓ posix | ✓ |  |
| `stty` |  | ✓ pending | ✓ | ✓ coreutils |
| `su` |  | ✓ lsb | ✓ |  |
| `sulogin` |  | ✓ pending | ✓ |  |
| `sum` | ✓ |  | ✓ | ✓ coreutils |
| `sv` |  |  | ✓ |  |
| `svc` |  |  | ✓ |  |
| `svlogd` |  |  | ✓ |  |
| `svok` |  |  | ✓ |  |
| `swaplabel` |  |  | ✓ |  |
| `swapoff` |  | ✓ other | ✓ |  |
| `swapon` |  | ✓ other | ✓ |  |
| `switch_root` |  | ✓ other | ✓ |  |
| `sync` | ✓ | ✓ lsb | ✓ | ✓ coreutils |
| `sysctl` |  | ✓ other | ✓ | ✓ procps |
| `syslogd` |  | ✓ pending | ✓ |  |
| `tac` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `tail` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `tar` |  | ✓ posix | ✓ | ✓ tar |
| `taskset` |  | ✓ other | ✓ |  |
| `tc` |  |  | ✓ |  |
| `tcpsvd` |  | ✓ pending | ✓ |  |
| `tee` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `telnet` |  | ✓ pending | ✓ |  |
| `telnetd` |  | ✓ pending | ✓ |  |
| `test` |  | ✓ posix | ✓ | ✓ coreutils |
| `tftp` |  | ✓ pending | ✓ |  |
| `tftpd` |  | ✓ pending | ✓ |  |
| `time` |  | ✓ posix | ✓ |  |
| `timeout` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `tldr` | ✓ |  |  |  |
| `tload` |  |  |  | ✓ procps |
| `tm` | ✓ |  |  |  |
| `top` | ✓ | ✓ posix | ✓ | ✓ procps |
| `top-commands` | ✓ |  |  |  |
| `touch` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `toysh` |  | ✓ pending |  |  |
| `tr` | ✓ | ✓ pending | ✓ | ✓ coreutils |
| `traceroute` |  | ✓ pending | ✓ |  |
| `traceroute6` |  | ✓ pending | ✓ |  |
| `trap` |  | ✓ pending |  |  |
| `tree` | ✓ |  | ✓ |  |
| `true` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `truncate` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `ts` |  | ✓ other | ✓ |  |
| `tsort` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `tty` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `ttysize` |  |  | ✓ |  |
| `tunctl` |  | ✓ net | ✓ |  |
| `tune2fs` |  |  | ✓ |  |
| `type` |  | ✓ pending |  |  |
| `ubiattach` |  |  | ✓ |  |
| `ubidetach` |  |  | ✓ |  |
| `ubimkvol` |  |  | ✓ |  |
| `ubirename` |  |  | ✓ |  |
| `ubirmvol` |  |  | ✓ |  |
| `ubirsvol` |  |  | ✓ |  |
| `ubiupdatevol` |  |  | ✓ |  |
| `uclampset` |  | ✓ other |  |  |
| `ucsicontrol` |  | ✓ other |  |  |
| `udhcpc` |  |  | ✓ |  |
| `udhcpc6` |  |  | ✓ |  |
| `udhcpd` |  |  | ✓ |  |
| `udpsvd` |  | ✓ pending | ✓ |  |
| `uevent` |  |  | ✓ |  |
| `ulimit` |  | ✓ posix |  |  |
| `umask` |  | ✓ pending |  |  |
| `umount` |  | ✓ lsb | ✓ |  |
| `unalias` |  | ✓ pending |  |  |
| `uname` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `uncompress` |  |  | ✓ |  |
| `unexpand` | ✓ |  | ✓ | ✓ coreutils |
| `unicode` |  | ✓ other |  |  |
| `uniq` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `unit` |  |  | ✓ |  |
| `unix2dos` |  | ✓ other | ✓ |  |
| `unlink` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `unlzma` |  |  | ✓ |  |
| `unlzop` |  |  | ✓ |  |
| `unset` |  | ✓ pending |  |  |
| `unshare` |  | ✓ other | ✓ |  |
| `unxz` |  |  | ✓ |  |
| `unzip` |  |  | ✓ |  |
| `updatedb` |  |  |  | ✓ findutils |
| `uptime` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `useradd` |  | ✓ pending |  | ✓ shadow |
| `userdel` |  | ✓ pending |  | ✓ shadow |
| `usermod` |  |  |  | ✓ shadow |
| `users` |  |  | ✓ | ✓ coreutils |
| `usleep` |  | ✓ other | ✓ |  |
| `uudecode` | ✓ | ✓ posix | ✓ |  |
| `uuencode` | ✓ | ✓ posix | ✓ |  |
| `uuidgen` | ✓ | ✓ other | ✓ | ✓ util-linux |
| `vconfig` |  | ✓ other | ✓ |  |
| `vdir` |  |  |  | ✓ coreutils |
| `vi` | ✓ | ✓ pending | ✓ |  |
| `vigr` |  |  |  | ✓ shadow |
| `vipw` |  |  |  | ✓ shadow |
| `vlock` |  |  | ✓ |  |
| `vmstat` | ✓ | ✓ other | ✓ | ✓ procps |
| `volname` |  |  | ✓ |  |
| `w` |  | ✓ other | ✓ | ✓ procps |
| `wait` |  | ✓ pending |  |  |
| `wall` |  |  | ✓ |  |
| `watch` |  | ✓ other | ✓ | ✓ procps |
| `watchdog` |  | ✓ other | ✓ |  |
| `wc` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `wget` |  | ✓ net | ✓ |  |
| `which` | ✓ | ✓ other | ✓ |  |
| `who` |  | ✓ posix | ✓ | ✓ coreutils |
| `whoami` | ✓ | ✓ posix | ✓ | ✓ coreutils |
| `whois` |  |  | ✓ |  |
| `xargs` |  | ✓ posix | ✓ | ✓ findutils |
| `xxd` | ✓ | ✓ other | ✓ |  |
| `xz` |  |  | ✓ |  |
| `xzcat` |  | ✓ pending | ✓ |  |
| `yes` | ✓ | ✓ other | ✓ | ✓ coreutils |
| `zcat` |  | ✓ lsb | ✓ |  |
| `zcip` |  |  | ✓ |  |
| **Total** | **114** | **349** (250 default, 96 pending) | **445** | **190** |

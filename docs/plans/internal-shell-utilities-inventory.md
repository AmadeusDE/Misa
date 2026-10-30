# Internal shell and utilities: pinned inventory

Research date: 2026-10-30. Misa baseline: `2e11a987d15c186bfc803cb73ea8c9bbdc22d46f`.

Companion to `internal-shell-utilities.md`. This appendix is a name/source/disposition inventory, not a behavior-conformance result.

Published roadmap and task definitions: [Misa #40](https://github.com/AmadeusDE/Misa/issues/40).

## Sources and licensing

Each link is pinned. Utility counts are declared names/directories, including aliases and experimental implementations. `src/uu` directory enumeration can omit generated aliases; Toybox alias declarations supply some of these. This is a reproducible candidate union, not a promise of exhaustive installed-binary aliases.

| Repository | Pinned revision | License checked | Enumerated names |
|---|---|---|---:|
| [landley/toybox](https://github.com/landley/toybox/tree/6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb) | `6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb` | [0BSD](https://github.com/landley/toybox/blob/6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb/LICENSE) | 349 |
| [mvdan/sh](https://github.com/mvdan/sh/tree/e96ef1dae52c71e86cf87eb594cb4917fa49aaba) | `e96ef1dae52c71e86cf87eb594cb4917fa49aaba` | [BSD-3-Clause](https://github.com/mvdan/sh/blob/e96ef1dae52c71e86cf87eb594cb4917fa49aaba/LICENSE) | shell; separate |
| [uutils/acl](https://github.com/uutils/acl/tree/af3feeda769cb55fe37d16f0acf2e77c685eed8d) | `af3feeda769cb55fe37d16f0acf2e77c685eed8d` | [MIT](https://github.com/uutils/acl/blob/af3feeda769cb55fe37d16f0acf2e77c685eed8d/LICENSE) | 3 |
| [uutils/bsdutils](https://github.com/uutils/bsdutils/tree/3ba11da4ddf49141b2266cec1f956f79db84d3b3) | `3ba11da4ddf49141b2266cec1f956f79db84d3b3` | [MIT](https://github.com/uutils/bsdutils/blob/3ba11da4ddf49141b2266cec1f956f79db84d3b3/LICENSE) | 1 |
| [uutils/coreutils](https://github.com/uutils/coreutils/tree/fee61ff5d4f87e0f912a52e639942b5f7ad58663) | `fee61ff5d4f87e0f912a52e639942b5f7ad58663` | [MIT](https://github.com/uutils/coreutils/blob/fee61ff5d4f87e0f912a52e639942b5f7ad58663/LICENSE) | 108 |
| [uutils/findutils](https://github.com/uutils/findutils/tree/d71f3242b0bc3b21bb311f04b23ae62e5f3fed67) | `d71f3242b0bc3b21bb311f04b23ae62e5f3fed67` | [MIT](https://github.com/uutils/findutils/blob/d71f3242b0bc3b21bb311f04b23ae62e5f3fed67/LICENSE) | 4 |
| [uutils/grep](https://github.com/uutils/grep/tree/0be8c54f268d66185faaee9d30f8a6f3c55c0a8b) | `0be8c54f268d66185faaee9d30f8a6f3c55c0a8b` | [MIT](https://github.com/uutils/grep/blob/0be8c54f268d66185faaee9d30f8a6f3c55c0a8b/LICENSE) | 1 |
| [uutils/hostname](https://github.com/uutils/hostname/tree/c4de07add34238b1dd577da17d03129b3f0940cf) | `c4de07add34238b1dd577da17d03129b3f0940cf` | [MIT](https://github.com/uutils/hostname/blob/c4de07add34238b1dd577da17d03129b3f0940cf/LICENSE) | 1 |
| [uutils/login](https://github.com/uutils/login/tree/2ac4e1518bd743f45b89e8db146d77feb14da197) | `2ac4e1518bd743f45b89e8db146d77feb14da197` | [MIT](https://github.com/uutils/login/blob/2ac4e1518bd743f45b89e8db146d77feb14da197/LICENSE) | 1 |
| [uutils/procps](https://github.com/uutils/procps/tree/e697667b3f758803e38d8ab4c8f912171beca602) | `e697667b3f758803e38d8ab4c8f912171beca602` | [MIT](https://github.com/uutils/procps/blob/e697667b3f758803e38d8ab4c8f912171beca602/LICENSE) | 18 |
| [uutils/red](https://github.com/uutils/red/tree/f18ab518e41e98ab0d2ad72e54a342ddb7b1ca43) | `f18ab518e41e98ab0d2ad72e54a342ddb7b1ca43` | [MIT](https://github.com/uutils/red/blob/f18ab518e41e98ab0d2ad72e54a342ddb7b1ca43/LICENSE) | 1 |
| [uutils/sed](https://github.com/uutils/sed/tree/42bffb9d3018a586a889c790bea4be58e249586a) | `42bffb9d3018a586a889c790bea4be58e249586a` | [MIT](https://github.com/uutils/sed/blob/42bffb9d3018a586a889c790bea4be58e249586a/LICENSE) | 1 |
| [uutils/shadow](https://github.com/uutils/shadow/tree/92ac29b0bf382434b145d2a0e557ca92f8ac219f) | `92ac29b0bf382434b145d2a0e557ca92f8ac219f` | [MIT](https://github.com/uutils/shadow/blob/92ac29b0bf382434b145d2a0e557ca92f8ac219f/LICENSE) | 28 |
| [uutils/tar](https://github.com/uutils/tar/tree/dffa9350f78b3bfbc9b4fc70a0f0205a53a937b6) | `dffa9350f78b3bfbc9b4fc70a0f0205a53a937b6` | [MIT](https://github.com/uutils/tar/blob/dffa9350f78b3bfbc9b4fc70a0f0205a53a937b6/LICENSE) | 1 |
| [uutils/util-linux](https://github.com/uutils/util-linux/tree/413b20967dcf8cd41f095dabf4858d4f02c17401) | `413b20967dcf8cd41f095dabf4858d4f02c17401` | [MIT](https://github.com/uutils/util-linux/blob/413b20967dcf8cd41f095dabf4858d4f02c17401/LICENSE) | 23 |

The installed mvdan module **v3.13.1** was separately inspected and probed; current upstream code is not assumed to describe that version. The import PR must choose a single source revision.

**Excluded:** uutils/awk and uutils/diffutils (repository metadata says Apache-2.0), and non-utility infrastructure/website/tracking repositories. No source from those excluded implementations was cloned for this research. `uutils/login` is archived; `uutils/red` is a second sed reference. Repository LICENSE files do not settle the provenance of every fixture or imported file.

**Test candidates:** `T` means Toybox `tests/<name>.test`; a uutils name means an exact `tests/by-util/test_<name>.rs` or `tests/test_<name>.rs` exists; findutils locate/updatedb also uses `tests/db_tests.rs`. A dash means no match under this conservative filename rule, not no tests anywhere. The table does not count cases, run upstream suites, or assert that Misa adapters accept them. Inspect per-file licensing before copying. Shared alias tests may live under the primary command.

## Totals and interpretation

- Misa runtime registry: **115** command names.
- Approved-source union: **407** declared names.
- Toybox USE_SH declarations separated: **30**.
- Remaining unmatched registry names: **278**; every one has a roadmap disposition below.
- The union overlaps Misa in 99 declared names; Misa also has 16 names outside this approved-source enumeration.
- `:` remains in the 278 because Toybox declares it separately from USE_SH; its work belongs in S11. `ulimit` is also shell state work despite its Toybox standalone declaration.
- `clear` exists as a Misa UI action, but is not a registered standalone utility. `cd`, `alias`, `unalias`, `exit`, `jobs`, `fg`, `bg` are handled in UI paths; that does not provide complete shell-language behavior.
- Pending Toybox source is explicitly labeled by its `toys/pending/` path. Non-pending is not a certification of completeness or portability. The table intentionally avoids source-LOC-based effort estimates.

## Current Misa registered commands

```text
arch b2sum base32 base64 basename basenc bashtop battleship btop bytop cal cat chess chmod chown cksum cmp comm cp cut date devices diff dirname du echo env expand expr factor false fetch fmt fold groups head hostid hostname htop id kill l1 lc link ln logname ls man mcookie md5sum mines mkdir mkfifo mktemp mv nice nl nproc numfmt paste pathchk pgrep pkill printenv printf pwd readlink realpath rev rm rmdir seq sha1sum sha224sum sha256sum sha384sum sha3sum sha512sum shuf sleep sort split sshc stat strings sum sync tac tail tee timeout tldr tm top top-commands touch tr tree true truncate tsort tty uname unexpand uniq unlink uudecode uuencode uuidgen vi wc which whoami xxd yes
```

Names already present are not new-command tasks. Existing correctness/option gaps are F08/U44 and command-specific compatibility follow-ups. `install.go` implements `misa --install` symlinks, not the missing install utility. Existing `top`/htop-style viewers do not supply the missing scriptable ps/free/vmstat commands.

## Shell declarations excluded from the utility missing count

```text
-bash -sh -toysh . [[ alias bash break cd command continue declare eval exec exit export jobs local return set sh shift source toysh trap type umask unalias unset wait
```

These are candidate entry aliases, builtins or syntax in Toybox, not an assertion that Misa lacks every one. Do not advertise a `bash` alias as full Bash compatibility or expose `[[` as an ordinary argument-list command. The internal-shell roadmap additionally covers interpreter-owned read/getopts/readonly/shift/trap/umask/ulimit/wait/exec semantics that cannot be inferred from multicall registration.

## Missing coreutils directory names

```text
chcon chgrp chroot csplit dd df dir dircolors install join mknod more nohup od pinky pr ptx runcon shred stdbuf stty test uptime users vdir who
```

## Complete unmatched-name inventory

`T` source paths are relative to the pinned Toybox root above; other labels are relative to that pinned uutils repository. Task IDs refer to the companion roadmap. Optional groups are candidate work, with feasibility gates described there.

| Name | Roadmap task | Approved source locations | Candidate tests |
|---|---|---|---|
| `:` | S11 | T: `toys/posix/true.c` | — |
| `[` | U01 | T: `toys/posix/test.c` | — |
| `acpi` | O15 | T: `toys/other/acpi.c` | — |
| `addgroup` | O10 | T: `toys/pending/groupadd.c` | — |
| `adduser` | O10 | T: `toys/pending/useradd.c` | — |
| `arp` | O07 | T: `toys/pending/arp.c` | — |
| `arping` | O08 | T: `toys/pending/arping.c` | — |
| `ascii` | O18 | T: `toys/other/ascii.c` | — |
| `awk` | U41–U42 | T: `toys/pending/awk.c` | T |
| `bc` | U43 | T: `toys/pending/bc.c` | T |
| `blkdiscard` | O12 | T: `toys/other/blkdiscard.c` | — |
| `blkid` | O03 | T: `toys/other/blkid.c` | T |
| `blockdev` | O12 | T: `toys/other/blockdev.c`; util-linux: `src/uu/blockdev` | util-linux |
| `bootchartd` | O25 | T: `toys/pending/bootchartd.c` | — |
| `brctl` | O07 | T: `toys/pending/brctl.c` | — |
| `bunzip2` | O17 | T: `toys/other/bzcat.c` | — |
| `bzcat` | O17 | T: `toys/other/bzcat.c` | T |
| `chacl` | O02 | acl: `src/uu/chacl` | acl |
| `chage` | O09 | shadow: `src/uu/chage` | shadow |
| `chattr` | O02 | T: `toys/other/lsattr.c` | T |
| `chcon` | O16 | T: `toys/other/chcon.c`; coreutils: `src/uu/chcon` | coreutils |
| `chcpu` | O04 | util-linux: `src/uu/chcpu` | util-linux |
| `chfn` | O11 | shadow: `src/uu/chfn` | shadow |
| `chgpasswd` | O11 | shadow: `src/uu/chgpasswd` | shadow |
| `chgrp` | U13 | T: `toys/posix/chgrp.c`; coreutils: `src/uu/chgrp` | T, coreutils |
| `chpasswd` | O11 | shadow: `src/uu/chpasswd` | shadow |
| `chroot` | O01 | T: `toys/other/chroot.c`; coreutils: `src/uu/chroot` | coreutils |
| `chrt` | O04 | T: `toys/other/chrt.c` | — |
| `chsh` | O11 | T: `toys/pending/chsh.c`; shadow: `src/uu/chsh` | shadow |
| `chvt` | O23 | T: `toys/other/openvt.c` | — |
| `clear` | U24 | T: `toys/other/clear.c` | — |
| `count` | U37 | T: `toys/other/count.c` | — |
| `cpio` | U34 | T: `toys/posix/cpio.c` | T |
| `crc32` | U37 | T: `toys/posix/cksum.c` | — |
| `crond` | O25 | T: `toys/pending/crond.c` | — |
| `crontab` | O25 | T: `toys/pending/crontab.c` | — |
| `csplit` | U21 | T: `toys/pending/csplit.c`; coreutils: `src/uu/csplit` | coreutils |
| `ctrlaltdel` | O27 | util-linux: `src/uu/ctrlaltdel` | util-linux |
| `dd` | U16–U17 | T: `toys/posix/dd.c`; coreutils: `src/uu/dd` | T, coreutils |
| `deallocvt` | O23 | T: `toys/other/openvt.c` | — |
| `delgroup` | O10 | T: `toys/pending/groupdel.c` | — |
| `deluser` | O10 | T: `toys/pending/userdel.c` | — |
| `devmem` | O15 | T: `toys/other/devmem.c` | T |
| `df` | U15 | T: `toys/posix/df.c`; coreutils: `src/uu/df` | coreutils |
| `dhcp` | O26 | T: `toys/pending/dhcp.c` | — |
| `dhcp6` | O26 | T: `toys/pending/dhcp6.c` | — |
| `dhcpd` | O26 | T: `toys/pending/dhcpd.c` | — |
| `dir` | U23 | coreutils: `src/uu/dir` | coreutils |
| `dircolors` | U23 | coreutils: `src/uu/dircolors` | coreutils |
| `dmesg` | O05 | T: `toys/lsb/dmesg.c`; util-linux: `src/uu/dmesg` | util-linux |
| `dnsdomainname` | O06 | T: `toys/lsb/hostname.c` | — |
| `dos2unix` | U37 | T: `toys/other/dos2unix.c` | — |
| `dumpleases` | O26 | T: `toys/pending/dumpleases.c` | — |
| `egrep` | U04–U05 | T: `toys/posix/grep.c` | — |
| `eject` | O12 | T: `toys/other/eject.c` | — |
| `expiry` | O09 | shadow: `src/uu/expiry` | shadow |
| `fallocate` | U39 | T: `toys/other/fallocate.c` | T |
| `fdisk` | O13 | T: `toys/pending/fdisk.c` | — |
| `fgrep` | U04–U05 | T: `toys/posix/grep.c` | — |
| `file` | U35 | T: `toys/posix/file.c` | T |
| `find` | U06–U07 | T: `toys/posix/find.c`; findutils: `src/find/main.rs` | T, findutils |
| `flock` | U39 | T: `toys/other/flock.c` | — |
| `free` | U28 | T: `toys/other/free.c`; procps: `src/uu/free` | procps |
| `freeramdisk` | O12 | T: `toys/other/freeramdisk.c` | — |
| `fsck` | O13 | T: `toys/pending/fsck.c` | — |
| `fsfreeze` | O12 | T: `toys/other/fsfreeze.c`; util-linux: `src/uu/fsfreeze` | util-linux |
| `fstype` | O03 | T: `toys/other/blkid.c` | T |
| `fsync` | U39 | T: `toys/other/fsync.c` | — |
| `ftpget` | O08 | T: `toys/net/ftpget.c` | — |
| `ftpput` | O08 | T: `toys/net/ftpget.c` | — |
| `getconf` | U38 | T: `toys/posix/getconf.c` | — |
| `getenforce` | O16 | T: `toys/android/getenforce.c` | — |
| `getfacl` | O02 | acl: `src/uu/getfacl` | acl |
| `getfattr` | O02 | T: `toys/pending/getfattr.c` | T |
| `getopt` | U38 | T: `toys/other/getopt.c` | T |
| `getty` | O11 | T: `toys/pending/getty.c` | — |
| `gitcheckout` | O28 | T: `toys/pending/git.c` | — |
| `gitclone` | O28 | T: `toys/pending/git.c` | — |
| `gitfetch` | O28 | T: `toys/pending/git.c` | — |
| `gitinit` | O28 | T: `toys/pending/git.c` | — |
| `gitremote` | O28 | T: `toys/pending/git.c` | — |
| `gpasswd` | O11 | shadow: `src/uu/gpasswd` | shadow |
| `gpiodetect` | O15 | T: `toys/other/gpiod.c` | — |
| `gpiofind` | O15 | T: `toys/other/gpiod.c` | — |
| `gpioget` | O15 | T: `toys/other/gpiod.c` | — |
| `gpioinfo` | O15 | T: `toys/other/gpiod.c` | — |
| `gpioset` | O15 | T: `toys/other/gpiod.c` | — |
| `grep` | U04–U05 | T: `toys/posix/grep.c`; grep: `src` | T, grep |
| `groupadd` | O10 | T: `toys/pending/groupadd.c`; shadow: `src/uu/groupadd` | T, shadow |
| `groupdel` | O10 | T: `toys/pending/groupdel.c`; shadow: `src/uu/groupdel` | T, shadow |
| `groupmod` | O10 | shadow: `src/uu/groupmod` | shadow |
| `grpck` | O09 | shadow: `src/uu/grpck` | shadow |
| `grpconv` | O10 | shadow: `src/uu/grpconv` | — |
| `grpunconv` | O10 | shadow: `src/uu/grpunconv` | — |
| `gunzip` | U31 | T: `toys/lsb/gzip.c` | T |
| `gzip` | U31 | T: `toys/lsb/gzip.c` | T |
| `halt` | O27 | T: `toys/other/reboot.c` | — |
| `hd` | U19 | T: `toys/other/hd.c` | — |
| `help` | O23 | T: `toys/other/help.c` | — |
| `hexdump` | U19 | T: `toys/pending/hexdump.c`; util-linux: `src/uu/hexdump` | T, util-linux |
| `hexedit` | O23 | T: `toys/other/hexedit.c` | — |
| `host` | O06 | T: `toys/net/host.c` | — |
| `httpd` | O26 | T: `toys/net/httpd.c` | T |
| `hugetop` | O04 | procps: `src/uu/hugetop` | procps |
| `hwclock` | O15 | T: `toys/other/hwclock.c` | — |
| `i2cdetect` | O15 | T: `toys/other/i2ctools.c` | — |
| `i2cdump` | O15 | T: `toys/other/i2ctools.c` | — |
| `i2cget` | O15 | T: `toys/other/i2ctools.c` | — |
| `i2cset` | O15 | T: `toys/other/i2ctools.c` | — |
| `i2ctransfer` | O15 | T: `toys/other/i2ctools.c` | — |
| `iconv` | O18 | T: `toys/posix/iconv.c` | T |
| `ifconfig` | O07 | T: `toys/net/ifconfig.c` | T |
| `init` | O27 | T: `toys/pending/init.c` | — |
| `inotifyd` | O24 | T: `toys/other/inotifyd.c` | — |
| `insmod` | O14 | T: `toys/other/insmod.c` | — |
| `install` | U14 | T: `toys/posix/cp.c`; coreutils: `src/uu/install` | T, coreutils |
| `ionice` | O04 | T: `toys/other/ionice.c` | — |
| `iorenice` | O04 | T: `toys/other/ionice.c` | — |
| `iotop` | O04 | T: `toys/posix/ps.c` | — |
| `ip` | O07 | T: `toys/pending/ip.c` | — |
| `ipaddr` | O07 | T: `toys/pending/ip.c` | — |
| `ipcrm` | O01 | T: `toys/pending/ipcrm.c` | — |
| `ipcs` | O01 | T: `toys/pending/ipcs.c` | — |
| `iplink` | O07 | T: `toys/pending/ip.c` | — |
| `iproute` | O07 | T: `toys/pending/ip.c` | — |
| `iprule` | O07 | T: `toys/pending/ip.c` | — |
| `iptunnel` | O07 | T: `toys/pending/ip.c` | — |
| `join` | U20 | coreutils: `src/uu/join` | coreutils |
| `killall` | U27 | T: `toys/lsb/killall.c` | T |
| `killall5` | O27 | T: `toys/posix/kill.c` | — |
| `klogd` | O25 | T: `toys/pending/klogd.c` | — |
| `last` | U40 | T: `toys/pending/last.c`; util-linux: `src/uu/last` | util-linux |
| `linux32` | O01 | T: `toys/other/linux32.c` | — |
| `load_policy` | O16 | T: `toys/android/load_policy.c` | — |
| `locate` | O19 | findutils: `src/locate/main.rs` | findutils |
| `log` | O16 | T: `toys/android/log.c` | — |
| `logger` | O05 | T: `toys/posix/logger.c` | — |
| `login` | O11 | T: `toys/other/login.c`; login: `src/uu/login`; shadow: `src/uu/login` | shadow |
| `losetup` | O12 | T: `toys/other/losetup.c` | T |
| `lsattr` | O02 | T: `toys/other/lsattr.c` | T |
| `lscpu` | O03 | util-linux: `src/uu/lscpu` | util-linux |
| `lsipc` | O03 | util-linux: `src/uu/lsipc` | — |
| `lslocks` | O03 | util-linux: `src/uu/lslocks` | util-linux |
| `lsmem` | O03 | util-linux: `src/uu/lsmem` | util-linux |
| `lsmod` | O14 | T: `toys/other/lsmod.c` | — |
| `lsns` | O03 | util-linux: `src/uu/lsns` | util-linux |
| `lsof` | O04 | T: `toys/pending/lsof.c` | — |
| `lspci` | O15 | T: `toys/other/lsusb.c` | — |
| `lsusb` | O15 | T: `toys/other/lsusb.c` | — |
| `makedevs` | O12 | T: `toys/other/makedevs.c` | — |
| `mdev` | O27 | T: `toys/pending/mdev.c` | — |
| `memeater` | O24 | T: `toys/other/memeater.c` | — |
| `mesg` | U40 | util-linux: `src/uu/mesg` | util-linux |
| `microcom` | O08 | T: `toys/net/microcom.c` | — |
| `mix` | O23 | T: `toys/other/mix.c` | — |
| `mknod` | O01 | T: `toys/lsb/mknod.c`; coreutils: `src/uu/mknod` | coreutils |
| `mkpasswd` | O22 | T: `toys/other/mkpasswd.c` | T |
| `mkswap` | O12 | T: `toys/other/mkswap.c` | — |
| `modinfo` | O14 | T: `toys/other/modinfo.c` | T |
| `modprobe` | O14 | T: `toys/pending/modprobe.c` | — |
| `more` | U25 | T: `toys/pending/more.c`; coreutils: `src/uu/more` | T, coreutils |
| `mount` | O12 | T: `toys/lsb/mount.c` | T |
| `mountpoint` | O03 | T: `toys/other/mountpoint.c`; util-linux: `src/uu/mountpoint` | util-linux |
| `nbd-client` | O26 | T: `toys/other/nbd_client.c` | T |
| `nbd-server` | O26 | T: `toys/other/nbd_server.c` | — |
| `nbd_client` | O26 | T: `toys/other/nbd_client.c` | — |
| `nbd_server` | O26 | T: `toys/other/nbd_server.c` | — |
| `nc` | O06 | T: `toys/net/netcat.c` | — |
| `netcat` | O06 | T: `toys/net/netcat.c` | T |
| `netstat` | O07 | T: `toys/net/netstat.c` | — |
| `newgidmap` | O11 | shadow: `src/uu/newgidmap` | — |
| `newgrp` | O11 | shadow: `src/uu/newgrp` | shadow |
| `newuidmap` | O11 | shadow: `src/uu/newuidmap` | shadow |
| `newusers` | O10 | shadow: `src/uu/newusers` | shadow |
| `nfsmount` | O12 | T: `toys/lsb/mount.c` | — |
| `nohup` | U29 | T: `toys/posix/nohup.c`; coreutils: `src/uu/nohup` | coreutils |
| `nologin` | O09 | T: `toys/other/nologin.c`; util-linux: `src/uu/nologin` | util-linux |
| `nsenter` | O01 | T: `toys/other/nsenter.c` | — |
| `od` | U18 | T: `toys/posix/od.c`; coreutils: `src/uu/od` | coreutils |
| `oneit` | O27 | T: `toys/other/oneit.c` | — |
| `openvt` | O23 | T: `toys/other/openvt.c` | — |
| `partprobe` | O12 | T: `toys/other/partprobe.c` | — |
| `passwd` | O11 | T: `toys/lsb/passwd.c`; shadow: `src/uu/passwd` | shadow |
| `patch` | U36 | T: `toys/posix/patch.c` | T |
| `pidof` | U27 | T: `toys/lsb/pidof.c`; procps: `src/uu/pidof` | T, procps |
| `pidwait` | U27 | procps: `src/uu/pidwait` | procps |
| `ping` | O08 | T: `toys/net/ping.c` | — |
| `ping6` | O08 | T: `toys/net/ping.c` | — |
| `pinky` | U40 | coreutils: `src/uu/pinky` | coreutils |
| `pivot_root` | O27 | T: `toys/other/pivot_root.c` | — |
| `pmap` | O04 | T: `toys/other/pmap.c`; procps: `src/uu/pmap` | procps |
| `poweroff` | O27 | T: `toys/other/reboot.c` | — |
| `pr` | U22 | coreutils: `src/uu/pr` | coreutils |
| `prlimit` | O04 | T: `toys/posix/ulimit.c` | — |
| `ps` | U26 | T: `toys/posix/ps.c`; procps: `src/uu/ps` | T, procps |
| `ptx` | O21 | coreutils: `src/uu/ptx` | coreutils |
| `pwck` | O09 | shadow: `src/uu/pwck` | shadow |
| `pwconv` | O10 | shadow: `src/uu/pwconv` | shadow |
| `pwdx` | U27 | T: `toys/other/pwdx.c`; procps: `src/uu/pwdx` | procps |
| `pwgen` | O22 | T: `toys/other/pwgen.c` | — |
| `pwunconv` | O10 | shadow: `src/uu/pwunconv` | — |
| `readahead` | O12 | T: `toys/other/readahead.c` | — |
| `readelf` | U35 | T: `toys/other/readelf.c` | T |
| `reboot` | O27 | T: `toys/other/reboot.c` | — |
| `rename` | O20 | util-linux: `src/uu/rename` | util-linux |
| `renice` | U29 | T: `toys/posix/renice.c`; bsdutils: `src/uu/renice`; util-linux: `src/uu/renice` | T, bsdutils, util-linux |
| `reset` | U24 | T: `toys/other/reset.c` | — |
| `restorecon` | O16 | T: `toys/android/restorecon.c` | — |
| `rfkill` | O07 | T: `toys/net/rfkill.c` | — |
| `rmmod` | O14 | T: `toys/other/rmmod.c` | — |
| `route` | O07 | T: `toys/pending/route.c` | — |
| `rtcwake` | O15 | T: `toys/other/rtcwake.c` | — |
| `runcon` | O16 | T: `toys/android/runcon.c`; coreutils: `src/uu/runcon` | coreutils |
| `sed` | U10–U12 | T: `toys/posix/sed.c`; red: `red/src`; sed: `src` | T, sed |
| `sendevent` | O15 | T: `toys/android/sendevent.c` | — |
| `setenforce` | O16 | T: `toys/android/setenforce.c` | — |
| `setfacl` | O02 | acl: `src/uu/setfacl` | acl |
| `setfattr` | O02 | T: `toys/other/setfattr.c` | T |
| `setpgid` | U29 | util-linux: `src/uu/setpgid` | util-linux |
| `setsid` | U29 | T: `toys/other/setsid.c`; util-linux: `src/uu/setsid` | util-linux |
| `sg` | O11 | shadow: `src/uu/sg` | shadow |
| `shred` | O20 | T: `toys/other/shred.c`; coreutils: `src/uu/shred` | coreutils |
| `skill` | O04 | procps: `src/uu/skill` | procps |
| `slabtop` | O04 | procps: `src/uu/slabtop` | procps |
| `snice` | O04 | procps: `src/uu/snice` | procps |
| `sntp` | O08 | T: `toys/net/sntp.c` | — |
| `stdbuf` | O21 | coreutils: `src/uu/stdbuf` | coreutils |
| `strace` | O24 | T: `toys/pending/strace.c` | — |
| `stty` | U24 | T: `toys/pending/stty.c`; coreutils: `src/uu/stty` | coreutils |
| `su` | O11 | T: `toys/lsb/su.c` | — |
| `sulogin` | O11 | T: `toys/pending/sulogin.c` | — |
| `swapoff` | O12 | T: `toys/other/swapoff.c` | — |
| `swapon` | O12 | T: `toys/other/swapon.c` | — |
| `switch_root` | O27 | T: `toys/other/switch_root.c` | — |
| `sysctl` | O05 | T: `toys/other/sysctl.c`; procps: `src/uu/sysctl` | procps |
| `syslogd` | O25 | T: `toys/pending/syslogd.c` | — |
| `tar` | U32–U33 | T: `toys/posix/tar.c`; tar: `src/uu/tar` | T, tar |
| `taskset` | O04 | T: `toys/other/taskset.c` | T |
| `tcpsvd` | O26 | T: `toys/pending/tcpsvd.c` | — |
| `telnet` | O08 | T: `toys/pending/telnet.c` | — |
| `telnetd` | O26 | T: `toys/pending/telnetd.c` | — |
| `test` | U01 | T: `toys/posix/test.c`; coreutils: `src/uu/test` | T, coreutils |
| `tftp` | O08 | T: `toys/pending/tftp.c` | — |
| `tftpd` | O26 | T: `toys/pending/tftpd.c` | — |
| `time` | U30 | T: `toys/posix/time.c` | — |
| `tload` | O04 | procps: `src/uu/tload` | procps |
| `traceroute` | O08 | T: `toys/pending/traceroute.c` | — |
| `traceroute6` | O08 | T: `toys/pending/traceroute.c` | — |
| `ts` | U37 | T: `toys/other/ts.c` | — |
| `tunctl` | O07 | T: `toys/net/tunctl.c` | — |
| `uclampset` | O04 | T: `toys/other/uclampset.c` | — |
| `ucsicontrol` | O15 | T: `toys/other/ucsicontrol.c` | — |
| `udpsvd` | O26 | T: `toys/pending/tcpsvd.c` | — |
| `ulimit` | S15 | T: `toys/posix/ulimit.c` | — |
| `umount` | O12 | T: `toys/lsb/umount.c` | — |
| `unicode` | O18 | T: `toys/other/ascii.c` | T |
| `unix2dos` | U37 | T: `toys/other/dos2unix.c` | — |
| `unshare` | O01 | T: `toys/other/nsenter.c` | — |
| `updatedb` | O19 | findutils: `src/updatedb/main.rs` | findutils |
| `uptime` | U28 | T: `toys/other/uptime.c`; coreutils: `src/uu/uptime` | T, coreutils |
| `useradd` | O10 | T: `toys/pending/useradd.c`; shadow: `src/uu/useradd` | T, shadow |
| `userdel` | O10 | T: `toys/pending/userdel.c`; shadow: `src/uu/userdel` | shadow |
| `usermod` | O10 | shadow: `src/uu/usermod` | shadow |
| `users` | U40 | coreutils: `src/uu/users` | coreutils |
| `usleep` | U37 | T: `toys/other/usleep.c` | — |
| `vconfig` | O07 | T: `toys/other/vconfig.c` | — |
| `vdir` | U23 | coreutils: `src/uu/vdir` | coreutils |
| `vigr` | O10 | shadow: `src/uu/vigr` | — |
| `vipw` | O10 | shadow: `src/uu/vipw` | shadow |
| `vmstat` | U28 | T: `toys/other/vmstat.c`; procps: `src/uu/vmstat` | procps |
| `w` | U40 | T: `toys/other/w.c`; procps: `src/uu/w` | procps |
| `watch` | U30 | T: `toys/other/watch.c`; procps: `src/uu/watch` | procps |
| `watchdog` | O27 | T: `toys/other/watchdog.c` | — |
| `wget` | O06 | T: `toys/net/wget.c` | — |
| `who` | U40 | T: `toys/posix/who.c`; coreutils: `src/uu/who` | coreutils |
| `xargs` | U08–U09 | T: `toys/posix/xargs.c`; findutils: `src/xargs/main.rs` | T, findutils |
| `xzcat` | O17 | T: `toys/pending/xzcat.c` | T |
| `zcat` | U31 | T: `toys/lsb/gzip.c` | T |

## Selection and test-risk notes

- High-value command families are split across baseline and extended-semantics PRs, rather than one implementation PR per alias.
- Shell execution and registry resolution make existing utilities usable inside pipelines; they should precede chasing a nominal applet count.
- Filesystem/account/network/device mutators require isolated integration environments. Their names in this inventory do not authorize invoking them against the research host.
- Toybox fsck and similar tools may orchestrate external helpers. Inspect the implementation dependency chain before claiming self-containment.
- Upstream Rust implementations can rely on libraries not included in the approved source set. Their existence is not permission to inspect those dependencies; use permitted references, existing public APIs or native implementation work.
- Tests copied or invoked from a non-approved upstream remain outside scope even if an allowed repository offers a harness for them. In particular, do not run download scripts for GNU suites as part of this roadmap.
- New utility additions should refresh this inventory from the registry, not manually decrement a stale total.

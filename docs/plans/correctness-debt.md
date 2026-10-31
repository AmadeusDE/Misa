# F08 — existing utility correctness debt audit

This is the **initial baseline**, not a claim of upstream compatibility or a
new test denominator. It records every entry present in both known-failure
lists when the audit began. Results are host-dependent and `skip` never means
pass. F07's [accounting](upstream-test-accounting.md) and
[execution delta](upstream-test-execution-delta.md) describe the adapter's
coverage and safety limitations. No safety guard was bypassed. Baseline used
local fixtures, Linux and the host filesystem; no upstream material was imported.
Toybox's approved pin is 0BSD, uutils/coreutils' approved pin is MIT; see
`internal/inventory/sources.json` and `tests/provenance/fixtures.json` for
revisions and per-file provenance (repository license alone is not proof of
fixture provenance). This audit is not based on excluded sources.

## Reproduce

Run from the repository root. These commands target every utility with a listed
case, so also execute other cases of those utilities; failures and exit 1 are
expected on the initial revision. `-count=1` disables the test cache.

```sh
env -u OPENROUTER_API_KEY MISA_KNOWN_FAILURES=1 MISA_TEST_REPORT=/tmp/f08-audit-toybox-report go test -json -count=1 ./tests/toybox -run '^TestToyboxSuite/(chmod|mv|cp|du|env)/' > /tmp/f08-audit-toybox.json 2>&1
env -u OPENROUTER_API_KEY MISA_KNOWN_FAILURES=1 MISA_TEST_REPORT=/tmp/f08-audit-uutils-report go test -json -count=1 ./tests/uutils -run '^TestUutilsSuite/(id|ln|ls|mv|nl|numfmt|rmdir|seq|shuf|sort|touch|wc)/' > /tmp/f08-audit-uutils.json 2>&1
```

The JSON contains exact arguments, stdout/stderr and individual subtest
outcomes; report directories contain adapter records. For one case, substitute
`^TestToyboxSuite/<utility>/<case>$` or `^TestUutilsSuite/<utility>/<case>$`
for the `-run` expression (escape regex metacharacters in the case name).
The tables below preserve the full exact case names for that purpose.

Categories describe the observed failure, not necessarily the only possible
underlying issue: **utility** (link/copy/move/env/other), **adapter**,
**environment/filesystem**, **format/locale/random**, or **upstream-only
quirk**. Exclusion comments are hypotheses, not evidence. “Fail” means the
adapter reported failure; it does not prove the native upstream test ran in full.

## Toybox baseline

| Case | Result | Classification and observed vs expected |
| --- | --- | --- |
| `chmod/u-x_dir_file` | fail | utility: other / adapter: chmod verification reports failure even when the requested execute bit was removed (diagnostic shows identical modes); multiline setup is also split into invalid shell fragments; empty output vs expected permissions |
| `chmod/g-x_dir_file` | fail | utility: other / adapter: chmod verification reports failure even when the requested execute bit was removed (diagnostic shows identical modes); multiline setup is also split into invalid shell fragments; empty output vs expected permissions |
| `chmod/o-x_dir_file` | fail | utility: other / adapter: chmod verification reports failure even when the requested execute bit was removed (diagnostic shows identical modes); multiline setup is also split into invalid shell fragments; empty output vs expected permissions |
| `mv/interactive:_no_stdin` | fail | adapter / upstream-only quirk: setup reports stale `link1`; decline produces no `yes` vs expected `yes` (status semantics also differ); not an isolated native result |
| `mv/interactive:_answered_NO` | fail | adapter / upstream-only quirk: setup reports stale `link1`; decline produces no `yes` vs expected `yes` (status semantics also differ); not an isolated native result |
| `cp/symlink_dest_permissions` | fail | utility: copy: symlink **source** `woot -> walrus` (0644) copied to new `carpenter` is 0664 vs 0644 with umask 0002; exclusion calls it destination |
| `du/(no_options)` | pass | environment/filesystem: matched expected block counts here; not proof across filesystems |
| `du/-s` | pass | environment/filesystem: matched expected block counts here; not proof across filesystems |
| `du/counts_symlinks_without_following` | skip | adapter / environment/filesystem: skipped: safety guard rejects setup path `../du_2` as escaping sandbox; symlink/block-count behavior not observed |
| `du/-L_follows_symlinks` | skip | adapter / environment/filesystem: skipped: safety guard rejects setup path `../du_2` as escaping sandbox; symlink/block-count behavior not observed |
| `du/-L_avoid_endless_loop` | skip | adapter / environment/filesystem: skipped: safety guard rejects setup path `../du_2` as escaping sandbox; symlink/block-count behavior not observed |
| `du/-HL_follows_symlinks` | skip | adapter / environment/filesystem: skipped: safety guard rejects setup path `../du_2` as escaping sandbox; symlink/block-count behavior not observed |
| `du/-H_does_not_follow_unspecified_symlinks` | skip | adapter / environment/filesystem: skipped: safety guard rejects setup path `../du_2` as escaping sandbox; symlink/block-count behavior not observed |
| `du/-LH_does_not_follow_unspecified_symlinks` | skip | adapter / environment/filesystem: skipped: safety guard rejects setup path `../du_2` as escaping sandbox; symlink/block-count behavior not observed |
| `du/-H_follows_specified_symlinks` | skip | adapter / environment/filesystem: skipped: safety guard rejects setup path `../du_2` as escaping sandbox; symlink/block-count behavior not observed |
| `env/why_is_this_allowed` | fail | upstream-only quirk / reference conflict: Toybox expects `=BLAH` to pass through; Misa warns and drops it, matching the uutils empty-name cases; the batch intentionally retains uutils behavior |

## MIT uutils baseline

| Case | Result | Classification and observed vs expected |
| --- | --- | --- |
| `id/test_id_digital_username_0_L580` | fail | environment/filesystem / adapter: fixture assumes user named `200`; host reports no such user (exit 1 vs 0) |
| `ln/test_symlink_remove_existing_same_src_and_dest_0_L971` | fail | utility: link: `ln -sf a a` exits 0 instead of 1 and lacks same-file diagnostic |
| `ls/test_invalid_value_returns_1_7_L10` | fail | utility: other: invalid `--time` gives exit 2 / takes no argument vs expected exit 1 |
| `ls/test_invalid_value_time_style_1_L153` | fail | format/locale/random: invalid time-style is rejected but stderr wording differs from expected |
| `ls/test_time_style_empty_after_posix_prefix_L171` | fail | format/locale/random: invalid time-style is rejected but stderr wording differs from expected |
| `ls/test_long_options_detached_2_L7699` | fail | utility: other: `--time mtime` treats `mtime` as filename, exit 1 vs 0 |
| `mv/test_mv_same_file_dot_dir_0_L824` | fail | utility: move: same-file error says `'.' and './.'` vs `'.' and '.'` |
| `mv/test_mv_hardlink_preservation_0_L2725` | fail | utility: move: distinct hardlinked sources trigger duplicate-source warning vs expected empty stderr; adapter fixture only checks target existence, not inode equality; native regression additionally observed the second source skipped |
| `nl/test_negative_line_increment_L1` | fail | utility: other: negative increment prints first line then range error / exit 1, vs three numbered lines and exit 0 |
| `nl/test_negative_line_increment_1_L1` | fail | utility: other: negative increment prints first line then range error / exit 1, vs three numbered lines and exit 0 |
| `numfmt/test_locale_fr_output_L1689` | fail | format/locale/random: decimal `1.5K` vs French `1,5K` |
| `numfmt/test_locale_fr_input_comma_L1700` | fail | format/locale/random: comma input rejected vs expected `1,500` |
| `numfmt/test_locale_fr_rejects_period_L1711` | fail | format/locale/random: period input accepted vs expected invalid in French locale |
| `rmdir/test_rmdir_nonempty_directory_no_parents_0_L68` | fail | format/locale/random: error says `directory not empty` vs `Directory not empty` |
| `rmdir/test_rmdir_nonempty_directory_with_parents_0_L82` | fail | format/locale/random: error says `directory not empty` vs `Directory not empty` |
| `seq/test_equalize_widths_corner_cases_L1124` | fail | upstream-only quirk: hex operand with `-w` gets excessive zero padding vs fixture output |
| `seq/test_equalize_widths_corner_cases_1_L1129` | fail | upstream-only quirk: hex operand with `-w` gets excessive zero padding vs fixture output |
| `seq/test_equalize_widths_corner_cases_2_L1144` | fail | upstream-only quirk: hex operand with `-w` gets excessive zero padding vs fixture output |
| `shuf/test_gnu_compat_range_repeat_0_L960` | skip | adapter / format/locale/random: skipped: unsupported `at.append_bytes` random fixture; generator behavior not observed |
| `shuf/test_gnu_compat_from_file_0_L1011` | skip | adapter / format/locale/random: skipped: unsupported `at.append_bytes` random fixture; generator behavior not observed |
| `shuf/test_gnu_compat_limited_from_file_0_L1028` | skip | adapter / format/locale/random: skipped: unsupported `at.append_bytes` random fixture; generator behavior not observed |
| `shuf/test_gnu_compat_range_no_repeat_0_L1064` | skip | adapter / format/locale/random: skipped: unsupported `at.append_bytes` random fixture; generator behavior not observed |
| `shuf/test_seed_args_repeat_L1077` | fail | format/locale/random: seeded output differs from expected deterministic sequence |
| `shuf/test_seed_args_no_repeat_L1090` | fail | format/locale/random: seeded output differs from expected deterministic sequence |
| `shuf/test_seed_range_repeat_L1101` | fail | format/locale/random: seeded output differs from expected deterministic sequence |
| `shuf/test_seed_range_no_repeat_0_L1114` | fail | format/locale/random: seeded output differs from expected deterministic sequence |
| `shuf/test_seed_range_no_repeat_1_L1121` | fail | format/locale/random: seeded output differs from expected deterministic sequence |
| `sort/test_version_sort_unstable_L206` | fail | format/locale/random: version ordering puts `0.002` before `0.1`, fixture expects `0.1` first (stable tie order also differs) |
| `sort/test_version_sort_unstable_1_L211` | fail | format/locale/random: version ordering puts `0.002` before `0.1`, fixture expects `0.1` first (stable tie order also differs) |
| `sort/test_version_sort_unstable_2_L216` | fail | format/locale/random: version ordering puts `0.002` before `0.1`, fixture expects `0.1` first (stable tie order also differs) |
| `sort/test_version_sort_stable_L225` | fail | format/locale/random: version ordering puts `0.002` before `0.1`, fixture expects `0.1` first (stable tie order also differs) |
| `sort/test_numeric_sort_rejects_leading_plus_sign_L319` | fail | utility: other: `-n` orders `+1,+2,+10` numerically vs expected `+1,+10,+2` |
| `sort/test_random_source_is_not_read_without_random_sort_0_L485` | fail | utility: other: reads empty random source without `-R`, exits 2 vs sorted output and exit 0 |
| `sort/test_locale_interleaved_en_us_utf8_L3239` | fail | format/locale/random: bytewise case/punctuation order differs from locale collation expected by fixture |
| `sort/test_locale_complex_utf8_sorting_0_L3315` | fail | format/locale/random: bytewise case/punctuation order differs from locale collation expected by fixture |
| `sort/test_consistent_sorting_with_i18n_collate_0_L3380` | fail | format/locale/random: bytewise case/punctuation order differs from locale collation expected by fixture |
| `touch/test_touch_mtime_dst_fails_0_L800` | fail | environment/filesystem / utility: other: DST gap timestamp accepted vs expected failure; requires relevant timezone; host TZ and fixture environment matter |
| `wc/test_single_default_L244` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_single_all_counts_L268` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_file_many_empty_lines_L353` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_file_one_long_line_only_spaces_L362` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_file_one_long_word_L371` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_file_bytes_dictate_width_0_L397` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_total_auto_L572` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_total_auto_1_L576` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_total_never_L620` | fail | format/locale/random: counts match but fields lack expected leading width padding |
| `wc/test_args_override_1_L847` | fail | format/locale/random: counts match but fields lack expected leading width padding |

## First batch: resolved versus retained

Native regressions ran red before the implementation changes. Four previously
failing exclusions now execute and pass, and were removed:

- `cp/symlink_dest_permissions`: new regular files use the followed source's
  mode masked by umask, not an unconditional 0666. Existing destinations,
  including symlink targets, retain their mode unless preservation is requested.
- `ln/test_symlink_remove_existing_same_src_and_dest_0_L971`: forcing a symbolic
  link onto its existing source fails without replacing the source entry.
- `mv/test_mv_same_file_dot_dir_0_L824`: the same-file diagnostic names `.`
  rather than `./.` as the destination of `mv . .`.
- `mv/test_mv_hardlink_preservation_0_L2725`: distinct hardlinked source
  operands are moved rather than skipped as duplicate inodes. The native
  regression also checks both old paths disappear and the new files share an
  inode on the same filesystem. Cross-filesystem preservation is not established.

Final focused verification (both commands exit 0):

```sh
env -u OPENROUTER_API_KEY MISA_KNOWN_FAILURES=1 go test -count=1 ./tests/toybox -run '^TestToyboxSuite/cp/symlink_dest_permissions$'
env -u OPENROUTER_API_KEY MISA_KNOWN_FAILURES=1 go test -count=1 ./tests/uutils -run '^TestUutilsSuite/(ln|mv)/(test_symlink_remove_existing_same_src_and_dest_0_L971|test_mv_same_file_dot_dir_0_L824|test_mv_hardlink_preservation_0_L2725)$'
```

### Env reference conflict: intentionally unchanged

Toybox expects empty-name assignments to pass through. MIT uutils/coreutils
`tests/by-util/test_env.rs` at approved pin
`fee61ff5d4f87e0f912a52e639942b5f7ad58663` (`test_empty_name`, around line 338)
expects a warning and removal. Misa keeps the previously passing uutils behavior;
the Toybox exclusion now records this explicit policy. An intermediate change
accepted empty names but regressed two passing uutils cases, so it was reverted.
The pinned `test_ln.rs` and `test_mv.rs` also corroborate the link/move scenarios
above. References were inspected, not imported; local fixture provenance remains
separate from the approved source pins.

Final conflict verification: Toybox's leaf fails as expected (exit 1), while both
uutils leaves pass (exit 0):

```sh
env -u OPENROUTER_API_KEY MISA_KNOWN_FAILURES=1 go test -count=1 ./tests/toybox -run '^TestToyboxSuite/env/why_is_this_allowed$'
env -u OPENROUTER_API_KEY MISA_KNOWN_FAILURES=1 go test -count=1 ./tests/uutils -run '^TestUutilsSuite/env/(test_empty_name_L370|test_empty_name_and_empty_value_L395)$'
```

## Follow-ups and limits

- **F08 later batches:** chmod verification and multiline adapter setup, mv
  interactive wildcard cleanup/decline semantics, other utility bugs, and any
  separately reproduced cross-filesystem copy/move/hardlink defects.
- **F08/U44 compatibility batches:** `wc` widths, `seq` hexadecimal padding,
  `shuf` seeded sequences, `sort` ordering/random-source handling, `ls --time`,
  and `rmdir` diagnostics remain unchanged. Keep formatting and generator
  differences separate from filesystem correctness work.
- **O29:** selected locale collation, numeric and date behavior after dependency
  and size review (`numfmt`, locale-sensitive `sort`, and timezone concerns).
- Host/filesystem assumptions and adapter limitations remain visible. The two
  initially passing `du` cases do not prove portability; seven `du` and four
  `shuf` exclusions were skipped by safety/setup guards and remain unverified.
  No safety guard was loosened and no fixture bytes were changed.

The baseline covered 16 Toybox entries (7 failures, 2 passes, 7 skips) and
47 uutils entries (43 failures, 4 skips). This is not a new whole-suite
compatibility denominator. Utility-comparison and builtins-coverage Totals
remain unchanged.

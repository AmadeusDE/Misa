# F07 execution-change summary

This is a historical before/after audit on the development host, not a generated
current conformance report. No builtin behavior changed and existing known-failure
exclusions were retained. Additional discovery records are excluded from this
comparison because the old adapters silently omitted them.

| Suite | Before executed/passed | Before parsed skips | After executed/passed | After parsed skips | Newly skipped |
| --- | ---: | ---: | ---: | ---: | ---: |
| Toybox | 760 | 55 | 752 | 63 | 8 |
| uutils | 2874 | 208 | 2810 | 272 | 64 |

The Toybox guard is best-effort over reviewed fixtures, not a sandbox. Uutils
refusals include conservative containment checks and unsupported setup that the
old adapter silently ignored; these are not newly discovered builtin bugs.
Wrapper refusals include cases that may only validate invalid operands today: the
adapter does not assume that future implementation changes remain no-child or
process-state-preserving.

## Toybox: every formerly executed case now skipped

| Case | Reason |
| --- | --- |
| `mktemp/-q_shouldn't_print_path` | literal /proc mutation path |
| `mktemp/-u_doesn't_need_to_be_able_to_write_to_dir` | literal /proc mutation path |
| `touch/-t_CCMMDDhhmm` | expected-output command substitution refused |
| `touch/-t_MMDDhhmm` | expected-output command substitution refused |
| `touch/-t_MMDDhhmm#01` | expected-output command substitution refused |
| `touch/-t_YYMMDDhhmm` | expected-output command substitution refused |
| `uuencode/uu_1-char` | expected-output command substitution refused |
| `uuencode/uu_2-char` | expected-output command substitution refused |

## uutils: every formerly executed case now skipped

| Case | Reason |
| --- | --- |
| `cp/test_canonicalize_symlink_0_L3436` | setup: unsafe symlink target "../dir/file" not allowed |
| `cp/test_copy_through_dangling_symlink_0_L3472` | unsafe filesystem argument "target" |
| `cp/test_copy_through_dangling_symlink_force_0_L3604` | unsafe filesystem argument "dest" |
| `cp/test_copy_through_dangling_symlink_posixly_correct_0_L3484` | unsafe filesystem argument "target" |
| `cp/test_cp_archive_recursive_0_L2617` | unsupported fixture setup: at.symlink_file("2", &file_2_link.to_string_lossy()) |
| `cp/test_cp_attributes_only_0_L4913` | unsupported fixture setup: at.set_mode(b, mode_b) |
| `cp/test_cp_copy_symlink_contents_recursive_0_L4116` | unsupported fixture setup: at.relative_symlink_file("no-file", &path_concat!("src-dir", "slink")) |
| `cp/test_cp_from_stream_permission_0_L7412` | unsupported fixture setup: at.set_mode("target", mode) |
| `cp/test_cp_parents_recursive_source_ending_in_parent_dir_0_L4098` | unsafe filesystem argument "src/sub/.." |
| `cp/test_cp_symbolic_link_loop_0_L4238` | unsupported fixture setup: at.plus("loop") |
| `cp/test_remove_destination_symbolic_link_loop_0_L4225` | unsupported fixture setup: at.plus("loop") |
| `env/test_braced_variable_error_missing_closing_brace_L2024` | env: no-child, process-state-preserving validation could not be established |
| `env/test_braced_variable_error_rejects_default_syntax_L2032` | env: no-child, process-state-preserving validation could not be established |
| `env/test_braced_variable_error_starts_with_digit_L2040` | env: no-child, process-state-preserving validation could not be established |
| `env/test_braced_variable_error_unexpected_character_L2048` | env: no-child, process-state-preserving validation could not be established |
| `env/test_emoji_env_vars_L1951` | env: no-child, process-state-preserving validation could not be established |
| `env/test_env_block_realtime_signal_1_L1073` | env: no-child, process-state-preserving validation could not be established |
| `env/test_env_block_realtime_signal_L1068` | env: no-child, process-state-preserving validation could not be established |
| `env/test_env_block_signal_flag_L1058` | env: no-child, process-state-preserving validation could not be established |
| `env/test_env_permissions_L152` | env: no-child, process-state-preserving validation could not be established |
| `env/test_fail_null_with_program_L435` | env: no-child, process-state-preserving validation could not be established |
| `env/test_flags_after_command_1_L85` | env: no-child, process-state-preserving validation could not be established |
| `env/test_flags_after_command_2_L93` | env: no-child, process-state-preserving validation could not be established |
| `env/test_flags_after_command_3_L101` | env: no-child, process-state-preserving validation could not be established |
| `env/test_flags_after_command_4_L108` | env: no-child, process-state-preserving validation could not be established |
| `env/test_flags_after_command_5_L115` | env: no-child, process-state-preserving validation could not be established |
| `env/test_flags_after_command_6_L122` | env: no-child, process-state-preserving validation could not be established |
| `env/test_flags_after_command_L79` | env: no-child, process-state-preserving validation could not be established |
| `env/test_reject_shell_style_variable_expansions_1_L6` | env: no-child, process-state-preserving validation could not be established |
| `env/test_reject_shell_style_variable_expansions_2_L6` | env: no-child, process-state-preserving validation could not be established |
| `env/test_reject_shell_style_variable_expansions_3_L6` | env: no-child, process-state-preserving validation could not be established |
| `env/test_reject_shell_style_variable_expansions_L6` | env: no-child, process-state-preserving validation could not be established |
| `env/test_simple_braced_variable_L2015` | env: no-child, process-state-preserving validation could not be established |
| `ln/test_symlink_overwrite_force_0_L108` | unsafe filesystem argument "test_symlink_overwrite_force_link" |
| `ls/test_invalid_utf8_0_L5514` | unsupported fixture setup: at.touch(filename) |
| `mkdir/test_mkdir_trailing_dot_2_L455` | unsafe filesystem argument "test_dir_b/.." |
| `mkdir/test_recursive_reporting_2_L292` | unsafe filesystem argument "test_dir/../test_dir_a/../test_dir_b" |
| `mv/test_mv_dangling_symlink_to_folder_0_L576` | unsafe filesystem argument "abc" |
| `mv/test_mv_file_to_broken_symlink_directory_0_L490` | unsafe filesystem argument "broken" |
| `mv/test_mv_file_to_broken_symlink_file_0_L475` | unsafe filesystem argument "broken" |
| `mv/test_mv_same_broken_symlink_0_L636` | unsafe filesystem argument "broken" |
| `nice/test_bare_adjustment_L73` | nice: no-child, process-state-preserving validation could not be established |
| `nice/test_command_where_command_takes_n_flag_L60` | nice: no-child, process-state-preserving validation could not be established |
| `nice/test_command_with_args_L52` | nice: no-child, process-state-preserving validation could not be established |
| `nice/test_command_with_no_adjustment_L39` | nice: no-child, process-state-preserving validation could not be established |
| `nice/test_command_with_no_args_L44` | nice: no-child, process-state-preserving validation could not be established |
| `nice/test_nice_adj_negative_L18` | nice: no-child, process-state-preserving validation could not be established |
| `nice/test_nice_huge_L92` | nice: no-child, process-state-preserving validation could not be established |
| `nice/test_nice_huge_negative_L104` | nice: no-child, process-state-preserving validation could not be established |
| `rm/test_preserve_root_literal_root_L1588` | unsafe filesystem argument "/" |
| `rmdir/test_rmdir_remove_symlink_dangling_0_L240` | unsafe filesystem argument "dl/" |
| `split/test_elide_dev_null_l_chunks_0_L1239` | unsafe filesystem argument "/dev/null" |
| `split/test_elide_dev_null_n_chunks_0_L1200` | unsafe filesystem argument "/dev/null" |
| `split/test_io_error_L2139` | unsafe filesystem argument "/proc/self/mem" |
| `split/test_number_by_lines_kth_dev_null_L1309` | unsafe filesystem argument "/dev/null" |
| `timeout/test_command_cannot_invoke_L279` | timeout: no-child, process-state-preserving validation could not be established |
| `timeout/test_command_not_found_L271` | timeout: no-child, process-state-preserving validation could not be established |
| `timeout/test_kill_subprocess_L222` | timeout: no-child, process-state-preserving validation could not be established |
| `timeout/test_realtime_signal_names_1_L346` | timeout: no-child, process-state-preserving validation could not be established |
| `timeout/test_realtime_signal_names_2_L350` | timeout: no-child, process-state-preserving validation could not be established |
| `timeout/test_realtime_signal_names_L341` | timeout: no-child, process-state-preserving validation could not be established |
| `timeout/test_with_background_child_L302` | timeout: no-child, process-state-preserving validation could not be established |
| `touch/test_touch_trailing_slash_no_create_2_L994` | unsafe filesystem argument "dangling/" |
| `touch/test_touch_trailing_slash_no_create_3_L1000` | unsafe filesystem argument "loop/" |

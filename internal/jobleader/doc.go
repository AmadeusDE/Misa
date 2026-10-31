// Package jobleader runs one command the way an interactive shell runs a job.
// The leader holds the terminal's session and puts the command in its own
// foreground process group. A program alone in its session cannot stop on ^Z
// (the kernel drops SIGTSTP for orphaned process groups); under a leader it
// can, and the leader then stops itself so whoever watches the session (Misa)
// sees the suspension, and resumes the job when it is continued.
//
// Suspension needs POSIX sessions and process groups. Where Supported is false
// (Windows), Run only runs the command and waits, and ^Z stays ordinary input.
package jobleader

// Flag selects leader mode on the Misa binary: misa --job-leader cmd args...
const Flag = "--job-leader"

package pkgmanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ListAction struct {
	Cmd        []string `yaml:"cmd"`
	SkipHeader bool     `yaml:"skip_header"`
	ItemDesc   string   `yaml:"item_desc"`
	ItemAction []string `yaml:"item_action"`
}

type Action struct {
	Title        string      `yaml:"title"`
	Desc         string      `yaml:"desc"`
	Run          []string    `yaml:"run,omitempty"`
	ShellPrefill string      `yaml:"shell_prefill,omitempty"`
	List         *ListAction `yaml:"list,omitempty"`
}

type Manager struct {
	Name    string   `yaml:"name"`
	Desc    string   `yaml:"desc"`
	Detect  string   `yaml:"detect"`
	Distro  bool     `yaml:"distro,omitempty"`
	Actions []Action `yaml:"actions"`
}

func (m *Manager) Available() bool {
	if m.Detect == "" {
		return true
	}
	_, err := exec.LookPath(m.Detect)
	return err == nil
}

func (la *ListAction) FetchItems() ([]string, error) {
	if len(la.Cmd) == 0 {
		return nil, nil
	}
	out, err := exec.Command(la.Cmd[0], la.Cmd[1:]...).Output()
	if err != nil {
		return nil, err
	}
	skippedHeader := false
	var items []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if la.SkipHeader && !skippedHeader {
			skippedHeader = true
			continue
		}
		items = append(items, line)
	}
	return items, nil
}

func (la *ListAction) ItemCmd(item string) []string {
	result := make([]string, len(la.ItemAction))
	for i, arg := range la.ItemAction {
		result[i] = strings.ReplaceAll(arg, "{}", item)
	}
	return result
}

func All() []Manager {
	hideDistro := os.Getenv("MISA_ANDROID_APP") == "1"
	var managers []Manager
	for _, m := range append(builtins(), loadExternal()...) {
		if hideDistro && m.Distro {
			continue
		}
		managers = append(managers, m)
	}
	return managers
}

func loadExternal() []Manager {
	var managers []Manager
	for _, dir := range searchDirs("managers") {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var m Manager
			if yaml.Unmarshal(data, &m) == nil && m.Name != "" {
				managers = append(managers, m)
			}
		}
	}
	return managers
}

func searchDirs(subdir string) []string {
	var dirs []string
	if execPath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(execPath), subdir))
	}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(cfgDir, "misa", subdir))
	}
	return dirs
}

func onTermux() bool {
	if os.Getenv("MISA_ANDROID_APP") == "1" {
		return false
	}
	return os.Getenv("TERMUX_VERSION") != "" || strings.Contains(os.Getenv("PREFIX"), "/com.termux/")
}

func builtins() []Manager {
	managers := []Manager{pacmanDef(), flatpakDef(), gitDef(), powerDef()}
	if onTermux() {
		managers = append(managers, pkgDef())
	}
	return managers
}

func pacmanDef() Manager {
	return Manager{
		Name:   "Pacman",
		Desc:   "Package management",
		Detect: "pacman",
		Distro: true,
		Actions: []Action{
			{Title: "System upgrade", Desc: "pacman -Syu", Run: []string{"sudo", "pacman", "-Syu"}},
			{Title: "Search", Desc: "Search repos for a package", ShellPrefill: "pacman -Ss "},
			{Title: "Install", Desc: "Install a package by name", ShellPrefill: "sudo pacman -S "},
			{Title: "Search installed", Desc: "Search locally installed packages", ShellPrefill: "pacman -Qs "},
			{Title: "Remove package", Desc: "Uninstall a package and its deps", ShellPrefill: "sudo pacman -Rns "},
			{Title: "Clean cache", Desc: "pacman -Scc", Run: []string{"sudo", "pacman", "-Scc"}},
			{Title: "Clear orphans", Desc: "Remove unused dependencies", Run: []string{"sh", "-c", "sudo pacman -Rns $(pacman -Qdtq) 2>/dev/null || echo 'No orphans found'"}},
		},
	}
}

func flatpakDef() Manager {
	return Manager{
		Name:   "Flatpak",
		Desc:   "Flatpak management",
		Detect: "flatpak",
		Distro: true,
		Actions: []Action{
			{Title: "Update all", Desc: "flatpak update", Run: []string{"flatpak", "update"}},
			{Title: "Search", Desc: "Search Flathub for an app", ShellPrefill: "flatpak search "},
			{Title: "Install", Desc: "Install a flatpak by name", ShellPrefill: "flatpak install "},
			{
				Title: "List installed",
				Desc:  "Show installed flatpak apps",
				List: &ListAction{
					Cmd:        []string{"flatpak", "list", "--app", "--columns=application"},
					SkipHeader: true,
					ItemDesc:   "Installed flatpak",
					ItemAction: []string{"flatpak", "run", "{}"},
				},
			},
			{
				Title: "Uninstall",
				Desc:  "Remove a flatpak app",
				List: &ListAction{
					Cmd:        []string{"flatpak", "list", "--app", "--columns=application"},
					SkipHeader: true,
					ItemDesc:   "Click to uninstall",
					ItemAction: []string{"flatpak", "uninstall", "{}"},
				},
			},
			{Title: "Clean unused", Desc: "flatpak uninstall --unused", Run: []string{"flatpak", "uninstall", "--unused"}},
		},
	}
}

func pkgDef() Manager {
	return Manager{
		Name:   "pkg",
		Desc:   "Termux packages",
		Detect: "pkg",
		Distro: true,
		Actions: []Action{
			{Title: "Upgrade", Desc: "pkg upgrade", Run: []string{"pkg", "upgrade"}},
			{Title: "Search", Desc: "Search packages", ShellPrefill: "pkg search "},
			{Title: "Install", Desc: "Install a package", ShellPrefill: "pkg install "},
			{Title: "Remove", Desc: "Remove a package", ShellPrefill: "pkg uninstall "},
		},
	}
}

func gitDef() Manager {
	return Manager{
		Name:   "Git",
		Desc:   "Version control commands",
		Detect: "git",
		Actions: []Action{
			{Title: "Status", Desc: "Show working tree status", ShellPrefill: "git status"},
			{Title: "Diff", Desc: "Show unstaged changes", ShellPrefill: "git diff "},
			{Title: "Add file", Desc: "Stage a specific file", ShellPrefill: "git add "},
			{Title: "Add all", Desc: "Stage all changes", ShellPrefill: "git add -A"},
			{Title: "Commit", Desc: "Commit staged changes", ShellPrefill: "git commit -m \""},
			{Title: "Pull", Desc: "Pull from remote", ShellPrefill: "git pull "},
			{Title: "Push", Desc: "Push to remote", ShellPrefill: "git push "},
			{Title: "Log", Desc: "View commit history", ShellPrefill: "git log --oneline "},
			{Title: "Branch", Desc: "List or create branches", ShellPrefill: "git branch "},
			{Title: "Checkout", Desc: "Switch branches or restore files", ShellPrefill: "git checkout "},
			{Title: "Merge", Desc: "Merge a branch", ShellPrefill: "git merge "},
			{Title: "Stash", Desc: "Stash working changes", ShellPrefill: "git stash "},
			{Title: "Reset", Desc: "Reset staged changes", ShellPrefill: "git reset "},
			{Title: "Clone", Desc: "Clone a repository", ShellPrefill: "git clone "},
		},
	}
}

func powerDef() Manager {
	return Manager{
		Name: "Power",
		Desc: "Power and session controls",
		Actions: []Action{
			{Title: "Lock screen", Desc: "Lock the current session", Run: []string{"loginctl", "lock-session"}},
			{Title: "Sleep", Desc: "Suspend to RAM", Run: []string{"systemctl", "suspend"}},
			{Title: "Hibernate", Desc: "Suspend to disk", Run: []string{"systemctl", "hibernate"}},
			{Title: "Reboot", Desc: "Restart the system", Run: []string{"systemctl", "reboot"}},
			{Title: "Shutdown", Desc: "Power off the system", Run: []string{"systemctl", "poweroff"}},
			{Title: "Log out", Desc: "End the current session", Run: []string{"loginctl", "terminate-session", "self"}},
		},
	}
}

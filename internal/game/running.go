package game

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// RunningGames reports the games that look like they are running: a process
// works in or runs from the game's install folder, names a path inside it, or
// is a Steam launch for its app ID. Processes that cannot be read are ignored.
func RunningGames(games []*Game) map[uint64]bool {
	return runningGames("/proc", os.Getpid(), games)
}

func runningGames(processRoot string, self int, games []*Game) map[uint64]bool {
	running := make(map[uint64]bool)
	if len(games) == 0 {
		return running
	}
	folders := make([][]string, len(games))
	for index, entry := range games {
		folders[index] = installFolders(entry.InstallDir)
	}
	processes, err := os.ReadDir(processRoot)
	if err != nil {
		return running
	}
	for _, process := range processes {
		pid, err := strconv.Atoi(process.Name())
		if err != nil || pid == self {
			continue
		}
		directory := filepath.Join(processRoot, process.Name())
		paths := processPaths(directory)
		arguments := commandLine(directory)
		for index, entry := range games {
			if running[entry.AppID] {
				continue
			}
			running[entry.AppID] = launchedFor(arguments, entry.AppID) || anyWithin(folders[index], paths) || anyWithin(folders[index], arguments)
		}
	}
	for appID, isRunning := range running {
		if !isRunning {
			delete(running, appID)
		}
	}
	return running
}

// installFolders returns the folder as stored and, when a symbolic link leads
// to it, the real folder that /proc reports.
func installFolders(installDir string) []string {
	if installDir == "" {
		return nil
	}
	folder := filepath.Clean(installDir)
	folders := []string{folder}
	if resolved, err := filepath.EvalSymlinks(folder); err == nil && resolved != folder {
		folders = append(folders, resolved)
	}
	return folders
}

func processPaths(directory string) []string {
	var paths []string
	for _, link := range []string{"cwd", "exe"} {
		if target, err := os.Readlink(filepath.Join(directory, link)); err == nil {
			paths = append(paths, target)
		}
	}
	return paths
}

func commandLine(directory string) []string {
	data, err := os.ReadFile(filepath.Join(directory, "cmdline"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
}

// launchedFor matches Steam's launch wrapper, which carries AppId=<id>.
func launchedFor(arguments []string, appID uint64) bool {
	return slices.Contains(arguments, "AppId="+strconv.FormatUint(appID, 10))
}

func anyWithin(folders, paths []string) bool {
	for _, folder := range folders {
		for _, path := range paths {
			if path == folder || strings.HasPrefix(path, folder+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

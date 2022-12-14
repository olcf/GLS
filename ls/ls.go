package ls

import (
	"fmt"
	"gls/columnize"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gls/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// #cgo LDFLAGS: -L ../attr_check/lib -lgpfs -lstdc++ -lattr_check
// #cgo CFLAGS: -I ../attr_check
// #include "attr_check.h"
import "C"

type XAttr int

const (
	Ret0 XAttr = iota
	Ret1
	Ret2
)


// Wrapper around os.FileInfo. Including the FileInfo struct as well. Prbably need to collapse this into 1 object
type fileInfoAttr struct {
	FileInfo  os.FileInfo
	Username  string
	Groupname string
	Mtime     string
	State     XAttr
	Size      int64
	Mode      string
}

// Flags to modify the way the output is printed to the screen.
// This probably should be changed such that we have setters setting these values from main()
type Flags struct {
	Long       bool
	Human      bool
	All        bool
	SortByTime bool
	Color      map[string]bool
	NoColor    bool
	Debug      bool
}

func (l *List) SetFlags(f Flags) {
	if f.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	l.Flags = f
}

// Our main object. Contains *all* paths and the configuration for listing to the screen
type List struct {
	paths     []string
	fileInfos map[string][]fileInfoAttr
	Flags
}

// boilerplate
func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// Converts bytes to GBytes
func bytesToGB(size int64) int64 {
	return ((size / 1024) / 1024) / 1024
}

// Worker thread that stats a file and gets the right data for it.
func (l *List) fileStatWorker(input chan string, output chan fileInfoAttr, wg *sync.WaitGroup, base string) {
	defer wg.Done()
	for cur := range input {
		fia := l.doFileStat(cur, base)
		output <- fia
	}
}

// Launches batched workers for stattin
// files: Slice of files in the directory
// base: The base dir path
func (l *List) doBulkFileStat(files []string, base string) []fileInfoAttr {
	maxNProcs := config.MaxGoRoutines
	var nProcs int
	if config.AlwaysUseMaxGoRoutines {
		nProcs = maxNProcs
	} else {
		if len(files) < maxNProcs {
			// Limit the number of goroutines created. No need for 16 or 32 threads, for only 5 files
			nProcs = len(files)/2 + 1
		} else {
			// If there are more files than max number of allowable threads, then use the max thread num
			nProcs = maxNProcs
		}
	}
	if l.Flags.Debug {
		log.Debug().Msgf("Launching %s threads", strconv.Itoa(nProcs))
		log.Debug().Msgf("Maximum threads: %s", strconv.Itoa(maxNProcs))
		log.Debug().Msgf("Cores: %s", strconv.Itoa(runtime.NumCPU()))
	}
	inputChan := make(chan string, len(files))
	outputChan := make(chan fileInfoAttr, len(files))
	var wg sync.WaitGroup
	for n := 0; n < nProcs; n++ {
		log.Debug().Msgf("Launching stat worker %d", n)
		wg.Add(1)
		go l.fileStatWorker(inputChan, outputChan, &wg, base)
	}
	var FIAs []fileInfoAttr
	for _, file := range files {
		log.Debug().Msgf("Queuing work: %s", file)
		inputChan <- file
	}
	close(inputChan)
	wg.Wait()
	log.Debug().Msgf("Stat workers completed; starting gather")

	for out := range outputChan {
		FIAs = append(FIAs, out)
		if len(outputChan) <= 0 {
			close(outputChan)
		}
	}
	log.Debug().Msgf("Gather complete")
	return FIAs
}

// Performs the file stat and checks extended GPFS attributes
func (l *List) doFileStat(file string, base string) fileInfoAttr {
	fInfo, err := os.Lstat(file)
	checkErr(err)
	fia := fileInfoAttr{
		FileInfo: fInfo,
		State:    -1,
	}
	fia.populateMetadata()
	if !fia.FileInfo.IsDir() && l.Flags.Color[base] {
		fileStatus := attr_check(file)
		switch fileStatus {
		case 0:
			fia.State = Ret0
		case 1:
			fia.State = Ret1
		case 2:
			fia.State = Ret2
		}
	}

	return fia
}

// Make pretty size values
func humanizeSize(b int64) string {
	const unit = 1000
	if b < unit {
	return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// Since the stdlib function doesn't take into account extra data like directories, symlinks, stickybits, etc, lets make our own
func fileModeToString(mode os.FileMode) string {
	mStr := []rune(mode.Perm().String())
	if mode.IsDir() {
		mStr[0] = 'd'
	}
	if mode&os.ModeSetgid != 0 {
		mStr[6] = 's'
	}
	if mode&os.ModeSetuid != 0 {
		mStr[3] = 's'
	}
	if mode&os.ModeSticky != 0 {
		mStr[9] = 't'
	}
	if mode&os.ModeSymlink != 0 {
		mStr[0] = 'l'
	}
	if mode&os.ModeDevice != 0 {
		if mode&os.ModeCharDevice == 0 {
			mStr[0] = 'b' // block devicea
		} else {
			mStr[0] = 'c' // character device
		}
	}
	if mode&os.ModeNamedPipe != 0 {
		mStr[0] = 'p'
	}
	if mode&os.ModeSocket != 0 {
		mStr[0] = 's'
	}
	return string(mStr)

}

// Is this file a symlink?
func isSymlink(f os.FileInfo) bool {
	if f.Mode()&os.ModeSymlink != 0 {
		return true
	} else {
		return false
	}
}

// config.Return a pointer to a new List object
func New(inputPaths []string) *List {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	return &List{
		paths: inputPaths,
	}
}

// Look up the username, and group name so we're not just looking at integers here; Make the mTime look pretty, as well as the mode string
func (f *fileInfoAttr) populateMetadata() {
	uid := f.FileInfo.Sys().(*syscall.Stat_t).Uid
	gid := f.FileInfo.Sys().(*syscall.Stat_t).Gid
	username, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	checkErr(err)
	group, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	checkErr(err)

	f.Mode = fileModeToString(f.FileInfo.Mode())
	f.Username = username.Username
	f.Groupname = group.Name
	f.Size = f.FileInfo.Size()
	f.Mtime = f.FileInfo.ModTime().Format("Jan 02 15:04 2006")
	log.Debug().Msgf("Gathered metadata for %s: username: %s, groupname: %s, mode: %s, size: %d, mtime: %s", f.FileInfo.Name(), f.Username, f.Groupname, f.Mode, f.Size, f.Mtime)
}

// Is this a hidden file? (prefixed with a .)
func (l *List) isHiddenFile(f fileInfoAttr) bool {
	if []rune(f.FileInfo.Name())[0] == '.' {
		return true
	}
	return false
}

// This gets the tab deleniated string to print to the screen with all the info
func (l *List) getLongListing(fileInfo fileInfoAttr) string {
	curLine := fileInfo.Mode + "\t"
	// Find UID and resolve name
	curLine += fileInfo.Username + "\t"
	// Find GID and resolve name
	curLine += fileInfo.Groupname + "\t"
	// Get file size and make human readable if -h
	if l.Flags.Human {
		curLine += humanizeSize(fileInfo.Size) + "\t"
		//Make file.FileInfo.Size() human readable
	} else {
		curLine += strconv.FormatInt(fileInfo.Size, 10) + "\t"
	}
	// Find mtime and make human readable
	curLine += fileInfo.Mtime + "\t "
	return curLine
}

// Launch batches of workers for all directories passed into List
func (l *List) StatAll() {
	// Stat everything in paths and populate l.fileInfos
	l.fileInfos = make(map[string][]fileInfoAttr)

	for _, path := range l.paths {
		baseDirSlice := strings.Split(path, "/")
		baseDir := strings.Join(baseDirSlice[:len(baseDirSlice)-1], "/")
		fia := l.doFileStat(path, path)
		if !fia.FileInfo.IsDir() {
			l.fileInfos[baseDir] = append(l.fileInfos[baseDir], fia)
		} else {
			// use regex with filepath.Glob to get just visable files or all files if -a
			rex := `/[^\.]*`
			if l.Flags.All {
				rex = `/*`
			}
			files, err := filepath.Glob(path + rex)
			checkErr(err)
			var dirEntries []fileInfoAttr
			if l.Flags.All {
				curDir := l.doFileStat(path+"/.", path)
				parentDir := l.doFileStat(path+"/..", path)
				dots := []fileInfoAttr{curDir, parentDir}
				dirEntries = append(dots, dirEntries...)
			}
			if len(files) > 0 {
				l.fileInfos[path] = make([]fileInfoAttr, len(files))
				dirEntries = append(dirEntries, l.doBulkFileStat(files, path)...)
			}
			l.fileInfos[path] = dirEntries

		}
	}
}

// Get modified filename to show where the symlink points
func (l *List) getSymlinkString(f os.FileInfo, base string) string {
	target, err := filepath.EvalSymlinks(base + "/" + f.Name())
	if base != "/" {
		target = strings.Replace(target, base, ".", 1)
	}
	checkErr(err)
	var curLine string
	if l.Flags.NoColor {
		curLine = columnize.Colorize(columnize.Reset, f.Name())
	} else {
		curLine = columnize.Colorize(columnize.LightBlue, f.Name())
	}
	if l.Flags.Long {
		curLine += " -> " + target
	}
	return curLine
}

// Make pretty colors based upon attributes like symlink, storage pool, etc
func (l *List) getProcessedFilename(file fileInfoAttr, base string) (string, columnize.Color) {
	if file.FileInfo.IsDir() {
		var color columnize.Color = columnize.Blue
		if l.Flags.NoColor {
			color = columnize.Reset
		}
		return file.FileInfo.Name(), color
	} else if isSymlink(file.FileInfo) {
		var color columnize.Color = columnize.LightBlue
		if l.Flags.NoColor {
			color = columnize.Reset
		}
		return l.getSymlinkString(file.FileInfo, base), color
	} else if bytesToGB(file.FileInfo.Size()) > config.MaxFileSizeGB && config.DisableSizeChecking != true {
		if l.Flags.NoColor {
			return fmt.Sprintf("%s %s", "(TOO LARGE TO MIGRATE)", file.FileInfo.Name()), columnize.Reset
		} else {
			return file.FileInfo.Name(), columnize.BlinkingRedBackground
		}
	}
	switch file.State {
	case 0:
		if l.Flags.NoColor {
			return fmt.Sprintf("(%s) %s", config.Ret0Str, file.FileInfo.Name()), columnize.Reset
		} else {
			return file.FileInfo.Name(), columnize.Green
		}
	case 1:
		if l.Flags.NoColor {
			return fmt.Sprintf("(%s) %s", config.Ret1Str, file.FileInfo.Name()), columnize.Reset
		} else {
			return file.FileInfo.Name(), columnize.Yellow
		}
	case 2:
		if l.Flags.NoColor {
			return fmt.Sprintf("(%s) %s", config.Ret2Str, file.FileInfo.Name()), columnize.Reset
		} else {
			return file.FileInfo.Name(), columnize.Red
		}
	default:
		return file.FileInfo.Name(), columnize.Reset
	}
}

// Sort the output based upon values in List.Flags
func (l *List) Sort() {
	log.Debug().Msgf("Starting sort")
	if l.Flags.SortByTime {
		for _, fileinfos := range l.fileInfos {
			sort.Slice(fileinfos, func(i, j int) bool {
				iTime, err := time.Parse("Jan 02 15:04 2006", fileinfos[i].Mtime)
				checkErr(err)
				jTime, err := time.Parse("Jan 02 15:04 2006", fileinfos[j].Mtime)
				checkErr(err)
				return iTime.Before(jTime)
			})
		}
	} else {
		//Sort alphabetically by default
		for _, fileinfos := range l.fileInfos {
			sort.Slice(fileinfos, func(i, j int) bool {
				return fileinfos[i].FileInfo.Name() < fileinfos[j].FileInfo.Name()
			})
		}
	}
	log.Debug().Msgf("Sort finished")
}

// Print the whole list to the screen. This includes all paths in List
func (l *List) Print() {
	l.Sort()
	// Loop through l.fileInfos and pretty prent the information
	log.Debug().Msgf("Printing to screen")
	var count int
	for base, directory := range l.fileInfos {
		count++
		columnize.NewAlignRight()
		if len(l.fileInfos) > 1 {
			if count > 1 {
				fmt.Println()
			}
			fmt.Println(base + ":")
		}
		for _, file := range directory {
			var curLine []string
			if l.Flags.Long {
				if !l.isHiddenFile(file) || l.Flags.All {
					curLine = append(curLine, l.getLongListing(file))
					name, color := l.getProcessedFilename(file, base)
					curLine = append(curLine, name)
					columnize.PrintLine(
						columnize.ColumnizeRow(
							color,
							len(curLine)-1,
							curLine))
				}
			} else {
				if !l.isHiddenFile(file) || l.Flags.All {
					name, color := l.getProcessedFilename(file, base)
					// not -l so print in columns
					curLine = append(curLine, name)
					columnize.PrintLine(
						columnize.ColumnizeRow(
							color,
							len(curLine)-1,
							curLine))
				}
			}
		}
		columnize.Flush()
	}
}

// Wrapper function around C function that calls gpfs_fgetattrs(). The user of this function doesn't need to deal with the C.* functions this way
func attr_check(path string) int {
	cs := C.CString(path)
	return int(C.attr_check(cs))
}

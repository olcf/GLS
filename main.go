package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"strings"

	"gls/columnize"
	"gls/config"
	"gls/ls"

	// We use kingpin here to allow combining of short flags (e.g. -lha) and better handle positional arguments
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// Boilerplate check Error function. We panic here because we're recovering in main()
func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// Returns output and return code
func runCommand(cmd []string) ([]string, int) {
	command := exec.Command(cmd[0], cmd[1:]...)
	output, err := command.CombinedOutput()

	var exitCode int

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	return strings.Split(string(output), "\n"), exitCode
}

// Make a pretty output for our hints blurb
func displayHints() {
	columnize.New()
	columnize.PrintLine(
		columnize.ColumnizeRow(
			columnize.Blue,
			0,
			[]string{"Blue:", "Indicates a directory"}))
	columnize.PrintLine(
		columnize.ColumnizeRow(
			columnize.Green,
			0,
			[]string{"Green:", config.Ret0Hint}))
	columnize.PrintLine(
		columnize.ColumnizeRow(
			columnize.Yellow,
			0,
			[]string{"Yellow:", config.Ret1Hint}))
	columnize.PrintLine(
		columnize.ColumnizeRow(
			columnize.Red,
			0,
			[]string{"Red:", config.Ret2Hint}))
	columnize.PrintLine(
		columnize.ColumnizeRow(
			columnize.LightBlue,
			0,
			[]string{"Light Blue:", "Indicates a symbolic link"}))
	columnize.PrintLine(
		columnize.ColumnizeRow(
			columnize.BlinkingRedBackground,
			0,
			[]string{"White on Red:", "Indicates a file resident on disk that will never be able to migrate to tape because it is too large"}))
	columnize.Flush()
}

// This function is used to check if we should attempt colorizing the results.
// I.E. files that aren't on GPFS aren't technically 'resident' or 'migrated' they just are
func checkForColorize(paths []string) map[string]bool {
	matches := config.GpfsRoots

	ret := make(map[string]bool, len(paths))
	for _, path := range paths {
		ret[path] = false
		for _, match := range matches {
			if path == match || strings.Contains(path, match) {
				ret[path] = true
			}
		}
	}
	return ret
}

func main() {
	// Preserve error messages when panicing. Output without stack trace
	if config.SuppressStackTrace {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Aborting: ", r)
				os.Exit(1)
			}
		}()
	}

	long := kingpin.Flag("long", "Long listing").Short('l').Bool()
	human := kingpin.Flag("human", "Human readable listing").Short('h').Bool()
	all := kingpin.Flag("all", "Show all files including hidden files").Short('a').Bool()
	disable := kingpin.Flag("disable-wrapper", "Disable wrapper and fall back to standard ls").Bool()
	hints := kingpin.Flag("hints", "Display hints about color code meanings").Short('H').Bool()
	time := kingpin.Flag("time", "Sort output by time last modified").Short('t').Bool()
	noColor := kingpin.Flag("no-color", "Disable coloring and use text for storage pool location").Short('n').Bool()
	paths := kingpin.Arg("paths", "Paths to list").Default(".").Strings()
	var cpuprofPath *string
	var debug *bool

	if config.HideDebugFlags {
		cpuprofPath = kingpin.Flag("cpuprof", "Enable output of CPU Profiling data").Hidden().String()
		debug = kingpin.Flag("debug", "Display debug information").Short('v').Hidden().Bool()
	} else {
		cpuprofPath = kingpin.Flag("cpuprof", "Enable output of CPU Profiling data").String()
		debug = kingpin.Flag("debug", "Display debug information").Short('v').Bool()
	}

	kingpin.Parse()

	if len(*cpuprofPath) != 0 {
		cpuprofile := *cpuprofPath
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	if *hints {
		displayHints()
		os.Exit(0)
	}

	//TODO: Fix this. Have a remove function that returns []string without --disable-wrapper
	if *disable == true {
		var cmd []string
		cmd = append(cmd, "ls")
		for _, arg := range os.Args[1:] {
			if arg != "--disable-wrapper" {
				cmd = append(cmd, arg)
			}
		}
		out, err := runCommand(cmd)
		for _, line := range out {
			fmt.Println(line)
		}
		os.Exit(err)
	}

	var cleanPaths []string
	//Get the absolute paths, and clean them (in case of symlinks)
	for _, path := range *paths {
		p, err := filepath.Abs(path)
		checkErr(err)
		cleanPaths = append(cleanPaths, filepath.Clean(p))
	}

	listFlags := ls.Flags{
		Long:       *long,
		Human:      *human,
		All:        *all,
		Color:      checkForColorize(cleanPaths),
		SortByTime: *time,
		NoColor:    *noColor,
		Debug:      *debug,
	}

	list := ls.New(cleanPaths)
	list.SetFlags(listFlags)
	list.StatAll()
	list.Print()
}

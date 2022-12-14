package config

import (
	"math"
	"runtime"
)

type XAttr int

var (
	// Root paths to the mounted GPFS filesystems
	GpfsRoots = []string{"/gpfs/themis", "/nl/themis"}
	// Max file size for an individual file that can be migrated to tape
	MaxFileSizeGB int64 = 19450
	// Disable stack trace upon failure
	SuppressStackTrace = true
	// Maximum number of GoRoutines to launch when statting files
	MaxGoRoutines = int(math.Ceil(float64(runtime.NumCPU() / 2)))
	// Always use maximum number of go routines; never try and minimize based upon directory size
	AlwaysUseMaxGoRoutines = false
	//Hide the debug flag options from --help
	HideDebugFlags = true
	// Disables checking the size of the file against MaxFileSizeGB
	DisableSizeChecking = false

	// Customize the following based upon the return codes for attr_check.cpp
	// If a non default attr_check is used, these should be changed, else DO NOT TOUCH
	// Ret*Str is the string representation used when --no-color flag is present
	Ret0Str string = "Resident"
	Ret1Str string = "Premigrated"
	Ret2Str string = "Migrated"
	//Ret*Hint is the string description for the return code/color when using -H
	Ret0Hint string = "Indicates a file that is resident on disk"
	Ret1Hint string = "Indicates a file that has been premigrated (e.g. resident on both tape and disk)"
	Ret2Hint string = "Indicates a file that has been migrated to tape"
)



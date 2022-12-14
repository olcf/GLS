# GLS: GPFS-aware LS
`gls` provides an `ls`-like mechanism to provide users insight into what storage pool their files live on in multi-tiered storage systems. This was specfically designed with a GPFS/Spectrum Scale and Spectrum Archive system in mind, but can be extended to any other multi-tiered filesystem by changing the functionality of `attr_check`

Example output:
![example output](https://github.com/olcf/gls/blob/main/images/output.png?raw=true)

### Build Prerequisites

* Golang 1.19
* libgpfs.so (Provided by gpfs-base)
* g++
* NFPM (see [the NFPM website](https://nfpm.goreleaser.com/install/))

### Building

Once all the prerequisites are installed, change any of the predefined configuration values in `config/config.go` to match your environment. Next, run `$ make` to build the binary. This will output the `gls` binary to the current working directory. To build an RPM package, run `$ make rpm`. To install, run `# make install`

### Usage:
Usage of `gls` is similar to standard `ls`. One exception to this is `--disable-wrapper` which disables the `attr_check` module and falls back to `ls`; anything on the commandline after this flag gets passed directly to `ls`. This can be useful for enviornments using `gls` as a drop in replacement for `ls` or environments that alias `ls` to `/usr/local/bin/gls`. Another exception is `-n` or `--no-color`. This disables text coloring and uses text annotations to denote what the state of the file is.

Finally, `--hints` shows an explanation of what the color scheme maps to (Resident on the primary pool, premigrated/resident on both pools, or migrated/only resident on the external pool)

![hints_example](https://github.com/olcf/gls/blob/main/images/hints.png?raw=true)

```bash
[user@hostname 12:37:10][~]# ./gls --help
usage: gls [<flags>] [<paths>...]

Flags:
      --help             Show context-sensitive help (also try --help-long and --help-man).
  -l, --long             Long listing
  -h, --human            Human readable listing
  -a, --all              Show all files including hidden files
      --disable-wrapper  Disable wrapper and fall back to standard ls
  -H, --hints            Display hints about color code meanings
  -t, --time             Sort output by time last modified
  -n, --no-color         Disable coloring and use text for storage pool location

Args:
  [<paths>]  Paths to list

```



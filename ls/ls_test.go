package ls

import (
	"testing"
	"reflect"
	"sync"
	"os"
	"path/filepath"
	"gls/columnize"
	"bytes"
	//"fmt"
	"io"
	"github.com/spf13/afero"
)

var afs afero.Fs

func InitFS() {
	afs = afero.NewMemMapFs()
	err := afs.MkdirAll("/nl/themis", 0777)
	checkErr(err)
	_, err = afs.Create("/nl/themis/redhat-release")
	checkErr(err)
	_, err = afs.Create("/nl/themis/test")
	checkErr(err)
}

func TestNewLS(t *testing.T) {
	InitFS()
	testPath := "/nl/themis/test"
	l := New([]string{testPath})
	want := &List{paths: []string{testPath}}
	if !reflect.DeepEqual(*l, *want) {
		t.Fatalf("ls.New(%s) = %v; want %v", testPath, l, want)
	}
}

func TestSetFlags(t *testing.T) {
	testPath := "/nl/themis/test"
	testFlags := Flags{
		Long: true,
		Debug: true,
	}
	want := &List{paths: []string{testPath}, Flags: testFlags}
	l := New([]string{testPath})

	l.SetFlags(testFlags)

	if !reflect.DeepEqual(*l, *want) {
		t.Fatalf("ls.SetFlags(%v) = %v; want %v", testFlags, l, want)
	}
}

func TestBytesToGB(t *testing.T) {
	var testInB int64 = 1074000000
	var want int64 = 1
	have := bytesToGB(testInB)
	if have != want {
		t.Fatalf("ls.bytesToGB(%d)= %d; want %d", testInB, have, want)
	}
}

func TestFileStatWorker(t *testing.T) {
	testList := New([]string{"/nl/themis"})
	inputChan := make(chan string, 1)
	outputChan := make(chan fileInfoAttr, 1)
	base := "/nl/themis"
	inputPath := "/nl/themis/redhat-release"
	var wg sync.WaitGroup
	wg.Add(1)

	inputChan <- inputPath

	go testList.fileStatWorker(inputChan, outputChan, &wg, base)
	var have fileInfoAttr
	for out := range outputChan {
		have = out
		if len(outputChan) <= 0 {
			close(outputChan)
		}
	}
	// If the file actually exists then succeed
	var dontWant os.FileInfo
	if have.FileInfo == dontWant {
		t.Fatalf("ls.fileStatWorker(%s) = %v; want len > 0", inputPath, have.FileInfo)
	}
}

func TestDoBulkFileStat(t *testing.T) {
	testList := New([]string{"/"})
	base := "/"
	fileList := []string{"/usr", "/bin"}
	have := testList.doBulkFileStat(fileList, base)
	if len(have) != 2 {
		t.Fatalf("ls.doBulkFileStat(%v) = %v; want len == 0", fileList, have)
	}
	var dontWant os.FileInfo
	for idx, f := range have {
		if f.FileInfo == dontWant {
			t.Fatalf("ls.doBulkFileStat(%v)[%d] = %v; want not nil", fileList, idx, f.FileInfo)
		}
	}

}

func TestDoFileStat(t *testing.T) {
	testList := New([]string{"/nl/themis"})
	have := testList.doFileStat("/nl/themis/redhat-release", "/nl/themis")
	fileInfo, _ := afs.Stat("/nl/themis/redhat-release")

	// {0xc000188270 root root Mar 31 04:28 2021 -1 45 -rw-r--r--}
	want := fileInfoAttr{
		FileInfo: fileInfo,
		Username: "root",
		Groupname: "root",
		Mtime: "Mar 31 04:28 2021",
		State: -1,
		Size: 45,
		Mode: "-rw-r--r--",
	}

	if !reflect.DeepEqual(have, want) {
		t.Fatalf("ls.doFileStat(\"/nl/themis/redhat-release\", \"/nl/themis\") = %v; want %v", have, want)
	}
}

func TestHumanizeSize(t *testing.T) {
	var testVal int64 = 123456
	have := humanizeSize(testVal)
	want := "123.5 kB"
	if have != want {
		t.Fatalf("ls.humanizeSize(%d) = %s; want %s", testVal, have, want)
	}
}

func TestFileModeToString(t *testing.T) {
	fileInfo, _ := afs.Stat("/nl/themis/redhat-release")
	have := fileModeToString(fileInfo.Mode())
	want := "-rw-r--r--"
	if have != want {
		t.Fatalf("ls.fileModeToString(%d) = %s; want %s", fileInfo.Mode(), have, want)
	}
}

func TestIsSymlink(t *testing.T) {
	fi, _ := afs.Stat("/bin") // /bin on rhel systems appear to be symlinks
	have := isSymlink(fi)
	want := true
	if have != want {
		t.Fatalf("ls.isSymlink(%v) = %t; want %t", fi, have, want)
	}
	fi, _ = afs.Stat("/nl/themis/redhat-release")
	have = isSymlink(fi)
	want = false
	if have != want {
		t.Fatalf("ls.isSymlink(%v) = %t; want %t", fi, have, want)
	}
}

func TestPopulateMetadata(t *testing.T) {
	testFile, _ := afs.Stat("/nl/themis/redhat-release")
	have := fileInfoAttr{
		FileInfo: testFile,
	}
	origHave := have
	//root root Mar 31 04:28 2021 0 45 -rw-r--r--}
	want := fileInfoAttr{
		FileInfo: testFile,
		Username: "root",
		Groupname: "root",
		Mtime: "Mar 31 04:28 2021",
		State: 0,
		Size: 45,
		Mode: "-rw-r--r--",
	}

	have.populateMetadata()
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("ls.populateMetadata(fileInfoAttr: %v) = %v; want %v", origHave, have, want)
	}
}

func TestIsHiddenFile(t *testing.T) {
	fi, _ := afs.Stat("/nl/themis/redhat-release")
	l := List{}
	test := fileInfoAttr{
		FileInfo: fi,
	}
	have := l.isHiddenFile(test)
	want := false
	if have != want {
		t.Fatalf("ls.isHiddenFile(%v) = %t; want %t", test, have, want)
	}

	f, _ := os.OpenFile("/tmp/.hidden", os.O_RDONLY|os.O_CREATE, 0666)
	f.Close()

	fi, _ = afs.Stat("/tmp/.hidden")
	test = fileInfoAttr{
		FileInfo: fi,
	}
	have = l.isHiddenFile(test)
	want = true
	if have != want {
		t.Fatalf("ls.isHiddenFile(%v) = %t; want %t", test, have, want)
	}
	_ = os.Remove("/tmp/.hidden")
}

func TestStatAll(t *testing.T) {
	path, _ := filepath.Abs(".")
	l := New([]string{path})
	l.StatAll()
	if len(l.fileInfos[path]) != 2 {
		t.Fatalf("ls.StatAll(%s) = %v; want len == 2", path, l.fileInfos[path])
	}
}

func TestGetSymlinkString(t *testing.T) {
	fi, _ := afs.Stat("/bin")
	l := List{}
	have := []byte(string([]rune(l.getSymlinkString(fi, "/"))))
	want := []byte{27, 91, 48, 48, 48, 48, 51, 54, 109, 98, 105, 110, 27, 91, 48, 48, 48, 48, 48, 48, 109}
	if !bytes.Equal(have, want) {
		t.Fatalf("ls.getSymlinkString(%s) = %v; want %v", "/bin", have, want)
	}

	flags := Flags{
		Long: true,
	}
	l.SetFlags(flags)
	have = []byte(string([]rune(l.getSymlinkString(fi, "/"))))
	want = []byte{27, 91, 48, 48, 48, 48, 51, 54, 109, 98, 105, 110, 27, 91, 48, 48, 48, 48, 48, 48, 109, 32, 45, 62, 32, 47, 117, 115, 114, 47, 98, 105, 110}
	if !bytes.Equal(have, want) {
		t.Fatalf("ls(flags=long).getSymlinkString(%s) = %v; want %v", "/bin", have, want)
	}
}

// TODO: add test for long filename
func TestGetProcessedFileName(t *testing.T) {
	base, err := filepath.Abs(".")
	checkErr(err)
	l := New([]string{base})
	l.StatAll()
	str, color := l.getProcessedFilename(l.fileInfos[base][0], base)
	wantStr := "ls.go"
	wantColor := columnize.Reset
	if str != wantStr {
		t.Fatalf("ls(flags=long).getProcessedFilename(%s) = %s; want %s", base, str, wantStr)
	}
	if color != wantColor {
		t.Fatalf("ls(flags=long).getProcessedFilename(%s) = %s; want %s", base, color, wantColor)
	}
}


func TestSort(t *testing.T) {
	path, _ := filepath.Abs(".")
	l := New([]string{path})
	l.StatAll()
	l.Sort()
	if l.fileInfos[path][0].FileInfo.Name() != "ls.go" && l.fileInfos[path][len(l.fileInfos)-1].FileInfo.Name() != "ls_test.go" {
		t.Fatalf("ls.Sort(%s) = %v; want [0] == 'ls.go' && [1] == 'ls_test.go'", path, l.fileInfos[path])
	}
}

func captureOutput(f func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	stdout := os.Stdout
	stderr := os.Stderr
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
	}()
	os.Stdout = writer
	os.Stderr = writer
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var buf bytes.Buffer
		wg.Done()
		io.Copy(&buf, reader)
		out <- buf.String()
	}()
	wg.Wait()
	f()
	writer.Close()
	return <-out
}

func TestPrint(t *testing.T) {
	path, err := filepath.Abs(".")
	checkErr(err)
	l := New([]string{path})
	//TODO: Figure out how to set -l without being bitten by the mtime changing on files resulting in different output
//	flags := Flags{
//		Long: true,
//	}
//
//	l.SetFlags(flags)
	l.StatAll()
	l.Sort()

	output := captureOutput(
		func() {
			l.Print()
	})
	have := []byte(string([]rune(output)))
	want := []byte{27, 91, 48, 48, 48, 48, 48, 48, 109, 108, 115, 46, 103, 111, 27, 91, 48, 48, 48, 48, 48, 48, 109, 10, 27, 91, 48, 48, 48, 48, 48, 48, 109, 108, 115, 95, 116, 101, 115, 116, 46, 103, 111, 27, 91, 48, 48, 48, 48, 48, 48, 109, 10}
	if !bytes.Equal(have, want) {
		t.Fatalf("ls.Print(%s) = %v; want %v", path, have, want)
	}
}

func TestAttrCheck(t *testing.T) {
	//This won't be testable on a non-gpfs system
	//Maybe here i just need to embed some files that have the extended gpfs attribute?
	//That might not work. This assumes that go:embed, git and nfs preserve gpfs extended attrs
}

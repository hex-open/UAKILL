package main

import (
	"bufio"
	"golang.org/x/sys/windows"
	"log"
	"os"
	"strings"
	"unsafe"
)

type ProcessStruct struct {
	processName string // 进程名称
	processID   uint32 // 进程id
}

type ProcessStructSlice []ProcessStruct

func (a ProcessStructSlice) Len() int { // 重写 Len() 方法
	return len(a)
}
func (a ProcessStructSlice) Swap(i, j int) { // 重写 Swap() 方法
	a[i], a[j] = a[j], a[i]
}
func (a ProcessStructSlice) Less(i, j int) bool { // 重写 Less() 方法， 从大到小排序
	if strings.Compare(a[j].processName, a[i].processName) < 0 {
		return true
	} else {
		return false
	}
}

var processMap = map[string]uint32{}
var processList = ProcessStructSlice{}

func fetchAllProcess() {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		log.Fatal("Failed To Get the Process SnapShot")
	}
	//goland:noinspection ALL
	defer windows.CloseHandle(snapshot)
	var procEntry windows.ProcessEntry32
	procEntry.Size = uint32(unsafe.Sizeof(procEntry))
	if err = windows.Process32First(snapshot, &procEntry); err != nil {
		log.Fatal("Failed To Get the Process SnapShot")
	}
	for {
		err = windows.Process32Next(snapshot, &procEntry)
		if err != nil {
			break
		}

		processName := windows.UTF16ToString(procEntry.ExeFile[:])
		processPid := procEntry.ProcessID

		processMap[processName] = processPid
		processList = append(processList, ProcessStruct{
			processName: processName,
			processID:   processPid,
		})
	}
}

func getProcessID(processName string) int {
	return int(processMap[processName])
}

func SuspendThread(pid int) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		log.Fatal("Failed To Get the Process SnapShot")
	}
	//goland:noinspection ALL
	defer windows.CloseHandle(snapshot)
	var threadEntry windows.ThreadEntry32
	threadEntry.Size = uint32(unsafe.Sizeof(threadEntry))
	if err = windows.Thread32First(snapshot, &threadEntry); err != nil {
		log.Fatal("Failed To Get the Process SnapShot")
	}
	for {
		err = windows.Thread32Next(snapshot, &threadEntry)
		if err != nil {
			break
		}
		if threadEntry.OwnerProcessID == uint32(pid) {
			thread, _ := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, threadEntry.ThreadID)
			_, err := windows.SuspendThread(thread)
			if err != nil {
				//log.Println("SuspendThread Error", err)
			}
			err = windows.CloseHandle(thread)
			if err != nil {
				//log.Println("CloseHandle Error", err)
			}
		}
	}
}

func TerminateProcess(pid int) {
	//log.Println("kill pid", pid)
	proc, err := windows.OpenProcess(windows.PROCESS_TERMINATE, true, uint32(pid))
	if err != nil {
		//log.Println("OpenProcess Error", err)
	}
	err = windows.TerminateProcess(proc, 0)
	if err != nil {
		//log.Println("TerminateProcess Error", err)
	}
	err = windows.CloseHandle(proc)
	if err != nil {
		//log.Println("CloseHandle Error", err)
	}
}

func kill(procList []string) {
	fetchAllProcess()
	for _, proc := range procList {
		pid := getProcessID(proc)
		if pid != 0 {
			log.Println("SuspendThread", proc, pid)
			SuspendThread(pid)
		}
	}
	for _, proc := range procList {
		pid := getProcessID(proc)
		if pid != 0 {
			log.Println("TerminateProcess", proc, pid)
			TerminateProcess(pid)
		}
	}
}

func startKill() {
	procAttr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}
	process, _ := os.StartProcess(os.Args[0], []string{os.Args[0], "kill"}, procAttr)
	_, _ = process.Wait()
}

func check(procList []string) {
	fetchAllProcess()
	count := 0
	for _, proc := range procList {
		pid := getProcessID(proc)
		if pid != 0 {
			log.Println("Kill fail", proc, pid)
			count += 1
		}
	}
	if count == 0 {
		log.Println("UA is killed")
		os.Exit(0)
	}
	os.Exit(1)
}

func runAsAdmin() {
	verb := windows.StringToUTF16("runas")
	file := windows.StringToUTF16(os.Args[0])
	hwnd := windows.CurrentProcess()

	_ = windows.ShellExecute(hwnd, &verb[0], &file[0], nil, nil, windows.SW_SHOWNORMAL)
}

func main() {
	token := windows.GetCurrentProcessToken()
	if !token.IsElevated() {
		log.Println("Please run as admin")
		runAsAdmin()
		os.Exit(0)
	}
	procList := []string{
		"UniAccessAgentDaemon.exe",
		"HutiehuaApp.exe",
		"Tinaiat.exe",
		"LvaNac.exe",
		"UniSensitive.exe",
		"UniAccessAgent.exe",
		"UniAccessAgentTray.exe",
	}
	//log.Println(os.Args)
	if len(os.Args) == 2 && os.Args[1] == "kill" {
		kill(procList)
		os.Exit(0)
	}
	if len(os.Args) == 2 && os.Args[1] == "check" {
		check(procList)
		os.Exit(0)
	}
	reader := bufio.NewReaderSize(os.Stdin, 1)
	log.Println("Press Enter Start")
	_, _ = reader.ReadByte()
	log.Println("Start kill UA")
	for {
		startKill()
		procAttr := &os.ProcAttr{
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		}
		process, _ := os.StartProcess(os.Args[0], []string{os.Args[0], "check"}, procAttr)
		state, _ := process.Wait()
		if state.ExitCode() == 0 {
			log.Println("Press Enter Exit")
			_, _ = reader.ReadByte()
			break
		}
		log.Println("Retry kill UA")
		windows.SleepEx(1000, false)
	}
}

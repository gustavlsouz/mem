package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mackerelio/go-osstat/memory"
)

/* Context stores all arguments */
type Context struct {
	MaxUsage        uint64
	CriticalUsage   uint64
	Wait            time.Duration
	Ignores         []string
	Targets         []string
	CriticalTargets []string
	UserInfo        *user.User
	Processes       []map[string]string
	OsArgs          []string
	Memory          *memory.Stats
	KillTask        string
}

/* Choose the target*/
func (context *Context) GetTargets() []string {
	if context.KillTask == "Targets" {
		return context.Targets
	}
	if context.KillTask == "CriticalTargets" {
		return context.CriticalTargets
	}
	return nil
}

func toKb(bytes uint64) uint64 {
	return bytes / 1024
}

func toMb(bytes uint64) uint64 {
	return toKb(bytes) / 1024
}

func toGb(bytes uint64) uint64 {
	return toMb(bytes) / 1024
}

func kill(process map[string]string) {
	pid := process["pid"]
	fmt.Printf("\n\n")
	log.Println("kill", pid)
	log.Println("path: ", process["path"])
	cmd := exec.Command("kill", pid)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Println(fmt.Sprint(err) + ": " + stderr.String())
		return
	}
	output := out.String()
	if output == "" {
		log.Println(pid, "killed")
		return
	}
	log.Println("Result: " + output)
}

var clear map[string]func() //create a map for storing clear funcs

func init() {
	clear = make(map[string]func()) //Initialize it
	clear["linux"] = func() {
		cmd := exec.Command("clear") //Linux example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
	clear["windows"] = func() {
		cmd := exec.Command("cmd", "/c", "cls") //Windows example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func callClear() {
	value, ok := clear[runtime.GOOS] //runtime.GOOS -> linux, windows, darwin etc.
	if ok {                          //if we defined a clear func for that platform:
		value() //we execute it
	} else { //unsupported platform
		panic("Your platform is unsupported! I can't clear terminal screen :(")
	}
}

func psAux() []map[string]string {
	log.Println("ps -ux")
	cmd := exec.Command("ps", "-ux")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Println("something wrong to execute 'ps'")
		log.Println(fmt.Sprint(err) + " : " + stderr.String())

	}
	raw := out.String()
	lines := strings.Split(raw, "\n")
	// fields := lines[0]
	processesRaw := lines[1:]
	processes := make([]map[string]string, 0)
	// fmt.Println(fields)
	pattern := regexp.MustCompile("\\s+")
	commandsPatter := regexp.MustCompile("\\-\\-\\S+")
	// otherCommandsPattern := regexp.MustCompile(" \\-\\S")
	pathPattern := regexp.MustCompile("\\/[A-z0-9-/\\.]+")
	for _, line := range processesRaw {
		columns := pattern.Split(line, -1)
		if len(columns) >= 2 {
			item := make(map[string]string)
			item["user"] = columns[0]
			item["pid"] = columns[1]
			// log.Println("\n")
			// log.Println(line)
			replacement := strings.Trim(commandsPatter.ReplaceAllString(line, ""), "")
			// log.Println(replacement)
			path := pathPattern.FindString(replacement)
			// log.Println("path: ", path)
			item["path"] = path
			item["line"] = line
			processes = append(processes, item)
		}
	}
	return processes
}

func containsPattern(arr []string, str string) bool {
	for _, text := range arr {
		pattern := regexp.MustCompile("\\b" + text + "\\b")
		if pattern.MatchString(str) {
			return true
		}
	}
	return false
}

func contains(arr []string, str string) bool {
	for _, text := range arr {
		if strings.Contains(str, text) {
			return true
		}
	}
	return false
}

func getProcessessForTargets(context *Context) []map[string]string {
	targetProcesses := make([]map[string]string, 0)
	targets := context.GetTargets()
	for _, process := range context.Processes {
		proccessNameInPath := containsPattern(targets, process["path"])
		inIgnore := containsPattern(context.Ignores, process["path"])
		if inIgnore {
			log.Println(process["path"], "in ignore")
		}
		if proccessNameInPath && !inIgnore && process["user"] == context.UserInfo.Username {
			// log.Println(process["path"])
			targetProcesses = append(targetProcesses, process)
		}
	}
	return targetProcesses
}

func killTargets(context *Context) bool {
	targetProcesses := getProcessessForTargets(context)
	for _, process := range targetProcesses {
		kill(process)
	}
	return len(targetProcesses) > 0
}

func logInfo(maxUsage, criticalUsage uint64) {
	log.Println("version 0.0.4")
	log.Println("max usage:", maxUsage, "MB")
	log.Println("critical:", criticalUsage, "MB")
}

func checkKilled(killed bool) {
	if killed {
		log.Println("kills executed")
	} else {
		log.Println("nothing to kill")
	}
}

func main() {
	log.Println("initializing...")
	userInfo, errUserInfo := user.Current()

	if errUserInfo != nil {
		panic(errUserInfo)
	}

	context := &Context{
		Wait:     time.Second * 2,
		MaxUsage: 6800,
		// MaxUsage: 1000,
		CriticalUsage: 7200,
		// CriticalUsage:   1100,
		Ignores:         []string{"golang"},
		Targets:         []string{"postman", "firefox", "code", "chromium-browser"},
		CriticalTargets: []string{"spotify", "brave"},
		UserInfo:        userInfo,
		OsArgs:          os.Args[1:],
	}

	if contains(context.OsArgs, "--debug") {
		context.Ignores = append(context.Ignores, "code")
	}

	logInfo(context.MaxUsage, context.CriticalUsage)

	logs := 0
	for {
		if logs > 30 {
			callClear()
			logInfo(context.MaxUsage, context.CriticalUsage)
			logs = 0
		}
		memory, err := memory.Get()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			time.Sleep(context.Wait)
			context.Memory = nil
			continue
		}

		context.Memory = memory

		used := toMb(memory.Used)

		if used >= context.MaxUsage {
			log.Printf("memory used: %d MB\n", used)
			log.Println("max usage has been reached")
			context.Processes = psAux()
			killed := killTargets(context)
			checkKilled(killed)
			if used >= context.CriticalUsage {
				log.Println("critical usage has been reached")
				context.KillTask = "CriticalTargets"
				criticalKilled := killTargets(context)
				checkKilled(criticalKilled)
				log.Println("critical kills executed")
			}
		}
		logs++
		context.KillTask = "Targets"
		time.Sleep(context.Wait)
	}
}

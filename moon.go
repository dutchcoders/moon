package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"regexp"

	color2 "github.com/fatih/color"
)

type Moon struct {
	outChan chan Print
	errChan chan Print

	startedChan  chan string
	finishedChan chan string
}

func (m *Moon) run(name, command string) {
	var err error
	defer func() {
		m.finishedChan <- name

		if err == nil {
			return
		}

		m.errChan <- Print{
			Host: "",
			Line: fmt.Sprintf("Error occured: %s\n", err),
		}
	}()

	args := strings.Split(command, " ")

	cmd := exec.Command(args[0], args[1:]...)

	var stdout io.ReadCloser
	if stdout, err = cmd.StdoutPipe(); err != nil {
		err = fmt.Errorf("can't get stdout pipe for command: %s", err)
		return
	}

	var stderr io.ReadCloser
	if stderr, err = cmd.StderrPipe(); err != nil {
		err = fmt.Errorf("can't get stderr pipe for command: %s", err)
		return
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		err = fmt.Errorf("Error occured during start: %s\n", err.Error())
		return
	}

	m.startedChan <- name

	go func() {
		r := bufio.NewReader(stdout)
		for {
			s, err := r.ReadString('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				fmt.Println(color2.RedString("Error reading from stdout: %s", err.Error()))
				break
			}

			m.outChan <- Print{
				Host: name,
				Line: s,
			}
		}
	}()

	go func() {
		r := bufio.NewReader(stderr)
		for {
			s, err := r.ReadString('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				fmt.Println(color2.RedString("Error reading from stderr: %s", err.Error()))
				break
			}

			m.errChan <- Print{
				Host: name,
				Line: s,
			}
		}
	}()

	cmd.Wait()
}

func kill(process *os.Process) error {
	// https://github.com/golang/go/issues/8854
	pgid, err := syscall.Getpgid(process.Pid)
	if err != nil {
		return err
	}

	syscall.Kill(-pgid, syscall.SIGTERM)

	waiter := make(chan struct{})
	go func() {
		process.Wait()
		waiter <- struct{}{}
	}()

	select {
	case <-time.After(10 * time.Second):
		fmt.Fprintln(os.Stderr, color2.RedString("Killing unresponding processes. We've asked them nicely once before."))
		err := syscall.Kill(-pgid, syscall.SIGKILL)
		return err
	case <-waiter:
	}

	return nil
}

func New() *Moon {
	return &Moon{
		outChan:      make(chan Print),
		startedChan:  make(chan string),
		finishedChan: make(chan string),
		errChan:      make(chan Print),
	}
}

type Print struct {
	Host string
	Line string
}

func main() {
	// parse arguments
	a := NewTerminal(os.Stdout)
	a.Reset().DisableCursor()

	defer a.EnableCursor()

	writeToFile := ""

	plus := []*regexp.Regexp{}
	minus := []*regexp.Regexp{}
	highlights := []*regexp.Regexp{}

	for i := 1; i < len(os.Args); i++ {
		negative := false
		highlight := false

		arg := os.Args[i]

		if arg == "-f" {
			i++

			arg = os.Args[i]

			writeToFile = arg
			continue
		}

		if arg == "-v" {
			// negative
			i++
			arg = os.Args[i]
			negative = true
		}

		if arg == "-h" {
			// highlights
			i++
			arg = os.Args[i]
			highlight = true
		}

		r, err := regexp.Compile(arg)
		if err != nil {
			panic(err)
		}

		if negative {
			minus = append(minus, r)

		} else if highlight {
			highlights = append(highlights, r)
		} else {
			plus = append(plus, r)
		}
	}

	var w io.Writer = bufio.NewWriter(ioutil.Discard)

	if writeToFile == "" {
	} else if f, err := os.OpenFile(writeToFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err != nil {
		fmt.Println(err.Error())
		return
	} else {
		w2 := bufio.NewWriter(f)

		defer w2.Flush()

		w = f
	}

	m := New()

	numproc := 0

	// start output handler
	go func() {
		highlightf := color2.New(color2.FgBlack, color2.BgYellow).Sprintf

		for {
			// show counter with active processes, will be erased when new input occurs
			print := func(s Print) {
				fmt.Printf("\033[2K")
				fmt.Printf("\033[999D")

				if len(plus) > 0 {
					match := false

					for _, r := range plus {
						match = match || r.MatchString(s.Line)
					}

					if !match {
						return
					}
				}

				for _, r := range highlights {
					s.Line = r.ReplaceAllString(s.Line, highlightf(`$0`))
				}

				for _, r := range minus {
					if r.MatchString(s.Line) {
						return
					}
				}

				fmt.Printf(String("%{color:2;hiyellow;bg-blue}%{host}%{color:reset} %{time:15:04:05.000}  ▶ %{message}", Message(s.Line), Host(s.Host)))
			}

			banner := func() {
				fmt.Printf("\033[64;44mMoon ▶ running (%d) ▶ \033[0m", numproc)

				d := time.Now().Format("15:04:05")
				fmt.Printf("\033[99C")
				fmt.Printf("\033[%dD", len(d)-1)

				fmt.Fprintf(os.Stdout, "\x1b[;30;1m%s\x1b[0m", d)
			}

			select {
			case s := <-m.startedChan:
				numproc++
				fmt.Printf("\033[2K")
				fmt.Printf("\033[999D")
				fmt.Printf(String("%{color:2;hiyellow;bg-blue}%{host}%{color:reset} %{time:15:04:05.000}  ▶ %{message}\n", Message("started"), Host(s)))
				banner()
			case s := <-m.finishedChan:
				// time process duration
				numproc--
				fmt.Printf("\033[2K")
				fmt.Printf("\033[999D")
				fmt.Printf(String("%{color:2;hiyellow;bg-blue}%{host}%{color:reset} %{time:15:04:05.000}  ▶ %{message}\n", Message("finished"), Host(s)))
				banner()
			case s := <-m.outChan:
				print(s)
				fmt.Fprintf(w, String("%{host} %{time:15:04:05.000}  ▶ %{message}", Message(s.Line), Host(s.Host)))
				banner()
			case s := <-m.errChan:
				print(s)
				fmt.Fprintf(w, String("%{host} %{time:15:04:05.000}  ▶ %{message}", Message(s.Line), Host(s.Host)))
				banner()
			}
		}
	}()

	var wg sync.WaitGroup

	// read commands from stdin
	fi, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}

	if fi.Mode()&os.ModeNamedPipe > 0 {
		scanner := bufio.NewScanner(os.Stdin)

		wg.Add(1)
		for scanner.Scan() {
			s := scanner.Text()

			s = strings.TrimSpace(s)

			if strings.HasPrefix(s, "#") {
				continue
			}

			if s == "" {
				continue
			}
			// we want to sync on all processes to start, to return errors. If all started then start processing
			parts := strings.Split(s, ",")
			fmt.Fprintln(os.Stderr, color2.YellowString("Starting (%s): %+v", parts[0], parts[1:]))

			go m.run(parts[0], parts[1])
		}

		if err := scanner.Err(); err != nil {
			panic(err)
		}
	}

	terminating := make(chan os.Signal, 1)
	signal.Notify(terminating, os.Interrupt)
	signal.Notify(terminating, syscall.SIGTERM)

	<-terminating
}

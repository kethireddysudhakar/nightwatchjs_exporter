package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func start_nightwatch_runner(cfg Config) {
	metrics := setupProm()
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %s", err)
	}
	if err := os.Chdir(cfg.NightwatchjsCmd); err != nil {
		//fmt.Println("inside change Dir")
		log.Fatalf("Error changing to nightwatch directory %s: %s", cfg.NightwatchjsCmd, err)
	}
	fmt.Println(os.Getwd())
	for {
		start := time.Now()
		fmt.Println("workingDirectory:" + wd)
		r, err := run_nightwatch(wd, cfg)
		if err != nil {
			log.Printf("Error with nightwatch - not updating metrics: %s", err)
			metrics.Errors.Inc()
			time.Sleep(cfg.DelayTime)
			continue
		}
		durationSec := time.Now().Sub(start).Seconds()
		for name, module := range r.Modules {
			metrics.ModuleAssertions.WithLabelValues(name).Set(module.AssertionsCount)
			metrics.ModuleTesttime.WithLabelValues(name).Set(module.TestTime)
			metrics.ModuleTestscount.WithLabelValues(name).Set(module.TestsCount)
			metrics.ModuleSkippedcount.WithLabelValues(name).Set(module.SkippedCount)
			metrics.ModuleFailedcount.WithLabelValues(name).Set(module.FailedCount)
			metrics.ModuleErrorscount.WithLabelValues(name).Set(module.ErrorsCount)
			metrics.ModulePassedcount.WithLabelValues(name).Set(module.PassedCount)
			metrics.ModuleTests.WithLabelValues(name).Set(module.Tests)
			metrics.ModuleFailures.WithLabelValues(name).Set(module.Failures)
			metrics.ModuleErrors.WithLabelValues(name).Set(module.Errors)
		}
		metrics.Passed.Set(r.Passed)
		metrics.Failed.Set(r.Failed)
		metrics.Errors.Set(r.Errors)
		metrics.Skipped.Set(r.Skipped)
		metrics.Total.Set(r.Tests)
		metrics.Assertions.Set(r.Assertions)
		metrics.TestDuration.Set(durationSec)
		time.Sleep(cfg.DelayTime)
	}
}

type NightwatchResult struct {
	Passed      float64
	Failed      float64
	Errors      float64
	Skipped     float64
	Tests       float64
	Assertions  float64
	ErrMessages []string
	Modules     map[string]NightwatchModule
}

type NightwatchModule struct {
	ReportPrefix    string
	LastError       interface{} `json:"-"`
	AssertionsCount float64
	Skipped         []interface{} `json:"-"`
	TestTime        float64       `json:"time,string"`
	Completed       map[string]NightwatchTest
	Errmessages     []interface{}
	TestsCount      float64
	SkippedCount    float64
	FailedCount     float64
	ErrorsCount     float64
	PassedCount     float64
	Group           string
	Tests           float64
	Failures        float64
	Errors          float64
}

type NightwatchTest struct {
	TestTime   float64       `json:"time,string"`
	Assertions []interface{} `json:"-"`
	Passed     float64
	Errors     float64
	Failed     float64
	Skipped    float64
	Tests      float64
	Steps      []interface{} `json:"-"`
	LastError  interface{}   `json:"-"`
	StackTrace string        `json:"-"`
	TestCases  interface{}   `json:"-"`
	TimeMs     int
}

func run_nightwatch(wd string, cfg Config) (NightwatchResult, error) {
	var nwResult NightwatchResult
	reporterFile := filepath.Join(wd, "nightwatch_json_reporter.js")
	fmt.Println(os.Getwd())
	//os.Chdir("/Users/sudhakarkethireddy/Documents/synthetics/night3")
	fmt.Println(cfg.NightwatchjsDir + "reporter:" + reporterFile)
	cmd := exec.Command("npx", "nightwatch", cfg.NightwatchjsDir, "--repoter", reporterFile)
	//cmd := exec.Command("npx", "nightwatch", "/Users/sudhakarkethireddy/Documents/synthetics/night3/nightwatch/examples/basic/salesgeico.js", "--reporter", "/Users/sudhakarkethireddy/Documents/GitHub/nightwatchjs_exporter/nightwatch_json_reporter.js")
	cmdout, err := cmd.StdoutPipe()
	if err != nil {
		//fmt.Println("insisde if")
		return nwResult, err
	}
	defer func() {
		cmdout.Close()
	}()
	//fmt.Println(cmdout)
	result := make(chan NightwatchResult)
	nwError := make(chan error)
	//fmt.Println("result:" + nwError.(string))
	go func(stdout io.ReadCloser) {
		/* scanner1 := bufio.NewScanner(stdout)
		scanner1.Split(bufio.ScanLines)
		buf := make([]byte, 0, 64*1024)
		scanner1.Buffer(buf, 1024*1024)
		for ok := true; ok != false; ok = scanner1.Scan() {
			t1 := scanner1.Text()
			fmt.Println("scanner1:" + t1)
			/* if t == "NIGHTWATCHJSON" {
				fmt.Println("textNext" + t)
				nightwatchJson = scanner.Text()
				if ok := scanner.Scan(); ok != true {
					nwError <- fmt.Errorf("Error reading from nightwatch")
					return
				}
				nightwatchJson = scanner.Text()
				fmt.Printf("%+v\n", nightwatchJson)
				break
			}
		} */

		scanner := bufio.NewScanner(stdout)
		scanner.Split(bufio.ScanLines)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		nightwatchJson := ""
		for ok := true; ok != false; ok = scanner.Scan() {
			t := scanner.Text()
			fmt.Println("text:" + t)
			if t == "NIGHTWATCHJSON" {
				fmt.Println("textNext" + t)
				nightwatchJson = scanner.Text()
				if ok := scanner.Scan(); ok != true {
					nwError <- fmt.Errorf("Error reading from nightwatch")
					return
				}
				nightwatchJson = scanner.Text()
				//fmt.Printf("%+v\n", nightwatchJson)
				break
			}
		}
		if scanner.Err() != nil {
			nwError <- fmt.Errorf("Error reading from nightwatch stdout: %s", scanner.Err())
			return
		}
		var r NightwatchResult
		fmt.Printf("%+v\n", r)
		if err := json.Unmarshal([]byte(nightwatchJson), &r); err != nil {
			nwError <- fmt.Errorf("Error parsing nightwatch output: %s: %s", err, nightwatchJson)
			return
		}
		result <- r
		return
	}(cmdout)

	if err := cmd.Start(); err != nil {
		return nwResult, fmt.Errorf("Start error: %s", err)
	}
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// nightwatch calls exit(5) on test failures, which is cool
			if exitErr.ExitCode() != 5 && exitErr.ExitCode() != 10 {
				return nwResult, fmt.Errorf("Nightwatch exited with error: %s", exitErr)
			}
		} else {
			return nwResult, fmt.Errorf("Nightwatch exececution error: %s", err)
		}
	}
	select {
	case e := <-nwError:
		return nwResult, e
	case r := <-result:
		nwResult = r
	}
	return nwResult, nil
}

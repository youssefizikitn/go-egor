package commands

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/chermehdi/egor/config"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/table"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

var (
	AC int8 = 0
	SK int8 = 1
	RE int8 = 2
	WA int8 = 3
)

var (
	green   = color.New(color.FgGreen).SprintfFunc()
	red     = color.New(color.FgRed).SprintfFunc()
	magenta = color.New(color.FgMagenta).SprintfFunc()
	yellow  = color.New(color.FgYellow).SprintfFunc()
)

const WorkDir = "work"

// Checks the output of a given testcase against it's expected output
type Checker interface {
	// Execute the check (got, expected) and returns
	// nil if the output match, otherwise an error with a description message.
	Check(string, string) error
}

// Default implementation of the Checker interface.
type DiffChecker struct {
}

func (c *DiffChecker) Check(got, expected string) error {
	// Compare the trimmed output from both input and output
	if strings.TrimRight(got, " \t\n\r") != strings.TrimRight(expected, " \t\n\r") {
		return errors.New(fmt.Sprintf("Checker failed, expected:\n%s\nfound:\n%s", expected, got))
	}
	return nil
}

// Case description contains minimum information required to run one test case.
type CaseDescription struct {
	InputFile  string
	OutputFile string
	WorkFile   string
	CustomCase bool
}

func getWorkFile(fileName string) string {
	fileNameParts := strings.Split(fileName, ".")
	fileNameNoExtension := fileNameParts[0]
	return fmt.Sprintf("%s-ex.out", fileNameNoExtension)
}

// Creates a new CaseDescription from a pair of input and output IoFiles
func NewCaseDescription(input, output config.IoFile) *CaseDescription {
	base, file := filepath.Split(input.Path)
	workFilePath := path.Join(base, getWorkFile(file))
	return &CaseDescription{
		InputFile:  input.Path,
		OutputFile: output.Path,
		WorkFile:   workFilePath,
		CustomCase: input.Custom,
	}
}

// Report the execution status for a given testcase.
// Type stores also checker response
type CaseStatus struct {
	Status       int8
	CheckerError error
	Stderr       string
}

// Implementation must be able to prepare the working environment to compile and execute testscases,
// And run each testcase and report the status back to the invoker, and perform any necessary cleanup (binaries created, directories created ...)
type Judge interface {
	// setup the working directory and perform any necessary compilation of the task
	// if the setup returned an error, the Judge should abort the operation and report the error back.
	Setup() error

	// Run on every test case, and the status is reported back to the invoker.
	// The implementation is free to Run all testcases at once, or report every test case execution status once it finishes.
	// If it's needed, running independent cases can be done on different go routines.
	RunTestCase(CaseDescription) CaseStatus

	// Return the Checker instance associated with this judge
	Checker() Checker

	// Return the working directory of the judge
	WorkDir() string

	// Cleanup the working directory, if an error occured, implementation must report it to the caller.
	Cleanup() error
}

// This represents the result of running the testcases of a given task
type JudgeReport interface {
	// Add the pair To the list of executions processed
	Add(status CaseStatus, description CaseDescription)

	// Display the current report to os.Stdout
	Display()
}

// Judge report that is printed to the console.
// the report will contain the case descriptions that the judge ran and also their execution status
// The order of insertion is supposed to be the same, i.e the i'th element of the Stats slice correspond to the i'th
// element in the Descs slice.
type ConsoleJudgeReport struct {
	Stats []CaseStatus
	Descs []CaseDescription
}

// Append the pair of status, description to the report object.
func (c *ConsoleJudgeReport) Add(status CaseStatus, description CaseDescription) {
	c.Stats = append(c.Stats, status)
	c.Descs = append(c.Descs, description)
}

// Utility function to get the string representation of some given status.
func getDisplayStatus(status int8) string {
	switch status {
	case AC:
		return green("AC")
	case RE:
		return magenta("RE")
	case SK:
		return yellow("SK")
	case WA:
		return red("WA")
	}
	return "Unknown"
}

func getStderrDisplay(stderr string) string {
	if stderr == "" {
		return "-"
	}
	return red(stderr)
}

func (c *ConsoleJudgeReport) Display() {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Test Name", "Status", "Custom", "Additional infos", "Stderr"})
	for i, stat := range c.Stats {
		output := "None"
		if stat.CheckerError != nil {
			output = fmt.Sprintf("FAILED, %s", stat.CheckerError.Error())
		}
		t.AppendRow([]interface{}{
			i,
			c.Descs[i].InputFile,
			getDisplayStatus(stat.Status),
			c.Descs[i].CustomCase,
			output,
			getStderrDisplay(stat.Stderr),
		})
	}
	t.SetStyle(table.StyleLight)
	t.Render()
}

func newJudgeReport() JudgeReport {
	return &ConsoleJudgeReport{Stats: []CaseStatus{}}
}

// Utility function to execute the given command that is associated with the given judge
// the method returns the case status and the error (if any)
func execute(judge Judge, desc CaseDescription, command string, args ...string) (CaseStatus, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = judge.WorkDir()
	inputFile, err := os.Open(desc.InputFile)
	if err != nil {
		return CaseStatus{
			Status:       RE,
			CheckerError: nil,
		}, err
	}
	defer inputFile.Close()

	outputFile, err := os.OpenFile(desc.WorkFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return CaseStatus{
			Status:       RE,
			CheckerError: nil,
		}, err
	}
	defer outputFile.Close()

	var stderrBuffer bytes.Buffer
	cmd.Stdin = inputFile
	cmd.Stdout = outputFile
	cmd.Stderr = &stderrBuffer
	if err = cmd.Run(); err != nil {
		return CaseStatus{
			Status:       RE,
			CheckerError: nil,
			Stderr:       stderrBuffer.String(),
		}, err
	}

	expectedOutput, err := ioutil.ReadFile(desc.OutputFile)
	if err != nil {
		return CaseStatus{
			Status:       RE,
			CheckerError: nil,
		}, err
	}
	output, err := ioutil.ReadFile(desc.WorkFile)

	if err != nil {
		return CaseStatus{
			Status:       RE,
			CheckerError: nil,
		}, err
	}
	err = judge.Checker().Check(string(output), string(expectedOutput))
	if err != nil {
		return CaseStatus{
			Status:       WA,
			CheckerError: err,
			Stderr:       stderrBuffer.String(),
		}, err
	}
	return CaseStatus{
		Status:       AC,
		CheckerError: nil,
		Stderr:       stderrBuffer.String(),
	}, err
}

//
// Java judge
//
type JavaJudge struct {
	Meta           config.EgorMeta
	CurrentWorkDir string
	checker        Checker
}

func (judge *JavaJudge) Setup() error {
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}
	workDirPath := path.Join(currentDir, WorkDir)
	if _, err = os.Stat(workDirPath); os.IsNotExist(err) {
		if err := os.Mkdir(workDirPath, 0777); err != nil {
			return err
		}
	}
	//TODO(chermehdi): make the executables path configurable #14
	// Compilation for Java
	var stderrBuffer bytes.Buffer
	cmd := exec.Command("javac", judge.Meta.TaskFile, "-d", WorkDir)
	cmd.Dir = currentDir
	cmd.Stderr = &stderrBuffer
	if err = cmd.Run(); err != nil {
		color.Red("Could not  compile, Cause: \n%s", stderrBuffer.String())
		return err
	}
	judge.CurrentWorkDir = workDirPath
	return nil
}

func (judge *JavaJudge) WorkDir() string {
	return judge.CurrentWorkDir
}

func (judge *JavaJudge) RunTestCase(desc CaseDescription) CaseStatus {
	// We suppose that all java executables will be called Main
	caseStatus, _ := execute(judge, desc, "java", "Main")
	return caseStatus
}

func (judge *JavaJudge) Cleanup() error {
	return os.RemoveAll(judge.CurrentWorkDir)
}

func (judge *JavaJudge) Checker() Checker {
	return judge.checker
}

//
// C / Cpp Judge
//
type CppJudge struct {
	Meta           config.EgorMeta
	CurrentWorkDir string
	checker        Checker
}

func (judge *CppJudge) Setup() error {
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}
	workDirPath := path.Join(currentDir, WorkDir)
	if _, err = os.Stat(workDirPath); os.IsNotExist(err) {
		if err := os.Mkdir(workDirPath, 0777); err != nil {
			return err
		}
	}
	var stderrBuffer bytes.Buffer
	cmd := exec.Command("g++", judge.Meta.TaskFile, "-o", "work/sol")
	cmd.Dir = currentDir
	cmd.Stderr = &stderrBuffer
	if err = cmd.Run(); err != nil {
		color.Red("Could not  compile, Cause: \n%s", stderrBuffer.String())
		return err
	}
	judge.CurrentWorkDir = workDirPath
	return nil
}

func (judge *CppJudge) Checker() Checker {
	return judge.checker
}

func (judge *CppJudge) WorkDir() string {
	return judge.CurrentWorkDir
}

func (judge *CppJudge) RunTestCase(desc CaseDescription) CaseStatus {
	// We suppose that all java executables will be called Main
	caseStatus, _ := execute(judge, desc, "./sol")
	return caseStatus
}

func (judge *CppJudge) Cleanup() error {
	return os.RemoveAll(judge.CurrentWorkDir)
}

//
// Python Judge
//
type PythonJudge struct {
	Meta           config.EgorMeta
	CurrentWorkDir string
	checker        Checker
}

func (judge *PythonJudge) Setup() error {
	// No setup required for python
	return nil
}

func (judge *PythonJudge) RunTestCase(desc CaseDescription) CaseStatus {
	caseStatus, _ := execute(judge, desc, "python", "main.py")
	return caseStatus
}

func (judge *PythonJudge) Checker() Checker {
	return judge.checker
}

func (judge *PythonJudge) WorkDir() string {
	return judge.CurrentWorkDir
}
func (judge *PythonJudge) Cleanup() error {
	// No cleanup required for python
	return nil
}

// Creates and returns a Judge implementation corresponding to the given language
func NewJudgeFor(meta config.EgorMeta) (Judge, error) {
	switch meta.TaskLang {
	case "java":
		return &JavaJudge{Meta: meta, checker: &DiffChecker{}}, nil
	case "cpp":
		return &CppJudge{Meta: meta, checker: &DiffChecker{}}, nil
	case "c":
		return &CppJudge{Meta: meta, checker: &DiffChecker{}}, nil
	case "python":
		return &PythonJudge{Meta: meta, checker: &DiffChecker{}}, nil
	}
	return nil, errors.New(fmt.Sprintf("Cannot find judge for the given lang %s", meta.TaskLang))
}

func RunAction(_ *cli.Context) error {
	configuration, err := config.LoadDefaultConfiguration()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	egorMetaFile := path.Join(cwd, configuration.ConfigFileName)
	egorMeta, err := config.LoadMetaFromPath(egorMetaFile)
	if err != nil {
		return err
	}

	judge, err := NewJudgeFor(egorMeta)
	if err != nil {
		return err
	}
	if err := judge.Setup(); err != nil {
		return err
	}
	defer judge.Cleanup()
	report := newJudgeReport()

	for i, input := range egorMeta.Inputs {
		output := egorMeta.Outputs[i]
		caseDescription := NewCaseDescription(input, output)
		status := judge.RunTestCase(*caseDescription)
		report.Add(status, *caseDescription)
	}
	report.Display()
	return nil
}

var TestCommand = cli.Command{
	Name:      "test",
	Aliases:   []string{"r"},
	Usage:     "Run test cases using the provided solution",
	UsageText: "egor test",
	Action:    RunAction,
}

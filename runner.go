package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"talisman/checksumcalculator"
	"talisman/detector"
	"talisman/git_repo"
	"talisman/report"
	"talisman/scanner"
	"talisman/utility"
)

const (
	//CompletedSuccessfully is an exit status that says that the current runners run completed without errors
	CompletedSuccessfully int = 0

	//CompletedWithErrors is an exit status that says that the current runners run completed with failures
	CompletedWithErrors int = 1

	//DefaultScopeConfigFileName represents the name of the file which will have preset ignores for each scope
	DefaultScopeConfigFileName string = "scope_config.go"
)

//Runner represents a single run of the validations for a given commit range
type Runner struct {
	additions []git_repo.Addition
	results   *detector.DetectionResults
}

//NewRunner returns a new Runner.
func NewRunner(additions []git_repo.Addition) *Runner {
	return &Runner{
		additions: additions,
		results:   detector.NewDetectionResults(),
	}
}

//RunWithoutErrors will validate the commit range for errors and return either COMPLETED_SUCCESSFULLY or COMPLETED_WITH_ERRORS
func (r *Runner) RunWithoutErrors() int {
	r.doRun()
	r.printReport()
	return r.exitStatus()
}

//Scan scans git commit history for potential secrets and returns 0 or 1 as exit code
func (r *Runner) Scan(reportDirectory string) int {

	fmt.Printf("\n\n")
	utility.CreateArt("Running Scan..")
	additions := scanner.GetAdditions()
	ignores := detector.TalismanRCIgnore{}
	detector.DefaultChain().Test(additions, ignores, r.results)
	reportsPath := report.GenerateReport(r.results, reportDirectory)
	fmt.Printf("\nPlease check '%s' folder for the talisman scan report\n", reportsPath)
	fmt.Printf("\n")
	return r.exitStatus()
}

//RunChecksumCalculator runs the checksum calculator against the patterns given as input
func (r *Runner) RunChecksumCalculator(fileNamePatterns []string) int {
	exitStatus := 1
	cc := checksumcalculator.NewChecksumCalculator(fileNamePatterns)
	rcSuggestion := cc.SuggestTalismanRC()
	if rcSuggestion != "" {
		fmt.Print(rcSuggestion)
		exitStatus = 0
	}
	return exitStatus
}

func (r *Runner) doRun() {
	ignoresNew := detector.ReadConfigFromRCFile(readRepoFile())
	var applicableScopeFileNames []string
	if ignoresNew.ScopeConfig != nil {
		scopeMap := readScopeConfig()
		for _, scope := range ignoresNew.ScopeConfig {
			if len(scopeMap[scope.ScopeName]) > 0 {
				applicableScopeFileNames = append(applicableScopeFileNames, scopeMap[scope.ScopeName]...)
			}
		}
	}
	additionsToScan := filterAdditionsByScope(r.additions, applicableScopeFileNames)
	detector.DefaultChain().Test(additionsToScan, ignoresNew, r.results)
}

func readScopeConfig() map[string][]string {
	var scope map[string][]string
	err := yaml.Unmarshal([]byte(scopeConfig), &scope)
	if err != nil {
		log.Println("Unable to parse scope_config.go")
		log.Printf("error: %v", err)
		return scope
	}
	return scope
}

func filterAdditionsByScope(additions []git_repo.Addition, scopeFiles []string) []git_repo.Addition {
	var result []git_repo.Addition
	for _, addition := range additions {
		isFilePresentInScope := false
		for _, fileName := range scopeFiles {
			if addition.Matches(fileName) {
				isFilePresentInScope = true
			}
		}
		if !isFilePresentInScope {
			result = append(result, addition)
		}
	}
	return result
}

func (r *Runner) printReport() {
	if r.results.HasWarnings() {
		fmt.Println(r.results.ReportWarnings())
	}
	if r.results.HasIgnores() || r.results.HasFailures() {
		fmt.Println(r.results.Report())
	}
}

func (r *Runner) exitStatus() int {
	if r.results.HasFailures() {
		return CompletedWithErrors
	}
	return CompletedSuccessfully
}

func readRepoFile() func(string) ([]byte, error) {
	wd, _ := os.Getwd()
	repo := git_repo.RepoLocatedAt(wd)
	return repo.ReadRepoFileOrNothing
}

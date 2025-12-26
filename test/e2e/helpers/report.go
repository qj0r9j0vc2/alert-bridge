package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestReport represents a comprehensive test execution report
type TestReport struct {
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time"`
	Duration     string            `json:"duration"`
	TotalTests   int               `json:"total_tests"`
	PassedTests  int               `json:"passed_tests"`
	FailedTests  int               `json:"failed_tests"`
	SkippedTests int               `json:"skipped_tests"`
	TestResults  []TestResult      `json:"test_results"`
	Environment  map[string]string `json:"environment"`
	Summary      string            `json:"summary"`
}

// TestResult represents the result of a single test
type TestResult struct {
	Name      string      `json:"name"`
	Status    string      `json:"status"` // passed, failed, skipped
	Duration  string      `json:"duration"`
	StartTime time.Time   `json:"start_time"`
	EndTime   time.Time   `json:"end_time"`
	Error     string      `json:"error,omitempty"`
	Phases    []TestPhase `json:"phases,omitempty"`
}

// TestPhase represents a phase within a test
type TestPhase struct {
	Name      string    `json:"name"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`
}

// TestReporter manages test reporting
type TestReporter struct {
	report      *TestReport
	currentTest *TestResult
	outputPath  string
}

// NewTestReporter creates a new test reporter
func NewTestReporter(outputPath string) *TestReporter {
	return &TestReporter{
		report: &TestReport{
			StartTime:   time.Now(),
			TestResults: make([]TestResult, 0),
			Environment: make(map[string]string),
		},
		outputPath: outputPath,
	}
}

// StartTest marks the beginning of a test
func (r *TestReporter) StartTest(name string) {
	r.currentTest = &TestResult{
		Name:      name,
		StartTime: time.Now(),
		Phases:    make([]TestPhase, 0),
		Status:    "running",
	}
}

// EndTest marks the end of a test
func (r *TestReporter) EndTest(t *testing.T) {
	if r.currentTest == nil {
		return
	}

	r.currentTest.EndTime = time.Now()
	r.currentTest.Duration = r.currentTest.EndTime.Sub(r.currentTest.StartTime).String()

	if t.Failed() {
		r.currentTest.Status = "failed"
		r.report.FailedTests++
	} else if t.Skipped() {
		r.currentTest.Status = "skipped"
		r.report.SkippedTests++
	} else {
		r.currentTest.Status = "passed"
		r.report.PassedTests++
	}

	r.report.TotalTests++
	r.report.TestResults = append(r.report.TestResults, *r.currentTest)
	r.currentTest = nil
}

// RecordPhase records a test phase
func (r *TestReporter) RecordPhase(name string, startTime time.Time) {
	if r.currentTest == nil {
		return
	}

	endTime := time.Now()
	phase := TestPhase{
		Name:      name,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime).String(),
	}

	r.currentTest.Phases = append(r.currentTest.Phases, phase)
}

// RecordError records a test error
func (r *TestReporter) RecordError(err string) {
	if r.currentTest == nil {
		return
	}

	r.currentTest.Error = err
}

// SetEnvironment sets environment information
func (r *TestReporter) SetEnvironment(key, value string) {
	r.report.Environment[key] = value
}

// Finalize finalizes the report
func (r *TestReporter) Finalize() {
	r.report.EndTime = time.Now()
	r.report.Duration = r.report.EndTime.Sub(r.report.StartTime).String()

	// Generate summary
	r.report.Summary = fmt.Sprintf(
		"%d/%d tests passed (%.1f%%)",
		r.report.PassedTests,
		r.report.TotalTests,
		float64(r.report.PassedTests)/float64(r.report.TotalTests)*100,
	)
}

// SaveJSON saves the report as JSON
func (r *TestReporter) SaveJSON() error {
	r.Finalize()

	// Create directory if it doesn't exist
	dir := filepath.Dir(r.outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal report
	data, err := json.MarshalIndent(r.report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// Write to file
	if err := os.WriteFile(r.outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// PrintSummary prints a human-readable summary
func (r *TestReporter) PrintSummary() {
	r.Finalize()

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println("                       Test Summary                             ")
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("Total Duration:  %s\n", r.report.Duration)
	fmt.Printf("Total Tests:     %d\n", r.report.TotalTests)
	fmt.Printf("Passed:          %d\n", r.report.PassedTests)
	fmt.Printf("Failed:          %d\n", r.report.FailedTests)
	fmt.Printf("Skipped:         %d\n", r.report.SkippedTests)
	fmt.Println()

	if r.report.FailedTests > 0 {
		fmt.Println("Failed Tests:")
		for _, result := range r.report.TestResults {
			if result.Status == "failed" {
				fmt.Printf("  %s (%s)\n", result.Name, result.Duration)
				if result.Error != "" {
					fmt.Printf("    Error: %s\n", result.Error)
				}
			}
		}
		fmt.Println()
	}

	fmt.Printf("Success Rate:    %.1f%%\n", float64(r.report.PassedTests)/float64(r.report.TotalTests)*100)
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println()
}

// PrintDetailedReport prints a detailed test report
func (r *TestReporter) PrintDetailedReport() {
	r.Finalize()

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println("                    Detailed Test Report                        ")
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println()

	for _, result := range r.report.TestResults {
		var statusIcon string
		switch result.Status {
		case "passed":
			statusIcon = "[PASS]"
		case "failed":
			statusIcon = "[FAIL]"
		case "skipped":
			statusIcon = "[SKIP]"
		default:
			statusIcon = "[UNKN]"
		}

		fmt.Printf("%s %s (%s)\n", statusIcon, result.Name, result.Duration)

		if len(result.Phases) > 0 {
			fmt.Println("  Phases:")
			for _, phase := range result.Phases {
				fmt.Printf("    - %s: %s\n", phase.Name, phase.Duration)
			}
		}

		if result.Error != "" {
			fmt.Printf("  Error: %s\n", result.Error)
		}

		fmt.Println()
	}

	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println()
}

// FormatDuration formats a duration in human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// Global reporter instance (for convenience in tests)
var globalReporter *TestReporter

// InitReporter initializes the global reporter
func InitReporter(outputPath string) {
	globalReporter = NewTestReporter(outputPath)

	// Set environment variables
	globalReporter.SetEnvironment("E2E_BASE_PORT", os.Getenv("E2E_BASE_PORT"))
	globalReporter.SetEnvironment("E2E_SERVICE_TIMEOUT", os.Getenv("E2E_SERVICE_TIMEOUT"))
	globalReporter.SetEnvironment("E2E_TEST_TIMEOUT", os.Getenv("E2E_TEST_TIMEOUT"))
}

// GetReporter returns the global reporter
func GetReporter() *TestReporter {
	return globalReporter
}

// SaveReportIfInitialized saves the report if reporter is initialized
func SaveReportIfInitialized() error {
	if globalReporter != nil {
		return globalReporter.SaveJSON()
	}
	return nil
}

// PrintSummaryIfInitialized prints summary if reporter is initialized
func PrintSummaryIfInitialized() {
	if globalReporter != nil {
		globalReporter.PrintSummary()
	}
}

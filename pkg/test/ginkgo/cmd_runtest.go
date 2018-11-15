package ginkgo

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/ginkgo/types"
)

// TestOptions handles running a single test.
type TestOptions struct {
	DryRun      bool
	Out, ErrOut io.Writer
}

func (opt *TestOptions) Run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("only a single test name may be passed")
	}

	tests, err := testsForSuite(config.GinkgoConfig)
	if err != nil {
		return err
	}
	var test *testCase
	for _, t := range tests {
		if t.name == args[0] {
			test = t
			break
		}
	}
	if test == nil {
		return fmt.Errorf("no test exists with that name")
	}

	if opt.DryRun {
		fmt.Printf("Running test (dry-run)\n")
		return nil
	}

	config.GinkgoConfig.FocusString = fmt.Sprintf("^%s$", regexp.QuoteMeta(" [Top Level] "+test.name))
	config.DefaultReporterConfig.NoColor = true
	w := ginkgo.GinkgoWriterType()
	w.SetStream(true)
	reporter := NewMinimalReporter(test.name, test.location)
	ginkgo.GlobalSuite().Run(reporter, "", []reporters.Reporter{reporter}, w, config.GinkgoConfig)
	summary, setup := reporter.Summary()
	if summary == nil && setup != nil {
		summary = &types.SpecSummary{
			Failure: setup.Failure,
			State:   setup.State,
		}
	}

	// TODO: print stack line?
	switch {
	case summary == nil:
		return fmt.Errorf("test suite set up failed, see logs")
	case summary.Passed():
	case summary.Skipped():
		if len(summary.Failure.Message) > 0 {
			fmt.Fprintf(os.Stderr, "skip [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.Message)
		}
		if len(summary.Failure.ForwardedPanic) > 0 {
			fmt.Fprintf(os.Stderr, "skip [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.ForwardedPanic)
		}
		os.Exit(3)
	case summary.Failed(), summary.Panicked():
		if len(summary.Failure.ForwardedPanic) > 0 {
			if len(summary.Failure.Location.FullStackTrace) > 0 {
				fmt.Fprintf(os.Stderr, "\n%s\n", summary.Failure.Location.FullStackTrace)
			}
			fmt.Fprintf(os.Stderr, "fail [%s:%d]: Test Panicked: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.ForwardedPanic)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "fail [%s:%d]: %s\n", lastFilenameSegment(summary.Failure.Location.FileName), summary.Failure.Location.LineNumber, summary.Failure.Message)
		os.Exit(1)
	default:
		return fmt.Errorf("unrecognized test case outcome: %#v", summary)
	}
	return nil
}

func lastFilenameSegment(filename string) string {
	if parts := strings.Split(filename, "/vendor/"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	if parts := strings.Split(filename, "/src/"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return filename
}

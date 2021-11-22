// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/google/go-cmp/cmp"
)

const expectedTestResultSuffix = "-expected.json"

var dynamicFieldKeys = []string{"agent","elastic_agent","host","cloud"}

func writeTestResult(testCasePath string, events testrunner.Events) error {
	var trd testrunner.TestExpectedResult

	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	trd.Expected = events
	data, err := json.Marshal(&trd)
	if err != nil {
		return fmt.Errorf("unable to marshal test results: %w", err)
	}
	err = os.WriteFile(filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile)), data, 0644)
	if err != nil {
		return fmt.Errorf("unable to write test results file: %w", err)
	}
	return nil
}

func diffEvents(testCasePath string, config *testConfig, events testrunner.Events) error {
	got, err := filterResults(events, config)
	if err != nil {
		return fmt.Errorf("unable to adjust test results: %w", err)
	}

	want, err := readExpectedEvents(testCasePath, config)
	if err != nil {
		return fmt.Errorf("unable to read expected events: %w", err)
	}

	if report := cmp.Diff(want, got); report != "" {
		reportDetails := fmt.Sprintf("Events mismatch (-want +got):\n%s", report)
		logger.Debug(reportDetails)
		return testrunner.ErrTestCaseFailed{
			Reason: "Expected results are different from actual ones",
			Details: reportDetails,
		}
	} else {
		logger.Debugf("All events match (want: %d got: %d)", len(want), len(got))
	}

	return nil
}

func filterResults(events testrunner.Events, config *testConfig) (testrunner.Events, error) {
	var filtered testrunner.Events
	for i, event := range events {
		if event == nil {
			filtered = append(filtered, nil)
			continue
		}
		for _, key := range dynamicFieldKeys {
			if err := event.Delete(key); err != nil && err != common.ErrKeyNotFound {
				return nil, fmt.Errorf("unable to remove dynamic field %q from event %d: %w", key, i, err)
			}
		}
		for key := range config.DynamicFields {
			err := event.Delete(key)
			if err != nil && err != common.ErrKeyNotFound {
				return nil, fmt.Errorf("unable to remove dynamic field %q from event %d: %w", key, i, err)
			}
		}

		filtered = append(filtered, event)
	}

	return filtered, nil
}

func readExpectedEvents(testCasePath string, config *testConfig) (testrunner.Events, error) {
	var trd testrunner.TestExpectedResult

	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	path := filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to open expected events file: %w", err)
	}

	if err = json.Unmarshal(data, &trd); err != nil {
		return nil, fmt.Errorf("unable to unmarshal expected events file: %w", err)
	}

	if trd.Expected, err = filterResults(trd.Expected, config); err != nil {
		return nil, fmt.Errorf("unable to filter expected events: %w", err)
	}

	return trd.Expected, nil
}

func expectedTestResultFile(testFile string) string {
	return strings.TrimSuffix(testFile, filepath.Ext(testFile))+expectedTestResultSuffix
}

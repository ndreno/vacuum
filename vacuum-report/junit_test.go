// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package vacuum_report

import (
	"encoding/xml"
	"github.com/daveshanley/vacuum/model"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"testing"
	"time"
)

func TestBuildJUnitReport(t *testing.T) {
	tests := []struct {
		name       string
		severity   string
		categoryID string
		category   string
		expectFail bool
	}{
		{"error result", model.SeverityError, model.CategoryExamples, "OAS Linting - Examples", true},
		{"warn result", model.SeverityWarn, model.CategoryOperations, "OAS Linting - Operations", true},
		{"info result", model.SeverityInfo, model.CategorySchemas, "OAS Linting - Schemas", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := buildFakeResultSet(
				"testing, 123",
				"$.somewhere.out.there",
				"one",
				tt.severity,
				tt.categoryID,
				tt.category,
				"test",
				1,
			)

			f := time.Now().Add(-time.Millisecond * 5)
			data := BuildJUnitReport(rs, f, []string{"test"})
			assert.NotEmpty(t, data, "JUnit report should not be empty")

			var suites TestSuites
			err := xml.Unmarshal(data, &suites)
			assert.NoError(t, err)

			// Root checks
			expectedTests := 1
			expectedFailures := 0
			if tt.expectFail {
				expectedFailures = 1
			}
			assert.Equal(t, expectedTests, suites.Tests)
			assert.Equal(t, expectedFailures, suites.Failures)
			assert.Len(t, suites.TestSuites, 1)

			suite := suites.TestSuites[0]
			assert.Equal(t, tt.category, suite.Name)
			assert.Equal(t, 1, suite.Tests)
			assert.Equal(t, expectedFailures, suite.Failures)

			// Test case checks
			assert.Len(t, suite.TestCases, 1)
			tc := suite.TestCases[0]
			assert.Equal(t, "oas-linter.one", tc.ClassName)
			assert.Contains(t, tc.Name, "$.somewhere.out.there")

			// Failure checks
			if tt.expectFail {
				assert.NotNil(t, tc.Failure, "Failure block must be present")
				assert.Contains(t, tc.Failure.Contents, "testing, 123")
				assert.Contains(t, tc.Failure.Contents, "File: test")
				assert.Contains(t, tc.Failure.Contents, "Line: 1")
			}

			// Properties checks
			assert.NotNil(t, tc.Properties)
			props := map[string]string{}
			for _, p := range tc.Properties.Properties {
				props[p.Name] = p.Value
			}
			assert.Equal(t, "one", props["rule"])
			assert.Equal(t, tt.severity, props["severity"])
			assert.Equal(t, "1", props["line"])
			assert.Equal(t, "test", props["file"])
			assert.Equal(t, "$.somewhere.out.there", props["json_path"]) // matches actual XML
		})
	}
}

func TestJUnitXMLIsGitLabCompatible(t *testing.T) {
	rs := buildFakeResultSet(
		"testing, 123",
		"$.somewhere.out.there",
		"one",
		model.SeverityError,
		model.CategoryExamples,
		"OAS Linting - Examples",
		"test",
		1,
	)

	data := BuildJUnitReport(rs, time.Now().Add(-time.Millisecond*5), []string{"test"})
	assert.NotEmpty(t, data, "JUnit report should not be empty")

	var suites TestSuites
	err := xml.Unmarshal(data, &suites)
	assert.NoError(t, err, "XML must unmarshal correctly")

	// Root attributes
	assert.GreaterOrEqual(t, suites.Tests, 1)
	assert.GreaterOrEqual(t, suites.Failures, 1)
	assert.True(t, suites.Time > 0)

	// Suites
	assert.Len(t, suites.TestSuites, 1)
	suite := suites.TestSuites[0]
	assert.NotEmpty(t, suite.Name)
	assert.Equal(t, 1, suite.Tests)
	assert.Equal(t, 1, suite.Failures)
	assert.True(t, suite.Time > 0)

	// Test case
	assert.Len(t, suite.TestCases, 1)
	tc := suite.TestCases[0]
	assert.NotEmpty(t, tc.Name)
	assert.NotEmpty(t, tc.ClassName)

	// Failure must exist for errors/warnings
	assert.NotNil(t, tc.Failure)
	assert.Contains(t, tc.Failure.Contents, "testing, 123")

	// Properties
	assert.NotNil(t, tc.Properties)
	props := map[string]string{}
	for _, p := range tc.Properties.Properties {
		props[p.Name] = p.Value
	}
	assert.Equal(t, "one", props["rule"])
	assert.Equal(t, model.SeverityError, props["severity"])
	assert.Equal(t, "1", props["line"])
	assert.Equal(t, "test", props["file"])
	assert.Equal(t, "$.somewhere.out.there", props["json_path"])
}

func buildFakeResultSet(
	message, path, ruleID, severity, categoryID, categoryName, file string, line int,
) *model.RuleResultSet {
	res := &model.RuleFunctionResult{
		Message: message,
		Path:    path,
		RuleId:  ruleID,
		Rule: &model.Rule{
			Id:       ruleID,
			Severity: severity,
			RuleCategory: &model.RuleCategory{
				Id:   categoryID,
				Name: categoryName,
			},
		},
		StartNode: &yaml.Node{Line: line},
	}

	rs := &model.RuleResultSet{
		Results:     []*model.RuleFunctionResult{res},
		CategoryMap: make(map[*model.RuleCategory][]*model.RuleFunctionResult),
	}
	rs.CategoryMap[res.Rule.RuleCategory] = []*model.RuleFunctionResult{res}

	return rs
}

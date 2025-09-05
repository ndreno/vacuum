// Copyright 2023 Princess B33f Heavy Industries / Dave Shanley
// SPDX-License-Identifier: MIT

package vacuum_report

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/daveshanley/vacuum/model"
	"strings"
	"text/template"
	"time"
)

type TestSuites struct {
	XMLName    xml.Name     `xml:"testsuites"`
	TestSuites []*TestSuite `xml:"testsuite"`
	Tests      int          `xml:"tests,attr"`
	Failures   int          `xml:"failures,attr"`
	Time       float64      `xml:"time,attr"`
}

type TestSuite struct {
	XMLName   xml.Name    `xml:"testsuite"`
	Name      string      `xml:"name,attr"`
	Tests     int         `xml:"tests,attr"`
	Failures  int         `xml:"failures,attr"`
	Time      float64     `xml:"time,attr"`
	TestCases []*TestCase `xml:"testcase"`
}

type Properties struct {
	XMLName    xml.Name    `xml:"properties"`
	Properties []*Property `xml:"property"`
}

type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type TestCase struct {
	Name       string      `xml:"name,attr"`
	ClassName  string      `xml:"classname,attr"`
	Failure    *Failure    `xml:"failure,omitempty"`
	Properties *Properties `xml:"properties,omitempty"`
}

type Failure struct {
	Message  string `xml:"message,attr"`
	Type     string `xml:"type,attr"`
	Contents string `xml:",innerxml"`
}

func BuildJUnitReport(resultSet *model.RuleResultSet, t time.Time, args []string) []byte {
	since := time.Since(t)
	var suites []*TestSuite
	var cats = model.RuleCategoriesOrdered
	tmpl := `File: {{ .File }}
Line: {{ .Line }}
JSON Path: {{ .Path }}
Rule: {{ .RuleId }}
Severity: {{ .Severity }}

{{ .Message }}`

	parsedTemplate, err := template.New("failure").Parse(tmpl)
	if err != nil {
		// Handle error, e.g., log it or return an empty report
		return []byte{}
	}

	gf, gtc := 0, 0 // global failure count, global test cases count

	for _, val := range cats {
		categoryResults := resultSet.GetResultsByRuleCategory(val.Id)
		f := 0
		var tc []*TestCase

		for _, r := range categoryResults {
			line := 1
			if r.StartNode != nil {
				line = r.StartNode.Line
			}

			file := ""
			if r.Origin != nil && r.Origin.AbsoluteLocation != "" {
				file = r.Origin.AbsoluteLocation
			} else if len(args) > 0 {
				file = args[0]
			}

			// Prepare template data
			templateData := struct {
				File     string
				Line     int
				Path     string
				RuleId   string
				Severity string
				Message  string
			}{
				File:     file,
				Line:     line,
				Path:     r.Path,
				RuleId:   r.Rule.Id,
				Severity: r.Rule.Severity,
				Message:  r.Message,
			}

			var sb bytes.Buffer
			err := parsedTemplate.Execute(&sb, templateData)
			if err != nil {
				// Handle error, e.g., log it or skip this test case
				continue
			}

			if r.Rule.Severity == model.SeverityError || r.Rule.Severity == model.SeverityWarn {
				f++
				gf++
			}

			// Create test case name with rule and location info
			testCaseName := fmt.Sprintf("Rule: %s - JSON Path: %s", r.Rule.Id, r.Path)
			if len(testCaseName) > 200 { // Prevent excessively long names
				testCaseName = testCaseName[:200] + "..."
			}

			tCase := &TestCase{
				Name:      testCaseName, // This should now be the descriptive name
				ClassName: fmt.Sprintf("oas-linter.%s", r.Rule.Id),
				Failure: &Failure{
					Message:  r.Message,
					Type:     strings.ToUpper(r.Rule.Severity),
					Contents: sb.String(),
				},
				Properties: &Properties{
					Properties: []*Property{
						{Name: "rule", Value: r.Rule.Id},
						{Name: "severity", Value: r.Rule.Severity},
						{Name: "line", Value: fmt.Sprintf("%d", line)},
						{Name: "file", Value: file},
						{Name: "json_path", Value: r.Path},
					},
				},
			}
			tc = append(tc, tCase)
		}

		if len(tc) > 0 {
			ts := &TestSuite{
				Name:      fmt.Sprintf("OAS Linting - %s", val.Name), // Improved suite name
				Tests:     len(categoryResults),
				Failures:  f,
				Time:      since.Seconds(),
				TestCases: tc,
			}
			suites = append(suites, ts)
		}
		gtc += len(tc)
	}

	allSuites := &TestSuites{
		TestSuites: suites,
		Tests:      gtc,
		Failures:   gf,
		Time:       since.Seconds(),
	}

	// Add XML declaration
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")
	if err := encoder.Encode(allSuites); err != nil {
		// Handle error, e.g., log it or return an empty report
		return []byte{}
	}

	return buf.Bytes()
}

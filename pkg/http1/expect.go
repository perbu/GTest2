package http1

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Expect performs an assertion on HTTP fields
// field: the field to check (e.g., "req.method", "resp.status", "resp.http.content-type")
// op: comparison operator (==, !=, <, >, <=, >=, ~)
// expected: the expected value
func (h *HTTP) Expect(field, op, expected string) error {
	// Get the actual value
	actual, err := h.getField(field)
	if err != nil {
		return err
	}

	// Perform comparison
	result, err := compare(actual, op, expected)
	if err != nil {
		return err
	}

	if !result {
		return fmt.Errorf("expect failed: %s (%s) %s %s", field, actual, op, expected)
	}

	h.Logger.Log(4, "expect %s (%s) %s %s - OK", field, actual, op, expected)
	return nil
}

// getField retrieves the value of a field from the HTTP session
func (h *HTTP) getField(field string) (string, error) {
	parts := strings.SplitN(field, ".", 3)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid field: %s", field)
	}

	category := parts[0] // req or resp
	name := parts[1]

	switch category {
	case "req":
		return h.getRequestField(name, parts)
	case "resp":
		return h.getResponseField(name, parts)
	default:
		return "", fmt.Errorf("unknown field category: %s", category)
	}
}

// getRequestField retrieves a request field value
func (h *HTTP) getRequestField(name string, parts []string) (string, error) {
	switch name {
	case "method":
		return h.Method, nil
	case "url":
		return h.URL, nil
	case "proto":
		return h.Proto, nil
	case "body":
		return string(h.Body), nil
	case "bodylen":
		return strconv.Itoa(h.BodyLen), nil
	case "http":
		// req.http.headername
		if len(parts) < 3 {
			return "", fmt.Errorf("missing header name")
		}
		return h.GetRequestHeader(parts[2]), nil
	default:
		return "", fmt.Errorf("unknown request field: %s", name)
	}
}

// getResponseField retrieves a response field value
func (h *HTTP) getResponseField(name string, parts []string) (string, error) {
	switch name {
	case "status":
		return strconv.Itoa(h.Status), nil
	case "reason":
		return h.Reason, nil
	case "proto":
		return h.Proto, nil
	case "body":
		return string(h.Body), nil
	case "bodylen":
		return strconv.Itoa(h.BodyLen), nil
	case "http":
		// resp.http.headername
		if len(parts) < 3 {
			return "", fmt.Errorf("missing header name")
		}
		return h.GetResponseHeader(parts[2]), nil
	default:
		return "", fmt.Errorf("unknown response field: %s", name)
	}
}

// compare performs the comparison operation
func compare(actual, op, expected string) (bool, error) {
	switch op {
	case "==":
		return actual == expected, nil
	case "!=":
		return actual != expected, nil
	case "~":
		// Regex match
		re, err := regexp.Compile(expected)
		if err != nil {
			return false, fmt.Errorf("invalid regex %s: %w", expected, err)
		}
		return re.MatchString(actual), nil
	case "!~":
		// Regex not match
		re, err := regexp.Compile(expected)
		if err != nil {
			return false, fmt.Errorf("invalid regex %s: %w", expected, err)
		}
		return !re.MatchString(actual), nil
	case "<", ">", "<=", ">=":
		return compareNumeric(actual, op, expected)
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

// compareNumeric performs numeric comparison
func compareNumeric(actual, op, expected string) (bool, error) {
	// Try to parse as integers first
	actualInt, err1 := strconv.ParseInt(actual, 10, 64)
	expectedInt, err2 := strconv.ParseInt(expected, 10, 64)

	if err1 == nil && err2 == nil {
		// Both are integers
		switch op {
		case "<":
			return actualInt < expectedInt, nil
		case ">":
			return actualInt > expectedInt, nil
		case "<=":
			return actualInt <= expectedInt, nil
		case ">=":
			return actualInt >= expectedInt, nil
		}
	}

	// Try as floats
	actualFloat, err1 := strconv.ParseFloat(actual, 64)
	expectedFloat, err2 := strconv.ParseFloat(expected, 64)

	if err1 != nil || err2 != nil {
		return false, fmt.Errorf("cannot compare non-numeric values with %s", op)
	}

	switch op {
	case "<":
		return actualFloat < expectedFloat, nil
	case ">":
		return actualFloat > expectedFloat, nil
	case "<=":
		return actualFloat <= expectedFloat, nil
	case ">=":
		return actualFloat >= expectedFloat, nil
	}

	return false, fmt.Errorf("unknown numeric operator: %s", op)
}
